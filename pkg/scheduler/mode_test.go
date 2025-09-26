package scheduler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"provisioner/pkg/opentofu"
	"provisioner/pkg/workspace"
)

func TestManualDeployInMode(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "scheduler-mode-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create workspace with mode schedules
	workspaceName := "test-mode-workspace"
	workspacesDir := filepath.Join(tempDir, "workspaces")
	workspaceDir := filepath.Join(workspacesDir, workspaceName)

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}

	// Create workspace config with mode schedules
	config := workspace.Config{
		Enabled:  true,
		Template: "web-app",
		ModeSchedules: map[string]interface{}{
			"hibernation": "0 23 * * 1-5",
			"busy":        "0 8 * * 1-5",
			"maintenance": "0 2 * * 0",
		},
	}

	configPath := filepath.Join(workspaceDir, "config.json")
	if err := writeConfigFile(configPath, config); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Create main.tf file
	mainTFPath := filepath.Join(workspaceDir, "main.tf")
	mainTFContent := `variable "deployment_mode" {
  description = "Deployment mode"
  type        = string
  default     = "busy"
}

resource "null_resource" "test" {
  count = var.deployment_mode == "hibernation" ? 0 : 1
}`
	if err := os.WriteFile(mainTFPath, []byte(mainTFContent), 0644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	// Create scheduler with mock client
	mockClient := opentofu.NewMockTofuClient()
	sched := NewWithClient(mockClient)
	sched.statePath = filepath.Join(tempDir, "scheduler.json")
	sched.configDir = tempDir

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Test manual deploy in valid mode
	err = sched.ManualDeployInMode(workspaceName, "busy")
	if err != nil {
		t.Errorf("expected no error for valid mode, got %v", err)
	}

	// Verify mock client was called
	if mockClient.DeployInModeCallCount != 1 {
		t.Errorf("expected DeployInModeCallCount = 1, got %d", mockClient.DeployInModeCallCount)
	}

	if len(mockClient.DeployInModeCalls) != 1 || mockClient.DeployInModeCalls[0] != "busy" {
		t.Errorf("expected mode 'busy', got %v", mockClient.DeployInModeCalls)
	}

	// Verify workspace state was updated
	workspaceState := sched.state.GetWorkspaceState(workspaceName)
	if workspaceState.Status != StatusDeployed {
		t.Errorf("expected status %s, got %s", StatusDeployed, workspaceState.Status)
	}

	if workspaceState.DeploymentMode != "busy" {
		t.Errorf("expected deployment mode 'busy', got '%s'", workspaceState.DeploymentMode)
	}

	// Test deploy in same mode (should be no-op)
	mockClient.Reset()
	err = sched.ManualDeployInMode(workspaceName, "busy")
	if err != nil {
		t.Errorf("expected no error for same mode, got %v", err)
	}

	// Verify no additional calls to DeployInMode (should be idempotent)
	if mockClient.DeployInModeCallCount != 0 {
		t.Errorf("expected no additional DeployInMode calls for same mode, got %d", mockClient.DeployInModeCallCount)
	}

	// Test deploy in different mode on new workspace to avoid confirmation prompt
	workspaceName2 := "test-mode-workspace-2"
	workspaceDir2 := filepath.Join(workspacesDir, workspaceName2)

	if err := os.MkdirAll(workspaceDir2, 0755); err != nil {
		t.Fatalf("failed to create workspace2 directory: %v", err)
	}

	configPath2 := filepath.Join(workspaceDir2, "config.json")
	if err := writeConfigFile(configPath2, config); err != nil {
		t.Fatalf("failed to write config file2: %v", err)
	}

	mainTFPath2 := filepath.Join(workspaceDir2, "main.tf")
	if err := os.WriteFile(mainTFPath2, []byte(mainTFContent), 0644); err != nil {
		t.Fatalf("failed to write main.tf2: %v", err)
	}

	// Reload workspaces to include the new one
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("failed to reload workspaces: %v", err)
	}

	// Deploy the new workspace in hibernation mode
	err = sched.ManualDeployInMode(workspaceName2, "hibernation")
	if err != nil {
		t.Errorf("expected no error for hibernation mode, got %v", err)
	}

	// Verify state was set correctly
	workspaceState2 := sched.state.GetWorkspaceState(workspaceName2)
	if workspaceState2.DeploymentMode != "hibernation" {
		t.Errorf("expected deployment mode 'hibernation', got '%s'", workspaceState2.DeploymentMode)
	}
}

func TestManualDeployInModeValidation(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "scheduler-mode-validation-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create workspace with mode schedules
	workspaceName := "test-validation-workspace"
	workspacesDir := filepath.Join(tempDir, "workspaces")
	workspaceDir := filepath.Join(workspacesDir, workspaceName)

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}

	// Create workspace config with mode schedules
	config := workspace.Config{
		Enabled:  true,
		Template: "web-app",
		ModeSchedules: map[string]interface{}{
			"hibernation": "0 23 * * 1-5",
			"busy":        "0 8 * * 1-5",
		},
	}

	configPath := filepath.Join(workspaceDir, "config.json")
	if err := writeConfigFile(configPath, config); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Create main.tf file
	mainTFPath := filepath.Join(workspaceDir, "main.tf")
	if err := os.WriteFile(mainTFPath, []byte("# test"), 0644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	// Create scheduler
	mockClient := opentofu.NewMockTofuClient()
	sched := NewWithClient(mockClient)
	sched.statePath = filepath.Join(tempDir, "scheduler.json")
	sched.configDir = tempDir

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Test invalid mode
	err = sched.ManualDeployInMode(workspaceName, "invalid-mode")
	if err == nil {
		t.Errorf("expected error for invalid mode")
	}

	expectedError := "mode 'invalid-mode' not available"
	if err != nil && !strings.Contains(err.Error(), expectedError) {
		t.Errorf("expected error containing '%s', got '%s'", expectedError, err.Error())
	}

	// Test nonexistent workspace
	err = sched.ManualDeployInMode("nonexistent", "busy")
	if err == nil {
		t.Errorf("expected error for nonexistent workspace")
	}

	expectedError = "workspace 'nonexistent' not found"
	if err != nil && !strings.Contains(err.Error(), expectedError) {
		t.Errorf("expected error containing '%s', got '%s'", expectedError, err.Error())
	}
}

func TestManualDeployInModeTraditionalWorkspace(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "scheduler-traditional-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create traditional workspace (uses deploy_schedule)
	workspaceName := "traditional-workspace"
	workspacesDir := filepath.Join(tempDir, "workspaces")
	workspaceDir := filepath.Join(workspacesDir, workspaceName)

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}

	// Create workspace config with deploy_schedule (not mode_schedules)
	config := workspace.Config{
		Enabled:        true,
		DeploySchedule: "0 9 * * 1-5",
	}

	configPath := filepath.Join(workspaceDir, "config.json")
	if err := writeConfigFile(configPath, config); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Create main.tf file
	mainTFPath := filepath.Join(workspaceDir, "main.tf")
	if err := os.WriteFile(mainTFPath, []byte("# test"), 0644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	// Create scheduler
	mockClient := opentofu.NewMockTofuClient()
	sched := NewWithClient(mockClient)
	sched.statePath = filepath.Join(tempDir, "scheduler.json")
	sched.configDir = tempDir

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Test ManualDeployInMode on traditional workspace
	err = sched.ManualDeployInMode(workspaceName, "busy")
	if err == nil {
		t.Errorf("expected error for mode parameter on traditional workspace")
	}

	expectedError := "uses traditional scheduling"
	if err != nil && !strings.Contains(err.Error(), expectedError) {
		t.Errorf("expected error containing '%s', got '%s'", expectedError, err.Error())
	}
}

func TestGetWorkspace(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "scheduler-get-workspace-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create workspace
	workspaceName := "test-get-workspace"
	workspacesDir := filepath.Join(tempDir, "workspaces")
	workspaceDir := filepath.Join(workspacesDir, workspaceName)

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}

	config := workspace.Config{
		Enabled:        true,
		DeploySchedule: "0 9 * * 1-5",
	}

	configPath := filepath.Join(workspaceDir, "config.json")
	if err := writeConfigFile(configPath, config); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	mainTFPath := filepath.Join(workspaceDir, "main.tf")
	if err := os.WriteFile(mainTFPath, []byte("# test"), 0644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	// Create scheduler
	mockClient := opentofu.NewMockTofuClient()
	sched := NewWithClient(mockClient)
	sched.statePath = filepath.Join(tempDir, "scheduler.json")
	sched.configDir = tempDir

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("failed to load workspaces: %v", err)
	}
	if err := sched.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Test GetWorkspace with existing workspace
	ws := sched.GetWorkspace(workspaceName)
	if ws == nil {
		t.Errorf("expected workspace, got nil")
	} else if ws.Name != workspaceName {
		t.Errorf("expected workspace name '%s', got '%s'", workspaceName, ws.Name)
	}

	// Test GetWorkspace with nonexistent workspace
	ws = sched.GetWorkspace("nonexistent")
	if ws != nil {
		t.Errorf("expected nil for nonexistent workspace, got %v", ws)
	}
}

// Helper function to write config file
func writeConfigFile(path string, config workspace.Config) error {
	data := `{
  "enabled": true,
  "template": "` + config.Template + `"`

	if config.DeploySchedule != nil {
		data += `,
  "deploy_schedule": "` + config.DeploySchedule.(string) + `"`
	}

	if config.ModeSchedules != nil {
		data += `,
  "mode_schedules": {`
		first := true
		for mode, schedule := range config.ModeSchedules {
			if !first {
				data += ","
			}
			first = false
			data += `
    "` + mode + `": "` + schedule.(string) + `"`
		}
		data += `
  }`
	}

	data += `
}`

	return os.WriteFile(path, []byte(data), 0644)
}

