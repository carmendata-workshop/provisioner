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
	fmt.Printf(`Usage: %s [OPTIONS]

OpenTofu Environment Scheduler - Automatically manages OpenTofu environments on CRON schedules.

This daemon runs in the background and deploys/destroys environments based on their configured schedules.

Related Tools:
  environmentctl    Manage environments (list, deploy, destroy, status, logs)
  templatectl      Manage templates (add, list, show, update, remove)

Options:
  --help           Show this help
  --version        Show version
  --version-full   Show detailed version

Examples:
  %s               # Run scheduler daemon (default)
  %s --version     # Show version information

For manual operations, use the related CLI tools:
  environmentctl list              # List all environments
  environmentctl deploy my-app     # Deploy environment immediately
  environmentctl status my-app     # Show environment status
  templatectl list                 # List all templates
`, os.Args[0], os.Args[0], os.Args[0])
}

func main() {
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

	// Check for any non-flag arguments
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unknown argument '%s'\n\n", flag.Arg(0))
		printUsage()
		os.Exit(1)
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