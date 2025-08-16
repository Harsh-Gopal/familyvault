package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
	"familyvault/internal/core/upload"
)

func TestDownloadHandler(t *testing.T) {
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

	// Create session directory
	sessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create and upload test files
	testFiles := map[string]string{
		"document.txt": "This is a text document content",
		"image.jpg":    "This is fake JPEG content",
		"report.pdf":   "This is fake PDF content",
	}

	for filename, content := range testFiles {
		// Encrypt and save file
		filePath := filepath.Join(sessionDir, filename)
		if err := upload.EncryptAndSave(newTestFile(content), filePath); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}

		// Add to manifest
		manifest.Add(manifest.FileRecord{
			SessionID:  testSession.ID,
			Filename:   filename,
			UploadedAt: time.Now(),
		})
	}

	tests := []struct {
		name           string
		sessionID      string
		filename       string
		expectedStatus int
		expectedType   string
		expectContent  bool
	}{
		{
			name:           "download text file",
			sessionID:      testSession.ID,
			filename:       "document.txt",
			expectedStatus: http.StatusOK,
			expectedType:   "text/plain; charset=utf-8",
			expectContent:  true,
		},
		{
			name:           "download image file",
			sessionID:      testSession.ID,
			filename:       "image.jpg",
			expectedStatus: http.StatusOK,
			expectedType:   "image/jpeg",
			expectContent:  true,
		},
		{
			name:           "download PDF file",
			sessionID:      testSession.ID,
			filename:       "report.pdf",
			expectedStatus: http.StatusOK,
			expectedType:   "application/pdf",
			expectContent:  true,
		},
		{
			name:           "invalid session",
			sessionID:      "invalid-session-id",
			filename:       "document.txt",
			expectedStatus: http.StatusUnauthorized,
			expectedType:   "",
			expectContent:  false,
		},
		{
			name:           "missing session",
			sessionID:      "",
			filename:       "document.txt",
			expectedStatus: http.StatusUnauthorized,
			expectedType:   "",
			expectContent:  false,
		},
		{
			name:           "missing filename",
			sessionID:      testSession.ID,
			filename:       "",
			expectedStatus: http.StatusBadRequest,
			expectedType:   "",
			expectContent:  false,
		},
		{
			name:           "file not in manifest",
			sessionID:      testSession.ID,
			filename:       "nonexistent.txt",
			expectedStatus: http.StatusNotFound,
			expectedType:   "",
			expectContent:  false,
		},
		{
			name:           "unsafe filename with path traversal",
			sessionID:      testSession.ID,
			filename:       "../../../etc/passwd",
			expectedStatus: http.StatusBadRequest,
			expectedType:   "",
			expectContent:  false,
		},
		{
			name:           "unsafe filename with backslashes",
			sessionID:      testSession.ID,
			filename:       "..\\..\\windows\\system32\\config",
			expectedStatus: http.StatusBadRequest,
			expectedType:   "",
			expectContent:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build URL with filename parameter
			url := "/download"
			if tt.filename != "" {
				url += "?filename=" + tt.filename
			}

			req := httptest.NewRequest("GET", url, nil)
			if tt.sessionID != "" {
				req.Header.Set("X-Session-ID", tt.sessionID)
			}

			w := httptest.NewRecorder()
			downloadHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectContent && w.Code == http.StatusOK {
				// Verify Content-Type header
				if tt.expectedType != "" {
					contentType := w.Header().Get("Content-Type")
					if contentType != tt.expectedType {
						t.Errorf("Expected Content-Type %s, got %s", tt.expectedType, contentType)
					}
				}

				// Verify Content-Disposition header
				expectedDisposition := `attachment; filename="` + tt.filename + `"`
				contentDisposition := w.Header().Get("Content-Disposition")
				if contentDisposition != expectedDisposition {
					t.Errorf("Expected Content-Disposition %s, got %s", expectedDisposition, contentDisposition)
				}

				// Verify content is decrypted correctly
				expectedContent := testFiles[tt.filename]
				actualContent := w.Body.String()
				if actualContent != expectedContent {
					t.Errorf("Content mismatch for %s. Expected %q, got %q", tt.filename, expectedContent, actualContent)
				}

				// Verify Content-Length header
				contentLength := w.Header().Get("Content-Length")
				if contentLength == "" {
					t.Error("Expected Content-Length header to be set")
				}
			}
		})
	}
}

func TestDownloadHandlerWithQueryParamAuth(t *testing.T) {
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

	// Create session directory and test file
	sessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	filename := "test.txt"
	content := "Test content for query param auth"
	filePath := filepath.Join(sessionDir, filename)
	if err := upload.EncryptAndSave(newTestFile(content), filePath); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Add to manifest
	manifest.Add(manifest.FileRecord{
		SessionID:  testSession.ID,
		Filename:   filename,
		UploadedAt: time.Now(),
	})

	// Test with session_id query parameter
	url := "/download?filename=" + filename + "&session_id=" + testSession.ID
	req := httptest.NewRequest("GET", url, nil)

	w := httptest.NewRecorder()
	downloadHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != content {
		t.Errorf("Content mismatch. Expected %q, got %q", content, w.Body.String())
	}
}

func TestDownloadHandlerFileNotOnDisk(t *testing.T) {
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

	// Add file to manifest but don't create it on disk
	filename := "missing.txt"
	manifest.Add(manifest.FileRecord{
		SessionID:  testSession.ID,
		Filename:   filename,
		UploadedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", "/download?filename="+filename, nil)
	req.Header.Set("X-Session-ID", testSession.ID)

	w := httptest.NewRecorder()
	downloadHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestSanitizeDownloadFilename(t *testing.T) {
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
		{"", "", true},
		{".", "", true},
		{"..", "", true},
		{"file\x00with\x1fnull.txt", "", true}, // Control characters
		{"valid-file_name.123.txt", "valid-file_name.123.txt", false},
	}

	for _, tt := range tests {
		t.Run("sanitize_"+tt.input, func(t *testing.T) {
			result, err := sanitizeDownloadFilename(tt.input)

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

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"document.txt", "text/plain; charset=utf-8"},
		{"image.jpg", "image/jpeg"},
		{"image.jpeg", "image/jpeg"},
		{"image.png", "image/png"},
		{"image.gif", "image/gif"},
		{"document.pdf", "application/pdf"},
		{"archive.zip", "application/zip"},
		{"data.json", "application/json"},
		{"data.xml", "application/xml"},
		{"document.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"spreadsheet.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"presentation.pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
		{"document.odt", "application/vnd.oasis.opendocument.text"},
		{"unknown.unknownext", "application/octet-stream"},
		{"noextension", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := detectContentType(tt.filename)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
