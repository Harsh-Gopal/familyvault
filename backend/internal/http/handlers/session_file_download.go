package handlers

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"familyvault/internal/core/drive"

	"github.com/gorilla/mux"
)

// SessionFileDownloadHandler handles GET /sessions/:id/files/:filename/download
func SessionFileDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID and filename from URL
	vars := mux.Vars(r)
	sessionID := vars["id"]
	filename := vars["filename"]

	// Validate session ID
	if !isValidSessionID(sessionID) {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	// Validate filename
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") ||
		strings.Contains(filename, "\\") || strings.Contains(filename, "@") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Resolve and validate authenticated session
	authSessionID := r.Header.Get("X-Session-ID")
	if authSessionID == "" {
		authSessionID = r.URL.Query().Get("session_id")
	}
	if authSessionID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Find file in active session or backup
	filePath, err := findSessionFile(sessionID, filename)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	w.Header().Set("Accept-Ranges", "bytes")

	// Handle range requests
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		handleRangeRequest(w, r, file, fileInfo.Size(), filename)
		return
	}

	// Handle gzip compression if requested
	acceptEncoding := r.Header.Get("Accept-Encoding")
	if strings.Contains(acceptEncoding, "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length") // Remove content-length for gzip

		gzipWriter := gzip.NewWriter(w)
		defer gzipWriter.Close()

		_, err = io.Copy(gzipWriter, file)
	} else {
		_, err = io.Copy(w, file)
	}

	if err != nil {
		// Can't send error response after starting to write body
		return
	}
}

// handleRangeRequest handles HTTP range requests for partial downloads
func handleRangeRequest(w http.ResponseWriter, r *http.Request, file *os.File, fileSize int64, filename string) {
	rangeHeader := r.Header.Get("Range")

	// Parse range header (e.g., "bytes=0-1023")
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		http.Error(w, "Invalid range header", http.StatusBadRequest)
		return
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	rangeParts := strings.Split(rangeSpec, "-")

	if len(rangeParts) != 2 {
		http.Error(w, "Invalid range format", http.StatusBadRequest)
		return
	}

	var start, end int64
	var err error

	// Parse start
	if rangeParts[0] != "" {
		start, err = strconv.ParseInt(rangeParts[0], 10, 64)
		if err != nil || start < 0 {
			http.Error(w, "Invalid range start", http.StatusBadRequest)
			return
		}
	}

	// Parse end
	if rangeParts[1] != "" {
		end, err = strconv.ParseInt(rangeParts[1], 10, 64)
		if err != nil || end < start {
			http.Error(w, "Invalid range end", http.StatusBadRequest)
			return
		}
	} else {
		end = fileSize - 1
	}

	// Validate range
	if start >= fileSize {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		http.Error(w, "Range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	if end >= fileSize {
		end = fileSize - 1
	}

	contentLength := end - start + 1

	// Set range response headers
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.WriteHeader(http.StatusPartialContent)

	// Seek to start position
	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		return
	}

	// Copy the requested range
	_, err = io.CopyN(w, file, contentLength)
	if err != nil {
		return
	}
}

// findSessionFile finds a file in active session or backup
func findSessionFile(sessionID, filename string) (string, error) {
	// Try active session first
	activePath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID, filename)
	if _, err := os.Stat(activePath); err == nil {
		return activePath, nil
	}

	// Try backup location
	backupPath, err := findSessionBackupPath(sessionID)
	if err != nil {
		return "", os.ErrNotExist
	}

	backupFilePath := filepath.Join(backupPath, filename)
	if _, err := os.Stat(backupFilePath); err == nil {
		return backupFilePath, nil
	}

	return "", os.ErrNotExist
}
