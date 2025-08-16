package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"familyvault/internal/auth/localjwt"
	"familyvault/internal/core/groups"
	"familyvault/internal/core/rbac"

	"github.com/gorilla/mux"
)

// ContextKey represents context keys for auth data
type ContextKey string

const (
	ClaimsContextKey ContextKey = "claims"
	GroupContextKey  ContextKey = "group"
	UserContextKey   ContextKey = "user"
)

// AuthMiddleware provides authentication and authorization middleware
type AuthMiddleware struct {
	jwtManager *localjwt.JWTManager
	store      *groups.Store
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(jwtManager *localjwt.JWTManager, store *groups.Store) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: jwtManager,
		store:      store,
	}
}

// WithAuth middleware that requires authentication
func (m *AuthMiddleware) WithAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := m.extractToken(r)
		if token == "" {
			http.Error(w, "Unauthorized: missing token", http.StatusUnauthorized)
			return
		}

		claims, err := m.jwtManager.ParseToken(token)
		if err != nil {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		// Verify device is approved
		if !m.store.IsDeviceApproved(claims.DeviceID) {
			http.Error(w, "Unauthorized: device not approved", http.StatusUnauthorized)
			return
		}

		// Verify membership is active
		membership, exists := m.store.GetMembership(claims.GroupID, claims.UserID)
		if !exists || !membership.IsActive() {
			http.Error(w, "Unauthorized: membership not active", http.StatusUnauthorized)
			return
		}

		// Update device last seen
		m.store.UpdateDeviceLastSeen(claims.DeviceID)

		// Add claims to context
		ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)

		// Add user and group to context for convenience
		if user, exists := m.store.GetUser(claims.UserID); exists {
			ctx = context.WithValue(ctx, UserContextKey, user)
		}
		if group, exists := m.store.GetGroup(claims.GroupID); exists {
			ctx = context.WithValue(ctx, GroupContextKey, group)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole middleware that requires a minimum role
func (m *AuthMiddleware) RequireRole(minRole rbac.Role) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return m.WithAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaimsFromContext(r.Context())
			if claims == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if rbac.GetRoleHierarchy(claims.Role) < rbac.GetRoleHierarchy(minRole) {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		}))
	}
}

// RequirePermission middleware that requires a specific permission
func (m *AuthMiddleware) RequirePermission(permissionCheck func(rbac.Role) bool) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return m.WithAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaimsFromContext(r.Context())
			if claims == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !permissionCheck(claims.Role) {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		}))
	}
}

// RequireGroupParam middleware that validates group ID in URL matches claims
func RequireGroupParam(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaimsFromContext(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		vars := mux.Vars(r)
		groupID := vars["group_id"]
		if groupID == "" {
			http.Error(w, "Bad Request: missing group_id", http.StatusBadRequest)
			return
		}

		if groupID != claims.GroupID {
			http.Error(w, "Forbidden: group access denied", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractToken extracts JWT token from request
func (m *AuthMiddleware) extractToken(r *http.Request) string {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// Try X-Auth-Token header
	if token := r.Header.Get("X-Auth-Token"); token != "" {
		return token
	}

	// Try query parameter
	return r.URL.Query().Get("token")
}

// Context helper functions

// GetClaimsFromContext extracts JWT claims from context
func GetClaimsFromContext(ctx context.Context) *localjwt.Claims {
	if claims, ok := ctx.Value(ClaimsContextKey).(*localjwt.Claims); ok {
		return claims
	}
	return nil
}

// GetUserFromContext extracts user from context
func GetUserFromContext(ctx context.Context) *groups.User {
	if user, ok := ctx.Value(UserContextKey).(*groups.User); ok {
		return user
	}
	return nil
}

// GetGroupFromContext extracts group from context
func GetGroupFromContext(ctx context.Context) *groups.Group {
	if group, ok := ctx.Value(GroupContextKey).(*groups.Group); ok {
		return group
	}
	return nil
}

// RequireOwnership checks if user owns the resource or is admin
func RequireOwnership(userID string, claims *localjwt.Claims) error {
	if claims.Role == rbac.RoleAdmin {
		return nil // Admins can access anything
	}

	if claims.UserID != userID {
		return fmt.Errorf("access denied: not owner")
	}

	return nil
}
