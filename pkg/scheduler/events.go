package scheduler

import (
	"time"
)

// DeploymentEventType represents the type of deployment event
type DeploymentEventType string

const (
	// EventDeploymentCompleted is triggered when a workspace deployment succeeds
	EventDeploymentCompleted DeploymentEventType = "deployment-completed"

	// EventDeploymentFailed is triggered when a workspace deployment fails
	EventDeploymentFailed DeploymentEventType = "deployment-failed"

	// EventDestroyCompleted is triggered when a workspace destruction succeeds
	EventDestroyCompleted DeploymentEventType = "destroy-completed"

	// EventDestroyFailed is triggered when a workspace destruction fails
	EventDestroyFailed DeploymentEventType = "destroy-failed"

	// EventReboot is triggered when the system starts up
	EventReboot DeploymentEventType = "reboot"
)

// DeploymentEvent represents an event that can trigger jobs
type DeploymentEvent struct {
	// Type of the event
	Type DeploymentEventType `json:"type"`

	// WorkspaceID that triggered the event
	WorkspaceID string `json:"workspace_id"`

	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Mode for mode-based deployments (optional)
	Mode string `json:"mode,omitempty"`

	// Error message for failed events (optional)
	Error string `json:"error,omitempty"`
}

// Interface methods to work with job package
func (e *DeploymentEvent) GetType() string {
	return string(e.Type)
}

func (e *DeploymentEvent) GetWorkspaceID() string {
	return e.WorkspaceID
}

func (e *DeploymentEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

func (e *DeploymentEvent) GetMode() string {
	return e.Mode
}

func (e *DeploymentEvent) GetError() string {
	return e.Error
}

// MatchesSchedule checks if this event matches a special schedule
func (e *DeploymentEvent) MatchesSchedule(schedule string) bool {
	switch schedule {
	case "@deployment":
		return e.Type == EventDeploymentCompleted
	case "@deployment-failed":
		return e.Type == EventDeploymentFailed
	case "@destroy":
		return e.Type == EventDestroyCompleted
	case "@destroy-failed":
		return e.Type == EventDestroyFailed
	case "@reboot":
		return e.Type == EventReboot
	default:
		return false
	}
}

// NewDeploymentEvent creates a new deployment event
func NewDeploymentEvent(eventType DeploymentEventType, workspaceID string) *DeploymentEvent {
	return &DeploymentEvent{
		Type:        eventType,
		WorkspaceID: workspaceID,
		Timestamp:   time.Now(),
	}
}

// NewDeploymentEventWithMode creates a new deployment event with mode information
func NewDeploymentEventWithMode(eventType DeploymentEventType, workspaceID, mode string) *DeploymentEvent {
	return &DeploymentEvent{
		Type:        eventType,
		WorkspaceID: workspaceID,
		Timestamp:   time.Now(),
		Mode:        mode,
	}
}

// NewDeploymentEventWithError creates a new deployment event with error information
func NewDeploymentEventWithError(eventType DeploymentEventType, workspaceID, errorMsg string) *DeploymentEvent {
	return &DeploymentEvent{
		Type:        eventType,
		WorkspaceID: workspaceID,
		Timestamp:   time.Now(),
		Error:       errorMsg,
	}
}