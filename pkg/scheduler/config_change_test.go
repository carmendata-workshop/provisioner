package scheduler

import (
	"testing"
	"time"
)

func TestConfigChangeHandling(t *testing.T) {
	state := NewState()
	scheduler := &Scheduler{state: state}

	testEnv := "test-config-change"
	now := time.Now()
	schedules := []string{"* * * * *"} // Every minute

	t.Run("failed environment resets on config change", func(t *testing.T) {
		// Set environment to failed state
		state.SetEnvironmentError(testEnv, true, "deployment failed")
		envState := state.GetEnvironmentState(testEnv)

		if envState.Status != StatusDeployFailed {
			t.Fatalf("expected status %s, got %s", StatusDeployFailed, envState.Status)
		}

		// Simulate config change
		configTime := now.Add(time.Minute)
		state.SetEnvironmentConfigModified(testEnv, configTime)
		envState = state.GetEnvironmentState(testEnv)

		// Should reset to destroyed and clear error
		if envState.Status != StatusDestroyed {
			t.Errorf("expected status %s after config change, got %s", StatusDestroyed, envState.Status)
		}

		if envState.LastDeployError != "" {
			t.Errorf("expected empty error after config change, got '%s'", envState.LastDeployError)
		}

		// Should be able to deploy again
		shouldDeploy := scheduler.ShouldRunDeploySchedule(schedules, now.Add(2*time.Minute), envState)
		if !shouldDeploy {
			t.Error("expected to deploy after config change on failed environment")
		}
	})

	t.Run("deployed environment resets on config change", func(t *testing.T) {
		// Reset state for new test
		state = NewState()
		scheduler = &Scheduler{state: state}

		// Set environment to deployed state
		state.SetEnvironmentStatus(testEnv, StatusDeployed)
		envState := state.GetEnvironmentState(testEnv)

		if envState.Status != StatusDeployed {
			t.Fatalf("expected status %s, got %s", StatusDeployed, envState.Status)
		}

		originalDeployTime := envState.LastDeployed
		if originalDeployTime == nil {
			t.Fatal("expected LastDeployed to be set after deployment")
		}

		// Simulate config change
		configTime := now.Add(time.Minute)
		state.SetEnvironmentConfigModified(testEnv, configTime)
		envState = state.GetEnvironmentState(testEnv)

		// Should reset to destroyed for redeployment
		if envState.Status != StatusDestroyed {
			t.Errorf("expected status %s after config change, got %s", StatusDestroyed, envState.Status)
		}

		// Should clear LastDeployed to ensure redeployment
		if envState.LastDeployed != nil {
			t.Errorf("expected LastDeployed to be nil after config change, got %v", envState.LastDeployed)
		}

		// Should be able to deploy again
		shouldDeploy := scheduler.ShouldRunDeploySchedule(schedules, now.Add(2*time.Minute), envState)
		if !shouldDeploy {
			t.Error("expected to deploy after config change on deployed environment")
		}
	})

	t.Run("destroyed environment unaffected by config change", func(t *testing.T) {
		// Reset state for new test
		state = NewState()

		// Environment starts as destroyed (default)
		envState := state.GetEnvironmentState(testEnv)
		if envState.Status != StatusDestroyed {
			t.Fatalf("expected initial status %s, got %s", StatusDestroyed, envState.Status)
		}

		// Simulate config change
		configTime := now.Add(time.Minute)
		state.SetEnvironmentConfigModified(testEnv, configTime)
		envState = state.GetEnvironmentState(testEnv)

		// Should remain destroyed (no change needed)
		if envState.Status != StatusDestroyed {
			t.Errorf("expected status to remain %s, got %s", StatusDestroyed, envState.Status)
		}

		// Config modification time should still be recorded
		if envState.LastConfigModified == nil {
			t.Error("expected LastConfigModified to be set")
		}
	})
}