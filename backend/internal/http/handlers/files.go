package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/session"
)

type fileEntry struct {
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	Path       string `json:"path"` // relative to session folder
	UploadedAt string `json:"uploaded_at"`
}

// GET /files
// Lists files uploaded for the active session.
// Requires session ID via header "X-Session-ID" or query parameter "session_id".
func filesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate drive availability
	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	// Resolve and validate session
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		sessionID = r.URL.Query().Get("session_id")
	}
	current := session.Get()
	if sessionID == "" || current == nil || current.ID != sessionID || time.Now().After(current.Expires) {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	// Build session directory path
	sessionDir := filepath.Join(drive.GetDrivePath(), "uploads", current.ID)

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("files list: session=%s ip=%s dir not found", current.ID, r.RemoteAddr)
			httpError(w, http.StatusNotFound, "no uploads found for session")
			return
		}
		log.Printf("files list failed: session=%s ip=%s readdir err=%v", current.ID, r.RemoteAddr, err)
		httpError(w, http.StatusInternalServerError, "failed to list uploads")
		return
	}

	list := make([]fileEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			// Skip unreadable entries; log and continue
			log.Printf("files list warn: session=%s ip=%s stat err=%v", current.ID, r.RemoteAddr, err)
			continue
		}
		list = append(list, fileEntry{
			Filename:   e.Name(),
			Size:       info.Size(),
			Path:       e.Name(), // relative path within the session folder
			UploadedAt: info.ModTime().Format(time.RFC3339),
		})
	}

	if len(list) == 0 {
		httpError(w, http.StatusNotFound, "no uploads found for session")
		return
	}

	// Optional: sort by modified time descending
	sort.Slice(list, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, list[i].UploadedAt)
		tj, _ := time.Parse(time.RFC3339, list[j].UploadedAt)
		return ti.After(tj)
	})

	log.Printf("files list success: session=%s ip=%s count=%d", current.ID, r.RemoteAddr, len(list))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

type deleteResponse struct {
	Status   string `json:"status"`
	Filename string `json:"filename"`
}

// DELETE /files/:filename
// Deletes a file uploaded for the active session.
// Requires session ID via header "X-Session-ID" or query parameter "session_id".
func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate drive availability
	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	// Extract filename from URL path
	filename := filepath.Base(r.URL.Path)
	if filename == "" || filename == "." || filename == "/" || filename == "files" {
		httpError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	// Resolve and validate session
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		sessionID = r.URL.Query().Get("session_id")
	}
	current := session.Get()
	if sessionID == "" || current == nil || current.ID != sessionID || time.Now().After(current.Expires) {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	// Build file path
	sessionDir := filepath.Join(drive.GetDrivePath(), "uploads", current.ID)
	filePath := filepath.Join(sessionDir, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			log.Printf("delete file not found: session=%s ip=%s filename=%s", current.ID, r.RemoteAddr, filename)
			httpError(w, http.StatusNotFound, "file not found")
			return
		}
		log.Printf("delete file stat error: session=%s ip=%s filename=%s err=%v", current.ID, r.RemoteAddr, filename, err)
		httpError(w, http.StatusInternalServerError, "failed to access file")
		return
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		log.Printf("delete file failed: session=%s ip=%s filename=%s err=%v", current.ID, r.RemoteAddr, filename, err)
		httpError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	log.Printf("delete file success: session=%s ip=%s filename=%s", current.ID, r.RemoteAddr, filename)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(deleteResponse{
		Status:   "deleted",
		Filename: filename,
	})
}
