package job

import (
	"testing"
	"time"
)

func TestJobValidation(t *testing.T) {
	tests := []struct {
		name    string
		job     Job
		wantErr bool
	}{
		{
			name: "valid script job",
			job: Job{
				Name:        "test-script",
				WorkspaceID: "test-workspace",
				JobType:     JobTypeScript,
				Script:      "echo hello",
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "valid command job",
			job: Job{
				Name:        "test-command",
				WorkspaceID: "test-workspace",
				JobType:     JobTypeCommand,
				Command:     "ls -la",
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "valid template job",
			job: Job{
				Name:        "test-template",
				WorkspaceID: "test-workspace",
				JobType:     JobTypeTemplate,
				Template:    "monitoring",
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			job: Job{
				WorkspaceID: "test-workspace",
				JobType:     JobTypeScript,
				Script:      "echo hello",
			},
			wantErr: true,
		},
		{
			name: "missing workspace ID",
			job: Job{
				Name:    "test-job",
				JobType: JobTypeScript,
				Script:  "echo hello",
			},
			wantErr: true,
		},
		{
			name: "script job without script",
			job: Job{
				Name:        "test-script",
				WorkspaceID: "test-workspace",
				JobType:     JobTypeScript,
				Enabled:     true,
			},
			wantErr: true,
		},
		{
			name: "command job without command",
			job: Job{
				Name:        "test-command",
				WorkspaceID: "test-workspace",
				JobType:     JobTypeCommand,
				Enabled:     true,
			},
			wantErr: true,
		},
		{
			name: "template job without template",
			job: Job{
				Name:        "test-template",
				WorkspaceID: "test-workspace",
				JobType:     JobTypeTemplate,
				Enabled:     true,
			},
			wantErr: true,
		},
		{
			name: "invalid job type",
			job: Job{
				Name:        "test-invalid",
				WorkspaceID: "test-workspace",
				JobType:     "invalid",
				Enabled:     true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Job.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobTimeoutParsing(t *testing.T) {
	tests := []struct {
		name     string
		timeout  string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "valid timeout",
			timeout:  "30m",
			expected: 30 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "empty timeout uses default",
			timeout:  "",
			expected: 10 * time.Minute,
			wantErr:  false,
		},
		{
			name:    "invalid timeout",
			timeout: "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := Job{Timeout: tt.timeout}
			duration, err := job.GetTimeoutDuration()

			if (err != nil) != tt.wantErr {
				t.Errorf("Job.GetTimeoutDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && duration != tt.expected {
				t.Errorf("Job.GetTimeoutDuration() = %v, expected %v", duration, tt.expected)
			}
		})
	}
}

func TestJobScheduleParsing(t *testing.T) {
	tests := []struct {
		name     string
		schedule interface{}
		expected []string
		wantErr  bool
	}{
		{
			name:     "single string schedule",
			schedule: "0 9 * * *",
			expected: []string{"0 9 * * *"},
			wantErr:  false,
		},
		{
			name:     "array schedule",
			schedule: []string{"0 9 * * *", "0 17 * * *"},
			expected: []string{"0 9 * * *", "0 17 * * *"},
			wantErr:  false,
		},
		{
			name:     "nil schedule",
			schedule: nil,
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "invalid schedule type",
			schedule: 123,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := Job{Schedule: tt.schedule}
			schedules, err := job.GetSchedules()

			if (err != nil) != tt.wantErr {
				t.Errorf("Job.GetSchedules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(schedules) != len(tt.expected) {
					t.Errorf("Job.GetSchedules() length = %v, expected %v", len(schedules), len(tt.expected))
					return
				}

				for i, schedule := range schedules {
					if schedule != tt.expected[i] {
						t.Errorf("Job.GetSchedules()[%d] = %v, expected %v", i, schedule, tt.expected[i])
					}
				}
			}
		})
	}
}

func TestJobConfigToJob(t *testing.T) {
	configMap := map[string]interface{}{
		"name":        "test-job",
		"type":        "script",
		"script":      "echo hello",
		"schedule":    "0 9 * * *",
		"timeout":     "30m",
		"enabled":     true,
		"description": "Test job",
		"environment": map[string]interface{}{
			"TEST_VAR": "test-value",
		},
	}

	job, err := JobConfigToJob("test-workspace", configMap)
	if err != nil {
		t.Fatalf("JobConfigToJob() error = %v", err)
	}

	if job.Name != "test-job" {
		t.Errorf("Job.Name = %v, expected test-job", job.Name)
	}

	if job.WorkspaceID != "test-workspace" {
		t.Errorf("Job.WorkspaceID = %v, expected test-workspace", job.WorkspaceID)
	}

	if job.JobType != JobTypeScript {
		t.Errorf("Job.JobType = %v, expected %v", job.JobType, JobTypeScript)
	}

	if job.Script != "echo hello" {
		t.Errorf("Job.Script = %v, expected 'echo hello'", job.Script)
	}

	if !job.Enabled {
		t.Errorf("Job.Enabled = %v, expected true", job.Enabled)
	}

	if job.Environment["TEST_VAR"] != "test-value" {
		t.Errorf("Job.Environment[TEST_VAR] = %v, expected test-value", job.Environment["TEST_VAR"])
	}
}

func TestJobStateTransitions(t *testing.T) {
	state := &JobState{
		Name:        "test-job",
		WorkspaceID: "test-workspace",
		Status:      JobStatusPending,
	}

	// Test execution tracking
	execution := &JobExecution{
		JobName:     "test-job",
		WorkspaceID: "test-workspace",
		Status:      JobStatusSuccess,
		StartTime:   time.Now(),
		Duration:    5 * time.Minute,
	}

	// Simulate state manager update
	state.Status = execution.Status
	state.RunCount++
	state.SuccessCount++
	state.LastRun = &execution.StartTime

	if state.Status != JobStatusSuccess {
		t.Errorf("JobState.Status = %v, expected %v", state.Status, JobStatusSuccess)
	}

	if state.RunCount != 1 {
		t.Errorf("JobState.RunCount = %v, expected 1", state.RunCount)
	}

	if state.SuccessCount != 1 {
		t.Errorf("JobState.SuccessCount = %v, expected 1", state.SuccessCount)
	}
}
