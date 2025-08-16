package session

import (
	"testing"
	"time"
)

// TestSessionCleanupIntegration tests the full cleanup integration
func TestSessionCleanupIntegration(t *testing.T) {
	// Ensure we have a clean state
	StopCleanupRoutine()
	Close()

	// Set up temporary drive path
	driveDir := t.TempDir()
	SetDrivePath(driveDir)

	// Start the cleanup routine for this test
	StartCleanupRoutine()
	defer StopCleanupRoutine()

	// Create a session with very short duration for testing
	shortDuration := 200 * time.Millisecond
	s, err := Open(shortDuration)
	if err != nil {
		t.Fatalf("Failed to open session: %v", err)
	}

	// Verify session exists
	if !IsOpen() {
		t.Fatal("Session should be active")
	}

	// Wait for expiry plus cleanup interval buffer
	time.Sleep(shortDuration + 100*time.Millisecond)

	// Give the cleanup routine time to run (it runs every 5 minutes normally,
	// but we'll trigger it manually for faster testing)
	cleanupExpiredSessions()

	// Session should be cleaned up
	if Get() != nil {
		t.Fatal("Session should be cleaned up after expiry")
	}

	_ = s // Use the variable
}
