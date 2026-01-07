package auth

import (
	"errors"
	"time"

	"github.com/kenmoini/msaki/internal/config"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrProviderNotFound   = errors.New("authentication provider not found")
)

// User represents an authenticated user
type User struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	Provider string `json:"provider"`
}

// AuthManager manages authentication providers and JWT tokens
type AuthManager struct {
	providers  map[string]*HTPasswdProvider
	jwtManager *JWTManager
}

// NewAuthManager creates a new authentication manager from configuration
func NewAuthManager(configs []config.AuthConfig) (*AuthManager, error) {
	am := &AuthManager{
		providers:  make(map[string]*HTPasswdProvider),
		jwtManager: NewJWTManager("", 24*time.Hour), // Default 24h token duration
	}

	for _, cfg := range configs {
		if cfg.Provider != "htpasswd" {
			continue // Skip unsupported providers
		}

		provider, err := NewHTPasswdProvider(cfg.Name, cfg.Path, cfg.Role)
		if err != nil {
			return nil, err
		}
		am.providers[cfg.Name] = provider
	}

	return am, nil
}

// Authenticate validates credentials against all providers
// Returns the user info and JWT token if successful
func (am *AuthManager) Authenticate(username, password string) (*User, string, error) {
	for name, provider := range am.providers {
		if provider.Authenticate(username, password) {
			user := &User{
				Username: username,
				Role:     provider.Role(),
				Provider: name,
			}

			token, err := am.jwtManager.GenerateToken(username, user.Role)
			if err != nil {
				return nil, "", err
			}

			return user, token, nil
		}
	}

	return nil, "", ErrInvalidCredentials
}

// ValidateToken validates a JWT token and returns the user info
func (am *AuthManager) ValidateToken(token string) (*User, error) {
	claims, err := am.jwtManager.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	return &User{
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}

// HasProvider checks if any authentication provider is configured
func (am *AuthManager) HasProvider() bool {
	return len(am.providers) > 0
}

// ReloadProviders reloads all htpasswd files
func (am *AuthManager) ReloadProviders() error {
	for _, provider := range am.providers {
		if err := provider.Reload(); err != nil {
			return err
		}
	}
	return nil
}
