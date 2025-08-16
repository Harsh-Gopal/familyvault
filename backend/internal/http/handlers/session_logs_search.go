package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// LogSearchParams holds query parameters for log search
type LogSearchParams struct {
	Query     string   `json:"query"`
	Regex     bool     `json:"regex"`
	CaseMatch bool     `json:"case"`
	Level     LogLevel `json:"level,omitempty"`
	Limit     int      `json:"limit"`
	Offset    int      `json:"offset"`
}

// LogSearchResult represents a search result
type LogSearchResult struct {
	LineNumber int    `json:"line_number"`
	Content    string `json:"content"`
	Timestamp  string `json:"timestamp,omitempty"`
	Level      string `json:"level,omitempty"`
}

// LogSearchResponse represents the search response
type LogSearchResponse struct {
	SessionID    string            `json:"session_id"`
	Query        string            `json:"query"`
	TotalMatches int               `json:"total_matches"`
	Results      []LogSearchResult `json:"results"`
	Limit        int               `json:"limit"`
	Offset       int               `json:"offset"`
}

// SessionLogsSearchHandler handles GET /sessions/:id/logs/search
func SessionLogsSearchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from URL
	vars := mux.Vars(r)
	sessionID := vars["id"]

	// Validate session ID
	if !isValidSessionID(sessionID) {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	// Resolve and validate authenticated session
	authSessionID := r.Header.Get("X-Session-ID")
	if authSessionID == "" {
		authSessionID = r.URL.Query().Get("session_id")
	}
	if authSessionID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse search parameters
	params, err := parseLogSearchParams(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid search parameters: %v", err), http.StatusBadRequest)
		return
	}

	// Search logs
	results, totalMatches, err := searchSessionLogs(sessionID, params)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Session logs not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create response
	response := LogSearchResponse{
		SessionID:    sessionID,
		Query:        params.Query,
		TotalMatches: totalMatches,
		Results:      results,
		Limit:        params.Limit,
		Offset:       params.Offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// parseLogSearchParams parses and validates search parameters
func parseLogSearchParams(r *http.Request) (*LogSearchParams, error) {
	params := &LogSearchParams{
		Regex:     false,
		CaseMatch: false,
		Limit:     100, // Default limit
		Offset:    0,   // Default offset
	}

	// Parse query (required)
	query := r.URL.Query().Get("query")
	if query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}
	params.Query = query

	// Parse regex flag
	if regexStr := r.URL.Query().Get("regex"); regexStr != "" {
		params.Regex = regexStr == "true"
	}

	// Parse case flag
	if caseStr := r.URL.Query().Get("case"); caseStr != "" {
		params.CaseMatch = caseStr == "true"
	}

	// Parse level filter
	if levelStr := r.URL.Query().Get("level"); levelStr != "" {
		switch strings.ToLower(levelStr) {
		case "debug":
			params.Level = LogLevelDebug
		case "info":
			params.Level = LogLevelInfo
		case "warn":
			params.Level = LogLevelWarn
		case "error":
			params.Level = LogLevelError
		default:
			return nil, fmt.Errorf("invalid level: %s", levelStr)
		}
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 10000 {
			return nil, fmt.Errorf("limit must be between 1 and 10000")
		}
		params.Limit = limit
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return nil, fmt.Errorf("offset must be non-negative")
		}
		params.Offset = offset
	}

	return params, nil
}

// searchSessionLogs searches logs in a session
func searchSessionLogs(sessionID string, params *LogSearchParams) ([]LogSearchResult, int, error) {
	// Find log file
	logFilePath, err := findSessionLogFile(sessionID)
	if err != nil {
		return nil, 0, err
	}

	// Open log file
	file, err := os.Open(logFilePath)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	// Prepare search pattern
	var searchPattern *regexp.Regexp
	if params.Regex {
		flags := ""
		if !params.CaseMatch {
			flags = "(?i)"
		}
		searchPattern, err = regexp.Compile(flags + params.Query)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid regex pattern: %v", err)
		}
	}

	// Search through file
	var allMatches []LogSearchResult
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		// Check if line matches search criteria
		if matchesSearchCriteria(line, params, searchPattern) {
			// Parse log entry for additional info
			logEntry, _ := parseLogEntry(line)

			// Apply level filter if specified
			if params.Level != "" && logEntry != nil && logEntry.Level != params.Level {
				continue
			}

			result := LogSearchResult{
				LineNumber: lineNumber,
				Content:    line,
			}

			if logEntry != nil {
				result.Timestamp = logEntry.Timestamp
				result.Level = string(logEntry.Level)
			}

			allMatches = append(allMatches, result)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	totalMatches := len(allMatches)

	// Apply pagination
	start := params.Offset
	end := start + params.Limit

	if start >= len(allMatches) {
		return []LogSearchResult{}, totalMatches, nil
	}

	if end > len(allMatches) {
		end = len(allMatches)
	}

	return allMatches[start:end], totalMatches, nil
}

// matchesSearchCriteria checks if a line matches the search criteria
func matchesSearchCriteria(line string, params *LogSearchParams, pattern *regexp.Regexp) bool {
	if params.Regex {
		return pattern.MatchString(line)
	}

	// Simple text search
	searchText := params.Query
	lineText := line

	if !params.CaseMatch {
		searchText = strings.ToLower(searchText)
		lineText = strings.ToLower(lineText)
	}

	return strings.Contains(lineText, searchText)
}
