package handlers

import (
	"encoding/json"
	"fmt"
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

func TestSessionStatusHandler(t *testing.T) {
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

	// Create test files with different sizes
	testFiles := map[string]string{
		"document.txt": "This is a test document with some content",
		"image.jpg":    "This is fake JPEG content that is longer than the document",
		"data.csv":     "name,age,city\nJohn,30,NYC\nJane,25,LA\nBob,35,SF",
	}

	for filename, content := range testFiles {
		// Create file on disk
		filePath := filepath.Join(sessionDir, filename)
		if err := upload.EncryptAndSave(newTestFileForStatus(content), filePath); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}

		// Add to manifest
		manifest.Add(manifest.FileRecord{
			SessionID:  testSession.ID,
			Filename:   filename,
			UploadedAt: time.Now().Add(-time.Duration(len(testFiles)-1) * time.Hour), // Different timestamps
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
		"description":  "Test session for status endpoint",
	}
	manifest.UpdateSessionMetadata(testSession.ID, sessionMetadata)

	tests := []struct {
		name           string
		sessionID      string
		authSessionID  string
		expectedStatus int
		expectSuccess  bool
		expectedState  SessionStatus
	}{
		{
			name:           "active session status",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedState:  StatusActive,
		},
		{
			name:           "nonexistent session",
			sessionID:      "nonexistent-session-id",
			authSessionID:  testSession.ID,
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
		{
			name:           "unauthorized access",
			sessionID:      testSession.ID,
			authSessionID:  "",
			expectedStatus: http.StatusUnauthorized,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var url string
			if tt.name == "invalid URL format" {
				url = "/sessions/" + tt.sessionID + "/invalid"
			} else {
				url = "/sessions/" + tt.sessionID + "/status"
			}

			req := httptest.NewRequest("GET", url, nil)
			if tt.authSessionID != "" {
				req.Header.Set("X-Session-ID", tt.authSessionID)
			}
			w := httptest.NewRecorder()

			sessionStatusHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectSuccess && w.Code == http.StatusOK {
				// Parse response
				var response SessionStatusResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Verify basic fields
				if response.SessionID != testSession.ID {
					t.Errorf("Expected session ID %s, got %s", testSession.ID, response.SessionID)
				}

				if response.Status != tt.expectedState {
					t.Errorf("Expected status %s, got %s", tt.expectedState, response.Status)
				}

				if response.FilesCount != len(testFiles) {
					t.Errorf("Expected files count %d, got %d", len(testFiles), response.FilesCount)
				}

				if response.TotalSizeBytes <= 0 {
					t.Error("Expected total size to be greater than 0")
				}

				if response.ProjectName != "Test Project" {
					t.Errorf("Expected project name 'Test Project', got %s", response.ProjectName)
				}

				if response.CreatedBy != "test_user" {
					t.Errorf("Expected created by 'test_user', got %s", response.CreatedBy)
				}

				if response.CreatedAt == nil {
					t.Error("Expected CreatedAt to be set")
				}

				if response.LastUpdated == nil {
					t.Error("Expected LastUpdated to be set")
				}

				if len(response.Tags) == 0 {
					t.Error("Expected tags to be populated")
				}

				if response.Tags["category"] != "test" {
					t.Errorf("Expected category tag 'test', got %s", response.Tags["category"])
				}

				if len(response.SessionMetadata) == 0 {
					t.Error("Expected session metadata to be populated")
				}

				if response.BackupInfo != nil {
					t.Error("Expected backup info to be nil for active session")
				}
			}
		})
	}
}

func TestSessionStatusHandlerDeletedSession(t *testing.T) {
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
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(sessionDir, filename)
		if err := upload.EncryptAndSave(newTestFileForStatus(content), filePath); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}

		manifest.Add(manifest.FileRecord{
			SessionID:  testSession.ID,
			Filename:   filename,
			UploadedAt: time.Now(),
			Tags: map[string]string{
				"category": "test",
			},
		})
	}

	// Add session metadata
	sessionMetadata := map[string]interface{}{
		"project_name": "Deleted Test Project",
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

	// Now test status of deleted session
	req = httptest.NewRequest("GET", "/sessions/"+testSession.ID+"/status", nil)
	req.Header.Set("X-Session-ID", testSession.ID)
	w = httptest.NewRecorder()

	sessionStatusHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response SessionStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify deleted session status
	if response.Status != StatusDeletedWithBackup {
		t.Errorf("Expected status %s, got %s", StatusDeletedWithBackup, response.Status)
	}

	if response.FilesCount != len(testFiles) {
		t.Errorf("Expected files count %d, got %d", len(testFiles), response.FilesCount)
	}

	if response.ProjectName != "Deleted Test Project" {
		t.Errorf("Expected project name 'Deleted Test Project', got %s", response.ProjectName)
	}

	if response.BackupInfo == nil {
		t.Error("Expected backup info to be populated")
	} else {
		if !response.BackupInfo.BackupExists {
			t.Error("Expected backup to exist")
		}
		if response.BackupInfo.DeletedAt.IsZero() {
			t.Error("Expected deleted at timestamp to be set")
		}
	}
}

func TestSessionStatusHandlerDriveNotAvailable(t *testing.T) {
	// Setup test environment with non-existent drive path
	drive.SetDrivePath("/nonexistent/path")

	req := httptest.NewRequest("GET", "/sessions/test-session/status", nil)
	req.Header.Set("X-Session-ID", "test-session")
	w := httptest.NewRecorder()

	sessionStatusHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestSessionStatusHandlerWrongMethod(t *testing.T) {
	req := httptest.NewRequest("POST", "/sessions/test-session/status", nil)
	w := httptest.NewRecorder()

	sessionStatusHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestIsSessionActive(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	manifest.Clear()

	testSessionID := "test-session-active"

	// Test 1: Session doesn't exist
	if isSessionActive(testSessionID) {
		t.Error("Expected session to not be active initially")
	}

	// Test 2: Session exists in manifest
	manifest.Add(manifest.FileRecord{
		SessionID:  testSessionID,
		Filename:   "test.txt",
		UploadedAt: time.Now(),
		Tags:       map[string]string{},
	})

	if !isSessionActive(testSessionID) {
		t.Error("Expected session to be active with manifest entry")
	}

	// Clear manifest for next test
	manifest.Clear()

	// Test 3: Session exists in metadata
	manifest.UpdateSessionMetadata(testSessionID, map[string]interface{}{
		"test": "value",
	})

	if !isSessionActive(testSessionID) {
		t.Error("Expected session to be active with metadata")
	}

	// Clear metadata for next test
	manifest.ClearSessionMetadata(testSessionID)

	// Test 4: Session exists on disk
	sessionDir := filepath.Join(tempDir, "uploads", testSessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	if !isSessionActive(testSessionID) {
		t.Error("Expected session to be active with directory on disk")
	}
}

func TestCalculateSessionSize(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	testSessionID := "test-session-size"
	sessionDir := filepath.Join(tempDir, "uploads", testSessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create test files with known sizes
	testFiles := map[string]string{
		"file1.txt":        "Hello World",         // 11 bytes
		"file2.txt":        "This is a test file", // 17 bytes
		"subdir/file3.txt": "Nested file content", // 18 bytes
	}

	expectedSize := int64(0)
	for filename, content := range testFiles {
		filePath := filepath.Join(sessionDir, filename)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", filename, err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
		expectedSize += int64(len(content))
	}

	// Test calculating session size
	actualSize, err := calculateSessionSize(testSessionID)
	if err != nil {
		t.Fatalf("calculateSessionSize failed: %v", err)
	}

	if actualSize != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, actualSize)
	}

	// Test with non-existent session
	_, err = calculateSessionSize("nonexistent-session")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}
}

func TestCalculateBackupSize(t *testing.T) {
	tempDir := t.TempDir()

	backupDir := filepath.Join(tempDir, "backup")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup directory: %v", err)
	}

	// Create test files with known sizes
	testFiles := map[string]string{
		"file1.txt":        "Hello World",         // 11 bytes
		"file2.txt":        "This is a test file", // 17 bytes
		"subdir/file3.txt": "Nested file content", // 18 bytes
	}

	expectedSize := int64(0)
	for filename, content := range testFiles {
		filePath := filepath.Join(backupDir, filename)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", filename, err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
		expectedSize += int64(len(content))
	}

	// Create backup metadata file (should be excluded from size calculation)
	metadataPath := filepath.Join(backupDir, ".backup_metadata.json")
	metadataContent := `{"session_id": "test"}`
	if err := os.WriteFile(metadataPath, []byte(metadataContent), 0644); err != nil {
		t.Fatalf("Failed to create metadata file: %v", err)
	}

	// Test calculating backup size
	actualSize, err := calculateBackupSize(backupDir)
	if err != nil {
		t.Fatalf("calculateBackupSize failed: %v", err)
	}

	if actualSize != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, actualSize)
	}

	// Test with non-existent backup
	_, err = calculateBackupSize("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for non-existent backup")
	}
}

func TestFindSessionBackupInfo(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	testSessionID := "test-session-backup-info"
	uploadsDir := filepath.Join(tempDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		t.Fatalf("Failed to create uploads directory: %v", err)
	}

	// Test 1: No backup exists
	_, err := findSessionBackupInfo(testSessionID)
	if err == nil {
		t.Error("Expected error for non-existent backup")
	}

	// Test 2: Create backup directory
	timestamp := time.Now().Unix()
	backupDir := filepath.Join(uploadsDir, testSessionID+".deleted."+fmt.Sprintf("%d", timestamp))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup directory: %v", err)
	}

	// Test finding the backup
	backupInfo, err := findSessionBackupInfo(testSessionID)
	if err != nil {
		t.Fatalf("Failed to find backup: %v", err)
	}

	if backupInfo.BackupPath != backupDir {
		t.Errorf("Expected backup path %s, got %s", backupDir, backupInfo.BackupPath)
	}

	if !backupInfo.BackupExists {
		t.Error("Expected backup to exist")
	}

	if backupInfo.DeletedAt.Unix() != timestamp {
		t.Errorf("Expected deleted at %d, got %d", timestamp, backupInfo.DeletedAt.Unix())
	}

	// Test 3: Multiple backups (should return most recent)
	olderTimestamp := timestamp - 3600
	olderBackupDir := filepath.Join(uploadsDir, testSessionID+".deleted."+fmt.Sprintf("%d", olderTimestamp))
	if err := os.MkdirAll(olderBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create older backup directory: %v", err)
	}

	backupInfo, err = findSessionBackupInfo(testSessionID)
	if err != nil {
		t.Fatalf("Failed to find backup: %v", err)
	}

	// Should still return the newer backup
	if backupInfo.BackupPath != backupDir {
		t.Errorf("Expected newer backup path %s, got %s", backupDir, backupInfo.BackupPath)
	}
}

func TestGetSessionStatus(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	manifest.Clear()

	testSessionID := "test-session-status"

	// Test 1: Non-existent session
	nonExistentStatus, err := getSessionStatus(testSessionID)
	if err != nil {
		t.Fatalf("Unexpected error for non-existent session: %v", err)
	}
	if nonExistentStatus.Status != StatusDeletedNoBackup {
		t.Errorf("Expected status %s for non-existent session, got %s", StatusDeletedNoBackup, nonExistentStatus.Status)
	}

	// Test 2: Active session
	sessionDir := filepath.Join(tempDir, "uploads", testSessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	manifest.Add(manifest.FileRecord{
		SessionID:  testSessionID,
		Filename:   "test.txt",
		UploadedAt: time.Now(),
		Tags:       map[string]string{"category": "test"},
	})

	manifest.UpdateSessionMetadata(testSessionID, map[string]interface{}{
		"project_name": "Test Project",
		"created_by":   "test_user",
	})

	status, err := getSessionStatus(testSessionID)
	if err != nil {
		t.Fatalf("Failed to get session status: %v", err)
	}

	if status.Status != StatusActive {
		t.Errorf("Expected status %s, got %s", StatusActive, status.Status)
	}

	if status.SessionID != testSessionID {
		t.Errorf("Expected session ID %s, got %s", testSessionID, status.SessionID)
	}

	if status.FilesCount != 1 {
		t.Errorf("Expected files count 1, got %d", status.FilesCount)
	}

	if status.ProjectName != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got %s", status.ProjectName)
	}
}

// Helper function to create a test file reader for status tests
func newTestFileForStatus(content string) *testFileForStatus {
	return &testFileForStatus{strings.NewReader(content)}
}

type testFileForStatus struct {
	*strings.Reader
}

func (tf *testFileForStatus) Close() error                                  { return nil }
func (tf *testFileForStatus) ReadAt(p []byte, off int64) (n int, err error) { return 0, nil }
func (tf *testFileForStatus) Seek(offset int64, whence int) (int64, error)  { return 0, nil }
