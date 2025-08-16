package session

import (
	"errors"
	"familyvault/internal/core/paths"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session holds the session state with group context
type Session struct {
	ID            string    `json:"session_id"`
	GroupID       string    `json:"group_id"`
	StartedByUser string    `json:"started_by_user"`
	CreatedAt     time.Time `json:"created_at"`
	Expires       time.Time `json:"expires"`
}

// Manager handles sessions for a specific group
type Manager struct {
	mu             sync.RWMutex
	groupID        string
	currentSession *Session
	cleanupTicker  *time.Ticker
	stopCleanup    chan bool
}

// Global managers cache
var (
	managersMu sync.RWMutex
	managers   = make(map[string]*Manager)
)

const (
	// Default session timeout in minutes
	defaultSessionTimeoutMinutes = 60
	// Cleanup interval - how often to check for expired sessions
	cleanupInterval = 5 * time.Minute
)

// NewManager creates a new session manager for a group
func NewManager(groupID string) *Manager {
	return &Manager{
		groupID: groupID,
	}
}

// GetManager returns a session manager for the specified group
func GetManager(groupID string) *Manager {
	managersMu.RLock()
	if manager, exists := managers[groupID]; exists {
		managersMu.RUnlock()
		return manager
	}
	managersMu.RUnlock()

	managersMu.Lock()
	defer managersMu.Unlock()

	// Double-check after acquiring write lock
	if manager, exists := managers[groupID]; exists {
		return manager
	}

	manager := NewManager(groupID)
	managers[groupID] = manager
	return manager
}

// IsOpen returns whether there is an active (non-expired) session
func (m *Manager) IsOpen() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.currentSession == nil {
		return false
	}
	return time.Now().Before(m.currentSession.Expires)
}

// Get returns the current session or nil if none/expired
func (m *Manager) Get() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.currentSession == nil {
		return nil
	}
	if time.Now().After(m.currentSession.Expires) {
		return nil
	}
	return m.currentSession
}

// GetAllActive returns a slice of all active (non-expired) sessions
func (m *Manager) GetAllActive() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.currentSession == nil {
		return []*Session{}
	}
	if time.Now().After(m.currentSession.Expires) {
		return []*Session{}
	}
	return []*Session{m.currentSession}
}

// Open starts a new session with the given duration and user
func (m *Manager) Open(userID string, duration time.Duration) (*Session, error) {
	if duration <= 0 {
		return nil, errors.New("duration must be positive")
	}
	now := time.Now()
	newSession := &Session{
		ID:            uuid.NewString(),
		GroupID:       m.groupID,
		StartedByUser: userID,
		CreatedAt:     now,
		Expires:       now.Add(duration),
	}

	// Ensure session directories exist
	if err := paths.EnsureSessionDirectories(m.groupID, newSession.ID); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.currentSession = newSession
	m.mu.Unlock()
	return newSession, nil
}

// OpenWithDefaultTimeout opens a session with the default timeout
func (m *Manager) OpenWithDefaultTimeout(userID string) (*Session, error) {
	timeout := getSessionTimeout()
	return m.Open(userID, timeout)
}

// getSessionTimeout returns the configured session timeout from environment variable.
func getSessionTimeout() time.Duration {
	timeoutStr := os.Getenv("SESSION_TIMEOUT_MINUTES")
	if timeoutStr == "" {
		return time.Duration(defaultSessionTimeoutMinutes) * time.Minute
	}

	timeoutMinutes, err := strconv.Atoi(timeoutStr)
	if err != nil || timeoutMinutes <= 0 {
		log.Printf("Invalid SESSION_TIMEOUT_MINUTES value: %s, using default %d", timeoutStr, defaultSessionTimeoutMinutes)
		return time.Duration(defaultSessionTimeoutMinutes) * time.Minute
	}

	return time.Duration(timeoutMinutes) * time.Minute
}

// Close ends the active session, if any
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentSession = nil
}

var ErrSessionNotFound = errors.New("session not found")

// Revoke removes the session with the given ID if it exists and is not already expired
func (m *Manager) Revoke(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentSession == nil {
		return ErrSessionNotFound
	}
	if m.currentSession.ID != sessionID {
		return ErrSessionNotFound
	}
	// If already expired, treat as not found
	if time.Now().After(m.currentSession.Expires) {
		m.currentSession = nil
		return ErrSessionNotFound
	}
	// Revoke by clearing currentSession
	m.currentSession = nil
	return nil
}

// StartCleanupRoutine starts the background session cleanup goroutine
func (m *Manager) StartCleanupRoutine() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Don't start multiple cleanup routines
	if m.cleanupTicker != nil {
		return
	}

	m.cleanupTicker = time.NewTicker(cleanupInterval)
	m.stopCleanup = make(chan bool, 1)

	go func() {
		ticker := m.cleanupTicker
		stop := m.stopCleanup
		if ticker == nil || stop == nil {
			log.Printf("Session cleanup routine: ticker or stop channel is nil, exiting")
			return
		}
		log.Printf("Session cleanup routine started for group %s (interval: %v)", m.groupID, cleanupInterval)
		for {
			select {
			case <-ticker.C:
				m.cleanupExpiredSessions()
			case <-stop:
				log.Printf("Session cleanup routine stopped for group %s", m.groupID)
				return
			}
		}
	}()
}

// StopCleanupRoutine stops the background session cleanup goroutine
func (m *Manager) StopCleanupRoutine() {
	m.mu.Lock()
	ticker := m.cleanupTicker
	stop := m.stopCleanup
	m.cleanupTicker = nil
	m.stopCleanup = nil
	m.mu.Unlock()

	if ticker != nil {
		ticker.Stop()
	}
	if stop != nil {
		select {
		case stop <- true:
		default:
		}
	}
}

// cleanupExpiredSessions checks for and cleans up expired sessions
func (m *Manager) cleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentSession == nil {
		return
	}

	now := time.Now()
	if now.After(m.currentSession.Expires) {
		sessionID := m.currentSession.ID
		age := now.Sub(m.currentSession.CreatedAt)

		log.Printf("Cleaning up expired session: group=%s id=%s age=%v", m.groupID, sessionID, age)

		// Clean up session files
		sessionDir := paths.UploadsDir(m.groupID, sessionID)
		if err := os.RemoveAll(sessionDir); err != nil {
			log.Printf("Failed to cleanup session files: group=%s id=%s path=%s err=%v", m.groupID, sessionID, sessionDir, err)
		} else {
			log.Printf("Session files cleaned up: group=%s id=%s path=%s", m.groupID, sessionID, sessionDir)
		}

		// Remove session from memory
		m.currentSession = nil
		log.Printf("Expired session removed from memory: group=%s id=%s", m.groupID, sessionID)
	}
}

// Legacy functions for backward compatibility

var (
	legacyMu        sync.RWMutex
	legacySession   *Session
	legacyDrivePath string
)

// IsOpen returns whether there is an active session (legacy)
// Deprecated: Use Manager.IsOpen instead
func IsOpen() bool {
	legacyMu.RLock()
	defer legacyMu.RUnlock()
	if legacySession == nil {
		return false
	}
	return time.Now().Before(legacySession.Expires)
}

// Get returns the current session (legacy)
// Deprecated: Use Manager.Get instead
func Get() *Session {
	legacyMu.RLock()
	defer legacyMu.RUnlock()
	if legacySession == nil {
		return nil
	}
	if time.Now().After(legacySession.Expires) {
		return nil
	}
	return legacySession
}

// Open starts a new session (legacy)
// Deprecated: Use Manager.Open instead
func Open(duration time.Duration) (*Session, error) {
	if duration <= 0 {
		return nil, errors.New("duration must be positive")
	}
	now := time.Now()
	newSession := &Session{
		ID:        uuid.NewString(),
		CreatedAt: now,
		Expires:   now.Add(duration),
	}
	legacyMu.Lock()
	legacySession = newSession
	legacyMu.Unlock()
	return newSession, nil
}

// Close ends the active session (legacy)
// Deprecated: Use Manager.Close instead
func Close() {
	legacyMu.Lock()
	defer legacyMu.Unlock()
	legacySession = nil
}

// OpenWithDefaultTimeout opens a session with the default timeout (legacy)
// Deprecated: Use Manager.OpenWithDefaultTimeout instead
func OpenWithDefaultTimeout() (*Session, error) {
	timeout := getSessionTimeout()
	return Open(timeout)
}

// Revoke removes the session with the given ID (legacy)
// Deprecated: Use Manager.Revoke instead
func Revoke(sessionID string) error {
	legacyMu.Lock()
	defer legacyMu.Unlock()
	if legacySession == nil {
		return ErrSessionNotFound
	}
	if legacySession.ID != sessionID {
		return ErrSessionNotFound
	}
	// If already expired, treat as not found
	if time.Now().After(legacySession.Expires) {
		legacySession = nil
		return ErrSessionNotFound
	}
	// Revoke by clearing legacySession
	legacySession = nil
	return nil
}

// GetAllActive returns a slice of all active sessions (legacy)
// Deprecated: Use Manager.GetAllActive instead
func GetAllActive() []*Session {
	legacyMu.RLock()
	defer legacyMu.RUnlock()
	if legacySession == nil {
		return []*Session{}
	}
	if time.Now().After(legacySession.Expires) {
		return []*Session{}
	}
	return []*Session{legacySession}
}

// Legacy cleanup functions for backward compatibility

var (
	legacyCleanupTicker *time.Ticker
	legacyStopCleanup   chan bool
)

// StartCleanupRoutine starts the background session cleanup goroutine (legacy)
// Deprecated: Use Manager.StartCleanupRoutine instead
func StartCleanupRoutine() {
	legacyMu.Lock()
	defer legacyMu.Unlock()

	// Don't start multiple cleanup routines
	if legacyCleanupTicker != nil {
		return
	}

	legacyCleanupTicker = time.NewTicker(cleanupInterval)
	legacyStopCleanup = make(chan bool, 1)

	go func() {
		ticker := legacyCleanupTicker
		stop := legacyStopCleanup
		if ticker == nil || stop == nil {
			return
		}
		for {
			select {
			case <-ticker.C:
				legacyCleanupExpiredSessions()
			case <-stop:
				return
			}
		}
	}()
}

// StopCleanupRoutine stops the background session cleanup goroutine (legacy)
// Deprecated: Use Manager.StopCleanupRoutine instead
func StopCleanupRoutine() {
	legacyMu.Lock()
	ticker := legacyCleanupTicker
	stop := legacyStopCleanup
	legacyCleanupTicker = nil
	legacyStopCleanup = nil
	legacyMu.Unlock()

	if ticker != nil {
		ticker.Stop()
	}
	if stop != nil {
		select {
		case stop <- true:
		default:
		}
	}
}

// SetDrivePath sets the drive path for cleanup operations (legacy)
// Deprecated: Use paths package instead
func SetDrivePath(path string) {
	legacyMu.Lock()
	defer legacyMu.Unlock()
	legacyDrivePath = path
}

// legacyCleanupExpiredSessions checks for and cleans up expired sessions (legacy)
func legacyCleanupExpiredSessions() {
	legacyMu.Lock()
	defer legacyMu.Unlock()

	if legacySession == nil {
		return
	}

	now := time.Now()
	if now.After(legacySession.Expires) {
		sessionID := legacySession.ID

		// Clean up session files if drive path is set
		if legacyDrivePath != "" {
			sessionDir := filepath.Join(legacyDrivePath, "uploads", sessionID)
			os.RemoveAll(sessionDir)
		}

		// Remove session from memory
		legacySession = nil
	}
}

// cleanupExpiredSessions is exposed for testing (legacy)
// Deprecated: Use Manager methods instead
func cleanupExpiredSessions() {
	legacyCleanupExpiredSessions()
}
