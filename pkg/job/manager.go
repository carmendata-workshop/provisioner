package job

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"provisioner/pkg/logging"
	"provisioner/pkg/opentofu"
	"provisioner/pkg/template"
)

// Manager coordinates job execution, state management, and scheduling
type Manager struct {
	stateManager    *StateManager
	templateManager *template.Manager
	tofuClient      opentofu.TofuClient
	stateDir        string
}

// NewManager creates a new job manager
func NewManager(stateDir string, tofuClient opentofu.TofuClient, templateManager *template.Manager) *Manager {
	jobStatePath := filepath.Join(stateDir, "jobs.json")
	stateManager := NewStateManager(jobStatePath)

	return &Manager{
		stateManager:    stateManager,
		templateManager: templateManager,
		tofuClient:      tofuClient,
		stateDir:        stateDir,
	}
}

// LoadState loads job states from disk
func (m *Manager) LoadState() error {
	return m.stateManager.LoadState()
}

// SaveState saves job states to disk
func (m *Manager) SaveState() error {
	return m.stateManager.SaveState()
}

// ExecuteJob executes a single job
func (m *Manager) ExecuteJob(job *Job) *JobExecution {
	// Get workspace deployment directory
	workspaceDeploymentDir := filepath.Join(m.stateDir, "deployments", job.WorkspaceID)

	// Create executor
	executor := NewExecutor(workspaceDeploymentDir, m.tofuClient, m.templateManager)

	// Update job state to running
	m.stateManager.SetJobStatus(job.WorkspaceID, job.Name, JobStatusRunning)
	if err := m.stateManager.SaveState(); err != nil {
		logging.LogWorkspace(job.WorkspaceID, "Failed to save job state: %v", err)
	}

	// Execute the job
	execution := executor.ExecuteJob(job)

	// Update state with execution results
	m.stateManager.UpdateJobExecution(execution)
	if err := m.stateManager.SaveState(); err != nil {
		logging.LogWorkspace(job.WorkspaceID, "Failed to save job state after execution: %v", err)
	}

	return execution
}

// ExecuteJobAsync executes a job asynchronously
func (m *Manager) ExecuteJobAsync(job *Job) {
	go func() {
		execution := m.ExecuteJob(job)
		logging.LogWorkspace(job.WorkspaceID, "JOB %s: Async execution completed with status %s",
			job.Name, execution.Status)
	}()
}

// GetJobState returns the current state of a job
func (m *Manager) GetJobState(workspaceID, jobName string) *JobState {
	return m.stateManager.GetJobState(workspaceID, jobName)
}

// GetAllJobStates returns all job states for a workspace
func (m *Manager) GetAllJobStates(workspaceID string) map[string]*JobState {
	return m.stateManager.GetAllJobStates(workspaceID)
}

// ShouldRunJob determines if a job should run based on its schedule and current state
func (m *Manager) ShouldRunJob(job *Job, now time.Time) bool {
	jobState := m.stateManager.GetJobState(job.WorkspaceID, job.Name)

	// Don't run if job is disabled
	if !job.Enabled {
		return false
	}

	// Don't run if already running
	if jobState.Status == JobStatusRunning {
		return false
	}

	// Don't retry failed jobs (until config changes)
	if jobState.Status == JobStatusFailed || jobState.Status == JobStatusTimeout {
		return false
	}

	// Check if any schedule has passed and we haven't run since then
	schedules, err := job.GetSchedules()
	if err != nil {
		logging.LogWorkspace(job.WorkspaceID, "JOB %s: Invalid schedule: %v", job.Name, err)
		return false
	}

	if len(schedules) == 0 {
		return false // No schedule defined
	}

	// Use the same schedule checking logic as workspace scheduling
	// This is a simplified check - you would integrate with the existing CRON parsing
	for _, scheduleStr := range schedules {
		// For now, this is a placeholder - you would use the existing ParseCron function
		// and getLastScheduledTimeToday logic from the scheduler package
		if m.shouldRunForSchedule(scheduleStr, now, jobState) {
			return true
		}
	}

	return false
}

// shouldRunForSchedule checks if a job should run for a specific schedule
func (m *Manager) shouldRunForSchedule(scheduleStr string, now time.Time, jobState *JobState) bool {
	// Skip special schedules in time-based processing
	if strings.HasPrefix(scheduleStr, "@") {
		return false // Special schedules are event-based, not time-based
	}

	// For CRON schedules, we need a simpler check here since we can't import scheduler
	// This is a basic time-based check - in practice, you would use proper CRON parsing
	if jobState.LastRun == nil {
		return true // Never run before
	}

	// Run if last run was more than 1 hour ago (simplified CRON check)
	return now.Sub(*jobState.LastRun) > time.Hour
}

// ProcessWorkspaceJobs processes all jobs for a workspace configuration
func (m *Manager) ProcessWorkspaceJobs(workspaceID string, jobConfigs []interface{}, now time.Time) {
	// Convert job configs to job objects
	activeJobs := make([]string, 0, len(jobConfigs))
	jobs := make([]*Job, 0, len(jobConfigs))

	for _, configInterface := range jobConfigs {
		job, err := JobConfigToJob(workspaceID, configInterface)
		if err != nil {
			logging.LogWorkspace(workspaceID, "Invalid job configuration: %v", err)
			continue
		}

		activeJobs = append(activeJobs, job.Name)
		jobs = append(jobs, job)
	}

	// Cleanup states for jobs that no longer exist
	m.stateManager.CleanupJobStates(workspaceID, activeJobs)

	// Check each job to see if it should run
	for _, job := range jobs {
		if m.ShouldRunJob(job, now) {
			logging.LogWorkspace(workspaceID, "JOB %s: Triggering execution", job.Name)
			m.ExecuteJobAsync(job)
		}
	}
}

// ManualExecuteJob executes a job immediately, bypassing schedule checks
func (m *Manager) ManualExecuteJob(workspaceID, jobName string, jobConfig interface{}) error {
	job, err := JobConfigToJob(workspaceID, jobConfig)
	if err != nil {
		return fmt.Errorf("invalid job configuration: %w", err)
	}

	if job.Name != jobName {
		return fmt.Errorf("job name mismatch: expected %s, got %s", jobName, job.Name)
	}

	jobState := m.stateManager.GetJobState(workspaceID, jobName)
	if jobState.Status == JobStatusRunning {
		return fmt.Errorf("job '%s' is already running", jobName)
	}

	logging.LogWorkspace(workspaceID, "JOB %s: Manual execution requested", jobName)

	// Execute synchronously for immediate feedback
	execution := m.ExecuteJob(job)

	if execution.Status == JobStatusSuccess {
		return nil
	} else {
		return fmt.Errorf("job execution failed: %s", execution.Error)
	}
}

// KillJob attempts to kill a running job
func (m *Manager) KillJob(workspaceID, jobName string) error {
	jobState := m.stateManager.GetJobState(workspaceID, jobName)
	if jobState.Status != JobStatusRunning {
		return fmt.Errorf("job '%s' is not running", jobName)
	}

	// This is a simplified implementation - in practice, you would need to track
	// the PID of running jobs and kill them. For now, just mark as failed.
	m.stateManager.SetJobStatus(workspaceID, jobName, JobStatusFailed)
	if err := m.stateManager.SaveState(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	logging.LogWorkspace(workspaceID, "JOB %s: Killed", jobName)
	return nil
}

// SetJobConfigModified marks jobs in a workspace as having modified configuration
func (m *Manager) SetJobConfigModified(workspaceID string, modTime time.Time) {
	// Mark all jobs in this workspace as having modified configuration
	jobStates := m.stateManager.GetAllJobStates(workspaceID)
	for jobName := range jobStates {
		m.stateManager.SetJobConfigModified(workspaceID, jobName, modTime)
	}
}

// ListJobs returns information about all jobs in a workspace
func (m *Manager) ListJobs(workspaceID string) map[string]*JobState {
	return m.stateManager.GetAllJobStates(workspaceID)
}

// ShouldRunJobForEvent determines if a job should run based on a deployment event
func (m *Manager) ShouldRunJobForEvent(job *Job, event DeploymentEvent) bool {
	jobState := m.stateManager.GetJobState(job.WorkspaceID, job.Name)

	// Don't run if job is disabled
	if !job.Enabled {
		return false
	}

	// Don't run if already running (only if we have a job state)
	if jobState != nil && jobState.Status == JobStatusRunning {
		return false
	}

	// Only process jobs for the same workspace as the event
	if job.WorkspaceID != event.GetWorkspaceID() {
		return false
	}

	// Check if any schedule matches this event
	schedules, err := job.GetSchedules()
	if err != nil {
		logging.LogWorkspace(job.WorkspaceID, "JOB %s: Invalid schedule: %v", job.Name, err)
		return false
	}

	for _, scheduleStr := range schedules {
		if event.MatchesSchedule(scheduleStr) {
			return true
		}
	}

	return false
}

// ProcessWorkspaceJobsForEvent processes jobs that should be triggered by a deployment event
func (m *Manager) ProcessWorkspaceJobsForEvent(workspaceID string, jobConfigs []interface{}, event DeploymentEvent) {
	// Convert job configs to job objects
	activeJobs := make([]string, 0, len(jobConfigs))
	jobs := make([]*Job, 0, len(jobConfigs))

	for _, configInterface := range jobConfigs {
		job, err := JobConfigToJob(workspaceID, configInterface)
		if err != nil {
			logging.LogWorkspace(workspaceID, "Invalid job configuration: %v", err)
			continue
		}

		activeJobs = append(activeJobs, job.Name)
		jobs = append(jobs, job)
	}

	// Cleanup states for jobs that no longer exist
	m.stateManager.CleanupJobStates(workspaceID, activeJobs)

	// Check each job to see if it should run for this event
	for _, job := range jobs {
		if m.ShouldRunJobForEvent(job, event) {
			logging.LogWorkspace(workspaceID, "JOB %s: Triggering execution due to event: %s", job.Name, event.GetType())
			m.ExecuteJobAsync(job)
		}
	}
}