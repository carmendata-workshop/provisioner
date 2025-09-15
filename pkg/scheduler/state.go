package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type EnvironmentStatus string

const (
	StatusDeployed   EnvironmentStatus = "deployed"
	StatusDestroyed  EnvironmentStatus = "destroyed"
	StatusPending    EnvironmentStatus = "pending"
	StatusDeploying  EnvironmentStatus = "deploying"
	StatusDestroying EnvironmentStatus = "destroying"
)

type EnvironmentState struct {
	Name             string            `json:"name"`
	Status           EnvironmentStatus `json:"status"`
	LastDeployed     *time.Time        `json:"last_deployed,omitempty"`
	LastDestroyed    *time.Time        `json:"last_destroyed,omitempty"`
	LastDeployError  string            `json:"last_deploy_error,omitempty"`
	LastDestroyError string            `json:"last_destroy_error,omitempty"`
}

type State struct {
	Environments map[string]*EnvironmentState `json:"environments"`
	LastUpdated  time.Time                    `json:"last_updated"`
}

func NewState() *State {
	return &State{
		Environments: make(map[string]*EnvironmentState),
		LastUpdated:  time.Now(),
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

	if state.Environments == nil {
		state.Environments = make(map[string]*EnvironmentState)
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

func (s *State) GetEnvironmentState(name string) *EnvironmentState {
	if env, exists := s.Environments[name]; exists {
		return env
	}

	// Create new environment state
	env := &EnvironmentState{
		Name:   name,
		Status: StatusDestroyed,
	}
	s.Environments[name] = env
	return env
}

func (s *State) SetEnvironmentStatus(name string, status EnvironmentStatus) {
	env := s.GetEnvironmentState(name)
	env.Status = status

	now := time.Now()
	switch status {
	case StatusDeployed:
		env.LastDeployed = &now
		env.LastDeployError = ""
	case StatusDestroyed:
		env.LastDestroyed = &now
		env.LastDestroyError = ""
	}
}

func (s *State) SetEnvironmentError(name string, isDeployError bool, errorMsg string) {
	env := s.GetEnvironmentState(name)

	if isDeployError {
		env.LastDeployError = errorMsg
		env.Status = StatusDestroyed
	} else {
		env.LastDestroyError = errorMsg
		env.Status = StatusDeployed
	}
}