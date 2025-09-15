package opentofu

// TofuClient defines the interface for OpenTofu operations
type TofuClient interface {
	Deploy(environmentPath string) error
	DestroyEnvironment(environmentPath string) error
}

// Ensure Client implements TofuClient interface
var _ TofuClient = (*Client)(nil)