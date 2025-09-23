package opentofu

import "provisioner/pkg/environment"

// TofuClient defines the interface for OpenTofu operations
type TofuClient interface {
	Deploy(env *environment.Environment) error
	DestroyEnvironment(env *environment.Environment) error
}

// Ensure Client implements TofuClient interface
var _ TofuClient = (*Client)(nil)
