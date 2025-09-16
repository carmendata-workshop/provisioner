package opentofu

// MockTofuClient is a mock implementation of TofuClient for testing
type MockTofuClient struct {
	DeployFunc       func(environmentPath string) error
	DestroyFunc      func(environmentPath string) error
	DeployCallCount  int
	DestroyCallCount int
	DeployCallPaths  []string
	DestroyCallPaths []string
}

// NewMockTofuClient creates a new mock client with default success behavior
func NewMockTofuClient() *MockTofuClient {
	return &MockTofuClient{
		DeployCallPaths:  make([]string, 0),
		DestroyCallPaths: make([]string, 0),
	}
}

// Deploy mocks the deploy operation
func (m *MockTofuClient) Deploy(environmentPath string) error {
	m.DeployCallCount++
	m.DeployCallPaths = append(m.DeployCallPaths, environmentPath)

	if m.DeployFunc != nil {
		return m.DeployFunc(environmentPath)
	}

	// Default success behavior
	return nil
}

// DestroyEnvironment mocks the destroy operation
func (m *MockTofuClient) DestroyEnvironment(environmentPath string) error {
	m.DestroyCallCount++
	m.DestroyCallPaths = append(m.DestroyCallPaths, environmentPath)

	if m.DestroyFunc != nil {
		return m.DestroyFunc(environmentPath)
	}

	// Default success behavior
	return nil
}

// Reset clears all call counts and paths
func (m *MockTofuClient) Reset() {
	m.DeployCallCount = 0
	m.DestroyCallCount = 0
	m.DeployCallPaths = m.DeployCallPaths[:0]
	m.DestroyCallPaths = m.DestroyCallPaths[:0]
}

// SetDeployError configures the mock to return an error on deploy
func (m *MockTofuClient) SetDeployError(err error) {
	m.DeployFunc = func(string) error {
		return err
	}
}

// SetDestroyError configures the mock to return an error on destroy
func (m *MockTofuClient) SetDestroyError(err error) {
	m.DestroyFunc = func(string) error {
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

// GetLastDeployPath returns the path from the most recent deploy call
func (m *MockTofuClient) GetLastDeployPath() string {
	if len(m.DeployCallPaths) == 0 {
		return ""
	}
	return m.DeployCallPaths[len(m.DeployCallPaths)-1]
}

// GetLastDestroyPath returns the path from the most recent destroy call
func (m *MockTofuClient) GetLastDestroyPath() string {
	if len(m.DestroyCallPaths) == 0 {
		return ""
	}
	return m.DestroyCallPaths[len(m.DestroyCallPaths)-1]
}

// Ensure MockTofuClient implements TofuClient interface
var _ TofuClient = (*MockTofuClient)(nil)
