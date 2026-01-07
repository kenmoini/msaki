package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

// Claims represents the JWT claims for MSAKI
type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	secretKey     []byte
	tokenDuration time.Duration
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	// If no secret key provided, generate a random one
	key := []byte(secretKey)
	if len(key) == 0 {
		key = generateRandomKey(32)
	}

	return &JWTManager{
		secretKey:     key,
		tokenDuration: tokenDuration,
	}
}

// GenerateToken creates a new JWT token for the given user
func (m *JWTManager) GenerateToken(username, role string) (string, error) {
	now := time.Now()
	claims := &Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "msaki",
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// ValidateToken validates a JWT token and returns the claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			// Validate signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrInvalidToken
			}
			return m.secretKey, nil
		},
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// generateRandomKey generates a random key of the specified length
func generateRandomKey(length int) []byte {
	key := make([]byte, length)
	if _, err := rand.Read(key); err != nil {
		// Fallback to a default key (not recommended for production)
		return []byte("msaki-default-secret-key-change-me")
	}
	return key
}

// GenerateSecretKey generates a random secret key string
func GenerateSecretKey() string {
	key := generateRandomKey(32)
	return base64.StdEncoding.EncodeToString(key)
}
