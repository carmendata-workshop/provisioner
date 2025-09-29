package workspace

import (
	"testing"
)

func TestValidateCustomDeployConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *CustomDeployConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "Nil config is valid",
			config:      nil,
			shouldError: false,
		},
		{
			name: "All commands specified",
			config: &CustomDeployConfig{
				InitCommand:  "make init",
				PlanCommand:  "make plan",
				ApplyCommand: "make apply",
			},
			shouldError: false,
		},
		{
			name: "Only apply command specified",
			config: &CustomDeployConfig{
				ApplyCommand: "make deploy",
			},
			shouldError: false,
		},
		{
			name: "Only init command specified",
			config: &CustomDeployConfig{
				InitCommand: "make init",
			},
			shouldError: false,
		},
		{
			name: "Only plan command specified",
			config: &CustomDeployConfig{
				PlanCommand: "make plan",
			},
			shouldError: false,
		},
		{
			name:        "No commands specified",
			config:      &CustomDeployConfig{},
			shouldError: true,
			errorMsg:    "at least one custom command must be specified (init_command, plan_command, or apply_command)",
		},
		{
			name: "Empty init command",
			config: &CustomDeployConfig{
				InitCommand: "   ",
			},
			shouldError: true,
			errorMsg:    "init_command cannot be empty or whitespace-only",
		},
		{
			name: "Empty plan command",
			config: &CustomDeployConfig{
				PlanCommand: "   ",
			},
			shouldError: true,
			errorMsg:    "plan_command cannot be empty or whitespace-only",
		},
		{
			name: "Empty apply command",
			config: &CustomDeployConfig{
				ApplyCommand: "\t\n",
			},
			shouldError: true,
			errorMsg:    "apply_command cannot be empty or whitespace-only",
		},
		{
			name: "Mixed empty and valid commands",
			config: &CustomDeployConfig{
				InitCommand:  "make init",
				ApplyCommand: "   ",
			},
			shouldError: true,
			errorMsg:    "apply_command cannot be empty or whitespace-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCustomDeployConfig(tt.config)
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s' but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateCustomDestroyConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *CustomDestroyConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "Nil config is valid",
			config:      nil,
			shouldError: false,
		},
		{
			name: "Both commands specified",
			config: &CustomDestroyConfig{
				InitCommand:    "make init",
				DestroyCommand: "make destroy",
			},
			shouldError: false,
		},
		{
			name: "Only destroy command specified",
			config: &CustomDestroyConfig{
				DestroyCommand: "make destroy",
			},
			shouldError: false,
		},
		{
			name: "Only init command specified",
			config: &CustomDestroyConfig{
				InitCommand: "make init",
			},
			shouldError: false,
		},
		{
			name:        "No commands specified",
			config:      &CustomDestroyConfig{},
			shouldError: true,
			errorMsg:    "at least one custom command must be specified (init_command or destroy_command)",
		},
		{
			name: "Empty init command",
			config: &CustomDestroyConfig{
				InitCommand: "   ",
			},
			shouldError: true,
			errorMsg:    "init_command cannot be empty or whitespace-only",
		},
		{
			name: "Empty destroy command",
			config: &CustomDestroyConfig{
				DestroyCommand: "\t\n",
			},
			shouldError: true,
			errorMsg:    "destroy_command cannot be empty or whitespace-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCustomDestroyConfig(tt.config)
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s' but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestConfigValidateWithCustomCommands(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		shouldError bool
		errorMsg    string
	}{
		{
			name: "Valid config with custom deploy commands",
			config: Config{
				Enabled:        true,
				DeploySchedule: "0 9 * * *",
				CustomDeploy: &CustomDeployConfig{
					InitCommand:  "make init",
					ApplyCommand: "make deploy",
				},
			},
			shouldError: false,
		},
		{
			name: "Valid config with custom destroy commands",
			config: Config{
				Enabled:         true,
				DeploySchedule:  "0 9 * * *",
				DestroySchedule: "0 18 * * *",
				CustomDestroy: &CustomDestroyConfig{
					DestroyCommand: "make destroy",
				},
			},
			shouldError: false,
		},
		{
			name: "Valid config with both custom deploy and destroy",
			config: Config{
				Enabled:         true,
				DeploySchedule:  "0 9 * * *",
				DestroySchedule: "0 18 * * *",
				CustomDeploy: &CustomDeployConfig{
					ApplyCommand: "make deploy",
				},
				CustomDestroy: &CustomDestroyConfig{
					DestroyCommand: "make destroy",
				},
			},
			shouldError: false,
		},
		{
			name: "Invalid custom deploy config",
			config: Config{
				Enabled:        true,
				DeploySchedule: "0 9 * * *",
				CustomDeploy: &CustomDeployConfig{
					InitCommand: "   ",
				},
			},
			shouldError: true,
			errorMsg:    "custom_deploy validation failed: init_command cannot be empty or whitespace-only",
		},
		{
			name: "Invalid custom destroy config",
			config: Config{
				Enabled:         true,
				DeploySchedule:  "0 9 * * *",
				DestroySchedule: "0 18 * * *",
				CustomDestroy:   &CustomDestroyConfig{},
			},
			shouldError: true,
			errorMsg:    "custom_destroy validation failed: at least one custom command must be specified (init_command or destroy_command)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s' but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}