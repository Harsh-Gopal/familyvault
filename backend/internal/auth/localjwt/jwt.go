package localjwt

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"familyvault/internal/core/rbac"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents JWT claims for the local auth system
type Claims struct {
	GroupID  string    `json:"group_id"`
	UserID   string    `json:"user_id"`
	DeviceID string    `json:"device_id"`
	Role     rbac.Role `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager handles local JWT operations
type JWTManager struct {
	secretKey []byte
}

// NewJWTManager creates a new JWT manager with persistent secret
func NewJWTManager(dataPath string) (*JWTManager, error) {
	secretPath := filepath.Join(dataPath, "jwt_secret")

	// Try to load existing secret
	secretKey, err := os.ReadFile(secretPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Generate new secret
			secretKey = make([]byte, 32)
			if _, err := rand.Read(secretKey); err != nil {
				return nil, fmt.Errorf("failed to generate secret: %w", err)
			}

			// Persist secret
			if err := os.WriteFile(secretPath, secretKey, 0600); err != nil {
				return nil, fmt.Errorf("failed to persist secret: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to read secret: %w", err)
		}
	}

	return &JWTManager{
		secretKey: secretKey,
	}, nil
}

// IssueToken issues a new JWT token
func (jm *JWTManager) IssueToken(groupID, userID, deviceID string, role rbac.Role, ttl time.Duration) (string, error) {
	claims := Claims{
		GroupID:  groupID,
		UserID:   userID,
		DeviceID: deviceID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "familyvault",
			Subject:   userID,
			ID:        generateJTI(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jm.secretKey)
}

// ParseToken parses and validates a JWT token
func (jm *JWTManager) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jm.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// RefreshToken creates a new token with extended expiry
func (jm *JWTManager) RefreshToken(oldToken string, ttl time.Duration) (string, error) {
	claims, err := jm.ParseToken(oldToken)
	if err != nil {
		return "", err
	}

	// Issue new token with same claims but extended expiry
	return jm.IssueToken(claims.GroupID, claims.UserID, claims.DeviceID, claims.Role, ttl)
}

// generateJTI generates a unique JWT ID
func generateJTI() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
