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
		{"valid range", "0 9 * * 1-5", false},
		{"valid list", "0 9,17 * * 1,3,5", false},
		{"mixed individual values", "0 9 * * 1,2,4,5", false},
		{"range plus individual", "0 9 * * 1-3,5", false},
		{"individual plus range", "0 9 * * 1,3-5", false},
		{"multiple ranges", "0 9 * * 1-2,4-5", false},
		{"complex mixed", "0 9 * * 1,3,5-6", false},
		{"invalid range format", "0 9 * * 1-5-7", true},
		{"invalid range order", "0 9 * * 5-1", true},
		{"range out of bounds", "0 9 * * 1-8", true},
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

func TestCronRanges(t *testing.T) {
	tests := []struct {
		name     string
		cronExpr string
		testTime time.Time
		expected bool
	}{
		{
			name:     "weekday range match Monday",
			cronExpr: "0 9 * * 1-5",                                // Monday-Friday at 9am
			testTime: time.Date(2024, 6, 17, 9, 0, 0, 0, time.UTC), // Monday
			expected: true,
		},
		{
			name:     "weekday range match Friday",
			cronExpr: "0 9 * * 1-5",                                // Monday-Friday at 9am
			testTime: time.Date(2024, 6, 21, 9, 0, 0, 0, time.UTC), // Friday
			expected: true,
		},
		{
			name:     "weekday range no match Saturday",
			cronExpr: "0 9 * * 1-5",                                // Monday-Friday at 9am
			testTime: time.Date(2024, 6, 22, 9, 0, 0, 0, time.UTC), // Saturday
			expected: false,
		},
		{
			name:     "weekday range no match Sunday",
			cronExpr: "0 9 * * 1-5",                                // Monday-Friday at 9am
			testTime: time.Date(2024, 6, 23, 9, 0, 0, 0, time.UTC), // Sunday
			expected: false,
		},
		{
			name:     "hour range match",
			cronExpr: "0 9-17 * * *",                                // Every hour from 9am-5pm
			testTime: time.Date(2024, 6, 17, 14, 0, 0, 0, time.UTC), // 2pm
			expected: true,
		},
		{
			name:     "hour range no match",
			cronExpr: "0 9-17 * * *",                               // Every hour from 9am-5pm
			testTime: time.Date(2024, 6, 17, 8, 0, 0, 0, time.UTC), // 8am
			expected: false,
		},
		{
			name:     "list match first",
			cronExpr: "0 9 * * 1,3,5",                              // Monday, Wednesday, Friday
			testTime: time.Date(2024, 6, 17, 9, 0, 0, 0, time.UTC), // Monday
			expected: true,
		},
		{
			name:     "list match middle",
			cronExpr: "0 9 * * 1,3,5",                              // Monday, Wednesday, Friday
			testTime: time.Date(2024, 6, 19, 9, 0, 0, 0, time.UTC), // Wednesday
			expected: true,
		},
		{
			name:     "list no match",
			cronExpr: "0 9 * * 1,3,5",                              // Monday, Wednesday, Friday
			testTime: time.Date(2024, 6, 18, 9, 0, 0, 0, time.UTC), // Tuesday
			expected: false,
		},
		{
			name:     "combined range and time",
			cronExpr: "30 17 * * 1-5",                                // 5:30pm weekdays
			testTime: time.Date(2024, 6, 19, 17, 30, 0, 0, time.UTC), // Wednesday 5:30pm
			expected: true,
		},
		{
			name:     "combined range wrong time",
			cronExpr: "30 17 * * 1-5",                               // 5:30pm weekdays
			testTime: time.Date(2024, 6, 19, 17, 0, 0, 0, time.UTC), // Wednesday 5:00pm
			expected: false,
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
				t.Errorf("expected %v for %s at %s (dow=%d), got %v",
					tt.expected, tt.cronExpr, tt.testTime.Format("2006-01-02 15:04:05 Mon"), int(tt.testTime.Weekday()), result)
			}
		})
	}
}

func TestCronMixedRangesAndValues(t *testing.T) {
	tests := []struct {
		name     string
		cronExpr string
		expected []int
	}{
		{
			name:     "individual values excluding Wednesday",
			cronExpr: "0 9 * * 1,2,4,5",
			expected: []int{1, 2, 4, 5}, // Mon,Tue,Thu,Fri
		},
		{
			name:     "range plus individual",
			cronExpr: "0 9 * * 1-3,5",
			expected: []int{1, 2, 3, 5}, // Mon-Wed,Fri
		},
		{
			name:     "individual plus range",
			cronExpr: "0 9 * * 1,3-5",
			expected: []int{1, 3, 4, 5}, // Mon,Wed-Fri
		},
		{
			name:     "multiple ranges",
			cronExpr: "0 9 * * 1-2,4-5",
			expected: []int{1, 2, 4, 5}, // Mon-Tue,Thu-Fri
		},
		{
			name:     "complex mixed",
			cronExpr: "0 9 * * 1,3,5-6",
			expected: []int{1, 3, 5, 6}, // Mon,Wed,Fri-Sat
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule, err := ParseCron(tt.cronExpr)
			if err != nil {
				t.Fatalf("failed to parse cron %s: %v", tt.cronExpr, err)
			}

			if len(schedule.DOW) != len(tt.expected) {
				t.Errorf("expected %d DOW values, got %d", len(tt.expected), len(schedule.DOW))
			}

			for i, expected := range tt.expected {
				if i >= len(schedule.DOW) || schedule.DOW[i] != expected {
					t.Errorf("expected DOW values %v, got %v", tt.expected, schedule.DOW)
					break
				}
			}
		})
	}
}
