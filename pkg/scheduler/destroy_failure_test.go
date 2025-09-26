package scheduler

import (
	"testing"
	"time"
)

func TestDestroyFailureHandling(t *testing.T) {
	// Test that failed destroy operations don't retry until config changes
	state := NewState()
	scheduler := &Scheduler{state: state}

	testWorkspace := "test-destroy-fail"
	now := time.Now()

	// Create workspace in deployed state
	state.SetWorkspaceStatus(testWorkspace, StatusDeployed)
	workspaceState := state.GetWorkspaceState(testWorkspace)
	if workspaceState.Status != StatusDeployed {
		t.Fatalf("expected initial status %s, got %s", StatusDeployed, workspaceState.Status)
	}

	// Should destroy initially when deployed
	schedules := []string{"* * * * *"} // Every minute
	shouldDestroy := scheduler.ShouldRunDestroySchedule(schedules, now, workspaceState)
	if !shouldDestroy {
		t.Error("expected to destroy initially when status is deployed")
	}

	// Simulate destroy failure
	state.SetWorkspaceError(testWorkspace, false, "destroy failed")
	workspaceState = state.GetWorkspaceState(testWorkspace)

	if workspaceState.Status != StatusDestroyFailed {
		t.Errorf("expected status %s after destroy error, got %s", StatusDestroyFailed, workspaceState.Status)
	}

	if workspaceState.LastDestroyError != "destroy failed" {
		t.Errorf("expected error 'destroy failed', got '%s'", workspaceState.LastDestroyError)
	}

	// Should NOT destroy again while in failed state
	shouldDestroy = scheduler.ShouldRunDestroySchedule(schedules, now.Add(time.Minute), workspaceState)
	if shouldDestroy {
		t.Error("expected NOT to destroy when in destroy failed state")
	}

	// Simulate config change
	configTime := now.Add(2 * time.Minute)
	state.SetWorkspaceConfigModified(testWorkspace, configTime)
	workspaceState = state.GetWorkspaceState(testWorkspace)

	// Should reset to deployed state and clear error
	if workspaceState.Status != StatusDeployed {
		t.Errorf("expected status %s after config change, got %s", StatusDeployed, workspaceState.Status)
	}

	if workspaceState.LastDestroyError != "" {
		t.Errorf("expected empty error after config change, got '%s'", workspaceState.LastDestroyError)
	}

	// Should be able to destroy again after config change
	shouldDestroy = scheduler.ShouldRunDestroySchedule(schedules, now.Add(3*time.Minute), workspaceState)
	if !shouldDestroy {
		t.Error("expected to destroy again after config change")
	}
}

func TestDestroyFailureWithMultipleSchedules(t *testing.T) {
	// Test destroy failure handling with multiple destroy schedules
	state := NewState()
	scheduler := &Scheduler{state: state}

	testWorkspace := "test-destroy-multi"
	now := time.Now()

	// Set workspace to deployed state
	state.SetWorkspaceStatus(testWorkspace, StatusDeployed)
	workspaceState := state.GetWorkspaceState(testWorkspace)
	schedules := []string{"0 18 * * *", "0 22 * * *"} // 6 PM and 10 PM

	// Set up a time when destruction should happen (after 6 PM)
	testTime := time.Date(now.Year(), now.Month(), now.Day(), 18, 30, 0, 0, now.Location())

	// Should destroy initially
	shouldDestroy := scheduler.ShouldRunDestroySchedule(schedules, testTime, workspaceState)
	if !shouldDestroy {
		t.Error("expected to destroy at 6:30 PM when 6 PM schedule should have run")
	}

	// Simulate failure
	state.SetWorkspaceError(testWorkspace, false, "destroy failed")
	workspaceState = state.GetWorkspaceState(testWorkspace)

	// Should NOT destroy again even though 10 PM schedule will come
	testTime = time.Date(now.Year(), now.Month(), now.Day(), 22, 30, 0, 0, now.Location())
	shouldDestroy = scheduler.ShouldRunDestroySchedule(schedules, testTime, workspaceState)
	if shouldDestroy {
		t.Error("expected NOT to destroy at 10:30 PM when in destroy failed state")
	}
}

func TestStateTransitions(t *testing.T) {
	// Test all possible state transitions with config changes
	tests := []struct {
		name           string
		initialStatus  WorkspaceStatus
		configChange   bool
		expectedStatus WorkspaceStatus
	}{
		{"deploy_failed_with_config_change", StatusDeployFailed, true, StatusDestroyed},
		{"destroy_failed_with_config_change", StatusDestroyFailed, true, StatusDeployed},
		{"deployed_with_config_change", StatusDeployed, true, StatusDestroyed},
		{"destroyed_with_config_change", StatusDestroyed, true, StatusDestroyed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewState()
			testWorkspace := "test-workspace"

			// Set initial status
			state.SetWorkspaceStatus(testWorkspace, tt.initialStatus)
			workspaceState := state.GetWorkspaceState(testWorkspace)

			if tt.configChange {
				state.SetWorkspaceConfigModified(testWorkspace, time.Now())
				workspaceState = state.GetWorkspaceState(testWorkspace)
			}

			if workspaceState.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, workspaceState.Status)
			}
		})
	}
}
