package opentofu

import (
	"fmt"
	"testing"

	"provisioner/pkg/workspace"
)

func TestMockTofuClientDeployInMode(t *testing.T) {
	// Test DeployInMode method on mock client
	mock := NewMockTofuClient()

	// Create test workspace
	testWorkspace := &workspace.Workspace{
		Name: "test-workspace",
		Config: workspace.Config{
			Template: "web-app",
			ModeSchedules: map[string]interface{}{
				"hibernation": "0 23 * * 1-5",
				"busy":        "0 8 * * 1-5",
			},
		},
		Path: "/tmp/test-workspace",
	}

	// Test successful deploy in mode
	err := mock.DeployInMode(testWorkspace, "busy")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify call was tracked
	if mock.DeployInModeCallCount != 1 {
		t.Errorf("expected DeployInModeCallCount = 1, got %d", mock.DeployInModeCallCount)
	}

	if len(mock.DeployInModeCallWorkspaces) != 1 {
		t.Errorf("expected 1 workspace in DeployInModeCallWorkspaces, got %d", len(mock.DeployInModeCallWorkspaces))
	}

	if mock.DeployInModeCallWorkspaces[0].Name != "test-workspace" {
		t.Errorf("expected workspace name 'test-workspace', got '%s'", mock.DeployInModeCallWorkspaces[0].Name)
	}

	if len(mock.DeployInModeCalls) != 1 {
		t.Errorf("expected 1 mode in DeployInModeCalls, got %d", len(mock.DeployInModeCalls))
	}

	if mock.DeployInModeCalls[0] != "busy" {
		t.Errorf("expected mode 'busy', got '%s'", mock.DeployInModeCalls[0])
	}

	// Test multiple calls
	err = mock.DeployInMode(testWorkspace, "hibernation")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if mock.DeployInModeCallCount != 2 {
		t.Errorf("expected DeployInModeCallCount = 2, got %d", mock.DeployInModeCallCount)
	}

	if mock.DeployInModeCalls[1] != "hibernation" {
		t.Errorf("expected second mode 'hibernation', got '%s'", mock.DeployInModeCalls[1])
	}
}

func TestMockTofuClientDeployInModeError(t *testing.T) {
	// Test error handling
	mock := NewMockTofuClient()

	// Configure mock to return error
	expectedError := fmt.Errorf("deployment failed")
	mock.DeployInModeFunc = func(ws *workspace.Workspace, mode string) error {
		return expectedError
	}

	testWorkspace := &workspace.Workspace{
		Name: "test-workspace",
		Config: workspace.Config{
			Template: "web-app",
		},
		Path: "/tmp/test-workspace",
	}

	// Test error is returned
	err := mock.DeployInMode(testWorkspace, "busy")
	if err != expectedError {
		t.Errorf("expected error %v, got %v", expectedError, err)
	}

	// Verify call was still tracked
	if mock.DeployInModeCallCount != 1 {
		t.Errorf("expected DeployInModeCallCount = 1, got %d", mock.DeployInModeCallCount)
	}
}

func TestMockTofuClientReset(t *testing.T) {
	// Test that Reset clears DeployInMode tracking
	mock := NewMockTofuClient()

	testWorkspace := &workspace.Workspace{
		Name: "test-workspace",
		Config: workspace.Config{
			Template: "web-app",
		},
		Path: "/tmp/test-workspace",
	}

	// Make some calls
	_ = mock.Deploy(testWorkspace)
	_ = mock.DeployInMode(testWorkspace, "busy")
	_ = mock.DestroyWorkspace(testWorkspace)

	// Verify calls were tracked
	if mock.DeployCallCount != 1 {
		t.Errorf("expected DeployCallCount = 1, got %d", mock.DeployCallCount)
	}
	if mock.DeployInModeCallCount != 1 {
		t.Errorf("expected DeployInModeCallCount = 1, got %d", mock.DeployInModeCallCount)
	}
	if mock.DestroyCallCount != 1 {
		t.Errorf("expected DestroyCallCount = 1, got %d", mock.DestroyCallCount)
	}

	// Reset
	mock.Reset()

	// Verify all counts are cleared
	if mock.DeployCallCount != 0 {
		t.Errorf("expected DeployCallCount = 0 after Reset, got %d", mock.DeployCallCount)
	}
	if mock.DeployInModeCallCount != 0 {
		t.Errorf("expected DeployInModeCallCount = 0 after Reset, got %d", mock.DeployInModeCallCount)
	}
	if mock.DestroyCallCount != 0 {
		t.Errorf("expected DestroyCallCount = 0 after Reset, got %d", mock.DestroyCallCount)
	}

	// Verify slices are cleared
	if len(mock.DeployCallWorkspaces) != 0 {
		t.Errorf("expected empty DeployCallWorkspaces after Reset, got %d", len(mock.DeployCallWorkspaces))
	}
	if len(mock.DeployInModeCallWorkspaces) != 0 {
		t.Errorf("expected empty DeployInModeCallWorkspaces after Reset, got %d", len(mock.DeployInModeCallWorkspaces))
	}
	if len(mock.DeployInModeCalls) != 0 {
		t.Errorf("expected empty DeployInModeCalls after Reset, got %d", len(mock.DeployInModeCalls))
	}
	if len(mock.DestroyCallWorkspaces) != 0 {
		t.Errorf("expected empty DestroyCallWorkspaces after Reset, got %d", len(mock.DestroyCallWorkspaces))
	}
}

func TestTofuClientInterface(t *testing.T) {
	// Test that both Client and MockTofuClient implement TofuClient interface
	var client TofuClient
	var mock TofuClient

	// This should compile without error
	client = &Client{}
	mock = &MockTofuClient{}

	// Test that interface methods exist
	testWorkspace := &workspace.Workspace{
		Name: "test",
		Config: workspace.Config{
			Template: "web-app",
		},
		Path: "/tmp/test",
	}

	// These calls should compile (though they may fail at runtime for Client)
	_ = client.Deploy(testWorkspace)
	_ = client.DeployInMode(testWorkspace, "busy")
	_ = client.DestroyWorkspace(testWorkspace)

	_ = mock.Deploy(testWorkspace)
	_ = mock.DeployInMode(testWorkspace, "busy")
	_ = mock.DestroyWorkspace(testWorkspace)
}
