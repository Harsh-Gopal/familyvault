package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionExpiry(t *testing.T) {
	// Clean up any existing session
	Close()

	// Test short duration session
	shortDuration := 100 * time.Millisecond
	s, err := Open(shortDuration)
	if err != nil {
		t.Fatalf("Failed to open session: %v", err)
	}

	// Session should be active initially
	if !IsOpen() {
		t.Fatal("Session should be active immediately after creation")
	}

	// Wait for expiry
	time.Sleep(shortDuration + 50*time.Millisecond)

	// Session should be expired now
	if IsOpen() {
		t.Fatal("Session should be expired")
	}

	// Get should return nil for expired session
	if Get() != nil {
		t.Fatal("Get() should return nil for expired session")
	}

	// Verify session has correct timestamps
	if s.CreatedAt.IsZero() {
		t.Fatal("Session should have CreatedAt timestamp")
	}
	if s.Expires.Before(s.CreatedAt) {
		t.Fatal("Session Expires should be after CreatedAt")
	}
}

func TestSessionTimeout_EnvironmentVariable(t *testing.T) {
	// Test default timeout
	originalEnv := os.Getenv("SESSION_TIMEOUT_MINUTES")
	defer os.Setenv("SESSION_TIMEOUT_MINUTES", originalEnv)

	// Test with no env var (should use default)
	os.Unsetenv("SESSION_TIMEOUT_MINUTES")
	timeout := getSessionTimeout()
	expected := time.Duration(defaultSessionTimeoutMinutes) * time.Minute
	if timeout != expected {
		t.Fatalf("Expected default timeout %v, got %v", expected, timeout)
	}

	// Test with valid env var
	os.Setenv("SESSION_TIMEOUT_MINUTES", "30")
	timeout = getSessionTimeout()
	expected = 30 * time.Minute
	if timeout != expected {
		t.Fatalf("Expected 30 minute timeout, got %v", timeout)
	}

	// Test with invalid env var (should use default)
	os.Setenv("SESSION_TIMEOUT_MINUTES", "invalid")
	timeout = getSessionTimeout()
	expected = time.Duration(defaultSessionTimeoutMinutes) * time.Minute
	if timeout != expected {
		t.Fatalf("Expected default timeout for invalid env var, got %v", timeout)
	}

	// Test with zero env var (should use default)
	os.Setenv("SESSION_TIMEOUT_MINUTES", "0")
	timeout = getSessionTimeout()
	expected = time.Duration(defaultSessionTimeoutMinutes) * time.Minute
	if timeout != expected {
		t.Fatalf("Expected default timeout for zero env var, got %v", timeout)
	}
}

func TestSessionCleanup(t *testing.T) {
	// Ensure cleanup routine is stopped
	StopCleanupRoutine()
	defer StopCleanupRoutine()

	// Set up temporary drive path
	driveDir := t.TempDir()
	SetDrivePath(driveDir)

	// Clean up any existing session
	Close()

	// Create a session with short duration
	shortDuration := 100 * time.Millisecond
	s, err := Open(shortDuration)
	if err != nil {
		t.Fatalf("Failed to open session: %v", err)
	}

	// Create session directory and files
	sessionDir := filepath.Join(driveDir, "uploads", s.ID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	testFile := filepath.Join(sessionDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify session and files exist
	if !IsOpen() {
		t.Fatal("Session should be active")
	}
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Fatal("Session directory should exist")
	}
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatal("Test file should exist")
	}

	// Wait for session to expire
	time.Sleep(shortDuration + 50*time.Millisecond)

	// Run cleanup manually
	cleanupExpiredSessions()

	// Verify session is cleaned up from memory
	if Get() != nil {
		t.Fatal("Session should be cleaned up from memory")
	}

	// Verify files are cleaned up
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Fatal("Session directory should be deleted")
	}
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatal("Test file should be deleted")
	}
}

func TestSkipActiveSession(t *testing.T) {
	// Ensure cleanup routine is stopped
	StopCleanupRoutine()
	defer StopCleanupRoutine()

	// Set up temporary drive path
	driveDir := t.TempDir()
	SetDrivePath(driveDir)

	// Clean up any existing session
	Close()

	// Create a session with long duration
	longDuration := 1 * time.Hour
	s, err := Open(longDuration)
	if err != nil {
		t.Fatalf("Failed to open session: %v", err)
	}

	// Create session directory and files
	sessionDir := filepath.Join(driveDir, "uploads", s.ID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}

	testFile := filepath.Join(sessionDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Run cleanup on active session
	cleanupExpiredSessions()

	// Verify session and files are NOT cleaned up
	if !IsOpen() {
		t.Fatal("Active session should not be cleaned up")
	}
	if Get() == nil {
		t.Fatal("Active session should remain in memory")
	}
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Fatal("Session directory should not be deleted for active session")
	}
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatal("Test file should not be deleted for active session")
	}
}

func TestCleanupRoutineLifecycle(t *testing.T) {
	// Test the lifecycle without actually starting the ticker
	// to avoid race conditions in tests

	// Multiple stops should be safe
	StopCleanupRoutine()
	StopCleanupRoutine()

	// Note: We skip actually starting the routine in tests
	// to avoid race conditions between test goroutines
}

func TestOpenWithDefaultTimeout(t *testing.T) {
	originalEnv := os.Getenv("SESSION_TIMEOUT_MINUTES")
	defer os.Setenv("SESSION_TIMEOUT_MINUTES", originalEnv)

	// Set custom timeout
	os.Setenv("SESSION_TIMEOUT_MINUTES", "15")

	// Clean up any existing session
	Close()

	s, err := OpenWithDefaultTimeout()
	if err != nil {
		t.Fatalf("Failed to open session with default timeout: %v", err)
	}

	// Verify session duration is approximately 15 minutes
	duration := s.Expires.Sub(s.CreatedAt)
	expected := 15 * time.Minute

	// Allow some tolerance for execution time
	if duration < expected-time.Second || duration > expected+time.Second {
		t.Fatalf("Expected session duration ~%v, got %v", expected, duration)
	}
}

func TestCleanupWithoutDrivePath(t *testing.T) {
	// This test doesn't use the cleanup routine to avoid race conditions

	// Set empty drive path
	SetDrivePath("")

	// Clean up any existing session
	Close()

	// Create expired session
	shortDuration := 100 * time.Millisecond
	s, err := Open(shortDuration)
	if err != nil {
		t.Fatalf("Failed to open session: %v", err)
	}

	// Wait for expiry
	time.Sleep(shortDuration + 50*time.Millisecond)

	// Run cleanup manually (should not fail even without drive path)
	cleanupExpiredSessions()

	// Verify session is cleaned up from memory
	if Get() != nil {
		t.Fatal("Session should be cleaned up from memory even without drive path")
	}

	_ = s // Use variable to avoid unused error
}

func TestNoSessionCleanup(t *testing.T) {
	// Ensure cleanup routine is stopped
	StopCleanupRoutine()
	defer StopCleanupRoutine()

	// Clean up any existing session
	Close()

	// Run cleanup with no session (should not panic)
	cleanupExpiredSessions()

	// Should still be no session
	if Get() != nil {
		t.Fatal("Should be no session after cleanup with no session")
	}
}
