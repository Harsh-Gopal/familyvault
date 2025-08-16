package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"familyvault/internal/core/download"
	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
)

// GET /download-all
// Downloads all files for a session as a streaming ZIP archive.
// Requires session ID via header "X-Session-ID" or query parameter "session_id".
// Returns a ZIP file containing all decrypted files for the session.
func downloadAllHandler(w http.ResponseWriter, r *http.Request) {
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

	// Get files to include in ZIP
	filesToInclude, err := getFilesForSession(current.ID)
	if err != nil {
		log.Printf("download-all error getting files: session=%s err=%v", current.ID, err)
		httpError(w, http.StatusInternalServerError, "failed to get file list")
		return
	}

	if len(filesToInclude) == 0 {
		httpError(w, http.StatusNotFound, "no files")
		return
	}

	// Set response headers for ZIP download
	zipFilename := fmt.Sprintf("session_%s.zip", current.ID)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFilename))

	// Stream ZIP archive
	if err := streamZipArchive(w, current.ID, filesToInclude); err != nil {
		log.Printf("download-all stream error: session=%s err=%v", current.ID, err)
		// Headers already sent, can't send error response
		return
	}

	log.Printf("download-all success: session=%s files=%d", current.ID, len(filesToInclude))
}

// getFilesForSession returns the list of files to include in the ZIP archive.
// First tries to get files from manifest, falls back to reading directory.
func getFilesForSession(sessionID string) ([]string, error) {
	// Try to get files from manifest first
	records := manifest.List()
	var manifestFiles []string
	for _, record := range records {
		if record.SessionID == sessionID {
			manifestFiles = append(manifestFiles, record.Filename)
		}
	}

	if len(manifestFiles) > 0 {
		return manifestFiles, nil
	}

	// Fallback: read directory
	sessionDir := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var dirFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			dirFiles = append(dirFiles, entry.Name())
		}
	}

	return dirFiles, nil
}

// streamZipArchive creates and streams a ZIP archive containing all specified files.
// Uses io.Pipe to stream without buffering the entire archive in memory.
func streamZipArchive(w http.ResponseWriter, sessionID string, filenames []string) error {
	// Create pipe for streaming
	pr, pw := io.Pipe()
	defer pr.Close()

	// Track errors from the ZIP creation goroutine
	errChan := make(chan error, 1)

	// Create ZIP archive in a separate goroutine
	go func() {
		defer pw.Close()

		zipWriter := zip.NewWriter(pw)
		defer zipWriter.Close()

		var errors []string
		sessionDir := filepath.Join(drive.GetDrivePath(), "uploads", sessionID)

		for _, filename := range filenames {
			// Sanitize filename to prevent path traversal
			sanitizedName := filepath.Base(filename)
			if sanitizedName == "" || sanitizedName == "." || sanitizedName == "/" ||
				strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
				errors = append(errors, fmt.Sprintf("skipped unsafe filename: %s", filename))
				continue
			}

			filePath := filepath.Join(sessionDir, sanitizedName)

			// Check if file exists
			if _, err := os.Stat(filePath); err != nil {
				if os.IsNotExist(err) {
					errors = append(errors, fmt.Sprintf("file not found: %s", sanitizedName))
				} else {
					errors = append(errors, fmt.Sprintf("file access error: %s - %v", sanitizedName, err))
				}
				continue
			}

			// Create ZIP entry
			zipEntry, err := zipWriter.Create(sanitizedName)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to create zip entry: %s - %v", sanitizedName, err))
				continue
			}

			// Decrypt and copy file content to ZIP entry
			if err := download.DecryptAndStream(filePath, zipEntry); err != nil {
				errors = append(errors, fmt.Sprintf("decryption failed: %s - %v", sanitizedName, err))
				continue
			}
		}

		// If there were any errors, add an errors.log file to the ZIP
		if len(errors) > 0 {
			errorEntry, err := zipWriter.Create("errors.log")
			if err == nil {
				errorContent := fmt.Sprintf("Errors encountered during ZIP creation:\n\n%s\n", strings.Join(errors, "\n"))
				_, _ = errorEntry.Write([]byte(errorContent))
			}
		}

		errChan <- nil
	}()

	// Copy from pipe to response writer
	_, copyErr := io.Copy(w, pr)

	// Wait for ZIP creation to complete
	zipErr := <-errChan

	if zipErr != nil {
		return zipErr
	}
	return copyErr
}
