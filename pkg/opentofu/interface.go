package opentofu

import "provisioner/pkg/workspace"

// TofuClient defines the interface for OpenTofu operations
type TofuClient interface {
	Deploy(ws *workspace.Workspace) error
	DestroyWorkspace(ws *workspace.Workspace) error
}

// Ensure Client implements TofuClient interface
var _ TofuClient = (*Client)(nil)
