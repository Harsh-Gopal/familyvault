package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"familyvault/internal/auth"
	"familyvault/internal/core/drive"

	"github.com/gorilla/mux"
)

// FileUploadResponse represents the response after file upload
type FileUploadResponse struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	UploadedAt  string `json:"uploaded_at"`
	ContentType string `json:"content_type"`
	SessionID   string `json:"session_id"`
}

// SessionFileUploadHandler handles POST /sessions/:id/files/upload
func SessionFileUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from URL
	vars := mux.Vars(r)
	sessionID := vars["id"]

	// Validate session ID
	if !isValidSessionID(sessionID) {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	// Check upload permission
	user := auth.GetUserFromContext(r.Context())
	if user == nil || !auth.HasPermission(user.Role, "upload") {
		http.Error(w, "Forbidden: upload permission required", http.StatusForbidden)
		return
	}

	// Ensure session directory exists
	sessionPath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		http.Error(w, "Failed to create session directory", http.StatusInternalServerError)
		return
	}

	// Check if this is a resumable upload
	if r.Header.Get("Upload-Resumable") == "1" {
		handleResumableUpload(w, r, sessionID, sessionPath)
		return
	}

	// Handle regular multipart upload
	handleMultipartUpload(w, r, sessionID, sessionPath)
}

// handleMultipartUpload handles regular multipart/form-data uploads
func handleMultipartUpload(w http.ResponseWriter, r *http.Request, sessionID, sessionPath string) {
	// Parse multipart form (32MB max memory)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file size (100MB limit for regular upload)
	if fileHeader.Size > 100*1024*1024 {
		http.Error(w, "File too large (max 100MB for regular upload)", http.StatusRequestEntityTooLarge)
		return
	}

	// Sanitize filename
	filename, err := sanitizeFilename(fileHeader.Filename)
	if err != nil || filename == "" {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Check if file already exists and generate unique name if needed
	finalFilename := generateUniqueFilename(sessionPath, filename)
	filePath := filepath.Join(sessionPath, finalFilename)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copy file content
	bytesWritten, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(filePath) // Cleanup on error
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Create response
	response := FileUploadResponse{
		Filename:    finalFilename,
		Size:        bytesWritten,
		UploadedAt:  time.Now().UTC().Format(time.RFC3339),
		ContentType: fileHeader.Header.Get("Content-Type"),
		SessionID:   sessionID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleResumableUpload handles resumable uploads for large files
func handleResumableUpload(w http.ResponseWriter, r *http.Request, sessionID, sessionPath string) {
	// Get upload parameters
	filename := r.Header.Get("Upload-Filename")
	if filename == "" {
		http.Error(w, "Upload-Filename header required", http.StatusBadRequest)
		return
	}

	filename, err := sanitizeFilename(filename)
	if err != nil || filename == "" {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	totalSizeStr := r.Header.Get("Upload-Length")
	if totalSizeStr == "" {
		http.Error(w, "Upload-Length header required", http.StatusBadRequest)
		return
	}

	totalSize, err := strconv.ParseInt(totalSizeStr, 10, 64)
	if err != nil || totalSize <= 0 {
		http.Error(w, "Invalid Upload-Length", http.StatusBadRequest)
		return
	}

	// Validate total size (1GB limit for resumable upload)
	if totalSize > 1024*1024*1024 {
		http.Error(w, "File too large (max 1GB)", http.StatusRequestEntityTooLarge)
		return
	}

	offsetStr := r.Header.Get("Upload-Offset")
	offset := int64(0)
	if offsetStr != "" {
		offset, err = strconv.ParseInt(offsetStr, 10, 64)
		if err != nil || offset < 0 {
			http.Error(w, "Invalid Upload-Offset", http.StatusBadRequest)
			return
		}
	}

	// For resumable uploads, use the original filename (don't generate unique)
	finalFilename := filename
	filePath := filepath.Join(sessionPath, finalFilename)

	// Open or create file (don't truncate for resumable uploads)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Seek to offset
	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		http.Error(w, "Failed to seek file", http.StatusInternalServerError)
		return
	}

	// Copy chunk
	bytesWritten, err := io.Copy(file, r.Body)
	if err != nil {
		http.Error(w, "Failed to write chunk", http.StatusInternalServerError)
		return
	}

	newOffset := offset + bytesWritten

	// Set response headers
	w.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))

	if newOffset >= totalSize {
		// Upload complete
		response := FileUploadResponse{
			Filename:    finalFilename,
			Size:        totalSize,
			UploadedAt:  time.Now().UTC().Format(time.RFC3339),
			ContentType: "application/octet-stream",
			SessionID:   sessionID,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	} else {
		// Partial upload
		w.WriteHeader(http.StatusNoContent)
	}
}
