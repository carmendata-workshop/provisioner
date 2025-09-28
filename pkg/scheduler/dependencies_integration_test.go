package scheduler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"provisioner/pkg/opentofu"
	"provisioner/pkg/workspace"
)

func TestSubWorkspaceWithDependencies(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "dependency-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create workspace with dependency chain: foundation → database → app → monitoring
	workspacesDir := filepath.Join(tempDir, "workspaces")
	workspaceName := "layered-infrastructure"
	workspaceDir := filepath.Join(workspacesDir, workspaceName)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}

	// Create main.tf file
	mainTFPath := filepath.Join(workspaceDir, "main.tf")
	mainTFContent := `resource "null_resource" "main" {
  provisioner "local-exec" {
    command = "echo 'Main infrastructure deployed'"
  }
}`
	if err := os.WriteFile(mainTFPath, []byte(mainTFContent), 0644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	// Create workspace config with dependency chain
	config := workspace.Config{
		Enabled:     true,
		Description: "Layered infrastructure with job dependencies",
		Jobs: []workspace.JobConfig{
			{
				Name:        "foundation",
				Type:        "template",
				Schedule:    "@deployment",
				Template:    "network-foundation",
				Timeout:     "10m",
				Enabled:     true,
				Description: "Deploy network foundation (no dependencies)",
			},
			{
				Name:        "database",
				Type:        "template",
				Schedule:    "@deployment",
				Template:    "postgres-cluster",
				DependsOn:   []string{"foundation"},
				Timeout:     "15m",
				Enabled:     true,
				Description: "Deploy database after foundation",
			},
			{
				Name:        "app",
				Type:        "template",
				Schedule:    "@deployment",
				Template:    "web-application",
				DependsOn:   []string{"database"},
				Timeout:     "10m",
				Enabled:     true,
				Description: "Deploy application after database",
			},
			{
				Name:        "monitoring",
				Type:        "template",
				Schedule:    "@deployment",
				Template:    "prometheus-grafana",
				DependsOn:   []string{"app"},
				Timeout:     "5m",
				Enabled:     true,
				Description: "Deploy monitoring after application",
			},
			{
				Name:        "backup",
				Type:        "script",
				Schedule:    "0 2 * * *", // Regular CRON schedule, not event-based
				Script:      "#!/bin/bash\necho 'Running backup'",
				Timeout:     "30m",
				Enabled:     true,
				Description: "Regular backup (should not be triggered by deployment)",
			},
		},
	}

	// Write config manually
	configPath := filepath.Join(workspaceDir, "config.json")
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create mock client that succeeds
	mockClient := opentofu.NewMockTofuClient()
	mockClient.SetDeploySuccess()

	// Create scheduler with mock client
	sched := NewWithClient(mockClient)
	sched.configDir = tempDir
	sched.statePath = filepath.Join(tempDir, "scheduler.json")

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("failed to load workspaces: %v", err)
	}

	if err := sched.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Load job manager state
	if err := sched.jobManager.LoadState(); err != nil {
		t.Fatalf("failed to load job state: %v", err)
	}

	// Manually deploy the main workspace to trigger the dependency chain
	err = sched.ManualDeploy(workspaceName)
	if err != nil {
		t.Fatalf("failed to manually deploy workspace: %v", err)
	}

	// Verify that deployment was successful
	workspaceState := sched.state.GetWorkspaceState(workspaceName)
	if workspaceState.Status != StatusDeployed {
		t.Errorf("expected workspace status %s, got %s", StatusDeployed, workspaceState.Status)
	}

	// Give dependency chain some time to process
	time.Sleep(200 * time.Millisecond)

	// Check that jobs were triggered (though they may not complete due to missing templates)
	if sched.jobManager != nil {
		jobStates := sched.jobManager.GetAllJobStates(workspaceName)

		// Foundation should be triggered first
		if foundationState, exists := jobStates["foundation"]; exists {
			if foundationState.Status == "" {
				t.Error("foundation job should have been triggered")
			}
		} else {
			t.Error("foundation job state not found")
		}

		// Backup job should NOT be triggered (it's time-based, not event-based)
		if backupState, exists := jobStates["backup"]; exists {
			if backupState.RunCount > 0 {
				t.Error("backup job should not have been triggered by deployment event")
			}
		}

		t.Logf("Job states after deployment:")
		for name, state := range jobStates {
			if state != nil {
				t.Logf("  %s: status=%s, runCount=%d", name, state.Status, state.RunCount)
			}
		}
	} else {
		t.Error("job manager is nil")
	}
}

func TestCircularDependencyDetection(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "circular-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create workspace with circular dependencies
	workspacesDir := filepath.Join(tempDir, "workspaces")
	workspaceName := "circular-deps"
	workspaceDir := filepath.Join(workspacesDir, workspaceName)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}

	// Create main.tf file
	mainTFPath := filepath.Join(workspaceDir, "main.tf")
	mainTFContent := `resource "null_resource" "main" {}`
	if err := os.WriteFile(mainTFPath, []byte(mainTFContent), 0644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	// Create workspace config with circular dependencies
	config := workspace.Config{
		Enabled:     true,
		Description: "Workspace with circular dependencies (should fail)",
		Jobs: []workspace.JobConfig{
			{
				Name:      "job1",
				Type:      "script",
				Schedule:  "@deployment",
				Script:    "echo 'job1'",
				DependsOn: []string{"job2"},
				Enabled:   true,
			},
			{
				Name:      "job2",
				Type:      "script",
				Schedule:  "@deployment",
				Script:    "echo 'job2'",
				DependsOn: []string{"job1"},
				Enabled:   true,
			},
		},
	}

	// Write config manually
	configPath := filepath.Join(workspaceDir, "config.json")
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create mock client that succeeds
	mockClient := opentofu.NewMockTofuClient()
	mockClient.SetDeploySuccess()

	// Create scheduler with mock client
	sched := NewWithClient(mockClient)
	sched.configDir = tempDir
	sched.statePath = filepath.Join(tempDir, "scheduler.json")

	// Load workspaces - this should fail due to circular dependencies
	err = sched.LoadWorkspaces()
	if err == nil {
		t.Fatalf("expected LoadWorkspaces to fail due to circular dependencies, but it succeeded")
	}

	// Check that the error message mentions circular dependency
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("expected error to mention circular dependency, got: %v", err)
	}

	t.Logf("Successfully detected circular dependency: %v", err)
}