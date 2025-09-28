package main

import (
	"flag"
	"fmt"
	"os"

	"provisioner/pkg/job"
	"provisioner/pkg/scheduler"
	"provisioner/pkg/version"
)

func printUsage() {
	fmt.Printf(`Usage: %s [OPTIONS] COMMAND [ARGUMENTS...]

Job management CLI for OpenTofu Workspace Scheduler.

Commands:
  list [JOB]                   List all jobs or show specific job details
  status [JOB]                 Show status of all jobs or specific job
  run JOB                      Run specific job immediately
  kill JOB                     Kill running job
  logs JOB                     Show recent logs for specific job (coming soon)

Options:
  --workspace NAME             Operate on jobs within the specified workspace
  --help                       Show this help
  --version                    Show version
  --version-full               Show detailed version

Examples:
  # Standalone jobs (default)
  %s list                              # List all standalone jobs
  %s status                            # Show status of all standalone jobs
  %s status cleanup-temp               # Show status of 'cleanup-temp' standalone job
  %s run cleanup-temp                  # Run 'cleanup-temp' standalone job immediately
  %s kill long-job                     # Kill running standalone job

  # Workspace jobs (with --workspace flag)
  %s --workspace my-app list           # List all jobs in 'my-app' workspace
  %s --workspace my-app status         # Show status of all jobs in 'my-app'
  %s --workspace my-app status backup-db # Show status of 'backup-db' job
  %s --workspace my-app run backup-db  # Run 'backup-db' job immediately
  %s --workspace my-app kill backup-db # Kill running job

Notes:
  By default, jobctl operates on standalone jobs (defined in jobs/ directory).
  Use --workspace flag to operate on jobs within a specific workspace.
  Workspace jobs are defined in workspace configuration files (workspaces/*/config.json).

Related Tools:
  provisioner      Workspace scheduler daemon
  workspacectl     Workspace management CLI
  templatectl      Template management CLI
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func main() {
	var workspaceName = flag.String("workspace", "", "Operate on jobs within the specified workspace")
	var showVersion = flag.Bool("version", false, "Show version information")
	var showFullVersion = flag.Bool("version-full", false, "Show detailed version information")
	var showHelp = flag.Bool("help", false, "Show help information")

	flag.Usage = printUsage
	flag.Parse()

	if *showHelp {
		printUsage()
		return
	}

	if *showVersion {
		fmt.Println(version.GetVersion())
		return
	}

	if *showFullVersion {
		fmt.Println(version.GetFullVersion())
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no command specified\n\n")
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	// Route to workspace or standalone job handlers
	if *workspaceName != "" {
		handleWorkspaceJob(*workspaceName, command, args[1:])
	} else {
		handleStandaloneJob(command, args[1:])
	}
}

func handleStandaloneJob(command string, args []string) {
	switch command {
	case "list":
		if len(args) > 0 {
			fmt.Fprintf(os.Stderr, "Error: list command takes no arguments\n\n")
			printUsage()
			os.Exit(2)
		}
		if err := runStandaloneListCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "status":
		jobName := ""
		if len(args) > 0 {
			if len(args) != 1 {
				fmt.Fprintf(os.Stderr, "Error: status command takes optional job name\n\n")
				printUsage()
				os.Exit(2)
			}
			jobName = args[0]
		}
		if err := runStandaloneStatusCommand(jobName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "run":
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "Error: run command requires job name\n\n")
			printUsage()
			os.Exit(2)
		}
		jobName := args[0]
		if err := runStandaloneRunCommand(jobName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "kill":
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "Error: kill command requires job name\n\n")
			printUsage()
			os.Exit(2)
		}
		jobName := args[0]
		if err := runStandaloneKillCommand(jobName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "logs":
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "Error: logs command requires job name\n\n")
			printUsage()
			os.Exit(2)
		}
		fmt.Printf("Job logs feature coming soon!\n")
		fmt.Printf("For now, check system logs: journalctl -u provisioner\n")

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func handleWorkspaceJob(workspaceName, command string, args []string) {
	switch command {
	case "list":
		if len(args) > 0 {
			fmt.Fprintf(os.Stderr, "Error: list command takes no arguments when using --workspace\n\n")
			printUsage()
			os.Exit(2)
		}
		if err := runWorkspaceListCommand(workspaceName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "status":
		jobName := ""
		if len(args) > 0 {
			if len(args) != 1 {
				fmt.Fprintf(os.Stderr, "Error: status command takes optional job name\n\n")
				printUsage()
				os.Exit(2)
			}
			jobName = args[0]
		}
		if err := runWorkspaceStatusCommand(workspaceName, jobName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "run":
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "Error: run command requires job name\n\n")
			printUsage()
			os.Exit(2)
		}
		jobName := args[0]
		if err := runWorkspaceJobCommand(workspaceName, jobName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "kill":
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "Error: kill command requires job name\n\n")
			printUsage()
			os.Exit(2)
		}
		jobName := args[0]
		if err := runWorkspaceKillCommand(workspaceName, jobName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "logs":
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "Error: logs command requires job name\n\n")
			printUsage()
			os.Exit(2)
		}
		fmt.Printf("Job logs feature coming soon!\n")
		fmt.Printf("For now, check workspace logs: workspacectl logs %s\n", workspaceName)

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

// Standalone job functions

func runStandaloneListCommand() error {
	sched := scheduler.NewQuiet()
	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	standaloneJobManager := sched.GetStandaloneJobManager()
	if standaloneJobManager == nil {
		return fmt.Errorf("standalone job manager not available")
	}

	jobs, err := standaloneJobManager.ListStandaloneJobs()
	if err != nil {
		return fmt.Errorf("failed to load standalone jobs: %w", err)
	}

	if len(jobs) == 0 {
		fmt.Printf("No standalone jobs configured\n")
		return nil
	}

	fmt.Printf("%-20s %-10s %-15s %-30s\n", "JOB NAME", "TYPE", "ENABLED", "DESCRIPTION")
	fmt.Printf("%-20s %-10s %-15s %-30s\n", "--------", "----", "-------", "-----------")

	for _, job := range jobs {
		enabled := "false"
		if job.Enabled {
			enabled = "true"
		}

		description := job.Description
		if len(description) > 30 {
			description = description[:27] + "..."
		}

		fmt.Printf("%-20s %-10s %-15s %-30s\n",
			job.Name,
			job.Type,
			enabled,
			description)
	}

	return nil
}

func runStandaloneStatusCommand(jobName string) error {
	sched := scheduler.NewQuiet()
	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}
	if err := sched.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if jobManager := sched.GetJobManager(); jobManager != nil {
		if err := jobManager.LoadState(); err != nil {
			return fmt.Errorf("failed to load job state: %w", err)
		}
	}

	standaloneJobManager := sched.GetStandaloneJobManager()
	if standaloneJobManager == nil {
		return fmt.Errorf("standalone job manager not available")
	}

	if jobName != "" {
		return showStandaloneJobStatus(standaloneJobManager, jobName)
	} else {
		return showAllStandaloneJobsStatus(standaloneJobManager)
	}
}

func runStandaloneRunCommand(jobName string) error {
	sched := scheduler.NewQuiet()
	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}
	if err := sched.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if jobManager := sched.GetJobManager(); jobManager != nil {
		if err := jobManager.LoadState(); err != nil {
			return fmt.Errorf("failed to load job state: %w", err)
		}
	}

	standaloneJobManager := sched.GetStandaloneJobManager()
	if standaloneJobManager == nil {
		return fmt.Errorf("standalone job manager not available")
	}

	fmt.Printf("Running standalone job '%s'...\n", jobName)

	if err := standaloneJobManager.ExecuteStandaloneJob(jobName); err != nil {
		return fmt.Errorf("failed to execute standalone job: %w", err)
	}

	fmt.Printf("Standalone job '%s' completed successfully\n", jobName)
	return nil
}

func runStandaloneKillCommand(jobName string) error {
	sched := scheduler.NewQuiet()
	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}
	if err := sched.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	standaloneJobManager := sched.GetStandaloneJobManager()
	if standaloneJobManager == nil {
		return fmt.Errorf("standalone job manager not available")
	}

	fmt.Printf("Killing standalone job '%s'...\n", jobName)

	if err := standaloneJobManager.KillStandaloneJob(jobName); err != nil {
		return fmt.Errorf("failed to kill standalone job: %w", err)
	}

	fmt.Printf("Standalone job '%s' killed successfully\n", jobName)
	return nil
}

// Workspace job functions

func runWorkspaceListCommand(workspaceName string) error {
	sched := scheduler.NewQuiet()

	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	workspace := sched.GetWorkspace(workspaceName)
	if workspace == nil {
		return fmt.Errorf("workspace '%s' not found", workspaceName)
	}

	jobConfigs := workspace.Config.GetJobConfigs()
	if len(jobConfigs) == 0 {
		fmt.Printf("No jobs defined for workspace '%s'\n", workspaceName)
		return nil
	}

	fmt.Printf("%-20s %-10s %-15s %-30s\n", "JOB NAME", "TYPE", "ENABLED", "DESCRIPTION")
	fmt.Printf("%-20s %-10s %-15s %-30s\n", "--------", "----", "-------", "-----------")

	for _, jobConfig := range jobConfigs {
		enabled := "false"
		if jobConfig.Enabled {
			enabled = "true"
		}

		description := jobConfig.Description
		if len(description) > 30 {
			description = description[:27] + "..."
		}

		fmt.Printf("%-20s %-10s %-15s %-30s\n",
			jobConfig.Name,
			jobConfig.Type,
			enabled,
			description)
	}

	return nil
}

func runWorkspaceStatusCommand(workspaceName, jobName string) error {
	sched := scheduler.NewQuiet()

	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}
	if err := sched.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if jobManager := sched.GetJobManager(); jobManager != nil {
		if err := jobManager.LoadState(); err != nil {
			return fmt.Errorf("failed to load job state: %w", err)
		}
	}

	if jobName != "" {
		return showWorkspaceJobStatus(sched, workspaceName, jobName)
	} else {
		return showAllWorkspaceJobsStatus(sched, workspaceName)
	}
}

func runWorkspaceJobCommand(workspaceName, jobName string) error {
	sched := scheduler.NewQuiet()

	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	if err := sched.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	fmt.Printf("Running job '%s' in workspace '%s'...\n", jobName, workspaceName)

	if err := sched.ManualExecuteJob(workspaceName, jobName); err != nil {
		return fmt.Errorf("failed to execute job: %w", err)
	}

	fmt.Printf("Job '%s' completed successfully\n", jobName)
	return nil
}

func runWorkspaceKillCommand(workspaceName, jobName string) error {
	sched := scheduler.NewQuiet()

	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}
	if err := sched.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	fmt.Printf("Killing job '%s' in workspace '%s'...\n", jobName, workspaceName)

	if err := sched.KillJob(workspaceName, jobName); err != nil {
		return fmt.Errorf("failed to kill job: %w", err)
	}

	fmt.Printf("Job '%s' killed successfully\n", jobName)
	return nil
}

// Status display functions

func showStandaloneJobStatus(standaloneJobManager *job.StandaloneJobManager, jobName string) error {
	jobStates := standaloneJobManager.GetStandaloneJobStates()
	jobState, exists := jobStates[jobName]
	if !exists {
		return fmt.Errorf("standalone job '%s' not found", jobName)
	}

	fmt.Printf("Job: %s\n", jobName)
	fmt.Printf("Type: standalone\n")
	fmt.Printf("Status: %s\n", jobState.Status)
	fmt.Printf("Run Count: %d\n", jobState.RunCount)
	fmt.Printf("Success Count: %d\n", jobState.SuccessCount)
	fmt.Printf("Failure Count: %d\n", jobState.FailureCount)

	if jobState.LastRun != nil {
		fmt.Printf("Last Run: %s\n", jobState.LastRun.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Last Run: Never\n")
	}

	if jobState.LastSuccess != nil {
		fmt.Printf("Last Success: %s\n", jobState.LastSuccess.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Last Success: Never\n")
	}

	if jobState.LastFailure != nil {
		fmt.Printf("Last Failure: %s\n", jobState.LastFailure.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Last Failure: Never\n")
	}

	if jobState.LastError != "" {
		fmt.Printf("Last Error: %s\n", jobState.LastError)
	}

	if jobState.NextRun != nil {
		fmt.Printf("Next Run: %s\n", jobState.NextRun.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func showAllStandaloneJobsStatus(standaloneJobManager *job.StandaloneJobManager) error {
	jobs, err := standaloneJobManager.ListStandaloneJobs()
	if err != nil {
		return fmt.Errorf("failed to list standalone jobs: %w", err)
	}

	if len(jobs) == 0 {
		fmt.Printf("No standalone jobs configured\n")
		return nil
	}

	jobStates := standaloneJobManager.GetStandaloneJobStates()

	fmt.Printf("Standalone jobs:\n\n")
	fmt.Printf("%-20s %-12s %-8s %-8s %-20s\n", "JOB NAME", "STATUS", "SUCCESS", "FAILED", "LAST RUN")
	fmt.Printf("%-20s %-12s %-8s %-8s %-20s\n", "--------", "------", "-------", "------", "--------")

	for _, jobConfig := range jobs {
		status := "pending"
		successCount := 0
		failureCount := 0
		lastRun := "Never"

		if !jobConfig.Enabled {
			status = "disabled"
		}

		if jobState, exists := jobStates[jobConfig.Name]; exists {
			status = string(jobState.Status)
			successCount = jobState.SuccessCount
			failureCount = jobState.FailureCount
			if jobState.LastRun != nil {
				lastRun = jobState.LastRun.Format("2006-01-02 15:04")
			}
		}

		fmt.Printf("%-20s %-12s %-8d %-8d %-20s\n",
			jobConfig.Name,
			status,
			successCount,
			failureCount,
			lastRun)
	}

	return nil
}

func showWorkspaceJobStatus(sched *scheduler.Scheduler, workspaceName, jobName string) error {
	jobState := sched.GetJobState(workspaceName, jobName)
	if jobState == nil {
		return fmt.Errorf("job '%s' not found in workspace '%s'", jobName, workspaceName)
	}

	fmt.Printf("Job: %s\n", jobName)
	fmt.Printf("Workspace: %s\n", workspaceName)
	fmt.Printf("Status: %s\n", jobState.Status)
	fmt.Printf("Run Count: %d\n", jobState.RunCount)
	fmt.Printf("Success Count: %d\n", jobState.SuccessCount)
	fmt.Printf("Failure Count: %d\n", jobState.FailureCount)

	if jobState.LastRun != nil {
		fmt.Printf("Last Run: %s\n", jobState.LastRun.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Last Run: Never\n")
	}

	if jobState.LastSuccess != nil {
		fmt.Printf("Last Success: %s\n", jobState.LastSuccess.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Last Success: Never\n")
	}

	if jobState.LastFailure != nil {
		fmt.Printf("Last Failure: %s\n", jobState.LastFailure.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Last Failure: Never\n")
	}

	if jobState.LastError != "" {
		fmt.Printf("Last Error: %s\n", jobState.LastError)
	}

	if jobState.NextRun != nil {
		fmt.Printf("Next Run: %s\n", jobState.NextRun.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func showAllWorkspaceJobsStatus(sched *scheduler.Scheduler, workspaceName string) error {
	workspace := sched.GetWorkspace(workspaceName)
	if workspace == nil {
		return fmt.Errorf("workspace '%s' not found", workspaceName)
	}

	jobConfigs := workspace.Config.GetJobConfigs()
	if len(jobConfigs) == 0 {
		fmt.Printf("No jobs defined for workspace '%s'\n", workspaceName)
		return nil
	}

	jobStates := sched.GetJobStates(workspaceName)

	fmt.Printf("Jobs in workspace '%s':\n\n", workspaceName)
	fmt.Printf("%-20s %-12s %-8s %-8s %-20s\n", "JOB NAME", "STATUS", "SUCCESS", "FAILED", "LAST RUN")
	fmt.Printf("%-20s %-12s %-8s %-8s %-20s\n", "--------", "------", "-------", "------", "--------")

	for _, jobConfig := range jobConfigs {
		status := "pending"
		successCount := 0
		failureCount := 0
		lastRun := "Never"

		if !jobConfig.Enabled {
			status = "disabled"
		}

		if jobState, exists := jobStates[jobConfig.Name]; exists {
			status = string(jobState.Status)
			successCount = jobState.SuccessCount
			failureCount = jobState.FailureCount
			if jobState.LastRun != nil {
				lastRun = jobState.LastRun.Format("2006-01-02 15:04")
			}
		}

		fmt.Printf("%-20s %-12s %-8d %-8d %-20s\n",
			jobConfig.Name,
			status,
			successCount,
			failureCount,
			lastRun)
	}

	return nil
}
