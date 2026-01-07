package models

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
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

// runHealthChecks runs health checks on all running or starting models
func (m *Manager) runHealthChecks() {
	m.mu.RLock()
	modelsToCheck := make([]*Model, 0)
	for _, model := range m.models {
		status := model.Status()
		// Run health checks on running models and starting models (after startDelay)
		if model.Config().HealthCheck.Enabled && (status == StatusRunning || status == StatusStarting) {
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
	modelCfg := model.Config()
	status := model.Status()

	// For starting models, check if startDelay has passed
	if status == StatusStarting {
		elapsed := time.Since(model.StartedAt())
		if elapsed < cfg.StartDelay.Duration {
			// Still within startDelay, skip health check
			return
		}
	}

	endpoint := m.getModelEndpoint(model)
	if endpoint == "" {
		return
	}

	healthURL := endpoint + cfg.Endpoint

	// Create HTTP client with appropriate TLS config
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: modelCfg.SkipTLSVerify,
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	// Perform HTTP GET health check
	resp, err := client.Get(healthURL)
	if err != nil {
		model.SetHealthy(false, fmt.Sprintf("Health check failed: %v", err))
		if status == StatusStarting {
			log.Printf("Health check for %s failed (still starting): %v", model.Name(), err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		model.SetHealthy(true, "OK")
		// Transition from starting to running when health check passes
		if status == StatusStarting {
			model.SetStatus(StatusRunning)
			model.UpdateActivity()
			model.AppendLog("system", "Health check passed, model is now running")
			log.Printf("Model %s health check passed, transitioning to running", model.Name())
		}
	} else {
		model.SetHealthy(false, fmt.Sprintf("Health check returned %d", resp.StatusCode))
		if status == StatusStarting {
			log.Printf("Health check for %s returned %d (still starting)", model.Name(), resp.StatusCode)
		}
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

	// Clear previous logs and set status
	model.ClearLogs()
	model.SetStatus(StatusStarting)
	model.SetStartedAt(time.Now())

	// Allocate port if needed
	port := 0
	if strings.Contains(model.Config().StartScript, "${PORT}") {
		var err error
		port, err = m.portAllocator.Allocate(model.Name())
		if err != nil {
			model.SetError(fmt.Sprintf("Failed to allocate port: %v", err))
			model.AppendLog("system", fmt.Sprintf("Failed to allocate port: %v", err))
			return err
		}
		model.SetPort(port)
	}

	// Execute start script with streaming output
	go func() {
		script := model.Config().StartScript
		if port > 0 {
			script = strings.ReplaceAll(script, "${PORT}", fmt.Sprintf("%d", port))
		}

		log.Printf("Starting model %s: %s", model.Name(), script)
		model.AppendLog("system", fmt.Sprintf("Executing start script: %s", script))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "sh", "-c", script)

		// Set up pipes for stdout and stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			model.SetError(fmt.Sprintf("Failed to create stdout pipe: %v", err))
			model.AppendLog("system", fmt.Sprintf("Failed to create stdout pipe: %v", err))
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			model.SetError(fmt.Sprintf("Failed to create stderr pipe: %v", err))
			model.AppendLog("system", fmt.Sprintf("Failed to create stderr pipe: %v", err))
			return
		}

		// Start the command
		if err := cmd.Start(); err != nil {
			model.SetError(fmt.Sprintf("Failed to start command: %v", err))
			model.AppendLog("system", fmt.Sprintf("Failed to start command: %v", err))
			m.portAllocator.Release(model.Name())
			return
		}

		// Stream stdout and stderr in goroutines
		var wg sync.WaitGroup
		wg.Add(2)

		go m.streamOutput(&wg, model, stdout, "stdout")
		go m.streamOutput(&wg, model, stderr, "stderr")

		// Wait for output streaming to complete
		wg.Wait()

		// Wait for command to finish
		err = cmd.Wait()
		if err != nil {
			log.Printf("Model %s start failed: %v", model.Name(), err)
			model.SetError(fmt.Sprintf("Start failed: %v", err))
			model.AppendLog("system", fmt.Sprintf("Script failed with error: %v", err))
			m.portAllocator.Release(model.Name())
			return
		}

		// If health check is enabled, stay in StatusStarting and let health checks transition to running
		// Otherwise, immediately mark as running
		if model.Config().HealthCheck.Enabled {
			model.AppendLog("system", "Start script completed, waiting for health check to pass...")
			log.Printf("Model %s start script completed, waiting for health check", model.Name())
		} else {
			model.SetStatus(StatusRunning)
			model.SetHealthy(true, "Started")
			model.UpdateActivity()
			model.AppendLog("system", "Model started successfully")
			log.Printf("Model %s started successfully", model.Name())
		}
	}()

	return nil
}

// streamOutput reads from a pipe and appends to model logs
func (m *Manager) streamOutput(wg *sync.WaitGroup, model *Model, pipe io.ReadCloser, stream string) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		model.AppendLog(stream, line)
		log.Printf("[%s:%s] %s", model.Name(), stream, line)
	}
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
		model.AppendLog("system", "Model stopped (no stop script)")
		m.portAllocator.Release(model.Name())
		return nil
	}

	model.SetStatus(StatusStopping)

	// Execute stop script with streaming output
	go func() {
		script := model.Config().StopScript
		port := model.Port()
		if port > 0 {
			script = strings.ReplaceAll(script, "${PORT}", fmt.Sprintf("%d", port))
		}

		log.Printf("Stopping model %s: %s", model.Name(), script)
		model.AppendLog("system", fmt.Sprintf("Executing stop script: %s", script))

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "sh", "-c", script)

		// Set up pipes for stdout and stderr
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		// Start the command
		if err := cmd.Start(); err != nil {
			model.AppendLog("system", fmt.Sprintf("Failed to start stop command: %v", err))
			model.SetStatus(StatusStopped)
			m.portAllocator.Release(model.Name())
			return
		}

		// Stream output
		var wg sync.WaitGroup
		wg.Add(2)
		go m.streamOutput(&wg, model, stdout, "stdout")
		go m.streamOutput(&wg, model, stderr, "stderr")
		wg.Wait()

		err := cmd.Wait()
		if err != nil {
			log.Printf("Model %s stop warning: %v", model.Name(), err)
			model.AppendLog("system", fmt.Sprintf("Stop script warning: %v", err))
			// Still mark as stopped even if stop script fails
		}

		model.SetStatus(StatusStopped)
		model.SetHealthy(false, "Stopped")
		model.AppendLog("system", "Model stopped")
		m.portAllocator.Release(model.Name())
		log.Printf("Model %s stopped", model.Name())
	}()

	return nil
}

// Restart restarts a model (can be used from error state)
func (m *Manager) Restart(name string) error {
	model := m.GetModel(name)
	if model == nil {
		return fmt.Errorf("model not found: %s", name)
	}

	if model.IsExternal() {
		return fmt.Errorf("external models cannot be restarted")
	}

	// If in error state, reset to stopped first
	if model.Status() == StatusError {
		model.SetStatus(StatusStopped)
		m.portAllocator.Release(model.Name())
	}

	// If running or stopping, stop first then start
	if model.Status() == StatusRunning || model.Status() == StatusStopping {
		// For restart, we'll stop synchronously then start
		model.ClearLogs()
		model.AppendLog("system", "Restarting model...")

		if model.HasStopScript() {
			script := model.Config().StopScript
			port := model.Port()
			if port > 0 {
				script = strings.ReplaceAll(script, "${PORT}", fmt.Sprintf("%d", port))
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			cmd := exec.CommandContext(ctx, "sh", "-c", script)
			output, err := cmd.CombinedOutput()
			cancel()

			if err != nil {
				model.AppendLog("system", fmt.Sprintf("Stop during restart warning: %v", err))
			}
			if len(output) > 0 {
				model.AppendLog("stdout", string(output))
			}
		}

		model.SetStatus(StatusStopped)
		m.portAllocator.Release(model.Name())
	}

	// Now start
	return m.Start(name)
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
