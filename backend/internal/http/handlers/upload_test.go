package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/session"
)

func newTestServer() *httptest.Server {
	mux := http.NewServeMux()
	RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func createMultipartBody(t *testing.T, fieldName, filename string, content []byte, extraFields map[string]string) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	// add file
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatalf("copy file content: %v", err)
	}
	for k, v := range extraFields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	return body, writer.FormDataContentType()
}

func TestUploadSuccess(t *testing.T) {
	// Simulate plugged-in drive
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	// Open a session
	s, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	srv := newTestServer()
	defer srv.Close()

	payload := []byte("hello world")
	body, contentType := createMultipartBody(t, "file", "greeting.txt", payload, nil)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/upload", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Session-ID", s.ID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if out["status"].(string) != "uploaded" {
		t.Fatalf("unexpected status: %v", out["status"])
	}
	// Check file content
	// Encrypted file path is under driveDir/uploads/<session_id>/greeting.txt
	savedPath := filepath.Join(driveDir, "uploads", s.ID, "greeting.txt")
	saved, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	// We can't compare ciphertext directly with plaintext; ensure file exists and has IV+ciphertext
	if len(saved) <= 16 {
		t.Fatalf("encrypted file too small")
	}
}

func TestUploadMissingSession(t *testing.T) {
	// Simulate plugged-in drive
	driveDir := t.TempDir()
	drive.SetDrivePath(driveDir)

	// Ensure some session exists but we won't send the header
	if _, err := session.Open(1 * time.Minute); err != nil {
		t.Fatalf("open session: %v", err)
	}

	srv := newTestServer()
	defer srv.Close()

	body, contentType := createMultipartBody(t, "file", "note.txt", []byte("data"), nil)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/upload", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
	// Intentionally omit X-Session-ID and session_id

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
