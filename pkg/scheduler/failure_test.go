package scheduler

import (
	"testing"
	"time"
)

func TestFailureHandling(t *testing.T) {
	// Test that failed workspaces don't retry until config changes
	state := NewState()
	scheduler := &Scheduler{state: state}

	testWorkspace := "test-workspace"
	now := time.Now()

	// Create initial workspace state
	workspaceState := state.GetWorkspaceState(testWorkspace)
	if workspaceState.Status != StatusDestroyed {
		t.Errorf("expected initial status %s, got %s", StatusDestroyed, workspaceState.Status)
	}

	// Should deploy initially
	schedules := []string{"* * * * *"} // Every minute
	shouldDeploy := scheduler.ShouldRunDeploySchedule(schedules, now, workspaceState)
	if !shouldDeploy {
		t.Error("expected to deploy initially when status is destroyed")
	}

	// Simulate deployment failure
	state.SetWorkspaceError(testWorkspace, true, "deployment failed")
	workspaceState = state.GetWorkspaceState(testWorkspace)

	if workspaceState.Status != StatusDeployFailed {
		t.Errorf("expected status %s after deploy error, got %s", StatusDeployFailed, workspaceState.Status)
	}

	if workspaceState.LastDeployError != "deployment failed" {
		t.Errorf("expected error 'deployment failed', got '%s'", workspaceState.LastDeployError)
	}

	// Should NOT deploy again while in failed state
	shouldDeploy = scheduler.ShouldRunDeploySchedule(schedules, now.Add(time.Minute), workspaceState)
	if shouldDeploy {
		t.Error("expected NOT to deploy when in failed state")
	}

	// Simulate config change
	configTime := now.Add(2 * time.Minute)
	state.SetWorkspaceConfigModified(testWorkspace, configTime)
	workspaceState = state.GetWorkspaceState(testWorkspace)

	// Should reset to destroyed state and clear error
	if workspaceState.Status != StatusDestroyed {
		t.Errorf("expected status %s after config change, got %s", StatusDestroyed, workspaceState.Status)
	}

	if workspaceState.LastDeployError != "" {
		t.Errorf("expected empty error after config change, got '%s'", workspaceState.LastDeployError)
	}

	// Should be able to deploy again after config change
	shouldDeploy = scheduler.ShouldRunDeploySchedule(schedules, now.Add(3*time.Minute), workspaceState)
	if !shouldDeploy {
		t.Error("expected to deploy again after config change")
	}
}

func TestFailureHandlingWithMultipleSchedules(t *testing.T) {
	// Test failure handling with multiple deploy schedules
	state := NewState()
	scheduler := &Scheduler{state: state}

	testWorkspace := "test-workspace-multi"
	now := time.Now()

	workspaceState := state.GetWorkspaceState(testWorkspace)
	schedules := []string{"0 9 * * *", "0 17 * * *"} // 9 AM and 5 PM

	// Set up a time when deployment should happen (after 9 AM)
	testTime := time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, now.Location())

	// Should deploy initially
	shouldDeploy := scheduler.ShouldRunDeploySchedule(schedules, testTime, workspaceState)
	if !shouldDeploy {
		t.Error("expected to deploy at 9:30 AM when 9 AM schedule should have run")
	}

	// Simulate failure
	state.SetWorkspaceError(testWorkspace, true, "deployment failed")
	workspaceState = state.GetWorkspaceState(testWorkspace)

	// Should NOT deploy again even though 5 PM schedule will come
	testTime = time.Date(now.Year(), now.Month(), now.Day(), 17, 30, 0, 0, now.Location())
	shouldDeploy = scheduler.ShouldRunDeploySchedule(schedules, testTime, workspaceState)
	if shouldDeploy {
		t.Error("expected NOT to deploy at 5:30 PM when in failed state")
	}
}
