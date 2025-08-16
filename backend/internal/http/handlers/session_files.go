package handlers

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"familyvault/internal/core/drive"

	"github.com/gorilla/mux"
)

// FileInfo represents a file in the session
type FileInfo struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	ModifiedTime string `json:"modified_time"`
	Type         string `json:"type"` // "active" or "backup"
}

// FilesQueryParams holds query parameters for file listing
type FilesQueryParams struct {
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	Ext     string `json:"ext"`
	MinSize int64  `json:"min_size"`
	MaxSize int64  `json:"max_size"`
	SortBy  string `json:"sort_by"` // name, size, modified_time
	Order   string `json:"order"`   // asc, desc
}

// FilesResponse represents the response structure
type FilesResponse struct {
	SessionID  string     `json:"session_id"`
	TotalFiles int        `json:"total_files"`
	Files      []FileInfo `json:"files"`
	Limit      int        `json:"limit"`
	Offset     int        `json:"offset"`
}

// SessionFilesHandler handles GET /sessions/:id/files
func SessionFilesHandler(w http.ResponseWriter, r *http.Request) {
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

	// For files, we allow viewing any session if user has valid authentication
	// In production, implement proper role-based access control

	// Parse query parameters
	params, err := parseFilesQueryParams(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid query parameters: %v", err), http.StatusBadRequest)
		return
	}

	// Get files for the session
	files, err := getSessionFiles(sessionID, params)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create response
	response := FilesResponse{
		SessionID:  sessionID,
		TotalFiles: len(files),
		Files:      files,
		Limit:      params.Limit,
		Offset:     params.Offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// parseFilesQueryParams parses and validates query parameters
func parseFilesQueryParams(r *http.Request) (*FilesQueryParams, error) {
	params := &FilesQueryParams{
		Limit:  1000,   // Default limit
		Offset: 0,      // Default offset
		SortBy: "name", // Default sort
		Order:  "asc",  // Default order
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

	// Parse extension filter
	if ext := r.URL.Query().Get("ext"); ext != "" {
		// Remove leading dot if present
		if strings.HasPrefix(ext, ".") {
			ext = ext[1:]
		}
		params.Ext = strings.ToLower(ext)
	}

	// Parse min_size
	if minSizeStr := r.URL.Query().Get("min_size"); minSizeStr != "" {
		minSize, err := strconv.ParseInt(minSizeStr, 10, 64)
		if err != nil || minSize < 0 {
			return nil, fmt.Errorf("min_size must be non-negative")
		}
		params.MinSize = minSize
	}

	// Parse max_size
	if maxSizeStr := r.URL.Query().Get("max_size"); maxSizeStr != "" {
		maxSize, err := strconv.ParseInt(maxSizeStr, 10, 64)
		if err != nil || maxSize < 0 {
			return nil, fmt.Errorf("max_size must be non-negative")
		}
		params.MaxSize = maxSize
	}

	// Validate size range
	if params.MinSize > 0 && params.MaxSize > 0 && params.MinSize > params.MaxSize {
		return nil, fmt.Errorf("min_size cannot be greater than max_size")
	}

	// Parse sort_by
	if sortBy := r.URL.Query().Get("sort_by"); sortBy != "" {
		switch sortBy {
		case "name", "size", "modified_time":
			params.SortBy = sortBy
		default:
			return nil, fmt.Errorf("sort_by must be one of: name, size, modified_time")
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

// getSessionFiles retrieves files for a session with filtering and pagination
func getSessionFiles(sessionID string, params *FilesQueryParams) ([]FileInfo, error) {
	var allFiles []FileInfo

	// Try to get files from active session first
	activeFiles, activeErr := getFilesFromDirectory(sessionID, "active", params)
	if activeErr == nil {
		allFiles = append(allFiles, activeFiles...)
	}

	// If no active session or we want to include backup files, check backup
	if activeErr != nil {
		backupFiles, backupErr := getFilesFromBackup(sessionID, params)
		if backupErr != nil {
			// If both active and backup failed, return the original error
			if activeErr != nil {
				return nil, activeErr
			}
			return nil, backupErr
		}
		allFiles = append(allFiles, backupFiles...)
	}

	// If no files found at all
	if len(allFiles) == 0 {
		return nil, os.ErrNotExist
	}

	// Apply sorting
	sortFiles(allFiles, params.SortBy, params.Order)

	// Apply pagination
	start := params.Offset
	end := start + params.Limit

	if start >= len(allFiles) {
		return []FileInfo{}, nil
	}

	if end > len(allFiles) {
		end = len(allFiles)
	}

	return allFiles[start:end], nil
}

// getFilesFromDirectory gets files from active session directory
func getFilesFromDirectory(sessionID, fileType string, params *FilesQueryParams) ([]FileInfo, error) {
	sessionPath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)

	// Check if directory exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil, err
	}

	var files []FileInfo

	err := filepath.WalkDir(sessionPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip log files
		if strings.HasSuffix(d.Name(), ".log") {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return err
		}

		// Apply filters
		if !matchesFilters(d.Name(), info.Size(), params) {
			return nil
		}

		// Get relative path from session directory
		relPath, err := filepath.Rel(sessionPath, path)
		if err != nil {
			relPath = d.Name()
		}

		files = append(files, FileInfo{
			Name:         relPath,
			Size:         info.Size(),
			ModifiedTime: info.ModTime().UTC().Format(time.RFC3339),
			Type:         fileType,
		})

		return nil
	})

	return files, err
}

// getFilesFromBackup gets files from backup directory
func getFilesFromBackup(sessionID string, params *FilesQueryParams) ([]FileInfo, error) {
	backupPath, err := findSessionBackupPath(sessionID)
	if err != nil {
		return nil, err
	}

	var files []FileInfo

	err = filepath.WalkDir(backupPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip log files
		if strings.HasSuffix(d.Name(), ".log") {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return err
		}

		// Apply filters
		if !matchesFilters(d.Name(), info.Size(), params) {
			return nil
		}

		// Get relative path from backup directory
		relPath, err := filepath.Rel(backupPath, path)
		if err != nil {
			relPath = d.Name()
		}

		files = append(files, FileInfo{
			Name:         relPath,
			Size:         info.Size(),
			ModifiedTime: info.ModTime().UTC().Format(time.RFC3339),
			Type:         "backup",
		})

		return nil
	})

	return files, err
}

// findSessionBackupPath finds the backup directory for a session
func findSessionBackupPath(sessionID string) (string, error) {
	backupBasePath := filepath.Join(drive.GetDrivePath(), "backup")

	// Check if backup directory exists
	if _, err := os.Stat(backupBasePath); os.IsNotExist(err) {
		return "", os.ErrNotExist
	}

	var foundPath string
	var latestTime time.Time

	// Walk through backup directories to find the session
	err := filepath.WalkDir(backupBasePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == sessionID {
			// Get the modification time of this backup
			info, err := d.Info()
			if err != nil {
				return err
			}

			// Keep track of the most recent backup
			if foundPath == "" || info.ModTime().After(latestTime) {
				foundPath = path
				latestTime = info.ModTime()
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if foundPath == "" {
		return "", os.ErrNotExist
	}

	return foundPath, nil
}

// matchesFilters checks if a file matches the specified filters
func matchesFilters(filename string, size int64, params *FilesQueryParams) bool {
	// Extension filter
	if params.Ext != "" {
		ext := strings.ToLower(filepath.Ext(filename))
		if ext != "" && ext[1:] != params.Ext { // Remove leading dot
			return false
		}
		if ext == "" && params.Ext != "" {
			return false
		}
	}

	// Size filters
	if params.MinSize > 0 && size < params.MinSize {
		return false
	}
	if params.MaxSize > 0 && size > params.MaxSize {
		return false
	}

	return true
}

// sortFiles sorts the files based on the specified criteria
func sortFiles(files []FileInfo, sortBy, order string) {
	sort.Slice(files, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "size":
			less = files[i].Size < files[j].Size
		case "modified_time":
			// Parse timestamps for comparison
			timeI, errI := time.Parse(time.RFC3339, files[i].ModifiedTime)
			timeJ, errJ := time.Parse(time.RFC3339, files[j].ModifiedTime)
			if errI != nil || errJ != nil {
				// Fallback to string comparison if parsing fails
				less = files[i].ModifiedTime < files[j].ModifiedTime
			} else {
				less = timeI.Before(timeJ)
			}
		default: // "name"
			less = files[i].Name < files[j].Name
		}

		if order == "desc" {
			return !less
		}
		return less
	})
}
