package job

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StandaloneJobConfig represents a job configuration file
type StandaloneJobConfig struct {
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
	Tags        []string          `json:"tags,omitempty"`
}

// Validate validates the standalone job configuration
func (sjc *StandaloneJobConfig) Validate() error {
	if sjc.Name == "" {
		return fmt.Errorf("job name is required")
	}

	if sjc.Type == "" {
		return fmt.Errorf("job type is required")
	}

	// Validate job type
	switch sjc.Type {
	case "script":
		if sjc.Script == "" {
			return fmt.Errorf("script is required for script jobs")
		}
	case "command":
		if sjc.Command == "" {
			return fmt.Errorf("command is required for command jobs")
		}
	case "template":
		if sjc.Template == "" {
			return fmt.Errorf("template is required for template jobs")
		}
	default:
		return fmt.Errorf("invalid job type: %s", sjc.Type)
	}

	// Validate schedule
	if sjc.Schedule == nil {
		return fmt.Errorf("schedule is required")
	}

	// Parse schedule to validate format
	schedules, err := parseScheduleField(sjc.Schedule)
	if err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	// Basic validation that schedule strings are not empty
	for _, schedule := range schedules {
		if schedule == "" {
			return fmt.Errorf("empty schedule expression found")
		}
		// Basic CRON format check (5 fields separated by spaces)
		fields := strings.Fields(schedule)
		if len(fields) != 5 {
			return fmt.Errorf("invalid schedule expression '%s': expected 5 fields, got %d", schedule, len(fields))
		}
	}

	return nil
}

// ToJob converts the standalone job configuration to a Job
func (sjc *StandaloneJobConfig) ToJob() (*Job, error) {
	job := &Job{
		Name:        sjc.Name,
		WorkspaceID: "_standalone_",
		Schedule:    sjc.Schedule,
		Environment: sjc.Environment,
		WorkingDir:  sjc.WorkingDir,
		Timeout:     sjc.Timeout,
		Enabled:     sjc.Enabled,
		Description: sjc.Description,
	}

	// Set job type and type-specific fields
	switch sjc.Type {
	case "script":
		job.JobType = JobTypeScript
		job.Script = sjc.Script
	case "command":
		job.JobType = JobTypeCommand
		job.Command = sjc.Command
	case "template":
		job.JobType = JobTypeTemplate
		job.Template = sjc.Template
	default:
		return nil, fmt.Errorf("invalid job type: %s", sjc.Type)
	}

	// Validate the resulting job
	if err := job.Validate(); err != nil {
		return nil, fmt.Errorf("job validation failed: %w", err)
	}

	return job, nil
}

// StandaloneJobManager handles standalone jobs that aren't tied to workspaces
type StandaloneJobManager struct {
	jobsDir    string
	stateDir   string
	manager    *Manager
}

// NewStandaloneJobManager creates a new standalone job manager
func NewStandaloneJobManager(jobsDir, stateDir string, manager *Manager) *StandaloneJobManager {
	return &StandaloneJobManager{
		jobsDir:  jobsDir,
		stateDir: stateDir,
		manager:  manager,
	}
}

// LoadStandaloneJobs loads all standalone job configurations
func (sjm *StandaloneJobManager) LoadStandaloneJobs() ([]StandaloneJobConfig, error) {
	var jobs []StandaloneJobConfig

	// Check if jobs directory exists
	if _, err := os.Stat(sjm.jobsDir); os.IsNotExist(err) {
		return jobs, nil // No jobs directory, return empty list
	}

	// Read all .json files in the jobs directory
	entries, err := os.ReadDir(sjm.jobsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read jobs directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		jobPath := filepath.Join(sjm.jobsDir, entry.Name())
		jobConfig, err := sjm.loadStandaloneJobConfig(jobPath)
		if err != nil {
			fmt.Printf("Warning: failed to load job %s: %v\n", entry.Name(), err)
			continue
		}

		// If no name is specified, derive from filename
		if jobConfig.Name == "" {
			jobConfig.Name = strings.TrimSuffix(entry.Name(), ".json")
		}

		jobs = append(jobs, jobConfig)
	}

	return jobs, nil
}

// loadStandaloneJobConfig loads a single job configuration file
func (sjm *StandaloneJobManager) loadStandaloneJobConfig(configPath string) (StandaloneJobConfig, error) {
	var config StandaloneJobConfig

	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

// ProcessStandaloneJobs processes all standalone jobs for scheduling
func (sjm *StandaloneJobManager) ProcessStandaloneJobs() error {
	jobs, err := sjm.LoadStandaloneJobs()
	if err != nil {
		return fmt.Errorf("failed to load standalone jobs: %w", err)
	}

	// Convert to interface{} format and process with job manager
	jobConfigInterfaces := make([]interface{}, 0, len(jobs))
	activeJobNames := make([]string, 0, len(jobs))

	for _, jobConfig := range jobs {
		// Validate job configuration
		if err := sjm.validateStandaloneJob(jobConfig); err != nil {
			fmt.Printf("Warning: invalid job configuration %s: %v\n", jobConfig.Name, err)
			continue
		}

		configMap := map[string]interface{}{
			"name":        jobConfig.Name,
			"type":        jobConfig.Type,
			"schedule":    jobConfig.Schedule,
			"script":      jobConfig.Script,
			"command":     jobConfig.Command,
			"template":    jobConfig.Template,
			"environment": jobConfig.Environment,
			"working_dir": jobConfig.WorkingDir,
			"timeout":     jobConfig.Timeout,
			"enabled":     jobConfig.Enabled,
			"description": jobConfig.Description,
		}

		jobConfigInterfaces = append(jobConfigInterfaces, configMap)
		activeJobNames = append(activeJobNames, jobConfig.Name)
	}

	// Process jobs using the standard job manager with special workspace ID
	const standaloneWorkspaceID = "_standalone_"
	if len(jobConfigInterfaces) > 0 {
		sjm.manager.ProcessWorkspaceJobs(standaloneWorkspaceID, jobConfigInterfaces, time.Now())
	}

	// Cleanup old job states that no longer exist
	sjm.manager.stateManager.CleanupJobStates(standaloneWorkspaceID, activeJobNames)

	return nil
}

// validateStandaloneJob validates a standalone job configuration
func (sjm *StandaloneJobManager) validateStandaloneJob(job StandaloneJobConfig) error {
	if job.Name == "" {
		return fmt.Errorf("job name is required")
	}

	// Validate job type and required fields
	switch job.Type {
	case "script":
		if job.Script == "" {
			return fmt.Errorf("script content is required for script jobs")
		}
	case "command":
		if job.Command == "" {
			return fmt.Errorf("command is required for command jobs")
		}
	case "template":
		if job.Template == "" {
			return fmt.Errorf("template name is required for template jobs")
		}
	default:
		return fmt.Errorf("invalid job type: %s (must be script, command, or template)", job.Type)
	}

	return nil
}

// ListStandaloneJobs returns all standalone job configurations
func (sjm *StandaloneJobManager) ListStandaloneJobs() ([]StandaloneJobConfig, error) {
	return sjm.LoadStandaloneJobs()
}

// GetStandaloneJobStates returns all job states for standalone jobs
func (sjm *StandaloneJobManager) GetStandaloneJobStates() map[string]*JobState {
	const standaloneWorkspaceID = "_standalone_"
	return sjm.manager.GetAllJobStates(standaloneWorkspaceID)
}

// ExecuteStandaloneJob executes a standalone job immediately
func (sjm *StandaloneJobManager) ExecuteStandaloneJob(jobName string) error {
	jobs, err := sjm.LoadStandaloneJobs()
	if err != nil {
		return fmt.Errorf("failed to load standalone jobs: %w", err)
	}

	// Find the job
	var targetJob *StandaloneJobConfig
	for _, job := range jobs {
		if job.Name == jobName {
			targetJob = &job
			break
		}
	}

	if targetJob == nil {
		return fmt.Errorf("standalone job '%s' not found", jobName)
	}

	// Convert to interface{} format
	configMap := map[string]interface{}{
		"name":        targetJob.Name,
		"type":        targetJob.Type,
		"schedule":    targetJob.Schedule,
		"script":      targetJob.Script,
		"command":     targetJob.Command,
		"template":    targetJob.Template,
		"environment": targetJob.Environment,
		"working_dir": targetJob.WorkingDir,
		"timeout":     targetJob.Timeout,
		"enabled":     targetJob.Enabled,
		"description": targetJob.Description,
	}

	const standaloneWorkspaceID = "_standalone_"
	return sjm.manager.ManualExecuteJob(standaloneWorkspaceID, jobName, configMap)
}

// KillStandaloneJob kills a running standalone job
func (sjm *StandaloneJobManager) KillStandaloneJob(jobName string) error {
	const standaloneWorkspaceID = "_standalone_"
	return sjm.manager.KillJob(standaloneWorkspaceID, jobName)
}

// CreateStandaloneJob creates a new standalone job configuration file
func (sjm *StandaloneJobManager) CreateStandaloneJob(jobName string, config StandaloneJobConfig) error {
	// Ensure jobs directory exists
	if err := os.MkdirAll(sjm.jobsDir, 0755); err != nil {
		return fmt.Errorf("failed to create jobs directory: %w", err)
	}

	// Set the name if not provided
	if config.Name == "" {
		config.Name = jobName
	}

	// Validate the job
	if err := sjm.validateStandaloneJob(config); err != nil {
		return fmt.Errorf("invalid job configuration: %w", err)
	}

	// Write the configuration file
	jobPath := filepath.Join(sjm.jobsDir, jobName+".json")

	// Check if job already exists
	if _, err := os.Stat(jobPath); err == nil {
		return fmt.Errorf("job '%s' already exists", jobName)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal job config: %w", err)
	}

	if err := os.WriteFile(jobPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write job config: %w", err)
	}

	return nil
}

// RemoveStandaloneJob removes a standalone job configuration
func (sjm *StandaloneJobManager) RemoveStandaloneJob(jobName string) error {
	jobPath := filepath.Join(sjm.jobsDir, jobName+".json")

	if _, err := os.Stat(jobPath); os.IsNotExist(err) {
		return fmt.Errorf("job '%s' does not exist", jobName)
	}

	return os.Remove(jobPath)
}

// parseScheduleField parses a schedule field that can be a string or array of strings
func parseScheduleField(schedule interface{}) ([]string, error) {
	switch s := schedule.(type) {
	case string:
		return []string{s}, nil
	case []string:
		return s, nil
	case []interface{}:
		var schedules []string
		for _, item := range s {
			if str, ok := item.(string); ok {
				schedules = append(schedules, str)
			} else {
				return nil, fmt.Errorf("schedule array contains non-string element: %v", item)
			}
		}
		return schedules, nil
	default:
		return nil, fmt.Errorf("schedule must be a string or array of strings, got %T", schedule)
	}
}