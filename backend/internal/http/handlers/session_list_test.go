package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"familyvault/internal/core/session"
)

func TestGetActiveSessions_Empty(t *testing.T) {
	// Ensure no active session
	session.Close()

	srv := newTestServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/sessions/active")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var arr []activeSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(arr) != 0 {
		t.Fatalf("expected empty list, got %d", len(arr))
	}
}

func TestGetActiveSessions_WithActiveAndExpired(t *testing.T) {
	session.Close()

	// Create a short-lived session that will expire
	expired, err := session.Open(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("open expired: %v", err)
	}

	// Create a longer session to remain active
	active, err := session.Open(2 * time.Minute)
	if err != nil {
		t.Fatalf("open active: %v", err)
	}

	// Wait for the first to expire
	_ = expired
	time.Sleep(120 * time.Millisecond)

	srv := newTestServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/sessions/active")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var arr []activeSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(arr))
	}
	if arr[0].SessionID != active.ID {
		t.Fatalf("expected active session id %s, got %s", active.ID, arr[0].SessionID)
	}
	if arr[0].RemainingMinutes <= 0 {
		t.Fatalf("expected remaining minutes > 0, got %d", arr[0].RemainingMinutes)
	}
}
