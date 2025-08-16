package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
	"familyvault/internal/core/upload"
)

func TestSearchFilesHandler(t *testing.T) {
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

	// Create test files with different types and upload times
	baseTime := time.Now().Add(-time.Hour)
	testFiles := []struct {
		filename   string
		content    string
		uploadTime time.Time
		tags       map[string]string
	}{
		{
			filename:   "document1.txt",
			content:    "This is a text document",
			uploadTime: baseTime,
			tags:       map[string]string{"category": "document", "priority": "high"},
		},
		{
			filename:   "image1.jpg",
			content:    "fake image content",
			uploadTime: baseTime.Add(10 * time.Minute),
			tags:       map[string]string{"category": "photo", "event": "vacation"},
		},
		{
			filename:   "report.pdf",
			content:    "fake pdf content",
			uploadTime: baseTime.Add(20 * time.Minute),
			tags:       map[string]string{"category": "document", "type": "report"},
		},
		{
			filename:   "backup.zip",
			content:    "fake zip content",
			uploadTime: baseTime.Add(30 * time.Minute),
			tags:       nil,
		},
	}

	for _, tf := range testFiles {
		// Encrypt and save file
		filePath := filepath.Join(sessionDir, tf.filename)
		if err := upload.EncryptAndSave(newTestFile(tf.content), filePath); err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.filename, err)
		}

		// Add to manifest
		manifest.Add(manifest.FileRecord{
			SessionID:  testSession.ID,
			Filename:   tf.filename,
			UploadedAt: tf.uploadTime,
			Tags:       tf.tags,
		})
	}

	tests := []struct {
		name           string
		sessionID      string
		queryParams    map[string]string
		expectedStatus int
		expectedCount  int
		expectedFiles  []string
	}{
		{
			name:           "no filters - return all files",
			sessionID:      testSession.ID,
			queryParams:    map[string]string{},
			expectedStatus: http.StatusOK,
			expectedCount:  4,
			expectedFiles:  []string{"document1.txt", "image1.jpg", "report.pdf", "backup.zip"},
		},
		{
			name:           "filter by name substring",
			sessionID:      testSession.ID,
			queryParams:    map[string]string{"name": "document"},
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			expectedFiles:  []string{"document1.txt"},
		},
		{
			name:           "filter by file type",
			sessionID:      testSession.ID,
			queryParams:    map[string]string{"type": "txt"},
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			expectedFiles:  []string{"document1.txt"},
		},
		{
			name:      "filter by date range",
			sessionID: testSession.ID,
			queryParams: map[string]string{
				"date_from": baseTime.Add(5 * time.Minute).Format(time.RFC3339),
				"date_to":   baseTime.Add(25 * time.Minute).Format(time.RFC3339),
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			expectedFiles:  []string{"image1.jpg", "report.pdf"},
		},
		{
			name:           "filter by tags",
			sessionID:      testSession.ID,
			queryParams:    map[string]string{"tags": "document"},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			expectedFiles:  []string{"document1.txt", "report.pdf"},
		},
		{
			name:      "multiple filters",
			sessionID: testSession.ID,
			queryParams: map[string]string{
				"type": "txt",
				"tags": "high",
			},
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			expectedFiles:  []string{"document1.txt"},
		},
		{
			name:           "no matches",
			sessionID:      testSession.ID,
			queryParams:    map[string]string{"name": "nonexistent"},
			expectedStatus: http.StatusNotFound,
			expectedCount:  0,
			expectedFiles:  []string{},
		},
		{
			name:           "invalid session",
			sessionID:      "invalid-session-id",
			queryParams:    map[string]string{},
			expectedStatus: http.StatusUnauthorized,
			expectedCount:  0,
			expectedFiles:  []string{},
		},
		{
			name:           "missing session",
			sessionID:      "",
			queryParams:    map[string]string{},
			expectedStatus: http.StatusUnauthorized,
			expectedCount:  0,
			expectedFiles:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build URL with query parameters
			u, _ := url.Parse("/search-files")
			q := u.Query()
			for key, value := range tt.queryParams {
				q.Set(key, value)
			}
			u.RawQuery = q.Encode()

			req := httptest.NewRequest("GET", u.String(), nil)
			if tt.sessionID != "" {
				req.Header.Set("X-Session-ID", tt.sessionID)
			}

			w := httptest.NewRecorder()
			searchFilesHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				// Parse response
				var results []FileSearchResult
				if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if len(results) != tt.expectedCount {
					t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
				}

				// Check that expected files are present
				foundFiles := make(map[string]bool)
				for _, result := range results {
					foundFiles[result.Name] = true
				}

				for _, expectedFile := range tt.expectedFiles {
					if !foundFiles[expectedFile] {
						t.Errorf("Expected file %s not found in results", expectedFile)
					}
				}
			}
		})
	}
}

func TestSearchFilesHandlerValidation(t *testing.T) {
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
		queryParams    map[string]string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid name with path traversal",
			queryParams:    map[string]string{"name": "../etc/passwd"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid name parameter",
		},
		{
			name:           "invalid type with path traversal",
			queryParams:    map[string]string{"type": "../txt"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid type parameter",
		},
		{
			name:           "invalid date_from format",
			queryParams:    map[string]string{"date_from": "2023-01-01"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid date_from parameter",
		},
		{
			name:           "invalid date_to format",
			queryParams:    map[string]string{"date_to": "invalid-date"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid date_to parameter",
		},
		{
			name: "invalid date range",
			queryParams: map[string]string{
				"date_from": "2023-01-02T00:00:00Z",
				"date_to":   "2023-01-01T00:00:00Z",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid date range",
		},
		{
			name:           "invalid tags with path traversal",
			queryParams:    map[string]string{"tags": "../admin,normal"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid tags parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build URL with query parameters
			u, _ := url.Parse("/search-files")
			q := u.Query()
			for key, value := range tt.queryParams {
				q.Set(key, value)
			}
			u.RawQuery = q.Encode()

			req := httptest.NewRequest("GET", u.String(), nil)
			req.Header.Set("X-Session-ID", testSession.ID)

			w := httptest.NewRecorder()
			searchFilesHandler(w, req)

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

func TestSearchFilesHandlerFallbackToDirectory(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear() // Clear manifest to test directory fallback

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

	// Create test files directly on filesystem (no manifest entries)
	testFiles := []string{"test1.txt", "test2.jpg", "test3.pdf"}
	for _, filename := range testFiles {
		filePath := filepath.Join(sessionDir, filename)
		if err := upload.EncryptAndSave(newTestFile("test content"), filePath); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	req := httptest.NewRequest("GET", "/search-files", nil)
	req.Header.Set("X-Session-ID", testSession.ID)

	w := httptest.NewRecorder()
	searchFilesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Parse response
	var results []FileSearchResult
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(results) != len(testFiles) {
		t.Errorf("Expected %d results, got %d", len(testFiles), len(results))
	}

	// Verify file types are correctly extracted
	expectedTypes := map[string]string{
		"test1.txt": "txt",
		"test2.jpg": "jpg",
		"test3.pdf": "pdf",
	}

	for _, result := range results {
		expectedType, exists := expectedTypes[result.Name]
		if !exists {
			t.Errorf("Unexpected file in results: %s", result.Name)
			continue
		}

		if result.Type != expectedType {
			t.Errorf("File %s: expected type %s, got %s", result.Name, expectedType, result.Type)
		}

		if result.Tags != nil {
			t.Errorf("File %s: expected no tags from filesystem, got %v", result.Name, result.Tags)
		}
	}
}
