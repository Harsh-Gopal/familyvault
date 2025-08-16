package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"familyvault/internal/core/drive"
)

func TestHealthHandler(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create some test sessions to count
	for i := 0; i < 3; i++ {
		sessionPath := filepath.Join(tempDir, "uploads", fmt.Sprintf("session-%d", i))
		err := os.MkdirAll(sessionPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create test session %d: %v", i, err)
		}

		// Add a test file to each session
		testFile := filepath.Join(sessionPath, "test.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file in session %d: %v", i, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	HealthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		return
	}

	var response HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify response structure
	if response.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response.Status)
	}

	if response.ActiveSessions != 3 {
		t.Errorf("Expected 3 active sessions, got %d", response.ActiveSessions)
	}

	if response.Uptime == "" {
		t.Error("Uptime should not be empty")
	}

	if response.CPUUsage == "" {
		t.Error("CPU usage should not be empty")
	}

	if response.MemoryUsage == "" {
		t.Error("Memory usage should not be empty")
	}

	if response.DiskUsage == "" {
		t.Error("Disk usage should not be empty")
	}

	if response.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}

	// Verify timestamp is valid RFC3339
	_, err = time.Parse(time.RFC3339, response.Timestamp)
	if err != nil {
		t.Errorf("Invalid timestamp format: %v", err)
	}
}

func TestHealthHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	HealthHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for POST method, got %d", w.Code)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			expected: "45s",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			expected: "2m30s",
		},
		{
			name:     "hours, minutes, and seconds",
			duration: 3*time.Hour + 15*time.Minute + 45*time.Second,
			expected: "3h15m45s",
		},
		{
			name:     "days, hours, minutes, and seconds",
			duration: 2*24*time.Hour + 5*time.Hour + 30*time.Minute + 15*time.Second,
			expected: "2d5h30m15s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCountActiveSessions(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Test with no sessions
	count := countActiveSessions()
	if count != 0 {
		t.Errorf("Expected 0 sessions, got %d", count)
	}

	// Create uploads directory
	uploadsPath := filepath.Join(tempDir, "uploads")
	err := os.MkdirAll(uploadsPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create uploads directory: %v", err)
	}

	// Test with empty uploads directory
	count = countActiveSessions()
	if count != 0 {
		t.Errorf("Expected 0 sessions in empty directory, got %d", count)
	}

	// Create test sessions
	sessionCount := 5
	for i := 0; i < sessionCount; i++ {
		sessionPath := filepath.Join(uploadsPath, fmt.Sprintf("session-%d", i))
		err := os.MkdirAll(sessionPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// Create a regular file (should not be counted)
	regularFile := filepath.Join(uploadsPath, "not-a-session.txt")
	err = os.WriteFile(regularFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	count = countActiveSessions()
	if count != sessionCount {
		t.Errorf("Expected %d sessions, got %d", sessionCount, count)
	}
}

func TestGetDiskUsage(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Test with empty directory
	usage := getDiskUsage()
	if usage == "unknown" {
		t.Error("Disk usage should not be unknown for empty directory")
	}

	// Create some test files
	testFiles := []struct {
		path    string
		content string
	}{
		{"file1.txt", "small content"},
		{"subdir/file2.txt", strings.Repeat("x", 1024)}, // 1KB
		{"subdir/file3.txt", strings.Repeat("y", 2048)}, // 2KB
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(tempDir, tf.path)
		err := os.MkdirAll(filepath.Dir(filePath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory for %s: %v", tf.path, err)
		}

		err = os.WriteFile(filePath, []byte(tf.content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", tf.path, err)
		}
	}

	usage = getDiskUsage()

	// Should show some usage now
	if usage == "0 B" {
		t.Error("Expected non-zero disk usage after creating files")
	}

	// Should contain a unit
	validUnits := []string{"B", "KB", "MB", "GB"}
	hasValidUnit := false
	for _, unit := range validUnits {
		if strings.Contains(usage, unit) {
			hasValidUnit = true
			break
		}
	}

	if !hasValidUnit {
		t.Errorf("Disk usage should contain a valid unit, got: %s", usage)
	}
}
