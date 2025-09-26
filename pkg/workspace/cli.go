package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
)

func RunAddCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("workspace add requires NAME argument")
	}

	name := args[0]
	var template, description, deploySchedule, destroySchedule string
	enabled := true

	// Parse optional flags
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--template=") {
			template = strings.TrimPrefix(arg, "--template=")
		} else if arg == "--template" && i+1 < len(args) {
			template = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--description=") {
			description = strings.TrimPrefix(arg, "--description=")
		} else if arg == "--description" && i+1 < len(args) {
			description = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--deploy-schedule=") {
			deploySchedule = strings.TrimPrefix(arg, "--deploy-schedule=")
		} else if arg == "--deploy-schedule" && i+1 < len(args) {
			deploySchedule = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--destroy-schedule=") {
			destroySchedule = strings.TrimPrefix(arg, "--destroy-schedule=")
		} else if arg == "--destroy-schedule" && i+1 < len(args) {
			destroySchedule = args[i+1]
			i++
		} else if arg == "--disabled" {
			enabled = false
		}
	}

	// Validate template exists if specified
	if template != "" {
		templatesDir := getTemplatesDir()
		templatePath := filepath.Join(templatesDir, template)
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			return fmt.Errorf("template '%s' does not exist", template)
		}
	}

	if err := CreateWorkspace(name, template, description, deploySchedule, destroySchedule, enabled); err != nil {
		return err
	}

	fmt.Printf("Workspace '%s' created successfully\n", name)
	if template != "" {
		fmt.Printf("Using template: %s\n", template)
	} else {
		fmt.Printf("Created with empty main.tf - add your OpenTofu configuration\n")
	}
	return nil
}

func RunShowCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("workspace show requires exactly one NAME argument")
	}

	name := args[0]
	workspacesDir := getDefaultWorkspacesDir()
	workspacePath := filepath.Join(workspacesDir, name)

	// Check if workspace exists
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return fmt.Errorf("workspace '%s' does not exist", name)
	}

	// Load workspace config
	configPath := filepath.Join(workspacePath, "config.json")
	config, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Create workspace object for helper methods
	workspace := Workspace{
		Name:   name,
		Config: config,
		Path:   workspacePath,
	}

	// Show basic info
	fmt.Printf("Name:        %s\n", name)
	fmt.Printf("Enabled:     %t\n", config.Enabled)
	if config.Description != "" {
		fmt.Printf("Description: %s\n", config.Description)
	}
	fmt.Printf("Path:        %s\n", workspacePath)

	// Show template info
	if config.Template != "" {
		fmt.Printf("Template:    %s\n", config.Template)
		if workspace.IsUsingTemplate() {
			fmt.Printf("Source:      Template-based\n")
		} else {
			fmt.Printf("Source:      Local main.tf (template overridden)\n")
		}
	} else {
		fmt.Printf("Source:      Local main.tf\n")
	}

	// Show schedules
	deploySchedules, err := config.GetDeploySchedules()
	if err != nil {
		fmt.Printf("Deploy Schedule: Error - %v\n", err)
	} else if len(deploySchedules) == 0 {
		fmt.Printf("Deploy Schedule: None (permanent deployment)\n")
	} else {
		fmt.Printf("Deploy Schedule: %s\n", strings.Join(deploySchedules, ", "))
	}

	destroySchedules, err := config.GetDestroySchedules()
	if err != nil {
		fmt.Printf("Destroy Schedule: Error - %v\n", err)
	} else if len(destroySchedules) == 0 {
		fmt.Printf("Destroy Schedule: None (permanent deployment)\n")
	} else {
		fmt.Printf("Destroy Schedule: %s\n", strings.Join(destroySchedules, ", "))
	}

	// Show OpenTofu file status
	mainTFPath := workspace.GetMainTFPath()
	if _, err := os.Stat(mainTFPath); err == nil {
		fmt.Printf("OpenTofu Config: %s\n", mainTFPath)
	} else {
		fmt.Printf("OpenTofu Config: Missing (%s)\n", mainTFPath)
	}

	// Show current deployment status if possible by reading state directly
	stateDir := os.Getenv("PROVISIONER_STATE_DIR")
	if stateDir == "" {
		stateDir = "state"
	}
	statePath := filepath.Join(stateDir, "scheduler.json")

	if stateData, err := os.ReadFile(statePath); err == nil {
		var state struct {
			Workspaces map[string]struct {
				Status           string     `json:"status"`
				LastDeployed     *time.Time `json:"last_deployed"`
				LastDestroyed    *time.Time `json:"last_destroyed"`
				LastDeployError  string     `json:"last_deploy_error"`
				LastDestroyError string     `json:"last_destroy_error"`
			} `json:"workspaces"`
		}

		if json.Unmarshal(stateData, &state) == nil {
			if workspaceState, exists := state.Workspaces[name]; exists {
				fmt.Printf("\nCurrent Status:\n")
				fmt.Printf("  State:       %s\n", workspaceState.Status)
				if workspaceState.LastDeployed != nil {
					fmt.Printf("  Last Deploy: %s\n", workspaceState.LastDeployed.Format(time.RFC3339))
				}
				if workspaceState.LastDestroyed != nil {
					fmt.Printf("  Last Destroy: %s\n", workspaceState.LastDestroyed.Format(time.RFC3339))
				}
				if workspaceState.LastDeployError != "" {
					fmt.Printf("  Deploy Error: %s\n", workspaceState.LastDeployError)
				}
				if workspaceState.LastDestroyError != "" {
					fmt.Printf("  Destroy Error: %s\n", workspaceState.LastDestroyError)
				}
			}
		}
	}

	return nil
}

func RunUpdateCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("workspace update requires NAME argument")
	}

	name := args[0]
	var template, description, deploySchedule, destroySchedule string
	var enabled *bool

	// Parse optional flags
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--template=") {
			template = strings.TrimPrefix(arg, "--template=")
		} else if arg == "--template" && i+1 < len(args) {
			template = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--description=") {
			description = strings.TrimPrefix(arg, "--description=")
		} else if arg == "--description" && i+1 < len(args) {
			description = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--deploy-schedule=") {
			deploySchedule = strings.TrimPrefix(arg, "--deploy-schedule=")
		} else if arg == "--deploy-schedule" && i+1 < len(args) {
			deploySchedule = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--destroy-schedule=") {
			destroySchedule = strings.TrimPrefix(arg, "--destroy-schedule=")
		} else if arg == "--destroy-schedule" && i+1 < len(args) {
			destroySchedule = args[i+1]
			i++
		} else if arg == "--enable" {
			val := true
			enabled = &val
		} else if arg == "--disable" {
			val := false
			enabled = &val
		}
	}

	// Validate template exists if specified
	if template != "" {
		templatesDir := getTemplatesDir()
		templatePath := filepath.Join(templatesDir, template)
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			return fmt.Errorf("template '%s' does not exist", template)
		}
	}

	if err := UpdateWorkspace(name, template, description, deploySchedule, destroySchedule, enabled); err != nil {
		return err
	}

	fmt.Printf("Workspace '%s' updated successfully\n", name)
	return nil
}

func RunRemoveCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workspace remove requires NAME argument")
	}

	name := args[0]
	force := false

	// Check for --force flag
	for _, arg := range args[1:] {
		if arg == "--force" {
			force = true
		}
	}

	// Check if workspace is currently deployed (unless forced)
	if !force {
		stateDir := os.Getenv("PROVISIONER_STATE_DIR")
		if stateDir == "" {
			stateDir = "state"
		}
		statePath := filepath.Join(stateDir, "scheduler.json")

		if stateData, err := os.ReadFile(statePath); err == nil {
			var state struct {
				Workspaces map[string]struct {
					Status string `json:"status"`
				} `json:"workspaces"`
			}

			if json.Unmarshal(stateData, &state) == nil {
				if workspaceState, exists := state.Workspaces[name]; exists && workspaceState.Status == "deployed" {
					return fmt.Errorf("workspace '%s' is currently deployed. Use --force to remove anyway, or destroy it first", name)
				}
			}
		}

		// Confirm removal
		fmt.Printf("Are you sure you want to remove workspace '%s'? (y/N): ", name)
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			fmt.Println("Cancelled")
			return nil
		}
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	if err := RemoveWorkspace(name); err != nil {
		return err
	}

	fmt.Printf("Workspace '%s' removed successfully\n", name)
	return nil
}

func RunValidateCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workspace validate requires NAME or --all argument")
	}

	if args[0] == "--all" {
		workspacesDir := getDefaultWorkspacesDir()
		workspaces, err := LoadWorkspaces(workspacesDir)
		if err != nil {
			return err
		}

		hasErrors := false
		for _, workspace := range workspaces {
			if err := ValidateWorkspace(workspace.Name); err != nil {
				fmt.Printf("✗ %s: %v\n", workspace.Name, err)
				hasErrors = true
			} else {
				fmt.Printf("✓ %s: valid\n", workspace.Name)
			}
		}

		if hasErrors {
			return fmt.Errorf("some workspaces have validation errors")
		}
		return nil
	}

	name := args[0]
	if err := ValidateWorkspace(name); err != nil {
		return fmt.Errorf("workspace '%s' validation failed: %v", name, err)
	}

	fmt.Printf("Workspace '%s' is valid\n", name)
	return nil
}

func RunListCommand(args []string) error {
	detailed := false

	// Parse flags
	for _, arg := range args {
		if arg == "--detailed" {
			detailed = true
		}
	}

	workspacesDir := getDefaultWorkspacesDir()
	workspaces, err := LoadWorkspaces(workspacesDir)
	if err != nil {
		return err
	}

	if len(workspaces) == 0 {
		fmt.Println("No workspaces found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if detailed {
		if _, err := fmt.Fprintln(w, "NAME\tENABLED\tSOURCE\tTEMPLATE\tDEPLOY SCHEDULE\tDESTROY SCHEDULE\tDESCRIPTION"); err != nil {
			return err
		}
		for _, workspace := range workspaces {
			source := "Local"
			if workspace.IsUsingTemplate() {
				source = "Template"
			}

			deploySchedules, _ := workspace.Config.GetDeploySchedules()
			destroySchedules, _ := workspace.Config.GetDestroySchedules()

			if _, err := fmt.Fprintf(w, "%s\t%t\t%s\t%s\t%s\t%s\t%s\n",
				workspace.Name,
				workspace.Config.Enabled,
				source,
				workspace.Config.Template,
				strings.Join(deploySchedules, ","),
				strings.Join(destroySchedules, ","),
				workspace.Config.Description,
			); err != nil {
				return err
			}
		}
	} else {
		if _, err := fmt.Fprintln(w, "NAME\tENABLED\tSOURCE\tDESCRIPTION"); err != nil {
			return err
		}
		for _, workspace := range workspaces {
			source := "Local"
			if workspace.IsUsingTemplate() {
				source = fmt.Sprintf("Template(%s)", workspace.Config.Template)
			}

			if _, err := fmt.Fprintf(w, "%s\t%t\t%s\t%s\n",
				workspace.Name,
				workspace.Config.Enabled,
				source,
				workspace.Config.Description,
			); err != nil {
				return err
			}
		}
	}

	return w.Flush()
}
