package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
)

func TestUploadFileHandler(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear()

	// Create test session
	testSession, err := session.Open(time.Hour)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	tests := []struct {
		name           string
		sessionID      string
		filename       string
		content        string
		tags           string
		expectedStatus int
		expectFile     bool
	}{
		{
			name:           "successful upload with tags",
			sessionID:      testSession.ID,
			filename:       "document.pdf",
			content:        "This is a PDF document content",
			tags:           `{"category":"document","priority":"high"}`,
			expectedStatus: http.StatusCreated,
			expectFile:     true,
		},
		{
			name:           "successful upload without tags",
			sessionID:      testSession.ID,
			filename:       "image.jpg",
			content:        "This is a JPEG image content",
			tags:           "",
			expectedStatus: http.StatusCreated,
			expectFile:     true,
		},
		{
			name:           "invalid session",
			sessionID:      "invalid-session-id",
			filename:       "test.txt",
			content:        "test content",
			tags:           "",
			expectedStatus: http.StatusUnauthorized,
			expectFile:     false,
		},
		{
			name:           "missing session",
			sessionID:      "",
			filename:       "test.txt",
			content:        "test content",
			tags:           "",
			expectedStatus: http.StatusUnauthorized,
			expectFile:     false,
		},
		{
			name:           "empty file",
			sessionID:      testSession.ID,
			filename:       "empty.txt",
			content:        "",
			tags:           "",
			expectedStatus: http.StatusBadRequest,
			expectFile:     false,
		},
		{
			name:           "invalid file extension",
			sessionID:      testSession.ID,
			filename:       "malware.exe",
			content:        "malicious content",
			tags:           "",
			expectedStatus: http.StatusBadRequest,
			expectFile:     false,
		},
		{
			name:           "invalid tags JSON",
			sessionID:      testSession.ID,
			filename:       "test.txt",
			content:        "test content",
			tags:           `{"invalid": json}`,
			expectedStatus: http.StatusBadRequest,
			expectFile:     false,
		},
		{
			name:           "unsafe filename",
			sessionID:      testSession.ID,
			filename:       "../../../etc/passwd",
			content:        "malicious content",
			tags:           "",
			expectedStatus: http.StatusBadRequest,
			expectFile:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create multipart form
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			// Add file field
			if tt.content != "" || tt.filename != "" {
				fileWriter, err := writer.CreateFormFile("file", tt.filename)
				if err != nil {
					t.Fatalf("Failed to create form file: %v", err)
				}
				if _, err := fileWriter.Write([]byte(tt.content)); err != nil {
					t.Fatalf("Failed to write file content: %v", err)
				}
			}

			// Add tags field if provided
			if tt.tags != "" {
				if err := writer.WriteField("tags", tt.tags); err != nil {
					t.Fatalf("Failed to write tags field: %v", err)
				}
			}

			writer.Close()

			// Create request
			req := httptest.NewRequest("POST", "/upload-file", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			if tt.sessionID != "" {
				req.Header.Set("X-Session-ID", tt.sessionID)
			}

			w := httptest.NewRecorder()
			uploadFileHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectFile && w.Code == http.StatusCreated {
				// Parse response
				var response UploadFileResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Verify response structure
				if response.Name == "" {
					t.Error("Response missing name")
				}
				if response.Size <= 0 {
					t.Error("Response missing or invalid size")
				}
				if response.UploadTime.IsZero() {
					t.Error("Response missing upload time")
				}
				if response.Type == "" {
					t.Error("Response missing type")
				}

				// Verify file exists on disk
				sessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
				filePath := filepath.Join(sessionDir, response.Name)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("Expected file %s to exist on disk", filePath)
				}

				// Verify manifest entry
				records := manifest.List()
				found := false
				for _, record := range records {
					if record.SessionID == testSession.ID && record.Filename == response.Name {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected manifest entry for uploaded file")
				}

				// Verify tags if provided
				if tt.tags != "" {
					if response.Tags == nil {
						t.Error("Expected tags in response")
					} else {
						var expectedTags map[string]string
						if err := json.Unmarshal([]byte(tt.tags), &expectedTags); err == nil {
							for key, value := range expectedTags {
								if response.Tags[key] != value {
									t.Errorf("Tag mismatch: expected %s=%s, got %s", key, value, response.Tags[key])
								}
							}
						}
					}
				}
			}
		})
	}
}

func TestUploadFileHandlerValidation(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear()

	// Create test session
	testSession, err := session.Open(time.Hour)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	tests := []struct {
		name           string
		setupRequest   func() (*http.Request, error)
		expectedStatus int
		expectedError  string
	}{
		{
			name: "missing file field",
			setupRequest: func() (*http.Request, error) {
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)
				writer.Close()

				req := httptest.NewRequest("POST", "/upload-file", &buf)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("X-Session-ID", testSession.ID)
				return req, nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "missing or invalid file field",
		},
		{
			name: "invalid multipart form",
			setupRequest: func() (*http.Request, error) {
				req := httptest.NewRequest("POST", "/upload-file", strings.NewReader("invalid form data"))
				req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
				req.Header.Set("X-Session-ID", testSession.ID)
				return req, nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid multipart form",
		},
		{
			name: "too many tags",
			setupRequest: func() (*http.Request, error) {
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)

				fileWriter, _ := writer.CreateFormFile("file", "test.txt")
				fileWriter.Write([]byte("test content"))

				// Create tags with more than 20 entries
				tags := make(map[string]string)
				for i := 0; i < 25; i++ {
					tags[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
				}
				tagsJSON, _ := json.Marshal(tags)
				writer.WriteField("tags", string(tagsJSON))

				writer.Close()

				req := httptest.NewRequest("POST", "/upload-file", &buf)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("X-Session-ID", testSession.ID)
				return req, nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "too many tags",
		},
		{
			name: "tag key too long",
			setupRequest: func() (*http.Request, error) {
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)

				fileWriter, _ := writer.CreateFormFile("file", "test.txt")
				fileWriter.Write([]byte("test content"))

				longKey := strings.Repeat("a", 60) // Longer than 50 chars
				tags := map[string]string{longKey: "value"}
				tagsJSON, _ := json.Marshal(tags)
				writer.WriteField("tags", string(tagsJSON))

				writer.Close()

				req := httptest.NewRequest("POST", "/upload-file", &buf)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("X-Session-ID", testSession.ID)
				return req, nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "tag key too long",
		},
		{
			name: "unsafe tag characters",
			setupRequest: func() (*http.Request, error) {
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)

				fileWriter, _ := writer.CreateFormFile("file", "test.txt")
				fileWriter.Write([]byte("test content"))

				tags := map[string]string{"../malicious": "value"}
				tagsJSON, _ := json.Marshal(tags)
				writer.WriteField("tags", string(tagsJSON))

				writer.Close()

				req := httptest.NewRequest("POST", "/upload-file", &buf)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("X-Session-ID", testSession.ID)
				return req, nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "tag key contains unsafe characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := tt.setupRequest()
			if err != nil {
				t.Fatalf("Failed to setup request: %v", err)
			}

			w := httptest.NewRecorder()
			uploadFileHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedError != "" {
				var errorResp map[string]string
				if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				if !strings.Contains(errorResp["error"], tt.expectedError) {
					t.Errorf("Expected error to contain %q, got %q", tt.expectedError, errorResp["error"])
				}
			}
		})
	}
}

func TestUploadFileHandlerUniqueFilenames(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear()

	// Create test session
	testSession, err := session.Open(time.Hour)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	// Upload same filename twice
	filename := "duplicate.txt"
	content := "test content"

	var responses []UploadFileResponse

	for i := 0; i < 2; i++ {
		// Create multipart form
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		fileWriter, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}
		if _, err := fileWriter.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write file content: %v", err)
		}
		writer.Close()

		// Create request
		req := httptest.NewRequest("POST", "/upload-file", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Session-ID", testSession.ID)

		w := httptest.NewRecorder()
		uploadFileHandler(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("Upload %d failed with status %d", i+1, w.Code)
		}

		var response UploadFileResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response %d: %v", i+1, err)
		}

		responses = append(responses, response)
	}

	// Verify filenames are different
	if responses[0].Name == responses[1].Name {
		t.Errorf("Expected different filenames, both got %s", responses[0].Name)
	}

	// Verify first file keeps original name
	if responses[0].Name != filename {
		t.Errorf("Expected first file to keep original name %s, got %s", filename, responses[0].Name)
	}

	// Verify second file has timestamp suffix
	if !strings.Contains(responses[1].Name, "_") {
		t.Errorf("Expected second file to have timestamp suffix, got %s", responses[1].Name)
	}

	// Verify both files exist on disk
	sessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
	for i, response := range responses {
		filePath := filepath.Join(sessionDir, response.Name)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file %d (%s) to exist on disk", i+1, filePath)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"normal.txt", "normal.txt", false},
		{"file with spaces.pdf", "file with spaces.pdf", false},
		{"../../../etc/passwd", "", true},
		{"file/with/slashes.txt", "", true},
		{"file\\with\\backslashes.txt", "", true},
		{"file<with>bad:chars.txt", "file_with_bad_chars.txt", false},
		{"CON.txt", "", true},             // Reserved Windows name
		{"file.txt.", "file.txt", false},  // Trailing dot removed
		{" file.txt ", "file.txt", false}, // Whitespace trimmed
		{"", "", true},
		{".", "", true},
		{"..", "", true},
		{strings.Repeat("a", 300) + ".txt", strings.Repeat("a", 251) + ".txt", false}, // Long filename truncated
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("sanitize_%s", tt.input), func(t *testing.T) {
			result, err := sanitizeFilename(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input %q, but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestValidateTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     map[string]string
		hasError bool
	}{
		{
			name:     "valid tags",
			tags:     map[string]string{"category": "document", "priority": "high"},
			hasError: false,
		},
		{
			name:     "empty tags",
			tags:     map[string]string{},
			hasError: false,
		},
		{
			name: "too many tags",
			tags: func() map[string]string {
				tags := make(map[string]string)
				for i := 0; i < 25; i++ {
					tags[fmt.Sprintf("key%d", i)] = "value"
				}
				return tags
			}(),
			hasError: true,
		},
		{
			name:     "empty key",
			tags:     map[string]string{"": "value"},
			hasError: true,
		},
		{
			name:     "key too long",
			tags:     map[string]string{strings.Repeat("a", 60): "value"},
			hasError: true,
		},
		{
			name:     "value too long",
			tags:     map[string]string{"key": strings.Repeat("a", 250)},
			hasError: true,
		},
		{
			name:     "unsafe key characters",
			tags:     map[string]string{"../malicious": "value"},
			hasError: true,
		},
		{
			name:     "unsafe value characters",
			tags:     map[string]string{"key": "../malicious"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTags(tt.tags)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for tags %v, but got none", tt.tags)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for tags %v: %v", tt.tags, err)
				}
			}
		})
	}
}
