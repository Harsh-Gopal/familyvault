package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
)

// UpdateMetadataRequest represents the request body for updating metadata
type UpdateMetadataRequest struct {
	FileID   string                 `json:"file_id,omitempty"`
	Metadata map[string]interface{} `json:"metadata"`
}

// UpdateMetadataResponse represents the response for successful metadata update
type UpdateMetadataResponse struct {
	Success   bool                   `json:"success"`
	FileID    string                 `json:"file_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// PATCH /update-metadata
// Updates metadata for an uploaded file or for the entire active session.
// Requires session ID via header "X-Session-ID" or query parameter "session_id".
// Accepts JSON body with optional file_id and required metadata fields.
func updateMetadataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
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

	// Parse request body
	var req UpdateMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	// Validate metadata is provided
	if req.Metadata == nil || len(req.Metadata) == 0 {
		httpError(w, http.StatusBadRequest, "metadata field is required")
		return
	}

	// Sanitize metadata values
	sanitizedMetadata := sanitizeMetadata(req.Metadata)

	updateTime := time.Now()

	if req.FileID != "" {
		// Update file-level metadata
		success := manifest.UpdateFileMetadata(current.ID, req.FileID, sanitizedMetadata)
		if !success {
			log.Printf("update-metadata file not found: session=%s file_id=%s", current.ID, req.FileID)
			httpError(w, http.StatusNotFound, "file not found")
			return
		}

		// Create response
		response := UpdateMetadataResponse{
			Success:   true,
			FileID:    req.FileID,
			Metadata:  sanitizedMetadata,
			UpdatedAt: updateTime,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("update-metadata encode response error: session=%s file_id=%s err=%v", current.ID, req.FileID, err)
			httpError(w, http.StatusInternalServerError, "failed to encode response")
			return
		}

		log.Printf("update-metadata file success: session=%s file_id=%s", current.ID, req.FileID)
	} else {
		// Update session-level metadata
		manifest.UpdateSessionMetadata(current.ID, sanitizedMetadata)

		// Create response
		response := UpdateMetadataResponse{
			Success:   true,
			Metadata:  sanitizedMetadata,
			UpdatedAt: updateTime,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("update-metadata encode response error: session=%s err=%v", current.ID, err)
			httpError(w, http.StatusInternalServerError, "failed to encode response")
			return
		}

		log.Printf("update-metadata session success: session=%s", current.ID)
	}
}

// sanitizeMetadata cleans and validates metadata values to prevent XSS and other attacks
func sanitizeMetadata(metadata map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for key, value := range metadata {
		// Sanitize key
		cleanKey := sanitizeMetadataString(key)
		if cleanKey == "" {
			continue // Skip empty keys
		}

		// Sanitize value based on type
		switch v := value.(type) {
		case string:
			cleanValue := sanitizeMetadataString(v)
			if cleanValue != "" {
				sanitized[cleanKey] = cleanValue
			}
		case map[string]interface{}:
			// Recursively sanitize nested objects
			cleanValue := sanitizeMetadata(v)
			if len(cleanValue) > 0 {
				sanitized[cleanKey] = cleanValue
			}
		case []interface{}:
			// Sanitize arrays
			var cleanArray []interface{}
			for _, item := range v {
				if str, ok := item.(string); ok {
					cleanStr := sanitizeMetadataString(str)
					if cleanStr != "" {
						cleanArray = append(cleanArray, cleanStr)
					}
				} else {
					cleanArray = append(cleanArray, item)
				}
			}
			if len(cleanArray) > 0 {
				sanitized[cleanKey] = cleanArray
			}
		case bool, int, int64, float64:
			// Keep primitive types as-is
			sanitized[cleanKey] = v
		default:
			// Convert other types to string and sanitize
			str := strings.TrimSpace(string(fmt.Sprintf("%v", v)))
			cleanValue := sanitizeMetadataString(str)
			if cleanValue != "" {
				sanitized[cleanKey] = cleanValue
			}
		}
	}

	return sanitized
}

// sanitizeMetadataString sanitizes a string value for metadata
func sanitizeMetadataString(value string) string {
	// Trim whitespace
	value = strings.TrimSpace(value)

	// Check length limits
	if len(value) == 0 {
		return ""
	}
	if len(value) > 1000 {
		value = value[:1000] // Truncate very long values
	}

	// HTML escape to prevent XSS
	value = html.EscapeString(value)

	// Remove or replace dangerous characters
	value = strings.ReplaceAll(value, "\x00", "") // Remove null bytes
	value = strings.ReplaceAll(value, "\r", "")   // Remove carriage returns

	// Replace multiple consecutive whitespace with single space
	value = strings.Join(strings.Fields(value), " ")

	return value
}
