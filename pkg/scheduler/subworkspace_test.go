package scheduler

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"provisioner/pkg/job"
	"provisioner/pkg/opentofu"
	"provisioner/pkg/workspace"
)

func TestSubWorkspaceDeploymentTrigger(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "subworkspace-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create workspace directory with main infrastructure and sub-workspace jobs
	workspacesDir := filepath.Join(tempDir, "workspaces")
	workspaceName := "main-infrastructure"
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

	// Create temporary templates directory for mock templates (set up early so paths work)
	tempTemplatesDir := filepath.Join(tempDir, "state", "templates")
	if err := os.MkdirAll(tempTemplatesDir, 0755); err != nil {
		t.Fatalf("failed to create temp templates directory: %v", err)
	}

	// Create database template
	databaseTemplateDir := filepath.Join(tempTemplatesDir, "database")
	if err := os.MkdirAll(databaseTemplateDir, 0755); err != nil {
		t.Fatalf("failed to create database template directory: %v", err)
	}
	databaseTemplate := `resource "null_resource" "database" {
  provisioner "local-exec" {
    command = "echo 'Database deployed'"
  }
}`
	if err := os.WriteFile(filepath.Join(databaseTemplateDir, "main.tf"), []byte(databaseTemplate), 0644); err != nil {
		t.Fatalf("failed to write database template: %v", err)
	}

	// Create monitoring template
	monitoringTemplateDir := filepath.Join(tempTemplatesDir, "monitoring")
	if err := os.MkdirAll(monitoringTemplateDir, 0755); err != nil {
		t.Fatalf("failed to create monitoring template directory: %v", err)
	}
	monitoringTemplate := `resource "null_resource" "monitoring" {
  provisioner "local-exec" {
    command = "echo 'Monitoring deployed'"
  }
}`
	if err := os.WriteFile(filepath.Join(monitoringTemplateDir, "main.tf"), []byte(monitoringTemplate), 0644); err != nil {
		t.Fatalf("failed to write monitoring template: %v", err)
	}

	// Create workspace config with @deployment triggered sub-workspace jobs
	config := workspace.Config{
		Enabled:     true,
		Description: "Main infrastructure with sub-workspaces",
		Jobs: []workspace.JobConfig{
			{
				Name:        "database-cluster",
				Type:        "template",
				Schedule:    "@deployment",
				Template:    "database",
				Timeout:     "10m",
				Enabled:     true,
				Description: "Deploy database cluster after main infrastructure",
			},
			{
				Name:        "monitoring-stack",
				Type:        "template",
				Schedule:    "@deployment",
				Template:    "monitoring",
				Timeout:     "5m",
				Enabled:     true,
				Description: "Deploy monitoring stack after main infrastructure",
			},
			{
				Name:        "regular-backup",
				Type:        "script",
				Schedule:    "0 2 * * *", // Regular CRON schedule
				Script:      "#!/bin/bash\necho 'Running backup'",
				Timeout:     "30m",
				Enabled:     true,
				Description: "Regular nightly backup",
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

	// Create mock client to track deployments
	mockClient := opentofu.NewMockTofuClient()
	mockClient.SetDeploySuccess() // Successful deployment

	// Create state directory structure
	stateDir := filepath.Join(tempDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create state directory: %v", err)
	}

	// Create deployment directory for the workspace
	deploymentDir := filepath.Join(stateDir, "deployments", workspaceName)
	if err := os.MkdirAll(deploymentDir, 0755); err != nil {
		t.Fatalf("failed to create deployment directory: %v", err)
	}

	// Set state directory environment variable
	originalStateDir := os.Getenv("PROVISIONER_STATE_DIR")
	os.Setenv("PROVISIONER_STATE_DIR", stateDir)
	defer os.Setenv("PROVISIONER_STATE_DIR", originalStateDir)

	// Create job state file
	jobStatePath := filepath.Join(stateDir, "jobs.json")
	if err := os.WriteFile(jobStatePath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create job state file: %v", err)
	}

	// Create scheduler with mock client
	sched := NewWithClient(mockClient)
	sched.configDir = tempDir
	sched.statePath = filepath.Join(stateDir, "scheduler.json") // Use state dir for state

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("failed to load workspaces: %v", err)
	}

	if err := sched.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Force job manager initialization and state loading
	if sched.jobManager != nil {
		if err := sched.jobManager.LoadState(); err != nil {
			t.Logf("Failed to load job manager state: %v", err)
		}
	}

	// Manually deploy the main workspace
	err = sched.ManualDeploy(workspaceName)
	if err != nil {
		t.Fatalf("failed to manually deploy workspace: %v", err)
	}

	// Verify that deployment was successful
	workspaceState := sched.state.GetWorkspaceState(workspaceName)
	if workspaceState.Status != StatusDeployed {
		t.Errorf("expected workspace status %s, got %s", StatusDeployed, workspaceState.Status)
	}

	// Give jobs more time to process the deployment event and complete
	time.Sleep(500 * time.Millisecond)

	// Force job manager to load any saved state
	if sched.jobManager != nil {
		err = sched.jobManager.LoadState()
		if err != nil {
			t.Logf("Failed to load job state: %v", err)
		}
	}

	// Check job states to verify that @deployment jobs were triggered
	if sched.jobManager != nil {
		jobStates := sched.jobManager.GetAllJobStates(workspaceName)

		// Check database-cluster job
		if dbJobState, exists := jobStates["database-cluster"]; exists {
			if dbJobState.Status != job.JobStatusSuccess && dbJobState.Status != job.JobStatusRunning {
				t.Errorf("expected database-cluster job to be triggered, got status: %s", dbJobState.Status)
			}
		} else {
			t.Error("database-cluster job state not found")
		}

		// Check monitoring-stack job
		if monJobState, exists := jobStates["monitoring-stack"]; exists {
			if monJobState.Status != job.JobStatusSuccess && monJobState.Status != job.JobStatusRunning {
				t.Errorf("expected monitoring-stack job to be triggered, got status: %s", monJobState.Status)
			}
		} else {
			t.Error("monitoring-stack job state not found")
		}

		// Check that regular backup job was NOT triggered (it's time-based, not event-based)
		if backupJobState, exists := jobStates["regular-backup"]; exists {
			if backupJobState.Status == job.JobStatusSuccess || backupJobState.Status == job.JobStatusRunning {
				t.Error("regular backup job should not have been triggered by deployment event")
			}
		}
	} else {
		t.Error("job manager is nil")
	}
}

func TestSubWorkspaceFailedDeploymentTrigger(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "subworkspace-fail-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create workspace with @deployment-failed job
	workspacesDir := filepath.Join(tempDir, "workspaces")
	workspaceName := "failing-workspace"
	workspaceDir := filepath.Join(workspacesDir, workspaceName)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}

	// Create main.tf file
	mainTFPath := filepath.Join(workspaceDir, "main.tf")
	mainTFContent := `resource "null_resource" "fail" {
  provisioner "local-exec" {
    command = "exit 1"  # This will fail
  }
}`
	if err := os.WriteFile(mainTFPath, []byte(mainTFContent), 0644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	// Create workspace config with @deployment-failed triggered job
	config := workspace.Config{
		Enabled:     true,
		Description: "Workspace that handles deployment failures",
		Jobs: []workspace.JobConfig{
			{
				Name:        "cleanup-after-failure",
				Type:        "script",
				Schedule:    "@deployment-failed",
				Script:      "#!/bin/bash\necho 'Cleaning up after failed deployment'",
				Timeout:     "5m",
				Enabled:     true,
				Description: "Cleanup job triggered on deployment failure",
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

	// Create mock client that will fail deployment
	mockClient := opentofu.NewMockTofuClient()
	mockClient.SetDeployError(errors.New("deployment failed"))

	// Create state directory structure
	stateDir := filepath.Join(tempDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create state directory: %v", err)
	}

	// Create deployment directory for the workspace
	deploymentDir := filepath.Join(stateDir, "deployments", workspaceName)
	if err := os.MkdirAll(deploymentDir, 0755); err != nil {
		t.Fatalf("failed to create deployment directory: %v", err)
	}

	// Set state directory environment variable
	originalStateDir := os.Getenv("PROVISIONER_STATE_DIR")
	os.Setenv("PROVISIONER_STATE_DIR", stateDir)
	defer os.Setenv("PROVISIONER_STATE_DIR", originalStateDir)

	// Create job state file
	jobStatePath := filepath.Join(stateDir, "jobs.json")
	if err := os.WriteFile(jobStatePath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create job state file: %v", err)
	}

	// Create scheduler with mock client
	sched := NewWithClient(mockClient)
	sched.configDir = tempDir
	sched.statePath = filepath.Join(stateDir, "scheduler.json") // Use state dir for state

	// Load workspaces and state
	if err := sched.LoadWorkspaces(); err != nil {
		t.Fatalf("failed to load workspaces: %v", err)
	}

	if err := sched.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Force job manager initialization and state loading
	if sched.jobManager != nil {
		if err := sched.jobManager.LoadState(); err != nil {
			t.Logf("Failed to load job manager state: %v", err)
		}
	}

	// Manually deploy the workspace (which will fail)
	err = sched.ManualDeploy(workspaceName)
	if err != nil {
		t.Fatalf("manual deploy should not return error even when deployment fails: %v", err)
	}

	// Verify that deployment failed
	workspaceState := sched.state.GetWorkspaceState(workspaceName)
	if workspaceState.Status != StatusDeployFailed {
		t.Errorf("expected workspace status %s, got %s", StatusDeployFailed, workspaceState.Status)
	}

	// Give jobs more time to process the deployment-failed event
	time.Sleep(500 * time.Millisecond)

	// Force job manager to load any saved state
	if sched.jobManager != nil {
		err = sched.jobManager.LoadState()
		if err != nil {
			t.Logf("Failed to load job state: %v", err)
		}
	}

	// Check that the cleanup job was triggered
	if sched.jobManager != nil {
		jobStates := sched.jobManager.GetAllJobStates(workspaceName)

		if cleanupJobState, exists := jobStates["cleanup-after-failure"]; exists {
			if cleanupJobState.Status != job.JobStatusSuccess && cleanupJobState.Status != job.JobStatusRunning {
				t.Errorf("expected cleanup job to be triggered, got status: %s", cleanupJobState.Status)
			}
		} else {
			t.Error("cleanup-after-failure job state not found")
		}
	} else {
		t.Error("job manager is nil")
	}
}

func TestSpecialScheduleParsing(t *testing.T) {
	testCases := []struct {
		schedule string
		valid    bool
	}{
		{"@deployment", true},
		{"@deployment-failed", true},
		{"@destroy", true},
		{"@destroy-failed", true},
		{"@reboot", true},
		{"@invalid", false},
		{"0 9 * * 1-5", true},  // Regular CRON
		{"*/15 * * * *", true}, // Regular CRON
	}

	for _, tc := range testCases {
		_, err := ParseCron(tc.schedule)
		if tc.valid && err != nil {
			t.Errorf("expected schedule '%s' to be valid, got error: %v", tc.schedule, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("expected schedule '%s' to be invalid, but it was accepted", tc.schedule)
		}
	}
}
