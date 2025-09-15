package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CronSchedule struct {
	Minute int
	Hour   int
	Day    int
	Month  int
	DOW    int // Day of week
}

func ParseCron(cronExpr string) (*CronSchedule, error) {
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("invalid cron expression: expected 5 fields, got %d", len(fields))
	}

	schedule := &CronSchedule{}

	// Parse minute
	if fields[0] == "*" {
		schedule.Minute = -1
	} else if strings.HasPrefix(fields[0], "*/") {
		// Handle */N format
		interval, err := strconv.Atoi(fields[0][2:])
		if err != nil {
			return nil, fmt.Errorf("invalid minute interval: %s", fields[0])
		}
		schedule.Minute = -interval // Use negative to indicate interval
	} else {
		minute, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("invalid minute: %s", fields[0])
		}
		if minute < 0 || minute > 59 {
			return nil, fmt.Errorf("minute out of range: %d", minute)
		}
		schedule.Minute = minute
	}

	// Parse hour
	if fields[1] == "*" {
		schedule.Hour = -1
	} else {
		hour, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("invalid hour: %s", fields[1])
		}
		if hour < 0 || hour > 23 {
			return nil, fmt.Errorf("hour out of range: %d", hour)
		}
		schedule.Hour = hour
	}

	// Parse day
	if fields[2] == "*" {
		schedule.Day = -1
	} else {
		day, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("invalid day: %s", fields[2])
		}
		if day < 1 || day > 31 {
			return nil, fmt.Errorf("day out of range: %d", day)
		}
		schedule.Day = day
	}

	// Parse month
	if fields[3] == "*" {
		schedule.Month = -1
	} else {
		month, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, fmt.Errorf("invalid month: %s", fields[3])
		}
		if month < 1 || month > 12 {
			return nil, fmt.Errorf("month out of range: %d", month)
		}
		schedule.Month = month
	}

	// Parse day of week
	if fields[4] == "*" {
		schedule.DOW = -1
	} else {
		dow, err := strconv.Atoi(fields[4])
		if err != nil {
			return nil, fmt.Errorf("invalid day of week: %s", fields[4])
		}
		if dow < 0 || dow > 6 {
			return nil, fmt.Errorf("day of week out of range: %d", dow)
		}
		schedule.DOW = dow
	}

	return schedule, nil
}

func (c *CronSchedule) ShouldRun(now time.Time) bool {
	// Check minute
	if c.Minute >= 0 {
		if now.Minute() != c.Minute {
			return false
		}
	} else if c.Minute < -1 {
		// Handle interval format (*/N)
		interval := -c.Minute
		if now.Minute()%interval != 0 {
			return false
		}
	}

	// Check hour
	if c.Hour >= 0 && now.Hour() != c.Hour {
		return false
	}

	// Check day
	if c.Day >= 0 && now.Day() != c.Day {
		return false
	}

	// Check month
	if c.Month >= 0 && int(now.Month()) != c.Month {
		return false
	}

	// Check day of week
	if c.DOW >= 0 && int(now.Weekday()) != c.DOW {
		return false
	}

	return true
}