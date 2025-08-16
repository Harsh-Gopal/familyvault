package handlers

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"familyvault/internal/core/download"
	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
)

// GET /download?filename=<filename>
// Downloads a single file from the active session with full security and streaming.
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
	sanitizedName, err := sanitizeDownloadFilename(filename)
	if err != nil {
		log.Printf("download attempt with unsafe filename: ip=%s filename=%s err=%v", r.RemoteAddr, filename, err)
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

	// Check if file exists in manifest for this session
	if !manifest.Belongs(current.ID, sanitizedName) {
		log.Printf("download file not in manifest: session=%s ip=%s filename=%s", current.ID, r.RemoteAddr, sanitizedName)
		httpError(w, http.StatusNotFound, "file not found")
		return
	}

	// Build file path
	sessionDir := filepath.Join(drive.GetDrivePath(), "uploads", current.ID)
	filePath := filepath.Join(sessionDir, sanitizedName)

	// Check if file exists on disk and get info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("download file not found on disk: session=%s ip=%s filename=%s", current.ID, r.RemoteAddr, sanitizedName)
			httpError(w, http.StatusNotFound, "file not found")
			return
		}
		log.Printf("download file stat error: session=%s ip=%s filename=%s err=%v", current.ID, r.RemoteAddr, sanitizedName, err)
		httpError(w, http.StatusInternalServerError, "failed to access file")
		return
	}

	// Detect MIME type based on file extension
	contentType := detectContentType(sanitizedName)

	// Set headers for download
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizedName))

	// Set content length for decrypted content (encrypted size - 16 bytes IV)
	if sz := fileInfo.Size(); sz > 16 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", sz-16))
	}

	// Stream decrypted content
	if err := download.DecryptAndStream(filePath, w); err != nil {
		log.Printf("download decrypt stream error: session=%s ip=%s filename=%s err=%v", current.ID, r.RemoteAddr, sanitizedName, err)
		// Headers already sent, can't send error response
		return
	}

	log.Printf("download success: session=%s ip=%s filename=%s size=%d", current.ID, r.RemoteAddr, sanitizedName, fileInfo.Size())
}

// sanitizeDownloadFilename validates and sanitizes a filename for download
func sanitizeDownloadFilename(filename string) (string, error) {
	// Check for path traversal attempts
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return "", fmt.Errorf("filename contains unsafe characters")
	}

	// Remove any path components
	sanitized := filepath.Base(filename)

	// Check for empty or invalid names
	if sanitized == "" || sanitized == "." || sanitized == ".." {
		return "", fmt.Errorf("invalid filename")
	}

	// Additional validation for control characters and other unsafe characters
	for _, char := range sanitized {
		if char < 32 || char == 127 { // Control characters
			return "", fmt.Errorf("filename contains control characters")
		}
	}

	return sanitized, nil
}

// detectContentType determines the MIME type based on file extension
func detectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	// Use Go's built-in MIME type detection
	if contentType := mime.TypeByExtension(ext); contentType != "" {
		return contentType
	}

	// Fallback for common types not in Go's mime package
	switch ext {
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".odt":
		return "application/vnd.oasis.opendocument.text"
	case ".ods":
		return "application/vnd.oasis.opendocument.spreadsheet"
	case ".odp":
		return "application/vnd.oasis.opendocument.presentation"
	default:
		return "application/octet-stream"
	}
}
