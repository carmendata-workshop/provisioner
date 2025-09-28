package job

import (
	"fmt"
	"slices"
)

// DependencyResolver handles job dependency checking and execution ordering
type DependencyResolver struct {
	jobs         []*Job
	jobsByName   map[string]*Job
	completedJobs map[string]bool
	failedJobs   map[string]bool
}

// NewDependencyResolver creates a new dependency resolver
func NewDependencyResolver(jobs []*Job) *DependencyResolver {
	resolver := &DependencyResolver{
		jobs:         jobs,
		jobsByName:   make(map[string]*Job),
		completedJobs: make(map[string]bool),
		failedJobs:   make(map[string]bool),
	}

	// Build job name index
	for _, job := range jobs {
		resolver.jobsByName[job.Name] = job
	}

	return resolver
}

// SetJobCompleted marks a job as completed successfully
func (dr *DependencyResolver) SetJobCompleted(jobName string) {
	dr.completedJobs[jobName] = true
	delete(dr.failedJobs, jobName) // Remove from failed if it was there
}

// SetJobFailed marks a job as failed
func (dr *DependencyResolver) SetJobFailed(jobName string) {
	dr.failedJobs[jobName] = true
	delete(dr.completedJobs, jobName) // Remove from completed if it was there
}

// IsJobCompleted checks if a job has completed successfully
func (dr *DependencyResolver) IsJobCompleted(jobName string) bool {
	return dr.completedJobs[jobName]
}

// IsJobFailed checks if a job has failed
func (dr *DependencyResolver) IsJobFailed(jobName string) bool {
	return dr.failedJobs[jobName]
}

// CanExecute checks if a job can be executed (all dependencies are satisfied)
func (dr *DependencyResolver) CanExecute(job *Job) (bool, string) {
	// Check if any dependencies are missing
	for _, depName := range job.DependsOn {
		// Check if dependency job exists
		if _, exists := dr.jobsByName[depName]; !exists {
			return false, fmt.Sprintf("dependency job '%s' does not exist", depName)
		}

		// Check if dependency has failed
		if dr.IsJobFailed(depName) {
			return false, fmt.Sprintf("dependency job '%s' has failed", depName)
		}

		// Check if dependency is completed
		if !dr.IsJobCompleted(depName) {
			return false, fmt.Sprintf("dependency job '%s' has not completed", depName)
		}
	}

	return true, ""
}

// GetReadyJobs returns all jobs that are ready to execute (dependencies satisfied)
func (dr *DependencyResolver) GetReadyJobs() []*Job {
	var readyJobs []*Job

	for _, job := range dr.jobs {
		// Skip if job is already completed or failed
		if dr.IsJobCompleted(job.Name) || dr.IsJobFailed(job.Name) {
			continue
		}

		// Check if job can execute
		if canExecute, _ := dr.CanExecute(job); canExecute {
			readyJobs = append(readyJobs, job)
		}
	}

	return readyJobs
}

// ValidateDependencies checks for circular dependencies and missing job references
func (dr *DependencyResolver) ValidateDependencies() error {
	// Check for circular dependencies using topological sort
	if err := dr.detectCycles(); err != nil {
		return err
	}

	// Check for missing dependency references
	for _, job := range dr.jobs {
		for _, depName := range job.DependsOn {
			if _, exists := dr.jobsByName[depName]; !exists {
				return fmt.Errorf("job '%s' depends on non-existent job '%s'", job.Name, depName)
			}
		}
	}

	return nil
}

// detectCycles uses DFS to detect circular dependencies
func (dr *DependencyResolver) detectCycles() error {
	// States: 0 = unvisited, 1 = visiting, 2 = visited
	state := make(map[string]int)

	var dfs func(jobName string) error
	dfs = func(jobName string) error {
		if state[jobName] == 1 {
			return fmt.Errorf("circular dependency detected involving job '%s'", jobName)
		}
		if state[jobName] == 2 {
			return nil // Already processed
		}

		state[jobName] = 1 // Mark as visiting

		job, exists := dr.jobsByName[jobName]
		if !exists {
			return fmt.Errorf("job '%s' not found", jobName)
		}

		// Visit dependencies
		for _, depName := range job.DependsOn {
			if err := dfs(depName); err != nil {
				return err
			}
		}

		state[jobName] = 2 // Mark as visited
		return nil
	}

	// Check all jobs
	for jobName := range dr.jobsByName {
		if state[jobName] == 0 {
			if err := dfs(jobName); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetExecutionOrder returns jobs in dependency-safe execution order using topological sort
func (dr *DependencyResolver) GetExecutionOrder() ([]*Job, error) {
	if err := dr.ValidateDependencies(); err != nil {
		return nil, err
	}

	visited := make(map[string]bool)
	var result []*Job

	var visit func(jobName string) error
	visit = func(jobName string) error {
		if visited[jobName] {
			return nil
		}

		job, exists := dr.jobsByName[jobName]
		if !exists {
			return fmt.Errorf("job '%s' not found", jobName)
		}

		// Visit dependencies first
		for _, depName := range job.DependsOn {
			if err := visit(depName); err != nil {
				return err
			}
		}

		visited[jobName] = true
		result = append(result, job)
		return nil
	}

	// Visit all jobs
	for jobName := range dr.jobsByName {
		if err := visit(jobName); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// GetDependentsOf returns all jobs that depend on the given job
func (dr *DependencyResolver) GetDependentsOf(jobName string) []*Job {
	var dependents []*Job

	for _, job := range dr.jobs {
		if slices.Contains(job.DependsOn, jobName) {
			dependents = append(dependents, job)
		}
	}

	return dependents
}

// HasPendingDependents checks if any jobs are waiting for this job to complete
func (dr *DependencyResolver) HasPendingDependents(jobName string) bool {
	dependents := dr.GetDependentsOf(jobName)
	for _, dependent := range dependents {
		if !dr.IsJobCompleted(dependent.Name) && !dr.IsJobFailed(dependent.Name) {
			return true
		}
	}
	return false
}