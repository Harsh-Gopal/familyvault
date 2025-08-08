package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/session"
)

func TestFilesList_Success(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	s, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	// Create two uploaded files under the session directory
	sessionDir := filepath.Join(driveDir, "uploads", s.ID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f1 := filepath.Join(sessionDir, "a.txt")
	f2 := filepath.Join(sessionDir, "b.txt")
	if err := os.WriteFile(f1, []byte("first"), 0o644); err != nil {
		t.Fatalf("write f1: %v", err)
	}
	// Ensure different mod times
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(f2, []byte("second"), 0o644); err != nil {
		t.Fatalf("write f2: %v", err)
	}

	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/files", nil)
	req.Header.Set("X-Session-ID", s.ID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 got %d: %s", resp.StatusCode, string(body))
	}

	var list []fileEntry
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 files, got %d", len(list))
	}
}

func TestFilesList_InvalidSession(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	// No active session
	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/files?session_id=invalid", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", resp.StatusCode)
	}
}

func TestFilesList_EmptyUploads(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	s, err := session.Open(1 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	// Ensure session dir exists but empty
	sessionDir := filepath.Join(driveDir, "uploads", s.ID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/files", nil)
	req.Header.Set("X-Session-ID", s.ID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", resp.StatusCode)
	}
}

func TestDeleteFile_Success(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	s, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	// Create a file to delete
	sessionDir := filepath.Join(driveDir, "uploads", s.ID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	testFile := filepath.Join(sessionDir, "delete-me.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/files/delete-me.txt", nil)
	req.Header.Set("X-Session-ID", s.ID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 got %d: %s", resp.StatusCode, string(body))
	}

	var result deleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != "deleted" {
		t.Fatalf("expected status 'deleted', got %s", result.Status)
	}
	if result.Filename != "delete-me.txt" {
		t.Fatalf("expected filename 'delete-me.txt', got %s", result.Filename)
	}

	// Verify file is actually deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatalf("file should have been deleted")
	}
}

func TestDeleteFile_InvalidSession(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	// No active session
	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/files/nonexistent.txt?session_id=invalid", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", resp.StatusCode)
	}
}

func TestDeleteFile_FileNotFound(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	s, err := session.Open(1 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	// Ensure session dir exists but file doesn't
	sessionDir := filepath.Join(driveDir, "uploads", s.ID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/files/nonexistent.txt", nil)
	req.Header.Set("X-Session-ID", s.ID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", resp.StatusCode)
	}
}
