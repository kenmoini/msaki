package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure for MSAKI
type Config struct {
	Global GlobalConfig  `yaml:"global"`
	Models []ModelConfig `yaml:"models"`
	Tests  []TestConfig  `yaml:"tests,omitempty"`
}

// GlobalConfig contains server-wide settings
type GlobalConfig struct {
	Server        ServerConfig        `yaml:"server"`
	Observability ObservabilityConfig `yaml:"observability"`
}

// ServerConfig defines the MSAKI server settings
type ServerConfig struct {
	Listen         ListenConfig      `yaml:"listen"`
	PortMapping    PortMappingConfig `yaml:"portMapping"`
	Authentication []AuthConfig      `yaml:"authentication"`
}

// ListenConfig defines the address and port the server binds to
type ListenConfig struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

// PortMappingConfig defines how dynamically allocated ports work
type PortMappingConfig struct {
	ContainerLabelOverrides bool `yaml:"containerLabelOverrides"`
	HostPortStart           int  `yaml:"hostPortStart"`
}

// AuthConfig defines an authentication provider
type AuthConfig struct {
	Name     string `yaml:"name"`
	Provider string `yaml:"provider"`
	Path     string `yaml:"path"`
	Role     string `yaml:"role"`
}

// ObservabilityConfig contains metrics and logging settings
type ObservabilityConfig struct {
	Metrics    MetricsConfig    `yaml:"metrics"`
	AccessLogs LogConfig        `yaml:"accessLogs"`
	ErrorLogs  LogConfig        `yaml:"errorLogs"`
	ChatLogs   ChatLogsConfig   `yaml:"chatLogs"`
}

// MetricsConfig defines Prometheus metrics settings
type MetricsConfig struct {
	Enabled    bool             `yaml:"enabled"`
	Engine     string           `yaml:"engine"`
	Prometheus PrometheusConfig `yaml:"prometheus"`
}

// PrometheusConfig defines Prometheus-specific settings
type PrometheusConfig struct {
	Path string `yaml:"path"`
}

// LogConfig defines settings for access and error logs
type LogConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Level        string `yaml:"level"`
	SharedOutput bool   `yaml:"sharedOutput"`
	File         string `yaml:"file"`
	FileRotation string `yaml:"fileRotation"`
}

// ChatLogsConfig defines chat logging settings
type ChatLogsConfig struct {
	Enabled      bool                   `yaml:"enabled"`
	LogDirectory string                 `yaml:"logDirectory"`
	Collections  []ChatLogCollection    `yaml:"collections"`
}

// ChatLogCollection defines a collection of chat logs
type ChatLogCollection struct {
	Name      string   `yaml:"name"`
	Models    []string `yaml:"models"`
	Requests  bool     `yaml:"requests"`
	Responses bool     `yaml:"responses"`
	Filename  string   `yaml:"filename"`
}

// ProviderType represents the API provider type for a model
type ProviderType string

const (
	// ProviderOpenAI is the OpenAI-compatible API provider (default)
	ProviderOpenAI ProviderType = "openai"
	// ProviderOllama is the Ollama API provider
	ProviderOllama ProviderType = "ollama"
	// ProviderAnthropic is the Anthropic API provider
	ProviderAnthropic ProviderType = "anthropic"
)

// ModelConfig defines a model endpoint configuration
type ModelConfig struct {
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description"`
	Aliases         []string          `yaml:"aliases,omitempty"`
	Tags            []string          `yaml:"tags,omitempty"`
	Provider        ProviderType      `yaml:"provider,omitempty"`
	ModelName       string            `yaml:"modelName,omitempty"` // The actual model name to send to the backend
	StartScript     string            `yaml:"startScript,omitempty"`
	StopScript      string            `yaml:"stopScript,omitempty"`
	BackendOverride string            `yaml:"backendOverride,omitempty"`
	Endpoint        string            `yaml:"endpoint,omitempty"`
	APIKeyEnv       string            `yaml:"api_key_env,omitempty"`
	APIKeyPath      string            `yaml:"api_key_path,omitempty"`
	SkipTLSVerify   bool              `yaml:"skip_tls_verify"`
	TTL             Duration          `yaml:"ttl,omitempty"`
	HealthCheck     HealthCheckConfig `yaml:"healthCheck"`
}

// HealthCheckConfig defines health check settings for a model
type HealthCheckConfig struct {
	Enabled    bool     `yaml:"enabled"`
	Endpoint   string   `yaml:"endpoint"`
	StartDelay Duration `yaml:"startDelay"`
	Interval   Duration `yaml:"interval"`
	Retries    int      `yaml:"retries"`
}

// TestConfig defines a test to run against models
type TestConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Prompt      string `yaml:"prompt"`
	Endpoint    string `yaml:"endpoint"`
	Method      string `yaml:"method"`
}

// Duration is a wrapper around time.Duration that supports YAML unmarshaling
type Duration struct {
	time.Duration
}

// UnmarshalYAML implements yaml.Unmarshaler for Duration
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	if s == "" {
		d.Duration = 0
		return nil
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = duration
	return nil
}

// MarshalYAML implements yaml.Marshaler for Duration
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// Load reads and parses a configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	cfg.SetDefaults()

	return &cfg, nil
}

// SetDefaults applies default values to the configuration
func (c *Config) SetDefaults() {
	if c.Global.Server.Listen.Address == "" {
		c.Global.Server.Listen.Address = "0.0.0.0"
	}
	if c.Global.Server.Listen.Port == 0 {
		c.Global.Server.Listen.Port = 8080
	}
	if c.Global.Server.PortMapping.HostPortStart == 0 {
		c.Global.Server.PortMapping.HostPortStart = 12000
	}
	if c.Global.Observability.Metrics.Prometheus.Path == "" {
		c.Global.Observability.Metrics.Prometheus.Path = "/metrics"
	}
	if c.Global.Observability.AccessLogs.Level == "" {
		c.Global.Observability.AccessLogs.Level = "info"
	}
	if c.Global.Observability.ErrorLogs.Level == "" {
		c.Global.Observability.ErrorLogs.Level = "error"
	}
}

// Validate checks the configuration for errors
func (c *Config) Validate() error {
	// Validate server config
	if c.Global.Server.Listen.Port < 0 || c.Global.Server.Listen.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Global.Server.Listen.Port)
	}

	// Validate authentication providers
	for _, auth := range c.Global.Server.Authentication {
		if auth.Name == "" {
			return fmt.Errorf("authentication provider name is required")
		}
		if auth.Provider == "" {
			return fmt.Errorf("authentication provider type is required for %s", auth.Name)
		}
		if auth.Provider != "htpasswd" {
			return fmt.Errorf("unsupported authentication provider: %s", auth.Provider)
		}
		if auth.Path == "" {
			return fmt.Errorf("path is required for authentication provider %s", auth.Name)
		}
		if auth.Role == "" {
			return fmt.Errorf("role is required for authentication provider %s", auth.Name)
		}
		if auth.Role != "administrator" && auth.Role != "user" {
			return fmt.Errorf("invalid role %q for authentication provider %s (must be 'administrator' or 'user')", auth.Role, auth.Name)
		}
	}

	// Validate models
	modelNames := make(map[string]bool)
	for _, model := range c.Models {
		if model.Name == "" {
			return fmt.Errorf("model name is required")
		}
		if modelNames[model.Name] {
			return fmt.Errorf("duplicate model name: %s", model.Name)
		}
		modelNames[model.Name] = true

		// Model must have either endpoint or startScript
		if model.Endpoint == "" && model.StartScript == "" {
			return fmt.Errorf("model %s must have either endpoint or startScript", model.Name)
		}

		// Validate aliases are unique
		for _, alias := range model.Aliases {
			if modelNames[alias] {
				return fmt.Errorf("model alias %q conflicts with existing model name", alias)
			}
		}
	}

	return nil
}

// GetModelByName returns a model configuration by name or alias
func (c *Config) GetModelByName(name string) *ModelConfig {
	for i := range c.Models {
		if c.Models[i].Name == name {
			return &c.Models[i]
		}
		for _, alias := range c.Models[i].Aliases {
			if alias == name {
				return &c.Models[i]
			}
		}
	}
	return nil
}

// ParseFileSize parses a file size string like "10Mb" to bytes
func ParseFileSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	re := regexp.MustCompile(`(?i)^(\d+)\s*(b|kb|mb|gb|tb)?$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid file size: %s", s)
	}

	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}

	unit := strings.ToLower(matches[2])
	switch unit {
	case "", "b":
		return value, nil
	case "kb":
		return value * 1024, nil
	case "mb":
		return value * 1024 * 1024, nil
	case "gb":
		return value * 1024 * 1024 * 1024, nil
	case "tb":
		return value * 1024 * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}
