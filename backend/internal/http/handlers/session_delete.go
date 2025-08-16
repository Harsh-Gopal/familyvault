package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
)

// DeleteSessionResponse represents the response for successful session deletion
type DeleteSessionResponse struct {
	Success          bool   `json:"success"`
	DeletedSessionID string `json:"deleted_session_id"`
	FilesRemoved     int    `json:"files_removed"`
	ManifestRemoved  bool   `json:"manifest_removed"`
	SessionRevoked   bool   `json:"session_revoked"`
}

// DELETE /sessions/{session_id}
// Permanently deletes the specified session and all related data.
func deleteSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate drive availability
	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	// Extract session_id from path
	// Expected path: /sessions/{session_id}
	sessionIDFromPath := strings.TrimPrefix(r.URL.Path, "/sessions/")
	sessionIDFromPath = path.Base(sessionIDFromPath)
	if sessionIDFromPath == "" || sessionIDFromPath == "sessions" || sessionIDFromPath == "." {
		httpError(w, http.StatusBadRequest, "invalid session id in path")
		return
	}

	log.Printf("delete-session request: session=%s ip=%s", sessionIDFromPath, r.RemoteAddr)

	// Resolve and validate authenticated session (header or query)
	authSessionID := r.Header.Get("X-Session-ID")
	if authSessionID == "" {
		authSessionID = r.URL.Query().Get("session_id")
	}
	if authSessionID == "" {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	// Ensure user is deleting their own session
	if sessionIDFromPath != authSessionID {
		httpError(w, http.StatusForbidden, "cannot delete another user's session")
		return
	}

	// Check if session exists in manifest or file system
	sessionExists := sessionExistsInSystem(sessionIDFromPath)
	if !sessionExists {
		log.Printf("delete-session not found: session=%s ip=%s", sessionIDFromPath, r.RemoteAddr)
		httpError(w, http.StatusNotFound, "session not found")
		return
	}

	// Perform atomic deletion
	filesRemoved, backupPath, err := deleteSessionData(sessionIDFromPath)
	if err != nil {
		log.Printf("delete-session failed: session=%s ip=%s err=%v", sessionIDFromPath, r.RemoteAddr, err)
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete session: %v", err))
		return
	}

	// Save backup metadata before removing from manifest
	backupMetadata := createBackupMetadata(sessionIDFromPath, backupPath)
	saveBackupMetadata(backupMetadata)

	// Remove from manifest
	manifestRemoved := removeSessionFromManifest(sessionIDFromPath)

	// Revoke from active session if it matches
	sessionRevoked := false
	if err := session.Revoke(sessionIDFromPath); err == nil {
		sessionRevoked = true
	}

	// Create response
	response := DeleteSessionResponse{
		Success:          true,
		DeletedSessionID: sessionIDFromPath,
		FilesRemoved:     filesRemoved,
		ManifestRemoved:  manifestRemoved,
		SessionRevoked:   sessionRevoked,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("delete-session encode response error: session=%s ip=%s err=%v", sessionIDFromPath, r.RemoteAddr, err)
		return
	}

	log.Printf("delete-session success: session=%s ip=%s files_removed=%d", sessionIDFromPath, r.RemoteAddr, filesRemoved)
}

// sessionExistsInSystem checks if a session exists in manifest or file system
func sessionExistsInSystem(sessionID string) bool {
	// Check if session has files in manifest
	allRecords := manifest.List()
	for _, record := range allRecords {
		if record.SessionID == sessionID {
			return true
		}
	}

	// Check if session has metadata
	if _, exists := manifest.GetSessionMetadata(sessionID); exists {
		return true
	}

	// Check if session directory exists on disk
	sessionPath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)
	if _, err := os.Stat(sessionPath); err == nil {
		return true
	}

	return false
}

// deleteSessionData performs atomic deletion of all session data
func deleteSessionData(sessionID string) (int, string, error) {
	sessionPath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)

	// Validate path is within expected directory structure (security check)
	basePath := filepath.Join(drive.GetDrivePath(), "uploads")
	if !strings.HasPrefix(sessionPath, basePath) {
		return 0, "", fmt.Errorf("invalid session path: potential path traversal")
	}

	// Count files before deletion for reporting
	filesCount := 0
	if _, err := os.Stat(sessionPath); err == nil {
		err := filepath.Walk(sessionPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				filesCount++
			}
			return nil
		})
		if err != nil {
			return 0, "", fmt.Errorf("failed to count files: %w", err)
		}
	}

	// Create backup path for restore capability
	backupPath := sessionPath + ".deleted." + fmt.Sprintf("%d", time.Now().Unix())

	// If session directory exists, rename it to backup (atomic operation)
	if _, err := os.Stat(sessionPath); err == nil {
		if err := os.Rename(sessionPath, backupPath); err != nil {
			return 0, "", fmt.Errorf("failed to backup session directory: %w", err)
		}
		log.Printf("delete-session backup created: session=%s backup=%s", sessionID, backupPath)
		return filesCount, backupPath, nil
	}

	return filesCount, "", nil
}

// createBackupMetadata creates backup metadata for a session before deletion
func createBackupMetadata(sessionID, backupPath string) *SessionBackupMetadata {
	// Get all file records for the session
	allRecords := manifest.List()
	var fileRecords []manifest.FileRecord
	for _, record := range allRecords {
		if record.SessionID == sessionID {
			fileRecords = append(fileRecords, record)
		}
	}

	// Get session metadata
	sessionMeta, _ := manifest.GetSessionMetadata(sessionID)

	return &SessionBackupMetadata{
		SessionID:       sessionID,
		FileRecords:     fileRecords,
		SessionMetadata: sessionMeta.Metadata,
		DeletedAt:       time.Now(),
		BackupPath:      backupPath,
	}
}

// saveBackupMetadata saves backup metadata to the backup directory
func saveBackupMetadata(backupInfo *SessionBackupMetadata) error {
	if backupInfo.BackupPath == "" {
		return nil // No backup path, skip saving metadata
	}

	metadataPath := filepath.Join(backupInfo.BackupPath, ".backup_metadata.json")
	data, err := json.Marshal(backupInfo)
	if err != nil {
		log.Printf("Failed to marshal backup metadata: %v", err)
		return err
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		log.Printf("Failed to save backup metadata: %v", err)
		return err
	}

	return nil
}

// removeSessionFromManifest removes all manifest entries for a session
func removeSessionFromManifest(sessionID string) bool {
	// Remove all file records for the session
	filesRemoved := manifest.RemoveAllForSession(sessionID)

	// Remove session metadata
	manifest.ClearSessionMetadata(sessionID)

	return filesRemoved > 0
}
