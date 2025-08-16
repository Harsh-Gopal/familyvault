package handlers

import (
	"encoding/json"
	"net/http"

	"familyvault/internal/auth/middleware"
	"familyvault/internal/core/groups"
	"familyvault/internal/core/manifest"

	"github.com/gorilla/mux"
)

// UsageEntry represents usage information for a user
type UsageEntry struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	UsedBytes   int64  `json:"used_bytes"`
	QuotaBytes  int64  `json:"quota_bytes,omitempty"`
	FileCount   int    `json:"file_count"`
}

// UsageResponse represents the usage response
type UsageResponse struct {
	GroupID         string       `json:"group_id"`
	TotalUsedBytes  int64        `json:"total_used_bytes"`
	TotalQuotaBytes int64        `json:"total_quota_bytes,omitempty"`
	Users           []UsageEntry `json:"users"`
}

// GroupUsageHandler handles GET /groups/{group_id}/usage
func GroupUsageHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]

	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get group members
	members, err := store.ListGroupMembers(groupID)
	if err != nil {
		http.Error(w, "Failed to get group members", http.StatusInternalServerError)
		return
	}

	// Get manifest manager for usage calculation
	manifestManager, err := manifest.GetManager(groupID)
	if err != nil {
		http.Error(w, "Failed to get manifest", http.StatusInternalServerError)
		return
	}

	var users []UsageEntry
	var totalUsedBytes int64
	var totalQuotaBytes int64

	for _, member := range members {
		user, exists := store.GetUser(member.UserID)
		if !exists {
			continue
		}

		// Calculate usage from manifest
		usedBytes := manifestManager.GetUserUsage(member.UserID)

		// Count files (simplified - in a real implementation, you'd count from manifest)
		fileCount := 0
		entries := manifestManager.ListByUser(member.UserID)
		fileCount = len(entries)

		usageEntry := UsageEntry{
			UserID:      member.UserID,
			DisplayName: user.DisplayName,
			Role:        string(member.Role),
			UsedBytes:   usedBytes,
			FileCount:   fileCount,
		}

		if member.QuotaBytes > 0 {
			usageEntry.QuotaBytes = member.QuotaBytes
			totalQuotaBytes += member.QuotaBytes
		}

		users = append(users, usageEntry)
		totalUsedBytes += usedBytes
	}

	response := UsageResponse{
		GroupID:         groupID,
		TotalUsedBytes:  totalUsedBytes,
		TotalQuotaBytes: totalQuotaBytes,
		Users:           users,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
