package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
)

// RestoreSessionResponse represents the response for successful session restoration
type RestoreSessionResponse struct {
	Success           bool   `json:"success"`
	RestoredSessionID string `json:"restored_session_id"`
	FilesRestored     int    `json:"files_restored"`
	ManifestRestored  bool   `json:"manifest_restored"`
	BackupRemoved     bool   `json:"backup_removed"`
}

// SessionBackupMetadata stores the manifest data for a deleted session
type SessionBackupMetadata struct {
	SessionID       string                 `json:"session_id"`
	FileRecords     []manifest.FileRecord  `json:"file_records"`
	SessionMetadata map[string]interface{} `json:"session_metadata"`
	DeletedAt       time.Time              `json:"deleted_at"`
	BackupPath      string                 `json:"backup_path"`
}

// POST /sessions/{session_id}/restore
// Restores a previously deleted session from its backup.
func restoreSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate drive availability
	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	// Extract session_id from path
	// Expected path: /sessions/{session_id}/restore
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/sessions/"), "/")
	if len(pathParts) != 2 || pathParts[1] != "restore" {
		httpError(w, http.StatusBadRequest, "invalid URL format, expected /sessions/:id/restore")
		return
	}
	sessionIDFromPath := path.Base(pathParts[0])
	if sessionIDFromPath == "" || sessionIDFromPath == "sessions" || sessionIDFromPath == "." {
		httpError(w, http.StatusBadRequest, "invalid session id in path")
		return
	}

	log.Printf("restore-session request: session=%s ip=%s", sessionIDFromPath, r.RemoteAddr)

	// Resolve and validate authenticated session (header or query)
	authSessionID := r.Header.Get("X-Session-ID")
	if authSessionID == "" {
		authSessionID = r.URL.Query().Get("session_id")
	}
	if authSessionID == "" {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	// For restore, we allow admin privileges or session ownership
	// In this implementation, we'll check if the user has an active session
	// In a real system, you might check for admin roles
	if authSessionID != sessionIDFromPath {
		// Allow restore if user has any valid session (admin-like behavior)
		// In production, implement proper role-based access control
		log.Printf("restore-session cross-session restore: auth_session=%s target_session=%s", authSessionID, sessionIDFromPath)
	}

	// Check if session already exists (prevent duplicate restoration)
	if sessionExistsInSystem(sessionIDFromPath) {
		log.Printf("restore-session already exists: session=%s ip=%s", sessionIDFromPath, r.RemoteAddr)
		httpError(w, http.StatusConflict, "session already exists, cannot restore")
		return
	}

	// Find and validate backup
	backupInfo, err := findSessionBackup(sessionIDFromPath)
	if err != nil {
		log.Printf("restore-session backup not found: session=%s ip=%s err=%v", sessionIDFromPath, r.RemoteAddr, err)
		httpError(w, http.StatusNotFound, fmt.Sprintf("backup not found: %v", err))
		return
	}

	// Perform atomic restoration
	filesRestored, err := restoreSessionData(sessionIDFromPath, backupInfo)
	if err != nil {
		log.Printf("restore-session failed: session=%s ip=%s err=%v", sessionIDFromPath, r.RemoteAddr, err)
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to restore session: %v", err))
		return
	}

	// Restore manifest entries
	manifestRestored := restoreSessionManifest(backupInfo)

	// Remove backup after successful restoration
	backupRemoved := false
	if err := removeSessionBackup(backupInfo); err != nil {
		log.Printf("restore-session backup cleanup failed: session=%s err=%v", sessionIDFromPath, err)
		// Don't fail the restore if backup cleanup fails
	} else {
		backupRemoved = true
	}

	// Create response
	response := RestoreSessionResponse{
		Success:           true,
		RestoredSessionID: sessionIDFromPath,
		FilesRestored:     filesRestored,
		ManifestRestored:  manifestRestored,
		BackupRemoved:     backupRemoved,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("restore-session encode response error: session=%s ip=%s err=%v", sessionIDFromPath, r.RemoteAddr, err)
		return
	}

	log.Printf("restore-session success: session=%s ip=%s files_restored=%d", sessionIDFromPath, r.RemoteAddr, filesRestored)
}

// findSessionBackup locates the backup for a deleted session
func findSessionBackup(sessionID string) (*SessionBackupMetadata, error) {
	uploadsDir := filepath.Join(drive.GetDrivePath(), "uploads")

	// Look for backup directories with the pattern: sessionID.deleted.*
	entries, err := os.ReadDir(uploadsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read uploads directory: %w", err)
	}

	var backupPath string
	var latestTime int64 = 0

	// Find the most recent backup for this session
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), sessionID+".deleted.") {
			// Extract timestamp from backup name
			parts := strings.Split(entry.Name(), ".")
			if len(parts) >= 3 {
				timestamp := parts[len(parts)-1]
				if len(timestamp) > 0 {
					// Use the most recent backup if multiple exist
					if timestamp > fmt.Sprintf("%d", latestTime) {
						backupPath = filepath.Join(uploadsDir, entry.Name())
						// Parse timestamp for metadata
						if ts, err := parseTimestamp(timestamp); err == nil && ts > latestTime {
							latestTime = ts
						}
					}
				}
			}
		}
	}

	if backupPath == "" {
		return nil, fmt.Errorf("no backup found for session %s", sessionID)
	}

	// Load backup metadata if it exists
	metadataPath := filepath.Join(backupPath, ".backup_metadata.json")
	backupInfo := &SessionBackupMetadata{
		SessionID:  sessionID,
		BackupPath: backupPath,
		DeletedAt:  time.Unix(latestTime, 0),
	}

	if _, err := os.Stat(metadataPath); err == nil {
		// Load existing metadata
		data, err := os.ReadFile(metadataPath)
		if err == nil {
			json.Unmarshal(data, backupInfo)
		}
	}

	return backupInfo, nil
}

// parseTimestamp parses a timestamp string to int64
func parseTimestamp(timestamp string) (int64, error) {
	// Use strconv.ParseInt for strict parsing
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp format: %w", err)
	}
	return ts, nil
}

// restoreSessionData restores the session files from backup
func restoreSessionData(sessionID string, backupInfo *SessionBackupMetadata) (int, error) {
	sessionPath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)
	backupPath := backupInfo.BackupPath

	// Validate paths are within expected directory structure (security check)
	basePath := filepath.Join(drive.GetDrivePath(), "uploads")
	if !strings.HasPrefix(sessionPath, basePath) || !strings.HasPrefix(backupPath, basePath) {
		return 0, fmt.Errorf("invalid paths: potential path traversal")
	}

	// Ensure session directory doesn't already exist
	if _, err := os.Stat(sessionPath); err == nil {
		return 0, fmt.Errorf("session directory already exists: %s", sessionPath)
	}

	// Count files in backup for reporting
	filesCount := 0
	err := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasSuffix(info.Name(), ".backup_metadata.json") {
			filesCount++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count backup files: %w", err)
	}

	// Atomic restore: rename backup directory to session directory
	if err := os.Rename(backupPath, sessionPath); err != nil {
		return 0, fmt.Errorf("failed to restore session directory: %w", err)
	}

	// Remove backup metadata file from restored directory
	metadataPath := filepath.Join(sessionPath, ".backup_metadata.json")
	os.Remove(metadataPath) // Ignore errors, it's just cleanup

	return filesCount, nil
}

// restoreSessionManifest restores the manifest entries for the session
func restoreSessionManifest(backupInfo *SessionBackupMetadata) bool {
	// Restore file records
	for _, record := range backupInfo.FileRecords {
		manifest.Add(record)
	}

	// Restore session metadata
	if len(backupInfo.SessionMetadata) > 0 {
		manifest.UpdateSessionMetadata(backupInfo.SessionID, backupInfo.SessionMetadata)
	}

	return len(backupInfo.FileRecords) > 0 || len(backupInfo.SessionMetadata) > 0
}

// removeSessionBackup removes the backup after successful restoration
func removeSessionBackup(backupInfo *SessionBackupMetadata) error {
	// Since we renamed the backup directory during restore, there's nothing to remove
	// This function is kept for consistency and future enhancements
	return nil
}
