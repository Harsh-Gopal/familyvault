package handlers

import (
	"archive/zip"
	"bytes"
	"io"
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
	"familyvault/internal/core/upload"
)

// testFile wraps a strings.Reader to implement multipart.File interface
type testFile struct {
	*strings.Reader
}

func (tf *testFile) Close() error {
	return nil
}

func newTestFile(content string) multipart.File {
	return &testFile{strings.NewReader(content)}
}

func TestDownloadAllHandler(t *testing.T) {
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

	// Create test files
	testFiles := map[string]string{
		"test1.txt": "Hello, World!",
		"test2.txt": "This is test file 2",
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
		expectedStatus int
		expectZip      bool
	}{
		{
			name:           "valid session with files",
			sessionID:      testSession.ID,
			expectedStatus: http.StatusOK,
			expectZip:      true,
		},
		{
			name:           "invalid session",
			sessionID:      "invalid-session-id",
			expectedStatus: http.StatusUnauthorized,
			expectZip:      false,
		},
		{
			name:           "missing session",
			sessionID:      "",
			expectedStatus: http.StatusUnauthorized,
			expectZip:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/download-all", nil)
			if tt.sessionID != "" {
				req.Header.Set("X-Session-ID", tt.sessionID)
			}

			w := httptest.NewRecorder()
			downloadAllHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectZip {
				// Verify ZIP content
				if w.Header().Get("Content-Type") != "application/zip" {
					t.Errorf("Expected Content-Type application/zip, got %s", w.Header().Get("Content-Type"))
				}

				expectedFilename := "session_" + testSession.ID + ".zip"
				expectedDisposition := "attachment; filename=\"" + expectedFilename + "\""
				if w.Header().Get("Content-Disposition") != expectedDisposition {
					t.Errorf("Expected Content-Disposition %s, got %s", expectedDisposition, w.Header().Get("Content-Disposition"))
				}

				// Parse ZIP and verify contents
				zipReader, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
				if err != nil {
					t.Fatalf("Failed to read ZIP: %v", err)
				}

				if len(zipReader.File) != len(testFiles) {
					t.Errorf("Expected %d files in ZIP, got %d", len(testFiles), len(zipReader.File))
				}

				for _, zipFile := range zipReader.File {
					expectedContent, exists := testFiles[zipFile.Name]
					if !exists {
						t.Errorf("Unexpected file in ZIP: %s", zipFile.Name)
						continue
					}

					rc, err := zipFile.Open()
					if err != nil {
						t.Errorf("Failed to open ZIP file %s: %v", zipFile.Name, err)
						continue
					}

					content, err := io.ReadAll(rc)
					rc.Close()
					if err != nil {
						t.Errorf("Failed to read ZIP file %s: %v", zipFile.Name, err)
						continue
					}

					if string(content) != expectedContent {
						t.Errorf("File %s content mismatch. Expected %q, got %q", zipFile.Name, expectedContent, string(content))
					}
				}
			}
		})
	}
}

func TestDownloadAllHandlerNoFiles(t *testing.T) {
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

	// Create empty session directory
	sessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	req := httptest.NewRequest("GET", "/download-all", nil)
	req.Header.Set("X-Session-ID", testSession.ID)

	w := httptest.NewRecorder()
	downloadAllHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestDownloadAllHandlerMissingFile(t *testing.T) {
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

	// Create one real file
	realFile := "real.txt"
	realContent := "This file exists"
	filePath := filepath.Join(sessionDir, realFile)
	if err := upload.EncryptAndSave(newTestFile(realContent), filePath); err != nil {
		t.Fatalf("Failed to create real test file: %v", err)
	}

	// Add both real and missing files to manifest
	manifest.Add(manifest.FileRecord{
		SessionID:  testSession.ID,
		Filename:   realFile,
		UploadedAt: time.Now(),
	})
	manifest.Add(manifest.FileRecord{
		SessionID:  testSession.ID,
		Filename:   "missing.txt",
		UploadedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", "/download-all", nil)
	req.Header.Set("X-Session-ID", testSession.ID)

	w := httptest.NewRecorder()
	downloadAllHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Parse ZIP and verify it contains the real file and errors.log
	zipReader, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("Failed to read ZIP: %v", err)
	}

	foundRealFile := false
	foundErrorsLog := false

	for _, zipFile := range zipReader.File {
		switch zipFile.Name {
		case realFile:
			foundRealFile = true
			rc, err := zipFile.Open()
			if err != nil {
				t.Errorf("Failed to open real file in ZIP: %v", err)
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Errorf("Failed to read real file in ZIP: %v", err)
				continue
			}
			if string(content) != realContent {
				t.Errorf("Real file content mismatch. Expected %q, got %q", realContent, string(content))
			}
		case "errors.log":
			foundErrorsLog = true
			rc, err := zipFile.Open()
			if err != nil {
				t.Errorf("Failed to open errors.log in ZIP: %v", err)
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Errorf("Failed to read errors.log in ZIP: %v", err)
				continue
			}
			if !strings.Contains(string(content), "missing.txt") {
				t.Errorf("errors.log should mention missing.txt, got: %s", string(content))
			}
		}
	}

	if !foundRealFile {
		t.Error("Expected to find real file in ZIP")
	}
	if !foundErrorsLog {
		t.Error("Expected to find errors.log in ZIP")
	}
}
