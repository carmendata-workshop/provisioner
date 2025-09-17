package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CronSchedule struct {
	Minute []int // Support ranges and lists
	Hour   []int
	Day    []int
	Month  []int
	DOW    []int // Day of week
}

func ParseCron(cronExpr string) (*CronSchedule, error) {
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("invalid cron expression: expected 5 fields, got %d", len(fields))
	}

	schedule := &CronSchedule{}
	var err error

	// Parse minute (0-59)
	schedule.Minute, err = parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field '%s': %w", fields[0], err)
	}

	// Parse hour (0-23)
	schedule.Hour, err = parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field '%s': %w", fields[1], err)
	}

	// Parse day (1-31)
	schedule.Day, err = parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("invalid day field '%s': %w", fields[2], err)
	}

	// Parse month (1-12)
	schedule.Month, err = parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("invalid month field '%s': %w", fields[3], err)
	}

	// Parse day of week (0-6, Sunday=0)
	schedule.DOW, err = parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("invalid day of week field '%s': %w", fields[4], err)
	}

	return schedule, nil
}

// parseField parses a CRON field supporting *, ranges (1-5), lists (1,3,5), and intervals (*/2)
func parseField(field string, min, max int) ([]int, error) {
	if field == "*" {
		// Return nil to indicate "match all"
		return nil, nil
	}

	var values []int

	// Handle comma-separated lists (1,3,5)
	parts := strings.Split(field, ",")
	for _, part := range parts {
		if strings.HasPrefix(part, "*/") {
			// Handle intervals (*/2)
			interval, err := strconv.Atoi(part[2:])
			if err != nil {
				return nil, fmt.Errorf("invalid interval: %s", part)
			}
			if interval <= 0 {
				return nil, fmt.Errorf("interval must be positive: %d", interval)
			}
			for i := min; i <= max; i += interval {
				values = append(values, i)
			}
		} else if strings.Contains(part, "-") {
			// Handle ranges (1-5)
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range format: %s", part)
			}
			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid range start: %s", rangeParts[0])
			}
			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid range end: %s", rangeParts[1])
			}
			if start < min || start > max || end < min || end > max {
				return nil, fmt.Errorf("range values out of bounds [%d-%d]: %d-%d", min, max, start, end)
			}
			if start > end {
				return nil, fmt.Errorf("invalid range: start > end: %d-%d", start, end)
			}
			for i := start; i <= end; i++ {
				values = append(values, i)
			}
		} else {
			// Handle single values
			value, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid value: %s", part)
			}
			if value < min || value > max {
				return nil, fmt.Errorf("value out of range [%d-%d]: %d", min, max, value)
			}
			values = append(values, value)
		}
	}

	return values, nil
}

func (c *CronSchedule) ShouldRun(now time.Time) bool {
	// Check minute
	if c.Minute != nil && !contains(c.Minute, now.Minute()) {
		return false
	}

	// Check hour
	if c.Hour != nil && !contains(c.Hour, now.Hour()) {
		return false
	}

	// Check day
	if c.Day != nil && !contains(c.Day, now.Day()) {
		return false
	}

	// Check month
	if c.Month != nil && !contains(c.Month, int(now.Month())) {
		return false
	}

	// Check day of week
	if c.DOW != nil && !contains(c.DOW, int(now.Weekday())) {
		return false
	}

	return true
}

// contains checks if a slice contains a specific value
func contains(slice []int, value int) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
