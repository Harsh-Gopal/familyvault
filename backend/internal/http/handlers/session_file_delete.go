package handlers

import (
	"net/http"
	"os"
	"strings"

	"familyvault/internal/auth"

	"github.com/gorilla/mux"
)

// SessionFileDeleteHandler handles DELETE /sessions/:id/files/:filename
func SessionFileDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID and filename from URL
	vars := mux.Vars(r)
	sessionID := vars["id"]
	filename := vars["filename"]

	// Validate session ID
	if !isValidSessionID(sessionID) {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	// Validate filename
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Check admin permission
	user := auth.GetUserFromContext(r.Context())
	if user == nil || !auth.HasPermission(user.Role, "delete") {
		http.Error(w, "Forbidden: admin role required", http.StatusForbidden)
		return
	}

	// Find and delete file
	filePath, err := findSessionFile(sessionID, filename)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
