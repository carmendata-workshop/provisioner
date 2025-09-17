package environment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Name            string `json:"name"`
	Enabled         bool   `json:"enabled"`
	DeploySchedule  string `json:"deploy_schedule"`
	DestroySchedule string `json:"destroy_schedule"`
	Description     string `json:"description"`
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
