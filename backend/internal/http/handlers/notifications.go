package handlers

import (
	"encoding/json"
	"net/http"

	"familyvault/internal/auth/middleware"
	"familyvault/internal/core/groups"
	"familyvault/internal/notify"

	"github.com/gorilla/mux"
)

// NotificationHandlers contains notification-related HTTP handlers
type NotificationHandlers struct {
	store    *groups.Store
	notifier *notify.NotificationService
}

// NewNotificationHandlers creates new notification handlers
func NewNotificationHandlers(store *groups.Store, notifier *notify.NotificationService) *NotificationHandlers {
	return &NotificationHandlers{
		store:    store,
		notifier: notifier,
	}
}

// NotifyRequest represents the request to send notifications
type NotifyRequest struct {
	Message  string   `json:"message"`
	Channels []string `json:"channels"`
}

// NotifyResponse represents the response after sending notifications
type NotifyResponse struct {
	Sent   int `json:"sent"`
	Failed int `json:"failed"`
}

// NotifyMembers handles POST /groups/{group_id}/notify
func (h *NotificationHandlers) NotifyMembers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]

	var req NotifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	if len(req.Channels) == 0 {
		req.Channels = []string{"email"} // Default to email
	}

	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get all active members of the group
	members, err := h.store.ListGroupMembers(groupID)
	if err != nil {
		http.Error(w, "Failed to get group members", http.StatusInternalServerError)
		return
	}

	totalSent := 0
	totalFailed := 0

	// Send notifications to each active member
	for _, member := range members {
		if !member.IsActive() {
			continue
		}

		// Skip the sender
		if member.UserID == claims.UserID {
			continue
		}

		user, exists := h.store.GetUser(member.UserID)
		if !exists {
			totalFailed++
			continue
		}

		// Determine contact info based on channels
		for _, channel := range req.Channels {
			var contact string
			switch channel {
			case "email":
				contact = user.Email
			case "sms":
				contact = user.Phone
			default:
				totalFailed++
				continue
			}

			if contact == "" {
				totalFailed++
				continue
			}

			// Send notification
			sent, failed := h.notifier.SendMultiChannel(contact, "FamilyVault Notification", req.Message, []string{channel})
			totalSent += sent
			totalFailed += failed
		}
	}

	response := NotifyResponse{
		Sent:   totalSent,
		Failed: totalFailed,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
