package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"familyvault/internal/core/session"
)

type activeSessionResponse struct {
	SessionID        string    `json:"session_id"`
	CreatedAt        time.Time `json:"created_at"`
	Expires          time.Time `json:"expires"`
	RemainingMinutes int       `json:"remaining_minutes"`
}

// GET /sessions/active
// Returns all active (non-expired) sessions and their metadata.
func listActiveSessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	now := time.Now()
	sessions := session.GetAllActive()
	resp := make([]activeSessionResponse, 0, len(sessions))
	for _, s := range sessions {
		remaining := int(s.Expires.Sub(now).Minutes())
		if remaining < 0 {
			remaining = 0
		}
		resp = append(resp, activeSessionResponse{
			SessionID:        s.ID,
			CreatedAt:        s.CreatedAt,
			Expires:          s.Expires,
			RemainingMinutes: remaining,
		})
	}

	// Sort by CreatedAt DESC (newest first)
	sort.Slice(resp, func(i, j int) bool { return resp[i].CreatedAt.After(resp[j].CreatedAt) })

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
