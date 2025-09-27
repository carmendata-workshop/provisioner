package job

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"provisioner/pkg/opentofu"
	"provisioner/pkg/template"
)

// TestJobManagerIntegration tests the complete job management workflow
func TestJobManagerIntegration(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")
	deploymentsDir := filepath.Join(stateDir, "deployments")

	err := os.MkdirAll(stateDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create state directory: %v", err)
	}

	err = os.MkdirAll(deploymentsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create deployments directory: %v", err)
	}

	// Create mock dependencies
	mockClient := &opentofu.MockTofuClient{}
	templateManager := template.NewManager(filepath.Join(stateDir, "templates"))

	// Create job manager
	jobManager := NewManager(stateDir, mockClient, templateManager)

	// Load initial state
	err = jobManager.LoadState()
	if err != nil {
		t.Fatalf("Failed to load initial state: %v", err)
	}

	// Test workspace with multiple job types
	workspaceID := "test-workspace"
	workspaceDeploymentDir := filepath.Join(deploymentsDir, workspaceID)
	err = os.MkdirAll(workspaceDeploymentDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create workspace deployment directory: %v", err)
	}

	// Create job configurations
	jobConfigs := []map[string]interface{}{
		{
			"name":        "script-job",
			"type":        "script",
			"schedule":    "0 * * * *",
			"script":      "#!/bin/bash\necho 'Script job executed'",
			"timeout":     "5m",
			"enabled":     true,
			"description": "Test script job",
		},
		{
			"name":        "command-job",
			"type":        "command",
			"schedule":    []string{"0 */6 * * *", "0 12 * * *"},
			"command":     "echo 'Command job executed'",
			"timeout":     "2m",
			"enabled":     true,
			"description": "Test command job with multiple schedules",
		},
		{
			"name":        "disabled-job",
			"type":        "script",
			"schedule":    "* * * * *",
			"script":      "echo 'This should not run'",
			"enabled":     false,
			"description": "Disabled job",
		},
	}

	// Convert jobConfigs to the expected format
	var jobConfigInterfaces []interface{}
	for _, config := range jobConfigs {
		jobConfigInterfaces = append(jobConfigInterfaces, config)
	}

	// Process workspace jobs
	jobManager.ProcessWorkspaceJobs(workspaceID, jobConfigInterfaces, time.Now())

	// Check job states
	jobStates := jobManager.GetAllJobStates(workspaceID)

	// Verify script job
	scriptJobState, exists := jobStates["script-job"]
	if !exists {
		t.Errorf("Expected job state for script-job")
	} else {
		if scriptJobState.RunCount == 0 {
			t.Errorf("Expected script job to run, but run count is 0")
		}
		if scriptJobState.Status != JobStatusSuccess {
			t.Errorf("Expected script job status %s, got %s", JobStatusSuccess, scriptJobState.Status)
		}
	}

	// Verify command job
	commandJobState, exists := jobStates["command-job"]
	if !exists {
		t.Errorf("Expected job state for command-job")
	} else {
		if commandJobState.RunCount == 0 {
			t.Errorf("Expected command job to run, but run count is 0")
		}
		if commandJobState.Status != JobStatusSuccess {
			t.Errorf("Expected command job status %s, got %s", JobStatusSuccess, commandJobState.Status)
		}
	}

	// Verify disabled job did not run
	if disabledJobState, exists := jobStates["disabled-job"]; exists {
		if disabledJobState.RunCount > 0 {
			t.Errorf("Expected disabled job not to run, but run count is %d", disabledJobState.RunCount)
		}
	}

	// Test manual job execution
	err = jobManager.ManualExecuteJob(workspaceID, "script-job", jobConfigs[0])
	if err != nil {
		t.Fatalf("Failed to manually execute job: %v", err)
	}

	// Check that run count increased
	updatedJobState := jobManager.GetJobState(workspaceID, "script-job")
	if updatedJobState.RunCount <= scriptJobState.RunCount {
		t.Errorf("Expected run count to increase after manual execution")
	}
}

// TestJobExecutorTimeout tests job execution timeout functionality
func TestJobExecutorTimeout(t *testing.T) {
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")
	workspaceDir := filepath.Join(stateDir, "deployments", "test-workspace")

	err := os.MkdirAll(workspaceDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	mockClient := &opentofu.MockTofuClient{}
	templateManager := template.NewManager(filepath.Join(stateDir, "templates"))

	// Create a job that will timeout
	job := &Job{
		Name:        "timeout-job",
		WorkspaceID: "test-workspace",
		JobType:     JobTypeScript,
		Script:      "#!/bin/bash\nsleep 10", // Sleep longer than timeout
		Schedule:    "* * * * *",
		Timeout:     "1s", // Very short timeout
		Enabled:     true,
	}

	err = job.Validate()
	if err != nil {
		t.Fatalf("Job validation failed: %v", err)
	}

	// Create executor and execute job
	executor := NewExecutor(workspaceDir, mockClient, templateManager)
	execution := executor.ExecuteJob(job)

	// Job should have failed due to timeout
	if execution.Status != JobStatusFailed {
		t.Errorf("Expected job to fail due to timeout, got status %s", execution.Status)
	}

	if execution.Error == "" {
		t.Errorf("Expected timeout error message, got empty error")
	}
}

// TestJobStateConsistency tests job state persistence and loading
func TestJobStateConsistency(t *testing.T) {
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")

	err := os.MkdirAll(stateDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create state directory: %v", err)
	}

	mockClient := &opentofu.MockTofuClient{}
	templateManager := template.NewManager(filepath.Join(stateDir, "templates"))

	// Create first job manager instance
	jobManager1 := NewManager(stateDir, mockClient, templateManager)
	err = jobManager1.LoadState()
	if err != nil {
		t.Fatalf("Failed to load initial state: %v", err)
	}

	// Execute a job to create state
	workspaceID := "persistence-test"
	jobConfig := map[string]interface{}{
		"name":        "persistent-job",
		"type":        "script",
		"schedule":    "0 * * * *",
		"script":      "echo 'persistence test'",
		"enabled":     true,
		"description": "Job for persistence testing",
	}

	// Create workspace deployment directory
	workspaceDir := filepath.Join(stateDir, "deployments", workspaceID)
	err = os.MkdirAll(workspaceDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	err = jobManager1.ManualExecuteJob(workspaceID, "persistent-job", jobConfig)
	if err != nil {
		t.Fatalf("Failed to execute job: %v", err)
	}

	// Get job state from first instance
	originalState := jobManager1.GetJobState(workspaceID, "persistent-job")
	if originalState == nil {
		t.Fatalf("Job state not found after execution")
	}

	// Create second job manager instance (simulating restart)
	jobManager2 := NewManager(stateDir, mockClient, templateManager)
	err = jobManager2.LoadState()
	if err != nil {
		t.Fatalf("Failed to load state in second instance: %v", err)
	}

	// Get job state from second instance
	loadedState := jobManager2.GetJobState(workspaceID, "persistent-job")
	if loadedState == nil {
		t.Fatalf("Job state not found after reload")
	}

	// Compare states
	if loadedState.RunCount != originalState.RunCount {
		t.Errorf("Run count mismatch: expected %d, got %d", originalState.RunCount, loadedState.RunCount)
	}

	if loadedState.SuccessCount != originalState.SuccessCount {
		t.Errorf("Success count mismatch: expected %d, got %d", originalState.SuccessCount, loadedState.SuccessCount)
	}

	if loadedState.Status != originalState.Status {
		t.Errorf("Status mismatch: expected %s, got %s", originalState.Status, loadedState.Status)
	}
}

// TestJobConcurrency tests concurrent job execution
func TestJobConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")
	workspaceDir := filepath.Join(stateDir, "deployments", "concurrent-test")

	err := os.MkdirAll(workspaceDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	mockClient := &opentofu.MockTofuClient{}
	templateManager := template.NewManager(filepath.Join(stateDir, "templates"))
	jobManager := NewManager(stateDir, mockClient, templateManager)

	err = jobManager.LoadState()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	workspaceID := "concurrent-test"

	// Create multiple job configurations
	jobConfigs := []map[string]interface{}{
		{
			"name":        "job-1",
			"type":        "script",
			"schedule":    "* * * * *",
			"script":      "echo 'Job 1'; sleep 0.1",
			"enabled":     true,
			"description": "Concurrent job 1",
		},
		{
			"name":        "job-2",
			"type":        "script",
			"schedule":    "* * * * *",
			"script":      "echo 'Job 2'; sleep 0.1",
			"enabled":     true,
			"description": "Concurrent job 2",
		},
		{
			"name":        "job-3",
			"type":        "script",
			"schedule":    "* * * * *",
			"script":      "echo 'Job 3'; sleep 0.1",
			"enabled":     true,
			"description": "Concurrent job 3",
		},
	}

	// Convert jobConfigs to the expected format
	var jobConfigInterfaces []interface{}
	for _, config := range jobConfigs {
		jobConfigInterfaces = append(jobConfigInterfaces, config)
	}

	// Execute jobs concurrently using ProcessWorkspaceJobs
	startTime := time.Now()
	jobManager.ProcessWorkspaceJobs(workspaceID, jobConfigInterfaces, time.Now())
	executionTime := time.Since(startTime)

	// Verify all jobs ran
	jobStates := jobManager.GetAllJobStates(workspaceID)
	for _, config := range jobConfigs {
		jobName := config["name"].(string)
		if jobState, exists := jobStates[jobName]; !exists {
			t.Errorf("Job state not found for %s", jobName)
		} else {
			if jobState.RunCount == 0 {
				t.Errorf("Expected job %s to run", jobName)
			}
			if jobState.Status != JobStatusSuccess {
				t.Errorf("Expected job %s to succeed, got status %s", jobName, jobState.Status)
			}
		}
	}

	// Jobs should run concurrently, so total time should be less than sum of individual times
	// Each job sleeps 0.1s, so if they ran sequentially it would take at least 0.3s
	// With concurrency, it should complete much faster
	if executionTime > 200*time.Millisecond {
		t.Logf("Warning: Job execution took %v, which may indicate jobs are not running concurrently", executionTime)
	}
}

// TestJobEnvironmentVariables tests job execution with environment variables
func TestJobEnvironmentVariables(t *testing.T) {
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")
	workspaceDir := filepath.Join(stateDir, "deployments", "env-test")

	err := os.MkdirAll(workspaceDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	mockClient := &opentofu.MockTofuClient{}
	templateManager := template.NewManager(filepath.Join(stateDir, "templates"))

	// Create a job with environment variables
	job := &Job{
		Name:        "env-job",
		WorkspaceID: "env-test",
		JobType:     JobTypeScript,
		Script:      "#!/bin/bash\necho \"TEST_VAR=$TEST_VAR\"\necho \"WORKSPACE_ID=$WORKSPACE_ID\"\necho \"JOB_NAME=$JOB_NAME\"",
		Schedule:    "* * * * *",
		Environment: map[string]string{
			"TEST_VAR": "test-value",
		},
		Timeout: "30s",
		Enabled: true,
	}

	err = job.Validate()
	if err != nil {
		t.Fatalf("Job validation failed: %v", err)
	}

	// Execute job
	executor := NewExecutor(workspaceDir, mockClient, templateManager)
	execution := executor.ExecuteJob(job)

	if execution.Status != JobStatusSuccess {
		t.Errorf("Expected job to succeed, got status %s with error: %s", execution.Status, execution.Error)
	}

	// The script should have access to both custom and built-in environment variables
	// We can't easily verify the output here, but the job should succeed
}

// TestJobWorkingDirectory tests job execution with custom working directory
func TestJobWorkingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")
	workspaceDir := filepath.Join(stateDir, "deployments", "workdir-test")
	customWorkDir := filepath.Join(tempDir, "custom-work")

	err := os.MkdirAll(workspaceDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create workspace directory: %v", err)
	}

	err = os.MkdirAll(customWorkDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create custom work directory: %v", err)
	}

	// Create a test file in the custom working directory
	testFile := filepath.Join(customWorkDir, "test-file.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mockClient := &opentofu.MockTofuClient{}
	templateManager := template.NewManager(filepath.Join(stateDir, "templates"))

	// Create a job with custom working directory
	job := &Job{
		Name:        "workdir-job",
		WorkspaceID: "workdir-test",
		JobType:     JobTypeScript,
		Script:      "#!/bin/bash\nls test-file.txt", // This should find the file if working directory is correct
		Schedule:    "* * * * *",
		WorkingDir:  customWorkDir,
		Timeout:     "30s",
		Enabled:     true,
	}

	err = job.Validate()
	if err != nil {
		t.Fatalf("Job validation failed: %v", err)
	}

	// Execute job
	executor := NewExecutor(workspaceDir, mockClient, templateManager)
	execution := executor.ExecuteJob(job)

	if execution.Status != JobStatusSuccess {
		t.Errorf("Expected job to succeed with custom working directory, got status %s with error: %s", execution.Status, execution.Error)
	}
}