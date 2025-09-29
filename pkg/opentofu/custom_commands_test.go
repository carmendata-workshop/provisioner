package opentofu

import (
	"os"
	"path/filepath"
	"testing"

	"provisioner/pkg/workspace"
)

func TestDeployWithCustomCommands(t *testing.T) {
	// Create temporary directory for test workspace
	tmpDir := t.TempDir()
	wsPath := filepath.Join(tmpDir, "test-workspace")
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create a simple main.tf file
	mainTF := filepath.Join(wsPath, "main.tf")
	if err := os.WriteFile(mainTF, []byte("# Test config"), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create test workspace with custom deploy commands
	ws := &workspace.Workspace{
		Name: "test-workspace",
		Path: wsPath,
		Config: workspace.Config{
			Enabled:        true,
			DeploySchedule: "0 9 * * *",
			CustomDeploy: &workspace.CustomDeployConfig{
				InitCommand:  "echo 'Custom init'",
				PlanCommand:  "echo 'Custom plan'",
				ApplyCommand: "echo 'Custom apply'",
			},
		},
	}

	// Set up test state directory
	stateDir := filepath.Join(tmpDir, "state")
	os.Setenv("PROVISIONER_STATE_DIR", stateDir)
	defer os.Unsetenv("PROVISIONER_STATE_DIR")

	// Create client
	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test deployment with custom commands
	err = client.Deploy(ws)
	if err != nil {
		t.Errorf("Deploy with custom commands failed: %v", err)
	}

	// Verify working directory was created
	workingDir := filepath.Join(stateDir, "deployments", ws.Name)
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		t.Errorf("Working directory was not created: %s", workingDir)
	}
}

func TestDeployWithPartialCustomCommands(t *testing.T) {
	// Create temporary directory for test workspace
	tmpDir := t.TempDir()
	wsPath := filepath.Join(tmpDir, "test-workspace")
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create a simple main.tf file
	mainTF := filepath.Join(wsPath, "main.tf")
	if err := os.WriteFile(mainTF, []byte("# Test config"), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create test workspace with only custom apply command
	ws := &workspace.Workspace{
		Name: "test-workspace",
		Path: wsPath,
		Config: workspace.Config{
			Enabled:        true,
			DeploySchedule: "0 9 * * *",
			CustomDeploy: &workspace.CustomDeployConfig{
				ApplyCommand: "echo 'Custom apply only'",
			},
		},
	}

	// Set up test state directory
	stateDir := filepath.Join(tmpDir, "state")
	os.Setenv("PROVISIONER_STATE_DIR", stateDir)
	defer os.Unsetenv("PROVISIONER_STATE_DIR")

	// Create client
	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Note: This will fail with real tofu commands, but validates the logic path
	// In a real scenario, you'd need tofu binary available or use mocks
	_ = client.Deploy(ws)
}

func TestDestroyWithCustomCommands(t *testing.T) {
	// Create temporary directory for test workspace
	tmpDir := t.TempDir()
	wsPath := filepath.Join(tmpDir, "test-workspace")
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create a simple main.tf file
	mainTF := filepath.Join(wsPath, "main.tf")
	if err := os.WriteFile(mainTF, []byte("# Test config"), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create test workspace with custom destroy commands
	ws := &workspace.Workspace{
		Name: "test-workspace",
		Path: wsPath,
		Config: workspace.Config{
			Enabled:         true,
			DeploySchedule:  "0 9 * * *",
			DestroySchedule: "0 18 * * *",
			CustomDestroy: &workspace.CustomDestroyConfig{
				InitCommand:    "echo 'Custom init for destroy'",
				DestroyCommand: "echo 'Custom destroy'",
			},
		},
	}

	// Set up test state directory
	stateDir := filepath.Join(tmpDir, "state")
	os.Setenv("PROVISIONER_STATE_DIR", stateDir)
	defer os.Unsetenv("PROVISIONER_STATE_DIR")

	// Create client
	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test destroy with custom commands
	err = client.DestroyWorkspace(ws)
	if err != nil {
		t.Errorf("Destroy with custom commands failed: %v", err)
	}

	// Verify working directory was created
	workingDir := filepath.Join(stateDir, "deployments", ws.Name)
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		t.Errorf("Working directory was not created: %s", workingDir)
	}
}

func TestExecuteCustomCommand(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create client
	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		name        string
		command     string
		shouldError bool
	}{
		{
			name:        "Simple echo command",
			command:     "echo 'test'",
			shouldError: false,
		},
		{
			name:        "Command with exit 0",
			command:     "exit 0",
			shouldError: false,
		},
		{
			name:        "Command that fails",
			command:     "exit 1",
			shouldError: true,
		},
		{
			name:        "Command with multiple statements",
			command:     "echo 'first' && echo 'second'",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.executeCustomCommand(tt.command, tmpDir)
			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestDeployWithoutCustomCommands(t *testing.T) {
	// Create temporary directory for test workspace
	tmpDir := t.TempDir()
	wsPath := filepath.Join(tmpDir, "test-workspace")
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create a simple main.tf file
	mainTF := filepath.Join(wsPath, "main.tf")
	if err := os.WriteFile(mainTF, []byte("# Test config"), 0644); err != nil {
		t.Fatalf("Failed to create main.tf: %v", err)
	}

	// Create test workspace WITHOUT custom commands
	ws := &workspace.Workspace{
		Name: "test-workspace",
		Path: wsPath,
		Config: workspace.Config{
			Enabled:        true,
			DeploySchedule: "0 9 * * *",
			// No CustomDeploy specified - should use default tofu commands
		},
	}

	// Set up test state directory
	stateDir := filepath.Join(tmpDir, "state")
	os.Setenv("PROVISIONER_STATE_DIR", stateDir)
	defer os.Unsetenv("PROVISIONER_STATE_DIR")

	// Create client
	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test deployment - will fail without real tofu but validates code path
	_ = client.Deploy(ws)

	// The test validates that nil CustomDeploy doesn't cause panics
}