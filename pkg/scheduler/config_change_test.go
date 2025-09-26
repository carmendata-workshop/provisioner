package scheduler

import (
	"testing"
	"time"
)

func TestConfigChangeHandling(t *testing.T) {
	state := NewState()
	scheduler := &Scheduler{state: state}

	testWorkspace := "test-config-change"
	now := time.Now()
	schedules := []string{"* * * * *"} // Every minute

	t.Run("failed workspace resets on config change", func(t *testing.T) {
		// Set workspace to failed state
		state.SetWorkspaceError(testWorkspace, true, "deployment failed")
		workspaceState := state.GetWorkspaceState(testWorkspace)

		if workspaceState.Status != StatusDeployFailed {
			t.Fatalf("expected status %s, got %s", StatusDeployFailed, workspaceState.Status)
		}

		// Simulate config change
		configTime := now.Add(time.Minute)
		state.SetWorkspaceConfigModified(testWorkspace, configTime)
		workspaceState = state.GetWorkspaceState(testWorkspace)

		// Should reset to destroyed and clear error
		if workspaceState.Status != StatusDestroyed {
			t.Errorf("expected status %s after config change, got %s", StatusDestroyed, workspaceState.Status)
		}

		if workspaceState.LastDeployError != "" {
			t.Errorf("expected empty error after config change, got '%s'", workspaceState.LastDeployError)
		}

		// Should be able to deploy again
		shouldDeploy := scheduler.ShouldRunDeploySchedule(schedules, now.Add(2*time.Minute), workspaceState)
		if !shouldDeploy {
			t.Error("expected to deploy after config change on failed workspace")
		}
	})

	t.Run("deployed workspace resets on config change", func(t *testing.T) {
		// Reset state for new test
		state = NewState()
		scheduler = &Scheduler{state: state}

		// Set workspace to deployed state
		state.SetWorkspaceStatus(testWorkspace, StatusDeployed)
		workspaceState := state.GetWorkspaceState(testWorkspace)

		if workspaceState.Status != StatusDeployed {
			t.Fatalf("expected status %s, got %s", StatusDeployed, workspaceState.Status)
		}

		originalDeployTime := workspaceState.LastDeployed
		if originalDeployTime == nil {
			t.Fatal("expected LastDeployed to be set after deployment")
		}

		// Simulate config change
		configTime := now.Add(time.Minute)
		state.SetWorkspaceConfigModified(testWorkspace, configTime)
		workspaceState = state.GetWorkspaceState(testWorkspace)

		// Should reset to destroyed for redeployment
		if workspaceState.Status != StatusDestroyed {
			t.Errorf("expected status %s after config change, got %s", StatusDestroyed, workspaceState.Status)
		}

		// Should clear LastDeployed to ensure redeployment
		if workspaceState.LastDeployed != nil {
			t.Errorf("expected LastDeployed to be nil after config change, got %v", workspaceState.LastDeployed)
		}

		// Should be able to deploy again
		shouldDeploy := scheduler.ShouldRunDeploySchedule(schedules, now.Add(2*time.Minute), workspaceState)
		if !shouldDeploy {
			t.Error("expected to deploy after config change on deployed workspace")
		}
	})

	t.Run("destroyed workspace unaffected by config change", func(t *testing.T) {
		// Reset state for new test
		state = NewState()

		// Workspace starts as destroyed (default)
		workspaceState := state.GetWorkspaceState(testWorkspace)
		if workspaceState.Status != StatusDestroyed {
			t.Fatalf("expected initial status %s, got %s", StatusDestroyed, workspaceState.Status)
		}

		// Simulate config change
		configTime := now.Add(time.Minute)
		state.SetWorkspaceConfigModified(testWorkspace, configTime)
		workspaceState = state.GetWorkspaceState(testWorkspace)

		// Should remain destroyed (no change needed)
		if workspaceState.Status != StatusDestroyed {
			t.Errorf("expected status to remain %s, got %s", StatusDestroyed, workspaceState.Status)
		}

		// Config modification time should still be recorded
		if workspaceState.LastConfigModified == nil {
			t.Error("expected LastConfigModified to be set")
		}
	})
}
