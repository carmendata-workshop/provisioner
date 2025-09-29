package environment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HealthCheck represents the health check configuration for an environment
type HealthCheck struct {
	Type    string `json:"type"`              // "http", "tcp", or "command"
	Path    string `json:"path,omitempty"`    // HTTP path (for http type)
	Port    int    `json:"port,omitempty"`    // Port number (for http/tcp types)
	Command string `json:"command,omitempty"` // Command to execute (for command type)
	Timeout string `json:"timeout"`           // Timeout duration (e.g., "30s", "1m")
}

// Config represents an environment configuration
type Config struct {
	Domain            string      `json:"domain"`
	ReservedIPs       []string    `json:"reserved_ips"`
	AssignedWorkspace string      `json:"assigned_workspace"`
	HealthCheck       HealthCheck `json:"healthcheck"`
}

// Environment represents a loaded environment with its configuration
type Environment struct {
	Name   string // Environment name (derived from filename)
	Config Config
	Path   string // Path to the config file
}

// LoadEnvironment loads a specific environment configuration
func LoadEnvironment(environmentName string) (*Environment, error) {
	configDir := getConfigDir()
	configPath := filepath.Join(configDir, fmt.Sprintf("%s.json", environmentName))

	config, err := loadConfigFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment '%s': %w", environmentName, err)
	}

	return &Environment{
		Name:   environmentName,
		Config: config,
		Path:   configPath,
	}, nil
}

// LoadAllEnvironments loads all environment configurations from the config directory
func LoadAllEnvironments() ([]Environment, error) {
	configDir := getConfigDir()

	// List all .json files in the config directory
	files, err := filepath.Glob(filepath.Join(configDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list environment files: %w", err)
	}

	var environments []Environment
	for _, file := range files {
		// Skip non-environment files
		filename := filepath.Base(file)
		if strings.HasPrefix(filename, ".") ||
		   filename == "config.json" ||
		   strings.Contains(filename, "scheduler") ||
		   strings.Contains(filename, "jobs") {
			continue
		}

		environmentName := strings.TrimSuffix(filename, ".json")

		config, err := loadConfigFile(file)
		if err != nil {
			fmt.Printf("Warning: failed to load environment '%s': %v\n", environmentName, err)
			continue
		}

		environments = append(environments, Environment{
			Name:   environmentName,
			Config: config,
			Path:   file,
		})
	}

	return environments, nil
}

// GetAssignedWorkspaces returns a map of workspace names to environment names
// for all workspaces that are currently assigned to any environment
func GetAssignedWorkspaces() (map[string]string, error) {
	environments, err := LoadAllEnvironments()
	if err != nil {
		return nil, err
	}

	assigned := make(map[string]string)
	for _, env := range environments {
		if env.Config.AssignedWorkspace != "" {
			assigned[env.Config.AssignedWorkspace] = env.Name
		}
	}

	return assigned, nil
}

// EnvironmentExists checks if an environment configuration file exists
func EnvironmentExists(environmentName string) bool {
	configDir := getConfigDir()
	configPath := filepath.Join(configDir, fmt.Sprintf("%s.json", environmentName))
	_, err := os.Stat(configPath)
	return err == nil
}

// SaveEnvironment saves an environment configuration to disk
func (e *Environment) SaveEnvironment() error {
	data, err := json.MarshalIndent(e.Config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal environment config: %w", err)
	}

	if err := os.WriteFile(e.Path, data, 0644); err != nil {
		return fmt.Errorf("failed to write environment config: %w", err)
	}

	return nil
}

// Validate validates the environment configuration
func (c *Config) Validate() error {
	if c.Domain == "" {
		return fmt.Errorf("domain is required")
	}

	if len(c.ReservedIPs) == 0 {
		return fmt.Errorf("at least one reserved IP is required")
	}

	// Validate each Reserved IP format (basic validation)
	for i, ip := range c.ReservedIPs {
		if ip == "" {
			return fmt.Errorf("reserved IP at index %d is empty", i)
		}
		// TODO: Add more sophisticated IP validation if needed
	}

	if c.AssignedWorkspace == "" {
		return fmt.Errorf("assigned_workspace is required")
	}

	// Validate health check configuration
	if err := c.HealthCheck.Validate(); err != nil {
		return fmt.Errorf("invalid health check configuration: %w", err)
	}

	return nil
}

// Validate validates the health check configuration
func (h *HealthCheck) Validate() error {
	if h.Type == "" {
		return fmt.Errorf("health check type is required")
	}

	switch h.Type {
	case "http":
		if h.Path == "" {
			h.Path = "/" // Default to root path
		}
		if h.Port <= 0 {
			h.Port = 80 // Default to port 80
		}
	case "tcp":
		if h.Port <= 0 {
			return fmt.Errorf("port is required for TCP health checks")
		}
	case "command":
		if h.Command == "" {
			return fmt.Errorf("command is required for command health checks")
		}
	default:
		return fmt.Errorf("invalid health check type '%s', must be 'http', 'tcp', or 'command'", h.Type)
	}

	if h.Timeout == "" {
		h.Timeout = "30s" // Default timeout
	}

	// Validate timeout format
	if _, err := time.ParseDuration(h.Timeout); err != nil {
		return fmt.Errorf("invalid timeout format '%s': %w", h.Timeout, err)
	}

	return nil
}

// GetTimeoutDuration returns the health check timeout as a time.Duration
func (h *HealthCheck) GetTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(h.Timeout)
}

// loadConfigFile loads a configuration file and returns the parsed config
func loadConfigFile(configPath string) (Config, error) {
	var config Config

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config, fmt.Errorf("configuration file does not exist: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return config, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// getConfigDir determines the configuration directory using auto-discovery
func getConfigDir() string {
	// First check environment variable (explicit override)
	if configDir := os.Getenv("PROVISIONER_CONFIG_DIR"); configDir != "" {
		return configDir
	}

	// Auto-detect system installation
	if _, err := os.Stat("/etc/provisioner"); err == nil {
		return "/etc/provisioner"
	}

	// Fall back to development default
	return "."
}