package groups

import (
	"testing"
	"time"

	"familyvault/internal/core/rbac"
)

func TestStore_CreateGroup(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create a user first
	user, err := store.AddUser("Test User", "test@example.com", "")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a group
	group, err := store.CreateGroup("Test Group", user.ID)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	if group.Name != "Test Group" {
		t.Errorf("Expected group name 'Test Group', got %s", group.Name)
	}

	if group.OwnerUser != user.ID {
		t.Errorf("Expected owner user %s, got %s", user.ID, group.OwnerUser)
	}

	// Verify group was persisted
	retrievedGroup, exists := store.GetGroup(group.ID)
	if !exists {
		t.Error("Group should exist after creation")
	}

	if retrievedGroup.Name != group.Name {
		t.Errorf("Expected persisted group name %s, got %s", group.Name, retrievedGroup.Name)
	}
}

func TestStore_AddUser(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	user, err := store.AddUser("John Doe", "john@example.com", "+1234567890")
	if err != nil {
		t.Fatalf("Failed to add user: %v", err)
	}

	if user.DisplayName != "John Doe" {
		t.Errorf("Expected display name 'John Doe', got %s", user.DisplayName)
	}

	if user.Email != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got %s", user.Email)
	}

	if user.Phone != "+1234567890" {
		t.Errorf("Expected phone '+1234567890', got %s", user.Phone)
	}

	// Verify user was persisted
	retrievedUser, exists := store.GetUser(user.ID)
	if !exists {
		t.Error("User should exist after creation")
	}

	if retrievedUser.DisplayName != user.DisplayName {
		t.Errorf("Expected persisted display name %s, got %s", user.DisplayName, retrievedUser.DisplayName)
	}
}

func TestStore_FindUserByContact(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Add a user
	user, err := store.AddUser("Jane Doe", "jane@example.com", "+9876543210")
	if err != nil {
		t.Fatalf("Failed to add user: %v", err)
	}

	// Find by email
	foundUser, exists := store.FindUserByContact("jane@example.com")
	if !exists {
		t.Error("Should find user by email")
	}
	if foundUser.ID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, foundUser.ID)
	}

	// Find by phone
	foundUser, exists = store.FindUserByContact("+9876543210")
	if !exists {
		t.Error("Should find user by phone")
	}
	if foundUser.ID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, foundUser.ID)
	}

	// Should not find non-existent contact
	_, exists = store.FindUserByContact("nonexistent@example.com")
	if exists {
		t.Error("Should not find non-existent user")
	}
}

func TestStore_Membership(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create user and group
	user, err := store.AddUser("Test User", "test@example.com", "")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	group, err := store.CreateGroup("Test Group", user.ID)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Add membership
	membership, err := store.AddMembership(group.ID, user.ID, rbac.RoleAdmin, StatusActive)
	if err != nil {
		t.Fatalf("Failed to add membership: %v", err)
	}

	if membership.Role != rbac.RoleAdmin {
		t.Errorf("Expected role %s, got %s", rbac.RoleAdmin, membership.Role)
	}

	if membership.Status != StatusActive {
		t.Errorf("Expected status %s, got %s", StatusActive, membership.Status)
	}

	// Get user role
	role, exists := store.GetUserRole(group.ID, user.ID)
	if !exists {
		t.Error("Should find user role")
	}
	if role != rbac.RoleAdmin {
		t.Errorf("Expected role %s, got %s", rbac.RoleAdmin, role)
	}

	// Update role
	err = store.UpdateMembershipRole(group.ID, user.ID, rbac.RoleMember)
	if err != nil {
		t.Fatalf("Failed to update role: %v", err)
	}

	role, exists = store.GetUserRole(group.ID, user.ID)
	if !exists {
		t.Error("Should find updated user role")
	}
	if role != rbac.RoleMember {
		t.Errorf("Expected updated role %s, got %s", rbac.RoleMember, role)
	}

	// List group members
	members, err := store.ListGroupMembers(group.ID)
	if err != nil {
		t.Fatalf("Failed to list group members: %v", err)
	}

	if len(members) != 1 {
		t.Errorf("Expected 1 member, got %d", len(members))
	}

	if members[0].UserID != user.ID {
		t.Errorf("Expected member user ID %s, got %s", user.ID, members[0].UserID)
	}
}

func TestStore_Device(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create user and group
	user, err := store.AddUser("Test User", "test@example.com", "")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	group, err := store.CreateGroup("Test Group", user.ID)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Register device
	device, err := store.RegisterDevice(group.ID, user.ID, "iPhone 13")
	if err != nil {
		t.Fatalf("Failed to register device: %v", err)
	}

	if device.Name != "iPhone 13" {
		t.Errorf("Expected device name 'iPhone 13', got %s", device.Name)
	}

	if device.Approved {
		t.Error("Device should not be approved initially")
	}

	// Check device approval status
	if store.IsDeviceApproved(device.ID) {
		t.Error("Device should not be approved initially")
	}

	// Approve device
	err = store.ApproveDevice(group.ID, device.ID)
	if err != nil {
		t.Fatalf("Failed to approve device: %v", err)
	}

	if !store.IsDeviceApproved(device.ID) {
		t.Error("Device should be approved after approval")
	}

	// Get device
	retrievedDevice, exists := store.GetDevice(device.ID)
	if !exists {
		t.Error("Device should exist")
	}

	if !retrievedDevice.Approved {
		t.Error("Retrieved device should be approved")
	}
}

func TestStore_PairingToken(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create user and group
	user, err := store.AddUser("Test User", "test@example.com", "")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	group, err := store.CreateGroup("Test Group", user.ID)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Issue pairing token
	ttl := 1 * time.Hour
	token, err := store.IssuePairingToken(group.ID, "newuser@example.com", user.ID, ttl)
	if err != nil {
		t.Fatalf("Failed to issue pairing token: %v", err)
	}

	if token.IssuedTo != "newuser@example.com" {
		t.Errorf("Expected issued to 'newuser@example.com', got %s", token.IssuedTo)
	}

	if token.CreatedBy != user.ID {
		t.Errorf("Expected created by %s, got %s", user.ID, token.CreatedBy)
	}

	if token.Used {
		t.Error("Token should not be used initially")
	}

	if token.IsExpired() {
		t.Error("Token should not be expired initially")
	}

	// Consume token
	consumedToken, err := store.ConsumePairingToken(token.Token)
	if err != nil {
		t.Fatalf("Failed to consume pairing token: %v", err)
	}

	if consumedToken.ID != token.ID {
		t.Errorf("Expected consumed token ID %s, got %s", token.ID, consumedToken.ID)
	}

	if !consumedToken.Used {
		t.Error("Consumed token should be marked as used")
	}

	// Try to consume again (should fail)
	_, err = store.ConsumePairingToken(token.Token)
	if err == nil {
		t.Error("Should not be able to consume token twice")
	}
}

func TestStore_Persistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create store and add data
	store1, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	user, err := store1.AddUser("Test User", "test@example.com", "")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	group, err := store1.CreateGroup("Test Group", user.ID)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Create new store instance (should load persisted data)
	store2, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create second store: %v", err)
	}

	// Verify data was loaded
	retrievedUser, exists := store2.GetUser(user.ID)
	if !exists {
		t.Error("User should be loaded from persistence")
	}

	if retrievedUser.DisplayName != user.DisplayName {
		t.Errorf("Expected loaded user name %s, got %s", user.DisplayName, retrievedUser.DisplayName)
	}

	retrievedGroup, exists := store2.GetGroup(group.ID)
	if !exists {
		t.Error("Group should be loaded from persistence")
	}

	if retrievedGroup.Name != group.Name {
		t.Errorf("Expected loaded group name %s, got %s", group.Name, retrievedGroup.Name)
	}
}

func TestMembership_Methods(t *testing.T) {
	membership := &Membership{
		Status:     StatusActive,
		QuotaBytes: 1000,
		UsedBytes:  500,
	}

	if !membership.IsActive() {
		t.Error("Membership should be active")
	}

	if membership.IsPending() {
		t.Error("Membership should not be pending")
	}

	if membership.IsRevoked() {
		t.Error("Membership should not be revoked")
	}

	if !membership.HasQuota() {
		t.Error("Membership should have quota")
	}

	if membership.IsQuotaExceeded() {
		t.Error("Membership should not exceed quota")
	}

	remaining := membership.RemainingQuota()
	if remaining != 500 {
		t.Errorf("Expected remaining quota 500, got %d", remaining)
	}

	// Test quota exceeded
	membership.UsedBytes = 1500
	if !membership.IsQuotaExceeded() {
		t.Error("Membership should exceed quota")
	}

	remaining = membership.RemainingQuota()
	if remaining != 0 {
		t.Errorf("Expected remaining quota 0, got %d", remaining)
	}
}

func TestPairingToken_Methods(t *testing.T) {
	token := &PairingToken{
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Used:      false,
	}

	if token.IsExpired() {
		t.Error("Token should not be expired")
	}

	// Test expired token
	expiredToken := &PairingToken{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Used:      false,
	}

	if !expiredToken.IsExpired() {
		t.Error("Token should be expired")
	}
}
