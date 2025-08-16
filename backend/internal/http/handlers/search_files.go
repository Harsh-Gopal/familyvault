package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
)

// FileSearchResult represents a file in search results
type FileSearchResult struct {
	Name       string            `json:"name"`
	Size       int64             `json:"size"`
	UploadTime time.Time         `json:"upload_time"`
	Type       string            `json:"type"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// SearchFilters holds the search criteria
type SearchFilters struct {
	Name     string
	Type     string
	DateFrom *time.Time
	DateTo   *time.Time
	Tags     []string
}

// GET /search-files
// Searches files for the active session based on various criteria.
// Requires session ID via header "X-Session-ID" or query parameter "session_id".
// Supports query parameters: name, type, date_from, date_to, tags
func searchFilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate drive availability
	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	// Resolve and validate session
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		sessionID = r.URL.Query().Get("session_id")
	}
	current := session.Get()
	if sessionID == "" || current == nil || current.ID != sessionID || time.Now().After(current.Expires) {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	// Parse search filters
	filters, err := parseSearchFilters(r)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get files for the session
	files, err := getFilesWithMetadata(current.ID)
	if err != nil {
		log.Printf("search-files error getting files: session=%s err=%v", current.ID, err)
		httpError(w, http.StatusInternalServerError, "failed to get file list")
		return
	}

	// Apply filters
	results := applyFilters(files, filters)

	if len(results) == 0 {
		httpError(w, http.StatusNotFound, "no files match the search criteria")
		return
	}

	// Return results
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		log.Printf("search-files error encoding response: session=%s err=%v", current.ID, err)
		httpError(w, http.StatusInternalServerError, "failed to encode response")
		return
	}

	log.Printf("search-files success: session=%s matches=%d", current.ID, len(results))
}

// parseSearchFilters extracts and validates search parameters from the request
func parseSearchFilters(r *http.Request) (*SearchFilters, error) {
	filters := &SearchFilters{}
	query := r.URL.Query()

	// Name filter (substring match)
	if name := query.Get("name"); name != "" {
		// Sanitize to prevent directory traversal
		if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
			return nil, errors.New("invalid name parameter: contains unsafe characters")
		}
		filters.Name = strings.ToLower(name)
	}

	// Type filter (file extension)
	if fileType := query.Get("type"); fileType != "" {
		// Sanitize file extension
		fileType = strings.TrimPrefix(fileType, ".")
		if strings.Contains(fileType, "..") || strings.Contains(fileType, "/") || strings.Contains(fileType, "\\") {
			return nil, errors.New("invalid type parameter: contains unsafe characters")
		}
		filters.Type = strings.ToLower(fileType)
	}

	// Date range filters
	if dateFromStr := query.Get("date_from"); dateFromStr != "" {
		dateFrom, err := time.Parse(time.RFC3339, dateFromStr)
		if err != nil {
			return nil, errors.New("invalid date_from parameter: must be RFC3339 format")
		}
		filters.DateFrom = &dateFrom
	}

	if dateToStr := query.Get("date_to"); dateToStr != "" {
		dateTo, err := time.Parse(time.RFC3339, dateToStr)
		if err != nil {
			return nil, errors.New("invalid date_to parameter: must be RFC3339 format")
		}
		filters.DateTo = &dateTo
	}

	// Validate date range
	if filters.DateFrom != nil && filters.DateTo != nil && filters.DateFrom.After(*filters.DateTo) {
		return nil, errors.New("invalid date range: date_from must be before date_to")
	}

	// Tags filter (comma-separated)
	if tagsStr := query.Get("tags"); tagsStr != "" {
		tags := strings.Split(tagsStr, ",")
		for i, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			// Sanitize tag names
			if strings.Contains(tag, "..") || strings.Contains(tag, "/") || strings.Contains(tag, "\\") {
				return nil, errors.New("invalid tags parameter: contains unsafe characters")
			}
			tags[i] = strings.ToLower(tag)
		}
		filters.Tags = tags
	}

	return filters, nil
}

// getFilesWithMetadata retrieves file information for a session
func getFilesWithMetadata(sessionID string) ([]FileSearchResult, error) {
	var results []FileSearchResult

	// Try to get files from manifest first
	records := manifest.List()
	manifestFiles := make(map[string]manifest.FileRecord)
	for _, record := range records {
		if record.SessionID == sessionID {
			manifestFiles[record.Filename] = record
		}
	}

	sessionDir := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)

	if len(manifestFiles) > 0 {
		// Use manifest data
		for filename, record := range manifestFiles {
			filePath := filepath.Join(sessionDir, filename)

			// Get file size from disk (if file exists)
			var size int64
			if stat, err := os.Stat(filePath); err == nil {
				size = stat.Size()
			}

			// Extract file extension
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))

			results = append(results, FileSearchResult{
				Name:       filename,
				Size:       size,
				UploadTime: record.UploadedAt,
				Type:       ext,
				Tags:       record.Tags,
			})
		}
	} else {
		// Fallback: read directory
		entries, err := os.ReadDir(sessionDir)
		if err != nil {
			if os.IsNotExist(err) {
				return []FileSearchResult{}, nil
			}
			return nil, err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			filename := entry.Name()
			filePath := filepath.Join(sessionDir, filename)

			// Get file info
			stat, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			// Extract file extension
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))

			results = append(results, FileSearchResult{
				Name:       filename,
				Size:       stat.Size(),
				UploadTime: stat.ModTime(),
				Type:       ext,
				Tags:       nil, // No tags available from filesystem
			})
		}
	}

	return results, nil
}

// applyFilters filters the file list based on search criteria
func applyFilters(files []FileSearchResult, filters *SearchFilters) []FileSearchResult {
	var results []FileSearchResult

	for _, file := range files {
		// Name filter (substring match, case-insensitive)
		if filters.Name != "" {
			if !strings.Contains(strings.ToLower(file.Name), filters.Name) {
				continue
			}
		}

		// Type filter (file extension match)
		if filters.Type != "" {
			if file.Type != filters.Type {
				continue
			}
		}

		// Date range filters
		if filters.DateFrom != nil {
			if file.UploadTime.Before(*filters.DateFrom) {
				continue
			}
		}

		if filters.DateTo != nil {
			if file.UploadTime.After(*filters.DateTo) {
				continue
			}
		}

		// Tags filter (all specified tags must be present)
		if len(filters.Tags) > 0 {
			if file.Tags == nil {
				continue // File has no tags, but tags are required
			}

			hasAllTags := true
			for _, requiredTag := range filters.Tags {
				found := false
				for tagKey, tagValue := range file.Tags {
					if strings.ToLower(tagKey) == requiredTag || strings.ToLower(tagValue) == requiredTag {
						found = true
						break
					}
				}
				if !found {
					hasAllTags = false
					break
				}
			}

			if !hasAllTags {
				continue
			}
		}

		results = append(results, file)
	}

	return results
}
