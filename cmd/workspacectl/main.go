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
  deploy WORKSPACE      Deploy specific workspace immediately
  destroy WORKSPACE     Destroy specific workspace immediately
  status [WORKSPACE]    Show status of all workspaces or specific workspace
  list [--detailed]       List all configured workspaces
  logs WORKSPACE        Show recent logs for specific workspace
  add NAME [OPTIONS]      Add new workspace
  show NAME               Show detailed workspace information
  update NAME [OPTIONS]   Update existing workspace
  remove NAME [--force]   Remove workspace
  validate NAME|--all     Validate workspace configuration

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
  %s deploy my-app                          # Deploy 'my-app' immediately
  %s destroy test-workspace                       # Destroy 'test-workspace' immediately
  %s status                                 # Show status of all workspaces
  %s status my-app                          # Show detailed status of 'my-app'
  %s logs my-app                            # Show recent logs for 'my-app'
  %s add dev-server --template web-app      # Add workspace using template
  %s update my-app --deploy-schedule "0 9 * * 1-5"  # Update deploy schedule

Related Tools:
  provisioner      Workspace scheduler daemon
  templatectl      Template management CLI
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func main() {
	// Parse command-line arguments
	if len(os.Args) >= 2 {
		command := os.Args[1]

		// Handle manual operations (deploy/destroy)
		if command == "deploy" || command == "destroy" {
			if len(os.Args) != 3 {
				fmt.Fprintf(os.Stderr, "Error: %s command requires exactly one workspace name\n\n", command)
				printUsage()
				os.Exit(2)
			}

			workspaceName := os.Args[2]
			if err := runManualOperation(command, workspaceName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle status command (can take optional workspace name)
		if command == "status" {
			workspaceName := ""
			if len(os.Args) == 3 {
				workspaceName = os.Args[2]
			} else if len(os.Args) > 3 {
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
			if err := workspace.RunListCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle logs command (requires workspace name)
		if command == "logs" {
			if len(os.Args) != 3 {
				fmt.Fprintf(os.Stderr, "Error: logs command requires exactly one workspace name\n\n")
				printUsage()
				os.Exit(2)
			}

			workspaceName := os.Args[2]
			if err := runLogsCommand(workspaceName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle workspace management commands
		switch command {
		case "add":
			if err := workspace.RunAddCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "show":
			if err := workspace.RunShowCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "update":
			if err := workspace.RunUpdateCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "remove":
			if err := workspace.RunRemoveCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "validate":
			if err := workspace.RunValidateCommand(os.Args[2:]); err != nil {
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

	// Parse flags for version/help commands
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
