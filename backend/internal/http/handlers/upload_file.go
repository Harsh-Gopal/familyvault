package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"familyvault/internal/config"
	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
	"familyvault/internal/core/upload"
)

// UploadFileResponse represents the response for successful file upload
type UploadFileResponse struct {
	Name       string            `json:"name"`
	Size       int64             `json:"size"`
	UploadTime time.Time         `json:"upload_time"`
	Type       string            `json:"type"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// POST /upload-file
// Uploads a file with optional tags for the active session.
// Requires session ID via header "X-Session-ID" or query parameter "session_id".
// Accepts multipart form-data with 'file' field (mandatory) and 'tags' field (optional JSON).
func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate drive availability
	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	// Get upload configuration
	uploadConfig := config.GetUploadConfig()

	// Set max request size to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, uploadConfig.MaxFileSize+1024*1024) // Add 1MB buffer for form data

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

	// Parse multipart form
	err := r.ParseMultipartForm(uploadConfig.MaxFileSize)
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			httpError(w, http.StatusRequestEntityTooLarge, "file too large")
			return
		}
		log.Printf("upload-file parse form error: session=%s err=%v", current.ID, err)
		httpError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	defer r.MultipartForm.RemoveAll()

	// Get file from form
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		httpError(w, http.StatusBadRequest, "missing or invalid file field")
		return
	}
	defer file.Close()

	// Validate file is not empty
	if fileHeader.Size == 0 {
		httpError(w, http.StatusBadRequest, "empty file not allowed")
		return
	}

	// Validate file size
	if fileHeader.Size > uploadConfig.MaxFileSize {
		httpError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file size exceeds maximum allowed size of %d MB", uploadConfig.MaxFileSize/(1024*1024)))
		return
	}

	// Sanitize and validate filename
	originalFilename := fileHeader.Filename
	if originalFilename == "" {
		httpError(w, http.StatusBadRequest, "filename is required")
		return
	}

	sanitizedFilename, err := sanitizeFilename(originalFilename)
	if err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid filename: %v", err))
		return
	}

	// Validate file extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(sanitizedFilename), "."))
	if !uploadConfig.IsExtensionAllowed(ext) {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("file type '%s' not allowed", ext))
		return
	}

	// Parse tags if provided
	var tags map[string]string
	if tagsStr := r.FormValue("tags"); tagsStr != "" {
		if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
			httpError(w, http.StatusBadRequest, "invalid tags JSON format")
			return
		}

		// Validate tags
		if err := validateTags(tags); err != nil {
			httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid tags: %v", err))
			return
		}
	}

	// Create session directory if it doesn't exist
	sessionDir := filepath.Join(drive.GetDrivePath(), "uploads", current.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		log.Printf("upload-file create dir error: session=%s err=%v", current.ID, err)
		httpError(w, http.StatusInternalServerError, "failed to create upload directory")
		return
	}

	// Generate unique filename to prevent overwrites
	finalFilename := generateUniqueFilename(sessionDir, sanitizedFilename)
	filePath := filepath.Join(sessionDir, finalFilename)

	// Save encrypted file
	uploadTime := time.Now()
	if err := upload.EncryptAndSave(file, filePath); err != nil {
		log.Printf("upload-file encrypt error: session=%s filename=%s err=%v", current.ID, finalFilename, err)
		httpError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	// Get final file size (encrypted size)
	stat, err := os.Stat(filePath)
	if err != nil {
		log.Printf("upload-file stat error: session=%s filename=%s err=%v", current.ID, finalFilename, err)
		httpError(w, http.StatusInternalServerError, "failed to get file info")
		return
	}

	// Add to manifest
	manifestRecord := manifest.FileRecord{
		SessionID:  current.ID,
		Filename:   finalFilename,
		UploadedAt: uploadTime,
		Tags:       tags,
	}
	manifest.Add(manifestRecord)

	// Create response
	response := UploadFileResponse{
		Name:       finalFilename,
		Size:       stat.Size(),
		UploadTime: uploadTime,
		Type:       ext,
		Tags:       tags,
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("upload-file encode response error: session=%s filename=%s err=%v", current.ID, finalFilename, err)
		return
	}

	log.Printf("upload-file success: session=%s filename=%s size=%d type=%s", current.ID, finalFilename, stat.Size(), ext)
}

// sanitizeFilename cleans and validates a filename for safe storage
func sanitizeFilename(filename string) (string, error) {
	// Check for path traversal attempts before processing
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return "", fmt.Errorf("filename contains unsafe characters")
	}

	// Remove any path components
	filename = filepath.Base(filename)

	// Check for empty or invalid names
	if filename == "" || filename == "." || filename == ".." {
		return "", fmt.Errorf("invalid filename")
	}

	// Check for reserved names (Windows)
	reservedNames := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
	nameWithoutExt := strings.ToUpper(strings.TrimSuffix(filename, filepath.Ext(filename)))
	for _, reserved := range reservedNames {
		if nameWithoutExt == reserved {
			return "", fmt.Errorf("filename uses reserved name")
		}
	}

	// Remove or replace problematic characters
	problematicChars := []string{"<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range problematicChars {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	// Trim whitespace and dots from ends
	filename = strings.Trim(filename, " .")

	// Ensure filename is not too long (255 bytes is typical filesystem limit)
	if len(filename) > 255 {
		ext := filepath.Ext(filename)
		nameWithoutExt := strings.TrimSuffix(filename, ext)
		maxNameLen := 255 - len(ext)
		if maxNameLen > 0 {
			filename = nameWithoutExt[:maxNameLen] + ext
		} else {
			return "", fmt.Errorf("filename too long")
		}
	}

	if filename == "" {
		return "", fmt.Errorf("filename became empty after sanitization")
	}

	return filename, nil
}

// generateUniqueFilename creates a unique filename to prevent overwrites
func generateUniqueFilename(dir, filename string) string {
	originalPath := filepath.Join(dir, filename)

	// If file doesn't exist, use original name
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return filename
	}

	// Generate unique name with timestamp suffix
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)
	timestamp := time.Now().Format("20060102_150405")

	uniqueFilename := fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext)
	uniquePath := filepath.Join(dir, uniqueFilename)

	// If still exists (very unlikely), add a counter
	counter := 1
	for {
		if _, err := os.Stat(uniquePath); os.IsNotExist(err) {
			return uniqueFilename
		}
		uniqueFilename = fmt.Sprintf("%s_%s_%d%s", nameWithoutExt, timestamp, counter, ext)
		uniquePath = filepath.Join(dir, uniqueFilename)
		counter++

		// Safety check to prevent infinite loop
		if counter > 1000 {
			break
		}
	}

	return uniqueFilename
}

// validateTags validates the tags map for security and format
func validateTags(tags map[string]string) error {
	if len(tags) > 20 {
		return fmt.Errorf("too many tags (maximum 20 allowed)")
	}

	for key, value := range tags {
		// Validate key
		if len(key) == 0 {
			return fmt.Errorf("empty tag key not allowed")
		}
		if len(key) > 50 {
			return fmt.Errorf("tag key too long (maximum 50 characters)")
		}
		if strings.Contains(key, "..") || strings.Contains(key, "/") || strings.Contains(key, "\\") {
			return fmt.Errorf("tag key contains unsafe characters")
		}

		// Validate value
		if len(value) > 200 {
			return fmt.Errorf("tag value too long (maximum 200 characters)")
		}
		if strings.Contains(value, "..") || strings.Contains(value, "/") || strings.Contains(value, "\\") {
			return fmt.Errorf("tag value contains unsafe characters")
		}
	}

	return nil
}
