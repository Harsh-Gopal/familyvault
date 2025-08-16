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
)

// SessionStatus represents the possible states of a session
type SessionStatus string

const (
	StatusActive            SessionStatus = "active"
	StatusDeletedWithBackup SessionStatus = "deleted_with_backup"
	StatusDeletedNoBackup   SessionStatus = "deleted_no_backup"
)

// BackupInfo contains information about a session backup
type BackupInfo struct {
	BackupPath   string    `json:"backup_path"`
	DeletedAt    time.Time `json:"deleted_at"`
	BackupExists bool      `json:"backup_exists"`
}

// SessionStatusResponse represents the response for session status requests
type SessionStatusResponse struct {
	SessionID       string                 `json:"session_id"`
	Status          SessionStatus          `json:"status"`
	FilesCount      int                    `json:"files_count"`
	TotalSizeBytes  int64                  `json:"total_size_bytes"`
	CreatedAt       *time.Time             `json:"created_at,omitempty"`
	LastUpdated     *time.Time             `json:"last_updated,omitempty"`
	ProjectName     string                 `json:"project_name,omitempty"`
	CreatedBy       string                 `json:"created_by,omitempty"`
	Tags            map[string]string      `json:"tags,omitempty"`
	SessionMetadata map[string]interface{} `json:"session_metadata,omitempty"`
	BackupInfo      *BackupInfo            `json:"backup_info,omitempty"`
}

// GET /sessions/{session_id}/status
// Retrieves the current status and metadata of a specific session.
func sessionStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate drive availability
	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	// Extract session_id from path
	// Expected path: /sessions/{session_id}/status
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/sessions/"), "/")
	if len(pathParts) != 2 || pathParts[1] != "status" {
		httpError(w, http.StatusBadRequest, "invalid URL format, expected /sessions/:id/status")
		return
	}
	sessionIDFromPath := path.Base(pathParts[0])
	if sessionIDFromPath == "" || sessionIDFromPath == "sessions" || sessionIDFromPath == "." {
		httpError(w, http.StatusBadRequest, "invalid session id in path")
		return
	}

	log.Printf("session-status request: session=%s ip=%s", sessionIDFromPath, r.RemoteAddr)

	// Resolve and validate authenticated session (header or query)
	authSessionID := r.Header.Get("X-Session-ID")
	if authSessionID == "" {
		authSessionID = r.URL.Query().Get("session_id")
	}
	if authSessionID == "" {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	// For status, we allow viewing any session if user has valid authentication
	// In production, implement proper role-based access control
	log.Printf("session-status auth: auth_session=%s target_session=%s", authSessionID, sessionIDFromPath)

	// Determine session status and gather information
	statusInfo, err := getSessionStatus(sessionIDFromPath)
	if err != nil {
		log.Printf("session-status error: session=%s ip=%s err=%v", sessionIDFromPath, r.RemoteAddr, err)
		httpError(w, http.StatusNotFound, "session not found")
		return
	}

	// If session has no backup and is not active, return 404
	if statusInfo.Status == StatusDeletedNoBackup {
		log.Printf("session-status not found: session=%s ip=%s", sessionIDFromPath, r.RemoteAddr)
		httpError(w, http.StatusNotFound, "session not found")
		return
	}

	// Create response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(statusInfo); err != nil {
		log.Printf("session-status encode response error: session=%s ip=%s err=%v", sessionIDFromPath, r.RemoteAddr, err)
		return
	}

	log.Printf("session-status success: session=%s ip=%s status=%s files=%d", sessionIDFromPath, r.RemoteAddr, statusInfo.Status, statusInfo.FilesCount)
}

// getSessionStatus determines the status of a session and gathers all relevant information
func getSessionStatus(sessionID string) (*SessionStatusResponse, error) {
	response := &SessionStatusResponse{
		SessionID: sessionID,
	}

	// Check if session is active
	if isSessionActive(sessionID) {
		response.Status = StatusActive
		if err := populateActiveSessionInfo(sessionID, response); err != nil {
			return nil, err
		}
	} else {
		// Check for backup
		backupInfo, err := findSessionBackupInfo(sessionID)
		if err == nil && backupInfo != nil {
			response.Status = StatusDeletedWithBackup
			response.BackupInfo = backupInfo
			if err := populateBackupSessionInfo(sessionID, backupInfo, response); err != nil {
				return nil, err
			}
		} else {
			response.Status = StatusDeletedNoBackup
		}
	}

	return response, nil
}

// isSessionActive checks if a session is currently active
func isSessionActive(sessionID string) bool {
	// Check if session directory exists
	sessionPath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)
	if _, err := os.Stat(sessionPath); err == nil {
		return true
	}

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

	return false
}

// populateActiveSessionInfo populates response with information for an active session
func populateActiveSessionInfo(sessionID string, response *SessionStatusResponse) error {
	// Get file records from manifest
	allRecords := manifest.List()
	var sessionRecords []manifest.FileRecord
	var earliestCreated *time.Time
	var latestUpdated *time.Time

	for _, record := range allRecords {
		if record.SessionID == sessionID {
			sessionRecords = append(sessionRecords, record)

			// Track creation and update times
			if earliestCreated == nil || record.UploadedAt.Before(*earliestCreated) {
				earliestCreated = &record.UploadedAt
			}
			if latestUpdated == nil || record.UploadedAt.After(*latestUpdated) {
				latestUpdated = &record.UploadedAt
			}
		}
	}

	response.FilesCount = len(sessionRecords)
	response.CreatedAt = earliestCreated
	response.LastUpdated = latestUpdated

	// Calculate total size by examining files on disk
	totalSize, err := calculateSessionSize(sessionID)
	if err != nil {
		log.Printf("session-status size calculation error: session=%s err=%v", sessionID, err)
		// Don't fail the request, just set size to 0
		totalSize = 0
	}
	response.TotalSizeBytes = totalSize

	// Get session metadata
	if sessionMeta, exists := manifest.GetSessionMetadata(sessionID); exists {
		response.SessionMetadata = sessionMeta.Metadata

		// Extract common fields from metadata
		if projectName, ok := sessionMeta.Metadata["project_name"].(string); ok {
			response.ProjectName = projectName
		}
		if createdBy, ok := sessionMeta.Metadata["created_by"].(string); ok {
			response.CreatedBy = createdBy
		}
	}

	// Aggregate tags from all files
	tags := make(map[string]string)
	for _, record := range sessionRecords {
		for key, value := range record.Tags {
			tags[key] = value
		}
	}
	if len(tags) > 0 {
		response.Tags = tags
	}

	return nil
}

// populateBackupSessionInfo populates response with information for a backed up session
func populateBackupSessionInfo(sessionID string, backupInfo *BackupInfo, response *SessionStatusResponse) error {
	// Try to load backup metadata
	metadataPath := filepath.Join(backupInfo.BackupPath, ".backup_metadata.json")
	if data, err := os.ReadFile(metadataPath); err == nil {
		var backupMetadata SessionBackupMetadata
		if json.Unmarshal(data, &backupMetadata) == nil {
			response.FilesCount = len(backupMetadata.FileRecords)
			response.SessionMetadata = backupMetadata.SessionMetadata

			// Extract common fields from metadata
			if projectName, ok := backupMetadata.SessionMetadata["project_name"].(string); ok {
				response.ProjectName = projectName
			}
			if createdBy, ok := backupMetadata.SessionMetadata["created_by"].(string); ok {
				response.CreatedBy = createdBy
			}

			// Find earliest and latest timestamps
			var earliestCreated *time.Time
			var latestUpdated *time.Time
			tags := make(map[string]string)

			for _, record := range backupMetadata.FileRecords {
				if earliestCreated == nil || record.UploadedAt.Before(*earliestCreated) {
					earliestCreated = &record.UploadedAt
				}
				if latestUpdated == nil || record.UploadedAt.After(*latestUpdated) {
					latestUpdated = &record.UploadedAt
				}

				// Aggregate tags
				for key, value := range record.Tags {
					tags[key] = value
				}
			}

			response.CreatedAt = earliestCreated
			response.LastUpdated = latestUpdated
			if len(tags) > 0 {
				response.Tags = tags
			}

			// Calculate backup size
			totalSize, err := calculateBackupSize(backupInfo.BackupPath)
			if err != nil {
				log.Printf("session-status backup size calculation error: session=%s err=%v", sessionID, err)
				totalSize = 0
			}
			response.TotalSizeBytes = totalSize
		}
	}

	return nil
}

// findSessionBackupInfo locates backup information for a session
func findSessionBackupInfo(sessionID string) (*BackupInfo, error) {
	uploadsDir := filepath.Join(drive.GetDrivePath(), "uploads")

	entries, err := os.ReadDir(uploadsDir)
	if err != nil {
		return nil, err
	}

	var backupPath string
	var latestTime int64 = 0

	// Find the most recent backup for this session
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), sessionID+".deleted.") {
			parts := strings.Split(entry.Name(), ".")
			if len(parts) >= 3 {
				timestamp := parts[len(parts)-1]
				if ts, err := parseTimestamp(timestamp); err == nil && ts > latestTime {
					latestTime = ts
					backupPath = filepath.Join(uploadsDir, entry.Name())
				}
			}
		}
	}

	if backupPath == "" {
		return nil, fmt.Errorf("no backup found for session %s", sessionID)
	}

	// Verify backup still exists
	backupExists := true
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		backupExists = false
	}

	return &BackupInfo{
		BackupPath:   backupPath,
		DeletedAt:    time.Unix(latestTime, 0),
		BackupExists: backupExists,
	}, nil
}

// calculateSessionSize calculates the total size of all files in an active session
func calculateSessionSize(sessionID string) (int64, error) {
	sessionPath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)

	var totalSize int64
	err := filepath.Walk(sessionPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	return totalSize, err
}

// calculateBackupSize calculates the total size of all files in a backup
func calculateBackupSize(backupPath string) (int64, error) {
	var totalSize int64
	err := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasSuffix(info.Name(), ".backup_metadata.json") {
			totalSize += info.Size()
		}
		return nil
	})

	return totalSize, err
}
