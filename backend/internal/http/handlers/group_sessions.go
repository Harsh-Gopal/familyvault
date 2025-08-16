package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"familyvault/internal/auth/middleware"
	"familyvault/internal/core/groups"
	"familyvault/internal/core/session"

	"github.com/gorilla/mux"
)

// GroupSessionOpenHandler handles POST /groups/{group_id}/sessions/open
func GroupSessionOpenHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]

	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get session manager for the group
	sessionManager := session.GetManager(groupID)

	// Check if there's already an active session
	if sessionManager.IsOpen() {
		http.Error(w, "Session already active", http.StatusConflict)
		return
	}

	// Parse request for custom duration (optional)
	var req struct {
		DurationMinutes int `json:"duration_minutes,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	duration := time.Hour // Default 1 hour
	if req.DurationMinutes > 0 {
		duration = time.Duration(req.DurationMinutes) * time.Minute
	}

	// Open new session
	sess, err := sessionManager.Open(claims.UserID, duration)
	if err != nil {
		http.Error(w, "Failed to open session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sess)
}

// GroupSessionCloseHandler handles POST /groups/{group_id}/sessions/close
func GroupSessionCloseHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]

	// Get session manager for the group
	sessionManager := session.GetManager(groupID)

	// Close the session
	sessionManager.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "closed"})
}

// GroupSessionActiveHandler handles GET /groups/{group_id}/sessions/active
func GroupSessionActiveHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]

	// Get session manager for the group
	sessionManager := session.GetManager(groupID)

	// Get active sessions
	activeSessions := sessionManager.GetAllActive()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activeSessions)
}

// GroupSessionHandler handles GET /groups/{group_id}/sessions/{session_id}
func GroupSessionHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]
	sessionID := vars["session_id"]

	// Get session manager for the group
	sessionManager := session.GetManager(groupID)

	// Get current session
	currentSession := sessionManager.Get()
	if currentSession == nil || currentSession.ID != sessionID {
		http.Error(w, "Session not found or not active", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(currentSession)
}

// GroupSessionStatusHandler handles GET /groups/{group_id}/sessions/{session_id}/status
func GroupSessionStatusHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]
	sessionID := vars["session_id"]

	// Get session manager for the group
	sessionManager := session.GetManager(groupID)

	// Get current session
	currentSession := sessionManager.Get()
	if currentSession == nil || currentSession.ID != sessionID {
		http.Error(w, "Session not found or not active", http.StatusNotFound)
		return
	}

	// Calculate remaining time
	now := time.Now()
	remaining := currentSession.Expires.Sub(now)
	if remaining < 0 {
		remaining = 0
	}

	status := map[string]interface{}{
		"session_id":        currentSession.ID,
		"group_id":          currentSession.GroupID,
		"started_by_user":   currentSession.StartedByUser,
		"created_at":        currentSession.CreatedAt,
		"expires":           currentSession.Expires,
		"remaining_seconds": int(remaining.Seconds()),
		"is_active":         remaining > 0,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// GroupSessionDeleteHandler handles DELETE /groups/{group_id}/sessions/{session_id}
func GroupSessionDeleteHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]
	sessionID := vars["session_id"]

	// Get session manager for the group
	sessionManager := session.GetManager(groupID)

	// Revoke the session
	if err := sessionManager.Revoke(sessionID); err != nil {
		if err == session.ErrSessionNotFound {
			http.Error(w, "Session not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to revoke session", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
}
