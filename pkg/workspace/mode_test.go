package workspace

import (
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid mode_schedules config",
			config: Config{
				Template: "web-app",
				ModeSchedules: map[string]interface{}{
					"hibernation": "0 23 * * 1-5",
					"busy":        []string{"0 8 * * 1-5", "0 12 * * 1-5"},
				},
			},
			expectError: false,
		},
		{
			name: "valid deploy_schedule config",
			config: Config{
				DeploySchedule: "0 9 * * 1-5",
			},
			expectError: false,
		},
		{
			name: "invalid - both mode_schedules and deploy_schedule",
			config: Config{
				Template: "web-app",
				ModeSchedules: map[string]interface{}{
					"busy": "0 8 * * 1-5",
				},
				DeploySchedule: "0 9 * * 1-5",
			},
			expectError: true,
			errorMsg:    "cannot specify both 'mode_schedules' and 'deploy_schedule'",
		},
		{
			name: "invalid - neither mode_schedules nor deploy_schedule",
			config: Config{
				Template: "web-app",
			},
			expectError: true,
			errorMsg:    "must specify either 'mode_schedules' or 'deploy_schedule'",
		},
		{
			name: "invalid - mode_schedules without template",
			config: Config{
				ModeSchedules: map[string]interface{}{
					"busy": "0 8 * * 1-5",
				},
			},
			expectError: true,
			errorMsg:    "'mode_schedules' requires 'template' field",
		},
		{
			name: "invalid - bad schedule in mode_schedules",
			config: Config{
				Template: "web-app",
				ModeSchedules: map[string]interface{}{
					"busy":    "0 8 * * 1-5",
					"invalid": 123, // Invalid type
				},
			},
			expectError: true,
			errorMsg:    "invalid schedule for mode 'invalid'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetModeSchedules(t *testing.T) {
	tests := []struct {
		name            string
		config          Config
		expectedModes   map[string][]string
		expectError     bool
	}{
		{
			name: "single string schedules",
			config: Config{
				ModeSchedules: map[string]interface{}{
					"hibernation": "0 23 * * 1-5",
					"busy":        "0 8 * * 1-5",
				},
			},
			expectedModes: map[string][]string{
				"hibernation": {"0 23 * * 1-5"},
				"busy":        {"0 8 * * 1-5"},
			},
			expectError: false,
		},
		{
			name: "array schedules",
			config: Config{
				ModeSchedules: map[string]interface{}{
					"busy": []string{"0 8 * * 1-5", "0 12 * * 1-5"},
					"quiet": "0 6 * * 1-5",
				},
			},
			expectedModes: map[string][]string{
				"busy":  {"0 8 * * 1-5", "0 12 * * 1-5"},
				"quiet": {"0 6 * * 1-5"},
			},
			expectError: false,
		},
		{
			name: "mixed interface array",
			config: Config{
				ModeSchedules: map[string]interface{}{
					"busy": []interface{}{"0 8 * * 1-5", "0 12 * * 1-5"},
				},
			},
			expectedModes: map[string][]string{
				"busy": {"0 8 * * 1-5", "0 12 * * 1-5"},
			},
			expectError: false,
		},
		{
			name: "nil mode schedules",
			config: Config{
				ModeSchedules: nil,
			},
			expectedModes: nil,
			expectError:   false,
		},
		{
			name: "invalid schedule type",
			config: Config{
				ModeSchedules: map[string]interface{}{
					"busy":    "0 8 * * 1-5",
					"invalid": 123,
				},
			},
			expectedModes: nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modes, err := tt.config.GetModeSchedules()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.expectedModes == nil {
				if modes != nil {
					t.Errorf("expected nil modes, got %v", modes)
				}
				return
			}

			if len(modes) != len(tt.expectedModes) {
				t.Errorf("expected %d modes, got %d", len(tt.expectedModes), len(modes))
				return
			}

			for mode, expectedSchedules := range tt.expectedModes {
				actualSchedules, exists := modes[mode]
				if !exists {
					t.Errorf("expected mode '%s' not found", mode)
					continue
				}

				if len(actualSchedules) != len(expectedSchedules) {
					t.Errorf("mode '%s': expected %d schedules, got %d", mode, len(expectedSchedules), len(actualSchedules))
					continue
				}

				for i, expected := range expectedSchedules {
					if actualSchedules[i] != expected {
						t.Errorf("mode '%s': expected schedule[%d] = '%s', got '%s'", mode, i, expected, actualSchedules[i])
					}
				}
			}
		})
	}
}