package job

import "time"

// DeploymentEvent interface to avoid circular imports with scheduler package
type DeploymentEvent interface {
	GetType() string
	GetWorkspaceID() string
	GetTimestamp() time.Time
	GetMode() string
	GetError() string
	MatchesSchedule(schedule string) bool
}

// SimpleDeploymentEvent is a basic implementation of DeploymentEvent
type SimpleDeploymentEvent struct {
	Type        string    `json:"type"`
	WorkspaceID string    `json:"workspace_id"`
	Timestamp   time.Time `json:"timestamp"`
	Mode        string    `json:"mode,omitempty"`
	Error       string    `json:"error,omitempty"`
}

func (e *SimpleDeploymentEvent) GetType() string {
	return e.Type
}

func (e *SimpleDeploymentEvent) GetWorkspaceID() string {
	return e.WorkspaceID
}

func (e *SimpleDeploymentEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

func (e *SimpleDeploymentEvent) GetMode() string {
	return e.Mode
}

func (e *SimpleDeploymentEvent) GetError() string {
	return e.Error
}

func (e *SimpleDeploymentEvent) MatchesSchedule(schedule string) bool {
	switch schedule {
	case "@deployment":
		return e.Type == "deployment-completed"
	case "@deployment-failed":
		return e.Type == "deployment-failed"
	case "@destroy":
		return e.Type == "destroy-completed"
	case "@destroy-failed":
		return e.Type == "destroy-failed"
	case "@reboot":
		return e.Type == "reboot"
	default:
		return false
	}
}

// NewSimpleDeploymentEvent creates a new simple deployment event
func NewSimpleDeploymentEvent(eventType, workspaceID string) *SimpleDeploymentEvent {
	return &SimpleDeploymentEvent{
		Type:        eventType,
		WorkspaceID: workspaceID,
		Timestamp:   time.Now(),
	}
}
