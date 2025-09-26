package opentofu

import "provisioner/pkg/workspace"

// MockTofuClient is a mock implementation of TofuClient for testing
type MockTofuClient struct {
	DeployFunc       func(ws *workspace.Workspace) error
	DestroyFunc      func(ws *workspace.Workspace) error
	DeployCallCount  int
	DestroyCallCount int
	DeployCallWorkspaces   []*workspace.Workspace
	DestroyCallWorkspaces  []*workspace.Workspace
}

// NewMockTofuClient creates a new mock client with default success behavior
func NewMockTofuClient() *MockTofuClient {
	return &MockTofuClient{
		DeployCallWorkspaces:  make([]*workspace.Workspace, 0),
		DestroyCallWorkspaces: make([]*workspace.Workspace, 0),
	}
}

// Deploy mocks the deploy operation
func (m *MockTofuClient) Deploy(ws *workspace.Workspace) error {
	m.DeployCallCount++
	m.DeployCallWorkspaces = append(m.DeployCallWorkspaces, ws)

	if m.DeployFunc != nil {
		return m.DeployFunc(ws)
	}

	// Default success behavior
	return nil
}

// DestroyWorkspace mocks the destroy operation
func (m *MockTofuClient) DestroyWorkspace(ws *workspace.Workspace) error {
	m.DestroyCallCount++
	m.DestroyCallWorkspaces = append(m.DestroyCallWorkspaces, ws)

	if m.DestroyFunc != nil {
		return m.DestroyFunc(ws)
	}

	// Default success behavior
	return nil
}

// Reset clears all call counts and workspaces
func (m *MockTofuClient) Reset() {
	m.DeployCallCount = 0
	m.DestroyCallCount = 0
	m.DeployCallWorkspaces = m.DeployCallWorkspaces[:0]
	m.DestroyCallWorkspaces = m.DestroyCallWorkspaces[:0]
}

// SetDeployError configures the mock to return an error on deploy
func (m *MockTofuClient) SetDeployError(err error) {
	m.DeployFunc = func(*workspace.Workspace) error {
		return err
	}
}

// SetDestroyError configures the mock to return an error on destroy
func (m *MockTofuClient) SetDestroyError(err error) {
	m.DestroyFunc = func(*workspace.Workspace) error {
		return err
	}
}

// SetDeploySuccess configures the mock to succeed on deploy
func (m *MockTofuClient) SetDeploySuccess() {
	m.DeployFunc = nil
}

// SetDestroySuccess configures the mock to succeed on destroy
func (m *MockTofuClient) SetDestroySuccess() {
	m.DestroyFunc = nil
}

// GetLastDeployWorkspace returns the workspace from the most recent deploy call
func (m *MockTofuClient) GetLastDeployWorkspace() *workspace.Workspace {
	if len(m.DeployCallWorkspaces) == 0 {
		return nil
	}
	return m.DeployCallWorkspaces[len(m.DeployCallWorkspaces)-1]
}

// GetLastDestroyWorkspace returns the workspace from the most recent destroy call
func (m *MockTofuClient) GetLastDestroyWorkspace() *workspace.Workspace {
	if len(m.DestroyCallWorkspaces) == 0 {
		return nil
	}
	return m.DestroyCallWorkspaces[len(m.DestroyCallWorkspaces)-1]
}

// Ensure MockTofuClient implements TofuClient interface
var _ TofuClient = (*MockTofuClient)(nil)
