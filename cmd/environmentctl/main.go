package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"provisioner/pkg/environment"
	"provisioner/pkg/version"
)

func main() {
	if len(os.Args) < 2 {
		showUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "status":
		handleStatus(os.Args[2:])
	case "switch":
		handleSwitch(os.Args[2:])
	case "list":
		handleList(os.Args[2:])
	case "version", "--version":
		showVersion()
	case "help", "--help":
		showUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		showUsage()
		os.Exit(1)
	}
}

func showUsage() {
	fmt.Println("environmentctl - DigitalOcean Environment Management")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  environmentctl status [ENVIRONMENT]    Show environment status")
	fmt.Println("  environmentctl switch ENV WORKSPACE    Switch environment to workspace")
	fmt.Println("  environmentctl list                    List all environments")
	fmt.Println("  environmentctl version                 Show version information")
	fmt.Println("  environmentctl help                    Show this help message")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  environmentctl status                  Show all environments")
	fmt.Println("  environmentctl status production       Show production environment only")
	fmt.Println("  environmentctl switch production blue  Switch production to blue workspace")
	fmt.Println("  environmentctl list                    List configured environments")
}

func showVersion() {
	info := version.GetBuildInfo()
	fmt.Printf("environmentctl %s\n", info.Version)
	fmt.Printf("Built: %s\n", info.BuildTime)
	fmt.Printf("Commit: %s\n", info.GitCommit)
}

func handleStatus(args []string) {
	if len(args) == 0 {
		// Show all environments
		showAllEnvironments()
	} else if len(args) == 1 {
		// Show specific environment
		environmentName := args[0]
		showEnvironment(environmentName)
	} else {
		fmt.Println("Usage: environmentctl status [ENVIRONMENT]")
		os.Exit(1)
	}
}

func handleSwitch(args []string) {
	if len(args) != 2 {
		fmt.Println("Usage: environmentctl switch ENVIRONMENT WORKSPACE")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  environmentctl switch production blue")
		os.Exit(1)
	}

	environmentName := args[0]
	workspaceName := args[1]

	performSwitch(environmentName, workspaceName)
}

func handleList(args []string) {
	if len(args) != 0 {
		fmt.Println("Usage: environmentctl list")
		os.Exit(1)
	}

	listEnvironments()
}

func showAllEnvironments() {
	environments, err := environment.LoadAllEnvironments()
	if err != nil {
		fmt.Printf("Error loading environments: %v\n", err)
		os.Exit(1)
	}

	if len(environments) == 0 {
		fmt.Println("No environments configured.")
		fmt.Println("Environment configurations should be placed in /etc/provisioner/ or current directory.")
		return
	}

	// Sort environments by name for consistent output
	sort.Slice(environments, func(i, j int) bool {
		return environments[i].Name < environments[j].Name
	})

	fmt.Println("Environment Status:")
	fmt.Println("")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ENVIRONMENT\tDOMAIN\tASSIGNED WORKSPACE\tRESERVED IPs\tHEALTH CHECK")
	fmt.Fprintln(w, "-----------\t------\t------------------\t------------\t------------")

	for _, env := range environments {
		reservedIPsStr := strings.Join(env.Config.ReservedIPs, ", ")
		healthCheckStr := fmt.Sprintf("%s", env.Config.HealthCheck.Type)
		if env.Config.HealthCheck.Port > 0 {
			healthCheckStr += fmt.Sprintf(":%d", env.Config.HealthCheck.Port)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			env.Name,
			env.Config.Domain,
			env.Config.AssignedWorkspace,
			reservedIPsStr,
			healthCheckStr)
	}

	w.Flush()
}

func showEnvironment(environmentName string) {
	env, err := environment.LoadEnvironment(environmentName)
	if err != nil {
		fmt.Printf("Error loading environment '%s': %v\n", environmentName, err)
		os.Exit(1)
	}

	fmt.Printf("Environment: %s\n", env.Name)
	fmt.Printf("Configuration file: %s\n", env.Path)
	fmt.Printf("Domain: %s\n", env.Config.Domain)
	fmt.Printf("Assigned workspace: %s\n", env.Config.AssignedWorkspace)
	fmt.Printf("Reserved IPs: %s\n", strings.Join(env.Config.ReservedIPs, ", "))
	fmt.Printf("Health check: %s", env.Config.HealthCheck.Type)

	switch env.Config.HealthCheck.Type {
	case "http":
		fmt.Printf(" %s:%d (timeout: %s)", env.Config.HealthCheck.Path, env.Config.HealthCheck.Port, env.Config.HealthCheck.Timeout)
	case "tcp":
		fmt.Printf(" port %d (timeout: %s)", env.Config.HealthCheck.Port, env.Config.HealthCheck.Timeout)
	case "command":
		fmt.Printf(" '%s' (timeout: %s)", env.Config.HealthCheck.Command, env.Config.HealthCheck.Timeout)
	}
	fmt.Println("")

	// Perform health check on current environment
	fmt.Printf("\nPerforming health check on current workspace '%s'...\n", env.Config.AssignedWorkspace)
	performHealthCheck(env)
}

func listEnvironments() {
	environments, err := environment.LoadAllEnvironments()
	if err != nil {
		fmt.Printf("Error loading environments: %v\n", err)
		os.Exit(1)
	}

	if len(environments) == 0 {
		fmt.Println("No environments configured.")
		return
	}

	// Sort environments by name
	sort.Slice(environments, func(i, j int) bool {
		return environments[i].Name < environments[j].Name
	})

	fmt.Println("Configured environments:")
	for _, env := range environments {
		fmt.Printf("  %s (assigned to: %s)\n", env.Name, env.Config.AssignedWorkspace)
	}
}

func performSwitch(environmentName, workspaceName string) {
	fmt.Printf("Switching environment '%s' to workspace '%s'...\n", environmentName, workspaceName)

	// Load environment
	env, err := environment.LoadEnvironment(environmentName)
	if err != nil {
		fmt.Printf("Error: Failed to load environment '%s': %v\n", environmentName, err)
		os.Exit(1)
	}

	// Check if already assigned to this workspace
	if env.Config.AssignedWorkspace == workspaceName {
		fmt.Printf("Environment '%s' is already assigned to workspace '%s'\n", environmentName, workspaceName)
		return
	}

	// Confirm the switch
	fmt.Printf("Current assignment: %s -> %s\n", environmentName, env.Config.AssignedWorkspace)
	fmt.Printf("New assignment: %s -> %s\n", environmentName, workspaceName)
	fmt.Printf("Reserved IPs to switch: %s\n", strings.Join(env.Config.ReservedIPs, ", "))
	fmt.Printf("\nThis will switch production traffic. Continue? (y/N): ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		fmt.Println("\nCancelled.")
		return
	}

	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Println("Cancelled.")
		return
	}

	// Perform the switch
	switchOp := &environment.SwitchOperation{
		Environment:     env,
		TargetWorkspace: workspaceName,
	}

	fmt.Println("\n--- Starting Environment Switch ---")
	result := switchOp.PerformSwitch()

	if result.Success {
		fmt.Printf("✓ Success: %s\n", result.Message)
		fmt.Printf("Environment '%s' is now assigned to workspace '%s'\n", environmentName, workspaceName)
	} else {
		fmt.Printf("✗ Failed: %s\n", result.Message)
		if result.Error != nil {
			fmt.Printf("Error details: %v\n", result.Error)
		}
		if result.RollbackRequired {
			fmt.Println("Rollback may be required. Check Reserved IP assignments manually.")
		}
		os.Exit(1)
	}
}

func performHealthCheck(env *environment.Environment) {
	// This is a basic implementation - in a full implementation,
	// we would get the current workspace's load balancer IPs and test them

	fmt.Printf("Health check type: %s\n", env.Config.HealthCheck.Type)
	fmt.Printf("Timeout: %s\n", env.Config.HealthCheck.Timeout)

	// For now, just show the configuration
	// A full implementation would:
	// 1. Get the current workspace's load_balancer_ips terraform output
	// 2. Perform health checks on each IP
	// 3. Report the results

	fmt.Println("Note: Full health check implementation requires workspace deployment information")
	fmt.Printf("Use 'workspacectl status %s' to verify workspace deployment status\n", env.Config.AssignedWorkspace)
}