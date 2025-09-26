package main

import (
	"flag"
	"fmt"
	"os"

	"provisioner/pkg/template"
	"provisioner/pkg/version"
)

func printUsage() {
	fmt.Printf(`Usage: %s COMMAND [ARGUMENTS...]

Template management CLI for OpenTofu Workspace Scheduler.

Commands:
  add NAME URL [OPTIONS]   Add new template from URL
  list [--detailed]        List all available templates
  show NAME                Show detailed template information
  update NAME|--all        Update template(s) from source
  remove NAME [--force]    Remove template
  validate NAME|--all      Validate template configuration

Add Options:
  --path PATH              Path within repository (default: root)
  --ref REF                Git reference (branch/tag/commit, default: main)
  --description DESC       Template description

Global Options:
  --help                   Show this help
  --version                Show version
  --version-full           Show detailed version

Examples:
  %s list                                        # List all templates
  %s add web-app https://github.com/org/templates --path web --ref v1.0
  %s show web-app                                # Show template details
  %s update web-app                              # Update specific template
  %s update --all                                # Update all templates
  %s remove web-app                              # Remove template
  %s validate --all                              # Validate all templates

Related Tools:
  provisioner      Workspace scheduler daemon
  workspacectl   Workspace management CLI
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func main() {
	// Parse command-line arguments
	if len(os.Args) >= 2 {
		command := os.Args[1]

		// Handle template commands
		switch command {
		case "add":
			if err := template.RunAddCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "list":
			if err := template.RunListCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "show":
			if err := template.RunShowCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "update":
			if err := template.RunUpdateCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "remove":
			if err := template.RunRemoveCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "validate":
			if err := template.RunValidateCommand(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		default:
			// Unknown command
			fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n\n", command)
			printUsage()
			os.Exit(1)
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

	// No command specified
	fmt.Fprintf(os.Stderr, "Error: no command specified\n\n")
	printUsage()
	os.Exit(1)
}
