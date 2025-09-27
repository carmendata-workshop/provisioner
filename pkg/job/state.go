package job

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StateManager handles persistence of job states
type StateManager struct {
	statePath string
	state     *State
}

// State represents the persistent state of all jobs
type State struct {
	Jobs        map[string]*JobState `json:"jobs"`
	LastUpdated time.Time            `json:"last_updated"`
}

// NewStateManager creates a new job state manager
func NewStateManager(statePath string) *StateManager {
	return &StateManager{
		statePath: statePath,
	}
}

// LoadState loads job state from disk
func (sm *StateManager) LoadState() error {
	// Initialize empty state if file doesn't exist
	if _, err := os.Stat(sm.statePath); os.IsNotExist(err) {
		sm.state = &State{
			Jobs:        make(map[string]*JobState),
			LastUpdated: time.Now(),
		}
		return nil
	}

	data, err := os.ReadFile(sm.statePath)
	if err != nil {
		return fmt.Errorf("failed to read job state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal job state: %w", err)
	}

	if state.Jobs == nil {
		state.Jobs = make(map[string]*JobState)
	}

	sm.state = &state
	return nil
}

// SaveState saves job state to disk
func (sm *StateManager) SaveState() error {
	if sm.state == nil {
		return fmt.Errorf("no state to save")
	}

	sm.state.LastUpdated = time.Now()

	// Ensure state directory exists
	if err := os.MkdirAll(filepath.Dir(sm.statePath), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal job state: %w", err)
	}

	if err := os.WriteFile(sm.statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write job state file: %w", err)
	}

	return nil
}

// GetJobState returns the state for a specific job
func (sm *StateManager) GetJobState(workspaceID, jobName string) *JobState {
	if sm.state == nil {
		return nil
	}

	key := fmt.Sprintf("%s:%s", workspaceID, jobName)
	if jobState, exists := sm.state.Jobs[key]; exists {
		return jobState
	}

	// Create new job state
	jobState := &JobState{
		Name:        jobName,
		WorkspaceID: workspaceID,
		Status:      JobStatusPending,
	}
	sm.state.Jobs[key] = jobState
	return jobState
}

// SetJobState updates the state for a specific job
func (sm *StateManager) SetJobState(workspaceID, jobName string, jobState *JobState) {
	if sm.state == nil {
		sm.state = &State{
			Jobs:        make(map[string]*JobState),
			LastUpdated: time.Now(),
		}
	}

	key := fmt.Sprintf("%s:%s", workspaceID, jobName)
	sm.state.Jobs[key] = jobState
}

// UpdateJobExecution updates job state based on execution results
func (sm *StateManager) UpdateJobExecution(execution *JobExecution) {
	jobState := sm.GetJobState(execution.WorkspaceID, execution.JobName)

	jobState.Status = execution.Status
	jobState.RunCount++

	now := time.Now()
	jobState.LastRun = &now

	if execution.Status == JobStatusSuccess {
		jobState.LastSuccess = &now
		jobState.SuccessCount++
		jobState.LastError = ""
		jobState.LastExitCode = 0
	} else if execution.Status == JobStatusFailed || execution.Status == JobStatusTimeout {
		jobState.LastFailure = &now
		jobState.FailureCount++
		jobState.LastError = execution.Error
		jobState.LastExitCode = execution.ExitCode
	}

	sm.SetJobState(execution.WorkspaceID, execution.JobName, jobState)
}

// SetJobStatus updates just the status of a job
func (sm *StateManager) SetJobStatus(workspaceID, jobName string, status JobStatus) {
	jobState := sm.GetJobState(workspaceID, jobName)
	jobState.Status = status
	sm.SetJobState(workspaceID, jobName, jobState)
}

// SetJobConfigModified marks a job's configuration as modified
func (sm *StateManager) SetJobConfigModified(workspaceID, jobName string, modTime time.Time) {
	jobState := sm.GetJobState(workspaceID, jobName)
	jobState.LastConfigModified = &modTime

	// Reset failed state to allow retries after config changes
	if jobState.Status == JobStatusFailed || jobState.Status == JobStatusTimeout {
		jobState.Status = JobStatusPending
		jobState.LastError = ""
	}

	sm.SetJobState(workspaceID, jobName, jobState)
}

// GetAllJobStates returns all job states for a workspace
func (sm *StateManager) GetAllJobStates(workspaceID string) map[string]*JobState {
	if sm.state == nil {
		return make(map[string]*JobState)
	}

	result := make(map[string]*JobState)
	prefix := workspaceID + ":"

	for key, jobState := range sm.state.Jobs {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			jobName := key[len(prefix):]
			result[jobName] = jobState
		}
	}

	return result
}

// CleanupJobStates removes job states for jobs that no longer exist in configuration
func (sm *StateManager) CleanupJobStates(workspaceID string, activeJobs []string) {
	if sm.state == nil {
		return
	}

	activeJobsMap := make(map[string]bool)
	for _, jobName := range activeJobs {
		activeJobsMap[jobName] = true
	}

	prefix := workspaceID + ":"
	toDelete := make([]string, 0)

	for key := range sm.state.Jobs {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			jobName := key[len(prefix):]
			if !activeJobsMap[jobName] {
				toDelete = append(toDelete, key)
			}
		}
	}

	for _, key := range toDelete {
		delete(sm.state.Jobs, key)
	}
}

// GetNextRunTime calculates the next run time for a job based on its schedule
func (sm *StateManager) GetNextRunTime(job *Job) (*time.Time, error) {
	schedules, err := job.GetSchedules()
	if err != nil {
		return nil, err
	}

	if len(schedules) == 0 {
		return nil, nil // No schedule defined
	}

	// For simplicity, use the first schedule to calculate next run
	// In a full implementation, you might want to find the earliest next run across all schedules
	if len(schedules) > 0 {
		// This is a simplified calculation - you would want to use the existing CRON parsing logic
		// For now, just return a time 1 hour from now as a placeholder
		nextRun := time.Now().Add(1 * time.Hour)
		return &nextRun, nil
	}

	return nil, nil
}

// SetJobNextRun sets the next scheduled run time for a job
func (sm *StateManager) SetJobNextRun(workspaceID, jobName string, nextRun *time.Time) {
	jobState := sm.GetJobState(workspaceID, jobName)
	jobState.NextRun = nextRun
	sm.SetJobState(workspaceID, jobName, jobState)
}

// GetLastUpdateTime returns the last update time of the state
func (sm *StateManager) GetLastUpdateTime() time.Time {
	if sm.state == nil {
		return time.Now()
	}
	return sm.state.LastUpdated
}