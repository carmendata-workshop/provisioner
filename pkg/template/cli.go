package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
)

func getDefaultTemplatesDir() string {
	// First check for explicit state directory override
	if stateDir := os.Getenv("PROVISIONER_STATE_DIR"); stateDir != "" {
		return filepath.Join(stateDir, "templates")
	}

	// Auto-detect system installation
	if _, err := os.Stat("/var/lib/provisioner"); err == nil {
		return "/var/lib/provisioner/templates"
	}

	// Default for development
	return "templates"
}

func RunAddCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("template add requires NAME and URL arguments")
	}

	name := args[0]
	sourceURL := args[1]

	var sourcePath, sourceRef, description string

	// Parse optional flags
	for i := 2; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--path=") {
			sourcePath = strings.TrimPrefix(arg, "--path=")
		} else if arg == "--path" && i+1 < len(args) {
			sourcePath = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--ref=") {
			sourceRef = strings.TrimPrefix(arg, "--ref=")
		} else if arg == "--ref" && i+1 < len(args) {
			sourceRef = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--description=") {
			description = strings.TrimPrefix(arg, "--description=")
		} else if arg == "--description" && i+1 < len(args) {
			description = args[i+1]
			i++
		}
	}

	manager := NewManager(getDefaultTemplatesDir())

	if err := manager.AddTemplate(name, sourceURL, sourcePath, sourceRef, description); err != nil {
		return err
	}

	fmt.Printf("Template '%s' added successfully\n", name)
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

	manager := NewManager(getDefaultTemplatesDir())
	templates, err := manager.ListTemplates()
	if err != nil {
		return err
	}

	if len(templates) == 0 {
		fmt.Println("No templates found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if detailed {
		if _, err := fmt.Fprintln(w, "NAME\tSOURCE\tPATH\tREF\tCREATED\tUPDATED\tDESCRIPTION"); err != nil {
			return err
		}
		for _, template := range templates {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				template.Name,
				template.SourceURL,
				template.SourcePath,
				template.SourceRef,
				template.CreatedAt.Format("2006-01-02"),
				template.UpdatedAt.Format("2006-01-02"),
				template.Description,
			); err != nil {
				return err
			}
		}
	} else {
		if _, err := fmt.Fprintln(w, "NAME\tSOURCE\tREF\tDESCRIPTION"); err != nil {
			return err
		}
		for _, template := range templates {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				template.Name,
				template.SourceURL,
				template.SourceRef,
				template.Description,
			); err != nil {
				return err
			}
		}
	}

	return w.Flush()
}

func RunShowCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("template show requires exactly one NAME argument")
	}

	name := args[0]
	manager := NewManager(getDefaultTemplatesDir())

	template, err := manager.GetTemplate(name)
	if err != nil {
		return err
	}

	fmt.Printf("Name:        %s\n", template.Name)
	fmt.Printf("Source URL:  %s\n", template.SourceURL)
	if template.SourcePath != "" {
		fmt.Printf("Source Path: %s\n", template.SourcePath)
	}
	fmt.Printf("Source Ref:  %s\n", template.SourceRef)
	fmt.Printf("Created:     %s\n", template.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:     %s\n", template.UpdatedAt.Format(time.RFC3339))
	if template.Description != "" {
		fmt.Printf("Description: %s\n", template.Description)
	}
	if template.Version != "" {
		fmt.Printf("Version:     %s\n", template.Version)
	}

	// Show template path
	templatePath := manager.GetTemplatePath(name)
	fmt.Printf("Path:        %s\n", templatePath)

	return nil
}

func RunUpdateCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("template update requires NAME or --all argument")
	}

	manager := NewManager(getDefaultTemplatesDir())

	if args[0] == "--all" {
		templates, err := manager.ListTemplates()
		if err != nil {
			return err
		}

		for _, template := range templates {
			fmt.Printf("Updating template '%s'...\n", template.Name)
			if err := manager.UpdateTemplate(template.Name); err != nil {
				fmt.Printf("  Error: %v\n", err)
			} else {
				fmt.Printf("  Updated successfully\n")
			}
		}
		return nil
	}

	name := args[0]
	if err := manager.UpdateTemplate(name); err != nil {
		return err
	}

	fmt.Printf("Template '%s' updated successfully\n", name)
	return nil
}

func RunRemoveCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("template remove requires NAME argument")
	}

	name := args[0]
	force := false

	// Check for --force flag
	for _, arg := range args[1:] {
		if arg == "--force" {
			force = true
		}
	}

	manager := NewManager(getDefaultTemplatesDir())

	// Confirm removal if not forced
	if !force {
		fmt.Printf("Are you sure you want to remove template '%s'? (y/N): ", name)
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

	if err := manager.RemoveTemplate(name, force); err != nil {
		return err
	}

	fmt.Printf("Template '%s' removed successfully\n", name)
	return nil
}

func RunValidateCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("template validate requires NAME or --all argument")
	}

	manager := NewManager(getDefaultTemplatesDir())

	if args[0] == "--all" {
		templates, err := manager.ListTemplates()
		if err != nil {
			return err
		}

		hasErrors := false
		for _, template := range templates {
			if err := manager.ValidateTemplate(template.Name); err != nil {
				fmt.Printf("✗ %s: %v\n", template.Name, err)
				hasErrors = true
			} else {
				fmt.Printf("✓ %s: valid\n", template.Name)
			}
		}

		if hasErrors {
			return fmt.Errorf("some templates have validation errors")
		}
		return nil
	}

	name := args[0]
	if err := manager.ValidateTemplate(name); err != nil {
		return fmt.Errorf("template '%s' validation failed: %v", name, err)
	}

	fmt.Printf("Template '%s' is valid\n", name)
	return nil
}
