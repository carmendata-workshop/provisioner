package scheduler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewState(t *testing.T) {
	state := NewState()

	if state.Environments == nil {
		t.Error("expected Environments map to be initialized")
	}

	if len(state.Environments) != 0 {
		t.Errorf("expected empty Environments map, got %d entries", len(state.Environments))
	}

	if state.LastUpdated.IsZero() {
		t.Error("expected LastUpdated to be set")
	}
}

func TestLoadStateMissingFile(t *testing.T) {
	// Try to load non-existent file
	state, err := LoadState("/tmp/non-existent-file.json")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}

	if state == nil {
		t.Fatal("expected state to be created")
	}

	if len(state.Environments) != 0 {
		t.Errorf("expected empty state, got %d environments", len(state.Environments))
	}
}

func TestSaveAndLoadState(t *testing.T) {
	// Create temporary file
	tempFile, err := os.CreateTemp("", "state-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	// Create state with test data
	state := NewState()
	state.SetEnvironmentStatus("test-env", StatusDeployed)

	// Save state
	if err := state.SaveState(tempFile.Name()); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Load state
	loadedState, err := LoadState(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Verify loaded state
	if len(loadedState.Environments) != 1 {
		t.Errorf("expected 1 environment, got %d", len(loadedState.Environments))
	}

	env := loadedState.GetEnvironmentState("test-env")
	if env.Status != StatusDeployed {
		t.Errorf("expected status %s, got %s", StatusDeployed, env.Status)
	}
}

func TestGetEnvironmentState(t *testing.T) {
	state := NewState()

	// Get non-existent environment (should create new)
	env := state.GetEnvironmentState("new-env")
	if env == nil {
		t.Fatal("expected environment state to be created")
	}

	if env.Name != "new-env" {
		t.Errorf("expected name 'new-env', got '%s'", env.Name)
	}

	if env.Status != StatusDestroyed {
		t.Errorf("expected initial status %s, got %s", StatusDestroyed, env.Status)
	}

	// Get existing environment
	env2 := state.GetEnvironmentState("new-env")
	if env != env2 {
		t.Error("expected same environment instance")
	}
}

func TestSetEnvironmentStatus(t *testing.T) {
	state := NewState()

	// Test deployed status
	state.SetEnvironmentStatus("test-env", StatusDeployed)
	env := state.GetEnvironmentState("test-env")

	if env.Status != StatusDeployed {
		t.Errorf("expected status %s, got %s", StatusDeployed, env.Status)
	}

	if env.LastDeployed == nil {
		t.Error("expected LastDeployed to be set")
	}

	if env.LastDeployError != "" {
		t.Errorf("expected LastDeployError to be cleared, got '%s'", env.LastDeployError)
	}

	// Test destroyed status
	state.SetEnvironmentStatus("test-env", StatusDestroyed)
	env = state.GetEnvironmentState("test-env")

	if env.Status != StatusDestroyed {
		t.Errorf("expected status %s, got %s", StatusDestroyed, env.Status)
	}

	if env.LastDestroyed == nil {
		t.Error("expected LastDestroyed to be set")
	}

	if env.LastDestroyError != "" {
		t.Errorf("expected LastDestroyError to be cleared, got '%s'", env.LastDestroyError)
	}
}

func TestSetEnvironmentError(t *testing.T) {
	state := NewState()

	// Test deploy error
	state.SetEnvironmentError("test-env", true, "deploy failed")
	env := state.GetEnvironmentState("test-env")

	if env.LastDeployError != "deploy failed" {
		t.Errorf("expected deploy error 'deploy failed', got '%s'", env.LastDeployError)
	}

	if env.Status != StatusDestroyed {
		t.Errorf("expected status %s after deploy error, got %s", StatusDestroyed, env.Status)
	}

	// Test destroy error
	state.SetEnvironmentError("test-env", false, "destroy failed")
	env = state.GetEnvironmentState("test-env")

	if env.LastDestroyError != "destroy failed" {
		t.Errorf("expected destroy error 'destroy failed', got '%s'", env.LastDestroyError)
	}

	if env.Status != StatusDeployed {
		t.Errorf("expected status %s after destroy error, got %s", StatusDeployed, env.Status)
	}
}

func TestSaveStateCreatesDirectory(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "state-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create state file path in non-existent subdirectory
	statePath := filepath.Join(tempDir, "subdir", "state.json")

	state := NewState()
	state.SetEnvironmentStatus("test", StatusDeployed)

	// Save should create directory
	if err := state.SaveState(statePath); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("expected state file to be created")
	}

	// Verify directory was created
	if _, err := os.Stat(filepath.Dir(statePath)); os.IsNotExist(err) {
		t.Error("expected state directory to be created")
	}
}