package environment

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
		return fmt.Errorf("environment add requires NAME argument")
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

	if err := CreateEnvironment(name, template, description, deploySchedule, destroySchedule, enabled); err != nil {
		return err
	}

	fmt.Printf("Environment '%s' created successfully\n", name)
	if template != "" {
		fmt.Printf("Using template: %s\n", template)
	} else {
		fmt.Printf("Created with empty main.tf - add your OpenTofu configuration\n")
	}
	return nil
}

func RunShowCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("environment show requires exactly one NAME argument")
	}

	name := args[0]
	environmentsDir := getDefaultEnvironmentsDir()
	envPath := filepath.Join(environmentsDir, name)

	// Check if environment exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' does not exist", name)
	}

	// Load environment config
	configPath := filepath.Join(envPath, "config.json")
	config, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load environment config: %w", err)
	}

	// Create environment object for helper methods
	env := Environment{
		Name:   name,
		Config: config,
		Path:   envPath,
	}

	// Show basic info
	fmt.Printf("Name:        %s\n", name)
	fmt.Printf("Enabled:     %t\n", config.Enabled)
	if config.Description != "" {
		fmt.Printf("Description: %s\n", config.Description)
	}
	fmt.Printf("Path:        %s\n", envPath)

	// Show template info
	if config.Template != "" {
		fmt.Printf("Template:    %s\n", config.Template)
		if env.IsUsingTemplate() {
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
	mainTFPath := env.GetMainTFPath()
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
			Environments map[string]struct {
				Status            string     `json:"status"`
				LastDeployed      *time.Time `json:"last_deployed"`
				LastDestroyed     *time.Time `json:"last_destroyed"`
				LastDeployError   string     `json:"last_deploy_error"`
				LastDestroyError  string     `json:"last_destroy_error"`
			} `json:"environments"`
		}

		if json.Unmarshal(stateData, &state) == nil {
			if envState, exists := state.Environments[name]; exists {
				fmt.Printf("\nCurrent Status:\n")
				fmt.Printf("  State:       %s\n", envState.Status)
				if envState.LastDeployed != nil {
					fmt.Printf("  Last Deploy: %s\n", envState.LastDeployed.Format(time.RFC3339))
				}
				if envState.LastDestroyed != nil {
					fmt.Printf("  Last Destroy: %s\n", envState.LastDestroyed.Format(time.RFC3339))
				}
				if envState.LastDeployError != "" {
					fmt.Printf("  Deploy Error: %s\n", envState.LastDeployError)
				}
				if envState.LastDestroyError != "" {
					fmt.Printf("  Destroy Error: %s\n", envState.LastDestroyError)
				}
			}
		}
	}

	return nil
}

func RunUpdateCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("environment update requires NAME argument")
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

	if err := UpdateEnvironment(name, template, description, deploySchedule, destroySchedule, enabled); err != nil {
		return err
	}

	fmt.Printf("Environment '%s' updated successfully\n", name)
	return nil
}

func RunRemoveCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment remove requires NAME argument")
	}

	name := args[0]
	force := false

	// Check for --force flag
	for _, arg := range args[1:] {
		if arg == "--force" {
			force = true
		}
	}

	// Check if environment is currently deployed (unless forced)
	if !force {
		stateDir := os.Getenv("PROVISIONER_STATE_DIR")
		if stateDir == "" {
			stateDir = "state"
		}
		statePath := filepath.Join(stateDir, "scheduler.json")

		if stateData, err := os.ReadFile(statePath); err == nil {
			var state struct {
				Environments map[string]struct {
					Status string `json:"status"`
				} `json:"environments"`
			}

			if json.Unmarshal(stateData, &state) == nil {
				if envState, exists := state.Environments[name]; exists && envState.Status == "deployed" {
					return fmt.Errorf("environment '%s' is currently deployed. Use --force to remove anyway, or destroy it first", name)
				}
			}
		}

		// Confirm removal
		fmt.Printf("Are you sure you want to remove environment '%s'? (y/N): ", name)
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

	if err := RemoveEnvironment(name); err != nil {
		return err
	}

	fmt.Printf("Environment '%s' removed successfully\n", name)
	return nil
}

func RunValidateCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment validate requires NAME or --all argument")
	}

	if args[0] == "--all" {
		environmentsDir := getDefaultEnvironmentsDir()
		environments, err := LoadEnvironments(environmentsDir)
		if err != nil {
			return err
		}

		hasErrors := false
		for _, env := range environments {
			if err := ValidateEnvironment(env.Name); err != nil {
				fmt.Printf("✗ %s: %v\n", env.Name, err)
				hasErrors = true
			} else {
				fmt.Printf("✓ %s: valid\n", env.Name)
			}
		}

		if hasErrors {
			return fmt.Errorf("some environments have validation errors")
		}
		return nil
	}

	name := args[0]
	if err := ValidateEnvironment(name); err != nil {
		return fmt.Errorf("environment '%s' validation failed: %v", name, err)
	}

	fmt.Printf("Environment '%s' is valid\n", name)
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

	environmentsDir := getDefaultEnvironmentsDir()
	environments, err := LoadEnvironments(environmentsDir)
	if err != nil {
		return err
	}

	if len(environments) == 0 {
		fmt.Println("No environments found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if detailed {
		if _, err := fmt.Fprintln(w, "NAME\tENABLED\tSOURCE\tTEMPLATE\tDEPLOY SCHEDULE\tDESTROY SCHEDULE\tDESCRIPTION"); err != nil {
			return err
		}
		for _, env := range environments {
			source := "Local"
			if env.IsUsingTemplate() {
				source = "Template"
			}

			deploySchedules, _ := env.Config.GetDeploySchedules()
			destroySchedules, _ := env.Config.GetDestroySchedules()

			if _, err := fmt.Fprintf(w, "%s\t%t\t%s\t%s\t%s\t%s\t%s\n",
				env.Name,
				env.Config.Enabled,
				source,
				env.Config.Template,
				strings.Join(deploySchedules, ","),
				strings.Join(destroySchedules, ","),
				env.Config.Description,
			); err != nil {
				return err
			}
		}
	} else {
		if _, err := fmt.Fprintln(w, "NAME\tENABLED\tSOURCE\tDESCRIPTION"); err != nil {
			return err
		}
		for _, env := range environments {
			source := "Local"
			if env.IsUsingTemplate() {
				source = fmt.Sprintf("Template(%s)", env.Config.Template)
			}

			if _, err := fmt.Fprintf(w, "%s\t%t\t%s\t%s\n",
				env.Name,
				env.Config.Enabled,
				source,
				env.Config.Description,
			); err != nil {
				return err
			}
		}
	}

	return w.Flush()
}