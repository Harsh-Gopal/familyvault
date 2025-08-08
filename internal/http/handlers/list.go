package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
)

type listedFile struct {
	Filename   string            `json:"filename"`
	UploadedAt time.Time         `json:"uploaded_at"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// GET /list - returns manifest entries for the active session
func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		sessionID = r.URL.Query().Get("session_id")
	}
	current := session.Get()
	if current == nil || sessionID == "" || current.ID != sessionID || time.Now().After(current.Expires) {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	all := manifest.List()
	out := make([]listedFile, 0)
	for _, rec := range all {
		if rec.SessionID == sessionID {
			out = append(out, listedFile{
				Filename:   rec.Filename,
				UploadedAt: rec.UploadedAt,
				Tags:       rec.Tags,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
