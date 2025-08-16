package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"familyvault/internal/auth/localjwt"
	"familyvault/internal/auth/middleware"
	"familyvault/internal/core/groups"
	"familyvault/internal/core/paths"
	"familyvault/internal/core/rbac"

	"github.com/gorilla/mux"
)

// GroupHandlers contains all group-related HTTP handlers
type GroupHandlers struct {
	store      *groups.Store
	jwtManager *localjwt.JWTManager
}

// NewGroupHandlers creates new group handlers
func NewGroupHandlers(store *groups.Store, jwtManager *localjwt.JWTManager) *GroupHandlers {
	return &GroupHandlers{
		store:      store,
		jwtManager: jwtManager,
	}
}

// CreateGroupRequest represents the request to create a new group
type CreateGroupRequest struct {
	Name             string `json:"name"`
	OwnerDisplayName string `json:"owner_display_name"`
	Email            string `json:"email,omitempty"`
	Phone            string `json:"phone,omitempty"`
}

// CreateGroupResponse represents the response after creating a group
type CreateGroupResponse struct {
	GroupID  string    `json:"group_id"`
	UserID   string    `json:"user_id"`
	DeviceID string    `json:"device_id"`
	Role     rbac.Role `json:"role"`
	Token    string    `json:"token"`
}

// CreateGroup handles POST /groups
func (h *GroupHandlers) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.OwnerDisplayName == "" {
		http.Error(w, "Name and owner_display_name are required", http.StatusBadRequest)
		return
	}

	// Create user first
	user, err := h.store.AddUser(req.OwnerDisplayName, req.Email, req.Phone)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Create group
	group, err := h.store.CreateGroup(req.Name, user.ID)
	if err != nil {
		http.Error(w, "Failed to create group", http.StatusInternalServerError)
		return
	}

	// Add owner membership
	_, err = h.store.AddMembership(group.ID, user.ID, rbac.RoleAdmin, groups.StatusActive)
	if err != nil {
		http.Error(w, "Failed to create membership", http.StatusInternalServerError)
		return
	}

	// Create device for the creator
	deviceName := r.Header.Get("X-Device-Name")
	if deviceName == "" {
		deviceName = "Default Device"
	}

	device, err := h.store.RegisterDevice(group.ID, user.ID, deviceName)
	if err != nil {
		http.Error(w, "Failed to register device", http.StatusInternalServerError)
		return
	}

	// Auto-approve the creator's device
	if err := h.store.ApproveDevice(group.ID, device.ID); err != nil {
		http.Error(w, "Failed to approve device", http.StatusInternalServerError)
		return
	}

	// Create group directories
	if err := paths.EnsureGroupDirectories(group.ID); err != nil {
		http.Error(w, "Failed to create group directories", http.StatusInternalServerError)
		return
	}

	// Issue JWT token
	token, err := h.jwtManager.IssueToken(group.ID, user.ID, device.ID, rbac.RoleAdmin, 24*time.Hour)
	if err != nil {
		http.Error(w, "Failed to issue token", http.StatusInternalServerError)
		return
	}

	response := CreateGroupResponse{
		GroupID:  group.ID,
		UserID:   user.ID,
		DeviceID: device.ID,
		Role:     rbac.RoleAdmin,
		Token:    token,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListGroups handles GET /groups
func (h *GroupHandlers) ListGroups(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	groups, err := h.store.ListUserGroups(claims.UserID)
	if err != nil {
		http.Error(w, "Failed to list groups", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

// GetGroup handles GET /groups/{group_id}
func (h *GroupHandlers) GetGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]

	group, exists := h.store.GetGroup(groupID)
	if !exists {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	// Get membership info
	members, err := h.store.ListGroupMembers(groupID)
	if err != nil {
		http.Error(w, "Failed to get group members", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"group":        group,
		"member_count": len(members),
		"members":      members,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// InviteMemberRequest represents the request to invite a member
type InviteMemberRequest struct {
	Contact    string `json:"contact"`
	TTLMinutes int    `json:"ttl_minutes,omitempty"`
}

// InviteMemberResponse represents the response after inviting a member
type InviteMemberResponse struct {
	PairingToken string `json:"pairing_token"`
	QR           string `json:"qr"`
}

// InviteMember handles POST /groups/{group_id}/members/invite
func (h *GroupHandlers) InviteMember(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]

	var req InviteMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Contact == "" {
		http.Error(w, "Contact is required", http.StatusBadRequest)
		return
	}

	if req.TTLMinutes <= 0 {
		req.TTLMinutes = 60 // Default 1 hour
	}

	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Issue pairing token
	ttl := time.Duration(req.TTLMinutes) * time.Minute
	token, err := h.store.IssuePairingToken(groupID, req.Contact, claims.UserID, ttl)
	if err != nil {
		http.Error(w, "Failed to issue pairing token", http.StatusInternalServerError)
		return
	}

	response := InviteMemberResponse{
		PairingToken: token.Token,
		QR:           fmt.Sprintf("familyvault://pair?token=%s", token.Token),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListMembers handles GET /groups/{group_id}/members
func (h *GroupHandlers) ListMembers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]

	members, err := h.store.ListGroupMembers(groupID)
	if err != nil {
		http.Error(w, "Failed to list members", http.StatusInternalServerError)
		return
	}

	// Enrich with user information
	var enrichedMembers []map[string]interface{}
	for _, member := range members {
		user, exists := h.store.GetUser(member.UserID)
		if !exists {
			continue
		}

		enrichedMember := map[string]interface{}{
			"membership": member,
			"user":       user,
		}
		enrichedMembers = append(enrichedMembers, enrichedMember)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(enrichedMembers)
}

// UpdateRoleRequest represents the request to update a member's role
type UpdateRoleRequest struct {
	Role string `json:"role"`
}

// UpdateMemberRole handles POST /groups/{group_id}/roles/{user_id}
func (h *GroupHandlers) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]
	userID := vars["user_id"]

	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if !rbac.IsValidRole(req.Role) {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	newRole := rbac.Role(req.Role)

	// Update role
	if err := h.store.UpdateMembershipRole(groupID, userID, newRole); err != nil {
		http.Error(w, "Failed to update role", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// RemoveMember handles DELETE /groups/{group_id}/members/{user_id}
func (h *GroupHandlers) RemoveMember(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]
	userID := vars["user_id"]

	// Set membership status to revoked
	if err := h.store.UpdateMembershipStatus(groupID, userID, groups.StatusRevoked); err != nil {
		http.Error(w, "Failed to remove member", http.StatusInternalServerError)
		return
	}

	// TODO: Disable all devices for this user in this group

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// PairRequest represents the request to pair a device
type PairRequest struct {
	Token      string `json:"token"`
	DeviceName string `json:"device_name"`
}

// PairResponse represents the response after pairing
type PairResponse struct {
	Pending  bool   `json:"pending"`
	GroupID  string `json:"group_id"`
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
}

// Pair handles POST /pair (no auth required)
func (h *GroupHandlers) Pair(w http.ResponseWriter, r *http.Request) {
	var req PairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Token == "" || req.DeviceName == "" {
		http.Error(w, "Token and device_name are required", http.StatusBadRequest)
		return
	}

	// Consume pairing token
	token, err := h.store.ConsumePairingToken(req.Token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusBadRequest)
		return
	}

	var userID string

	// Check if user already exists by contact
	if user, exists := h.store.FindUserByContact(token.IssuedTo); exists {
		userID = user.ID
	} else {
		// Create new user
		user, err := h.store.AddUser(token.IssuedTo, token.IssuedTo, "")
		if err != nil {
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
		userID = user.ID

		// Add pending membership
		_, err = h.store.AddMembership(token.GroupID, userID, rbac.RoleMember, groups.StatusPending)
		if err != nil {
			http.Error(w, "Failed to create membership", http.StatusInternalServerError)
			return
		}
	}

	// Register device (not approved yet)
	device, err := h.store.RegisterDevice(token.GroupID, userID, req.DeviceName)
	if err != nil {
		http.Error(w, "Failed to register device", http.StatusInternalServerError)
		return
	}

	response := PairResponse{
		Pending:  true,
		GroupID:  token.GroupID,
		UserID:   userID,
		DeviceID: device.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ApproveDeviceResponse represents the response after approving a device
type ApproveDeviceResponse struct {
	Token string    `json:"token"`
	Role  rbac.Role `json:"role"`
}

// ApproveDevice handles POST /groups/{group_id}/devices/{device_id}/approve
func (h *GroupHandlers) ApproveDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["group_id"]
	deviceID := vars["device_id"]

	device, exists := h.store.GetDevice(deviceID)
	if !exists {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	if device.GroupID != groupID {
		http.Error(w, "Device does not belong to group", http.StatusBadRequest)
		return
	}

	// Approve device
	if err := h.store.ApproveDevice(groupID, deviceID); err != nil {
		http.Error(w, "Failed to approve device", http.StatusInternalServerError)
		return
	}

	// Activate membership if pending
	if err := h.store.UpdateMembershipStatus(groupID, device.UserID, groups.StatusActive); err != nil {
		http.Error(w, "Failed to activate membership", http.StatusInternalServerError)
		return
	}

	// Get user role
	role, exists := h.store.GetUserRole(groupID, device.UserID)
	if !exists {
		role = rbac.RoleMember // Default role
	}

	// Issue JWT token
	token, err := h.jwtManager.IssueToken(groupID, device.UserID, deviceID, role, 24*time.Hour)
	if err != nil {
		http.Error(w, "Failed to issue token", http.StatusInternalServerError)
		return
	}

	response := ApproveDeviceResponse{
		Token: token,
		Role:  role,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// WhoAmI handles GET /me
func (h *GroupHandlers) WhoAmI(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, _ := h.store.GetUser(claims.UserID)
	group, _ := h.store.GetGroup(claims.GroupID)
	membership, _ := h.store.GetMembership(claims.GroupID, claims.UserID)

	response := map[string]interface{}{
		"claims":     claims,
		"user":       user,
		"group":      group,
		"membership": membership,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
