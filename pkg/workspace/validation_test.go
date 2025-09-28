package workspace

import (
	"strings"
	"testing"
)

func TestValidateJobDependencies(t *testing.T) {
	tests := []struct {
		name          string
		jobs          []JobConfig
		expectError   bool
		errorContains string
	}{
		{
			name: "no jobs",
			jobs: []JobConfig{},
			expectError: false,
		},
		{
			name: "single job no dependencies",
			jobs: []JobConfig{
				{Name: "job1", Type: "script", Script: "echo 'test'"},
			},
			expectError: false,
		},
		{
			name: "valid linear dependencies",
			jobs: []JobConfig{
				{Name: "job1", Type: "script", Script: "echo 'job1'"},
				{Name: "job2", Type: "script", Script: "echo 'job2'", DependsOn: []string{"job1"}},
				{Name: "job3", Type: "script", Script: "echo 'job3'", DependsOn: []string{"job2"}},
			},
			expectError: false,
		},
		{
			name: "valid parallel dependencies",
			jobs: []JobConfig{
				{Name: "foundation", Type: "template", Template: "vpc"},
				{Name: "database", Type: "template", Template: "db", DependsOn: []string{"foundation"}},
				{Name: "cache", Type: "template", Template: "redis", DependsOn: []string{"foundation"}},
				{Name: "app", Type: "template", Template: "web", DependsOn: []string{"database", "cache"}},
			},
			expectError: false,
		},
		{
			name: "circular dependency - simple",
			jobs: []JobConfig{
				{Name: "job1", Type: "script", Script: "echo 'job1'", DependsOn: []string{"job2"}},
				{Name: "job2", Type: "script", Script: "echo 'job2'", DependsOn: []string{"job1"}},
			},
			expectError: true,
			errorContains: "circular dependency",
		},
		{
			name: "circular dependency - complex",
			jobs: []JobConfig{
				{Name: "job1", Type: "script", Script: "echo 'job1'", DependsOn: []string{"job2"}},
				{Name: "job2", Type: "script", Script: "echo 'job2'", DependsOn: []string{"job3"}},
				{Name: "job3", Type: "script", Script: "echo 'job3'", DependsOn: []string{"job1"}},
			},
			expectError: true,
			errorContains: "circular dependency",
		},
		{
			name: "missing dependency",
			jobs: []JobConfig{
				{Name: "job1", Type: "script", Script: "echo 'job1'", DependsOn: []string{"nonexistent"}},
			},
			expectError: true,
			errorContains: "depends on non-existent job",
		},
		{
			name: "multiple missing dependencies",
			jobs: []JobConfig{
				{Name: "job1", Type: "script", Script: "echo 'job1'", DependsOn: []string{"missing1", "missing2"}},
			},
			expectError: true,
			errorContains: "depends on non-existent job",
		},
		{
			name: "self dependency",
			jobs: []JobConfig{
				{Name: "job1", Type: "script", Script: "echo 'job1'", DependsOn: []string{"job1"}},
			},
			expectError: true,
			errorContains: "circular dependency",
		},
		{
			name: "complex valid dependencies",
			jobs: []JobConfig{
				{Name: "foundation", Type: "template", Template: "vpc"},
				{Name: "database", Type: "template", Template: "db", DependsOn: []string{"foundation"}},
				{Name: "cache", Type: "template", Template: "redis", DependsOn: []string{"foundation"}},
				{Name: "app", Type: "template", Template: "web", DependsOn: []string{"database", "cache"}},
				{Name: "monitoring", Type: "template", Template: "monitor", DependsOn: []string{"app"}},
				{Name: "backup", Type: "script", Script: "backup.sh"}, // No dependencies
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJobDependencies(tt.jobs)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateJobDependencies_RealWorldExample(t *testing.T) {
	// Test with a realistic sub-workspace configuration similar to the demo
	jobs := []JobConfig{
		{
			Name:        "network-foundation",
			Type:        "template",
			Template:    "vpc-subnets",
			Schedule:    "@deployment",
			Description: "Deploy network foundation",
		},
		{
			Name:        "database-cluster",
			Type:        "template",
			Template:    "postgres-cluster",
			Schedule:    "@deployment",
			DependsOn:   []string{"network-foundation"},
			Description: "Deploy database after network",
		},
		{
			Name:        "cache-cluster",
			Type:        "template",
			Template:    "redis-cluster",
			Schedule:    "@deployment",
			DependsOn:   []string{"network-foundation"},
			Description: "Deploy cache after network",
		},
		{
			Name:        "application-stack",
			Type:        "template",
			Template:    "web-application",
			Schedule:    "@deployment",
			DependsOn:   []string{"database-cluster", "cache-cluster"},
			Description: "Deploy app after database and cache",
		},
		{
			Name:        "monitoring-stack",
			Type:        "template",
			Template:    "prometheus-grafana",
			Schedule:    "@deployment",
			DependsOn:   []string{"application-stack"},
			Description: "Deploy monitoring after application",
		},
		{
			Name:        "setup-dns",
			Type:        "script",
			Script:      "#!/bin/bash\necho 'Setting up DNS'",
			Schedule:    "@deployment",
			Description: "Configure DNS records",
		},
		{
			Name:        "cleanup-failed",
			Type:        "script",
			Script:      "#!/bin/bash\necho 'Cleaning up'",
			Schedule:    "@deployment-failed",
			Description: "Cleanup after failure",
		},
		{
			Name:        "regular-backup",
			Type:        "script",
			Script:      "#!/bin/bash\necho 'Backup'",
			Schedule:    "0 2 * * *",
			Description: "Regular backup job",
		},
	}

	err := ValidateJobDependencies(jobs)
	if err != nil {
		t.Errorf("expected valid configuration to pass validation, got: %v", err)
	}
}