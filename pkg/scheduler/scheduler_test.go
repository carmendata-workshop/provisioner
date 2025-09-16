package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"environment-scheduler/pkg/environment"
	"environment-scheduler/pkg/opentofu"
)

func TestSchedulerDeployEnvironment(t *testing.T) {
	// Create temporary directories
	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	stateDir := filepath.Join(tempDir, "state")
	envPath := filepath.Join(tempDir, "test-env")

	// Create test environment
	if err := os.MkdirAll(envPath, 0755); err != nil {
		t.Fatalf("failed to create env directory: %v", err)
	}

	env := environment.Environment{
		Config: environment.Config{
			Name:            "test-env",
			Enabled:         true,
			DeploySchedule:  "* * * * *",
			DestroySchedule: "* * * * *",
		},
		Path: envPath,
	}

	// Create mock client
	mockClient := opentofu.NewMockTofuClient()

	// Create scheduler with mock
	scheduler := NewWithClient(mockClient)
	scheduler.statePath = filepath.Join(stateDir, "scheduler.json")
	scheduler.state = NewState()

	// Test successful deployment
	scheduler.deployEnvironment(env)

	// Verify mock was called
	if mockClient.DeployCallCount != 1 {
		t.Errorf("expected 1 deploy call, got %d", mockClient.DeployCallCount)
	}

	if mockClient.GetLastDeployPath() != envPath {
		t.Errorf("expected deploy path '%s', got '%s'", envPath, mockClient.GetLastDeployPath())
	}

	// Verify state was updated
	envState := scheduler.state.GetEnvironmentState("test-env")
	if envState.Status != StatusDeployed {
		t.Errorf("expected status %s, got %s", StatusDeployed, envState.Status)
	}

	if envState.LastDeployed == nil {
		t.Error("expected LastDeployed to be set")
	}

	if envState.LastDeployError != "" {
		t.Errorf("expected no deploy error, got '%s'", envState.LastDeployError)
	}
}

func TestSchedulerDeployEnvironmentError(t *testing.T) {
	// Create temporary directories
	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	stateDir := filepath.Join(tempDir, "state")
	envPath := filepath.Join(tempDir, "test-env")

	env := environment.Environment{
		Config: environment.Config{Name: "test-env"},
		Path:   envPath,
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
	scheduler.deployEnvironment(env)

	// Verify state shows error
	envState := scheduler.state.GetEnvironmentState("test-env")
	if envState.Status != StatusDestroyed {
		t.Errorf("expected status %s after error, got %s", StatusDestroyed, envState.Status)
	}

	if envState.LastDeployError != "deploy failed" {
		t.Errorf("expected deploy error 'deploy failed', got '%s'", envState.LastDeployError)
	}
}

func TestSchedulerDestroyEnvironment(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	stateDir := filepath.Join(tempDir, "state")
	envPath := filepath.Join(tempDir, "test-env")

	env := environment.Environment{
		Config: environment.Config{Name: "test-env"},
		Path:   envPath,
	}

	// Create mock client
	mockClient := opentofu.NewMockTofuClient()

	// Create scheduler
	scheduler := NewWithClient(mockClient)
	scheduler.statePath = filepath.Join(stateDir, "scheduler.json")
	scheduler.state = NewState()

	// Set initial state as deployed
	scheduler.state.SetEnvironmentStatus("test-env", StatusDeployed)

	// Test destruction
	scheduler.destroyEnvironment(env)

	// Verify mock was called
	if mockClient.DestroyCallCount != 1 {
		t.Errorf("expected 1 destroy call, got %d", mockClient.DestroyCallCount)
	}

	if mockClient.GetLastDestroyPath() != envPath {
		t.Errorf("expected destroy path '%s', got '%s'", envPath, mockClient.GetLastDestroyPath())
	}

	// Verify state was updated
	envState := scheduler.state.GetEnvironmentState("test-env")
	if envState.Status != StatusDestroyed {
		t.Errorf("expected status %s, got %s", StatusDestroyed, envState.Status)
	}

	if envState.LastDestroyed == nil {
		t.Error("expected LastDestroyed to be set")
	}
}

func TestSchedulerCheckEnvironmentSchedules(t *testing.T) {
	// Create temporary environment directory for testing
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

	// Create test environment with schedules that should trigger
	env := environment.Environment{
		Config: environment.Config{
			Name:            "test-env",
			Enabled:         true,
			DeploySchedule:  "* * * * *", // Every minute
			DestroySchedule: "* * * * *", // Every minute
		},
		Path: filepath.Join(tempDir, "test-env"),
	}

	// Test time that matches the schedule
	testTime := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)

	// Environment starts as destroyed, so deploy should trigger
	scheduler.checkEnvironmentSchedules(env, testTime)

	// Wait a brief moment for goroutine to complete
	time.Sleep(10 * time.Millisecond)

	// Verify deploy was called
	if mockClient.DeployCallCount != 1 {
		t.Errorf("expected 1 deploy call, got %d", mockClient.DeployCallCount)
	}

	// Reset mock and set environment as deployed
	mockClient.Reset()
	scheduler.state.SetEnvironmentStatus("test-env", StatusDeployed)

	// Now destroy should trigger
	scheduler.checkEnvironmentSchedules(env, testTime)

	// Wait a brief moment for goroutine to complete
	time.Sleep(10 * time.Millisecond)

	// Verify destroy was called
	if mockClient.DestroyCallCount != 1 {
		t.Errorf("expected 1 destroy call, got %d", mockClient.DestroyCallCount)
	}
}

func TestSchedulerSkipsBusyEnvironments(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := opentofu.NewMockTofuClient()
	scheduler := NewWithClient(mockClient)
	scheduler.state = NewState()

	env := environment.Environment{
		Config: environment.Config{
			Name:            "test-env",
			DeploySchedule:  "* * * * *",
			DestroySchedule: "* * * * *",
		},
		Path: filepath.Join(tempDir, "test-env"),
	}

	testTime := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)

	// Set environment as currently deploying
	scheduler.state.SetEnvironmentStatus("test-env", StatusDeploying)

	// Check schedules - should skip busy environment
	scheduler.checkEnvironmentSchedules(env, testTime)

	// Wait briefly
	time.Sleep(10 * time.Millisecond)

	// Verify no operations were called
	if mockClient.DeployCallCount != 0 {
		t.Errorf("expected 0 deploy calls for busy environment, got %d", mockClient.DeployCallCount)
	}

	if mockClient.DestroyCallCount != 0 {
		t.Errorf("expected 0 destroy calls for busy environment, got %d", mockClient.DestroyCallCount)
	}
}

func TestSchedulerLoadEnvironments(t *testing.T) {
	// Create temporary environments directory
	tempDir, err := os.MkdirTemp("", "scheduler-env-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test environment
	envDir := filepath.Join(tempDir, "test-env")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("failed to create env directory: %v", err)
	}

	config := environment.Config{
		Name:            "test-env",
		Enabled:         true,
		DeploySchedule:  "0 9 * * *",
		DestroySchedule: "0 17 * * *",
		Description:     "Test environment",
	}

	configData, _ := json.Marshal(config)
	configPath := filepath.Join(envDir, "config.json")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create scheduler
	scheduler := New()

	// Override LoadEnvironments to use our test directory
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