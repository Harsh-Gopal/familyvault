package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
)

type openSessionResponse struct {
	SessionID string    `json:"session_id"`
	Expires   time.Time `json:"expires"`
}

// POST /session/open
// Starts a new session with configurable duration from SESSION_TIMEOUT_MINUTES env var.
func sessionOpenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Use configurable session timeout from environment variable
	newSession, err := session.OpenWithDefaultTimeout()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	// Simulate notifications to family members
	log.Printf("Session opened: id=%s expires=%s (notify family via SMS/Email: simulated)", newSession.ID, newSession.Expires.Format(time.RFC3339))

	resp := openSessionResponse{SessionID: newSession.ID, Expires: newSession.Expires}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// POST /session/close
// Ends the current session if present.
func sessionCloseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// On close: delete all files for the session, clear manifest, clear session
	current := session.Get()
	if current != nil {
		// Remove files on disk
		sessionDir := filepath.Join(drive.GetDrivePath(), "uploads", current.ID)
		if err := os.RemoveAll(sessionDir); err != nil {
			log.Printf("session close: failed to remove session dir: %v", err)
		}
		// Remove manifest records
		removed := manifest.RemoveAllForSession(current.ID)
		log.Printf("Session closed: id=%s removed_files=%d", current.ID, removed)
	} else {
		log.Printf("Session close requested but no active session")
	}
	session.Close()
	w.WriteHeader(http.StatusNoContent)
}
