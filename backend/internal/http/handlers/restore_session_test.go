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

func TestRestoreSessionHandler(t *testing.T) {
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
		if err := upload.EncryptAndSave(newTestFileForRestore(content), filePath); err != nil {
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

	// Delete the session to create a backup
	req := httptest.NewRequest("DELETE", "/sessions/"+testSession.ID, nil)
	req.Header.Set("X-Session-ID", testSession.ID)
	w := httptest.NewRecorder()
	deleteSessionHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to delete session for backup: %d", w.Code)
	}

	// Verify session was deleted
	if sessionExistsInSystem(testSession.ID) {
		t.Fatal("Session should be deleted")
	}

	tests := []struct {
		name           string
		sessionID      string
		authSessionID  string
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name:           "successful restoration",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:           "nonexistent backup",
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
			name:           "invalid URL format",
			sessionID:      "invalid",
			authSessionID:  testSession.ID,
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var url string
			if tt.name == "invalid URL format" {
				url = "/sessions/" + tt.sessionID + "/invalid"
			} else {
				url = "/sessions/" + tt.sessionID + "/restore"
			}

			req := httptest.NewRequest("POST", url, nil)
			if tt.authSessionID != "" {
				req.Header.Set("X-Session-ID", tt.authSessionID)
			}
			w := httptest.NewRecorder()

			restoreSessionHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectSuccess && w.Code == http.StatusOK {
				// Parse response
				var response RestoreSessionResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if !response.Success {
					t.Error("Expected success to be true")
				}

				if response.RestoredSessionID != testSession.ID {
					t.Errorf("Expected restored session ID %s, got %s", testSession.ID, response.RestoredSessionID)
				}

				if response.FilesRestored != len(testFiles) {
					t.Errorf("Expected files restored %d, got %d", len(testFiles), response.FilesRestored)
				}

				if !response.ManifestRestored {
					t.Error("Expected manifest to be restored")
				}

				// Verify files were actually restored to disk
				restoredSessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
				if _, err := os.Stat(restoredSessionDir); os.IsNotExist(err) {
					t.Error("Expected session directory to be restored")
				}

				// Verify manifest entries were restored
				allRecords := manifest.List()
				restoredRecords := 0
				for _, record := range allRecords {
					if record.SessionID == testSession.ID {
						restoredRecords++
					}
				}

				if restoredRecords != len(testFiles) {
					t.Errorf("Expected %d restored manifest records, got %d", len(testFiles), restoredRecords)
				}

				// Verify session metadata was restored
				if restoredMeta, exists := manifest.GetSessionMetadata(testSession.ID); !exists {
					t.Error("Expected session metadata to be restored")
				} else {
					if restoredMeta.Metadata["project_name"] != "Test Project" {
						t.Errorf("Expected project_name=Test Project, got %v", restoredMeta.Metadata["project_name"])
					}
				}
			}
		})
	}
}

func TestRestoreSessionHandlerDriveNotAvailable(t *testing.T) {
	// Setup test environment with non-existent drive path
	drive.SetDrivePath("/nonexistent/path")

	req := httptest.NewRequest("POST", "/sessions/test-session/restore", nil)
	req.Header.Set("X-Session-ID", "test-session")
	w := httptest.NewRecorder()

	restoreSessionHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRestoreSessionHandlerWrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/sessions/test-session/restore", nil)
	w := httptest.NewRecorder()

	restoreSessionHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestRestoreSessionHandlerAlreadyExists(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	manifest.Clear()

	testSessionID := "existing-session"

	// Create existing session
	sessionDir := filepath.Join(tempDir, "uploads", testSessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Add to manifest to make it exist in system
	manifest.Add(manifest.FileRecord{
		SessionID:  testSessionID,
		Filename:   "test.txt",
		UploadedAt: time.Now(),
		Tags:       map[string]string{},
	})

	req := httptest.NewRequest("POST", "/sessions/"+testSessionID+"/restore", nil)
	req.Header.Set("X-Session-ID", testSessionID)
	w := httptest.NewRecorder()

	restoreSessionHandler(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestFindSessionBackup(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	testSessionID := "test-session-backup"
	uploadsDir := filepath.Join(tempDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		t.Fatalf("Failed to create uploads directory: %v", err)
	}

	// Test 1: No backup exists
	_, err := findSessionBackup(testSessionID)
	if err == nil {
		t.Error("Expected error for non-existent backup")
	}

	// Test 2: Create backup directory
	backupDir := filepath.Join(uploadsDir, testSessionID+".deleted.1234567890")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup directory: %v", err)
	}

	// Create backup metadata
	backupMetadata := &SessionBackupMetadata{
		SessionID: testSessionID,
		FileRecords: []manifest.FileRecord{
			{
				SessionID:  testSessionID,
				Filename:   "test.txt",
				UploadedAt: time.Now(),
				Tags:       map[string]string{"category": "test"},
			},
		},
		SessionMetadata: map[string]interface{}{
			"project": "Test Project",
		},
		DeletedAt:  time.Now(),
		BackupPath: backupDir,
	}

	// Save metadata
	metadataPath := filepath.Join(backupDir, ".backup_metadata.json")
	data, _ := json.Marshal(backupMetadata)
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		t.Fatalf("Failed to write backup metadata: %v", err)
	}

	// Test finding the backup
	foundBackup, err := findSessionBackup(testSessionID)
	if err != nil {
		t.Fatalf("Failed to find backup: %v", err)
	}

	if foundBackup.SessionID != testSessionID {
		t.Errorf("Expected session ID %s, got %s", testSessionID, foundBackup.SessionID)
	}

	if len(foundBackup.FileRecords) != 1 {
		t.Errorf("Expected 1 file record, got %d", len(foundBackup.FileRecords))
	}

	if foundBackup.SessionMetadata["project"] != "Test Project" {
		t.Errorf("Expected project=Test Project, got %v", foundBackup.SessionMetadata["project"])
	}
}

func TestRestoreSessionData(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	testSessionID := "test-session-restore"
	uploadsDir := filepath.Join(tempDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		t.Fatalf("Failed to create uploads directory: %v", err)
	}

	// Create backup directory with test files
	backupDir := filepath.Join(uploadsDir, testSessionID+".deleted.1234567890")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup directory: %v", err)
	}

	// Create test files in backup
	testFiles := []string{"file1.txt", "file2.jpg", "subdir/file3.csv"}
	for _, filename := range testFiles {
		filePath := filepath.Join(backupDir, filename)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", filename, err)
		}
		if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	backupInfo := &SessionBackupMetadata{
		SessionID:  testSessionID,
		BackupPath: backupDir,
	}

	// Test successful restoration
	filesRestored, err := restoreSessionData(testSessionID, backupInfo)
	if err != nil {
		t.Fatalf("restoreSessionData failed: %v", err)
	}

	if filesRestored != len(testFiles) {
		t.Errorf("Expected %d files restored, got %d", len(testFiles), filesRestored)
	}

	// Verify directory was restored
	restoredDir := filepath.Join(uploadsDir, testSessionID)
	if _, err := os.Stat(restoredDir); os.IsNotExist(err) {
		t.Error("Expected session directory to be restored")
	}

	// Verify backup directory no longer exists
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Error("Expected backup directory to be moved")
	}

	// Test restoration when session already exists
	_, err = restoreSessionData(testSessionID, backupInfo)
	if err == nil {
		t.Error("Expected error when session already exists")
	}
}

func TestRestoreSessionDataPathTraversalProtection(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Test path traversal attack
	maliciousSessionID := "../../../etc"
	backupInfo := &SessionBackupMetadata{
		SessionID:  maliciousSessionID,
		BackupPath: "/tmp/malicious",
	}

	_, err := restoreSessionData(maliciousSessionID, backupInfo)
	if err == nil {
		t.Error("Expected error for path traversal attempt")
	}

	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

func TestRestoreSessionManifest(t *testing.T) {
	manifest.Clear()

	testSessionID := "test-session-manifest"

	// Create backup metadata with file records and session metadata
	backupInfo := &SessionBackupMetadata{
		SessionID: testSessionID,
		FileRecords: []manifest.FileRecord{
			{
				SessionID:  testSessionID,
				Filename:   "file1.txt",
				UploadedAt: time.Now(),
				Tags:       map[string]string{"category": "documents"},
			},
			{
				SessionID:  testSessionID,
				Filename:   "file2.jpg",
				UploadedAt: time.Now(),
				Tags:       map[string]string{"category": "images"},
			},
		},
		SessionMetadata: map[string]interface{}{
			"project": "Test Project",
			"owner":   "test_user",
		},
	}

	// Test restoring manifest
	restored := restoreSessionManifest(backupInfo)
	if !restored {
		t.Error("Expected manifest restoration to return true")
	}

	// Verify file records were restored
	allRecords := manifest.List()
	restoredRecords := 0
	for _, record := range allRecords {
		if record.SessionID == testSessionID {
			restoredRecords++
		}
	}

	if restoredRecords != 2 {
		t.Errorf("Expected 2 restored records, got %d", restoredRecords)
	}

	// Verify session metadata was restored
	sessionMeta, exists := manifest.GetSessionMetadata(testSessionID)
	if !exists {
		t.Error("Expected session metadata to be restored")
	} else {
		if sessionMeta.Metadata["project"] != "Test Project" {
			t.Errorf("Expected project=Test Project, got %v", sessionMeta.Metadata["project"])
		}
		if sessionMeta.Metadata["owner"] != "test_user" {
			t.Errorf("Expected owner=test_user, got %v", sessionMeta.Metadata["owner"])
		}
	}

	// Test restoring empty backup
	emptyBackup := &SessionBackupMetadata{
		SessionID:       "empty-session",
		FileRecords:     []manifest.FileRecord{},
		SessionMetadata: map[string]interface{}{},
	}

	restored = restoreSessionManifest(emptyBackup)
	if restored {
		t.Error("Expected empty manifest restoration to return false")
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"1234567890", 1234567890, false},
		{"0", 0, false},
		{"invalid", 0, true},
		{"", 0, true},
		{"123abc", 0, true},
	}

	for _, tt := range tests {
		t.Run("parse_"+tt.input, func(t *testing.T) {
			result, err := parseTimestamp(tt.input)
			if tt.hasError {
				if err == nil {
					t.Error("Expected error for invalid timestamp")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestCreateBackupMetadata(t *testing.T) {
	manifest.Clear()

	testSessionID := "test-backup-metadata"
	backupPath := "/tmp/backup/path"

	// Add test records to manifest
	testRecords := []manifest.FileRecord{
		{
			SessionID:  testSessionID,
			Filename:   "file1.txt",
			UploadedAt: time.Now(),
			Tags:       map[string]string{"category": "documents"},
		},
		{
			SessionID:  testSessionID,
			Filename:   "file2.jpg",
			UploadedAt: time.Now(),
			Tags:       map[string]string{"category": "images"},
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
	manifest.UpdateSessionMetadata(testSessionID, sessionMetadata)

	// Test creating backup metadata
	backupInfo := createBackupMetadata(testSessionID, backupPath)

	if backupInfo.SessionID != testSessionID {
		t.Errorf("Expected session ID %s, got %s", testSessionID, backupInfo.SessionID)
	}

	if backupInfo.BackupPath != backupPath {
		t.Errorf("Expected backup path %s, got %s", backupPath, backupInfo.BackupPath)
	}

	if len(backupInfo.FileRecords) != 2 {
		t.Errorf("Expected 2 file records, got %d", len(backupInfo.FileRecords))
	}

	// Verify only records for the target session are included
	for _, record := range backupInfo.FileRecords {
		if record.SessionID != testSessionID {
			t.Errorf("Expected session ID %s, got %s", testSessionID, record.SessionID)
		}
	}

	if backupInfo.SessionMetadata["project"] != "Test Project" {
		t.Errorf("Expected project=Test Project, got %v", backupInfo.SessionMetadata["project"])
	}

	if backupInfo.DeletedAt.IsZero() {
		t.Error("Expected DeletedAt to be set")
	}
}

// Helper function to create a test file reader for restore tests
func newTestFileForRestore(content string) *testFileForRestore {
	return &testFileForRestore{strings.NewReader(content)}
}

type testFileForRestore struct {
	*strings.Reader
}

func (tf *testFileForRestore) Close() error                                  { return nil }
func (tf *testFileForRestore) ReadAt(p []byte, off int64) (n int, err error) { return 0, nil }
func (tf *testFileForRestore) Seek(offset int64, whence int) (int64, error)  { return 0, nil }
