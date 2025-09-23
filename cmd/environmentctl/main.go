package main

import (
	"flag"
	"fmt"
	"os"

	"provisioner/pkg/environment"
	"provisioner/pkg/scheduler"
	"provisioner/pkg/version"
)

func printUsage() {
	fmt.Printf(`Usage: %s COMMAND [ARGUMENTS...]

Environment management CLI for OpenTofu Environment Scheduler.

Commands:
  deploy ENVIRONMENT      Deploy specific environment immediately
  destroy ENVIRONMENT     Destroy specific environment immediately
  status [ENVIRONMENT]    Show status of all environments or specific environment
  list [--detailed]       List all configured environments
  logs ENVIRONMENT        Show recent logs for specific environment
  add NAME [OPTIONS]      Add new environment
  show NAME               Show detailed environment information
  update NAME [OPTIONS]   Update existing environment
  remove NAME [--force]   Remove environment
  validate NAME|--all     Validate environment configuration

Add/Update Options:
  --template TEMPLATE            Use specified template
  --description DESC             Environment description
  --deploy-schedule CRON         Deploy schedule (CRON expression)
  --destroy-schedule CRON        Destroy schedule (CRON expression)
  --disabled                     Create disabled environment (add only)
  --enable/--disable             Enable/disable environment (update only)

Global Options:
  --help                         Show this help
  --version                      Show version
  --version-full                 Show detailed version

Examples:
  %s list                                    # List all environments
  %s deploy my-app                          # Deploy 'my-app' immediately
  %s destroy test-env                       # Destroy 'test-env' immediately
  %s status                                 # Show status of all environments
  %s status my-app                          # Show detailed status of 'my-app'
  %s logs my-app                            # Show recent logs for 'my-app'
  %s add dev-server --template web-app      # Add environment using template
  %s update my-app --deploy-schedule "0 9 * * 1-5"  # Update deploy schedule

Related Tools:
  provisioner      Environment scheduler daemon
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
				fmt.Fprintf(os.Stderr, "Error: %s command requires exactly one environment name\n\n", command)
				printUsage()
				os.Exit(2)
			}

			envName := os.Args[2]
			if err := runManualOperation(command, envName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle status command (can take optional environment name)
		if command == "status" {
			envName := ""
			if len(os.Args) == 3 {
				envName = os.Args[2]
			} else if len(os.Args) > 3 {
				fmt.Fprintf(os.Stderr, "Error: status command accepts at most one environment name\n\n")
				printUsage()
				os.Exit(2)
			}

			if err := runStatusCommand(envName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle list command
		if command == "list" {
			if err := environment.RunListCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle logs command (requires environment name)
		if command == "logs" {
			if len(os.Args) != 3 {
				fmt.Fprintf(os.Stderr, "Error: logs command requires exactly one environment name\n\n")
				printUsage()
				os.Exit(2)
			}

			envName := os.Args[2]
			if err := runLogsCommand(envName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle environment management commands
		switch command {
		case "add":
			if err := environment.RunAddCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "show":
			if err := environment.RunShowCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "update":
			if err := environment.RunUpdateCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "remove":
			if err := environment.RunRemoveCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "validate":
			if err := environment.RunValidateCommand(os.Args[2:]); err != nil {
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

func runManualOperation(command, envName string) error {
	// Initialize scheduler in quiet mode for CLI
	sched := scheduler.NewQuiet()

	// Load environments to validate the specified environment exists
	if err := sched.LoadEnvironments(); err != nil {
		return fmt.Errorf("failed to load environments: %w", err)
	}

	// Load state to check current environment status
	if err := sched.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Execute the manual operation
	switch command {
	case "deploy":
		return sched.ManualDeploy(envName)
	case "destroy":
		return sched.ManualDestroy(envName)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func runStatusCommand(envName string) error {
	// Initialize scheduler in quiet mode for CLI
	sched := scheduler.NewQuiet()

	// Use the ShowStatus method
	return sched.ShowStatus(envName)
}

func runLogsCommand(envName string) error {
	// Initialize scheduler in quiet mode for CLI
	sched := scheduler.NewQuiet()

	// Use the ShowLogs method
	return sched.ShowLogs(envName)
}