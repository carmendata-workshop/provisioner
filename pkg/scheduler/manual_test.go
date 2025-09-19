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

	// Create environment configuration
	envName := "test-env"
	envDir := filepath.Join(tempDir, "environments", envName)
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("Failed to create environment directory: %v", err)
	}

	// Create config.json
	configContent := `{
		"enabled": true,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(envDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(envDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create mock client
	mockClient := &opentofu.MockTofuClient{}

	// Create scheduler with mock client
	sched := NewWithClient(mockClient)
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load environments and state
	if err := sched.LoadEnvironments(); err != nil {
		t.Fatalf("Failed to load environments: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Test manual deploy
	err := sched.ManualDeploy(envName)
	if err != nil {
		t.Fatalf("Manual deploy failed: %v", err)
	}

	// Verify deployment was called
	if mockClient.DeployCallCount != 1 {
		t.Errorf("Expected Deploy to be called once, got %d calls", mockClient.DeployCallCount)
	}
	if len(mockClient.DeployCallPaths) == 0 || mockClient.DeployCallPaths[0] != envDir {
		t.Errorf("Deploy was not called with correct environment path. Expected %s, got %v", envDir, mockClient.DeployCallPaths)
	}

	// Verify state was updated
	envState := sched.state.GetEnvironmentState(envName)
	if envState.Status != StatusDeployed {
		t.Errorf("Expected status %s, got %s", StatusDeployed, envState.Status)
	}
	if envState.LastDeployed == nil {
		t.Error("LastDeployed should not be nil after successful deployment")
	}
}

func TestManualDestroy(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create environment configuration
	envName := "test-env"
	envDir := filepath.Join(tempDir, "environments", envName)
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("Failed to create environment directory: %v", err)
	}

	// Create config.json
	configContent := `{
		"enabled": true,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(envDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(envDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create mock client
	mockClient := &opentofu.MockTofuClient{}

	// Create scheduler with mock client
	sched := NewWithClient(mockClient)
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load environments and state
	if err := sched.LoadEnvironments(); err != nil {
		t.Fatalf("Failed to load environments: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Set initial state to deployed
	sched.state.SetEnvironmentStatus(envName, StatusDeployed)

	// Test manual destroy
	err := sched.ManualDestroy(envName)
	if err != nil {
		t.Fatalf("Manual destroy failed: %v", err)
	}

	// Verify destruction was called
	if mockClient.DestroyCallCount != 1 {
		t.Errorf("Expected DestroyEnvironment to be called once, got %d calls", mockClient.DestroyCallCount)
	}
	if len(mockClient.DestroyCallPaths) == 0 || mockClient.DestroyCallPaths[0] != envDir {
		t.Errorf("DestroyEnvironment was not called with correct environment path. Expected %s, got %v", envDir, mockClient.DestroyCallPaths)
	}

	// Verify state was updated
	envState := sched.state.GetEnvironmentState(envName)
	if envState.Status != StatusDestroyed {
		t.Errorf("Expected status %s, got %s", StatusDestroyed, envState.Status)
	}
	if envState.LastDestroyed == nil {
		t.Error("LastDestroyed should not be nil after successful destruction")
	}
}

func TestManualDeployNonExistentEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create empty environments directory
	if err := os.MkdirAll(filepath.Join(tempDir, "environments"), 0755); err != nil {
		t.Fatalf("Failed to create environments directory: %v", err)
	}

	// Create scheduler with no environments
	sched := NewWithClient(&opentofu.MockTofuClient{})
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load empty environments and state
	if err := sched.LoadEnvironments(); err != nil {
		t.Fatalf("Failed to load environments: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Test manual deploy of non-existent environment
	err := sched.ManualDeploy("nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent environment, got nil")
	}
	if err.Error() != "environment 'nonexistent' not found in configuration" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestManualDeployDisabledEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create environment configuration (disabled)
	envName := "disabled-env"
	envDir := filepath.Join(tempDir, "environments", envName)
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("Failed to create environment directory: %v", err)
	}

	// Create config.json with enabled: false
	configContent := `{
		"enabled": false,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(envDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(envDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create scheduler
	sched := NewWithClient(&opentofu.MockTofuClient{})
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load environments and state
	if err := sched.LoadEnvironments(); err != nil {
		t.Fatalf("Failed to load environments: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Test manual deploy of disabled environment
	err := sched.ManualDeploy(envName)
	if err == nil {
		t.Fatal("Expected error for disabled environment, got nil")
	}
	if err.Error() != "environment 'disabled-env' is disabled in configuration" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestManualDeployBusyEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create environment configuration
	envName := "busy-env"
	envDir := filepath.Join(tempDir, "environments", envName)
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("Failed to create environment directory: %v", err)
	}

	// Create config.json
	configContent := `{
		"enabled": true,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(envDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(envDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create scheduler
	sched := NewWithClient(&opentofu.MockTofuClient{})
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load environments and state
	if err := sched.LoadEnvironments(); err != nil {
		t.Fatalf("Failed to load environments: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Set environment as currently deploying
	sched.state.SetEnvironmentStatus(envName, StatusDeploying)

	// Test manual deploy of busy environment
	err := sched.ManualDeploy(envName)
	if err == nil {
		t.Fatal("Expected error for busy environment, got nil")
	}
	if err.Error() != "environment 'busy-env' is currently deploying, cannot deploy" {
		t.Errorf("Unexpected error message: %v", err)
	}

	// Test with destroying status
	sched.state.SetEnvironmentStatus(envName, StatusDestroying)
	err = sched.ManualDestroy(envName)
	if err == nil {
		t.Fatal("Expected error for busy environment, got nil")
	}
	if err.Error() != "environment 'busy-env' is currently destroying, cannot destroy" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestManualDeployWithError(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create environment configuration
	envName := "error-env"
	envDir := filepath.Join(tempDir, "environments", envName)
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("Failed to create environment directory: %v", err)
	}

	// Create config.json
	configContent := `{
		"enabled": true,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(envDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(envDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create mock client that will return an error
	mockClient := &opentofu.MockTofuClient{}
	mockClient.SetDeployError(errors.New("deployment failed"))

	// Create scheduler with mock client
	sched := NewWithClient(mockClient)
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load environments and state
	if err := sched.LoadEnvironments(); err != nil {
		t.Fatalf("Failed to load environments: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Test manual deploy (should succeed even though deployment fails)
	err := sched.ManualDeploy(envName)
	if err != nil {
		t.Fatalf("Manual deploy should not return error even when deployment fails: %v", err)
	}

	// Verify state reflects the error
	envState := sched.state.GetEnvironmentState(envName)
	if envState.Status != StatusDeployFailed {
		t.Errorf("Expected status %s, got %s", StatusDeployFailed, envState.Status)
	}
	if envState.LastDeployError == "" {
		t.Error("LastDeployError should not be empty after failed deployment")
	}
}

func TestManualOperationsWithFailedStates(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create environment configuration
	envName := "failed-env"
	envDir := filepath.Join(tempDir, "environments", envName)
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("Failed to create environment directory: %v", err)
	}

	// Create config.json
	configContent := `{
		"enabled": true,
		"deploy_schedule": "0 9 * * *",
		"destroy_schedule": "0 17 * * *"
	}`
	if err := os.WriteFile(filepath.Join(envDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Create main.tf
	tfContent := `resource "null_resource" "test" {}`
	if err := os.WriteFile(filepath.Join(envDir, "main.tf"), []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create mock client
	mockClient := &opentofu.MockTofuClient{}

	// Create scheduler
	sched := NewWithClient(mockClient)
	sched.statePath = stateFile
	sched.configDir = tempDir

	// Load environments and state
	if err := sched.LoadEnvironments(); err != nil {
		t.Fatalf("Failed to load environments: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Test manual deploy when in deploy failed state
	sched.state.SetEnvironmentError(envName, true, "Previous deploy failed")
	err := sched.ManualDeploy(envName)
	if err != nil {
		t.Fatalf("Manual deploy should work even when in deploy failed state: %v", err)
	}

	// Test manual destroy when in destroy failed state
	sched.state.SetEnvironmentError(envName, false, "Previous destroy failed")
	err = sched.ManualDestroy(envName)
	if err != nil {
		t.Fatalf("Manual destroy should work even when in destroy failed state: %v", err)
	}
}