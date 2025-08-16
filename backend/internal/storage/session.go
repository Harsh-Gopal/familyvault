package storage

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"familyvault/internal/core/drive"
)

// SessionStatus represents the status of a session
type SessionStatus string

const (
	StatusActive    SessionStatus = "active"
	StatusCompleted SessionStatus = "completed"
	StatusDeleted   SessionStatus = "deleted"
)

// SessionMetadata represents session metadata
type SessionMetadata struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Status       SessionStatus `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
	LastActivity time.Time     `json:"last_activity"`
	FileCount    int           `json:"file_count"`
	TotalSize    int64         `json:"total_size"`
	Tags         []string      `json:"tags"`
	Path         string        `json:"path,omitempty"`
}

// SessionStorage handles session storage operations
type SessionStorage struct {
	basePath string
}

// NewSessionStorage creates a new session storage
func NewSessionStorage() *SessionStorage {
	return &SessionStorage{
		basePath: drive.GetDrivePath(),
	}
}

// CreateSession creates a new session
func (s *SessionStorage) CreateSession(name string, tags []string) (*SessionMetadata, error) {
	sessionID := generateSessionID()
	sessionPath := filepath.Join(s.basePath, "uploads", sessionID)

	// Create session directory
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	metadata := &SessionMetadata{
		ID:           sessionID,
		Name:         name,
		Status:       StatusActive,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		FileCount:    0,
		TotalSize:    0,
		Tags:         tags,
		Path:         sessionPath,
	}

	// Save metadata
	if err := s.saveMetadata(metadata); err != nil {
		os.RemoveAll(sessionPath) // Cleanup on error
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return metadata, nil
}

// GetSession retrieves session metadata
func (s *SessionStorage) GetSession(sessionID string) (*SessionMetadata, error) {
	// Try active session first
	if metadata, err := s.getActiveSession(sessionID); err == nil {
		return metadata, nil
	}

	// Try backup location
	return s.getBackupSession(sessionID)
}

// ListSessions lists all sessions with optional filtering
func (s *SessionStorage) ListSessions(status SessionStatus, search string, limit, offset int) ([]*SessionMetadata, int, error) {
	var allSessions []*SessionMetadata

	// Get active sessions
	activeSessions, err := s.listActiveSessions()
	if err == nil {
		allSessions = append(allSessions, activeSessions...)
	}

	// Get backup sessions
	backupSessions, err := s.listBackupSessions()
	if err == nil {
		allSessions = append(allSessions, backupSessions...)
	}

	// Apply filters
	var filteredSessions []*SessionMetadata
	for _, session := range allSessions {
		// Status filter
		if status != "" && session.Status != status {
			continue
		}

		// Search filter
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(session.ID), searchLower) &&
				!strings.Contains(strings.ToLower(session.Name), searchLower) {
				continue
			}
		}

		filteredSessions = append(filteredSessions, session)
	}

	// Sort by last activity (newest first)
	sort.Slice(filteredSessions, func(i, j int) bool {
		return filteredSessions[i].LastActivity.After(filteredSessions[j].LastActivity)
	})

	total := len(filteredSessions)

	// Apply pagination
	start := offset
	end := offset + limit
	if start >= len(filteredSessions) {
		return []*SessionMetadata{}, total, nil
	}
	if end > len(filteredSessions) {
		end = len(filteredSessions)
	}

	return filteredSessions[start:end], total, nil
}

// UpdateSession updates session metadata
func (s *SessionStorage) UpdateSession(sessionID string, name string, tags []string) error {
	metadata, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	if name != "" {
		metadata.Name = name
	}
	if tags != nil {
		metadata.Tags = tags
	}
	metadata.LastActivity = time.Now()

	return s.saveMetadata(metadata)
}

// DeleteSession moves session to backup (soft delete)
func (s *SessionStorage) DeleteSession(sessionID string) error {
	metadata, err := s.getActiveSession(sessionID)
	if err != nil {
		return err
	}

	// Create backup directory
	backupDir := filepath.Join(s.basePath, "backup", time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Move session to backup
	sourcePath := filepath.Join(s.basePath, "uploads", sessionID)
	backupPath := filepath.Join(backupDir, sessionID)

	if err := os.Rename(sourcePath, backupPath); err != nil {
		return fmt.Errorf("failed to move session to backup: %w", err)
	}

	// Update metadata
	metadata.Status = StatusDeleted
	metadata.LastActivity = time.Now()
	metadata.Path = backupPath

	return s.saveMetadataToPath(metadata, backupPath)
}

// getActiveSession retrieves active session metadata
func (s *SessionStorage) getActiveSession(sessionID string) (*SessionMetadata, error) {
	sessionPath := filepath.Join(s.basePath, "uploads", sessionID)
	return s.loadMetadataFromPath(sessionPath)
}

// getBackupSession retrieves backup session metadata
func (s *SessionStorage) getBackupSession(sessionID string) (*SessionMetadata, error) {
	backupBasePath := filepath.Join(s.basePath, "backup")

	var foundPath string
	var latestTime time.Time

	// Walk through backup directories to find the session
	err := filepath.WalkDir(backupBasePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == sessionID {
			// Get the modification time of this backup
			info, err := d.Info()
			if err != nil {
				return err
			}

			// Keep track of the most recent backup
			if foundPath == "" || info.ModTime().After(latestTime) {
				foundPath = path
				latestTime = info.ModTime()
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if foundPath == "" {
		return nil, os.ErrNotExist
	}

	return s.loadMetadataFromPath(foundPath)
}

// listActiveSessions lists all active sessions
func (s *SessionStorage) listActiveSessions() ([]*SessionMetadata, error) {
	uploadsPath := filepath.Join(s.basePath, "uploads")

	if _, err := os.Stat(uploadsPath); os.IsNotExist(err) {
		return []*SessionMetadata{}, nil
	}

	var sessions []*SessionMetadata

	entries, err := os.ReadDir(uploadsPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			sessionPath := filepath.Join(uploadsPath, entry.Name())
			if metadata, err := s.loadMetadataFromPath(sessionPath); err == nil {
				sessions = append(sessions, metadata)
			}
		}
	}

	return sessions, nil
}

// listBackupSessions lists all backup sessions
func (s *SessionStorage) listBackupSessions() ([]*SessionMetadata, error) {
	backupPath := filepath.Join(s.basePath, "backup")

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return []*SessionMetadata{}, nil
	}

	var sessions []*SessionMetadata

	err := filepath.WalkDir(backupPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && strings.Contains(path, "/backup/") && path != backupPath {
			// Check if this looks like a session directory
			if metadata, err := s.loadMetadataFromPath(path); err == nil {
				sessions = append(sessions, metadata)
			}
		}

		return nil
	})

	return sessions, err
}

// saveMetadata saves metadata to session directory
func (s *SessionStorage) saveMetadata(metadata *SessionMetadata) error {
	return s.saveMetadataToPath(metadata, metadata.Path)
}

// saveMetadataToPath saves metadata to specific path
func (s *SessionStorage) saveMetadataToPath(metadata *SessionMetadata, sessionPath string) error {
	metadataPath := filepath.Join(sessionPath, "metadata.json")

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

// loadMetadataFromPath loads metadata from specific path
func (s *SessionStorage) loadMetadataFromPath(sessionPath string) (*SessionMetadata, error) {
	metadataPath := filepath.Join(sessionPath, "metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		// If metadata file doesn't exist, create basic metadata from directory
		if os.IsNotExist(err) {
			return s.createMetadataFromDirectory(sessionPath)
		}
		return nil, err
	}

	var metadata SessionMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	// Update file count and size
	s.updateMetadataStats(&metadata, sessionPath)

	return &metadata, nil
}

// createMetadataFromDirectory creates metadata from directory info
func (s *SessionStorage) createMetadataFromDirectory(sessionPath string) (*SessionMetadata, error) {
	info, err := os.Stat(sessionPath)
	if err != nil {
		return nil, err
	}

	sessionID := filepath.Base(sessionPath)
	status := StatusActive
	if strings.Contains(sessionPath, "/backup/") {
		status = StatusDeleted
	}

	metadata := &SessionMetadata{
		ID:           sessionID,
		Name:         sessionID,
		Status:       status,
		CreatedAt:    info.ModTime(),
		LastActivity: info.ModTime(),
		FileCount:    0,
		TotalSize:    0,
		Tags:         []string{},
		Path:         sessionPath,
	}

	s.updateMetadataStats(metadata, sessionPath)
	return metadata, nil
}

// updateMetadataStats updates file count and total size
func (s *SessionStorage) updateMetadataStats(metadata *SessionMetadata, sessionPath string) {
	var fileCount int
	var totalSize int64

	filepath.WalkDir(sessionPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && !strings.HasSuffix(d.Name(), "metadata.json") {
			fileCount++
			if info, err := d.Info(); err == nil {
				totalSize += info.Size()
			}
		}

		return nil
	})

	metadata.FileCount = fileCount
	metadata.TotalSize = totalSize
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}
