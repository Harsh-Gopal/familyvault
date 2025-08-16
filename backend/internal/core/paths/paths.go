package paths

import (
	"os"
	"path/filepath"

	"familyvault/internal/core/drive"
)

// GroupRoot returns the root directory for a group
func GroupRoot(groupID string) string {
	return filepath.Join(drive.GetDrivePath(), "groups", groupID)
}

// UploadsDir returns the uploads directory for a group and session
func UploadsDir(groupID, sessionID string) string {
	return filepath.Join(GroupRoot(groupID), "uploads", sessionID)
}

// UserUploadsDir returns the user-specific uploads directory
func UserUploadsDir(groupID, sessionID, userID string) string {
	return filepath.Join(UploadsDir(groupID, sessionID), userID)
}

// BackupsDir returns the backups directory for a group
func BackupsDir(groupID string) string {
	return filepath.Join(GroupRoot(groupID), "backups")
}

// ManifestsDir returns the manifests directory for a group
func ManifestsDir(groupID string) string {
	return filepath.Join(GroupRoot(groupID), "manifests")
}

// LogsDir returns the logs directory for a group
func LogsDir(groupID string) string {
	return filepath.Join(GroupRoot(groupID), "logs")
}

// SessionLogFile returns the log file path for a session
func SessionLogFile(groupID, sessionID string) string {
	return filepath.Join(UploadsDir(groupID, sessionID), "session.log")
}

// SessionMetricsDir returns the metrics directory for a session
func SessionMetricsDir(groupID, sessionID string) string {
	return filepath.Join(UploadsDir(groupID, sessionID), "metrics")
}

// SessionArtifactsDir returns the artifacts directory for a session
func SessionArtifactsDir(groupID, sessionID string) string {
	return filepath.Join(UploadsDir(groupID, sessionID), "artifacts")
}

// AuditLogFile returns the audit log file path for a group
func AuditLogFile(groupID string) string {
	return filepath.Join(LogsDir(groupID), "audit.log")
}

// EnsureGroupDirectories creates all necessary directories for a group
func EnsureGroupDirectories(groupID string) error {
	dirs := []string{
		GroupRoot(groupID),
		filepath.Join(GroupRoot(groupID), "uploads"),
		BackupsDir(groupID),
		ManifestsDir(groupID),
		LogsDir(groupID),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// EnsureSessionDirectories creates all necessary directories for a session
func EnsureSessionDirectories(groupID, sessionID string) error {
	dirs := []string{
		UploadsDir(groupID, sessionID),
		SessionMetricsDir(groupID, sessionID),
		SessionArtifactsDir(groupID, sessionID),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// EnsureUserSessionDirectories creates user-specific directories for a session
func EnsureUserSessionDirectories(groupID, sessionID, userID string) error {
	userDir := UserUploadsDir(groupID, sessionID, userID)
	return os.MkdirAll(userDir, 0755)
}
