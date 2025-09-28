package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Enabled         bool                       `json:"enabled"`
	Template        string                     `json:"template,omitempty"`
	DeploySchedule  interface{}                `json:"deploy_schedule"`
	DestroySchedule interface{}                `json:"destroy_schedule"`
	ModeSchedules   map[string]interface{}     `json:"mode_schedules,omitempty"`
	Jobs            []JobConfig                `json:"jobs,omitempty"`
	Description     string                     `json:"description"`
}

// JobConfig represents a job configuration in the workspace
// This avoids circular imports by not depending on the job package
type JobConfig struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`          // "script", "command", "template"
	Schedule    interface{}       `json:"schedule"`      // String or []string for CRON expressions
	Script      string            `json:"script,omitempty"`
	Command     string            `json:"command,omitempty"`
	Template    string            `json:"template,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	Enabled     bool              `json:"enabled"`
	Description string            `json:"description,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"` // Job dependencies
}

type Workspace struct {
	Name   string // Derived from folder name
	Config Config
	Path   string
}

func LoadWorkspaces(workspacesDir string) ([]Workspace, error) {
	var workspaces []Workspace

	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read workspaces directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		wsPath := filepath.Join(workspacesDir, entry.Name())
		configPath := filepath.Join(wsPath, "config.json")

		// Check if config.json exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		config, err := loadConfig(configPath)
		if err != nil {
			fmt.Printf("Warning: failed to load config for %s: %v\n", entry.Name(), err)
			continue
		}

		// Create workspace
		ws := Workspace{
			Name:   entry.Name(), // Use folder name as workspace name
			Config: config,
			Path:   wsPath,
		}

		// Validate that the workspace has either a local main.tf or a valid template
		if !ws.HasMainTF() {
			if ws.Config.Template == "" {
				fmt.Printf("Warning: workspace %s has no main.tf and no template specified\n", entry.Name())
			} else {
				fmt.Printf("Warning: workspace %s references template '%s' but template not found\n", entry.Name(), ws.Config.Template)
			}
			continue
		}

		// Validate job dependencies for circular dependencies
		if err := ValidateJobDependencies(ws.Config.Jobs); err != nil {
			return nil, fmt.Errorf("workspace %s has invalid job dependencies: %w", entry.Name(), err)
		}

		// Load all workspaces (enabled check will be done during scheduling)
		workspaces = append(workspaces, ws)
	}

	return workspaces, nil
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

func (w *Workspace) GetMainTFPath() string {
	// Check for local main.tf first (highest priority)
	localPath := filepath.Join(w.Path, "main.tf")
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	// If no local main.tf and template is specified, use template path
	if w.Config.Template != "" {
		templatesDir := getTemplatesDir()
		templatePath := filepath.Join(templatesDir, w.Config.Template, "main.tf")
		if _, err := os.Stat(templatePath); err == nil {
			return templatePath
		}
	}

	// Return local path even if it doesn't exist (for error handling)
	return localPath
}

func (w *Workspace) HasMainTF() bool {
	// Check for local main.tf first
	localPath := filepath.Join(w.Path, "main.tf")
	if _, err := os.Stat(localPath); err == nil {
		return true
	}

	// Check for template main.tf if template is specified
	if w.Config.Template != "" {
		templatesDir := getTemplatesDir()
		templatePath := filepath.Join(templatesDir, w.Config.Template, "main.tf")
		if _, err := os.Stat(templatePath); err == nil {
			return true
		}
	}

	return false
}

// GetTemplateDir returns the directory path for the template if one is specified
func (w *Workspace) GetTemplateDir() string {
	if w.Config.Template == "" {
		return ""
	}
	templatesDir := getTemplatesDir()
	return filepath.Join(templatesDir, w.Config.Template)
}

// IsUsingTemplate returns true if the workspace is using a template
func (w *Workspace) IsUsingTemplate() bool {
	return w.Config.Template != "" && !w.hasLocalMainTF()
}

// GetTemplateReference returns the template name if using a template
func (w *Workspace) GetTemplateReference() string {
	if w.IsUsingTemplate() {
		return w.Config.Template
	}
	return ""
}

// hasLocalMainTF checks if there's a local main.tf file
func (w *Workspace) hasLocalMainTF() bool {
	localPath := filepath.Join(w.Path, "main.tf")
	_, err := os.Stat(localPath)
	return err == nil
}

// GetDeploymentStatus returns the actual deployment status based on OpenTofu state files
// This is the source of truth for whether resources are actually deployed or destroyed
func (w *Workspace) GetDeploymentStatus() string {
	stateFile := w.getStateFilePath()
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

// getStateFilePath returns the path to the terraform.tfstate file for this workspace
func (w *Workspace) getStateFilePath() string {
	stateDir := getStateDir()

	// Try new deployment structure first
	deploymentStateFile := filepath.Join(stateDir, "deployments", w.Name, "terraform.tfstate")
	if _, err := os.Stat(deploymentStateFile); err == nil {
		return deploymentStateFile
	}

	// Fall back to old structure (direct in state dir)
	oldStateFile := filepath.Join(stateDir, w.Name, "terraform.tfstate")
	if _, err := os.Stat(oldStateFile); err == nil {
		return oldStateFile
	}

	// Default to deployment structure (for consistency)
	return deploymentStateFile
}

// getStateDir returns the state directory using the same logic as OpenTofu client
func getStateDir() string {
	// First check workspace variable (explicit override)
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
func (w *Workspace) GetLastStateChangeTime() *time.Time {
	stateFile := w.getStateFilePath()
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

// getDefaultWorkspacesDir returns the default workspaces directory
func getDefaultWorkspacesDir() string {
	// First check for explicit workspaces directory override
	if workspaceDir := os.Getenv("PROVISIONER_WORKSPACES_DIR"); workspaceDir != "" {
		return workspaceDir
	}

	// Use config directory + workspaces if PROVISIONER_CONFIG_DIR is set
	if configDir := os.Getenv("PROVISIONER_CONFIG_DIR"); configDir != "" {
		return filepath.Join(configDir, "workspaces")
	}

	// Auto-detect system installation
	if _, err := os.Stat("/etc/provisioner"); err == nil {
		return "/etc/provisioner/workspaces"
	}

	// Default to relative path for development
	return "workspaces"
}

// CreateWorkspace creates a new workspace with the given configuration
func CreateWorkspace(name, template, description, deploySchedule, destroySchedule string, enabled bool) error {
	workspacesDir := getDefaultWorkspacesDir()
	wsPath := filepath.Join(workspacesDir, name)

	// Check if workspace already exists
	if _, err := os.Stat(wsPath); err == nil {
		return fmt.Errorf("workspace '%s' already exists", name)
	}

	// Create workspace directory
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
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
	configPath := filepath.Join(wsPath, "config.json")
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Create main.tf if no template specified
	if template == "" {
		mainTFPath := filepath.Join(wsPath, "main.tf")
		mainTFContent := `# OpenTofu configuration for workspace: ` + name + `
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

// UpdateWorkspace updates an existing workspace configuration
func UpdateWorkspace(name, template, description, deploySchedule, destroySchedule string, enabled *bool) error {
	workspacesDir := getDefaultWorkspacesDir()
	wsPath := filepath.Join(workspacesDir, name)
	configPath := filepath.Join(wsPath, "config.json")

	// Check if workspace exists
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		return fmt.Errorf("workspace '%s' does not exist", name)
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

// RemoveWorkspace removes a workspace and its directory
func RemoveWorkspace(name string) error {
	workspacesDir := getDefaultWorkspacesDir()
	wsPath := filepath.Join(workspacesDir, name)

	// Check if workspace exists
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		return fmt.Errorf("workspace '%s' does not exist", name)
	}

	// Remove the entire workspace directory
	if err := os.RemoveAll(wsPath); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	return nil
}

// Validate validates the workspace configuration
func (c *Config) Validate() error {
	hasModeSchedules := c.ModeSchedules != nil && len(c.ModeSchedules) > 0
	hasDeploySchedule := c.DeploySchedule != nil

	// Mutually exclusive validation
	if hasModeSchedules && hasDeploySchedule {
		return fmt.Errorf("cannot specify both 'mode_schedules' and 'deploy_schedule'")
	}

	if !hasModeSchedules && !hasDeploySchedule {
		return fmt.Errorf("must specify either 'mode_schedules' or 'deploy_schedule'")
	}

	// Mode schedules require template
	if hasModeSchedules && c.Template == "" {
		return fmt.Errorf("'mode_schedules' requires 'template' field")
	}

	// Validate individual mode schedules
	if hasModeSchedules {
		for mode, schedule := range c.ModeSchedules {
			if _, err := normalizeScheduleField(schedule); err != nil {
				return fmt.Errorf("invalid schedule for mode '%s': %w", mode, err)
			}
		}
	}

	// Validate jobs
	for i, jobConfig := range c.Jobs {
		if err := validateJobConfig(jobConfig); err != nil {
			return fmt.Errorf("job %d (%s) validation failed: %w", i, jobConfig.Name, err)
		}
	}

	return nil
}

// GetJobConfigs returns all job configurations defined in this workspace
func (c *Config) GetJobConfigs() []JobConfig {
	return c.Jobs
}

// validateJobConfig validates a job configuration
func validateJobConfig(j JobConfig) error {
	if j.Name == "" {
		return fmt.Errorf("job name is required")
	}

	// Validate job type and required fields
	switch j.Type {
	case "script":
		if j.Script == "" {
			return fmt.Errorf("script content is required for script jobs")
		}
	case "command":
		if j.Command == "" {
			return fmt.Errorf("command is required for command jobs")
		}
	case "template":
		if j.Template == "" {
			return fmt.Errorf("template name is required for template jobs")
		}
	default:
		return fmt.Errorf("invalid job type: %s (must be script, command, or template)", j.Type)
	}

	// Validate schedule if provided
	if j.Schedule != nil {
		if _, err := normalizeScheduleField(j.Schedule); err != nil {
			return fmt.Errorf("invalid schedule: %w", err)
		}
	}

	// Validate timeout if provided
	if j.Timeout != "" {
		if _, err := time.ParseDuration(j.Timeout); err != nil {
			return fmt.Errorf("invalid timeout duration '%s': %w", j.Timeout, err)
		}
	}

	return nil
}

// GetModeSchedules returns all mode schedules as a map of mode -> []string
func (c *Config) GetModeSchedules() (map[string][]string, error) {
	if c.ModeSchedules == nil {
		return nil, nil
	}

	result := make(map[string][]string)
	for mode, schedule := range c.ModeSchedules {
		schedules, err := normalizeScheduleField(schedule)
		if err != nil {
			return nil, fmt.Errorf("invalid schedule for mode '%s': %w", mode, err)
		}
		result[mode] = schedules
	}
	return result, nil
}

// ValidateWorkspace validates a workspace's configuration and OpenTofu syntax
func ValidateWorkspace(name string) error {
	workspacesDir := getDefaultWorkspacesDir()
	wsPath := filepath.Join(workspacesDir, name)
	configPath := filepath.Join(wsPath, "config.json")

	// Check if workspace exists
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		return fmt.Errorf("workspace does not exist")
	}

	// Load and validate config
	config, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Validate config structure and schedule logic
	if err := config.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Create workspace object for validation
	ws := Workspace{
		Name:   name,
		Config: config,
		Path:   wsPath,
	}

	// Validate that workspace has a valid OpenTofu configuration
	if !ws.HasMainTF() {
		return fmt.Errorf("no valid OpenTofu configuration found (missing main.tf)")
	}

	// Validate schedules (legacy validation for backward compatibility)
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

// ValidateJobDependencies checks for circular dependencies in job configurations
func ValidateJobDependencies(jobs []JobConfig) error {
	if len(jobs) == 0 {
		return nil
	}

	// Build a map of job names to their dependencies
	jobsByName := make(map[string]*JobConfig)
	for i, job := range jobs {
		jobsByName[job.Name] = &jobs[i]
	}

	// Check for missing dependencies
	for _, job := range jobs {
		for _, depName := range job.DependsOn {
			if _, exists := jobsByName[depName]; !exists {
				return fmt.Errorf("job '%s' depends on non-existent job '%s'", job.Name, depName)
			}
		}
	}

	// Check for circular dependencies using DFS
	// States: 0 = unvisited, 1 = visiting, 2 = visited
	state := make(map[string]int)

	var dfs func(jobName string) error
	dfs = func(jobName string) error {
		if state[jobName] == 1 {
			return fmt.Errorf("circular dependency detected involving job '%s'", jobName)
		}
		if state[jobName] == 2 {
			return nil // Already processed
		}

		state[jobName] = 1 // Mark as visiting

		job, exists := jobsByName[jobName]
		if !exists {
			return fmt.Errorf("job '%s' not found", jobName)
		}

		// Visit dependencies
		for _, depName := range job.DependsOn {
			if err := dfs(depName); err != nil {
				return err
			}
		}

		state[jobName] = 2 // Mark as visited
		return nil
	}

	// Check all jobs for circular dependencies
	for jobName := range jobsByName {
		if state[jobName] == 0 {
			if err := dfs(jobName); err != nil {
				return err
			}
		}
	}

	return nil
}
