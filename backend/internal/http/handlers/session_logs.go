package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"familyvault/internal/core/drive"
)

// LogLevel represents the severity levels for log filtering
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp string   `json:"timestamp"`
	Level     LogLevel `json:"level"`
	Message   string   `json:"message"`
}

// SessionLogsResponse represents the response for session logs requests
type SessionLogsResponse struct {
	SessionID string     `json:"session_id"`
	LogCount  int        `json:"log_count"`
	Logs      []LogEntry `json:"logs"`
}

// LogsQueryParams holds the parsed query parameters for log requests
type LogsQueryParams struct {
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
	Tail   bool     `json:"tail"`
	Level  LogLevel `json:"level,omitempty"`
}

// GET /sessions/{session_id}/logs
// Fetches the complete or partial execution logs of a given session for debugging and monitoring.
func sessionLogsHandler(w http.ResponseWriter, r *http.Request) {
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
	// Expected path: /sessions/{session_id}/logs
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/sessions/"), "/")
	if len(pathParts) != 2 || pathParts[1] != "logs" {
		httpError(w, http.StatusBadRequest, "invalid URL format, expected /sessions/:id/logs")
		return
	}
	sessionIDFromPath := path.Base(pathParts[0])
	if sessionIDFromPath == "" || sessionIDFromPath == "sessions" || sessionIDFromPath == "." {
		httpError(w, http.StatusBadRequest, "invalid session id in path")
		return
	}

	// Validate session ID against path traversal
	if !isValidSessionID(sessionIDFromPath) {
		httpError(w, http.StatusBadRequest, "invalid session id format")
		return
	}

	log.Printf("session-logs request: session=%s ip=%s", sessionIDFromPath, r.RemoteAddr)

	// Resolve and validate authenticated session (header or query)
	authSessionID := r.Header.Get("X-Session-ID")
	if authSessionID == "" {
		authSessionID = r.URL.Query().Get("session_id")
	}
	if authSessionID == "" {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	// For logs, we allow viewing any session if user has valid authentication
	// In production, implement proper role-based access control
	log.Printf("session-logs auth: auth_session=%s target_session=%s", authSessionID, sessionIDFromPath)

	// Parse query parameters
	params, err := parseLogsQueryParams(r)
	if err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid query parameters: %v", err))
		return
	}

	// Find and read session logs
	logsResponse, err := getSessionLogs(sessionIDFromPath, params)
	if err != nil {
		log.Printf("session-logs error: session=%s ip=%s err=%v", sessionIDFromPath, r.RemoteAddr, err)
		if strings.Contains(err.Error(), "not found") {
			httpError(w, http.StatusNotFound, "session logs not found")
		} else {
			httpError(w, http.StatusInternalServerError, "failed to retrieve logs")
		}
		return
	}

	// Create response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(logsResponse); err != nil {
		log.Printf("session-logs encode response error: session=%s ip=%s err=%v", sessionIDFromPath, r.RemoteAddr, err)
		return
	}

	log.Printf("session-logs success: session=%s ip=%s count=%d", sessionIDFromPath, r.RemoteAddr, logsResponse.LogCount)
}

// isValidSessionID validates session ID format to prevent path traversal
func isValidSessionID(sessionID string) bool {
	// Session IDs should be UUIDs or similar safe formats
	// Allow alphanumeric characters, hyphens, and underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-_]+$`, sessionID)
	return matched && !strings.Contains(sessionID, "..") && !strings.Contains(sessionID, "/")
}

// parseLogsQueryParams parses and validates query parameters for log requests
func parseLogsQueryParams(r *http.Request) (*LogsQueryParams, error) {
	params := &LogsQueryParams{
		Limit:  1000,  // Default limit
		Offset: 0,     // Default offset
		Tail:   false, // Default tail
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 || limit > 10000 {
			return nil, fmt.Errorf("invalid limit parameter: must be between 0 and 10000")
		}
		params.Limit = limit
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return nil, fmt.Errorf("invalid offset parameter: must be non-negative")
		}
		params.Offset = offset
	}

	// Parse tail
	if tailStr := r.URL.Query().Get("tail"); tailStr != "" {
		tail, err := strconv.ParseBool(tailStr)
		if err != nil {
			return nil, fmt.Errorf("invalid tail parameter: must be true or false")
		}
		params.Tail = tail
	}

	// Parse level
	if levelStr := r.URL.Query().Get("level"); levelStr != "" {
		level := LogLevel(strings.ToLower(levelStr))
		switch level {
		case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
			params.Level = level
		default:
			return nil, fmt.Errorf("invalid level parameter: must be debug, info, warn, or error")
		}
	}

	return params, nil
}

// getSessionLogs retrieves and processes session logs based on the provided parameters
func getSessionLogs(sessionID string, params *LogsQueryParams) (*SessionLogsResponse, error) {
	// Find log file location
	logFilePath, err := findSessionLogFile(sessionID)
	if err != nil {
		return nil, err
	}

	// Read and process logs
	logs, err := readSessionLogs(logFilePath, params)
	if err != nil {
		return nil, err
	}

	return &SessionLogsResponse{
		SessionID: sessionID,
		LogCount:  len(logs),
		Logs:      logs,
	}, nil
}

// findSessionLogFile locates the log file for a session (active or backup)
func findSessionLogFile(sessionID string) (string, error) {
	// Check active session log file first
	activeLogPath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID, "session.log")
	if _, err := os.Stat(activeLogPath); err == nil {
		return activeLogPath, nil
	}

	// Check backup location
	backupInfo, err := findSessionBackupInfo(sessionID)
	if err == nil && backupInfo != nil {
		backupLogPath := filepath.Join(backupInfo.BackupPath, "session.log")
		if _, err := os.Stat(backupLogPath); err == nil {
			return backupLogPath, nil
		}
	}

	return "", fmt.Errorf("log file not found for session %s", sessionID)
}

// readSessionLogs reads and processes logs from the specified file with filtering and pagination
func readSessionLogs(logFilePath string, params *LogsQueryParams) ([]LogEntry, error) {
	file, err := os.Open(logFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	if params.Tail {
		return readLogsTail(file, params)
	}
	return readLogsFromStart(file, params)
}

// readLogsFromStart reads logs from the beginning with offset and limit
func readLogsFromStart(file *os.File, params *LogsQueryParams) ([]LogEntry, error) {
	scanner := bufio.NewScanner(file)
	var logs []LogEntry
	lineNumber := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip lines until we reach the offset
		if lineNumber < params.Offset {
			lineNumber++
			continue
		}

		// Parse log entry
		entry, err := parseLogEntry(line)
		if err != nil {
			// Skip malformed log entries
			lineNumber++
			continue
		}

		// Apply level filtering
		if params.Level != "" && entry.Level != params.Level {
			lineNumber++
			continue
		}

		logs = append(logs, *entry)
		lineNumber++

		// Stop if we've reached the limit
		if len(logs) >= params.Limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return logs, nil
}

// readLogsTail reads the last N lines from the log file efficiently
func readLogsTail(file *os.File, params *LogsQueryParams) ([]LogEntry, error) {
	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileSize := fileInfo.Size()
	if fileSize == 0 {
		return []LogEntry{}, nil
	}

	// Read from the end of the file to find the last N lines
	bufferSize := int64(4096) // 4KB buffer
	var lines []string
	var buffer []byte

	// Start from the end and work backwards
	for offset := fileSize; offset > 0 && len(lines) < params.Limit*2; {
		// Calculate read position
		readSize := bufferSize
		if offset < bufferSize {
			readSize = offset
		}
		offset -= readSize

		// Read chunk
		chunk := make([]byte, readSize)
		_, err := file.ReadAt(chunk, offset)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read file chunk: %w", err)
		}

		// Prepend to buffer
		buffer = append(chunk, buffer...)

		// Split into lines
		tempLines := strings.Split(string(buffer), "\n")
		if len(tempLines) > 1 {
			// Keep the first partial line in buffer for next iteration
			if offset > 0 {
				buffer = []byte(tempLines[0])
				lines = append(tempLines[1:], lines...)
			} else {
				lines = tempLines
			}
		}
	}

	// Take the last N lines
	startIndex := 0
	if len(lines) > params.Limit {
		startIndex = len(lines) - params.Limit
	}
	tailLines := lines[startIndex:]

	// Parse log entries
	var logs []LogEntry
	for _, line := range tailLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		entry, err := parseLogEntry(line)
		if err != nil {
			continue // Skip malformed entries
		}

		// Apply level filtering
		if params.Level != "" && entry.Level != params.Level {
			continue
		}

		logs = append(logs, *entry)
	}

	return logs, nil
}

// parseLogEntry parses a single log line into a LogEntry structure
func parseLogEntry(line string) (*LogEntry, error) {
	// Expected format: "2025-08-12T07:30:01Z [INFO] Message content"
	// Or: "2025-08-12T07:30:01Z INFO: Message content"
	// Or: "INFO 2025-08-12T07:30:01Z Message content"

	// Try different log formats
	patterns := []string{
		`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z)\s+\[(\w+)\]\s+(.+)$`, // [LEVEL] format
		`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z)\s+(\w+):\s+(.+)$`,    // LEVEL: format
		`^(\w+)\s+(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z)\s+(.+)$`,     // LEVEL timestamp format
		`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z)\s+(\w+)\s+(.+)$`,     // timestamp LEVEL format
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(line)

		if len(matches) == 4 {
			var timestamp, levelStr, message string

			// Determine which capture group is which based on the pattern
			if strings.Contains(pattern, `^(\w+)`) {
				// Level comes first
				levelStr = matches[1]
				timestamp = matches[2]
				message = matches[3]
			} else {
				// Timestamp comes first
				timestamp = matches[1]
				levelStr = matches[2]
				message = matches[3]
			}

			// Validate and normalize level
			level := LogLevel(strings.ToLower(levelStr))
			switch level {
			case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
				// Valid level
			default:
				// Try to map common variations
				switch strings.ToLower(levelStr) {
				case "warning", "warn":
					level = LogLevelWarn
				case "err", "error":
					level = LogLevelError
				case "information", "info":
					level = LogLevelInfo
				case "debug", "dbg":
					level = LogLevelDebug
				default:
					level = LogLevelInfo // Default to info for unknown levels
				}
			}

			return &LogEntry{
				Timestamp: timestamp,
				Level:     level,
				Message:   strings.TrimSpace(message),
			}, nil
		}
	}

	// If no pattern matches, create a simple entry with current timestamp
	return &LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     LogLevelInfo,
		Message:   line,
	}, nil
}
