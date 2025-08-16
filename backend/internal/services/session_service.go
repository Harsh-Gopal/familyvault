package services

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"familyvault/internal/storage"
)

// SessionService provides business logic for session operations
type SessionService struct {
	storage *storage.SessionStorage
}

// NewSessionService creates a new session service
func NewSessionService() *SessionService {
	return &SessionService{
		storage: storage.NewSessionStorage(),
	}
}

// CreateSession creates a new session with validation
func (s *SessionService) CreateSession(name string, tags []string) (*storage.SessionMetadata, error) {
	// Validate name
	if name == "" {
		name = fmt.Sprintf("Session %d", len(name))
	}

	// Sanitize name
	name = sanitizeString(name)

	// Validate tags
	var sanitizedTags []string
	for _, tag := range tags {
		if sanitized := sanitizeString(tag); sanitized != "" {
			sanitizedTags = append(sanitizedTags, sanitized)
		}
	}

	return s.storage.CreateSession(name, sanitizedTags)
}

// GetSession retrieves session metadata
func (s *SessionService) GetSession(sessionID string) (*storage.SessionMetadata, error) {
	if !isValidSessionID(sessionID) {
		return nil, fmt.Errorf("invalid session ID format")
	}

	return s.storage.GetSession(sessionID)
}

// ListSessions lists sessions with filtering and pagination
func (s *SessionService) ListSessions(status string, search string, limit, offset int) ([]*storage.SessionMetadata, int, error) {
	// Validate and convert status
	var sessionStatus storage.SessionStatus
	if status != "" {
		switch status {
		case "active":
			sessionStatus = storage.StatusActive
		case "completed":
			sessionStatus = storage.StatusCompleted
		case "deleted":
			sessionStatus = storage.StatusDeleted
		default:
			return nil, 0, fmt.Errorf("invalid status: %s", status)
		}
	}

	// Validate pagination
	if limit <= 0 || limit > 1000 {
		limit = 50 // Default limit
	}
	if offset < 0 {
		offset = 0
	}

	// Sanitize search
	search = sanitizeString(search)

	return s.storage.ListSessions(sessionStatus, search, limit, offset)
}

// UpdateSession updates session metadata
func (s *SessionService) UpdateSession(sessionID, name string, tags []string) error {
	if !isValidSessionID(sessionID) {
		return fmt.Errorf("invalid session ID format")
	}

	// Sanitize inputs
	if name != "" {
		name = sanitizeString(name)
	}

	var sanitizedTags []string
	for _, tag := range tags {
		if sanitized := sanitizeString(tag); sanitized != "" {
			sanitizedTags = append(sanitizedTags, sanitized)
		}
	}

	return s.storage.UpdateSession(sessionID, name, sanitizedTags)
}

// DeleteSession soft deletes a session
func (s *SessionService) DeleteSession(sessionID string) error {
	if !isValidSessionID(sessionID) {
		return fmt.Errorf("invalid session ID format")
	}

	return s.storage.DeleteSession(sessionID)
}

// UploadFile uploads a file to a session
func (s *SessionService) UploadFile(sessionID string, fileHeader *multipart.FileHeader) error {
	if !isValidSessionID(sessionID) {
		return fmt.Errorf("invalid session ID format")
	}

	// Validate file
	if fileHeader.Size > 100*1024*1024 { // 100MB limit for simple upload
		return fmt.Errorf("file too large: %d bytes (max 100MB)", fileHeader.Size)
	}

	// Sanitize filename
	filename := sanitizeFilename(fileHeader.Filename)
	if filename == "" {
		return fmt.Errorf("invalid filename")
	}

	// Get session to ensure it exists and is active
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	if session.Status != storage.StatusActive {
		return fmt.Errorf("cannot upload to inactive session")
	}

	// Open uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Create destination file
	destPath := filepath.Join(session.Path, filename)
	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy file content
	_, err = io.Copy(dst, src)
	if err != nil {
		os.Remove(destPath) // Cleanup on error
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// DeleteFile deletes a file from a session
func (s *SessionService) DeleteFile(sessionID, filename string) error {
	if !isValidSessionID(sessionID) {
		return fmt.Errorf("invalid session ID format")
	}

	// Sanitize filename
	filename = sanitizeFilename(filename)
	if filename == "" {
		return fmt.Errorf("invalid filename")
	}

	// Get session
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Construct file path
	filePath := filepath.Join(session.Path, filename)

	// Validate file exists and is within session directory
	if !strings.HasPrefix(filePath, session.Path) {
		return fmt.Errorf("invalid file path")
	}

	// Delete file
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found")
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// sanitizeString sanitizes a string for safe use
func sanitizeString(s string) string {
	// Remove dangerous characters
	s = strings.ReplaceAll(s, "..", "")
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "\\", "")
	s = strings.ReplaceAll(s, "\x00", "")

	// Trim whitespace
	s = strings.TrimSpace(s)

	// Limit length
	if len(s) > 255 {
		s = s[:255]
	}

	return s
}

// sanitizeFilename sanitizes a filename
func sanitizeFilename(filename string) string {
	// Remove path separators and dangerous characters
	filename = filepath.Base(filename)
	filename = strings.ReplaceAll(filename, "..", "")
	filename = strings.ReplaceAll(filename, "\x00", "")

	// Trim whitespace
	filename = strings.TrimSpace(filename)

	// Ensure it's not empty or just dots
	if filename == "" || filename == "." || filename == ".." {
		return ""
	}

	return filename
}

// isValidSessionID validates session ID format
func isValidSessionID(sessionID string) bool {
	if sessionID == "" {
		return false
	}

	// Check for path traversal attempts
	if strings.Contains(sessionID, "..") || strings.Contains(sessionID, "/") || strings.Contains(sessionID, "\\") {
		return false
	}

	// Check length
	if len(sessionID) > 100 {
		return false
	}

	return true
}
