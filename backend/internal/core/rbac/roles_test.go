package rbac

import "testing"

func TestPermissions(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		function func(Role) bool
		expected bool
	}{
		// Upload permissions
		{"admin can upload", RoleAdmin, CanUpload, true},
		{"member can upload", RoleMember, CanUpload, true},
		{"viewer cannot upload", RoleViewer, CanUpload, false},

		// Download permissions
		{"admin can download", RoleAdmin, CanDownload, true},
		{"member can download", RoleMember, CanDownload, true},
		{"viewer can download", RoleViewer, CanDownload, true},

		// Delete own permissions
		{"admin can delete own", RoleAdmin, CanDeleteOwn, true},
		{"member can delete own", RoleMember, CanDeleteOwn, true},
		{"viewer cannot delete own", RoleViewer, CanDeleteOwn, false},

		// Delete any permissions
		{"admin can delete any", RoleAdmin, CanDeleteAny, true},
		{"member cannot delete any", RoleMember, CanDeleteAny, false},
		{"viewer cannot delete any", RoleViewer, CanDeleteAny, false},

		// Manage members permissions
		{"admin can manage members", RoleAdmin, CanManageMembers, true},
		{"member cannot manage members", RoleMember, CanManageMembers, false},
		{"viewer cannot manage members", RoleViewer, CanManageMembers, false},

		// Start/stop sessions permissions
		{"admin can start/stop sessions", RoleAdmin, CanStartStopSessions, true},
		{"member cannot start/stop sessions", RoleMember, CanStartStopSessions, false},
		{"viewer cannot start/stop sessions", RoleViewer, CanStartStopSessions, false},

		// Manage metadata permissions
		{"admin can manage metadata", RoleAdmin, CanManageMetadata, true},
		{"member can manage metadata", RoleMember, CanManageMetadata, true},
		{"viewer cannot manage metadata", RoleViewer, CanManageMetadata, false},

		// View logs permissions
		{"admin can view logs", RoleAdmin, CanViewLogs, true},
		{"member can view logs", RoleMember, CanViewLogs, true},
		{"viewer can view logs", RoleViewer, CanViewLogs, true},

		// View metrics permissions
		{"admin can view metrics", RoleAdmin, CanViewMetrics, true},
		{"member can view metrics", RoleMember, CanViewMetrics, true},
		{"viewer can view metrics", RoleViewer, CanViewMetrics, true},

		// Notify members permissions
		{"admin can notify members", RoleAdmin, CanNotifyMembers, true},
		{"member cannot notify members", RoleMember, CanNotifyMembers, false},
		{"viewer cannot notify members", RoleViewer, CanNotifyMembers, false},

		// Manage devices permissions
		{"admin can manage devices", RoleAdmin, CanManageDevices, true},
		{"member cannot manage devices", RoleMember, CanManageDevices, false},
		{"viewer cannot manage devices", RoleViewer, CanManageDevices, false},

		// Invite members permissions
		{"admin can invite members", RoleAdmin, CanInviteMembers, true},
		{"member cannot invite members", RoleMember, CanInviteMembers, false},
		{"viewer cannot invite members", RoleViewer, CanInviteMembers, false},

		// Change roles permissions
		{"admin can change roles", RoleAdmin, CanChangeRoles, true},
		{"member cannot change roles", RoleMember, CanChangeRoles, false},
		{"viewer cannot change roles", RoleViewer, CanChangeRoles, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function(tt.role)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsValidRole(t *testing.T) {
	tests := []struct {
		role     string
		expected bool
	}{
		{"admin", true},
		{"member", true},
		{"viewer", true},
		{"invalid", false},
		{"", false},
		{"ADMIN", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := IsValidRole(tt.role)
			if result != tt.expected {
				t.Errorf("IsValidRole(%s) = %v, expected %v", tt.role, result, tt.expected)
			}
		})
	}
}

func TestGetRoleHierarchy(t *testing.T) {
	tests := []struct {
		role     Role
		expected int
	}{
		{RoleAdmin, 3},
		{RoleMember, 2},
		{RoleViewer, 1},
		{Role("invalid"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			result := GetRoleHierarchy(tt.role)
			if result != tt.expected {
				t.Errorf("GetRoleHierarchy(%s) = %d, expected %d", tt.role, result, tt.expected)
			}
		})
	}
}

func TestCanPromoteTo(t *testing.T) {
	tests := []struct {
		name        string
		currentRole Role
		targetRole  Role
		expected    bool
	}{
		{"admin can promote to admin", RoleAdmin, RoleAdmin, true},
		{"admin can promote to member", RoleAdmin, RoleMember, true},
		{"admin can promote to viewer", RoleAdmin, RoleViewer, true},
		{"member cannot promote to admin", RoleMember, RoleAdmin, false},
		{"member cannot promote to member", RoleMember, RoleMember, false},
		{"member cannot promote to viewer", RoleMember, RoleViewer, false},
		{"viewer cannot promote anyone", RoleViewer, RoleAdmin, false},
		{"viewer cannot promote anyone", RoleViewer, RoleMember, false},
		{"viewer cannot promote anyone", RoleViewer, RoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanPromoteTo(tt.currentRole, tt.targetRole)
			if result != tt.expected {
				t.Errorf("CanPromoteTo(%s, %s) = %v, expected %v",
					tt.currentRole, tt.targetRole, result, tt.expected)
			}
		})
	}
}
