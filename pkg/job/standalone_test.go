package job

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"provisioner/pkg/opentofu"
	"provisioner/pkg/template"
)

func TestStandaloneJobConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  StandaloneJobConfig
		wantErr bool
	}{
		{
			name: "valid script job",
			config: StandaloneJobConfig{
				Name:        "test-script",
				Type:        "script",
				Schedule:    "0 * * * *",
				Script:      "echo 'test'",
				Enabled:     true,
				Description: "Test script job",
			},
			wantErr: false,
		},
		{
			name: "valid command job",
			config: StandaloneJobConfig{
				Name:        "test-command",
				Type:        "command",
				Schedule:    "0 * * * *",
				Command:     "uptime",
				Enabled:     true,
				Description: "Test command job",
			},
			wantErr: false,
		},
		{
			name: "valid template job",
			config: StandaloneJobConfig{
				Name:        "test-template",
				Type:        "template",
				Schedule:    "0 * * * *",
				Template:    "monitoring",
				Enabled:     true,
				Description: "Test template job",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: StandaloneJobConfig{
				Type:        "script",
				Schedule:    "0 * * * *",
				Script:      "echo 'test'",
				Enabled:     true,
				Description: "Test script job",
			},
			wantErr: true,
		},
		{
			name: "invalid job type",
			config: StandaloneJobConfig{
				Name:        "test-invalid",
				Type:        "invalid",
				Schedule:    "0 * * * *",
				Script:      "echo 'test'",
				Enabled:     true,
				Description: "Test invalid job",
			},
			wantErr: true,
		},
		{
			name: "script job without script",
			config: StandaloneJobConfig{
				Name:        "test-script-missing",
				Type:        "script",
				Schedule:    "0 * * * *",
				Enabled:     true,
				Description: "Test script job without script",
			},
			wantErr: true,
		},
		{
			name: "command job without command",
			config: StandaloneJobConfig{
				Name:        "test-command-missing",
				Type:        "command",
				Schedule:    "0 * * * *",
				Enabled:     true,
				Description: "Test command job without command",
			},
			wantErr: true,
		},
		{
			name: "template job without template",
			config: StandaloneJobConfig{
				Name:        "test-template-missing",
				Type:        "template",
				Schedule:    "0 * * * *",
				Enabled:     true,
				Description: "Test template job without template",
			},
			wantErr: true,
		},
		{
			name: "invalid schedule",
			config: StandaloneJobConfig{
				Name:        "test-invalid-schedule",
				Type:        "script",
				Schedule:    "invalid",
				Script:      "echo 'test'",
				Enabled:     true,
				Description: "Test with invalid schedule",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestStandaloneJobConfigToJob(t *testing.T) {
	config := StandaloneJobConfig{
		Name:        "test-job",
		Type:        "script",
		Schedule:    "0 * * * *",
		Script:      "echo 'hello world'",
		Environment: map[string]string{"TEST_VAR": "value"},
		WorkingDir:  "/tmp",
		Timeout:     "10m",
		Enabled:     true,
		Description: "Test standalone job",
	}

	job, err := config.ToJob()
	if err != nil {
		t.Fatalf("Failed to convert config to job: %v", err)
	}

	if job.Name != config.Name {
		t.Errorf("Expected job name %s, got %s", config.Name, job.Name)
	}

	if job.WorkspaceID != "_standalone_" {
		t.Errorf("Expected workspace ID '_standalone_', got %s", job.WorkspaceID)
	}

	if job.JobType != JobTypeScript {
		t.Errorf("Expected job type %s, got %s", JobTypeScript, job.JobType)
	}

	if job.Script != config.Script {
		t.Errorf("Expected script %s, got %s", config.Script, job.Script)
	}

	schedules, err := job.GetSchedules()
	if err != nil {
		t.Fatalf("Failed to get schedules: %v", err)
	}

	if len(schedules) != 1 || schedules[0] != config.Schedule {
		t.Errorf("Expected schedule [%s], got %v", config.Schedule, schedules)
	}

	timeout, err := job.GetTimeoutDuration()
	if err != nil {
		t.Fatalf("Failed to get timeout duration: %v", err)
	}
	expectedTimeout, _ := time.ParseDuration("10m")
	if timeout != expectedTimeout {
		t.Errorf("Expected timeout %v, got %v", expectedTimeout, timeout)
	}
}

func TestStandaloneJobManagerFileOperations(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	jobsDir := filepath.Join(tempDir, "jobs")
	stateDir := filepath.Join(tempDir, "state")

	err := os.MkdirAll(jobsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create jobs directory: %v", err)
	}

	err = os.MkdirAll(stateDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create state directory: %v", err)
	}

	// Create mock dependencies
	mockClient := &opentofu.MockTofuClient{}
	templateManager := template.NewManager(filepath.Join(stateDir, "templates"))
	jobManager := NewManager(stateDir, mockClient, templateManager)

	// Create standalone job manager
	sjm := NewStandaloneJobManager(jobsDir, stateDir, jobManager)

	// Test 1: Empty jobs directory
	jobs, err := sjm.ListStandaloneJobs()
	if err != nil {
		t.Fatalf("Failed to list jobs from empty directory: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("Expected no jobs in empty directory, got %d", len(jobs))
	}

	// Test 2: Create a job file
	jobConfig := StandaloneJobConfig{
		Name:        "test-job",
		Type:        "script",
		Schedule:    "0 * * * *",
		Script:      "echo 'test'",
		Enabled:     true,
		Description: "Test job",
	}

	jobData, err := json.MarshalIndent(jobConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal job config: %v", err)
	}

	jobFile := filepath.Join(jobsDir, "test-job.json")
	err = os.WriteFile(jobFile, jobData, 0644)
	if err != nil {
		t.Fatalf("Failed to write job file: %v", err)
	}

	// Test 3: List jobs with one job
	jobs, err = sjm.ListStandaloneJobs()
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "test-job" {
		t.Errorf("Expected job name 'test-job', got %s", jobs[0].Name)
	}

	// Test 4: Test invalid JSON file
	invalidFile := filepath.Join(jobsDir, "invalid.json")
	err = os.WriteFile(invalidFile, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid file: %v", err)
	}

	_, err = sjm.ListStandaloneJobs()
	if err == nil {
		t.Errorf("Expected error when parsing invalid JSON, got none")
	}

	// Clean up invalid file for remaining tests
	os.Remove(invalidFile)

	// Test 5: Test non-JSON file (should be ignored)
	textFile := filepath.Join(jobsDir, "readme.txt")
	err = os.WriteFile(textFile, []byte("This is not a job file"), 0644)
	if err != nil {
		t.Fatalf("Failed to write text file: %v", err)
	}

	jobs, err = sjm.ListStandaloneJobs()
	if err != nil {
		t.Fatalf("Failed to list jobs with non-JSON file present: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job (ignoring non-JSON file), got %d", len(jobs))
	}
}

func TestStandaloneJobExecution(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	jobsDir := filepath.Join(tempDir, "jobs")
	stateDir := filepath.Join(tempDir, "state")

	err := os.MkdirAll(jobsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create jobs directory: %v", err)
	}

	err = os.MkdirAll(stateDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create state directory: %v", err)
	}

	// Create deployment directory for standalone jobs
	deploymentDir := filepath.Join(stateDir, "deployments", "_standalone_")
	err = os.MkdirAll(deploymentDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create deployment directory: %v", err)
	}

	// Create mock dependencies
	mockClient := &opentofu.MockTofuClient{}
	templateManager := template.NewManager(filepath.Join(stateDir, "templates"))
	jobManager := NewManager(stateDir, mockClient, templateManager)

	// Load initial state to initialize state manager
	err = jobManager.LoadState()
	if err != nil {
		t.Fatalf("Failed to load initial state: %v", err)
	}

	// Create standalone job manager
	sjm := NewStandaloneJobManager(jobsDir, stateDir, jobManager)

	// Create a simple script job
	jobConfig := StandaloneJobConfig{
		Name:        "echo-test",
		Type:        "script",
		Schedule:    "0 * * * *",
		Script:      "#!/bin/bash\necho 'Hello from standalone job'",
		Enabled:     true,
		Description: "Echo test job",
		Timeout:     "30s",
	}

	jobData, err := json.MarshalIndent(jobConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal job config: %v", err)
	}

	jobFile := filepath.Join(jobsDir, "echo-test.json")
	err = os.WriteFile(jobFile, jobData, 0644)
	if err != nil {
		t.Fatalf("Failed to write job file: %v", err)
	}

	// Test job execution
	err = sjm.ExecuteStandaloneJob("echo-test")
	if err != nil {
		t.Fatalf("Failed to execute standalone job: %v", err)
	}

	// Check job state
	jobStates := sjm.GetStandaloneJobStates()
	if len(jobStates) != 1 {
		t.Errorf("Expected 1 job state, got %d", len(jobStates))
	}

	jobState, exists := jobStates["echo-test"]
	if !exists {
		t.Fatalf("Job state not found for 'echo-test'")
	}

	if jobState.Status != JobStatusSuccess {
		t.Errorf("Expected job status %s, got %s", JobStatusSuccess, jobState.Status)
	}

	if jobState.RunCount != 1 {
		t.Errorf("Expected run count 1, got %d", jobState.RunCount)
	}

	if jobState.SuccessCount != 1 {
		t.Errorf("Expected success count 1, got %d", jobState.SuccessCount)
	}
}

func TestStandaloneJobScheduleProcessing(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	jobsDir := filepath.Join(tempDir, "jobs")
	stateDir := filepath.Join(tempDir, "state")

	err := os.MkdirAll(jobsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create jobs directory: %v", err)
	}

	err = os.MkdirAll(stateDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create state directory: %v", err)
	}

	// Create deployment directory
	deploymentDir := filepath.Join(stateDir, "deployments", "_standalone_")
	err = os.MkdirAll(deploymentDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create deployment directory: %v", err)
	}

	// Create mock dependencies
	mockClient := &opentofu.MockTofuClient{}
	templateManager := template.NewManager(filepath.Join(stateDir, "templates"))
	jobManager := NewManager(stateDir, mockClient, templateManager)

	// Load initial state
	err = jobManager.LoadState()
	if err != nil {
		t.Fatalf("Failed to load initial state: %v", err)
	}

	// Create standalone job manager
	sjm := NewStandaloneJobManager(jobsDir, stateDir, jobManager)

	// Create jobs with different schedules
	jobs := []StandaloneJobConfig{
		{
			Name:        "job-enabled",
			Type:        "script",
			Schedule:    "* * * * *", // Every minute (should run)
			Script:      "echo 'enabled job'",
			Enabled:     true,
			Description: "Enabled job",
		},
		{
			Name:        "job-disabled",
			Type:        "script",
			Schedule:    "* * * * *", // Every minute (but disabled)
			Script:      "echo 'disabled job'",
			Enabled:     false,
			Description: "Disabled job",
		},
		{
			Name:        "job-future",
			Type:        "script",
			Schedule:    "0 0 1 1 2050", // Year 2050 (should not run)
			Script:      "echo 'future job'",
			Enabled:     true,
			Description: "Future job",
		},
	}

	// Write job files
	for _, jobConfig := range jobs {
		jobData, err := json.MarshalIndent(jobConfig, "", "  ")
		if err != nil {
			t.Fatalf("Failed to marshal job config %s: %v", jobConfig.Name, err)
		}

		jobFile := filepath.Join(jobsDir, jobConfig.Name+".json")
		err = os.WriteFile(jobFile, jobData, 0644)
		if err != nil {
			t.Fatalf("Failed to write job file %s: %v", jobConfig.Name, err)
		}
	}

	// Process standalone jobs
	err = sjm.ProcessStandaloneJobs()
	if err != nil {
		t.Fatalf("Failed to process standalone jobs: %v", err)
	}

	// Check that only the enabled job with a current schedule ran
	jobStates := sjm.GetStandaloneJobStates()

	// The enabled job should have run
	if jobState, exists := jobStates["job-enabled"]; exists {
		if jobState.RunCount == 0 {
			t.Errorf("Expected enabled job to run, but run count is 0")
		}
	} else {
		t.Errorf("Expected job state for enabled job")
	}

	// The disabled job should not have run
	if jobState, exists := jobStates["job-disabled"]; exists {
		if jobState.RunCount > 0 {
			t.Errorf("Expected disabled job not to run, but run count is %d", jobState.RunCount)
		}
	}

	// The future job should not have run
	if jobState, exists := jobStates["job-future"]; exists {
		if jobState.RunCount > 0 {
			t.Errorf("Expected future job not to run, but run count is %d", jobState.RunCount)
		}
	}
}

func TestStandaloneJobErrorHandling(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	jobsDir := filepath.Join(tempDir, "jobs")
	stateDir := filepath.Join(tempDir, "state")

	err := os.MkdirAll(jobsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create jobs directory: %v", err)
	}

	err = os.MkdirAll(stateDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create state directory: %v", err)
	}

	// Create deployment directory
	deploymentDir := filepath.Join(stateDir, "deployments", "_standalone_")
	err = os.MkdirAll(deploymentDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create deployment directory: %v", err)
	}

	// Create mock dependencies
	mockClient := &opentofu.MockTofuClient{}
	templateManager := template.NewManager(filepath.Join(stateDir, "templates"))
	jobManager := NewManager(stateDir, mockClient, templateManager)

	// Load initial state
	err = jobManager.LoadState()
	if err != nil {
		t.Fatalf("Failed to load initial state: %v", err)
	}

	sjm := NewStandaloneJobManager(jobsDir, stateDir, jobManager)

	// Test 1: Execute non-existent job
	err = sjm.ExecuteStandaloneJob("non-existent")
	if err == nil {
		t.Errorf("Expected error when executing non-existent job")
	}

	// Test 2: Create a job that will fail
	failingJobConfig := StandaloneJobConfig{
		Name:        "failing-job",
		Type:        "command",
		Schedule:    "* * * * *",
		Command:     "exit 1", // Command that always fails
		Enabled:     true,
		Description: "Job that always fails",
	}

	jobData, err := json.MarshalIndent(failingJobConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal failing job config: %v", err)
	}

	jobFile := filepath.Join(jobsDir, "failing-job.json")
	err = os.WriteFile(jobFile, jobData, 0644)
	if err != nil {
		t.Fatalf("Failed to write failing job file: %v", err)
	}

	// Execute the failing job
	err = sjm.ExecuteStandaloneJob("failing-job")
	if err == nil {
		t.Errorf("Expected error when executing failing job")
	}

	// Check that failure is recorded in state
	jobStates := sjm.GetStandaloneJobStates()
	if jobState, exists := jobStates["failing-job"]; exists {
		if jobState.Status != JobStatusFailed {
			t.Errorf("Expected job status %s, got %s", JobStatusFailed, jobState.Status)
		}
		if jobState.FailureCount != 1 {
			t.Errorf("Expected failure count 1, got %d", jobState.FailureCount)
		}
	} else {
		t.Errorf("Expected job state for failing job")
	}
}

func TestStandaloneJobMultipleSchedules(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	jobsDir := filepath.Join(tempDir, "jobs")
	stateDir := filepath.Join(tempDir, "state")

	err := os.MkdirAll(jobsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create jobs directory: %v", err)
	}

	err = os.MkdirAll(stateDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create state directory: %v", err)
	}

	// Test multiple schedule formats
	testCases := []struct {
		name     string
		schedule interface{}
		valid    bool
	}{
		{
			name:     "single string schedule",
			schedule: "0 * * * *",
			valid:    true,
		},
		{
			name:     "array of schedules",
			schedule: []string{"0 * * * *", "30 * * * *"},
			valid:    true,
		},
		{
			name:     "mixed interface array",
			schedule: []interface{}{"0 * * * *", "30 * * * *"},
			valid:    true,
		},
		{
			name:     "invalid schedule type",
			schedule: 123,
			valid:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create job config with the test schedule
			jobConfig := map[string]interface{}{
				"name":        "test-" + tc.name,
				"type":        "script",
				"schedule":    tc.schedule,
				"script":      "echo 'test'",
				"enabled":     true,
				"description": "Test job with " + tc.name,
			}

			// Marshal to JSON and back to StandaloneJobConfig
			jobData, err := json.Marshal(jobConfig)
			if err != nil {
				t.Fatalf("Failed to marshal job config: %v", err)
			}

			var standaloneConfig StandaloneJobConfig
			err = json.Unmarshal(jobData, &standaloneConfig)
			if err != nil {
				if tc.valid {
					t.Fatalf("Failed to unmarshal valid job config: %v", err)
				}
				return // Expected failure for invalid configs
			}

			// Validate the config
			err = standaloneConfig.Validate()
			if tc.valid && err != nil {
				t.Errorf("Expected valid config to pass validation, got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Errorf("Expected invalid config to fail validation")
			}

			if tc.valid {
				// Convert to Job and test schedule parsing
				job, err := standaloneConfig.ToJob()
				if err != nil {
					t.Fatalf("Failed to convert config to job: %v", err)
				}

				schedules, err := job.GetSchedules()
				if err != nil {
					t.Fatalf("Failed to get schedules: %v", err)
				}

				// Verify we got the expected number of schedules
				switch s := tc.schedule.(type) {
				case string:
					if len(schedules) != 1 {
						t.Errorf("Expected 1 schedule for string input, got %d", len(schedules))
					}
				case []string:
					if len(schedules) != len(s) {
						t.Errorf("Expected %d schedules for array input, got %d", len(s), len(schedules))
					}
				case []interface{}:
					if len(schedules) != len(s) {
						t.Errorf("Expected %d schedules for interface array input, got %d", len(s), len(schedules))
					}
				}
			}
		})
	}
}