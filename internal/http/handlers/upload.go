package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
	"familyvault/internal/core/upload"
)

// uploadResponse describes a successful upload result.
type uploadResponse struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SessionID string `json:"session_id"`
	Filename  string `json:"filename"`
}

const (
	// 100 MiB max upload size to prevent abuse.
	maxUploadSizeBytes int64 = 100 * 1024 * 1024
	// Memory used by ParseMultipartForm before spilling to disk.
	parseFormMemoryBytes int64 = 32 * 1024 * 1024
)

// POST /upload
// Accepts multipart/form-data with a file field named "file" and a session ID
// provided via header "X-Session-ID" or form field "session_id".
// Stores the file under FAMILYVAULT_DRIVE_PATH/uploads/<SESSION_UUID>/original_filename
// with basic filename sanitization and size limits.
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Ensure the drive is available before proceeding.
	if !drive.IsDrivePlugged() {
		httpError(w, http.StatusBadRequest, "backup drive not available")
		return
	}

	// Enforce a hard cap on the request body size.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSizeBytes)

	// Parse multipart form (limit in-memory usage to 32 MiB).
	if err := r.ParseMultipartForm(parseFormMemoryBytes); err != nil {
		// Distinguish too large bodies from other parse errors.
		if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			httpError(w, http.StatusRequestEntityTooLarge, "upload exceeds 100MB limit")
			return
		}
		httpError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	// Validate session
	providedSessionID := r.Header.Get("X-Session-ID")
	if providedSessionID == "" {
		providedSessionID = r.FormValue("session_id")
	}
	if providedSessionID == "" {
		httpError(w, http.StatusUnauthorized, "missing session id")
		return
	}
	current := session.Get()
	if current == nil {
		httpError(w, http.StatusUnauthorized, "session expired or not open")
		return
	}
	if current.ID != providedSessionID || time.Now().After(current.Expires) {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	// Retrieve the uploaded file part. Expect field name "file".
	file, header, err := r.FormFile("file")
	if err != nil {
		httpError(w, http.StatusBadRequest, "missing or invalid file field 'file'")
		return
	}
	defer file.Close()

	// Sanitize the filename to prevent directory traversal.
	originalName := header.Filename
	sanitizedName := filepath.Base(originalName)
	if sanitizedName == "" || sanitizedName == "." || sanitizedName == string(filepath.Separator) {
		httpError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	// Prepare destination path: <drive>/uploads/<session-id>/<filename>
	destDir := filepath.Join(drive.GetDrivePath(), "uploads", current.ID)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		log.Printf("upload failed: session=%s ip=%s mkdir err=%v", current.ID, r.RemoteAddr, err)
		httpError(w, http.StatusInternalServerError, "failed to create destination directory")
		return
	}
	destPath := filepath.Join(destDir, sanitizedName)

	// Encrypt and save the file to destination path
	if err := upload.EncryptAndSave(file, destPath); err != nil {
		log.Printf("upload failed: session=%s ip=%s encrypt-save err=%v", current.ID, r.RemoteAddr, err)
		httpError(w, http.StatusInternalServerError, "failed to store file")
		return
	}

	// After encryption, get resulting file size
	info, statErr := os.Stat(destPath)
	written := int64(0)
	if statErr == nil {
		written = info.Size()
	}

	log.Printf("upload success: session=%s ip=%s filename=%s size=%d path=%s", current.ID, r.RemoteAddr, sanitizedName, written, destPath)

	// Capture optional metadata (example: tags via form field "tags" as raw string; future: JSON)
	tagsRaw := r.FormValue("tags")
	tags := map[string]string{}
	if strings.TrimSpace(tagsRaw) != "" {
		tags["raw"] = tagsRaw
	}
	manifest.Add(manifest.FileRecord{
		SessionID:  current.ID,
		Filename:   sanitizedName,
		UploadedAt: time.Now(),
		Tags:       tags,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":   "uploaded",
		"filename": sanitizedName,
	})
}

// httpError writes a JSON error response with the given status and message.
func httpError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
