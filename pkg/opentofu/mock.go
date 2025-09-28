package opentofu

import "provisioner/pkg/workspace"

// MockTofuClient is a mock implementation of TofuClient for testing
type MockTofuClient struct {
	// High-level operations
	DeployFunc       func(ws *workspace.Workspace) error
	DeployInModeFunc func(ws *workspace.Workspace, mode string) error
	DestroyFunc      func(ws *workspace.Workspace) error

	// Low-level operations
	InitFunc          func(workingDir string) error
	PlanFunc          func(workingDir string) error
	ApplyFunc         func(workingDir string) error
	DestroyDirFunc    func(workingDir string) error
	PlanWithModeFunc  func(workingDir, mode string) error
	ApplyWithModeFunc func(workingDir, mode string) error

	// Call tracking
	DeployCallCount       int
	DeployInModeCallCount int
	DestroyCallCount      int
	InitCallCount         int
	PlanCallCount         int
	ApplyCallCount        int
	DestroyDirCallCount   int

	DeployCallWorkspaces       []*workspace.Workspace
	DeployInModeCallWorkspaces []*workspace.Workspace
	DeployInModeCalls          []string // Track mode parameters
	DestroyCallWorkspaces      []*workspace.Workspace
	InitCallDirs               []string
	PlanCallDirs               []string
	ApplyCallDirs              []string
	DestroyDirCallDirs         []string
}

// NewMockTofuClient creates a new mock client with default success behavior
func NewMockTofuClient() *MockTofuClient {
	return &MockTofuClient{
		DeployCallWorkspaces:       make([]*workspace.Workspace, 0),
		DeployInModeCallWorkspaces: make([]*workspace.Workspace, 0),
		DeployInModeCalls:          make([]string, 0),
		DestroyCallWorkspaces:      make([]*workspace.Workspace, 0),
		InitCallDirs:               make([]string, 0),
		PlanCallDirs:               make([]string, 0),
		ApplyCallDirs:              make([]string, 0),
		DestroyDirCallDirs:         make([]string, 0),
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

// DeployInMode mocks the deploy in mode operation
func (m *MockTofuClient) DeployInMode(ws *workspace.Workspace, mode string) error {
	m.DeployInModeCallCount++
	m.DeployInModeCallWorkspaces = append(m.DeployInModeCallWorkspaces, ws)
	m.DeployInModeCalls = append(m.DeployInModeCalls, mode)

	if m.DeployInModeFunc != nil {
		return m.DeployInModeFunc(ws, mode)
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
	m.DeployInModeCallCount = 0
	m.DestroyCallCount = 0
	m.InitCallCount = 0
	m.PlanCallCount = 0
	m.ApplyCallCount = 0
	m.DestroyDirCallCount = 0

	m.DeployCallWorkspaces = m.DeployCallWorkspaces[:0]
	m.DeployInModeCallWorkspaces = m.DeployInModeCallWorkspaces[:0]
	m.DeployInModeCalls = m.DeployInModeCalls[:0]
	m.DestroyCallWorkspaces = m.DestroyCallWorkspaces[:0]
	m.InitCallDirs = m.InitCallDirs[:0]
	m.PlanCallDirs = m.PlanCallDirs[:0]
	m.ApplyCallDirs = m.ApplyCallDirs[:0]
	m.DestroyDirCallDirs = m.DestroyDirCallDirs[:0]
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

// Low-level operation methods

// Init mocks the init operation
func (m *MockTofuClient) Init(workingDir string) error {
	m.InitCallCount++
	m.InitCallDirs = append(m.InitCallDirs, workingDir)

	if m.InitFunc != nil {
		return m.InitFunc(workingDir)
	}
	return nil
}

// Plan mocks the plan operation
func (m *MockTofuClient) Plan(workingDir string) error {
	m.PlanCallCount++
	m.PlanCallDirs = append(m.PlanCallDirs, workingDir)

	if m.PlanFunc != nil {
		return m.PlanFunc(workingDir)
	}
	return nil
}

// Apply mocks the apply operation
func (m *MockTofuClient) Apply(workingDir string) error {
	m.ApplyCallCount++
	m.ApplyCallDirs = append(m.ApplyCallDirs, workingDir)

	if m.ApplyFunc != nil {
		return m.ApplyFunc(workingDir)
	}
	return nil
}

// Destroy mocks the destroy operation on a directory
func (m *MockTofuClient) Destroy(workingDir string) error {
	m.DestroyDirCallCount++
	m.DestroyDirCallDirs = append(m.DestroyDirCallDirs, workingDir)

	if m.DestroyDirFunc != nil {
		return m.DestroyDirFunc(workingDir)
	}
	return nil
}

// PlanWithMode mocks the plan operation with mode
func (m *MockTofuClient) PlanWithMode(workingDir, mode string) error {
	m.PlanCallCount++
	m.PlanCallDirs = append(m.PlanCallDirs, workingDir)

	if m.PlanWithModeFunc != nil {
		return m.PlanWithModeFunc(workingDir, mode)
	}
	return nil
}

// ApplyWithMode mocks the apply operation with mode
func (m *MockTofuClient) ApplyWithMode(workingDir, mode string) error {
	m.ApplyCallCount++
	m.ApplyCallDirs = append(m.ApplyCallDirs, workingDir)

	if m.ApplyWithModeFunc != nil {
		return m.ApplyWithModeFunc(workingDir, mode)
	}
	return nil
}

// Ensure MockTofuClient implements TofuClient interface
var _ TofuClient = (*MockTofuClient)(nil)
