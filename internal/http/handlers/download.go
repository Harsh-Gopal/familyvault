package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"familyvault/internal/core/download"
	"familyvault/internal/core/drive"
	"familyvault/internal/core/session"
)

// GET /download?filename=<filename>
// Downloads a file uploaded for the active session.
// Requires session ID via header "X-Session-ID" or query parameter "session_id".
// Returns the file as a downloadable attachment with proper MIME type detection.
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate drive availability
	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	// Get and validate filename parameter
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		httpError(w, http.StatusBadRequest, "filename parameter is required")
		return
	}

	// Sanitize filename to prevent directory traversal
	sanitizedName := filepath.Base(filename)
	if sanitizedName == "" || sanitizedName == "." || sanitizedName == "/" ||
		strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		log.Printf("download attempt with unsafe filename: ip=%s filename=%s", r.RemoteAddr, filename)
		httpError(w, http.StatusBadRequest, "invalid or unsafe filename")
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
	filePath := filepath.Join(sessionDir, sanitizedName)

	// Check if file exists and get info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("download file not found: session=%s ip=%s filename=%s", current.ID, r.RemoteAddr, sanitizedName)
			httpError(w, http.StatusNotFound, "file not found")
			return
		}
		log.Printf("download file stat error: session=%s ip=%s filename=%s err=%v", current.ID, r.RemoteAddr, sanitizedName, err)
		httpError(w, http.StatusInternalServerError, "failed to access file")
		return
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("download file open error: session=%s ip=%s filename=%s err=%v", current.ID, r.RemoteAddr, sanitizedName, err)
		httpError(w, http.StatusInternalServerError, "failed to open file")
		return
	}
	defer file.Close()

	// Set headers for download (decrypted content)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizedName+"\"")
	// If possible set plaintext size (ciphertext size - 16 bytes IV)
	if sz := fileInfo.Size(); sz > 16 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", sz-16))
	}

	// Stream decrypted content
	if err := download.DecryptAndStream(filePath, w); err != nil {
		log.Printf("download decrypt stream error: session=%s ip=%s filename=%s err=%v", current.ID, r.RemoteAddr, sanitizedName, err)
		httpError(w, http.StatusInternalServerError, "failed to decrypt or stream file")
		return
	}

	log.Printf("download success: session=%s ip=%s filename=%s", current.ID, r.RemoteAddr, sanitizedName)
}
