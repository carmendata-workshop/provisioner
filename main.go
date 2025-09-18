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

func main() {
	var showVersion = flag.Bool("version", false, "Show version information")
	var showFullVersion = flag.Bool("version-full", false, "Show detailed version information")
	var showHelp = flag.Bool("help", false, "Show help information")
	flag.Parse()

	if *showHelp {
		flag.Usage()
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
