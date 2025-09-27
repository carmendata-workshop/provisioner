package opentofu

import "provisioner/pkg/workspace"

// TofuClient defines the interface for OpenTofu operations
type TofuClient interface {
	// High-level workspace operations
	Deploy(ws *workspace.Workspace) error
	DeployInMode(ws *workspace.Workspace, mode string) error
	DestroyWorkspace(ws *workspace.Workspace) error

	// Low-level operations for job execution
	Init(workingDir string) error
	Plan(workingDir string) error
	Apply(workingDir string) error
	Destroy(workingDir string) error
	PlanWithMode(workingDir, mode string) error
	ApplyWithMode(workingDir, mode string) error
}

// Ensure Client implements TofuClient interface
var _ TofuClient = (*Client)(nil)
