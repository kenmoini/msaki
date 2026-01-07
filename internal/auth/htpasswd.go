package auth

import (
	"fmt"
	"sync"

	"github.com/tg123/go-htpasswd"
)

// HTPasswdProvider implements authentication using htpasswd files
type HTPasswdProvider struct {
	name     string
	path     string
	role     string
	htpasswd *htpasswd.File
	mu       sync.RWMutex
}

// NewHTPasswdProvider creates a new HTPasswd authentication provider
func NewHTPasswdProvider(name, path, role string) (*HTPasswdProvider, error) {
	hp, err := htpasswd.New(path, htpasswd.DefaultSystems, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load htpasswd file %s: %w", path, err)
	}

	return &HTPasswdProvider{
		name:     name,
		path:     path,
		role:     role,
		htpasswd: hp,
	}, nil
}

// Authenticate validates username and password
func (p *HTPasswdProvider) Authenticate(username, password string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.htpasswd.Match(username, password)
}

// Name returns the provider name
func (p *HTPasswdProvider) Name() string {
	return p.name
}

// Role returns the role assigned to users from this provider
func (p *HTPasswdProvider) Role() string {
	return p.role
}

// Reload reloads the htpasswd file from disk
func (p *HTPasswdProvider) Reload() error {
	hp, err := htpasswd.New(p.path, htpasswd.DefaultSystems, nil)
	if err != nil {
		return fmt.Errorf("failed to reload htpasswd file %s: %w", p.path, err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.htpasswd = hp
	return nil
}
