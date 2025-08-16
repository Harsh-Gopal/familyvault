package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
)

func TestSessionLogsHandler(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear()

	// Create test session
	testSession, err := session.Open(time.Hour)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	// Create session directory and log file
	sessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create test log file with various log levels
	logContent := `2025-08-12T07:30:01Z [INFO] Session started
2025-08-12T07:30:02Z [DEBUG] Processing chunk 1
2025-08-12T07:30:03Z [WARN] Low disk space warning
2025-08-12T07:30:04Z [ERROR] Failed to process file
2025-08-12T07:30:05Z [INFO] Session completed
2025-08-12T07:30:06Z [DEBUG] Cleanup started
2025-08-12T07:30:07Z [INFO] Cleanup completed`

	logFilePath := filepath.Join(sessionDir, "session.log")
	if err := os.WriteFile(logFilePath, []byte(logContent), 0644); err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	tests := []struct {
		name           string
		sessionID      string
		authSessionID  string
		queryParams    string
		expectedStatus int
		expectSuccess  bool
		expectedCount  int
	}{
		{
			name:           "basic log retrieval",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedCount:  7,
		},
		{
			name:           "limit logs",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			queryParams:    "?limit=3",
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedCount:  3,
		},
		{
			name:           "offset logs",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			queryParams:    "?offset=2&limit=3",
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedCount:  3,
		},
		{
			name:           "tail logs",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			queryParams:    "?tail=true&limit=2",
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedCount:  2,
		},
		{
			name:           "filter by level",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			queryParams:    "?level=info",
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedCount:  3, // 3 INFO level logs
		},
		{
			name:           "filter by error level",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			queryParams:    "?level=error",
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedCount:  1, // 1 ERROR level log
		},
		{
			name:           "nonexistent session",
			sessionID:      "nonexistent-session-id",
			authSessionID:  testSession.ID,
			queryParams:    "",
			expectedStatus: http.StatusNotFound,
			expectSuccess:  false,
		},
		{
			name:           "invalid session ID",
			sessionID:      "../../../etc/passwd",
			authSessionID:  testSession.ID,
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:           "unauthorized access",
			sessionID:      testSession.ID,
			authSessionID:  "",
			queryParams:    "",
			expectedStatus: http.StatusUnauthorized,
			expectSuccess:  false,
		},
		{
			name:           "invalid limit",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			queryParams:    "?limit=invalid",
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:           "invalid level",
			sessionID:      testSession.ID,
			authSessionID:  testSession.ID,
			queryParams:    "?level=invalid",
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/sessions/" + tt.sessionID + "/logs" + tt.queryParams
			req := httptest.NewRequest("GET", url, nil)
			if tt.authSessionID != "" {
				req.Header.Set("X-Session-ID", tt.authSessionID)
			}
			w := httptest.NewRecorder()

			sessionLogsHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectSuccess && w.Code == http.StatusOK {
				// Parse response
				var response SessionLogsResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.SessionID != testSession.ID {
					t.Errorf("Expected session ID %s, got %s", testSession.ID, response.SessionID)
				}

				if response.LogCount != tt.expectedCount {
					t.Errorf("Expected log count %d, got %d", tt.expectedCount, response.LogCount)
				}

				if len(response.Logs) != tt.expectedCount {
					t.Errorf("Expected %d logs, got %d", tt.expectedCount, len(response.Logs))
				}

				// Verify log structure
				if len(response.Logs) > 0 {
					firstLog := response.Logs[0]
					if firstLog.Timestamp == "" {
						t.Error("Expected timestamp to be set")
					}
					if firstLog.Level == "" {
						t.Error("Expected level to be set")
					}
					if firstLog.Message == "" {
						t.Error("Expected message to be set")
					}
				}

				// Verify level filtering
				if strings.Contains(tt.queryParams, "level=info") {
					for _, logEntry := range response.Logs {
						if logEntry.Level != LogLevelInfo {
							t.Errorf("Expected all logs to be INFO level, got %s", logEntry.Level)
						}
					}
				}

				// Verify tail behavior
				if strings.Contains(tt.queryParams, "tail=true") && len(response.Logs) > 0 {
					// Last log should be the most recent
					lastLog := response.Logs[len(response.Logs)-1]
					if !strings.Contains(lastLog.Message, "Cleanup completed") {
						t.Error("Expected tail to return the most recent logs")
					}
				}
			}
		})
	}
}

func TestSessionLogsHandlerDeletedSession(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear()

	// Create test session
	testSession, err := session.Open(time.Hour)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	// Create session directory and log file
	sessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create test log file
	logContent := `2025-08-12T07:30:01Z [INFO] Session started
2025-08-12T07:30:02Z [DEBUG] Processing files
2025-08-12T07:30:03Z [INFO] Session completed`

	logFilePath := filepath.Join(sessionDir, "session.log")
	if err := os.WriteFile(logFilePath, []byte(logContent), 0644); err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	// Add some files to manifest so session can be deleted
	manifest.Add(manifest.FileRecord{
		SessionID:  testSession.ID,
		Filename:   "test.txt",
		UploadedAt: time.Now(),
		Tags:       map[string]string{},
	})

	// Delete the session to create a backup
	req := httptest.NewRequest("DELETE", "/sessions/"+testSession.ID, nil)
	req.Header.Set("X-Session-ID", testSession.ID)
	w := httptest.NewRecorder()
	deleteSessionHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to delete session for backup: %d", w.Code)
	}

	// Now test logs retrieval from backup
	req = httptest.NewRequest("GET", "/sessions/"+testSession.ID+"/logs", nil)
	req.Header.Set("X-Session-ID", testSession.ID)
	w = httptest.NewRecorder()

	sessionLogsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response SessionLogsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify logs were retrieved from backup
	if response.LogCount != 3 {
		t.Errorf("Expected 3 logs from backup, got %d", response.LogCount)
	}

	if len(response.Logs) != 3 {
		t.Errorf("Expected 3 log entries, got %d", len(response.Logs))
	}
}

func TestSessionLogsHandlerWrongMethod(t *testing.T) {
	req := httptest.NewRequest("POST", "/sessions/test-session/logs", nil)
	w := httptest.NewRecorder()

	sessionLogsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestSessionLogsHandlerDriveNotAvailable(t *testing.T) {
	// Setup test environment with non-existent drive path
	drive.SetDrivePath("/nonexistent/path")

	req := httptest.NewRequest("GET", "/sessions/test-session/logs", nil)
	req.Header.Set("X-Session-ID", "test-session")
	w := httptest.NewRecorder()

	sessionLogsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestIsValidSessionID(t *testing.T) {
	tests := []struct {
		sessionID string
		expected  bool
	}{
		{"abc123-def456", true},
		{"session_123", true},
		{"valid-session-id", true},
		{"123456789", true},
		{"../../../etc/passwd", false},
		{"session/with/slashes", false},
		{"session..with..dots", false},
		{"", false},
		{"valid_session-123", true},
		{"session with spaces", false},
		{"session<script>", false},
	}

	for _, tt := range tests {
		t.Run("validate_"+tt.sessionID, func(t *testing.T) {
			result := isValidSessionID(tt.sessionID)
			if result != tt.expected {
				t.Errorf("Expected %v for session ID %q, got %v", tt.expected, tt.sessionID, result)
			}
		})
	}
}

func TestParseLogsQueryParams(t *testing.T) {
	tests := []struct {
		name        string
		queryString string
		expected    *LogsQueryParams
		expectError bool
	}{
		{
			name:        "default parameters",
			queryString: "",
			expected: &LogsQueryParams{
				Limit:  1000,
				Offset: 0,
				Tail:   false,
			},
			expectError: false,
		},
		{
			name:        "custom limit",
			queryString: "?limit=500",
			expected: &LogsQueryParams{
				Limit:  500,
				Offset: 0,
				Tail:   false,
			},
			expectError: false,
		},
		{
			name:        "custom offset",
			queryString: "?offset=100",
			expected: &LogsQueryParams{
				Limit:  1000,
				Offset: 100,
				Tail:   false,
			},
			expectError: false,
		},
		{
			name:        "tail enabled",
			queryString: "?tail=true",
			expected: &LogsQueryParams{
				Limit:  1000,
				Offset: 0,
				Tail:   true,
			},
			expectError: false,
		},
		{
			name:        "level filter",
			queryString: "?level=error",
			expected: &LogsQueryParams{
				Limit:  1000,
				Offset: 0,
				Tail:   false,
				Level:  LogLevelError,
			},
			expectError: false,
		},
		{
			name:        "all parameters",
			queryString: "?limit=200&offset=50&tail=true&level=warn",
			expected: &LogsQueryParams{
				Limit:  200,
				Offset: 50,
				Tail:   true,
				Level:  LogLevelWarn,
			},
			expectError: false,
		},
		{
			name:        "invalid limit",
			queryString: "?limit=invalid",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "negative offset",
			queryString: "?offset=-1",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid tail",
			queryString: "?tail=invalid",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid level",
			queryString: "?level=invalid",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "limit too high",
			queryString: "?limit=20000",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/sessions/test/logs"+tt.queryString, nil)
			result, err := parseLogsQueryParams(req)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result.Limit != tt.expected.Limit {
					t.Errorf("Expected limit %d, got %d", tt.expected.Limit, result.Limit)
				}
				if result.Offset != tt.expected.Offset {
					t.Errorf("Expected offset %d, got %d", tt.expected.Offset, result.Offset)
				}
				if result.Tail != tt.expected.Tail {
					t.Errorf("Expected tail %v, got %v", tt.expected.Tail, result.Tail)
				}
				if result.Level != tt.expected.Level {
					t.Errorf("Expected level %s, got %s", tt.expected.Level, result.Level)
				}
			}
		})
	}
}

func TestParseLogEntry(t *testing.T) {
	tests := []struct {
		name     string
		logLine  string
		expected *LogEntry
	}{
		{
			name:    "bracket format",
			logLine: "2025-08-12T07:30:01Z [INFO] Session started",
			expected: &LogEntry{
				Timestamp: "2025-08-12T07:30:01Z",
				Level:     LogLevelInfo,
				Message:   "Session started",
			},
		},
		{
			name:    "colon format",
			logLine: "2025-08-12T07:30:01Z INFO: Session started",
			expected: &LogEntry{
				Timestamp: "2025-08-12T07:30:01Z",
				Level:     LogLevelInfo,
				Message:   "Session started",
			},
		},
		{
			name:    "level first format",
			logLine: "INFO 2025-08-12T07:30:01Z Session started",
			expected: &LogEntry{
				Timestamp: "2025-08-12T07:30:01Z",
				Level:     LogLevelInfo,
				Message:   "Session started",
			},
		},
		{
			name:    "simple format",
			logLine: "2025-08-12T07:30:01Z INFO Session started",
			expected: &LogEntry{
				Timestamp: "2025-08-12T07:30:01Z",
				Level:     LogLevelInfo,
				Message:   "Session started",
			},
		},
		{
			name:    "warning level",
			logLine: "2025-08-12T07:30:01Z [WARNING] Low disk space",
			expected: &LogEntry{
				Timestamp: "2025-08-12T07:30:01Z",
				Level:     LogLevelWarn,
				Message:   "Low disk space",
			},
		},
		{
			name:    "error level",
			logLine: "2025-08-12T07:30:01Z [ERROR] Failed to process",
			expected: &LogEntry{
				Timestamp: "2025-08-12T07:30:01Z",
				Level:     LogLevelError,
				Message:   "Failed to process",
			},
		},
		{
			name:    "debug level",
			logLine: "2025-08-12T07:30:01Z [DEBUG] Processing chunk 1",
			expected: &LogEntry{
				Timestamp: "2025-08-12T07:30:01Z",
				Level:     LogLevelDebug,
				Message:   "Processing chunk 1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLogEntry(tt.logLine)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if result.Timestamp != tt.expected.Timestamp {
				t.Errorf("Expected timestamp %s, got %s", tt.expected.Timestamp, result.Timestamp)
			}

			if result.Level != tt.expected.Level {
				t.Errorf("Expected level %s, got %s", tt.expected.Level, result.Level)
			}

			if result.Message != tt.expected.Message {
				t.Errorf("Expected message %s, got %s", tt.expected.Message, result.Message)
			}
		})
	}
}

func TestReadLogsTail(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test log file with multiple lines
	logContent := `2025-08-12T07:30:01Z [INFO] Line 1
2025-08-12T07:30:02Z [DEBUG] Line 2
2025-08-12T07:30:03Z [WARN] Line 3
2025-08-12T07:30:04Z [ERROR] Line 4
2025-08-12T07:30:05Z [INFO] Line 5
2025-08-12T07:30:06Z [DEBUG] Line 6
2025-08-12T07:30:07Z [INFO] Line 7`

	logFilePath := filepath.Join(tempDir, "test.log")
	if err := os.WriteFile(logFilePath, []byte(logContent), 0644); err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	file, err := os.Open(logFilePath)
	if err != nil {
		t.Fatalf("Failed to open test log file: %v", err)
	}
	defer file.Close()

	// Test tail with limit 3
	params := &LogsQueryParams{
		Limit: 3,
		Tail:  true,
	}

	logs, err := readLogsTail(file, params)
	if err != nil {
		t.Fatalf("readLogsTail failed: %v", err)
	}

	if len(logs) != 3 {
		t.Errorf("Expected 3 logs, got %d", len(logs))
	}

	// Verify we got the last 3 lines
	if len(logs) > 0 && !strings.Contains(logs[len(logs)-1].Message, "Line 7") {
		t.Error("Expected last log to be Line 7")
	}
}

func TestFindSessionLogFile(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	testSessionID := "test-session-logs"

	// Test 1: No log file exists
	_, err := findSessionLogFile(testSessionID)
	if err == nil {
		t.Error("Expected error for non-existent log file")
	}

	// Test 2: Active session log file exists
	sessionDir := filepath.Join(tempDir, "uploads", testSessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	activeLogPath := filepath.Join(sessionDir, "session.log")
	if err := os.WriteFile(activeLogPath, []byte("test log"), 0644); err != nil {
		t.Fatalf("Failed to create active log file: %v", err)
	}

	logPath, err := findSessionLogFile(testSessionID)
	if err != nil {
		t.Fatalf("Failed to find active log file: %v", err)
	}

	if logPath != activeLogPath {
		t.Errorf("Expected log path %s, got %s", activeLogPath, logPath)
	}

	// Test 3: Remove active log and test backup log
	os.Remove(activeLogPath)
	os.RemoveAll(sessionDir)

	// Create backup directory with log file
	uploadsDir := filepath.Join(tempDir, "uploads")
	backupDir := filepath.Join(uploadsDir, testSessionID+".deleted.1234567890")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup directory: %v", err)
	}

	backupLogPath := filepath.Join(backupDir, "session.log")
	if err := os.WriteFile(backupLogPath, []byte("backup log"), 0644); err != nil {
		t.Fatalf("Failed to create backup log file: %v", err)
	}

	logPath, err = findSessionLogFile(testSessionID)
	if err != nil {
		t.Fatalf("Failed to find backup log file: %v", err)
	}

	if logPath != backupLogPath {
		t.Errorf("Expected backup log path %s, got %s", backupLogPath, logPath)
	}
}
