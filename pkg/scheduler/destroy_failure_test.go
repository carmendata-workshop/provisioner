package scheduler

import (
	"testing"
	"time"
)

func TestDestroyFailureHandling(t *testing.T) {
	// Test that failed destroy operations don't retry until config changes
	state := NewState()
	scheduler := &Scheduler{state: state}

	testEnv := "test-destroy-fail"
	now := time.Now()

	// Create environment in deployed state
	state.SetEnvironmentStatus(testEnv, StatusDeployed)
	envState := state.GetEnvironmentState(testEnv)
	if envState.Status != StatusDeployed {
		t.Fatalf("expected initial status %s, got %s", StatusDeployed, envState.Status)
	}

	// Should destroy initially when deployed
	schedules := []string{"* * * * *"} // Every minute
	shouldDestroy := scheduler.ShouldRunDestroySchedule(schedules, now, envState)
	if !shouldDestroy {
		t.Error("expected to destroy initially when status is deployed")
	}

	// Simulate destroy failure
	state.SetEnvironmentError(testEnv, false, "destroy failed")
	envState = state.GetEnvironmentState(testEnv)

	if envState.Status != StatusDestroyFailed {
		t.Errorf("expected status %s after destroy error, got %s", StatusDestroyFailed, envState.Status)
	}

	if envState.LastDestroyError != "destroy failed" {
		t.Errorf("expected error 'destroy failed', got '%s'", envState.LastDestroyError)
	}

	// Should NOT destroy again while in failed state
	shouldDestroy = scheduler.ShouldRunDestroySchedule(schedules, now.Add(time.Minute), envState)
	if shouldDestroy {
		t.Error("expected NOT to destroy when in destroy failed state")
	}

	// Simulate config change
	configTime := now.Add(2 * time.Minute)
	state.SetEnvironmentConfigModified(testEnv, configTime)
	envState = state.GetEnvironmentState(testEnv)

	// Should reset to deployed state and clear error
	if envState.Status != StatusDeployed {
		t.Errorf("expected status %s after config change, got %s", StatusDeployed, envState.Status)
	}

	if envState.LastDestroyError != "" {
		t.Errorf("expected empty error after config change, got '%s'", envState.LastDestroyError)
	}

	// Should be able to destroy again after config change
	shouldDestroy = scheduler.ShouldRunDestroySchedule(schedules, now.Add(3*time.Minute), envState)
	if !shouldDestroy {
		t.Error("expected to destroy again after config change")
	}
}

func TestDestroyFailureWithMultipleSchedules(t *testing.T) {
	// Test destroy failure handling with multiple destroy schedules
	state := NewState()
	scheduler := &Scheduler{state: state}

	testEnv := "test-destroy-multi"
	now := time.Now()

	// Set environment to deployed state
	state.SetEnvironmentStatus(testEnv, StatusDeployed)
	envState := state.GetEnvironmentState(testEnv)
	schedules := []string{"0 18 * * *", "0 22 * * *"} // 6 PM and 10 PM

	// Set up a time when destruction should happen (after 6 PM)
	testTime := time.Date(now.Year(), now.Month(), now.Day(), 18, 30, 0, 0, now.Location())

	// Should destroy initially
	shouldDestroy := scheduler.ShouldRunDestroySchedule(schedules, testTime, envState)
	if !shouldDestroy {
		t.Error("expected to destroy at 6:30 PM when 6 PM schedule should have run")
	}

	// Simulate failure
	state.SetEnvironmentError(testEnv, false, "destroy failed")
	envState = state.GetEnvironmentState(testEnv)

	// Should NOT destroy again even though 10 PM schedule will come
	testTime = time.Date(now.Year(), now.Month(), now.Day(), 22, 30, 0, 0, now.Location())
	shouldDestroy = scheduler.ShouldRunDestroySchedule(schedules, testTime, envState)
	if shouldDestroy {
		t.Error("expected NOT to destroy at 10:30 PM when in destroy failed state")
	}
}

func TestStateTransitions(t *testing.T) {
	// Test all possible state transitions with config changes
	tests := []struct {
		name           string
		initialStatus  EnvironmentStatus
		configChange   bool
		expectedStatus EnvironmentStatus
	}{
		{"deploy_failed_with_config_change", StatusDeployFailed, true, StatusDestroyed},
		{"destroy_failed_with_config_change", StatusDestroyFailed, true, StatusDeployed},
		{"deployed_with_config_change", StatusDeployed, true, StatusDestroyed},
		{"destroyed_with_config_change", StatusDestroyed, true, StatusDestroyed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewState()
			testEnv := "test-env"

			// Set initial status
			state.SetEnvironmentStatus(testEnv, tt.initialStatus)
			envState := state.GetEnvironmentState(testEnv)

			if tt.configChange {
				state.SetEnvironmentConfigModified(testEnv, time.Now())
				envState = state.GetEnvironmentState(testEnv)
			}

			if envState.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, envState.Status)
			}
		})
	}
}