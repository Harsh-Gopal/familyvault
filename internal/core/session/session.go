package session

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session holds the in-memory session state.
// Only a single active session is supported at a time in this initial version.

type Session struct {
	ID        string    `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
	Expires   time.Time `json:"expires"`
}

var (
	mutex          sync.RWMutex
	currentSession *Session
	cleanupTicker  *time.Ticker
	stopCleanup    chan bool
	drivePath      string
)

const (
	// Default session timeout in minutes
	defaultSessionTimeoutMinutes = 60
	// Cleanup interval - how often to check for expired sessions
	cleanupInterval = 5 * time.Minute
)

// IsOpen returns whether there is an active (non-expired) session.
func IsOpen() bool {
	mutex.RLock()
	defer mutex.RUnlock()
	if currentSession == nil {
		return false
	}
	return time.Now().Before(currentSession.Expires)
}

// Get returns the current session or nil if none/expired.
func Get() *Session {
	mutex.RLock()
	defer mutex.RUnlock()
	if currentSession == nil {
		return nil
	}
	if time.Now().After(currentSession.Expires) {
		return nil
	}
	return currentSession
}

// GetAllActive returns a slice of all active (non-expired) sessions.
// In this initial implementation, at most one session can be active.
func GetAllActive() []*Session {
	mutex.RLock()
	defer mutex.RUnlock()
	if currentSession == nil {
		return []*Session{}
	}
	if time.Now().After(currentSession.Expires) {
		return []*Session{}
	}
	return []*Session{currentSession}
}

// Open starts a new session with the given duration. If a session is already active,
// it will be replaced by the new one.
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
	mutex.Lock()
	currentSession = newSession
	mutex.Unlock()
	return newSession, nil
}

// OpenWithDefaultTimeout opens a session with the default timeout from environment variable.
func OpenWithDefaultTimeout() (*Session, error) {
	timeout := getSessionTimeout()
	return Open(timeout)
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

// Close ends the active session, if any.
func Close() {
	mutex.Lock()
	defer mutex.Unlock()
	currentSession = nil
}

var ErrSessionNotFound = errors.New("session not found")

// Revoke removes the session with the given ID if it exists and is not already
// expired. If the session does not exist or is already expired, returns ErrSessionNotFound.
func Revoke(sessionID string) error {
	mutex.Lock()
	defer mutex.Unlock()
	if currentSession == nil {
		return ErrSessionNotFound
	}
	if currentSession.ID != sessionID {
		return ErrSessionNotFound
	}
	// If already expired, treat as not found
	if time.Now().After(currentSession.Expires) {
		currentSession = nil
		return ErrSessionNotFound
	}
	// Revoke by clearing currentSession
	currentSession = nil
	return nil
}

// SetDrivePath sets the drive path for cleanup operations.
func SetDrivePath(path string) {
	mutex.Lock()
	defer mutex.Unlock()
	drivePath = path
}

// StartCleanupRoutine starts the background session cleanup goroutine.
func StartCleanupRoutine() {
	mutex.Lock()
	defer mutex.Unlock()

	// Don't start multiple cleanup routines
	if cleanupTicker != nil {
		return
	}

	cleanupTicker = time.NewTicker(cleanupInterval)
	stopCleanup = make(chan bool, 1)

	go func() {
		ticker := cleanupTicker
		stop := stopCleanup
		if ticker == nil || stop == nil {
			log.Printf("Session cleanup routine: ticker or stop channel is nil, exiting")
			return
		}
		log.Printf("Session cleanup routine started (interval: %v)", cleanupInterval)
		for {
			select {
			case <-ticker.C:
				cleanupExpiredSessions()
			case <-stop:
				log.Printf("Session cleanup routine stopped")
				return
			}
		}
	}()
}

// StopCleanupRoutine stops the background session cleanup goroutine.
func StopCleanupRoutine() {
	mutex.Lock()
	ticker := cleanupTicker
	stop := stopCleanup
	cleanupTicker = nil
	stopCleanup = nil
	mutex.Unlock()

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

// cleanupExpiredSessions checks for and cleans up expired sessions.
func cleanupExpiredSessions() {
	mutex.Lock()
	defer mutex.Unlock()

	if currentSession == nil {
		return
	}

	now := time.Now()
	if now.After(currentSession.Expires) {
		sessionID := currentSession.ID
		age := now.Sub(currentSession.CreatedAt)

		log.Printf("Cleaning up expired session: id=%s age=%v", sessionID, age)

		// Clean up session files if drive path is set
		if drivePath != "" {
			sessionDir := filepath.Join(drivePath, "uploads", sessionID)
			if err := os.RemoveAll(sessionDir); err != nil {
				log.Printf("Failed to cleanup session files: id=%s path=%s err=%v", sessionID, sessionDir, err)
			} else {
				log.Printf("Session files cleaned up: id=%s path=%s", sessionID, sessionDir)
			}
		}

		// Remove session from memory
		currentSession = nil
		log.Printf("Expired session removed from memory: id=%s", sessionID)
	}
}
