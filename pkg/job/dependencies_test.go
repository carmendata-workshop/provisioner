package job

import (
	"testing"
)

func TestDependencyResolver_ValidateDependencies(t *testing.T) {
	tests := []struct {
		name          string
		jobs          []*Job
		expectError   bool
		errorContains string
	}{
		{
			name: "no dependencies",
			jobs: []*Job{
				{Name: "job1", WorkspaceID: "ws1"},
				{Name: "job2", WorkspaceID: "ws1"},
			},
			expectError: false,
		},
		{
			name: "valid linear dependencies",
			jobs: []*Job{
				{Name: "job1", WorkspaceID: "ws1"},
				{Name: "job2", WorkspaceID: "ws1", DependsOn: []string{"job1"}},
				{Name: "job3", WorkspaceID: "ws1", DependsOn: []string{"job2"}},
			},
			expectError: false,
		},
		{
			name: "valid parallel dependencies",
			jobs: []*Job{
				{Name: "foundation", WorkspaceID: "ws1"},
				{Name: "database", WorkspaceID: "ws1", DependsOn: []string{"foundation"}},
				{Name: "cache", WorkspaceID: "ws1", DependsOn: []string{"foundation"}},
				{Name: "app", WorkspaceID: "ws1", DependsOn: []string{"database", "cache"}},
			},
			expectError: false,
		},
		{
			name: "circular dependency - simple",
			jobs: []*Job{
				{Name: "job1", WorkspaceID: "ws1", DependsOn: []string{"job2"}},
				{Name: "job2", WorkspaceID: "ws1", DependsOn: []string{"job1"}},
			},
			expectError:   true,
			errorContains: "circular dependency",
		},
		{
			name: "circular dependency - complex",
			jobs: []*Job{
				{Name: "job1", WorkspaceID: "ws1", DependsOn: []string{"job2"}},
				{Name: "job2", WorkspaceID: "ws1", DependsOn: []string{"job3"}},
				{Name: "job3", WorkspaceID: "ws1", DependsOn: []string{"job1"}},
			},
			expectError:   true,
			errorContains: "circular dependency",
		},
		{
			name: "missing dependency",
			jobs: []*Job{
				{Name: "job1", WorkspaceID: "ws1", DependsOn: []string{"nonexistent"}},
			},
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewDependencyResolver(tt.jobs)
			err := resolver.ValidateDependencies()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
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

func TestDependencyResolver_CanExecute(t *testing.T) {
	jobs := []*Job{
		{Name: "foundation", WorkspaceID: "ws1"},
		{Name: "database", WorkspaceID: "ws1", DependsOn: []string{"foundation"}},
		{Name: "app", WorkspaceID: "ws1", DependsOn: []string{"database", "foundation"}},
	}

	resolver := NewDependencyResolver(jobs)

	// Initially, only foundation should be able to execute
	canExecute, reason := resolver.CanExecute(jobs[0]) // foundation
	if !canExecute {
		t.Errorf("foundation should be able to execute, but got: %s", reason)
	}

	canExecute, _ = resolver.CanExecute(jobs[1]) // database
	if canExecute {
		t.Errorf("database should not be able to execute initially, but it can")
	}

	// Mark foundation as completed
	resolver.SetJobCompleted("foundation")

	canExecute, reason = resolver.CanExecute(jobs[1]) // database
	if !canExecute {
		t.Errorf("database should be able to execute after foundation completes, but got: %s", reason)
	}

	canExecute, _ = resolver.CanExecute(jobs[2]) // app
	if canExecute {
		t.Errorf("app should not be able to execute yet (database not complete), but it can")
	}

	// Mark database as completed
	resolver.SetJobCompleted("database")

	canExecute, reason = resolver.CanExecute(jobs[2]) // app
	if !canExecute {
		t.Errorf("app should be able to execute after all dependencies complete, but got: %s", reason)
	}
}

func TestDependencyResolver_GetReadyJobs(t *testing.T) {
	jobs := []*Job{
		{Name: "foundation", WorkspaceID: "ws1"},
		{Name: "database", WorkspaceID: "ws1", DependsOn: []string{"foundation"}},
		{Name: "cache", WorkspaceID: "ws1", DependsOn: []string{"foundation"}},
		{Name: "app", WorkspaceID: "ws1", DependsOn: []string{"database", "cache"}},
	}

	resolver := NewDependencyResolver(jobs)

	// Initially, only foundation should be ready
	readyJobs := resolver.GetReadyJobs()
	if len(readyJobs) != 1 || readyJobs[0].Name != "foundation" {
		t.Errorf("expected only foundation to be ready initially, got: %v", getJobNames(readyJobs))
	}

	// Mark foundation as completed
	resolver.SetJobCompleted("foundation")

	// Now database and cache should be ready
	readyJobs = resolver.GetReadyJobs()
	expectedReady := []string{"database", "cache"}
	actualReady := getJobNames(readyJobs)
	if !sameSlice(expectedReady, actualReady) {
		t.Errorf("expected %v to be ready, got: %v", expectedReady, actualReady)
	}

	// Mark database and cache as completed
	resolver.SetJobCompleted("database")
	resolver.SetJobCompleted("cache")

	// Now app should be ready
	readyJobs = resolver.GetReadyJobs()
	if len(readyJobs) != 1 || readyJobs[0].Name != "app" {
		t.Errorf("expected only app to be ready, got: %v", getJobNames(readyJobs))
	}

	// Mark app as completed
	resolver.SetJobCompleted("app")

	// No jobs should be ready now
	readyJobs = resolver.GetReadyJobs()
	if len(readyJobs) != 0 {
		t.Errorf("expected no jobs to be ready, got: %v", getJobNames(readyJobs))
	}
}

func TestDependencyResolver_GetExecutionOrder(t *testing.T) {
	jobs := []*Job{
		{Name: "app", WorkspaceID: "ws1", DependsOn: []string{"database", "cache"}},
		{Name: "database", WorkspaceID: "ws1", DependsOn: []string{"foundation"}},
		{Name: "foundation", WorkspaceID: "ws1"},
		{Name: "cache", WorkspaceID: "ws1", DependsOn: []string{"foundation"}},
	}

	resolver := NewDependencyResolver(jobs)

	order, err := resolver.GetExecutionOrder()
	if err != nil {
		t.Fatalf("expected no error getting execution order, got: %v", err)
	}

	// Foundation should come first
	if order[0].Name != "foundation" {
		t.Errorf("expected foundation to be first, got: %s", order[0].Name)
	}

	// App should come last
	if order[len(order)-1].Name != "app" {
		t.Errorf("expected app to be last, got: %s", order[len(order)-1].Name)
	}

	// Database and cache should come after foundation but before app
	foundationIndex := findJobIndex(order, "foundation")
	databaseIndex := findJobIndex(order, "database")
	cacheIndex := findJobIndex(order, "cache")
	appIndex := findJobIndex(order, "app")

	if databaseIndex <= foundationIndex || cacheIndex <= foundationIndex {
		t.Error("database and cache should come after foundation")
	}

	if appIndex <= databaseIndex || appIndex <= cacheIndex {
		t.Error("app should come after database and cache")
	}
}

func TestDependencyResolver_FailedDependency(t *testing.T) {
	jobs := []*Job{
		{Name: "foundation", WorkspaceID: "ws1"},
		{Name: "database", WorkspaceID: "ws1", DependsOn: []string{"foundation"}},
		{Name: "app", WorkspaceID: "ws1", DependsOn: []string{"database"}},
	}

	resolver := NewDependencyResolver(jobs)

	// Mark foundation as failed
	resolver.SetJobFailed("foundation")

	// Database should not be able to execute
	canExecute, reason := resolver.CanExecute(jobs[1]) // database
	if canExecute {
		t.Errorf("database should not be able to execute when foundation failed")
	}
	if !contains(reason, "has failed") {
		t.Errorf("expected failure reason to mention failed dependency, got: %s", reason)
	}

	// No jobs should be ready
	readyJobs := resolver.GetReadyJobs()
	if len(readyJobs) != 0 {
		t.Errorf("expected no jobs to be ready when foundation failed, got: %v", getJobNames(readyJobs))
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > 0 && (s[:len(substr)] == substr ||
			(len(s) > len(substr) && contains(s[1:], substr)))
}

func getJobNames(jobs []*Job) []string {
	names := make([]string, len(jobs))
	for i, job := range jobs {
		names[i] = job.Name
	}
	return names
}

func sameSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create a map to count occurrences
	counts := make(map[string]int)
	for _, item := range a {
		counts[item]++
	}

	for _, item := range b {
		counts[item]--
		if counts[item] < 0 {
			return false
		}
	}

	// Check if all counts are zero
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}

	return true
}

func findJobIndex(jobs []*Job, name string) int {
	for i, job := range jobs {
		if job.Name == name {
			return i
		}
	}
	return -1
}
