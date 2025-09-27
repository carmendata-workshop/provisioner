package job

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"provisioner/pkg/logging"
	"provisioner/pkg/opentofu"
	"provisioner/pkg/template"
)

// Executor handles job execution within workspace contexts
type Executor struct {
	workspaceDeploymentDir string
	tofuClient             opentofu.TofuClient
	templateManager        *template.Manager
}

// NewExecutor creates a new job executor for a workspace
func NewExecutor(workspaceDeploymentDir string, tofuClient opentofu.TofuClient, templateManager *template.Manager) *Executor {
	return &Executor{
		workspaceDeploymentDir: workspaceDeploymentDir,
		tofuClient:             tofuClient,
		templateManager:        templateManager,
	}
}

// ExecuteJob executes a job and returns the execution result
func (e *Executor) ExecuteJob(job *Job) *JobExecution {
	execution := &JobExecution{
		JobName:     job.Name,
		WorkspaceID: job.WorkspaceID,
		Status:      JobStatusRunning,
		StartTime:   time.Now(),
	}

	logging.LogWorkspace(job.WorkspaceID, "JOB %s: Starting execution", job.Name)

	// Get timeout duration
	timeout, err := job.GetTimeoutDuration()
	if err != nil {
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Invalid timeout: %v", err)
		e.finishExecution(execution)
		return execution
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Execute based on job type
	switch job.JobType {
	case JobTypeScript:
		e.executeScript(ctx, job, execution)
	case JobTypeCommand:
		e.executeCommand(ctx, job, execution)
	case JobTypeTemplate:
		e.executeTemplate(ctx, job, execution)
	default:
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Unknown job type: %s", job.JobType)
	}

	e.finishExecution(execution)
	return execution
}

// executeScript runs a shell script
func (e *Executor) executeScript(ctx context.Context, job *Job, execution *JobExecution) {
	// Create temporary script file
	scriptFile, err := e.createTempScript(job.Script)
	if err != nil {
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Failed to create script file: %v", err)
		return
	}
	defer os.Remove(scriptFile)

	// Execute script
	cmd := exec.CommandContext(ctx, "/bin/bash", scriptFile)
	e.setupCommand(cmd, job)
	e.runCommand(cmd, execution)
}

// executeCommand runs a single command
func (e *Executor) executeCommand(ctx context.Context, job *Job, execution *JobExecution) {
	// Parse command and arguments
	parts := strings.Fields(job.Command)
	if len(parts) == 0 {
		execution.Status = JobStatusFailed
		execution.Error = "Empty command"
		return
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	e.setupCommand(cmd, job)
	e.runCommand(cmd, execution)
}

// executeTemplate deploys or updates a template within the workspace
func (e *Executor) executeTemplate(ctx context.Context, job *Job, execution *JobExecution) {
	if e.tofuClient == nil {
		execution.Status = JobStatusFailed
		execution.Error = "OpenTofu client not available for template jobs"
		return
	}

	// Validate template exists
	if err := e.templateManager.ValidateTemplate(job.Template); err != nil {
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Template validation failed: %v", err)
		return
	}

	// Create subdirectory for this template job
	jobWorkingDir := filepath.Join(e.workspaceDeploymentDir, "jobs", job.Name)
	if err := os.MkdirAll(jobWorkingDir, 0755); err != nil {
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Failed to create job working directory: %v", err)
		return
	}

	// Copy template files to job working directory
	templatePath := e.templateManager.GetTemplatePath(job.Template)
	if err := e.copyTemplateFiles(templatePath, jobWorkingDir); err != nil {
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Failed to copy template files: %v", err)
		return
	}

	// Run OpenTofu commands directly in job working directory
	if err := e.tofuClient.Init(jobWorkingDir); err != nil {
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Template init failed: %v", err)
		return
	}

	if err := e.tofuClient.Plan(jobWorkingDir); err != nil {
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Template plan failed: %v", err)
		return
	}

	if err := e.tofuClient.Apply(jobWorkingDir); err != nil {
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Template apply failed: %v", err)
		return
	}

	execution.Status = JobStatusSuccess
	execution.Output = fmt.Sprintf("Template '%s' deployed successfully in job working directory", job.Template)
}

// copyTemplateFiles copies template files to the job working directory
func (e *Executor) copyTemplateFiles(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// setupCommand configures the command with environment and working directory
func (e *Executor) setupCommand(cmd *exec.Cmd, job *Job) {
	// Set working directory
	cmd.Dir = job.GetWorkingDirectory(e.workspaceDeploymentDir)

	// Set up environment
	cmd.Env = os.Environ()

	// Add job-specific environment variables
	for key, value := range job.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Add workspace-specific environment variables
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("WORKSPACE_ID=%s", job.WorkspaceID),
		fmt.Sprintf("JOB_NAME=%s", job.Name),
		fmt.Sprintf("WORKSPACE_DEPLOYMENT_DIR=%s", e.workspaceDeploymentDir),
	)
}

// runCommand executes the command and captures output
func (e *Executor) runCommand(cmd *exec.Cmd, execution *JobExecution) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Store PID when process starts
	err := cmd.Start()
	if err != nil {
		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Failed to start command: %v", err)
		return
	}

	execution.PID = cmd.Process.Pid
	logging.LogWorkspace(execution.WorkspaceID, "JOB %s: Process started with PID %d", execution.JobName, execution.PID)

	// Wait for command to complete
	err = cmd.Wait()

	// Capture output
	execution.Output = stdout.String()
	if stderr.Len() > 0 {
		if execution.Output != "" {
			execution.Output += "\n--- STDERR ---\n"
		}
		execution.Output += stderr.String()
	}

	// Determine exit status
	if err != nil {
		if ctx := cmd.ProcessState; ctx != nil {
			execution.ExitCode = ctx.ExitCode()
		}

		// Check if it was killed due to timeout
		if cmd.ProcessState != nil && cmd.ProcessState.Sys() != nil {
			if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
				if status.Signal() == syscall.SIGKILL {
					execution.Status = JobStatusTimeout
					execution.Error = "Job timed out"
					return
				}
			}
		}

		execution.Status = JobStatusFailed
		execution.Error = fmt.Sprintf("Command failed: %v", err)
	} else {
		execution.Status = JobStatusSuccess
		execution.ExitCode = 0
	}
}

// createTempScript creates a temporary script file
func (e *Executor) createTempScript(scriptContent string) (string, error) {
	tempFile, err := os.CreateTemp("", "job-script-*.sh")
	if err != nil {
		return "", err
	}

	if _, err := tempFile.WriteString(scriptContent); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", err
	}

	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	// Make script executable
	if err := os.Chmod(tempFile.Name(), 0755); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// finishExecution completes the execution record
func (e *Executor) finishExecution(execution *JobExecution) {
	now := time.Now()
	execution.EndTime = &now
	execution.Duration = now.Sub(execution.StartTime)

	// Log completion
	switch execution.Status {
	case JobStatusSuccess:
		logging.LogWorkspace(execution.WorkspaceID, "JOB %s: Completed successfully (duration: %v)",
			execution.JobName, execution.Duration.Round(time.Second))
	case JobStatusFailed:
		logging.LogWorkspace(execution.WorkspaceID, "JOB %s: Failed (duration: %v, exit: %d): %s",
			execution.JobName, execution.Duration.Round(time.Second), execution.ExitCode, execution.Error)
	case JobStatusTimeout:
		logging.LogWorkspace(execution.WorkspaceID, "JOB %s: Timed out (duration: %v)",
			execution.JobName, execution.Duration.Round(time.Second))
	}
}

// KillJob attempts to kill a running job by PID
func (e *Executor) KillJob(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// Send SIGTERM first for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// If SIGTERM fails, try SIGKILL
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process %d: %w", pid, err)
		}
	}

	return nil
}