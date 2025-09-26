package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWorkspaces(t *testing.T) {
	// Create temporary directory for test workspaces
	tempDir, err := os.MkdirTemp("", "test-workspaces-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test workspace 1 (enabled)
	workspace1Dir := filepath.Join(tempDir, "test-workspace-1")
	if err := os.MkdirAll(workspace1Dir, 0755); err != nil {
		t.Fatalf("failed to create workspace1 directory: %v", err)
	}

	config1 := Config{
		Enabled:         true,
		DeploySchedule:  "0 9 * * *",
		DestroySchedule: "0 17 * * *",
		Description:     "Test workspace 1",
	}
	config1Data, _ := json.Marshal(config1)
	if err := os.WriteFile(filepath.Join(workspace1Dir, "config.json"), config1Data, 0644); err != nil {
		t.Fatalf("failed to write config1: %v", err)
	}

	// Create main.tf for workspace1
	if err := os.WriteFile(filepath.Join(workspace1Dir, "main.tf"), []byte("# test tf"), 0644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	// Create test workspace 2 (disabled)
	workspace2Dir := filepath.Join(tempDir, "test-workspace-2")
	if err := os.MkdirAll(workspace2Dir, 0755); err != nil {
		t.Fatalf("failed to create workspace2 directory: %v", err)
	}

	config2 := Config{
		Enabled:         false,
		DeploySchedule:  "0 9 * * *",
		DestroySchedule: "0 17 * * *",
		Description:     "Test workspace 2 (disabled)",
	}
	config2Data, _ := json.Marshal(config2)
	if err := os.WriteFile(filepath.Join(workspace2Dir, "config.json"), config2Data, 0644); err != nil {
		t.Fatalf("failed to write config2: %v", err)
	}

	// Create main.tf for workspace2 so it passes validation
	if err := os.WriteFile(filepath.Join(workspace2Dir, "main.tf"), []byte("# test tf 2"), 0644); err != nil {
		t.Fatalf("failed to write main.tf for workspace2: %v", err)
	}

	// Create directory without config.json
	workspace3Dir := filepath.Join(tempDir, "test-workspace-3")
	if err := os.MkdirAll(workspace3Dir, 0755); err != nil {
		t.Fatalf("failed to create workspace3 directory: %v", err)
	}

	// Load workspaces
	workspaces, err := LoadWorkspaces(tempDir)
	if err != nil {
		t.Fatalf("failed to load workspaces: %v", err)
	}

	// Should load all workspaces with config.json (both enabled and disabled)
	if len(workspaces) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(workspaces))
	}

	// Check that both workspaces are loaded
	workspaceNames := make(map[string]bool)
	for _, workspace := range workspaces {
		workspaceNames[workspace.Name] = workspace.Config.Enabled
	}

	if !workspaceNames["test-workspace-1"] {
		t.Errorf("expected test-workspace-1 to be enabled")
	}
	if workspaceNames["test-workspace-2"] {
		t.Errorf("expected test-workspace-2 to be disabled")
	}

	// Find and test the enabled workspace
	for _, workspace := range workspaces {
		if workspace.Name == "test-workspace-1" {
			if !workspace.Config.Enabled {
				t.Errorf("expected test-workspace-1 to be enabled")
			}
			if !workspace.HasMainTF() {
				t.Errorf("expected main.tf to exist for test-workspace-1")
			}
		}
	}
}

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tempFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tempFile.Name()) }()

	config := Config{
		Enabled:         true,
		DeploySchedule:  "0 9 * * 1-5",
		DestroySchedule: "0 18 * * 1-5",
		Description:     "Test configuration",
	}

	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if _, err := tempFile.Write(configData); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	_ = tempFile.Close()

	// Load config
	loadedConfig, err := loadConfig(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify loaded config (name field no longer exists)
	if loadedConfig.Enabled != config.Enabled {
		t.Errorf("expected enabled %v, got %v", config.Enabled, loadedConfig.Enabled)
	}
	if loadedConfig.DeploySchedule != config.DeploySchedule {
		t.Errorf("expected deploy schedule '%s', got '%s'", config.DeploySchedule, loadedConfig.DeploySchedule)
	}
}

func TestWorkspaceHasMainTF(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "test-workspace-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	workspace := Workspace{
		Path: tempDir,
	}

	// Should not have main.tf initially
	if workspace.HasMainTF() {
		t.Errorf("expected HasMainTF() to be false initially")
	}

	// Create main.tf
	mainTFPath := filepath.Join(tempDir, "main.tf")
	if err := os.WriteFile(mainTFPath, []byte("# test"), 0644); err != nil {
		t.Fatalf("failed to create main.tf: %v", err)
	}

	// Should have main.tf now
	if !workspace.HasMainTF() {
		t.Errorf("expected HasMainTF() to be true after creating main.tf")
	}

	// Check path is correct
	expectedPath := filepath.Join(tempDir, "main.tf")
	if workspace.GetMainTFPath() != expectedPath {
		t.Errorf("expected main.tf path '%s', got '%s'", expectedPath, workspace.GetMainTFPath())
	}
}

func TestConfigMultipleSchedules(t *testing.T) {
	tests := []struct {
		name            string
		config          Config
		expectedDeploy  []string
		expectedDestroy []string
		expectError     bool
	}{
		{
			name: "single string schedule",
			config: Config{
				DeploySchedule:  "0 9 * * 1-5",
				DestroySchedule: "0 17 * * 1-5",
			},
			expectedDeploy:  []string{"0 9 * * 1-5"},
			expectedDestroy: []string{"0 17 * * 1-5"},
			expectError:     false,
		},
		{
			name: "multiple string schedules",
			config: Config{
				DeploySchedule:  []string{"0 7 * * 1,3,5", "0 8 * * 2,4"},
				DestroySchedule: "0 17 * * 1-5",
			},
			expectedDeploy:  []string{"0 7 * * 1,3,5", "0 8 * * 2,4"},
			expectedDestroy: []string{"0 17 * * 1-5"},
			expectError:     false,
		},
		{
			name: "mixed interface array",
			config: Config{
				DeploySchedule:  []interface{}{"0 6 * * 1-5", "0 14 * * 1-5"},
				DestroySchedule: "0 18 * * 1-5",
			},
			expectedDeploy:  []string{"0 6 * * 1-5", "0 14 * * 1-5"},
			expectedDestroy: []string{"0 18 * * 1-5"},
			expectError:     false,
		},
		{
			name: "invalid type in array",
			config: Config{
				DeploySchedule:  []interface{}{"0 9 * * 1-5", 123},
				DestroySchedule: "0 17 * * 1-5",
			},
			expectedDeploy: nil,
			expectError:    true,
		},
		{
			name: "invalid type for schedule",
			config: Config{
				DeploySchedule:  123,
				DestroySchedule: "0 17 * * 1-5",
			},
			expectedDeploy: nil,
			expectError:    true,
		},
		{
			name: "nil schedule",
			config: Config{
				DeploySchedule:  nil,
				DestroySchedule: "0 17 * * 1-5",
			},
			expectedDeploy: nil,
			expectError:    true,
		},
		{
			name: "permanent deployment (destroy_schedule: false)",
			config: Config{
				DeploySchedule:  "0 9 * * 1-5",
				DestroySchedule: false,
			},
			expectedDeploy:  []string{"0 9 * * 1-5"},
			expectedDestroy: []string{}, // Empty slice for permanent
			expectError:     false,
		},
		{
			name: "invalid boolean true for destroy schedule",
			config: Config{
				DeploySchedule:  "0 9 * * 1-5",
				DestroySchedule: true,
			},
			expectedDeploy: []string{"0 9 * * 1-5"},
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deploySchedules, deployErr := tt.config.GetDeploySchedules()
			destroySchedules, destroyErr := tt.config.GetDestroySchedules()

			if tt.expectError {
				if deployErr == nil && destroyErr == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if deployErr != nil {
				t.Errorf("unexpected deploy error: %v", deployErr)
				return
			}

			if destroyErr != nil {
				t.Errorf("unexpected destroy error: %v", destroyErr)
				return
			}

			if len(deploySchedules) != len(tt.expectedDeploy) {
				t.Errorf("expected %d deploy schedules, got %d", len(tt.expectedDeploy), len(deploySchedules))
				return
			}

			if len(destroySchedules) != len(tt.expectedDestroy) {
				t.Errorf("expected %d destroy schedules, got %d", len(tt.expectedDestroy), len(destroySchedules))
				return
			}

			for i, expected := range tt.expectedDeploy {
				if deploySchedules[i] != expected {
					t.Errorf("expected deploy schedule[%d] = '%s', got '%s'", i, expected, deploySchedules[i])
				}
			}

			for i, expected := range tt.expectedDestroy {
				if destroySchedules[i] != expected {
					t.Errorf("expected destroy schedule[%d] = '%s', got '%s'", i, expected, destroySchedules[i])
				}
			}
		})
	}
}

func TestConfigJSONSerialization(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected Config
	}{
		{
			name: "single schedule strings",
			jsonData: `{
				"enabled": true,
				"deploy_schedule": "0 9 * * 1-5",
				"destroy_schedule": "0 17 * * 1-5",
				"description": "test workspace"
			}`,
			expected: Config{
				Enabled:         true,
				DeploySchedule:  "0 9 * * 1-5",
				DestroySchedule: "0 17 * * 1-5",
				Description:     "test workspace",
			},
		},
		{
			name: "multiple deploy schedules",
			jsonData: `{
				"enabled": true,
				"deploy_schedule": ["0 7 * * 1,3,5", "0 8 * * 2,4"],
				"destroy_schedule": "0 17 * * 1-5",
				"description": "test workspace"
			}`,
			expected: Config{
				Enabled:         true,
				DeploySchedule:  []interface{}{"0 7 * * 1,3,5", "0 8 * * 2,4"},
				DestroySchedule: "0 17 * * 1-5",
				Description:     "test workspace",
			},
		},
		{
			name: "config with template",
			jsonData: `{
				"enabled": true,
				"template": "web-app-v2",
				"deploy_schedule": "0 9 * * 1-5",
				"destroy_schedule": "0 18 * * 1-5",
				"description": "Web app using template"
			}`,
			expected: Config{
				Enabled:         true,
				Template:        "web-app-v2",
				DeploySchedule:  "0 9 * * 1-5",
				DestroySchedule: "0 18 * * 1-5",
				Description:     "Web app using template",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config
			err := json.Unmarshal([]byte(tt.jsonData), &config)
			if err != nil {
				t.Fatalf("failed to unmarshal JSON: %v", err)
			}

			// Check all fields
			if config.Enabled != tt.expected.Enabled {
				t.Errorf("expected enabled %v, got %v", tt.expected.Enabled, config.Enabled)
			}
			if config.Template != tt.expected.Template {
				t.Errorf("expected template '%s', got '%s'", tt.expected.Template, config.Template)
			}

			// Test that the schedules can be processed
			deploySchedules, err := config.GetDeploySchedules()
			if err != nil {
				t.Errorf("failed to get deploy schedules: %v", err)
			}

			destroySchedules, err := config.GetDestroySchedules()
			if err != nil {
				t.Errorf("failed to get destroy schedules: %v", err)
			}

			// For the multiple schedule case
			if tt.name == "multiple deploy schedules" {
				if len(deploySchedules) != 2 {
					t.Errorf("expected 2 deploy schedules, got %d", len(deploySchedules))
				}
				if len(destroySchedules) != 1 {
					t.Errorf("expected 1 destroy schedule, got %d", len(destroySchedules))
				}
			}
		})
	}
}
