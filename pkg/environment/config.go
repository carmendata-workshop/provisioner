package environment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Enabled         bool        `json:"enabled"`
	Template        string      `json:"template,omitempty"`
	DeploySchedule  interface{} `json:"deploy_schedule"`
	DestroySchedule interface{} `json:"destroy_schedule"`
	Description     string      `json:"description"`
}

type Environment struct {
	Name   string // Derived from folder name
	Config Config
	Path   string
}

func LoadEnvironments(environmentsDir string) ([]Environment, error) {
	var environments []Environment

	entries, err := os.ReadDir(environmentsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read environments directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		envPath := filepath.Join(environmentsDir, entry.Name())
		configPath := filepath.Join(envPath, "config.json")

		// Check if config.json exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		config, err := loadConfig(configPath)
		if err != nil {
			fmt.Printf("Warning: failed to load config for %s: %v\n", entry.Name(), err)
			continue
		}

		// Create environment
		env := Environment{
			Name:   entry.Name(), // Use folder name as environment name
			Config: config,
			Path:   envPath,
		}

		// Validate that the environment has either a local main.tf or a valid template
		if !env.HasMainTF() {
			if env.Config.Template == "" {
				fmt.Printf("Warning: environment %s has no main.tf and no template specified\n", entry.Name())
			} else {
				fmt.Printf("Warning: environment %s references template '%s' but template not found\n", entry.Name(), env.Config.Template)
			}
			continue
		}

		// Load all environments (enabled check will be done during scheduling)
		environments = append(environments, env)
	}

	return environments, nil
}

func loadConfig(configPath string) (Config, error) {
	var config Config

	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

func (e *Environment) GetMainTFPath() string {
	// Check for local main.tf first (highest priority)
	localPath := filepath.Join(e.Path, "main.tf")
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	// If no local main.tf and template is specified, use template path
	if e.Config.Template != "" {
		templatesDir := getTemplatesDir()
		templatePath := filepath.Join(templatesDir, e.Config.Template, "main.tf")
		if _, err := os.Stat(templatePath); err == nil {
			return templatePath
		}
	}

	// Return local path even if it doesn't exist (for error handling)
	return localPath
}

func (e *Environment) HasMainTF() bool {
	// Check for local main.tf first
	localPath := filepath.Join(e.Path, "main.tf")
	if _, err := os.Stat(localPath); err == nil {
		return true
	}

	// Check for template main.tf if template is specified
	if e.Config.Template != "" {
		templatesDir := getTemplatesDir()
		templatePath := filepath.Join(templatesDir, e.Config.Template, "main.tf")
		if _, err := os.Stat(templatePath); err == nil {
			return true
		}
	}

	return false
}

// GetTemplateDir returns the directory path for the template if one is specified
func (e *Environment) GetTemplateDir() string {
	if e.Config.Template == "" {
		return ""
	}
	templatesDir := getTemplatesDir()
	return filepath.Join(templatesDir, e.Config.Template)
}

// IsUsingTemplate returns true if the environment is using a template
func (e *Environment) IsUsingTemplate() bool {
	return e.Config.Template != "" && !e.hasLocalMainTF()
}

// GetTemplateReference returns the template name if using a template
func (e *Environment) GetTemplateReference() string {
	if e.IsUsingTemplate() {
		return e.Config.Template
	}
	return ""
}

// hasLocalMainTF checks if there's a local main.tf file
func (e *Environment) hasLocalMainTF() bool {
	localPath := filepath.Join(e.Path, "main.tf")
	_, err := os.Stat(localPath)
	return err == nil
}

// GetDeploymentStatus returns the actual deployment status based on OpenTofu state files
// This is the source of truth for whether resources are actually deployed or destroyed
func (e *Environment) GetDeploymentStatus() string {
	stateFile := e.getStateFilePath()
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		return "destroyed"
	}

	// Check if state file has actual resources (not just empty state)
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return "destroyed" // Can't read state file, assume destroyed
	}

	// Simple check for resources in state file
	if strings.Contains(string(data), `"resources":[]`) {
		return "destroyed" // Empty resources array means destroyed
	}

	return "deployed"
}

// getStateFilePath returns the path to the terraform.tfstate file for this environment
func (e *Environment) getStateFilePath() string {
	stateDir := getStateDir()

	// Try new deployment structure first
	deploymentStateFile := filepath.Join(stateDir, "deployments", e.Name, "terraform.tfstate")
	if _, err := os.Stat(deploymentStateFile); err == nil {
		return deploymentStateFile
	}

	// Fall back to old structure (direct in state dir)
	oldStateFile := filepath.Join(stateDir, e.Name, "terraform.tfstate")
	if _, err := os.Stat(oldStateFile); err == nil {
		return oldStateFile
	}

	// Default to deployment structure (for consistency)
	return deploymentStateFile
}

// getStateDir returns the state directory using the same logic as OpenTofu client
func getStateDir() string {
	// First check environment variable (explicit override)
	if stateDir := os.Getenv("PROVISIONER_STATE_DIR"); stateDir != "" {
		return stateDir
	}

	// Auto-detect system installation
	if _, err := os.Stat("/var/lib/provisioner"); err == nil {
		return "/var/lib/provisioner"
	}

	// Fall back to development default
	return "state"
}

// GetLastStateChangeTime returns the last time the state file was modified
// This provides more accurate timing than managed state timestamps
func (e *Environment) GetLastStateChangeTime() *time.Time {
	stateFile := e.getStateFilePath()
	if info, err := os.Stat(stateFile); err == nil {
		modTime := info.ModTime()
		return &modTime
	}
	return nil
}

// getTemplatesDir returns the templates directory path
func getTemplatesDir() string {
	if stateDir := os.Getenv("PROVISIONER_STATE_DIR"); stateDir != "" {
		return filepath.Join(stateDir, "templates")
	}
	return "/var/lib/provisioner/templates"
}

// GetDeploySchedules returns deploy schedules as a slice, handling both string and []string formats
func (c *Config) GetDeploySchedules() ([]string, error) {
	return normalizeScheduleField(c.DeploySchedule)
}

// GetDestroySchedules returns destroy schedules as a slice, handling both string and []string formats
func (c *Config) GetDestroySchedules() ([]string, error) {
	return normalizeScheduleField(c.DestroySchedule)
}

// normalizeScheduleField converts interface{} schedule field to []string
func normalizeScheduleField(field interface{}) ([]string, error) {
	if field == nil {
		return nil, fmt.Errorf("schedule field is nil")
	}

	switch v := field.(type) {
	case bool:
		if !v { // false means permanent deployment (no schedule)
			return []string{}, nil // Empty slice = no schedules
		}
		return nil, fmt.Errorf("schedule boolean must be false (true is invalid)")
	case string:
		return []string{v}, nil
	case []interface{}:
		schedules := make([]string, len(v))
		for i, item := range v {
			if str, ok := item.(string); ok {
				schedules[i] = str
			} else {
				return nil, fmt.Errorf("schedule array must contain strings, got %T at index %d", item, i)
			}
		}
		return schedules, nil
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("schedule must be false, string, or array of strings, got %T", v)
	}
}

// getDefaultEnvironmentsDir returns the default environments directory
func getDefaultEnvironmentsDir() string {
	// First check for explicit environments directory override
	if envDir := os.Getenv("PROVISIONER_ENVIRONMENTS_DIR"); envDir != "" {
		return envDir
	}

	// Use config directory + environments if PROVISIONER_CONFIG_DIR is set
	if configDir := os.Getenv("PROVISIONER_CONFIG_DIR"); configDir != "" {
		return filepath.Join(configDir, "environments")
	}

	// Auto-detect system installation
	if _, err := os.Stat("/etc/provisioner"); err == nil {
		return "/etc/provisioner/environments"
	}

	// Default to relative path for development
	return "environments"
}

// CreateEnvironment creates a new environment with the given configuration
func CreateEnvironment(name, template, description, deploySchedule, destroySchedule string, enabled bool) error {
	environmentsDir := getDefaultEnvironmentsDir()
	envPath := filepath.Join(environmentsDir, name)

	// Check if environment already exists
	if _, err := os.Stat(envPath); err == nil {
		return fmt.Errorf("environment '%s' already exists", name)
	}

	// Create environment directory
	if err := os.MkdirAll(envPath, 0755); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}

	// Create config
	config := Config{
		Enabled:     enabled,
		Template:    template,
		Description: description,
	}

	// Set schedules
	if deploySchedule != "" {
		config.DeploySchedule = deploySchedule
	}
	if destroySchedule != "" {
		config.DestroySchedule = destroySchedule
	}

	// Write config.json
	configPath := filepath.Join(envPath, "config.json")
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Create main.tf if no template specified
	if template == "" {
		mainTFPath := filepath.Join(envPath, "main.tf")
		mainTFContent := `# OpenTofu configuration for environment: ` + name + `
# Add your infrastructure configuration here

terraform {
  required_providers {
    # Add required providers here
  }
}

# Add your resources here
`
		if err := os.WriteFile(mainTFPath, []byte(mainTFContent), 0644); err != nil {
			return fmt.Errorf("failed to create main.tf: %w", err)
		}
	}

	return nil
}

// UpdateEnvironment updates an existing environment configuration
func UpdateEnvironment(name, template, description, deploySchedule, destroySchedule string, enabled *bool) error {
	environmentsDir := getDefaultEnvironmentsDir()
	envPath := filepath.Join(environmentsDir, name)
	configPath := filepath.Join(envPath, "config.json")

	// Check if environment exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' does not exist", name)
	}

	// Load existing config
	config, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load existing config: %w", err)
	}

	// Update fields if provided
	if template != "" {
		config.Template = template
	}
	if description != "" {
		config.Description = description
	}
	if deploySchedule != "" {
		config.DeploySchedule = deploySchedule
	}
	if destroySchedule != "" {
		config.DestroySchedule = destroySchedule
	}
	if enabled != nil {
		config.Enabled = *enabled
	}

	// Write updated config
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// RemoveEnvironment removes an environment and its directory
func RemoveEnvironment(name string) error {
	environmentsDir := getDefaultEnvironmentsDir()
	envPath := filepath.Join(environmentsDir, name)

	// Check if environment exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' does not exist", name)
	}

	// Remove the entire environment directory
	if err := os.RemoveAll(envPath); err != nil {
		return fmt.Errorf("failed to remove environment directory: %w", err)
	}

	return nil
}

// ValidateEnvironment validates an environment's configuration and OpenTofu syntax
func ValidateEnvironment(name string) error {
	environmentsDir := getDefaultEnvironmentsDir()
	envPath := filepath.Join(environmentsDir, name)
	configPath := filepath.Join(envPath, "config.json")

	// Check if environment exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment does not exist")
	}

	// Load and validate config
	config, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Create environment object for validation
	env := Environment{
		Name:   name,
		Config: config,
		Path:   envPath,
	}

	// Validate that environment has a valid OpenTofu configuration
	if !env.HasMainTF() {
		return fmt.Errorf("no valid OpenTofu configuration found (missing main.tf)")
	}

	// Validate schedules
	if config.DeploySchedule != nil {
		if _, err := config.GetDeploySchedules(); err != nil {
			return fmt.Errorf("invalid deploy schedule: %w", err)
		}
	}

	if config.DestroySchedule != nil {
		if _, err := config.GetDestroySchedules(); err != nil {
			return fmt.Errorf("invalid destroy schedule: %w", err)
		}
	}

	// Validate template reference if specified
	if config.Template != "" {
		templatesDir := getTemplatesDir()
		templatePath := filepath.Join(templatesDir, config.Template)
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			return fmt.Errorf("referenced template '%s' does not exist", config.Template)
		}
	}

	return nil
}
