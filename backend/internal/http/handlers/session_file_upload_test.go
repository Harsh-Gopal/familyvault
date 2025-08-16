package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"familyvault/internal/auth"
	"familyvault/internal/core/drive"

	"github.com/gorilla/mux"
)

func TestSessionFileUploadHandler(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	sessionID := "test-upload-session"

	tests := []struct {
		name           string
		sessionID      string
		setupAuth      func(*http.Request)
		setupBody      func() (io.Reader, string)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "successful multipart upload",
			sessionID: sessionID,
			setupAuth: func(req *http.Request) {
				// Mock user context with upload permission
				user := &auth.Claims{Role: auth.RoleUser}
				ctx := context.WithValue(req.Context(), auth.UserContextKey, user)
				*req = *req.WithContext(ctx)
			},
			setupBody: func() (io.Reader, string) {
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)

				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("test file content"))
				writer.Close()

				return &buf, writer.FormDataContentType()
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response FileUploadResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if response.Filename != "test.txt" {
					t.Errorf("Expected filename test.txt, got %s", response.Filename)
				}
				if response.Size != 17 {
					t.Errorf("Expected size 17, got %d", response.Size)
				}
				if response.SessionID != sessionID {
					t.Errorf("Expected session ID %s, got %s", sessionID, response.SessionID)
				}
			},
		},
		{
			name:      "upload without permission",
			sessionID: sessionID,
			setupAuth: func(req *http.Request) {
				// Mock user context without upload permission
				user := &auth.Claims{Role: auth.RoleViewer}
				ctx := context.WithValue(req.Context(), auth.UserContextKey, user)
				*req = *req.WithContext(ctx)
			},
			setupBody: func() (io.Reader, string) {
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)

				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("test content"))
				writer.Close()

				return &buf, writer.FormDataContentType()
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:      "upload without authentication",
			sessionID: sessionID,
			setupAuth: func(req *http.Request) {
				// No authentication
			},
			setupBody: func() (io.Reader, string) {
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)

				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("test content"))
				writer.Close()

				return &buf, writer.FormDataContentType()
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:      "invalid session ID",
			sessionID: "invalid@session",
			setupAuth: func(req *http.Request) {
				user := &auth.Claims{Role: auth.RoleUser}
				ctx := context.WithValue(req.Context(), auth.UserContextKey, user)
				*req = *req.WithContext(ctx)
			},
			setupBody: func() (io.Reader, string) {
				return strings.NewReader(""), "text/plain"
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, contentType := tt.setupBody()

			url := fmt.Sprintf("/sessions/%s/files/upload", tt.sessionID)
			req := httptest.NewRequest(http.MethodPost, url, body)
			req.Header.Set("Content-Type", contentType)

			tt.setupAuth(req)

			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/files/upload", SessionFileUploadHandler)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				t.Logf("Response body: %s", w.Body.String())
				return
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestSessionFileUploadHandlerResumable(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	sessionID := "test-resumable-session"

	// Test resumable upload
	t.Run("resumable upload", func(t *testing.T) {
		content := "This is a test file for resumable upload"
		filename := "resumable-test.txt"

		// First chunk
		firstChunk := content[:20]
		req1 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sessions/%s/files/upload", sessionID),
			strings.NewReader(firstChunk))
		req1.Header.Set("Upload-Resumable", "1")
		req1.Header.Set("Upload-Filename", filename)
		req1.Header.Set("Upload-Length", fmt.Sprintf("%d", len(content)))
		req1.Header.Set("Upload-Offset", "0")

		// Mock authentication
		user := &auth.Claims{Role: auth.RoleUser}
		ctx := context.WithValue(req1.Context(), auth.UserContextKey, user)
		req1 = req1.WithContext(ctx)

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files/upload", SessionFileUploadHandler)

		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)

		if w1.Code != http.StatusNoContent {
			t.Errorf("Expected status 204 for partial upload, got %d", w1.Code)
			return
		}

		// Check offset header
		if w1.Header().Get("Upload-Offset") != "20" {
			t.Errorf("Expected Upload-Offset 20, got %s", w1.Header().Get("Upload-Offset"))
		}

		// Second chunk (complete upload)
		secondChunk := content[20:]
		req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sessions/%s/files/upload", sessionID),
			strings.NewReader(secondChunk))
		req2.Header.Set("Upload-Resumable", "1")
		req2.Header.Set("Upload-Filename", filename)
		req2.Header.Set("Upload-Length", fmt.Sprintf("%d", len(content)))
		req2.Header.Set("Upload-Offset", "20")

		ctx2 := context.WithValue(req2.Context(), auth.UserContextKey, user)
		req2 = req2.WithContext(ctx2)

		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		if w2.Code != http.StatusCreated {
			t.Errorf("Expected status 201 for complete upload, got %d", w2.Code)
			t.Logf("Response body: %s", w2.Body.String())
			return
		}

		// Verify response
		var response FileUploadResponse
		err := json.Unmarshal(w2.Body.Bytes(), &response)
		if err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Size != int64(len(content)) {
			t.Errorf("Expected size %d, got %d", len(content), response.Size)
		}

		// Verify file was created correctly
		filePath := filepath.Join(tempDir, "uploads", sessionID, filename)
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read uploaded file: %v", err)
		}

		if string(fileContent) != content {
			t.Errorf("File content mismatch. Expected: %s, Got: %s", content, string(fileContent))
		}
	})
}

func TestGenerateUniqueFilename(t *testing.T) {
	tempDir := t.TempDir()

	// Create existing file
	existingFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(existingFile, []byte("existing"), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "non-existing file",
			filename: "new.txt",
			expected: "new.txt",
		},
		{
			name:     "existing file",
			filename: "test.txt",
			expected: "test_", // Should have timestamp appended
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateUniqueFilename(tempDir, tt.filename)

			if tt.expected == "test_" {
				// For existing files, check that it starts with expected prefix
				if !strings.HasPrefix(result, tt.expected) {
					t.Errorf("Expected filename to start with %s, got %s", tt.expected, result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestSessionFileUploadHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/sessions/test/files/upload", nil)

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/files/upload", SessionFileUploadHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for GET method, got %d", w.Code)
	}
}
