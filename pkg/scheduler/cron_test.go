package scheduler

import (
	"testing"
	"time"
)

func TestParseCron(t *testing.T) {
	tests := []struct {
		name        string
		cronExpr    string
		expectError bool
	}{
		{"valid basic", "0 9 * * 1", false},
		{"valid interval", "*/5 * * * *", false},
		{"valid complex", "15 14 1 * *", false},
		{"invalid fields", "0 9 * *", true},
		{"invalid minute", "60 9 * * *", true},
		{"invalid hour", "0 25 * * *", true},
		{"invalid day", "0 9 32 * *", true},
		{"invalid month", "0 9 * 13 *", true},
		{"invalid dow", "0 9 * * 7", true},
		{"invalid interval", "*/abc * * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCron(tt.cronExpr)
			if tt.expectError && err == nil {
				t.Errorf("expected error for %s but got none", tt.cronExpr)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for %s: %v", tt.cronExpr, err)
			}
		})
	}
}

func TestCronShouldRun(t *testing.T) {
	tests := []struct {
		name     string
		cronExpr string
		testTime time.Time
		expected bool
	}{
		{
			name:     "exact match",
			cronExpr: "30 14 17 6 1",
			testTime: time.Date(2024, 6, 17, 14, 30, 0, 0, time.UTC), // Monday (day=17, month=6, dow=1)
			expected: true,
		},
		{
			name:     "wrong minute",
			cronExpr: "30 14 * * *",
			testTime: time.Date(2024, 6, 15, 14, 29, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "interval match",
			cronExpr: "*/5 * * * *",
			testTime: time.Date(2024, 6, 15, 14, 25, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "interval no match",
			cronExpr: "*/5 * * * *",
			testTime: time.Date(2024, 6, 15, 14, 23, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "wildcard hour",
			cronExpr: "0 * * * *",
			testTime: time.Date(2024, 6, 15, 14, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "all wildcards",
			cronExpr: "* * * * *",
			testTime: time.Date(2024, 6, 15, 14, 23, 0, 0, time.UTC),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule, err := ParseCron(tt.cronExpr)
			if err != nil {
				t.Fatalf("failed to parse cron %s: %v", tt.cronExpr, err)
			}

			result := schedule.ShouldRun(tt.testTime)
			if result != tt.expected {
				t.Errorf("expected %v for %s at %s, got %v",
					tt.expected, tt.cronExpr, tt.testTime.Format("2006-01-02 15:04:05"), result)
			}
		})
	}
}
