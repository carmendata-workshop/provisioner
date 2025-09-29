package main

import (
	"flag"
	"fmt"
	"os"

	"provisioner/pkg/scheduler"
	"provisioner/pkg/version"
	"provisioner/pkg/workspace"
)

func printUsage() {
	fmt.Printf(`Usage: %s COMMAND [ARGUMENTS...]

Workspace management CLI for OpenTofu Workspace Scheduler.

Commands:
  deploy WORKSPACE [MODE]  Deploy specific workspace immediately (with optional mode)
  destroy WORKSPACE        Destroy specific workspace immediately
  mode WORKSPACE MODE      Change workspace to specific mode
  status [WORKSPACE]       Show status of all workspaces or specific workspace
  list [--detailed]        List all configured workspaces
  logs WORKSPACE           Show recent logs for specific workspace
  add NAME [OPTIONS]       Add new workspace
  show NAME                Show detailed workspace information
  update NAME [OPTIONS]    Update existing workspace
  remove NAME [--force]    Remove workspace
  validate NAME|--all      Validate workspace configuration

Add/Update Options:
  --template TEMPLATE            Use specified template
  --description DESC             Workspace description
  --deploy-schedule CRON         Deploy schedule (CRON expression)
  --destroy-schedule CRON        Destroy schedule (CRON expression)
  --disabled                     Create disabled workspace (add only)
  --enable/--disable             Enable/disable workspace (update only)

Global Options:
  --help                         Show this help
  --version                      Show version
  --version-full                 Show detailed version

Examples:
  %s list                                    # List all workspaces
  %s deploy my-app                          # Deploy 'my-app' (prompts for mode if needed)
  %s deploy my-app busy                     # Deploy 'my-app' in 'busy' mode
  %s mode my-app hibernation                # Change 'my-app' to hibernation mode
  %s destroy test-workspace                 # Destroy 'test-workspace' immediately
  %s status                                 # Show status of all workspaces
  %s status my-app                          # Show detailed status of 'my-app'
  %s logs my-app                            # Show recent logs for 'my-app'
  %s add dev-server --template web-app      # Add workspace using template
  %s update my-app --deploy-schedule "0 9 * * 1-5"  # Update deploy schedule

Related Tools:
  provisioner      Workspace scheduler daemon
  templatectl      Template management CLI
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func main() {
	// Handle flags first (version, help)
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

	// Parse command-line arguments
	args := flag.Args()
	if len(args) >= 1 {
		command := args[0]

		// Handle deploy command (supports optional mode)
		if command == "deploy" {
			if len(args) < 2 || len(args) > 3 {
				fmt.Fprintf(os.Stderr, "Error: deploy command requires workspace name and optional mode\n\n")
				printUsage()
				os.Exit(2)
			}

			workspaceName := args[1]
			var mode string
			if len(args) == 3 {
				mode = args[2]
			}

			if err := runDeployCommand(workspaceName, mode); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle destroy command
		if command == "destroy" {
			if len(args) != 2 {
				fmt.Fprintf(os.Stderr, "Error: destroy command requires exactly one workspace name\n\n")
				printUsage()
				os.Exit(2)
			}

			workspaceName := args[1]
			if err := runManualOperation(command, workspaceName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle mode command
		if command == "mode" {
			if len(args) != 3 {
				fmt.Fprintf(os.Stderr, "Error: mode command requires workspace name and mode\n\n")
				printUsage()
				os.Exit(2)
			}

			workspaceName := args[1]
			mode := args[2]
			if err := runModeCommand(workspaceName, mode); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle status command (can take optional workspace name)
		if command == "status" {
			workspaceName := ""
			if len(args) == 2 {
				workspaceName = args[1]
			} else if len(args) > 2 {
				fmt.Fprintf(os.Stderr, "Error: status command accepts at most one workspace name\n\n")
				printUsage()
				os.Exit(2)
			}

			if err := runStatusCommand(workspaceName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle list command
		if command == "list" {
			if err := workspace.RunListCommand(args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle logs command (requires workspace name)
		if command == "logs" {
			if len(args) != 2 {
				fmt.Fprintf(os.Stderr, "Error: logs command requires exactly one workspace name\n\n")
				printUsage()
				os.Exit(2)
			}

			workspaceName := args[1]
			if err := runLogsCommand(workspaceName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle workspace management commands
		switch command {
		case "add":
			if err := workspace.RunAddCommand(args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "show":
			if err := workspace.RunShowCommand(args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "update":
			if err := workspace.RunUpdateCommand(args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "remove":
			if err := workspace.RunRemoveCommand(args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "validate":
			if err := workspace.RunValidateCommand(args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// If we reach here, it's an unknown command
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n\n", command)
		printUsage()
		os.Exit(1)
	}

	// No command specified
	fmt.Fprintf(os.Stderr, "Error: no command specified\n\n")
	printUsage()
	os.Exit(1)
}

func runManualOperation(command, workspaceName string) error {
	// Initialize scheduler in quiet mode for CLI
	sched := scheduler.NewQuiet()

	// Load workspaces to validate the specified workspace exists
	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	// Load state to check current workspace status
	if err := sched.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Execute the manual operation
	switch command {
	case "deploy":
		return sched.ManualDeploy(workspaceName)
	case "destroy":
		return sched.ManualDestroy(workspaceName)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func runStatusCommand(workspaceName string) error {
	// Initialize scheduler in quiet mode for CLI
	sched := scheduler.NewQuiet()

	// Use the ShowStatus method
	return sched.ShowStatus(workspaceName)
}

func runLogsCommand(workspaceName string) error {
	// Initialize scheduler in quiet mode for CLI
	sched := scheduler.NewQuiet()

	// Use the ShowLogs method
	return sched.ShowLogs(workspaceName)
}

func runDeployCommand(workspaceName, mode string) error {
	// Initialize scheduler in quiet mode for CLI
	sched := scheduler.NewQuiet()

	// Load workspaces to validate the specified workspace exists
	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	// Load state to check current workspace status
	if err := sched.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// If mode is specified, deploy in that mode
	if mode != "" {
		return sched.ManualDeployInMode(workspaceName, mode)
	}

	// Check if workspace uses mode scheduling
	workspace := sched.GetWorkspace(workspaceName)
	if workspace == nil {
		return fmt.Errorf("workspace '%s' not found", workspaceName)
	}

	// Handle mode-based workspaces
	if len(workspace.Config.ModeSchedules) > 0 {
		// Get available modes
		modeSchedules, err := workspace.Config.GetModeSchedules()
		if err != nil {
			return fmt.Errorf("failed to get mode schedules: %w", err)
		}

		modes := make([]string, 0, len(modeSchedules))
		for mode := range modeSchedules {
			modes = append(modes, mode)
		}

		// Prompt user for mode selection
		selectedMode, err := promptForMode(modes)
		if err != nil {
			return err
		}

		return sched.ManualDeployInMode(workspaceName, selectedMode)
	}

	// Handle traditional deploy_schedule workspaces
	return sched.ManualDeploy(workspaceName)
}

func runModeCommand(workspaceName, mode string) error {
	// Initialize scheduler in quiet mode for CLI
	sched := scheduler.NewQuiet()

	// Load workspaces to validate the specified workspace exists
	if err := sched.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	// Load state to check current workspace status
	if err := sched.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Execute the mode change
	return sched.ManualDeployInMode(workspaceName, mode)
}

func promptForMode(modes []string) (string, error) {
	fmt.Printf("Workspace uses mode-based scheduling. Select deployment mode:\n")
	for i, mode := range modes {
		fmt.Printf("%d) %s\n", i+1, mode)
	}

	fmt.Printf("Enter choice (1-%d): ", len(modes))
	var choice int
	if _, err := fmt.Scanln(&choice); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if choice < 1 || choice > len(modes) {
		return "", fmt.Errorf("choice must be between 1 and %d", len(modes))
	}

	return modes[choice-1], nil
}
