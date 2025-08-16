package handlers

import (
	"encoding/json"
	"net/http"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/session"
)

type statusResponse struct {
	DrivePlugged bool `json:"drive_plugged"`
	SessionOpen  bool `json:"session_open"`
}

// GET /status
func statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	resp := statusResponse{
		DrivePlugged: drive.IsDrivePlugged(),
		SessionOpen:  session.IsOpen(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

