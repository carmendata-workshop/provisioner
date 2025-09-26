package scheduler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewState(t *testing.T) {
	state := NewState()

	if state.Workspaces == nil {
		t.Error("expected Workspaces map to be initialized")
	}

	if len(state.Workspaces) != 0 {
		t.Errorf("expected empty Workspaces map, got %d entries", len(state.Workspaces))
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

	if len(state.Workspaces) != 0 {
		t.Errorf("expected empty state, got %d workspaces", len(state.Workspaces))
	}
}

func TestSaveAndLoadState(t *testing.T) {
	// Create temporary file
	tempFile, err := os.CreateTemp("", "state-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	_ = tempFile.Close()
	defer func() { _ = os.Remove(tempFile.Name()) }()

	// Create state with test data
	state := NewState()
	state.SetWorkspaceStatus("test-workspace", StatusDeployed)

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
	if len(loadedState.Workspaces) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(loadedState.Workspaces))
	}

	workspace := loadedState.GetWorkspaceState("test-workspace")
	if workspace.Status != StatusDeployed {
		t.Errorf("expected status %s, got %s", StatusDeployed, workspace.Status)
	}
}

func TestGetWorkspaceState(t *testing.T) {
	state := NewState()

	// Get non-existent workspace (should create new)
	workspace := state.GetWorkspaceState("new-workspace")
	if workspace == nil {
		t.Fatal("expected workspace state to be created")
	}

	if workspace.Name != "new-workspace" {
		t.Errorf("expected name 'new-workspace', got '%s'", workspace.Name)
	}

	if workspace.Status != StatusDestroyed {
		t.Errorf("expected initial status %s, got %s", StatusDestroyed, workspace.Status)
	}

	// Get existing workspace
	workspace2 := state.GetWorkspaceState("new-workspace")
	if workspace != workspace2 {
		t.Error("expected same workspace instance")
	}
}

func TestSetWorkspaceStatus(t *testing.T) {
	state := NewState()

	// Test deployed status
	state.SetWorkspaceStatus("test-workspace", StatusDeployed)
	workspace := state.GetWorkspaceState("test-workspace")

	if workspace.Status != StatusDeployed {
		t.Errorf("expected status %s, got %s", StatusDeployed, workspace.Status)
	}

	if workspace.LastDeployed == nil {
		t.Error("expected LastDeployed to be set")
	}

	if workspace.LastDeployError != "" {
		t.Errorf("expected LastDeployError to be cleared, got '%s'", workspace.LastDeployError)
	}

	// Test destroyed status
	state.SetWorkspaceStatus("test-workspace", StatusDestroyed)
	workspace = state.GetWorkspaceState("test-workspace")

	if workspace.Status != StatusDestroyed {
		t.Errorf("expected status %s, got %s", StatusDestroyed, workspace.Status)
	}

	if workspace.LastDestroyed == nil {
		t.Error("expected LastDestroyed to be set")
	}

	if workspace.LastDestroyError != "" {
		t.Errorf("expected LastDestroyError to be cleared, got '%s'", workspace.LastDestroyError)
	}
}

func TestSetWorkspaceError(t *testing.T) {
	state := NewState()

	// Test deploy error
	state.SetWorkspaceError("test-workspace", true, "deploy failed")
	workspace := state.GetWorkspaceState("test-workspace")

	if workspace.LastDeployError != "deploy failed" {
		t.Errorf("expected deploy error 'deploy failed', got '%s'", workspace.LastDeployError)
	}

	if workspace.Status != StatusDeployFailed {
		t.Errorf("expected status %s after deploy error, got %s", StatusDeployFailed, workspace.Status)
	}

	// Test destroy error
	state.SetWorkspaceError("test-workspace", false, "destroy failed")
	workspace = state.GetWorkspaceState("test-workspace")

	if workspace.LastDestroyError != "destroy failed" {
		t.Errorf("expected destroy error 'destroy failed', got '%s'", workspace.LastDestroyError)
	}

	if workspace.Status != StatusDestroyFailed {
		t.Errorf("expected status %s after destroy error, got %s", StatusDestroyFailed, workspace.Status)
	}
}

func TestSaveStateCreatesDirectory(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "state-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create state file path in non-existent subdirectory
	statePath := filepath.Join(tempDir, "subdir", "state.json")

	state := NewState()
	state.SetWorkspaceStatus("test", StatusDeployed)

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
