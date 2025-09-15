package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"environment-scheduler/pkg/scheduler"
	"environment-scheduler/pkg/version"
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

	log.Printf("Starting Environment Scheduler %s", version.GetVersion())

	// Initialize scheduler
	sched := scheduler.New()

	// Load environments and state
	if err := sched.LoadEnvironments(); err != nil {
		log.Printf("Error loading environments: %v", err)
	}

	if err := sched.LoadState(); err != nil {
		log.Printf("Error loading state: %v", err)
	}

	// Start scheduler
	go sched.Start()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Environment Scheduler started. Press Ctrl+C to stop.")

	<-sigChan
	log.Println("Shutting down...")

	// Save state on shutdown
	if err := sched.SaveState(); err != nil {
		log.Printf("Error saving state: %v", err)
	}

	log.Println("Environment Scheduler stopped.")
}