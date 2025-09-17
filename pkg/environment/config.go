package environment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Name            string      `json:"name"`
	Enabled         bool        `json:"enabled"`
	DeploySchedule  interface{} `json:"deploy_schedule"`
	DestroySchedule interface{} `json:"destroy_schedule"`
	Description     string      `json:"description"`
}

type Environment struct {
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

		// Load all environments (enabled check will be done during scheduling)
		environments = append(environments, Environment{
			Config: config,
			Path:   envPath,
		})
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
	return filepath.Join(e.Path, "main.tf")
}

func (e *Environment) HasMainTF() bool {
	_, err := os.Stat(e.GetMainTFPath())
	return err == nil
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
		return nil, fmt.Errorf("schedule must be string or array of strings, got %T", v)
	}
}
