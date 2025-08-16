package rbac

// Role represents user roles within a group
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// Permission functions for different operations

// CanUpload checks if role can upload files
func CanUpload(role Role) bool {
	return role == RoleAdmin || role == RoleMember
}

// CanDownload checks if role can download files
func CanDownload(role Role) bool {
	return role == RoleAdmin || role == RoleMember || role == RoleViewer
}

// CanDeleteOwn checks if role can delete their own files
func CanDeleteOwn(role Role) bool {
	return role == RoleAdmin || role == RoleMember
}

// CanDeleteAny checks if role can delete any files
func CanDeleteAny(role Role) bool {
	return role == RoleAdmin
}

// CanManageMembers checks if role can manage group members
func CanManageMembers(role Role) bool {
	return role == RoleAdmin
}

// CanStartStopSessions checks if role can start/stop sessions
func CanStartStopSessions(role Role) bool {
	return role == RoleAdmin
}

// CanManageMetadata checks if role can manage metadata
func CanManageMetadata(role Role) bool {
	return role == RoleAdmin || role == RoleMember
}

// CanViewLogs checks if role can view logs
func CanViewLogs(role Role) bool {
	return role == RoleAdmin || role == RoleMember || role == RoleViewer
}

// CanViewMetrics checks if role can view metrics
func CanViewMetrics(role Role) bool {
	return role == RoleAdmin || role == RoleMember || role == RoleViewer
}

// CanNotifyMembers checks if role can send notifications
func CanNotifyMembers(role Role) bool {
	return role == RoleAdmin
}

// CanManageDevices checks if role can approve/manage devices
func CanManageDevices(role Role) bool {
	return role == RoleAdmin
}

// CanInviteMembers checks if role can invite new members
func CanInviteMembers(role Role) bool {
	return role == RoleAdmin
}

// CanChangeRoles checks if role can change other members' roles
func CanChangeRoles(role Role) bool {
	return role == RoleAdmin
}

// IsValidRole checks if the role string is valid
func IsValidRole(role string) bool {
	switch Role(role) {
	case RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

// GetRoleHierarchy returns the hierarchy level of a role (higher = more permissions)
func GetRoleHierarchy(role Role) int {
	switch role {
	case RoleAdmin:
		return 3
	case RoleMember:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

// CanPromoteTo checks if a role can promote another user to a target role
func CanPromoteTo(currentRole, targetRole Role) bool {
	return currentRole == RoleAdmin && GetRoleHierarchy(targetRole) <= GetRoleHierarchy(currentRole)
}
