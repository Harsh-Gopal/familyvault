package handlers

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"familyvault/internal/core/drive"

	"github.com/gorilla/mux"
)

// ArtifactEntry represents a single artifact file
type ArtifactEntry struct {
	Name         string `json:"name"`
	SizeBytes    int64  `json:"size_bytes"`
	LastModified string `json:"last_modified"`
	Type         string `json:"type"`
}

// ArtifactsQueryParams holds query parameters for artifacts requests
type ArtifactsQueryParams struct {
	Type         string `json:"type"`
	NameContains string `json:"name_contains"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Limit        int    `json:"limit"`
	Offset       int    `json:"offset"`
	Order        string `json:"order"`
}

// ArtifactsResponse represents the response structure
type ArtifactsResponse struct {
	SessionID     string          `json:"session_id"`
	ArtifactCount int             `json:"artifact_count"`
	Artifacts     []ArtifactEntry `json:"artifacts"`
}

// SessionArtifactsHandler handles GET /sessions/:id/artifacts
func SessionArtifactsHandler(w http.ResponseWriter, r *http.Request) {
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

	// Resolve and validate authenticated session (header or query)
	authSessionID := r.Header.Get("X-Session-ID")
	if authSessionID == "" {
		authSessionID = r.URL.Query().Get("session_id")
	}
	if authSessionID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	params, err := parseArtifactsQueryParams(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid query parameters: %v", err), http.StatusBadRequest)
		return
	}

	// Get artifacts for the session
	artifacts, err := getSessionArtifacts(sessionID, params)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Session artifacts not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create response
	response := ArtifactsResponse{
		SessionID:     sessionID,
		ArtifactCount: len(artifacts),
		Artifacts:     artifacts,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// parseArtifactsQueryParams parses and validates query parameters
func parseArtifactsQueryParams(r *http.Request) (*ArtifactsQueryParams, error) {
	params := &ArtifactsQueryParams{
		Limit:  100,   // Default limit
		Offset: 0,     // Default offset
		Order:  "asc", // Default order
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 1000 {
			return nil, fmt.Errorf("limit must be between 1 and 1000")
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

	// Parse type filter
	if artifactType := r.URL.Query().Get("type"); artifactType != "" {
		// Validate MIME type format
		if !isValidMimeType(artifactType) {
			return nil, fmt.Errorf("invalid MIME type format")
		}
		params.Type = artifactType
	}

	// Parse name_contains filter
	if nameContains := r.URL.Query().Get("name_contains"); nameContains != "" {
		// Sanitize the search string
		params.NameContains = strings.ToLower(strings.TrimSpace(nameContains))
	}

	// Parse start_time
	if startTime := r.URL.Query().Get("start_time"); startTime != "" {
		_, err := time.Parse(time.RFC3339, startTime)
		if err != nil {
			return nil, fmt.Errorf("start_time must be in RFC3339 format")
		}
		params.StartTime = startTime
	}

	// Parse end_time
	if endTime := r.URL.Query().Get("end_time"); endTime != "" {
		_, err := time.Parse(time.RFC3339, endTime)
		if err != nil {
			return nil, fmt.Errorf("end_time must be in RFC3339 format")
		}
		params.EndTime = endTime
	}

	// Validate time range
	if params.StartTime != "" && params.EndTime != "" {
		startT, _ := time.Parse(time.RFC3339, params.StartTime)
		endT, _ := time.Parse(time.RFC3339, params.EndTime)
		if startT.After(endT) {
			return nil, fmt.Errorf("start_time cannot be after end_time")
		}
	}

	// Parse order
	if order := r.URL.Query().Get("order"); order != "" {
		switch order {
		case "asc", "desc":
			params.Order = order
		default:
			return nil, fmt.Errorf("order must be either 'asc' or 'desc'")
		}
	}

	return params, nil
}

// isValidMimeType validates MIME type format
func isValidMimeType(mimeType string) bool {
	// Basic MIME type validation (type/subtype)
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9!#$&\-\^_]*\/[a-zA-Z0-9][a-zA-Z0-9!#$&\-\^_.]*$`, mimeType)
	return matched
}

// getSessionArtifacts retrieves artifacts for a session with filtering and pagination
func getSessionArtifacts(sessionID string, params *ArtifactsQueryParams) ([]ArtifactEntry, error) {
	var allArtifacts []ArtifactEntry

	// Try to get artifacts from active session first
	activeArtifacts, activeErr := getArtifactsFromDirectory(sessionID, "active", params)
	if activeErr == nil {
		allArtifacts = append(allArtifacts, activeArtifacts...)
	}

	// If no active session or we want to include backup artifacts, check backup
	if activeErr != nil {
		backupArtifacts, backupErr := getArtifactsFromBackup(sessionID, params)
		if backupErr != nil {
			// If both active and backup failed, return the original error
			if activeErr != nil {
				return nil, activeErr
			}
			return nil, backupErr
		}
		allArtifacts = append(allArtifacts, backupArtifacts...)
	}

	// Remove duplicates (primary takes precedence)
	allArtifacts = removeDuplicateArtifacts(allArtifacts)

	// If no artifacts found at all
	if len(allArtifacts) == 0 {
		return nil, os.ErrNotExist
	}

	// Sort artifacts by modification time
	sortArtifacts(allArtifacts, params.Order)

	// Apply pagination
	start := params.Offset
	end := start + params.Limit

	if start >= len(allArtifacts) {
		return []ArtifactEntry{}, nil
	}

	if end > len(allArtifacts) {
		end = len(allArtifacts)
	}

	return allArtifacts[start:end], nil
}

// getArtifactsFromDirectory gets artifacts from active session directory
func getArtifactsFromDirectory(sessionID, artifactType string, params *ArtifactsQueryParams) ([]ArtifactEntry, error) {
	artifactsPath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID, "artifacts")

	// Check if artifacts directory exists
	if _, err := os.Stat(artifactsPath); os.IsNotExist(err) {
		return nil, err
	}

	var artifacts []ArtifactEntry

	err := filepath.WalkDir(artifactsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return err
		}

		// Get relative path from artifacts directory
		relPath, err := filepath.Rel(artifactsPath, path)
		if err != nil {
			relPath = d.Name()
		}

		// Detect MIME type
		mimeType := detectMimeType(d.Name())

		artifact := ArtifactEntry{
			Name:         relPath,
			SizeBytes:    info.Size(),
			LastModified: info.ModTime().UTC().Format(time.RFC3339),
			Type:         mimeType,
		}

		// Apply filters
		if matchesArtifactFilters(artifact, params) {
			artifacts = append(artifacts, artifact)
		}

		return nil
	})

	return artifacts, err
}

// getArtifactsFromBackup gets artifacts from backup directory
func getArtifactsFromBackup(sessionID string, params *ArtifactsQueryParams) ([]ArtifactEntry, error) {
	backupPath, err := findSessionBackupPath(sessionID)
	if err != nil {
		return nil, err
	}

	artifactsPath := filepath.Join(backupPath, "artifacts")
	if _, err := os.Stat(artifactsPath); os.IsNotExist(err) {
		return nil, err
	}

	var artifacts []ArtifactEntry

	err = filepath.WalkDir(artifactsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return err
		}

		// Get relative path from artifacts directory
		relPath, err := filepath.Rel(artifactsPath, path)
		if err != nil {
			relPath = d.Name()
		}

		// Detect MIME type
		mimeType := detectMimeType(d.Name())

		artifact := ArtifactEntry{
			Name:         relPath,
			SizeBytes:    info.Size(),
			LastModified: info.ModTime().UTC().Format(time.RFC3339),
			Type:         mimeType,
		}

		// Apply filters
		if matchesArtifactFilters(artifact, params) {
			artifacts = append(artifacts, artifact)
		}

		return nil
	})

	return artifacts, err
}

// detectMimeType detects MIME type from filename
func detectMimeType(filename string) string {
	ext := filepath.Ext(filename)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// Default to application/octet-stream for unknown types
		mimeType = "application/octet-stream"
	}
	return mimeType
}

// matchesArtifactFilters checks if an artifact matches the specified filters
func matchesArtifactFilters(artifact ArtifactEntry, params *ArtifactsQueryParams) bool {
	// Type filter (exact match)
	if params.Type != "" && artifact.Type != params.Type {
		return false
	}

	// Name contains filter (case-insensitive)
	if params.NameContains != "" {
		if !strings.Contains(strings.ToLower(artifact.Name), params.NameContains) {
			return false
		}
	}

	// Time range filters
	if params.StartTime != "" || params.EndTime != "" {
		artifactTime, err := time.Parse(time.RFC3339, artifact.LastModified)
		if err != nil {
			return false // Skip artifacts with unparseable timestamps
		}

		if params.StartTime != "" {
			startTime, _ := time.Parse(time.RFC3339, params.StartTime)
			if artifactTime.Before(startTime) {
				return false
			}
		}

		if params.EndTime != "" {
			endTime, _ := time.Parse(time.RFC3339, params.EndTime)
			if artifactTime.After(endTime) {
				return false
			}
		}
	}

	return true
}

// sortArtifacts sorts artifacts by modification time
func sortArtifacts(artifacts []ArtifactEntry, order string) {
	sort.Slice(artifacts, func(i, j int) bool {
		timeI, errI := time.Parse(time.RFC3339, artifacts[i].LastModified)
		timeJ, errJ := time.Parse(time.RFC3339, artifacts[j].LastModified)

		// Handle parsing errors by falling back to string comparison
		if errI != nil || errJ != nil {
			if order == "desc" {
				return artifacts[i].LastModified > artifacts[j].LastModified
			}
			return artifacts[i].LastModified < artifacts[j].LastModified
		}

		if order == "desc" {
			return timeI.After(timeJ)
		}
		return timeI.Before(timeJ)
	})
}

// removeDuplicateArtifacts removes duplicate artifacts (keeping first occurrence)
func removeDuplicateArtifacts(artifacts []ArtifactEntry) []ArtifactEntry {
	seen := make(map[string]bool)
	var result []ArtifactEntry

	for _, artifact := range artifacts {
		if !seen[artifact.Name] {
			seen[artifact.Name] = true
			result = append(result, artifact)
		}
	}

	return result
}
