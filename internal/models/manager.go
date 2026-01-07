package models

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/kenmoini/msaki/internal/config"
)

// Manager manages the lifecycle of all models
type Manager struct {
	models        map[string]*Model
	portAllocator *PortAllocator
	config        *config.Config
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewManager creates a new model manager
func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		models:        make(map[string]*Model),
		portAllocator: NewPortAllocator(cfg.Global.Server.PortMapping.HostPortStart),
		config:        cfg,
		ctx:           ctx,
		cancel:        cancel,
	}

	// Initialize models from configuration
	for i := range cfg.Models {
		model := NewModel(&cfg.Models[i])
		m.models[model.Name()] = model

		// Also index by aliases
		for _, alias := range model.Aliases() {
			m.models[alias] = model
		}
	}

	// Start background tasks
	m.startBackgroundTasks()

	return m
}

// startBackgroundTasks starts TTL checker and health monitor
func (m *Manager) startBackgroundTasks() {
	// TTL checker
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				m.checkTTLs()
			}
		}
	}()

	// Health checker
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				m.runHealthChecks()
			}
		}
	}()
}

// checkTTLs stops models that have exceeded their TTL
func (m *Manager) checkTTLs() {
	m.mu.RLock()
	modelsToCheck := make([]*Model, 0)
	for _, model := range m.models {
		if model.TTL() > 0 && model.Status() == StatusRunning && !model.IsExternal() {
			modelsToCheck = append(modelsToCheck, model)
		}
	}
	m.mu.RUnlock()

	for _, model := range modelsToCheck {
		if time.Since(model.LastActivity()) > model.TTL() {
			log.Printf("Model %s exceeded TTL, stopping...", model.Name())
			if err := m.Stop(model.Name()); err != nil {
				log.Printf("Failed to stop model %s: %v", model.Name(), err)
			}
		}
	}
}

// runHealthChecks runs health checks on all running models
func (m *Manager) runHealthChecks() {
	m.mu.RLock()
	modelsToCheck := make([]*Model, 0)
	for _, model := range m.models {
		if model.Status() == StatusRunning && model.Config().HealthCheck.Enabled {
			modelsToCheck = append(modelsToCheck, model)
		}
	}
	m.mu.RUnlock()

	for _, model := range modelsToCheck {
		go m.checkHealth(model)
	}
}

// checkHealth performs a health check on a single model
func (m *Manager) checkHealth(model *Model) {
	cfg := model.Config().HealthCheck
	endpoint := m.getModelEndpoint(model)
	if endpoint == "" {
		return
	}

	healthURL := endpoint + cfg.Endpoint

	// Simple HTTP GET health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := exec.CommandContext(ctx, "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", healthURL).Output()
	if err != nil {
		model.SetHealthy(false, fmt.Sprintf("Health check failed: %v", err))
		return
	}

	statusCode := strings.TrimSpace(string(req))
	if statusCode == "200" {
		model.SetHealthy(true, "OK")
	} else {
		model.SetHealthy(false, fmt.Sprintf("Health check returned %s", statusCode))
	}
}

// GetModel returns a model by name or alias
func (m *Manager) GetModel(name string) *Model {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.models[name]
}

// ListModels returns all unique models
func (m *Manager) ListModels() []*Model {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Use a map to deduplicate (since aliases point to same model)
	seen := make(map[string]bool)
	result := make([]*Model, 0)

	for _, model := range m.models {
		if !seen[model.Name()] {
			seen[model.Name()] = true
			result = append(result, model)
		}
	}

	return result
}

// Start starts a model
func (m *Manager) Start(name string) error {
	model := m.GetModel(name)
	if model == nil {
		return fmt.Errorf("model not found: %s", name)
	}

	if model.IsExternal() {
		return fmt.Errorf("external models cannot be started")
	}

	if model.Status() == StatusRunning {
		return nil // Already running
	}

	if model.Status() == StatusStarting {
		return fmt.Errorf("model is already starting")
	}

	if !model.HasStartScript() {
		return fmt.Errorf("model has no start script")
	}

	model.SetStatus(StatusStarting)

	// Allocate port if needed
	port := 0
	if strings.Contains(model.Config().StartScript, "${PORT}") {
		var err error
		port, err = m.portAllocator.Allocate(model.Name())
		if err != nil {
			model.SetError(fmt.Sprintf("Failed to allocate port: %v", err))
			return err
		}
		model.SetPort(port)
	}

	// Execute start script
	go func() {
		script := model.Config().StartScript
		if port > 0 {
			script = strings.ReplaceAll(script, "${PORT}", fmt.Sprintf("%d", port))
		}

		log.Printf("Starting model %s: %s", model.Name(), script)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "sh", "-c", script)
		output, err := cmd.CombinedOutput()

		if err != nil {
			log.Printf("Model %s start failed: %v\nOutput: %s", model.Name(), err, string(output))
			model.SetError(fmt.Sprintf("Start failed: %v", err))
			m.portAllocator.Release(model.Name())
			return
		}

		model.SetStatus(StatusRunning)
		model.SetHealthy(true, "Started")
		model.UpdateActivity()
		log.Printf("Model %s started successfully", model.Name())
	}()

	return nil
}

// Stop stops a model
func (m *Manager) Stop(name string) error {
	model := m.GetModel(name)
	if model == nil {
		return fmt.Errorf("model not found: %s", name)
	}

	if model.IsExternal() {
		return fmt.Errorf("external models cannot be stopped")
	}

	if model.Status() == StatusStopped {
		return nil // Already stopped
	}

	if model.Status() == StatusStopping {
		return fmt.Errorf("model is already stopping")
	}

	if !model.HasStopScript() {
		// No stop script, just mark as stopped
		model.SetStatus(StatusStopped)
		m.portAllocator.Release(model.Name())
		return nil
	}

	model.SetStatus(StatusStopping)

	// Execute stop script
	go func() {
		script := model.Config().StopScript
		port := model.Port()
		if port > 0 {
			script = strings.ReplaceAll(script, "${PORT}", fmt.Sprintf("%d", port))
		}

		log.Printf("Stopping model %s: %s", model.Name(), script)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "sh", "-c", script)
		output, err := cmd.CombinedOutput()

		if err != nil {
			log.Printf("Model %s stop warning: %v\nOutput: %s", model.Name(), err, string(output))
			// Still mark as stopped even if stop script fails
		}

		model.SetStatus(StatusStopped)
		model.SetHealthy(false, "Stopped")
		m.portAllocator.Release(model.Name())
		log.Printf("Model %s stopped", model.Name())
	}()

	return nil
}

// getModelEndpoint returns the endpoint URL for a model
func (m *Manager) getModelEndpoint(model *Model) string {
	if model.Endpoint() != "" {
		return model.Endpoint()
	}

	if model.BackendOverride() != "" {
		endpoint := model.BackendOverride()
		port := model.Port()
		if port > 0 {
			endpoint = strings.ReplaceAll(endpoint, "${PORT}", fmt.Sprintf("%d", port))
		}
		return endpoint
	}

	port := model.Port()
	if port > 0 {
		return fmt.Sprintf("http://localhost:%d", port)
	}

	return ""
}

// GetEndpoint returns the endpoint URL for a model by name
func (m *Manager) GetEndpoint(name string) string {
	model := m.GetModel(name)
	if model == nil {
		return ""
	}
	return m.getModelEndpoint(model)
}

// UpdateActivity updates the last activity time for a model
func (m *Manager) UpdateActivity(name string) {
	model := m.GetModel(name)
	if model != nil {
		model.UpdateActivity()
	}
}

// Shutdown stops all models and background tasks
func (m *Manager) Shutdown() {
	m.cancel()

	// Stop all running models
	for _, model := range m.ListModels() {
		if model.Status() == StatusRunning && !model.IsExternal() {
			m.Stop(model.Name())
		}
	}

	m.wg.Wait()
}

// PortAllocator returns the port allocator
func (m *Manager) PortAllocator() *PortAllocator {
	return m.portAllocator
}
