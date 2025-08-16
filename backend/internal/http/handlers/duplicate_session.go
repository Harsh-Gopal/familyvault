package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"

	"github.com/google/uuid"
)

// DuplicateSessionRequest represents the request for duplicating a session
type DuplicateSessionRequest struct {
	SourceSessionID string `json:"source_session_id"`
}

// DuplicateSessionResponse represents the response for successful session duplication
type DuplicateSessionResponse struct {
	Success         bool                   `json:"success"`
	NewSessionID    string                 `json:"new_session_id"`
	SourceSessionID string                 `json:"source_session_id"`
	FilesCount      int                    `json:"files_count"`
	SessionMetadata map[string]interface{} `json:"session_metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
}

// POST /sessions/:id/duplicate
// Creates a complete duplicate of the specified session, including all files and metadata
func duplicateSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate drive availability
	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	// Extract session ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/sessions/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "duplicate" {
		httpError(w, http.StatusBadRequest, "invalid URL format, expected /sessions/:id/duplicate")
		return
	}
	sourceSessionID := parts[0]

	if sourceSessionID == "" {
		httpError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	log.Printf("duplicate-session request: source_session=%s ip=%s", sourceSessionID, r.RemoteAddr)

	// Check if source session directory exists
	sourcePath := filepath.Join(drive.GetDrivePath(), "uploads", sourceSessionID)
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		log.Printf("duplicate-session source not found: source_session=%s path=%s", sourceSessionID, sourcePath)
		httpError(w, http.StatusNotFound, "source session not found")
		return
	}

	// Generate new session ID
	newSessionID := uuid.NewString()
	newPath := filepath.Join(drive.GetDrivePath(), "uploads", newSessionID)

	// Create new session directory
	if err := os.MkdirAll(newPath, 0755); err != nil {
		log.Printf("duplicate-session failed to create directory: new_session=%s path=%s err=%v", newSessionID, newPath, err)
		httpError(w, http.StatusInternalServerError, "failed to create new session directory")
		return
	}

	// Copy all files from source to destination
	filesCount, err := copySessionFiles(sourcePath, newPath)
	if err != nil {
		log.Printf("duplicate-session failed to copy files: source_session=%s new_session=%s err=%v", sourceSessionID, newSessionID, err)
		// Clean up the partially created directory
		os.RemoveAll(newPath)
		httpError(w, http.StatusInternalServerError, "failed to copy session files")
		return
	}

	// Duplicate manifest entries
	err = duplicateManifestEntries(sourceSessionID, newSessionID)
	if err != nil {
		log.Printf("duplicate-session failed to duplicate manifest: source_session=%s new_session=%s err=%v", sourceSessionID, newSessionID, err)
		// Clean up the created directory and files
		os.RemoveAll(newPath)
		httpError(w, http.StatusInternalServerError, "failed to duplicate session metadata")
		return
	}

	// Get session metadata for response
	sessionMeta, _ := manifest.GetSessionMetadata(newSessionID)

	// Create response
	response := DuplicateSessionResponse{
		Success:         true,
		NewSessionID:    newSessionID,
		SourceSessionID: sourceSessionID,
		FilesCount:      filesCount,
		SessionMetadata: sessionMeta.Metadata,
		CreatedAt:       time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("duplicate-session encode response error: source_session=%s new_session=%s err=%v", sourceSessionID, newSessionID, err)
		return
	}

	log.Printf("duplicate-session success: source_session=%s new_session=%s files_count=%d", sourceSessionID, newSessionID, filesCount)
}

// copySessionFiles recursively copies all files from source to destination directory
func copySessionFiles(sourcePath, destPath string) (int, error) {
	filesCount := 0

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path from source
		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Create destination path
		destFile := filepath.Join(destPath, relPath)

		// Create destination directory if needed
		destDir := filepath.Dir(destFile)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
		}

		// Copy file
		if err := copyFile(path, destFile); err != nil {
			return fmt.Errorf("failed to copy file %s to %s: %w", path, destFile, err)
		}

		filesCount++
		return nil
	})

	return filesCount, err
}

// copyFile copies a single file from source to destination
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy file contents
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}

// duplicateManifestEntries duplicates all manifest entries from source session to new session
func duplicateManifestEntries(sourceSessionID, newSessionID string) error {
	// Get all file records for the source session
	allRecords := manifest.List()
	var sourceRecords []manifest.FileRecord

	for _, record := range allRecords {
		if record.SessionID == sourceSessionID {
			sourceRecords = append(sourceRecords, record)
		}
	}

	// Duplicate each file record with new session ID
	for _, record := range sourceRecords {
		newRecord := manifest.FileRecord{
			SessionID:  newSessionID,
			Filename:   record.Filename,
			UploadedAt: time.Now(), // Update timestamp for the duplicate
			Tags:       make(map[string]string),
		}

		// Copy tags
		if record.Tags != nil {
			for key, value := range record.Tags {
				newRecord.Tags[key] = value
			}
		}

		// Add the new record to manifest
		manifest.Add(newRecord)
	}

	// Duplicate session metadata if it exists
	if sourceMeta, exists := manifest.GetSessionMetadata(sourceSessionID); exists {
		// Create new session metadata with copied values
		newMetadata := make(map[string]interface{})
		for key, value := range sourceMeta.Metadata {
			newMetadata[key] = value
		}

		// Update session metadata for the new session
		manifest.UpdateSessionMetadata(newSessionID, newMetadata)
	}

	return nil
}
