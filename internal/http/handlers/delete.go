package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
)

type deleteRequest struct {
	Filename string `json:"filename"`
}

// DELETE /delete - secure deletion of a file for the active session
func secureDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		sessionID = r.URL.Query().Get("session_id")
	}
	current := session.Get()
	if current == nil || sessionID == "" || current.ID != sessionID || time.Now().After(current.Expires) {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	var req deleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Filename) == "" {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := filepath.Base(req.Filename)
	if name == "" || name == "." || strings.Contains(req.Filename, "..") || strings.Contains(req.Filename, "/") || strings.Contains(req.Filename, "\\") {
		httpError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	if !manifest.Belongs(sessionID, name) {
		httpError(w, http.StatusForbidden, "file does not belong to this session")
		return
	}

	sessionDir := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)
	filePath := filepath.Join(sessionDir, name)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			// File already gone; treat as success but ensure manifest removal
			manifest.Remove(sessionID, name)
			log.Printf("delete: already missing session=%s ip=%s filename=%s", sessionID, r.RemoteAddr, name)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "deleted", "filename": name})
			return
		}
		log.Printf("delete failed: session=%s ip=%s filename=%s err=%v", sessionID, r.RemoteAddr, name, err)
		httpError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	manifest.Remove(sessionID, name)
	log.Printf("delete success: session=%s ip=%s filename=%s", sessionID, r.RemoteAddr, name)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "deleted", "filename": name})
}
