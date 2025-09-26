package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"provisioner/pkg/opentofu"
	"provisioner/pkg/workspace"
)

func TestSchedulerDeployWorkspace(t *testing.T) {
	// Create temporary directories
	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	stateDir := filepath.Join(tempDir, "state")
	workspacePath := filepath.Join(tempDir, "test-workspace")

	// Create test workspace
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}

	workspace := workspace.Workspace{
		Name: "test-workspace",
		Config: workspace.Config{
			Enabled:         true,
			DeploySchedule:  "* * * * *",
			DestroySchedule: "* * * * *",
		},
		Path: workspacePath,
	}

	// Create mock client
	mockClient := opentofu.NewMockTofuClient()

	// Create scheduler with mock
	scheduler := NewWithClient(mockClient)
	scheduler.statePath = filepath.Join(stateDir, "scheduler.json")
	scheduler.state = NewState()

	// Test successful deployment
	scheduler.deployWorkspace(workspace)

	// Verify mock was called
	if mockClient.DeployCallCount != 1 {
		t.Errorf("expected 1 deploy call, got %d", mockClient.DeployCallCount)
	}

	deployWorkspace := mockClient.GetLastDeployWorkspace()
	if deployWorkspace == nil || deployWorkspace.Path != workspacePath {
		t.Errorf("expected deploy workspace path '%s', got %v", workspacePath, deployWorkspace)
	}

	// Verify state was updated
	workspaceState := scheduler.state.GetWorkspaceState("test-workspace")
	if workspaceState.Status != StatusDeployed {
		t.Errorf("expected status %s, got %s", StatusDeployed, workspaceState.Status)
	}

	if workspaceState.LastDeployed == nil {
		t.Error("expected LastDeployed to be set")
	}

	if workspaceState.LastDeployError != "" {
		t.Errorf("expected no deploy error, got '%s'", workspaceState.LastDeployError)
	}
}

func TestSchedulerDeployWorkspaceError(t *testing.T) {
	// Create temporary directories
	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	stateDir := filepath.Join(tempDir, "state")
	workspacePath := filepath.Join(tempDir, "test-workspace")

	workspace := workspace.Workspace{
		Name:   "test-workspace",
		Config: workspace.Config{},
		Path:   workspacePath,
	}

	// Create mock client with error
	mockClient := opentofu.NewMockTofuClient()
	deployError := fmt.Errorf("deploy failed")
	mockClient.SetDeployError(deployError)

	// Create scheduler
	scheduler := NewWithClient(mockClient)
	scheduler.statePath = filepath.Join(stateDir, "scheduler.json")
	scheduler.state = NewState()

	// Test failed deployment
	scheduler.deployWorkspace(workspace)

	// Verify state shows error
	workspaceState := scheduler.state.GetWorkspaceState("test-workspace")
	if workspaceState.Status != StatusDeployFailed {
		t.Errorf("expected status %s after error, got %s", StatusDeployFailed, workspaceState.Status)
	}

	if workspaceState.LastDeployError != "deploy failed" {
		t.Errorf("expected deploy error 'deploy failed', got '%s'", workspaceState.LastDeployError)
	}
}

func TestSchedulerDestroyWorkspace(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	stateDir := filepath.Join(tempDir, "state")
	workspacePath := filepath.Join(tempDir, "test-workspace")

	workspace := workspace.Workspace{
		Name:   "test-workspace",
		Config: workspace.Config{},
		Path:   workspacePath,
	}

	// Create mock client
	mockClient := opentofu.NewMockTofuClient()

	// Create scheduler
	scheduler := NewWithClient(mockClient)
	scheduler.statePath = filepath.Join(stateDir, "scheduler.json")
	scheduler.state = NewState()

	// Set initial state as deployed
	scheduler.state.SetWorkspaceStatus("test-workspace", StatusDeployed)

	// Test destruction
	scheduler.destroyWorkspace(workspace)

	// Verify mock was called
	if mockClient.DestroyCallCount != 1 {
		t.Errorf("expected 1 destroy call, got %d", mockClient.DestroyCallCount)
	}

	destroyWorkspace := mockClient.GetLastDestroyWorkspace()
	if destroyWorkspace == nil || destroyWorkspace.Path != workspacePath {
		t.Errorf("expected destroy workspace path '%s', got %v", workspacePath, destroyWorkspace)
	}

	// Verify state was updated
	workspaceState := scheduler.state.GetWorkspaceState("test-workspace")
	if workspaceState.Status != StatusDestroyed {
		t.Errorf("expected status %s, got %s", StatusDestroyed, workspaceState.Status)
	}

	if workspaceState.LastDestroyed == nil {
		t.Error("expected LastDestroyed to be set")
	}
}

func TestSchedulerCheckWorkspaceSchedules(t *testing.T) {
	// Create temporary workspace directory for testing
	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create mock client
	mockClient := opentofu.NewMockTofuClient()

	// Create scheduler
	scheduler := NewWithClient(mockClient)
	scheduler.state = NewState()

	// Create test workspace with schedules that should trigger
	// Using specific time schedule instead of "* * * * *" to work with window-based logic
	workspace := workspace.Workspace{
		Name: "test-workspace",
		Config: workspace.Config{
			Enabled:         true,
			DeploySchedule:  "30 14 * * *", // 2:30 PM daily
			DestroySchedule: "30 14 * * *", // 2:30 PM daily
		},
		Path: filepath.Join(tempDir, "test-workspace"),
	}

	// Test time after the scheduled time (window-based logic)
	testTime := time.Date(2024, 6, 15, 14, 35, 0, 0, time.UTC) // 5 minutes after scheduled time

	// Workspace starts as destroyed, so deploy should trigger
	scheduler.checkWorkspaceSchedules(workspace, testTime)

	// Wait a brief moment for goroutine to complete
	time.Sleep(10 * time.Millisecond)

	// Verify deploy was called
	if mockClient.DeployCallCount != 1 {
		t.Errorf("expected 1 deploy call, got %d", mockClient.DeployCallCount)
	}

	// Reset mock and set workspace as deployed
	mockClient.Reset()
	scheduler.state.SetWorkspaceStatus("test-workspace", StatusDeployed)

	// Now destroy should trigger (since workspace is deployed and destroy time has passed)
	scheduler.checkWorkspaceSchedules(workspace, testTime)

	// Wait a brief moment for goroutine to complete
	time.Sleep(10 * time.Millisecond)

	// Verify destroy was called
	if mockClient.DestroyCallCount != 1 {
		t.Errorf("expected 1 destroy call, got %d", mockClient.DestroyCallCount)
	}
}

func TestSchedulerSkipsBusyWorkspaces(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := opentofu.NewMockTofuClient()
	scheduler := NewWithClient(mockClient)
	scheduler.state = NewState()

	workspace := workspace.Workspace{
		Name: "test-workspace",
		Config: workspace.Config{
			DeploySchedule:  "* * * * *",
			DestroySchedule: "* * * * *",
		},
		Path: filepath.Join(tempDir, "test-workspace"),
	}

	testTime := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)

	// Set workspace as currently deploying
	scheduler.state.SetWorkspaceStatus("test-workspace", StatusDeploying)

	// Check schedules - should skip busy workspace
	scheduler.checkWorkspaceSchedules(workspace, testTime)

	// Wait briefly
	time.Sleep(10 * time.Millisecond)

	// Verify no operations were called
	if mockClient.DeployCallCount != 0 {
		t.Errorf("expected 0 deploy calls for busy workspace, got %d", mockClient.DeployCallCount)
	}

	if mockClient.DestroyCallCount != 0 {
		t.Errorf("expected 0 destroy calls for busy workspace, got %d", mockClient.DestroyCallCount)
	}
}

func TestSchedulerLoadWorkspaces(t *testing.T) {
	// Create temporary workspaces directory
	tempDir, err := os.MkdirTemp("", "scheduler-workspace-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test workspace
	workspaceDir := filepath.Join(tempDir, "test-workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}

	config := workspace.Config{
		Enabled:         true,
		DeploySchedule:  "0 9 * * *",
		DestroySchedule: "0 17 * * *",
		Description:     "Test workspace",
	}

	configData, _ := json.Marshal(config)
	configPath := filepath.Join(workspaceDir, "config.json")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create scheduler
	scheduler := New()

	// Override LoadWorkspaces to use our test directory
	// We'll need to modify the method or create a test version

	// For now, test that a scheduler can be created
	if scheduler == nil {
		t.Error("expected scheduler to be created")
		return
	}

	if scheduler.statePath != "state/scheduler.json" {
		t.Errorf("expected default state path, got %s", scheduler.statePath)
	}
}

func TestSchedulerShouldRunAnySchedule(t *testing.T) {
	scheduler := New()

	// Test time: Monday 9:00 AM
	testTime := time.Date(2024, 6, 17, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		schedules []string
		expected  bool
	}{
		{
			name:      "single matching schedule",
			schedules: []string{"0 9 * * 1"},
			expected:  true,
		},
		{
			name:      "multiple schedules, first matches",
			schedules: []string{"0 9 * * 1", "0 10 * * 1"},
			expected:  true,
		},
		{
			name:      "multiple schedules, second matches",
			schedules: []string{"0 8 * * 1", "0 9 * * 1"},
			expected:  true,
		},
		{
			name:      "multiple schedules, none match",
			schedules: []string{"0 8 * * 1", "0 10 * * 1"},
			expected:  false,
		},
		{
			name:      "empty schedule list",
			schedules: []string{},
			expected:  false,
		},
		{
			name:      "invalid schedule in list",
			schedules: []string{"invalid", "0 9 * * 1"},
			expected:  true, // Should still match the valid one
		},
		{
			name:      "all invalid schedules",
			schedules: []string{"invalid1", "invalid2"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scheduler.shouldRunAnySchedule(tt.schedules, testTime)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSchedulerMultipleDeploySchedules(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create mock opentofu client
	mockClient := opentofu.NewMockTofuClient()

	// Create scheduler with mock
	scheduler := NewWithClient(mockClient)

	// Initialize state properly
	scheduler.state = NewState()

	// Add test workspace with multiple deploy schedules
	testWorkspace := workspace.Workspace{
		Name: "test-workspace",
		Config: workspace.Config{
			Enabled:         true,
			DeploySchedule:  []string{"0 9 * * 1", "0 14 * * 1"}, // Monday 9am and 2pm
			DestroySchedule: "0 17 * * 1",                        // Monday 5pm
			Description:     "Test workspace with multiple deploy schedules",
		},
		Path: tempDir,
	}
	scheduler.workspaces = []workspace.Workspace{testWorkspace}

	// Test Monday 9:05 AM - should trigger deploy (after 9:00 AM schedule)
	mondayAM := time.Date(2024, 6, 17, 9, 5, 0, 0, time.UTC)
	scheduler.checkWorkspaceSchedules(scheduler.workspaces[0], mondayAM)

	// Wait for goroutine to complete
	time.Sleep(10 * time.Millisecond)

	// Verify deployment was attempted
	if mockClient.DeployCallCount != 1 {
		t.Errorf("expected 1 deploy call for Monday 9am schedule, got %d", mockClient.DeployCallCount)
	}

	// Reset mock
	mockClient.Reset()

	// Test Monday 2:05 PM - should trigger deploy again (after 2:00 PM schedule)
	mondayPM := time.Date(2024, 6, 17, 14, 5, 0, 0, time.UTC)
	// Reset workspace state to allow deployment AND clear the last deployed time
	// so the new schedule window can trigger
	scheduler.state.SetWorkspaceStatus("test-workspace", StatusDestroyed)
	workspaceState := scheduler.state.GetWorkspaceState("test-workspace")
	workspaceState.LastDeployed = nil // Clear last deployed time to allow new schedule
	scheduler.checkWorkspaceSchedules(scheduler.workspaces[0], mondayPM)

	// Wait for goroutine to complete
	time.Sleep(10 * time.Millisecond)

	// Verify deployment was attempted again
	if mockClient.DeployCallCount != 1 {
		t.Errorf("expected 1 deploy call for Monday 2pm schedule, got %d", mockClient.DeployCallCount)
	}

	// Test Monday 10:00 AM - should NOT trigger deploy (no schedule)
	mockClient.Reset()
	mondayMid := time.Date(2024, 6, 17, 10, 0, 0, 0, time.UTC)
	scheduler.state.SetWorkspaceStatus("test-workspace", StatusDestroyed)
	scheduler.checkWorkspaceSchedules(scheduler.workspaces[0], mondayMid)

	// Wait for potential goroutine (shouldn't happen)
	time.Sleep(10 * time.Millisecond)

	// Verify deployment was NOT attempted
	if mockClient.DeployCallCount != 0 {
		t.Errorf("expected 0 deploy calls for Monday 10am (no matching schedule), got %d", mockClient.DeployCallCount)
	}
}
