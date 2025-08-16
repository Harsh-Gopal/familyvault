package groups

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"familyvault/internal/core/rbac"
)

// Store manages groups, users, memberships, devices, and pairing tokens
type Store struct {
	mu sync.RWMutex

	// In-memory maps
	groups      map[string]*Group
	users       map[string]*User
	memberships map[string]*Membership // key: groupID:userID
	devices     map[string]*Device
	tokens      map[string]*PairingToken

	// Persistence
	dataPath string
}

// NewStore creates a new group store
func NewStore(dataPath string) (*Store, error) {
	store := &Store{
		groups:      make(map[string]*Group),
		users:       make(map[string]*User),
		memberships: make(map[string]*Membership),
		devices:     make(map[string]*Device),
		tokens:      make(map[string]*PairingToken),
		dataPath:    dataPath,
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Load existing data
	if err := store.load(); err != nil {
		return nil, fmt.Errorf("failed to load data: %w", err)
	}

	return store, nil
}

// CreateGroup creates a new group with the specified owner
func (s *Store) CreateGroup(name string, ownerUserID string) (*Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	groupID := generateID()
	group := &Group{
		ID:        groupID,
		Name:      name,
		OwnerUser: ownerUserID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.groups[groupID] = group

	if err := s.persistGroups(); err != nil {
		delete(s.groups, groupID)
		return nil, err
	}

	return group, nil
}

// AddUser creates a new user
func (s *Store) AddUser(displayName, email, phone string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID := generateID()
	user := &User{
		ID:          userID,
		DisplayName: displayName,
		Email:       email,
		Phone:       phone,
		CreatedAt:   time.Now(),
	}

	s.users[userID] = user

	if err := s.persistUsers(); err != nil {
		delete(s.users, userID)
		return nil, err
	}

	return user, nil
}

// FindUserByContact finds a user by email or phone
func (s *Store) FindUserByContact(contact string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.Email == contact || user.Phone == contact {
			return user, true
		}
	}
	return nil, false
}

// AddMembership creates a new membership
func (s *Store) AddMembership(groupID, userID string, role rbac.Role, status MembershipStatus) (*Membership, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := membershipKey(groupID, userID)
	membership := &Membership{
		GroupID:   groupID,
		UserID:    userID,
		Role:      role,
		Status:    status,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.memberships[key] = membership

	if err := s.persistMemberships(); err != nil {
		delete(s.memberships, key)
		return nil, err
	}

	return membership, nil
}

// UpdateMembershipRole updates a user's role in a group
func (s *Store) UpdateMembershipRole(groupID, userID string, newRole rbac.Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := membershipKey(groupID, userID)
	membership, exists := s.memberships[key]
	if !exists {
		return fmt.Errorf("membership not found")
	}

	membership.Role = newRole
	membership.UpdatedAt = time.Now()

	return s.persistMemberships()
}

// UpdateMembershipStatus updates a user's membership status
func (s *Store) UpdateMembershipStatus(groupID, userID string, status MembershipStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := membershipKey(groupID, userID)
	membership, exists := s.memberships[key]
	if !exists {
		return fmt.Errorf("membership not found")
	}

	membership.Status = status
	membership.UpdatedAt = time.Now()

	return s.persistMemberships()
}

// GetUserRole returns the user's role in a group
func (s *Store) GetUserRole(groupID, userID string) (rbac.Role, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := membershipKey(groupID, userID)
	membership, exists := s.memberships[key]
	if !exists || !membership.IsActive() {
		return "", false
	}

	return membership.Role, true
}

// GetMembership returns a user's membership in a group
func (s *Store) GetMembership(groupID, userID string) (*Membership, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := membershipKey(groupID, userID)
	membership, exists := s.memberships[key]
	return membership, exists
}

// ListGroupMembers returns all memberships for a group
func (s *Store) ListGroupMembers(groupID string) ([]*Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var members []*Membership
	for _, membership := range s.memberships {
		if membership.GroupID == groupID {
			members = append(members, membership)
		}
	}

	return members, nil
}

// ListUserGroups returns all groups a user belongs to
func (s *Store) ListUserGroups(userID string) ([]*Group, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var groups []*Group
	for _, membership := range s.memberships {
		if membership.UserID == userID && membership.IsActive() {
			if group, exists := s.groups[membership.GroupID]; exists {
				groups = append(groups, group)
			}
		}
	}

	return groups, nil
}

// RegisterDevice creates a new device
func (s *Store) RegisterDevice(groupID, userID, deviceName string) (*Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deviceID := generateID()
	device := &Device{
		ID:       deviceID,
		UserID:   userID,
		GroupID:  groupID,
		Name:     deviceName,
		Approved: false,
		LastSeen: time.Now(),
	}

	s.devices[deviceID] = device

	if err := s.persistDevices(); err != nil {
		delete(s.devices, deviceID)
		return nil, err
	}

	return device, nil
}

// ApproveDevice approves a device for use
func (s *Store) ApproveDevice(groupID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, exists := s.devices[deviceID]
	if !exists {
		return fmt.Errorf("device not found")
	}

	if device.GroupID != groupID {
		return fmt.Errorf("device does not belong to group")
	}

	device.Approved = true
	device.LastSeen = time.Now()

	return s.persistDevices()
}

// GetDevice returns a device by ID
func (s *Store) GetDevice(deviceID string) (*Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	device, exists := s.devices[deviceID]
	return device, exists
}

// IsDeviceApproved checks if a device is approved
func (s *Store) IsDeviceApproved(deviceID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	device, exists := s.devices[deviceID]
	return exists && device.Approved
}

// UpdateDeviceLastSeen updates the last seen time for a device
func (s *Store) UpdateDeviceLastSeen(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, exists := s.devices[deviceID]
	if !exists {
		return fmt.Errorf("device not found")
	}

	device.LastSeen = time.Now()
	return s.persistDevices()
}

// IssuePairingToken creates a new pairing token
func (s *Store) IssuePairingToken(groupID, issuedTo, createdBy string, ttl time.Duration) (*PairingToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokenID := generateID()
	token := generateToken()

	pairingToken := &PairingToken{
		ID:        tokenID,
		GroupID:   groupID,
		IssuedTo:  issuedTo,
		Token:     token,
		ExpiresAt: time.Now().Add(ttl),
		Used:      false,
		CreatedBy: createdBy,
		CreatedAt: time.Now(),
	}

	s.tokens[tokenID] = pairingToken

	if err := s.persistTokens(); err != nil {
		delete(s.tokens, tokenID)
		return nil, err
	}

	return pairingToken, nil
}

// ConsumePairingToken marks a pairing token as used and returns it
func (s *Store) ConsumePairingToken(token string) (*PairingToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, pairingToken := range s.tokens {
		if pairingToken.Token == token {
			if pairingToken.Used {
				return nil, fmt.Errorf("token already used")
			}
			if pairingToken.IsExpired() {
				return nil, fmt.Errorf("token expired")
			}

			pairingToken.Used = true
			if err := s.persistTokens(); err != nil {
				return nil, err
			}

			return pairingToken, nil
		}
	}

	return nil, fmt.Errorf("token not found")
}

// GetGroup returns a group by ID
func (s *Store) GetGroup(groupID string) (*Group, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	group, exists := s.groups[groupID]
	return group, exists
}

// GetUser returns a user by ID
func (s *Store) GetUser(userID string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[userID]
	return user, exists
}

// UpdateUsedBytes updates the used bytes for a user in a group
func (s *Store) UpdateUsedBytes(groupID, userID string, delta int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := membershipKey(groupID, userID)
	membership, exists := s.memberships[key]
	if !exists {
		return fmt.Errorf("membership not found")
	}

	membership.UsedBytes += delta
	if membership.UsedBytes < 0 {
		membership.UsedBytes = 0
	}

	return s.persistMemberships()
}

// Helper functions

func membershipKey(groupID, userID string) string {
	return groupID + ":" + userID
}

func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Persistence methods

func (s *Store) load() error {
	if err := s.loadGroups(); err != nil {
		return err
	}
	if err := s.loadUsers(); err != nil {
		return err
	}
	if err := s.loadMemberships(); err != nil {
		return err
	}
	if err := s.loadDevices(); err != nil {
		return err
	}
	if err := s.loadTokens(); err != nil {
		return err
	}
	return nil
}

func (s *Store) loadGroups() error {
	path := filepath.Join(s.dataPath, "groups.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var groups []*Group
	if err := json.Unmarshal(data, &groups); err != nil {
		return err
	}

	for _, group := range groups {
		s.groups[group.ID] = group
	}
	return nil
}

func (s *Store) persistGroups() error {
	path := filepath.Join(s.dataPath, "groups.json")

	var groups []*Group
	for _, group := range s.groups {
		groups = append(groups, group)
	}

	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *Store) loadUsers() error {
	path := filepath.Join(s.dataPath, "users.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var users []*User
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	for _, user := range users {
		s.users[user.ID] = user
	}
	return nil
}

func (s *Store) persistUsers() error {
	path := filepath.Join(s.dataPath, "users.json")

	var users []*User
	for _, user := range s.users {
		users = append(users, user)
	}

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *Store) loadMemberships() error {
	path := filepath.Join(s.dataPath, "memberships.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var memberships []*Membership
	if err := json.Unmarshal(data, &memberships); err != nil {
		return err
	}

	for _, membership := range memberships {
		key := membershipKey(membership.GroupID, membership.UserID)
		s.memberships[key] = membership
	}
	return nil
}

func (s *Store) persistMemberships() error {
	path := filepath.Join(s.dataPath, "memberships.json")

	var memberships []*Membership
	for _, membership := range s.memberships {
		memberships = append(memberships, membership)
	}

	data, err := json.MarshalIndent(memberships, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *Store) loadDevices() error {
	path := filepath.Join(s.dataPath, "devices.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var devices []*Device
	if err := json.Unmarshal(data, &devices); err != nil {
		return err
	}

	for _, device := range devices {
		s.devices[device.ID] = device
	}
	return nil
}

func (s *Store) persistDevices() error {
	path := filepath.Join(s.dataPath, "devices.json")

	var devices []*Device
	for _, device := range s.devices {
		devices = append(devices, device)
	}

	data, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *Store) loadTokens() error {
	path := filepath.Join(s.dataPath, "tokens.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var tokens []*PairingToken
	if err := json.Unmarshal(data, &tokens); err != nil {
		return err
	}

	for _, token := range tokens {
		s.tokens[token.ID] = token
	}
	return nil
}

func (s *Store) persistTokens() error {
	path := filepath.Join(s.dataPath, "tokens.json")

	var tokens []*PairingToken
	for _, token := range s.tokens {
		tokens = append(tokens, token)
	}

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
