package handlers

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/session"
	"familyvault/internal/core/upload"
)

type testMultipartFile struct{ *bytes.Reader }

func (f testMultipartFile) Close() error { return nil }

func TestDownload_Success(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	s, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	// Create an encrypted file to download (match runtime behavior)
	sessionDir := filepath.Join(driveDir, "uploads", s.ID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	testFile := filepath.Join(sessionDir, "test.txt")
	// encrypt the data using the same function used by upload
	{
		data := []byte("Hello, World!")
		if err := upload.EncryptAndSave(testMultipartFile{bytes.NewReader(data)}, testFile); err != nil {
			t.Fatalf("EncryptAndSave: %v", err)
		}
	}

	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/download?filename=test.txt", nil)
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

	// Check headers
	if resp.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("unexpected content type: %s", resp.Header.Get("Content-Type"))
	}
	expectedDisposition := "attachment; filename=\"test.txt\""
	if resp.Header.Get("Content-Disposition") != expectedDisposition {
		t.Fatalf("unexpected content disposition: %s", resp.Header.Get("Content-Disposition"))
	}

	// Check decrypted content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "Hello, World!" {
		t.Fatalf("content mismatch: got %s", string(body))
	}
}

func TestDownload_InvalidSession(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	// No active session
	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/download?filename=test.txt&session_id=invalid", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", resp.StatusCode)
	}
}

func TestDownload_MissingFilename(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	s, err := session.Open(1 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/download", nil)
	req.Header.Set("X-Session-ID", s.ID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", resp.StatusCode)
	}
}

func TestDownload_UnsafeFilename(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	s, err := session.Open(1 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	srv := newTestServer()
	defer srv.Close()

	// Test various unsafe filenames
	unsafeFilenames := []string{
		"../test.txt",
		"../../etc/passwd",
		"test/../secret.txt",
		"/etc/passwd",
		"\\windows\\system32\\config",
	}

	for _, filename := range unsafeFilenames {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/download?filename="+filename, nil)
		req.Header.Set("X-Session-ID", s.ID)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for unsafe filename %s, got %d", filename, resp.StatusCode)
		}
	}
}

func TestDownload_FileNotFound(t *testing.T) {
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

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/download?filename=nonexistent.txt", nil)
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

func TestDownload_WithQueryParamAuth(t *testing.T) {
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	s, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	// Create an encrypted test file
	sessionDir := filepath.Join(driveDir, "uploads", s.ID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	testFile := filepath.Join(sessionDir, "document.pdf")
	{
		data := []byte("%PDF-1.4 fake pdf content")
		if err := upload.EncryptAndSave(testMultipartFile{bytes.NewReader(data)}, testFile); err != nil {
			t.Fatalf("EncryptAndSave: %v", err)
		}
	}

	srv := newTestServer()
	defer srv.Close()

	// Use session_id query parameter instead of header
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/download?filename=document.pdf&session_id="+s.ID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 got %d: %s", resp.StatusCode, string(body))
	}

	// We always set octet-stream for decrypted content
	if resp.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("unexpected content type: %s", resp.Header.Get("Content-Type"))
	}
}
