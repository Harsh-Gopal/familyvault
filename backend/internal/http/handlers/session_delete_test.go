package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/session"
)

func TestDeleteSession_Success(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	session.Close()
	s, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	// Create session directory to make it exist in the system
	sessionDir := filepath.Join(tempDir, "uploads", s.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/sessions/"+s.ID, nil)
	req.Header.Set("X-Session-ID", s.ID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
	// Now Get() should be nil
	if session.Get() != nil {
		t.Fatalf("session should be revoked")
	}
}

func TestDeleteSession_NotFound(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	session.Close()
	srv := newTestServer()
	defer srv.Close()

	// No active session, revoke should 404
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/sessions/nonexistent", nil)
	req.Header.Set("X-Session-ID", "nonexistent")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteSession_InvalidSession(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	session.Close()
	s, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	srv := newTestServer()
	defer srv.Close()

	// Missing auth header
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/sessions/"+s.ID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestDeleteSession_OtherUserSession(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	session.Close()
	s, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	srv := newTestServer()
	defer srv.Close()

	// Attempt to revoke a different session id than the authenticated one
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/sessions/other-id", nil)
	req.Header.Set("X-Session-ID", s.ID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}
