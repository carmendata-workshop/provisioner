package environment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
