package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"familyvault/internal/core/drive"

	"github.com/gorilla/mux"
)

func TestSessionFileDownloadHandler(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with files
	sessionID := "test-download-session"
	sessionPath := filepath.Join(tempDir, "uploads", sessionID)
	err := os.MkdirAll(sessionPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"document.txt": "This is a test document content",
		"image.jpg":    "fake image content",
		"data.csv":     "col1,col2\nval1,val2",
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(sessionPath, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	tests := []struct {
		name           string
		sessionID      string
		filename       string
		headers        map[string]string
		expectedStatus int
		expectedSize   int
		checkHeaders   func(*testing.T, http.Header)
	}{
		{
			name:           "download existing file",
			sessionID:      sessionID,
			filename:       "document.txt",
			expectedStatus: http.StatusOK,
			expectedSize:   len(testFiles["document.txt"]),
			checkHeaders: func(t *testing.T, headers http.Header) {
				if !strings.Contains(headers.Get("Content-Disposition"), "document.txt") {
					t.Error("Content-Disposition header missing filename")
				}
				if headers.Get("Content-Type") != "application/octet-stream" {
					t.Error("Content-Type should be application/octet-stream")
				}
			},
		},
		{
			name:           "download with gzip compression",
			sessionID:      sessionID,
			filename:       "document.txt",
			headers:        map[string]string{"Accept-Encoding": "gzip"},
			expectedStatus: http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				if headers.Get("Content-Encoding") != "gzip" {
					t.Error("Content-Encoding should be gzip")
				}
			},
		},
		{
			name:           "range request",
			sessionID:      sessionID,
			filename:       "document.txt",
			headers:        map[string]string{"Range": "bytes=0-10"},
			expectedStatus: http.StatusPartialContent,
			expectedSize:   11, // bytes 0-10 inclusive
			checkHeaders: func(t *testing.T, headers http.Header) {
				if !strings.Contains(headers.Get("Content-Range"), "bytes 0-10") {
					t.Error("Content-Range header incorrect")
				}
			},
		},
		{
			name:           "invalid session ID",
			sessionID:      "invalid@session",
			filename:       "document.txt",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid filename",
			sessionID:      sessionID,
			filename:       "invalid@filename",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "non-existent file",
			sessionID:      sessionID,
			filename:       "nonexistent.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "non-existent session",
			sessionID:      "nonexistent-session",
			filename:       "document.txt",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/sessions/%s/files/%s/download", tt.sessionID, tt.filename)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("X-Session-ID", "test-session")

			// Set additional headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Set up mux router
			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/files/{filename}/download", SessionFileDownloadHandler)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				return
			}

			if tt.expectedStatus == http.StatusOK || tt.expectedStatus == http.StatusPartialContent {
				if tt.expectedSize > 0 {
					// For gzip, we can't easily check exact size, so skip size check
					if tt.headers["Accept-Encoding"] != "gzip" && len(w.Body.Bytes()) != tt.expectedSize {
						t.Errorf("Expected size %d, got %d", tt.expectedSize, len(w.Body.Bytes()))
					}
				}

				if tt.checkHeaders != nil {
					tt.checkHeaders(t, w.Header())
				}
			}
		})
	}
}

func TestSessionFileDownloadHandlerAuthentication(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	sessionID := "auth-test-session"
	filename := "test.txt"

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files/%s/download", sessionID, filename), nil)
	// No authentication header

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/files/{filename}/download", SessionFileDownloadHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestSessionFileDownloadHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/sessions/test/files/test.txt/download", nil)
	req.Header.Set("X-Session-ID", "test-session")

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/files/{filename}/download", SessionFileDownloadHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for POST method, got %d", w.Code)
	}
}

func TestHandleRangeRequest(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with file
	sessionID := "range-test-session"
	sessionPath := filepath.Join(tempDir, "uploads", sessionID)
	err := os.MkdirAll(sessionPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create test file with known content
	content := "0123456789abcdefghijklmnopqrstuvwxyz"
	filename := "range-test.txt"
	filePath := filepath.Join(sessionPath, filename)
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name           string
		rangeHeader    string
		expectedStatus int
		expectedSize   int
		expectedRange  string
	}{
		{
			name:           "valid range start-end",
			rangeHeader:    "bytes=0-9",
			expectedStatus: http.StatusPartialContent,
			expectedSize:   10,
			expectedRange:  "bytes 0-9/36",
		},
		{
			name:           "valid range start only",
			rangeHeader:    "bytes=10-",
			expectedStatus: http.StatusPartialContent,
			expectedSize:   26, // from position 10 to end
			expectedRange:  "bytes 10-35/36",
		},
		{
			name:           "invalid range format",
			rangeHeader:    "bytes=invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "range not satisfiable",
			rangeHeader:    "bytes=100-200",
			expectedStatus: http.StatusRequestedRangeNotSatisfiable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/sessions/%s/files/%s/download", sessionID, filename)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("X-Session-ID", "test-session")
			req.Header.Set("Range", tt.rangeHeader)

			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/files/{filename}/download", SessionFileDownloadHandler)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				return
			}

			if tt.expectedStatus == http.StatusPartialContent {
				if len(w.Body.Bytes()) != tt.expectedSize {
					t.Errorf("Expected size %d, got %d", tt.expectedSize, len(w.Body.Bytes()))
				}

				if w.Header().Get("Content-Range") != tt.expectedRange {
					t.Errorf("Expected Content-Range %s, got %s", tt.expectedRange, w.Header().Get("Content-Range"))
				}
			}
		})
	}
}
