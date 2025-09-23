package scheduler

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"provisioner/pkg/environment"
	"provisioner/pkg/opentofu"
)

func TestImmediateDeploymentOnConfigChange(t *testing.T) {
	// Create temporary directories for test
	tempDir, err := os.MkdirTemp("", "scheduler-immediate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	stateDir := filepath.Join(tempDir, "state")
	envPath := filepath.Join(tempDir, "test-env")

	// Create mock client to track deployments
	mockClient := opentofu.NewMockTofuClient()
	deploymentTriggered := false

	// Override the client to track deployments
	mockClient.DeployFunc = func(env *environment.Environment) error {
		deploymentTriggered = true
		return nil
	}

	// Create scheduler
	scheduler := NewWithClient(mockClient)
	scheduler.statePath = filepath.Join(stateDir, "scheduler.json")
	scheduler.state = NewState()

	// Create test environment
	testEnv := environment.Environment{
		Name: "test-immediate",
		Config: environment.Config{
			Enabled:         true,
			DeploySchedule:  "* * * * *", // Every minute - should always be deployable
		},
		Path: envPath,
	}

	// Set up environments in scheduler
	scheduler.environments = []environment.Environment{testEnv}

	// Test 1: Environment in destroyed state should deploy immediately on config change
	t.Run("deploy immediately when destroyed", func(t *testing.T) {
		deploymentTriggered = false
		envState := scheduler.state.GetEnvironmentState("test-immediate")

		if envState.Status != StatusDestroyed {
			t.Fatalf("expected initial status %s, got %s", StatusDestroyed, envState.Status)
		}

		// Simulate config change - this should trigger immediate deployment
		now := time.Now()
		scheduler.checkEnvironmentForImmediateDeployment("test-immediate", now)

		// Give goroutine a moment to execute
		time.Sleep(10 * time.Millisecond)

		if !deploymentTriggered {
			t.Error("expected deployment to be triggered immediately on config change")
		}
	})

	// Test 2: Environment in failed state should deploy immediately after config change
	t.Run("deploy immediately when failed", func(t *testing.T) {
		// Reset state
		scheduler.state = NewState()
		deploymentTriggered = false

		// Set environment to failed state
		scheduler.state.SetEnvironmentError("test-immediate", true, "previous failure")
		envState := scheduler.state.GetEnvironmentState("test-immediate")

		if envState.Status != StatusDeployFailed {
			t.Fatalf("expected status %s, got %s", StatusDeployFailed, envState.Status)
		}

		// Simulate config change - this should reset state and trigger deployment
		now := time.Now()
		scheduler.state.SetEnvironmentConfigModified("test-immediate", now)
		scheduler.checkEnvironmentForImmediateDeployment("test-immediate", now)

		// Give goroutine a moment to execute
		time.Sleep(10 * time.Millisecond)

		if !deploymentTriggered {
			t.Error("expected deployment to be triggered immediately after config change on failed environment")
		}
	})

	// Test 3: Environment in deployed state should deploy immediately after config change
	t.Run("redeploy immediately when deployed", func(t *testing.T) {
		// Reset state
		scheduler.state = NewState()
		deploymentTriggered = false

		// Set environment to deployed state
		scheduler.state.SetEnvironmentStatus("test-immediate", StatusDeployed)
		envState := scheduler.state.GetEnvironmentState("test-immediate")

		if envState.Status != StatusDeployed {
			t.Fatalf("expected status %s, got %s", StatusDeployed, envState.Status)
		}

		// Simulate config change - this should reset state and trigger redeployment
		now := time.Now()
		scheduler.state.SetEnvironmentConfigModified("test-immediate", now)
		scheduler.checkEnvironmentForImmediateDeployment("test-immediate", now)

		// Give goroutine a moment to execute
		time.Sleep(10 * time.Millisecond)

		if !deploymentTriggered {
			t.Error("expected redeployment to be triggered immediately after config change on deployed environment")
		}
	})

	// Test 4: Busy environment should not deploy immediately
	t.Run("skip immediate deploy when busy", func(t *testing.T) {
		// Reset state
		scheduler.state = NewState()
		deploymentTriggered = false

		// Set environment to deploying state (busy)
		scheduler.state.SetEnvironmentStatus("test-immediate", StatusDeploying)

		// Simulate config change - this should NOT trigger deployment
		now := time.Now()
		scheduler.checkEnvironmentForImmediateDeployment("test-immediate", now)

		// Give potential goroutine a moment
		time.Sleep(10 * time.Millisecond)

		if deploymentTriggered {
			t.Error("expected NO deployment when environment is busy")
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

	// Environment with specific schedule (9 AM only)
	testEnv := environment.Environment{
		Name: "test-scheduled",
		Config: environment.Config{
			Enabled:        true,
			DeploySchedule: "0 9 * * *", // Only at 9:00 AM
		},
		Path: tempDir,
	}

	scheduler.environments = []environment.Environment{testEnv}

	// Test at 8 AM - should NOT deploy even with config change
	morningTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	deploymentTriggered := false

	mockClient.DeployFunc = func(env *environment.Environment) error {
		deploymentTriggered = true
		return nil
	}

	scheduler.checkEnvironmentForImmediateDeployment("test-scheduled", morningTime)
	time.Sleep(10 * time.Millisecond)

	if deploymentTriggered {
		t.Error("expected NO deployment at 8 AM when schedule is 9 AM")
	}

	// Test at 10 AM (after 9 AM schedule) - should deploy
	laterTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	deploymentTriggered = false

	scheduler.checkEnvironmentForImmediateDeployment("test-scheduled", laterTime)
	time.Sleep(10 * time.Millisecond)

	if !deploymentTriggered {
		t.Error("expected deployment at 10 AM when 9 AM schedule has passed")
	}
}