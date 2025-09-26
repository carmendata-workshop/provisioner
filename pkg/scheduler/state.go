package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type WorkspaceStatus string

const (
	StatusDeployed      WorkspaceStatus = "deployed"
	StatusDestroyed     WorkspaceStatus = "destroyed"
	StatusPending       WorkspaceStatus = "pending"
	StatusDeploying     WorkspaceStatus = "deploying"
	StatusDestroying    WorkspaceStatus = "destroying"
	StatusDeployFailed  WorkspaceStatus = "deploy_failed"
	StatusDestroyFailed WorkspaceStatus = "destroy_failed"
)

type WorkspaceState struct {
	Name               string          `json:"name"`
	Status             WorkspaceStatus `json:"status"`
	LastDeployed       *time.Time      `json:"last_deployed,omitempty"`
	LastDestroyed      *time.Time      `json:"last_destroyed,omitempty"`
	LastDeployError    string          `json:"last_deploy_error,omitempty"`
	LastDestroyError   string          `json:"last_destroy_error,omitempty"`
	LastConfigModified *time.Time      `json:"last_config_modified,omitempty"`
	DeploymentMode     string          `json:"deployment_mode,omitempty"`
}

type State struct {
	Workspaces  map[string]*WorkspaceState `json:"workspaces"`
	LastUpdated time.Time                  `json:"last_updated"`
}

func NewState() *State {
	return &State{
		Workspaces:  make(map[string]*WorkspaceState),
		LastUpdated: time.Now(),
	}
}

func LoadState(statePath string) (*State, error) {
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return NewState(), nil
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	if state.Workspaces == nil {
		state.Workspaces = make(map[string]*WorkspaceState)
	}

	return &state, nil
}

func (s *State) SaveState(statePath string) error {
	s.LastUpdated = time.Now()

	// Ensure state directory exists
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

func (s *State) GetWorkspaceState(name string) *WorkspaceState {
	if workspace, exists := s.Workspaces[name]; exists {
		return workspace
	}

	// Create new workspace state
	workspace := &WorkspaceState{
		Name:   name,
		Status: StatusDestroyed,
	}
	s.Workspaces[name] = workspace
	return workspace
}

func (s *State) SetWorkspaceStatus(name string, status WorkspaceStatus) {
	workspace := s.GetWorkspaceState(name)
	workspace.Status = status

	now := time.Now()
	switch status {
	case StatusDeployed:
		workspace.LastDeployed = &now
		workspace.LastDeployError = ""
	case StatusDestroyed:
		workspace.LastDestroyed = &now
		workspace.LastDestroyError = ""
	}
}

func (s *State) SetWorkspaceError(name string, isDeployError bool, errorMsg string) {
	workspace := s.GetWorkspaceState(name)

	if isDeployError {
		workspace.LastDeployError = errorMsg
		workspace.Status = StatusDeployFailed
	} else {
		workspace.LastDestroyError = errorMsg
		workspace.Status = StatusDestroyFailed
	}
}

// SetWorkspaceConfigModified updates the last config modification time for an workspace
func (s *State) SetWorkspaceConfigModified(name string, modTime time.Time) {
	workspace := s.GetWorkspaceState(name)
	workspace.LastConfigModified = &modTime

	// Handle state transitions based on current status when config is modified
	switch workspace.Status {
	case StatusDeployFailed:
		// If workspace was in deploy failed state, allow retries
		workspace.Status = StatusDestroyed
		workspace.LastDeployError = ""
	case StatusDestroyFailed:
		// If workspace was in destroy failed state, allow retries
		workspace.Status = StatusDeployed
		workspace.LastDestroyError = ""
	case StatusDeployed:
		// If workspace is deployed and config was modified, trigger redeployment
		workspace.Status = StatusDestroyed
		// Clear deployment timestamp to ensure redeployment
		workspace.LastDeployed = nil
	}
}

// SetWorkspaceState updates the entire workspace state
func (s *State) SetWorkspaceState(name string, workspaceState *WorkspaceState) {
	s.Workspaces[name] = workspaceState
}
