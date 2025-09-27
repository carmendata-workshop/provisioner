# CRON Scheduling Guide

The OpenTofu Workspace Scheduler uses enhanced CRON expressions for flexible scheduling.

## CRON Schedule Format

Uses standard 5-field CRON format: `minute hour day month day-of-week`

**Field Values:**
- `minute` - 0-59
- `hour` - 0-23
- `day` - 1-31
- `month` - 1-12
- `day-of-week` - 0-6 (Sunday=0)

## Supported Syntax

- `*` - Match all values
- `5` - Specific value
- `1-5` - Range of values
- `1,3,5` - List of values
- `*/5` - Every 5th value (intervals)
- `1-3,5` - Mixed ranges and values
- `1,3-5` - Mixed values and ranges
- `1-2,4-5` - Multiple ranges

## Basic Examples

- `0 9 * * 1-5` - 9:00 AM, Monday through Friday
- `*/15 * * * *` - Every 15 minutes
- `0 */2 * * *` - Every 2 hours
- `0 0 * * 0` - Midnight every Sunday

## Advanced Examples

- `0 9 * * 1,2,4,5` - 9:00 AM, Mon/Tue/Thu/Fri (excluding Wednesday)
- `0 9-17 * * 1-5` - Every hour from 9am-5pm, weekdays
- `30 8,12,17 * * 1-5` - 8:30am, 12:30pm, 5:30pm on weekdays
- `0 */3 * * 1,3,5` - Every 3 hours on Mon/Wed/Fri
- `15 9-17/2 * * 1-5` - 15 minutes past every 2nd hour 9am-5pm, weekdays

## Multiple Schedules

You can specify multiple schedules using arrays. The workspace will deploy/destroy when ANY of the schedules match:

### Single Schedule (String)
```json
{
  "deploy_schedule": "0 9 * * 1-5"
}
```

### Multiple Schedules (Array)
```json
{
  "deploy_schedule": ["0 7 * * 1,3,5", "0 8 * * 2,4"]
}
```

### Mixed Format
```json
{
  "deploy_schedule": ["0 6 * * 1-5", "0 14 * * 1-5"],
  "destroy_schedule": "0 18 * * 1-5"
}
```

## Common Patterns

### Business Hours
```json
{
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5"
}
```

### Extended Hours
```json
{
  "deploy_schedule": "0 7 * * 1-5",
  "destroy_schedule": "0 19 * * 1-5"
}
```

### Weekend Testing
```json
{
  "deploy_schedule": "0 10 * * 6,0",
  "destroy_schedule": "0 16 * * 6,0"
}
```

### Training Sessions (Specific Days)
```json
{
  "deploy_schedule": "30 8 * * 2,4",
  "destroy_schedule": "30 16 * * 2,4"
}
```

### Multiple Daily Cycles
```json
{
  "deploy_schedule": ["0 6 * * 1-5", "0 14 * * 1-5"],
  "destroy_schedule": ["0 12 * * 1-5", "0 18 * * 1-5"]
}
```

### Different Start Times by Day
```json
{
  "deploy_schedule": ["0 7 * * 1,3,5", "0 8 * * 2,4"],
  "destroy_schedule": "30 17 * * 1-5"
}
```

## Mode-Based Scheduling

For workspaces using `mode_schedules`, each mode can have its own schedule:

```json
{
  "mode_schedules": {
    "hibernation": ["0 23 * * 1-5", "0 0 * * 6,0"],
    "quiet": "0 6 * * 1-5",
    "busy": ["0 8 * * 1-5", "0 13 * * 1-5"],
    "maintenance": "0 2 * * 0"
  }
}
```

## Job Scheduling

Jobs within workspaces and standalone jobs also use the same CRON scheduling format:

### Workspace Job
```json
{
  "jobs": [
    {
      "name": "backup",
      "type": "script",
      "schedule": "0 2 * * *",
      "script": "#!/bin/bash\necho 'Creating backup...'"
    }
  ]
}
```

### Standalone Job
```json
{
  "name": "cleanup",
  "type": "script",
  "schedule": ["0 1 * * *", "0 13 * * *"],
  "script": "#!/bin/bash\necho 'Running cleanup...'"
}
```

## Validation

The scheduler validates CRON expressions at startup and will log warnings for invalid expressions. Basic validation includes:

- **Field Count**: Must have exactly 5 fields separated by spaces
- **Field Values**: Each field must be within valid ranges
- **Syntax**: Basic syntax validation for ranges, lists, and intervals

## Best Practices

1. **Avoid Overlap**: Ensure long-running operations don't overlap with next scheduled execution
2. **Off-Peak Hours**: Schedule resource-intensive operations during low usage periods
3. **Stagger Operations**: Distribute start times to avoid system load spikes
4. **Test Schedules**: Use shorter intervals during testing, then adjust to production schedules
5. **Document Complex Schedules**: Use clear descriptions for non-obvious scheduling patterns

## Timezone Considerations

The scheduler runs in the system timezone. Ensure your CRON expressions account for:

- System timezone settings
- Daylight saving time transitions
- Coordinated scheduling across different environments

## Examples by Use Case

### Development Environment
- **Deploy**: `0 9 * * 1-5` (9 AM weekdays)
- **Destroy**: `0 18 * * 1-5` (6 PM weekdays)

### Testing Environment
- **Deploy**: `0 6,14 * * *` (6 AM and 2 PM daily)
- **Destroy**: `0 12,20 * * *` (12 PM and 8 PM daily)

### Demo Environment
- **Deploy**: `0 8 * * 1-5` (8 AM weekdays only)
- **Destroy**: `0 17 * * 1-5` (5 PM weekdays only)

### Training Environment
- **Deploy**: `30 8 * * 2,4` (8:30 AM Tuesday/Thursday)
- **Destroy**: `30 16 * * 2,4` (4:30 PM Tuesday/Thursday)

### Maintenance Mode
- **Maintenance**: `0 2 * * 0` (2 AM Sunday)
- **Resume**: `0 6 * * 1` (6 AM Monday)