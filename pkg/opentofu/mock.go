package opentofu

import "provisioner/pkg/environment"

// MockTofuClient is a mock implementation of TofuClient for testing
type MockTofuClient struct {
	DeployFunc       func(env *environment.Environment) error
	DestroyFunc      func(env *environment.Environment) error
	DeployCallCount  int
	DestroyCallCount int
	DeployCallEnvs   []*environment.Environment
	DestroyCallEnvs  []*environment.Environment
}

// NewMockTofuClient creates a new mock client with default success behavior
func NewMockTofuClient() *MockTofuClient {
	return &MockTofuClient{
		DeployCallEnvs:  make([]*environment.Environment, 0),
		DestroyCallEnvs: make([]*environment.Environment, 0),
	}
}

// Deploy mocks the deploy operation
func (m *MockTofuClient) Deploy(env *environment.Environment) error {
	m.DeployCallCount++
	m.DeployCallEnvs = append(m.DeployCallEnvs, env)

	if m.DeployFunc != nil {
		return m.DeployFunc(env)
	}

	// Default success behavior
	return nil
}

// DestroyEnvironment mocks the destroy operation
func (m *MockTofuClient) DestroyEnvironment(env *environment.Environment) error {
	m.DestroyCallCount++
	m.DestroyCallEnvs = append(m.DestroyCallEnvs, env)

	if m.DestroyFunc != nil {
		return m.DestroyFunc(env)
	}

	// Default success behavior
	return nil
}

// Reset clears all call counts and environments
func (m *MockTofuClient) Reset() {
	m.DeployCallCount = 0
	m.DestroyCallCount = 0
	m.DeployCallEnvs = m.DeployCallEnvs[:0]
	m.DestroyCallEnvs = m.DestroyCallEnvs[:0]
}

// SetDeployError configures the mock to return an error on deploy
func (m *MockTofuClient) SetDeployError(err error) {
	m.DeployFunc = func(*environment.Environment) error {
		return err
	}
}

// SetDestroyError configures the mock to return an error on destroy
func (m *MockTofuClient) SetDestroyError(err error) {
	m.DestroyFunc = func(*environment.Environment) error {
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

// GetLastDeployEnv returns the environment from the most recent deploy call
func (m *MockTofuClient) GetLastDeployEnv() *environment.Environment {
	if len(m.DeployCallEnvs) == 0 {
		return nil
	}
	return m.DeployCallEnvs[len(m.DeployCallEnvs)-1]
}

// GetLastDestroyEnv returns the environment from the most recent destroy call
func (m *MockTofuClient) GetLastDestroyEnv() *environment.Environment {
	if len(m.DestroyCallEnvs) == 0 {
		return nil
	}
	return m.DestroyCallEnvs[len(m.DestroyCallEnvs)-1]
}

// Ensure MockTofuClient implements TofuClient interface
var _ TofuClient = (*MockTofuClient)(nil)
