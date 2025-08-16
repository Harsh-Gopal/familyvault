package localjwt

import (
	"os"
	"testing"
	"time"

	"familyvault/internal/core/rbac"
)

func TestJWTManager_IssueAndParseToken(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewJWTManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	// Issue a token
	groupID := "test-group"
	userID := "test-user"
	deviceID := "test-device"
	role := rbac.RoleAdmin
	ttl := 1 * time.Hour

	token, err := manager.IssueToken(groupID, userID, deviceID, role, ttl)
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	if token == "" {
		t.Error("Token should not be empty")
	}

	// Parse the token
	claims, err := manager.ParseToken(token)
	if err != nil {
		t.Fatalf("Failed to parse token: %v", err)
	}

	if claims.GroupID != groupID {
		t.Errorf("Expected group ID %s, got %s", groupID, claims.GroupID)
	}

	if claims.UserID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, claims.UserID)
	}

	if claims.DeviceID != deviceID {
		t.Errorf("Expected device ID %s, got %s", deviceID, claims.DeviceID)
	}

	if claims.Role != role {
		t.Errorf("Expected role %s, got %s", role, claims.Role)
	}

	if claims.Subject != userID {
		t.Errorf("Expected subject %s, got %s", userID, claims.Subject)
	}

	if claims.Issuer != "familyvault" {
		t.Errorf("Expected issuer 'familyvault', got %s", claims.Issuer)
	}
}

func TestJWTManager_InvalidToken(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewJWTManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	// Try to parse invalid token
	_, err = manager.ParseToken("invalid-token")
	if err == nil {
		t.Error("Should fail to parse invalid token")
	}

	// Try to parse empty token
	_, err = manager.ParseToken("")
	if err == nil {
		t.Error("Should fail to parse empty token")
	}
}

func TestJWTManager_ExpiredToken(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewJWTManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	// Issue a token with very short TTL
	token, err := manager.IssueToken("group", "user", "device", rbac.RoleMember, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Try to parse expired token
	_, err = manager.ParseToken(token)
	if err == nil {
		t.Error("Should fail to parse expired token")
	}
}

func TestJWTManager_RefreshToken(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewJWTManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	// Issue original token
	originalToken, err := manager.IssueToken("group", "user", "device", rbac.RoleMember, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to issue original token: %v", err)
	}

	// Refresh token
	refreshedToken, err := manager.RefreshToken(originalToken, 2*time.Hour)
	if err != nil {
		t.Fatalf("Failed to refresh token: %v", err)
	}

	if refreshedToken == originalToken {
		t.Error("Refreshed token should be different from original")
	}

	// Parse refreshed token
	claims, err := manager.ParseToken(refreshedToken)
	if err != nil {
		t.Fatalf("Failed to parse refreshed token: %v", err)
	}

	if claims.UserID != "user" {
		t.Errorf("Expected user ID 'user', got %s", claims.UserID)
	}

	// Original token should still be valid (until it expires)
	originalClaims, err := manager.ParseToken(originalToken)
	if err != nil {
		t.Fatalf("Original token should still be valid: %v", err)
	}

	if originalClaims.UserID != claims.UserID {
		t.Error("Original and refreshed tokens should have same user ID")
	}
}

func TestJWTManager_PersistentSecret(t *testing.T) {
	tempDir := t.TempDir()

	// Create first manager
	manager1, err := NewJWTManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create first JWT manager: %v", err)
	}

	// Issue token with first manager
	token, err := manager1.IssueToken("group", "user", "device", rbac.RoleAdmin, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	// Create second manager (should load same secret)
	manager2, err := NewJWTManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create second JWT manager: %v", err)
	}

	// Second manager should be able to parse token from first manager
	claims, err := manager2.ParseToken(token)
	if err != nil {
		t.Fatalf("Second manager should parse token from first: %v", err)
	}

	if claims.UserID != "user" {
		t.Errorf("Expected user ID 'user', got %s", claims.UserID)
	}
}

func TestJWTManager_DifferentSecrets(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	// Create managers with different data paths (different secrets)
	manager1, err := NewJWTManager(tempDir1)
	if err != nil {
		t.Fatalf("Failed to create first JWT manager: %v", err)
	}

	manager2, err := NewJWTManager(tempDir2)
	if err != nil {
		t.Fatalf("Failed to create second JWT manager: %v", err)
	}

	// Issue token with first manager
	token, err := manager1.IssueToken("group", "user", "device", rbac.RoleAdmin, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	// Second manager should NOT be able to parse token from first manager
	_, err = manager2.ParseToken(token)
	if err == nil {
		t.Error("Second manager should not parse token from first (different secrets)")
	}
}

func TestJWTManager_AllRoles(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewJWTManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	roles := []rbac.Role{rbac.RoleAdmin, rbac.RoleMember, rbac.RoleViewer}

	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			token, err := manager.IssueToken("group", "user", "device", role, 1*time.Hour)
			if err != nil {
				t.Fatalf("Failed to issue token for role %s: %v", role, err)
			}

			claims, err := manager.ParseToken(token)
			if err != nil {
				t.Fatalf("Failed to parse token for role %s: %v", role, err)
			}

			if claims.Role != role {
				t.Errorf("Expected role %s, got %s", role, claims.Role)
			}
		})
	}
}

func TestJWTManager_SecretFilePermissions(t *testing.T) {
	tempDir := t.TempDir()

	_, err := NewJWTManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	// Check that secret file has correct permissions
	secretPath := tempDir + "/jwt_secret"
	info, err := os.Stat(secretPath)
	if err != nil {
		t.Fatalf("Secret file should exist: %v", err)
	}

	mode := info.Mode()
	if mode.Perm() != 0600 {
		t.Errorf("Expected secret file permissions 0600, got %o", mode.Perm())
	}
}
