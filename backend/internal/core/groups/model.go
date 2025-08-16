package groups

import (
	"time"

	"familyvault/internal/core/rbac"
)

// Group represents a family vault group
type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerUser string    `json:"owner_user"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents a user in the system
type User struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email,omitempty"`
	Phone       string    `json:"phone,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// MembershipStatus represents the status of a group membership
type MembershipStatus string

const (
	StatusActive  MembershipStatus = "active"
	StatusPending MembershipStatus = "pending"
	StatusRevoked MembershipStatus = "revoked"
)

// Membership represents a user's membership in a group
type Membership struct {
	GroupID    string           `json:"group_id"`
	UserID     string           `json:"user_id"`
	Role       rbac.Role        `json:"role"`
	Status     MembershipStatus `json:"status"`
	QuotaBytes int64            `json:"quota_bytes,omitempty"`
	UsedBytes  int64            `json:"used_bytes,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

// Device represents a device registered to a user in a group
type Device struct {
	ID       string    `json:"id"`
	UserID   string    `json:"user_id"`
	GroupID  string    `json:"group_id"`
	Name     string    `json:"name"`
	Approved bool      `json:"approved"`
	LastSeen time.Time `json:"last_seen"`
}

// PairingToken represents a token used for device pairing
type PairingToken struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	IssuedTo  string    `json:"issued_to"` // email/phone
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedBy string    `json:"created_by"` // admin user id
	CreatedAt time.Time `json:"created_at"`
}

// IsExpired checks if the pairing token has expired
func (pt *PairingToken) IsExpired() bool {
	return time.Now().After(pt.ExpiresAt)
}

// IsActive checks if membership is active
func (m *Membership) IsActive() bool {
	return m.Status == StatusActive
}

// IsPending checks if membership is pending
func (m *Membership) IsPending() bool {
	return m.Status == StatusPending
}

// IsRevoked checks if membership is revoked
func (m *Membership) IsRevoked() bool {
	return m.Status == StatusRevoked
}

// HasQuota checks if the membership has a quota set
func (m *Membership) HasQuota() bool {
	return m.QuotaBytes > 0
}

// IsQuotaExceeded checks if the used bytes exceed the quota
func (m *Membership) IsQuotaExceeded() bool {
	return m.HasQuota() && m.UsedBytes >= m.QuotaBytes
}

// RemainingQuota returns the remaining quota in bytes
func (m *Membership) RemainingQuota() int64 {
	if !m.HasQuota() {
		return -1 // Unlimited
	}
	remaining := m.QuotaBytes - m.UsedBytes
	if remaining < 0 {
		return 0
	}
	return remaining
}
