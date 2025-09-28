package job

import (
	"fmt"
	"path/filepath"
	"time"
)

// JobType defines the type of job to execute
type JobType string

const (
	JobTypeScript   JobType = "script"   // Execute shell script
	JobTypeCommand  JobType = "command"  // Execute single command
	JobTypeTemplate JobType = "template" // Deploy/update template within workspace
)

// JobStatus represents the current status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusSuccess   JobStatus = "success"
	JobStatusFailed    JobStatus = "failed"
	JobStatusTimeout   JobStatus = "timeout"
	JobStatusDisabled  JobStatus = "disabled"
)

// Job represents a scheduled job within a workspace
type Job struct {
	Name        string            `json:"name"`
	WorkspaceID string            `json:"workspace_id"`
	JobType     JobType           `json:"type"`
	Schedule    interface{}       `json:"schedule"`    // String or []string for CRON expressions
	Script      string            `json:"script,omitempty"`      // Shell script content
	Command     string            `json:"command,omitempty"`     // Single command to execute
	Template    string            `json:"template,omitempty"`    // Template name for template jobs
	Environment map[string]string `json:"environment,omitempty"` // Environment variables
	WorkingDir  string            `json:"working_dir,omitempty"` // Working directory (relative to workspace)
	Timeout     string            `json:"timeout,omitempty"`     // Timeout duration (e.g., "30m", "1h")
	Enabled     bool              `json:"enabled"`
	Description string            `json:"description,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`  // Job dependencies
}

// JobExecution represents a single execution instance of a job
type JobExecution struct {
	JobName     string    `json:"job_name"`
	WorkspaceID string    `json:"workspace_id"`
	Status      JobStatus `json:"status"`
	StartTime   time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Duration    time.Duration `json:"duration"`
	ExitCode    int       `json:"exit_code"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
	PID         int       `json:"pid,omitempty"`
}

// JobState tracks the persistent state of a job across scheduler restarts
type JobState struct {
	Name               string        `json:"name"`
	WorkspaceID        string        `json:"workspace_id"`
	Status             JobStatus     `json:"status"`
	LastRun            *time.Time    `json:"last_run,omitempty"`
	LastSuccess        *time.Time    `json:"last_success,omitempty"`
	LastFailure        *time.Time    `json:"last_failure,omitempty"`
	LastError          string        `json:"last_error,omitempty"`
	LastExitCode       int           `json:"last_exit_code"`
	RunCount           int           `json:"run_count"`
	SuccessCount       int           `json:"success_count"`
	FailureCount       int           `json:"failure_count"`
	LastConfigModified *time.Time    `json:"last_config_modified,omitempty"`
	NextRun            *time.Time    `json:"next_run,omitempty"`
}

// GetSchedules returns job schedules as a slice, handling both string and []string formats
func (j *Job) GetSchedules() ([]string, error) {
	return normalizeScheduleField(j.Schedule)
}

// GetTimeoutDuration parses the timeout string and returns a duration
func (j *Job) GetTimeoutDuration() (time.Duration, error) {
	if j.Timeout == "" {
		return 10 * time.Minute, nil // Default timeout
	}
	return time.ParseDuration(j.Timeout)
}

// normalizeScheduleField converts interface{} schedule field to []string (reused from workspace package)
func normalizeScheduleField(field interface{}) ([]string, error) {
	if field == nil {
		return nil, nil
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

// Validate validates the job configuration
func (j *Job) Validate() error {
	if j.Name == "" {
		return fmt.Errorf("job name is required")
	}

	if j.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}

	// Validate job type and required fields
	switch j.JobType {
	case JobTypeScript:
		if j.Script == "" {
			return fmt.Errorf("script content is required for script jobs")
		}
	case JobTypeCommand:
		if j.Command == "" {
			return fmt.Errorf("command is required for command jobs")
		}
	case JobTypeTemplate:
		if j.Template == "" {
			return fmt.Errorf("template name is required for template jobs")
		}
	default:
		return fmt.Errorf("invalid job type: %s", j.JobType)
	}

	// Validate schedule if provided
	if j.Schedule != nil {
		if _, err := j.GetSchedules(); err != nil {
			return fmt.Errorf("invalid schedule: %w", err)
		}
	}

	// Validate timeout if provided
	if j.Timeout != "" {
		if _, err := j.GetTimeoutDuration(); err != nil {
			return fmt.Errorf("invalid timeout duration '%s': %w", j.Timeout, err)
		}
	}

	return nil
}

// GetWorkingDirectory returns the absolute working directory for job execution
func (j *Job) GetWorkingDirectory(workspaceDeploymentDir string) string {
	if j.WorkingDir == "" {
		return workspaceDeploymentDir
	}

	// If working dir is absolute, use it as-is
	if filepath.IsAbs(j.WorkingDir) {
		return j.WorkingDir
	}

	// Otherwise, make it relative to workspace deployment directory
	return filepath.Join(workspaceDeploymentDir, j.WorkingDir)
}

// JobConfigToJob converts a workspace JobConfig to a job.Job
// This allows the job package to work with workspace configurations without circular imports
func JobConfigToJob(workspaceID string, config interface{}) (*Job, error) {
	// Convert the config to a map for flexible handling
	configMap, ok := config.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid job config format")
	}

	job := &Job{
		WorkspaceID: workspaceID,
	}

	// Extract and validate required fields
	if name, ok := configMap["name"].(string); ok {
		job.Name = name
	} else {
		return nil, fmt.Errorf("job name is required")
	}

	if jobType, ok := configMap["type"].(string); ok {
		job.JobType = JobType(jobType)
	} else {
		return nil, fmt.Errorf("job type is required")
	}

	// Extract optional fields
	if script, ok := configMap["script"].(string); ok {
		job.Script = script
	}
	if command, ok := configMap["command"].(string); ok {
		job.Command = command
	}
	if template, ok := configMap["template"].(string); ok {
		job.Template = template
	}
	if workingDir, ok := configMap["working_dir"].(string); ok {
		job.WorkingDir = workingDir
	}
	if timeout, ok := configMap["timeout"].(string); ok {
		job.Timeout = timeout
	}
	if enabled, ok := configMap["enabled"].(bool); ok {
		job.Enabled = enabled
	} else {
		job.Enabled = true // Default to enabled
	}
	if description, ok := configMap["description"].(string); ok {
		job.Description = description
	}

	// Extract schedule
	if schedule, exists := configMap["schedule"]; exists {
		job.Schedule = schedule
	}

	// Extract environment variables
	if env, ok := configMap["environment"].(map[string]interface{}); ok {
		job.Environment = make(map[string]string)
		for key, value := range env {
			if strValue, ok := value.(string); ok {
				job.Environment[key] = strValue
			}
		}
	}

	// Extract dependencies
	if deps, ok := configMap["depends_on"].([]interface{}); ok {
		job.DependsOn = make([]string, len(deps))
		for i, dep := range deps {
			if strDep, ok := dep.(string); ok {
				job.DependsOn[i] = strDep
			}
		}
	}

	// Validate the job
	if err := job.Validate(); err != nil {
		return nil, fmt.Errorf("job validation failed: %w", err)
	}

	return job, nil
}