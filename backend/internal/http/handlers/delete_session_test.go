package handlers

import (
	"encoding/json"
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
	"familyvault/internal/core/upload"
)

func TestDeleteSessionHandler(t *testing.T) {
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

	// Create test files
	testFiles := map[string]string{
		"document.txt": "This is a test document",
		"image.jpg":    "This is fake JPEG content",
		"data.csv":     "name,age,city\nJohn,30,NYC\nJane,25,LA",
	}

	for filename, content := range testFiles {
		// Create file on disk
		filePath := filepath.Join(sessionDir, filename)
		if err := upload.EncryptAndSave(newTestFileForDelete(content), filePath); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}

		// Add to manifest
		manifest.Add(manifest.FileRecord{
			SessionID:  testSession.ID,
			Filename:   filename,
			UploadedAt: time.Now(),
			Tags: map[string]string{
				"category": "test",
				"type":     "document",
			},
		})
	}

	// Add session metadata
	sessionMetadata := map[string]interface{}{
		"project_name": "Test Project",
		"created_by":   "test_user",
	}
	manifest.UpdateSessionMetadata(testSession.ID, sessionMetadata)

	tests := []struct {
		name           string
		sessionID      string
		authSessionID  string
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name:           "successful deletion",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:           "nonexistent session",
			sessionID:      "nonexistent-session-id",
			authSessionID:  "nonexistent-session-id",
			expectedStatus: http.StatusNotFound,
			expectSuccess:  false,
		},
		{
			name:           "empty session ID",
			sessionID:      "",
			authSessionID:  testSession.ID,
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:           "invalid session ID format",
			sessionID:      "sessions",
			authSessionID:  testSession.ID,
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/sessions/" + tt.sessionID
			req := httptest.NewRequest("DELETE", url, nil)
			if tt.authSessionID != "" {
				req.Header.Set("X-Session-ID", tt.authSessionID)
			}
			w := httptest.NewRecorder()

			deleteSessionHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectSuccess && w.Code == http.StatusOK {
				// Parse response
				var response DeleteSessionResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if !response.Success {
					t.Error("Expected success to be true")
				}

				if response.DeletedSessionID != testSession.ID {
					t.Errorf("Expected deleted session ID %s, got %s", testSession.ID, response.DeletedSessionID)
				}

				if response.FilesRemoved != len(testFiles) {
					t.Errorf("Expected files removed %d, got %d", len(testFiles), response.FilesRemoved)
				}

				if !response.ManifestRemoved {
					t.Error("Expected manifest to be removed")
				}

				// Verify files were actually deleted from disk
				if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
					t.Error("Expected session directory to be deleted")
				}

				// Verify manifest entries were removed
				allRecords := manifest.List()
				for _, record := range allRecords {
					if record.SessionID == testSession.ID {
						t.Errorf("Found remaining manifest record for deleted session: %+v", record)
					}
				}

				// Verify session metadata was removed
				if _, exists := manifest.GetSessionMetadata(testSession.ID); exists {
					t.Error("Expected session metadata to be removed")
				}
			}
		})
	}
}

func TestDeleteSessionHandlerDriveNotAvailable(t *testing.T) {
	// Setup test environment with non-existent drive path
	drive.SetDrivePath("/nonexistent/path")

	req := httptest.NewRequest("DELETE", "/sessions/test-session", nil)
	req.Header.Set("X-Session-ID", "test-session")
	w := httptest.NewRecorder()

	deleteSessionHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestDeleteSessionHandlerWrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/sessions/test-session", nil)
	w := httptest.NewRecorder()

	deleteSessionHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestSessionExistsInSystem(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	manifest.Clear()

	testSessionID := "test-session-123"

	// Test 1: Session doesn't exist
	if sessionExistsInSystem(testSessionID) {
		t.Error("Expected session to not exist initially")
	}

	// Test 2: Session exists in manifest
	manifest.Add(manifest.FileRecord{
		SessionID:  testSessionID,
		Filename:   "test.txt",
		UploadedAt: time.Now(),
		Tags:       map[string]string{},
	})

	if !sessionExistsInSystem(testSessionID) {
		t.Error("Expected session to exist in manifest")
	}

	// Clear manifest for next test
	manifest.Clear()

	// Test 3: Session exists in metadata
	manifest.UpdateSessionMetadata(testSessionID, map[string]interface{}{
		"test": "value",
	})

	if !sessionExistsInSystem(testSessionID) {
		t.Error("Expected session to exist in metadata")
	}

	// Clear metadata for next test
	manifest.ClearSessionMetadata(testSessionID)

	// Test 4: Session exists on disk
	sessionDir := filepath.Join(tempDir, "uploads", testSessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	if !sessionExistsInSystem(testSessionID) {
		t.Error("Expected session to exist on disk")
	}
}

func TestDeleteSessionData(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	testSessionID := "test-session-456"
	sessionDir := filepath.Join(tempDir, "uploads", testSessionID)

	// Create session directory with test files
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create test files
	testFiles := []string{"file1.txt", "file2.jpg", "subdir/file3.csv"}
	for _, filename := range testFiles {
		filePath := filepath.Join(sessionDir, filename)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", filename, err)
		}
		if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Test successful deletion
	filesRemoved, backupPath, err := deleteSessionData(testSessionID)
	if err != nil {
		t.Fatalf("deleteSessionData failed: %v", err)
	}

	if filesRemoved != len(testFiles) {
		t.Errorf("Expected %d files removed, got %d", len(testFiles), filesRemoved)
	}

	// Verify backup path was returned
	if backupPath == "" {
		t.Error("Expected backup path to be returned")
	}

	// Verify original directory was moved to backup
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("Expected original session directory to be moved")
	}

	// Verify backup directory exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("Expected backup directory to exist")
	}

	// Test deletion of non-existent session (should not error)
	filesRemoved, _, err = deleteSessionData("nonexistent-session")
	if err != nil {
		t.Errorf("Expected no error for non-existent session, got: %v", err)
	}

	if filesRemoved != 0 {
		t.Errorf("Expected 0 files removed for non-existent session, got %d", filesRemoved)
	}
}

func TestDeleteSessionDataPathTraversalProtection(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Test path traversal attack
	maliciousSessionID := "../../../etc"
	_, _, err := deleteSessionData(maliciousSessionID)
	if err == nil {
		t.Error("Expected error for path traversal attempt")
	}

	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

func TestRemoveSessionFromManifest(t *testing.T) {
	manifest.Clear()

	testSessionID := "test-session-789"
	otherSessionID := "other-session-123"

	// Add test records
	testRecords := []manifest.FileRecord{
		{
			SessionID:  testSessionID,
			Filename:   "file1.txt",
			UploadedAt: time.Now(),
			Tags:       map[string]string{"category": "test"},
		},
		{
			SessionID:  testSessionID,
			Filename:   "file2.jpg",
			UploadedAt: time.Now(),
			Tags:       map[string]string{"category": "test"},
		},
		{
			SessionID:  otherSessionID,
			Filename:   "other-file.txt",
			UploadedAt: time.Now(),
			Tags:       map[string]string{"category": "other"},
		},
	}

	for _, record := range testRecords {
		manifest.Add(record)
	}

	// Add session metadata
	manifest.UpdateSessionMetadata(testSessionID, map[string]interface{}{
		"project": "Test Project",
	})
	manifest.UpdateSessionMetadata(otherSessionID, map[string]interface{}{
		"project": "Other Project",
	})

	// Test removing session from manifest
	removed := removeSessionFromManifest(testSessionID)
	if !removed {
		t.Error("Expected manifest removal to return true")
	}

	// Verify test session records were removed
	allRecords := manifest.List()
	testSessionRecords := 0
	otherSessionRecords := 0

	for _, record := range allRecords {
		if record.SessionID == testSessionID {
			testSessionRecords++
		} else if record.SessionID == otherSessionID {
			otherSessionRecords++
		}
	}

	if testSessionRecords != 0 {
		t.Errorf("Expected 0 records for test session, got %d", testSessionRecords)
	}

	if otherSessionRecords != 1 {
		t.Errorf("Expected 1 record for other session, got %d", otherSessionRecords)
	}

	// Verify test session metadata was removed
	if _, exists := manifest.GetSessionMetadata(testSessionID); exists {
		t.Error("Expected test session metadata to be removed")
	}

	// Verify other session metadata still exists
	if _, exists := manifest.GetSessionMetadata(otherSessionID); !exists {
		t.Error("Expected other session metadata to still exist")
	}

	// Test removing non-existent session
	removed = removeSessionFromManifest("nonexistent-session")
	if removed {
		t.Error("Expected manifest removal to return false for non-existent session")
	}
}

func TestDeleteSessionHandlerIdempotent(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	manifest.Clear()

	testSessionID := "test-session-idempotent"

	// First deletion attempt on non-existent session should return 404
	req := httptest.NewRequest("DELETE", "/sessions/"+testSessionID, nil)
	req.Header.Set("X-Session-ID", testSessionID)
	w := httptest.NewRecorder()

	deleteSessionHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d for non-existent session, got %d", http.StatusNotFound, w.Code)
	}

	// Create session and delete it
	sessionDir := filepath.Join(tempDir, "uploads", testSessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Add to manifest
	manifest.Add(manifest.FileRecord{
		SessionID:  testSessionID,
		Filename:   "test.txt",
		UploadedAt: time.Now(),
		Tags:       map[string]string{},
	})

	// First deletion should succeed
	req = httptest.NewRequest("DELETE", "/sessions/"+testSessionID, nil)
	req.Header.Set("X-Session-ID", testSessionID)
	w = httptest.NewRecorder()

	deleteSessionHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for first deletion, got %d", http.StatusOK, w.Code)
	}

	// Second deletion attempt should return 404 (idempotent)
	req = httptest.NewRequest("DELETE", "/sessions/"+testSessionID, nil)
	req.Header.Set("X-Session-ID", testSessionID)
	w = httptest.NewRecorder()

	deleteSessionHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d for second deletion (idempotent), got %d", http.StatusNotFound, w.Code)
	}
}

// Helper function to create a test file reader for deletion tests
func newTestFileForDelete(content string) *testFileForDelete {
	return &testFileForDelete{strings.NewReader(content)}
}

type testFileForDelete struct {
	*strings.Reader
}

func (tf *testFileForDelete) Close() error                                  { return nil }
func (tf *testFileForDelete) ReadAt(p []byte, off int64) (n int, err error) { return 0, nil }
func (tf *testFileForDelete) Seek(offset int64, whence int) (int64, error)  { return 0, nil }
