package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"provisioner/pkg/logging"
	"provisioner/pkg/scheduler"
	"provisioner/pkg/version"
)

func printUsage() {
	fmt.Printf(`Usage: %s [COMMAND] [ENVIRONMENT]

Commands:
  deploy ENVIRONMENT    Deploy specific environment immediately
  destroy ENVIRONMENT   Destroy specific environment immediately
  status [ENVIRONMENT]  Show status of all environments or specific environment
  list                  List all configured environments
  logs ENVIRONMENT      Show recent logs for specific environment

Options:
  --help               Show this help
  --version           Show version
  --version-full      Show detailed version

If no command is specified, runs the scheduler daemon.

Examples:
  %s deploy my-app        # Deploy 'my-app' environment immediately
  %s destroy test-env     # Destroy 'test-env' environment immediately
  %s status               # Show status of all environments
  %s status my-app        # Show detailed status of 'my-app' environment
  %s list                 # List all configured environments
  %s logs my-app          # Show recent logs for 'my-app' environment
  %s                      # Run scheduler daemon (default)
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func main() {
	// Parse command-line arguments for manual operations first
	if len(os.Args) >= 2 {
		command := os.Args[1]

		// Handle manual operations
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

		// Handle list command (no arguments)
		if command == "list" {
			if len(os.Args) != 2 {
				fmt.Fprintf(os.Stderr, "Error: list command takes no arguments\n\n")
				printUsage()
				os.Exit(2)
			}

			if err := runListCommand(); err != nil {
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

	logging.LogSystemd("Starting Environment Scheduler %s", version.GetVersion())

	// Initialize scheduler
	sched := scheduler.New()

	// Load environments and state
	if err := sched.LoadEnvironments(); err != nil {
		logging.LogSystemd("Error loading environments: %v", err)
	}

	if err := sched.LoadState(); err != nil {
		logging.LogSystemd("Error loading state: %v", err)
	}

	// Start scheduler
	go sched.Start()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logging.LogSystemd("Environment Scheduler started. Press Ctrl+C to stop.")

	<-sigChan
	logging.LogSystemd("Shutting down...")

	// Save state on shutdown
	if err := sched.SaveState(); err != nil {
		logging.LogSystemd("Error saving state: %v", err)
	}

	// Close log files
	logging.GetLogger().Close()

	logging.LogSystemd("Environment Scheduler stopped.")
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

func runListCommand() error {
	// Initialize scheduler in quiet mode for CLI
	sched := scheduler.NewQuiet()

	// Use the ListEnvironments method
	return sched.ListEnvironments()
}

func runLogsCommand(envName string) error {
	// Initialize scheduler in quiet mode for CLI
	sched := scheduler.NewQuiet()

	// Use the ShowLogs method
	return sched.ShowLogs(envName)
}
