package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UserRole represents user roles in the system
type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleUser   UserRole = "user"
	RoleViewer UserRole = "viewer"
)

// Claims represents JWT claims
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Role     UserRole `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT operations
type JWTManager struct {
	secretKey     []byte
	tokenDuration time.Duration
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	if secretKey == "" {
		// Generate a random secret key if none provided
		key := make([]byte, 32)
		rand.Read(key)
		secretKey = base64.StdEncoding.EncodeToString(key)
	}

	return &JWTManager{
		secretKey:     []byte(secretKey),
		tokenDuration: tokenDuration,
	}
}

// GenerateToken generates a JWT token for a user
func (manager *JWTManager) GenerateToken(userID, username string, role UserRole) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "familyvault",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(manager.secretKey)
}

// ValidateToken validates a JWT token and returns claims
func (manager *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return manager.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// HasPermission checks if a user role has permission for an operation
func HasPermission(userRole UserRole, operation string) bool {
	switch operation {
	case "read":
		return userRole == RoleAdmin || userRole == RoleUser || userRole == RoleViewer
	case "write", "upload":
		return userRole == RoleAdmin || userRole == RoleUser
	case "delete", "admin":
		return userRole == RoleAdmin
	default:
		return false
	}
}

// GetRoleFromString converts string to UserRole
func GetRoleFromString(role string) UserRole {
	switch role {
	case "admin":
		return RoleAdmin
	case "user":
		return RoleUser
	case "viewer":
		return RoleViewer
	default:
		return RoleViewer // Default to most restrictive
	}
}
