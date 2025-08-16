package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"familyvault/internal/core/download"
	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
	"familyvault/internal/core/upload"
)

func TestDuplicateSessionHandler(t *testing.T) {
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

	// Create session directory and test files
	sessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create test files with different content
	testFiles := map[string]string{
		"document.txt": "This is a test document",
		"image.jpg":    "This is fake JPEG content",
		"data.csv":     "name,age,city\nJohn,30,NYC\nJane,25,LA",
	}

	for filename, content := range testFiles {
		// Encrypt and save file
		filePath := filepath.Join(sessionDir, filename)
		if err := upload.EncryptAndSave(newTestFileForDuplicate(content), filePath); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}

		// Add to manifest with metadata
		manifest.Add(manifest.FileRecord{
			SessionID:  testSession.ID,
			Filename:   filename,
			UploadedAt: time.Now(),
			Tags: map[string]string{
				"category": "test",
				"type":     strings.Split(filename, ".")[1],
			},
		})
	}

	// Add session metadata
	sessionMetadata := map[string]interface{}{
		"project_name": "Test Project",
		"created_by":   "test_user",
		"description":  "Test session for duplication",
	}
	manifest.UpdateSessionMetadata(testSession.ID, sessionMetadata)

	tests := []struct {
		name           string
		sessionID      string
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name:           "successful duplication",
			sessionID:      testSession.ID,
			expectedStatus: http.StatusCreated,
			expectSuccess:  true,
		},
		{
			name:           "nonexistent session",
			sessionID:      "nonexistent-session-id",
			expectedStatus: http.StatusNotFound,
			expectSuccess:  false,
		},
		{
			name:           "empty session ID",
			sessionID:      "",
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/sessions/" + tt.sessionID + "/duplicate"
			req := httptest.NewRequest("POST", url, nil)
			w := httptest.NewRecorder()

			duplicateSessionHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectSuccess && w.Code == http.StatusCreated {
				// Parse response
				var response DuplicateSessionResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if !response.Success {
					t.Error("Expected success to be true")
				}

				if response.NewSessionID == "" {
					t.Error("Expected new session ID to be set")
				}

				if response.SourceSessionID != testSession.ID {
					t.Errorf("Expected source session ID %s, got %s", testSession.ID, response.SourceSessionID)
				}

				if response.FilesCount != len(testFiles) {
					t.Errorf("Expected files count %d, got %d", len(testFiles), response.FilesCount)
				}

				if response.CreatedAt.IsZero() {
					t.Error("Expected CreatedAt to be set")
				}

				// Verify files were copied
				newSessionDir := filepath.Join(tempDir, "uploads", response.NewSessionID)
				for filename, expectedContent := range testFiles {
					filePath := filepath.Join(newSessionDir, filename)
					if _, err := os.Stat(filePath); os.IsNotExist(err) {
						t.Errorf("Expected file %s to exist in new session", filename)
						continue
					}

					// Verify file content (decrypt and compare)
					decryptedContent, err := decryptAndRead(filePath)
					if err != nil {
						t.Errorf("Failed to decrypt file %s: %v", filename, err)
						continue
					}

					if string(decryptedContent) != expectedContent {
						t.Errorf("File %s content mismatch. Expected %q, got %q", filename, expectedContent, string(decryptedContent))
					}
				}

				// Verify manifest entries were duplicated
				allRecords := manifest.List()
				newSessionRecords := 0
				for _, record := range allRecords {
					if record.SessionID == response.NewSessionID {
						newSessionRecords++
						// Verify tags were copied
						if record.Tags["category"] != "test" {
							t.Errorf("Expected tag category=test, got %s", record.Tags["category"])
						}
					}
				}

				if newSessionRecords != len(testFiles) {
					t.Errorf("Expected %d manifest records for new session, got %d", len(testFiles), newSessionRecords)
				}

				// Verify session metadata was duplicated
				newSessionMeta, exists := manifest.GetSessionMetadata(response.NewSessionID)
				if !exists {
					t.Error("Expected session metadata to exist for new session")
				} else {
					if newSessionMeta.Metadata["project_name"] != "Test Project" {
						t.Errorf("Expected project_name=Test Project, got %v", newSessionMeta.Metadata["project_name"])
					}
					if newSessionMeta.Metadata["created_by"] != "test_user" {
						t.Errorf("Expected created_by=test_user, got %v", newSessionMeta.Metadata["created_by"])
					}
				}
			}
		})
	}
}

func TestDuplicateSessionHandlerInvalidURL(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "invalid URL format - missing duplicate",
			url:            "/sessions/test-session",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid URL format - extra path",
			url:            "/sessions/test-session/duplicate/extra",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "wrong HTTP method",
			url:            "/sessions/test-session/duplicate",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := "POST"
			if tt.name == "wrong HTTP method" {
				method = "GET"
			}

			req := httptest.NewRequest(method, tt.url, nil)
			w := httptest.NewRecorder()

			duplicateSessionHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestDuplicateSessionHandlerDriveNotAvailable(t *testing.T) {
	// Setup test environment with non-existent drive path
	drive.SetDrivePath("/nonexistent/path")

	req := httptest.NewRequest("POST", "/sessions/test-session/duplicate", nil)
	w := httptest.NewRecorder()

	duplicateSessionHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCopySessionFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory with test files
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"file1.txt":            "Content of file 1",
		"subdir/file2.txt":     "Content of file 2",
		"subdir/file3.csv":     "name,value\ntest,123",
		"another/deep/file.md": "# Markdown content",
	}

	for filePath, content := range testFiles {
		fullPath := filepath.Join(sourceDir, filePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", filePath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	// Create destination directory
	destDir := filepath.Join(tempDir, "dest")

	// Test copying files
	filesCount, err := copySessionFiles(sourceDir, destDir)
	if err != nil {
		t.Fatalf("copySessionFiles failed: %v", err)
	}

	if filesCount != len(testFiles) {
		t.Errorf("Expected files count %d, got %d", len(testFiles), filesCount)
	}

	// Verify all files were copied correctly
	for filePath, expectedContent := range testFiles {
		destFilePath := filepath.Join(destDir, filePath)
		if _, err := os.Stat(destFilePath); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist in destination", filePath)
			continue
		}

		actualContent, err := os.ReadFile(destFilePath)
		if err != nil {
			t.Errorf("Failed to read copied file %s: %v", filePath, err)
			continue
		}

		if string(actualContent) != expectedContent {
			t.Errorf("File %s content mismatch. Expected %q, got %q", filePath, expectedContent, string(actualContent))
		}
	}
}

func TestCopyFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file
	sourceFile := filepath.Join(tempDir, "source.txt")
	content := "Test file content"
	if err := os.WriteFile(sourceFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Test copying file
	destFile := filepath.Join(tempDir, "dest.txt")
	if err := copyFile(sourceFile, destFile); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify file was copied correctly
	actualContent, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}

	if string(actualContent) != content {
		t.Errorf("File content mismatch. Expected %q, got %q", content, string(actualContent))
	}

	// Verify file permissions were copied
	sourceInfo, err := os.Stat(sourceFile)
	if err != nil {
		t.Fatalf("Failed to get source file info: %v", err)
	}

	destInfo, err := os.Stat(destFile)
	if err != nil {
		t.Fatalf("Failed to get dest file info: %v", err)
	}

	if sourceInfo.Mode() != destInfo.Mode() {
		t.Errorf("File permissions mismatch. Expected %v, got %v", sourceInfo.Mode(), destInfo.Mode())
	}
}

func TestDuplicateManifestEntries(t *testing.T) {
	// Clear manifest for clean test
	manifest.Clear()

	sourceSessionID := "source-session-123"
	newSessionID := "new-session-456"

	// Add test records to manifest
	testRecords := []manifest.FileRecord{
		{
			SessionID:  sourceSessionID,
			Filename:   "file1.txt",
			UploadedAt: time.Now().Add(-time.Hour),
			Tags: map[string]string{
				"category": "documents",
				"type":     "text",
			},
		},
		{
			SessionID:  sourceSessionID,
			Filename:   "file2.jpg",
			UploadedAt: time.Now().Add(-30 * time.Minute),
			Tags: map[string]string{
				"category": "images",
				"type":     "jpeg",
			},
		},
		{
			SessionID:  "other-session",
			Filename:   "other-file.txt",
			UploadedAt: time.Now(),
			Tags:       map[string]string{"category": "other"},
		},
	}

	for _, record := range testRecords {
		manifest.Add(record)
	}

	// Add session metadata
	sessionMetadata := map[string]interface{}{
		"project": "Test Project",
		"owner":   "test_user",
	}
	manifest.UpdateSessionMetadata(sourceSessionID, sessionMetadata)

	// Test duplicating manifest entries
	err := duplicateManifestEntries(sourceSessionID, newSessionID)
	if err != nil {
		t.Fatalf("duplicateManifestEntries failed: %v", err)
	}

	// Verify records were duplicated
	allRecords := manifest.List()
	newSessionRecords := 0
	sourceSessionRecords := 0

	for _, record := range allRecords {
		if record.SessionID == newSessionID {
			newSessionRecords++
			// Verify tags were copied
			if record.Filename == "file1.txt" {
				if record.Tags["category"] != "documents" {
					t.Errorf("Expected category=documents, got %s", record.Tags["category"])
				}
				if record.Tags["type"] != "text" {
					t.Errorf("Expected type=text, got %s", record.Tags["type"])
				}
			}
		} else if record.SessionID == sourceSessionID {
			sourceSessionRecords++
		}
	}

	// Should have 2 records for new session (excluding the "other-session" record)
	if newSessionRecords != 2 {
		t.Errorf("Expected 2 records for new session, got %d", newSessionRecords)
	}

	// Original records should still exist
	if sourceSessionRecords != 2 {
		t.Errorf("Expected 2 records for source session, got %d", sourceSessionRecords)
	}

	// Verify session metadata was duplicated
	newSessionMeta, exists := manifest.GetSessionMetadata(newSessionID)
	if !exists {
		t.Error("Expected session metadata to exist for new session")
	} else {
		if newSessionMeta.Metadata["project"] != "Test Project" {
			t.Errorf("Expected project=Test Project, got %v", newSessionMeta.Metadata["project"])
		}
		if newSessionMeta.Metadata["owner"] != "test_user" {
			t.Errorf("Expected owner=test_user, got %v", newSessionMeta.Metadata["owner"])
		}
	}

	// Verify original session metadata still exists
	originalMeta, exists := manifest.GetSessionMetadata(sourceSessionID)
	if !exists {
		t.Error("Expected original session metadata to still exist")
	} else {
		if originalMeta.Metadata["project"] != "Test Project" {
			t.Errorf("Expected original project=Test Project, got %v", originalMeta.Metadata["project"])
		}
	}
}

// duplicateTestFile implements multipart.File for testing
type duplicateTestFile struct {
	*strings.Reader
}

func (tf *duplicateTestFile) Close() error                                  { return nil }
func (tf *duplicateTestFile) ReadAt(p []byte, off int64) (n int, err error) { return 0, nil }
func (tf *duplicateTestFile) Seek(offset int64, whence int) (int64, error)  { return 0, nil }

// Helper function to create a test file reader
func newTestFileForDuplicate(content string) multipart.File {
	return &duplicateTestFile{strings.NewReader(content)}
}

// decryptAndRead decrypts a file and returns its content
func decryptAndRead(filePath string) ([]byte, error) {
	var buf bytes.Buffer
	err := download.DecryptAndStream(filePath, &buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
