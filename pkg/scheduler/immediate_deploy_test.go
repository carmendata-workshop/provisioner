package scheduler

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"provisioner/pkg/opentofu"
	"provisioner/pkg/workspace"
)

func TestImmediateDeploymentOnConfigChange(t *testing.T) {
	// Create temporary directories for test
	tempDir, err := os.MkdirTemp("", "scheduler-immediate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	stateDir := filepath.Join(tempDir, "state")
	workspacePath := filepath.Join(tempDir, "test-workspace")

	// Create mock client to track deployments
	mockClient := opentofu.NewMockTofuClient()
	deploymentTriggered := false

	// Override the client to track deployments
	mockClient.DeployFunc = func(workspace *workspace.Workspace) error {
		deploymentTriggered = true
		return nil
	}

	// Create scheduler
	scheduler := NewWithClient(mockClient)
	scheduler.statePath = filepath.Join(stateDir, "scheduler.json")
	scheduler.state = NewState()

	// Create test workspace
	testWorkspace := workspace.Workspace{
		Name: "test-immediate",
		Config: workspace.Config{
			Enabled:        true,
			DeploySchedule: "* * * * *", // Every minute - should always be deployable
		},
		Path: workspacePath,
	}

	// Set up workspaces in scheduler
	scheduler.workspaces = []workspace.Workspace{testWorkspace}

	// Test 1: Workspace in destroyed state should deploy immediately on config change
	t.Run("deploy immediately when destroyed", func(t *testing.T) {
		deploymentTriggered = false
		workspaceState := scheduler.state.GetWorkspaceState("test-immediate")

		if workspaceState.Status != StatusDestroyed {
			t.Fatalf("expected initial status %s, got %s", StatusDestroyed, workspaceState.Status)
		}

		// Simulate config change - this should trigger immediate deployment
		now := time.Now()
		scheduler.checkWorkspaceForImmediateDeployment("test-immediate", now)

		// Give goroutine a moment to execute
		time.Sleep(10 * time.Millisecond)

		if !deploymentTriggered {
			t.Error("expected deployment to be triggered immediately on config change")
		}
	})

	// Test 2: Workspace in failed state should deploy immediately after config change
	t.Run("deploy immediately when failed", func(t *testing.T) {
		// Reset state
		scheduler.state = NewState()
		deploymentTriggered = false

		// Set workspace to failed state
		scheduler.state.SetWorkspaceError("test-immediate", true, "previous failure")
		workspaceState := scheduler.state.GetWorkspaceState("test-immediate")

		if workspaceState.Status != StatusDeployFailed {
			t.Fatalf("expected status %s, got %s", StatusDeployFailed, workspaceState.Status)
		}

		// Simulate config change - this should reset state and trigger deployment
		now := time.Now()
		scheduler.state.SetWorkspaceConfigModified("test-immediate", now)
		scheduler.checkWorkspaceForImmediateDeployment("test-immediate", now)

		// Give goroutine a moment to execute
		time.Sleep(10 * time.Millisecond)

		if !deploymentTriggered {
			t.Error("expected deployment to be triggered immediately after config change on failed workspace")
		}
	})

	// Test 3: Workspace in deployed state should deploy immediately after config change
	t.Run("redeploy immediately when deployed", func(t *testing.T) {
		// Reset state
		scheduler.state = NewState()
		deploymentTriggered = false

		// Set workspace to deployed state
		scheduler.state.SetWorkspaceStatus("test-immediate", StatusDeployed)
		workspaceState := scheduler.state.GetWorkspaceState("test-immediate")

		if workspaceState.Status != StatusDeployed {
			t.Fatalf("expected status %s, got %s", StatusDeployed, workspaceState.Status)
		}

		// Simulate config change - this should reset state and trigger redeployment
		now := time.Now()
		scheduler.state.SetWorkspaceConfigModified("test-immediate", now)
		scheduler.checkWorkspaceForImmediateDeployment("test-immediate", now)

		// Give goroutine a moment to execute
		time.Sleep(10 * time.Millisecond)

		if !deploymentTriggered {
			t.Error("expected redeployment to be triggered immediately after config change on deployed workspace")
		}
	})

	// Test 4: Busy workspace should not deploy immediately
	t.Run("skip immediate deploy when busy", func(t *testing.T) {
		// Reset state
		scheduler.state = NewState()
		deploymentTriggered = false

		// Set workspace to deploying state (busy)
		scheduler.state.SetWorkspaceStatus("test-immediate", StatusDeploying)

		// Simulate config change - this should NOT trigger deployment
		now := time.Now()
		scheduler.checkWorkspaceForImmediateDeployment("test-immediate", now)

		// Give potential goroutine a moment
		time.Sleep(10 * time.Millisecond)

		if deploymentTriggered {
			t.Error("expected NO deployment when workspace is busy")
		}
	})
}

func TestImmediateDeploymentWithScheduleCheck(t *testing.T) {
	// Test that immediate deployment respects schedule constraints
	tempDir, err := os.MkdirTemp("", "scheduler-schedule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := opentofu.NewMockTofuClient()
	scheduler := NewWithClient(mockClient)
	scheduler.state = NewState()

	// Workspace with specific schedule (9 AM only)
	testWorkspace := workspace.Workspace{
		Name: "test-scheduled",
		Config: workspace.Config{
			Enabled:        true,
			DeploySchedule: "0 9 * * *", // Only at 9:00 AM
		},
		Path: tempDir,
	}

	scheduler.workspaces = []workspace.Workspace{testWorkspace}

	// Test at 8 AM - should NOT deploy even with config change
	morningTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	deploymentTriggered := false

	mockClient.DeployFunc = func(workspace *workspace.Workspace) error {
		deploymentTriggered = true
		return nil
	}

	scheduler.checkWorkspaceForImmediateDeployment("test-scheduled", morningTime)
	time.Sleep(10 * time.Millisecond)

	if deploymentTriggered {
		t.Error("expected NO deployment at 8 AM when schedule is 9 AM")
	}

	// Test at 10 AM (after 9 AM schedule) - should deploy
	laterTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	deploymentTriggered = false

	scheduler.checkWorkspaceForImmediateDeployment("test-scheduled", laterTime)
	time.Sleep(10 * time.Millisecond)

	if !deploymentTriggered {
		t.Error("expected deployment at 10 AM when 9 AM schedule has passed")
	}
}
