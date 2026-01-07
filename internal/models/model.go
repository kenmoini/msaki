package models

import (
	"sync"
	"time"

	"github.com/kenmoini/msaki/internal/config"
)

// Status represents the current state of a model
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
	StatusError    Status = "error"
)

// Model represents a runtime model instance
type Model struct {
	config       *config.ModelConfig
	status       Status
	statusError  string
	port         int
	lastActivity time.Time
	healthy      bool
	healthMsg    string
	mu           sync.RWMutex
}

// NewModel creates a new Model from configuration
func NewModel(cfg *config.ModelConfig) *Model {
	status := StatusStopped
	// External endpoints are always considered "running"
	if cfg.Endpoint != "" && cfg.StartScript == "" {
		status = StatusRunning
	}

	return &Model{
		config:       cfg,
		status:       status,
		lastActivity: time.Now(),
		healthy:      cfg.Endpoint != "", // External endpoints start healthy
	}
}

// Name returns the model name
func (m *Model) Name() string {
	return m.config.Name
}

// Description returns the model description
func (m *Model) Description() string {
	return m.config.Description
}

// Aliases returns the model aliases
func (m *Model) Aliases() []string {
	return m.config.Aliases
}

// Tags returns the model tags
func (m *Model) Tags() []string {
	return m.config.Tags
}

// Status returns the current model status
func (m *Model) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// SetStatus sets the model status
func (m *Model) SetStatus(status Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = status
	if status != StatusError {
		m.statusError = ""
	}
}

// SetError sets the model to error status with a message
func (m *Model) SetError(err string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = StatusError
	m.statusError = err
}

// StatusError returns the error message if in error state
func (m *Model) StatusError() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.statusError
}

// Port returns the assigned port
func (m *Model) Port() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.port
}

// SetPort sets the assigned port
func (m *Model) SetPort(port int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.port = port
}

// IsHealthy returns whether the model is healthy
func (m *Model) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy
}

// SetHealthy sets the health status
func (m *Model) SetHealthy(healthy bool, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = healthy
	m.healthMsg = msg
}

// HealthMessage returns the health status message
func (m *Model) HealthMessage() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthMsg
}

// LastActivity returns the last activity time
func (m *Model) LastActivity() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastActivity
}

// UpdateActivity updates the last activity timestamp
func (m *Model) UpdateActivity() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastActivity = time.Now()
}

// Config returns the model configuration
func (m *Model) Config() *config.ModelConfig {
	return m.config
}

// HasStartScript returns whether the model has a start script
func (m *Model) HasStartScript() bool {
	return m.config.StartScript != ""
}

// HasStopScript returns whether the model has a stop script
func (m *Model) HasStopScript() bool {
	return m.config.StopScript != ""
}

// IsExternal returns whether this is an external endpoint
func (m *Model) IsExternal() bool {
	return m.config.Endpoint != "" && m.config.StartScript == ""
}

// TTL returns the TTL duration for auto-stop
func (m *Model) TTL() time.Duration {
	return m.config.TTL.Duration
}

// Endpoint returns the backend endpoint URL
func (m *Model) Endpoint() string {
	return m.config.Endpoint
}

// BackendOverride returns the backend override URL
func (m *Model) BackendOverride() string {
	return m.config.BackendOverride
}

// ToDTO converts the model to a data transfer object
func (m *Model) ToDTO() *ModelDTO {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &ModelDTO{
		Name:           m.config.Name,
		Description:    m.config.Description,
		Aliases:        m.config.Aliases,
		Tags:           m.config.Tags,
		Status:         string(m.status),
		StatusError:    m.statusError,
		Endpoint:       m.config.Endpoint,
		HasStartScript: m.config.StartScript != "",
		HasStopScript:  m.config.StopScript != "",
		Healthy:        m.healthy,
		HealthMessage:  m.healthMsg,
	}
}

// ModelDTO is the data transfer object for model information
type ModelDTO struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Aliases        []string `json:"aliases"`
	Tags           []string `json:"tags"`
	Status         string   `json:"status"`
	StatusError    string   `json:"statusError,omitempty"`
	Endpoint       string   `json:"endpoint,omitempty"`
	HasStartScript bool     `json:"hasStartScript"`
	HasStopScript  bool     `json:"hasStopScript"`
	Healthy        bool     `json:"healthy"`
	HealthMessage  string   `json:"healthMessage,omitempty"`
}
