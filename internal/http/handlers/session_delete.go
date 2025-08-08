package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"path"
	"strings"

	"familyvault/internal/core/session"
)

type revokeResponse struct {
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
}

// DELETE /sessions/{session_id}
// Revokes the specified session if it belongs to the requester.
func deleteSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Extract session_id from path
	// Expected path: /sessions/{session_id}
	sessionIDFromPath := strings.TrimPrefix(r.URL.Path, "/sessions/")
	sessionIDFromPath = path.Base(sessionIDFromPath)
	if sessionIDFromPath == "" || sessionIDFromPath == "sessions" {
		httpError(w, http.StatusBadRequest, "invalid session id in path")
		return
	}

	// Resolve and validate authenticated session (header or query)
	authSessionID := r.Header.Get("X-Session-ID")
	if authSessionID == "" {
		authSessionID = r.URL.Query().Get("session_id")
	}
	if authSessionID == "" {
		httpError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	// Ensure user is revoking their own session
	if sessionIDFromPath != authSessionID {
		httpError(w, http.StatusForbidden, "cannot revoke another user's session")
		return
	}

	// Revoke
	if err := session.Revoke(sessionIDFromPath); err != nil {
		if err == session.ErrSessionNotFound {
			httpError(w, http.StatusNotFound, "session not found or already revoked/expired")
			return
		}
		log.Printf("session revoke error: session=%s ip=%s err=%v", sessionIDFromPath, r.RemoteAddr, err)
		httpError(w, http.StatusInternalServerError, "failed to revoke session")
		return
	}

	log.Printf("session revoked: session=%s ip=%s", sessionIDFromPath, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(revokeResponse{Status: "revoked", SessionID: sessionIDFromPath})
}
