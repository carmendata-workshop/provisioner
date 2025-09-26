package scheduler

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"provisioner/pkg/opentofu"
)

func TestManualDeploy(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create workspace configuration
	workspaceName := "test-workspace"
	workspaceDir := filepath.Join(tempDir, "workspaces", workspaceName)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create config.json
	configContent := `{
		"enabled": true,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create mock client
	mockClient := &opentofu.MockTofuClient{}

	// Create scheduler with mock client
	sched := NewWithClient(mockClient)
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("Failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Test manual deploy
	err := sched.ManualDeploy(workspaceName)
	if err != nil {
		t.Fatalf("Manual deploy failed: %v", err)
	}

	// Verify deployment was called
	if mockClient.DeployCallCount != 1 {
		t.Errorf("Expected Deploy to be called once, got %d calls", mockClient.DeployCallCount)
	}
	if len(mockClient.DeployCallWorkspaces) == 0 || mockClient.DeployCallWorkspaces[0].Name != workspaceName {
		t.Errorf("Deploy was not called with correct workspace. Expected %s, got %v", workspaceName, mockClient.DeployCallWorkspaces)
	}

	// Verify state was updated
	workspaceState := sched.state.GetWorkspaceState(workspaceName)
	if workspaceState.Status != StatusDeployed {
		t.Errorf("Expected status %s, got %s", StatusDeployed, workspaceState.Status)
	}
	if workspaceState.LastDeployed == nil {
		t.Error("LastDeployed should not be nil after successful deployment")
	}
}

func TestManualDestroy(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create workspace configuration
	workspaceName := "test-workspace"
	workspaceDir := filepath.Join(tempDir, "workspaces", workspaceName)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create config.json
	configContent := `{
		"enabled": true,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create mock client
	mockClient := &opentofu.MockTofuClient{}

	// Create scheduler with mock client
	sched := NewWithClient(mockClient)
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("Failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Set initial state to deployed
	sched.state.SetWorkspaceStatus(workspaceName, StatusDeployed)

	// Test manual destroy
	err := sched.ManualDestroy(workspaceName)
	if err != nil {
		t.Fatalf("Manual destroy failed: %v", err)
	}

	// Verify destruction was called
	if mockClient.DestroyCallCount != 1 {
		t.Errorf("Expected DestroyWorkspace to be called once, got %d calls", mockClient.DestroyCallCount)
	}
	if len(mockClient.DestroyCallWorkspaces) == 0 || mockClient.DestroyCallWorkspaces[0].Name != workspaceName {
		t.Errorf("DestroyWorkspace was not called with correct workspace. Expected %s, got %v", workspaceName, mockClient.DestroyCallWorkspaces)
	}

	// Verify state was updated
	workspaceState := sched.state.GetWorkspaceState(workspaceName)
	if workspaceState.Status != StatusDestroyed {
		t.Errorf("Expected status %s, got %s", StatusDestroyed, workspaceState.Status)
	}
	if workspaceState.LastDestroyed == nil {
		t.Error("LastDestroyed should not be nil after successful destruction")
	}
}

func TestManualDeployNonExistentWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create empty workspaces directory
	if err := os.MkdirAll(filepath.Join(tempDir, "workspaces"), 0755); err != nil {
		t.Fatalf("Failed to create workspaces directory: %v", err)
	}

	// Create scheduler with no workspaces
	sched := NewWithClient(&opentofu.MockTofuClient{})
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load empty workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("Failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Test manual deploy of non-existent workspace
	err := sched.ManualDeploy("nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent workspace, got nil")
	}
	if err.Error() != "workspace 'nonexistent' not found in configuration" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestManualDeployDisabledWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create workspace configuration (disabled)
	workspaceName := "disabled-workspace"
	workspaceDir := filepath.Join(tempDir, "workspaces", workspaceName)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create config.json with enabled: false
	configContent := `{
		"enabled": false,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create scheduler
	sched := NewWithClient(&opentofu.MockTofuClient{})
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("Failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Test manual deploy of disabled workspace
	err := sched.ManualDeploy(workspaceName)
	if err == nil {
		t.Fatal("Expected error for disabled workspace, got nil")
	}
	if err.Error() != "workspace 'disabled-workspace' is disabled in configuration" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestManualDeployBusyWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create workspace configuration
	workspaceName := "busy-workspace"
	workspaceDir := filepath.Join(tempDir, "workspaces", workspaceName)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create config.json
	configContent := `{
		"enabled": true,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create scheduler
	sched := NewWithClient(&opentofu.MockTofuClient{})
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("Failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Set workspace as currently deploying
	sched.state.SetWorkspaceStatus(workspaceName, StatusDeploying)

	// Test manual deploy of busy workspace
	err := sched.ManualDeploy(workspaceName)
	if err == nil {
		t.Fatal("Expected error for busy workspace, got nil")
	}
	if err.Error() != "workspace 'busy-workspace' is currently deploying, cannot deploy" {
		t.Errorf("Unexpected error message: %v", err)
	}

	// Test with destroying status
	sched.state.SetWorkspaceStatus(workspaceName, StatusDestroying)
	err = sched.ManualDestroy(workspaceName)
	if err == nil {
		t.Fatal("Expected error for busy workspace, got nil")
	}
	if err.Error() != "workspace 'busy-workspace' is currently destroying, cannot destroy" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestManualDeployWithError(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create workspace configuration
	workspaceName := "error-workspace"
	workspaceDir := filepath.Join(tempDir, "workspaces", workspaceName)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create config.json
	configContent := `{
		"enabled": true,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create mock client that will return an error
	mockClient := &opentofu.MockTofuClient{}
	mockClient.SetDeployError(errors.New("deployment failed"))

	// Create scheduler with mock client
	sched := NewWithClient(mockClient)
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("Failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Test manual deploy (should succeed even though deployment fails)
	err := sched.ManualDeploy(workspaceName)
	if err != nil {
		t.Fatalf("Manual deploy should not return error even when deployment fails: %v", err)
	}

	// Verify state reflects the error
	workspaceState := sched.state.GetWorkspaceState(workspaceName)
	if workspaceState.Status != StatusDeployFailed {
		t.Errorf("Expected status %s, got %s", StatusDeployFailed, workspaceState.Status)
	}
	if workspaceState.LastDeployError == "" {
		t.Error("LastDeployError should not be empty after failed deployment")
	}
}

func TestManualOperationsWithFailedStates(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create workspace configuration
	workspaceName := "failed-workspace"
	workspaceDir := filepath.Join(tempDir, "workspaces", workspaceName)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create config.json
	configContent := `{
		"enabled": true,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create mock client
	mockClient := &opentofu.MockTofuClient{}

	// Create scheduler
	sched := NewWithClient(mockClient)
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("Failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Test manual deploy when in deploy failed state
	sched.state.SetWorkspaceError(workspaceName, true, "Previous deploy failed")
	err := sched.ManualDeploy(workspaceName)
	if err != nil {
		t.Fatalf("Manual deploy should work even when in deploy failed state: %v", err)
	}

	// Test manual destroy when in destroy failed state
	sched.state.SetWorkspaceError(workspaceName, false, "Previous destroy failed")
	err = sched.ManualDestroy(workspaceName)
	if err != nil {
		t.Fatalf("Manual destroy should work even when in destroy failed state: %v", err)
	}
}
