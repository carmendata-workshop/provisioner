# Job System Documentation

The OpenTofu Workspace Scheduler includes a comprehensive job scheduling system that allows you to run ad-hoc scheduled tasks both within workspace contexts and as standalone operations.

## Overview

The job system supports two deployment scenarios:

1. **Workspace-Embedded Jobs**: Jobs defined within workspace configurations that run in the context of that workspace
2. **Standalone Jobs**: Independent jobs that run outside of any workspace context, ideal for system maintenance and monitoring

## Job Types

All jobs support three execution types:

### Script Jobs
Execute shell scripts with full bash functionality:
```json
{
  "name": "backup-data",
  "type": "script",
  "schedule": "0 2 * * *",
  "script": "#!/bin/bash\nset -e\necho 'Creating backup...'\nmkdir -p /backup\ncp -r ./data /backup/$(date +%Y%m%d)",
  "timeout": "30m",
  "enabled": true,
  "description": "Daily data backup"
}
```

### Command Jobs
Execute single commands or simple command chains:
```json
{
  "name": "health-check",
  "type": "command",
  "schedule": "*/15 * * * *",
  "command": "curl -f http://localhost:8080/health || exit 1",
  "timeout": "30s",
  "enabled": true,
  "description": "Health check every 15 minutes"
}
```

### Template Jobs
Deploy or update OpenTofu templates:
```json
{
  "name": "deploy-monitoring",
  "type": "template",
  "schedule": "0 1 * * *",
  "template": "monitoring",
  "environment": {
    "MONITORING_LEVEL": "detailed",
    "ALERT_EMAIL": "admin@example.com"
  },
  "timeout": "15m",
  "enabled": true,
  "description": "Deploy monitoring infrastructure"
}
```

## Configuration Fields

All job types support these configuration fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique job identifier |
| `type` | string | Yes | Job type: `script`, `command`, or `template` |
| `schedule` | string/array | Yes | CRON expression(s) for scheduling |
| `enabled` | boolean | No | Whether job is active (default: true) |
| `description` | string | No | Human-readable description |
| `timeout` | string | No | Maximum execution time (default: 30m) |
| `environment` | object | No | Environment variables for execution |
| `working_dir` | string | No | Working directory for execution |

### Type-Specific Fields

**Script Jobs:**
- `script`: Shell script content to execute

**Command Jobs:**
- `command`: Command string to execute

**Template Jobs:**
- `template`: Name of template to deploy

## Workspace-Embedded Jobs

Jobs can be embedded directly in workspace configurations:

```json
{
  "enabled": true,
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "jobs": [
    {
      "name": "post-deploy-setup",
      "type": "script",
      "schedule": "0 10 * * 1-5",
      "script": "#!/bin/bash\necho 'Workspace deployed, running setup...'\n./setup.sh",
      "timeout": "10m",
      "enabled": true,
      "description": "Post-deployment setup tasks"
    },
    {
      "name": "monitoring-check",
      "type": "command",
      "schedule": ["0 */4 * * *"],
      "command": "./scripts/check-status.sh",
      "environment": {
        "WORKSPACE_NAME": "{{workspace_name}}"
      },
      "enabled": true,
      "description": "Regular monitoring checks"
    }
  ],
  "description": "Development workspace with automated jobs"
}
```

### Managing Workspace Jobs

```bash
# List jobs in a workspace
jobctl --workspace my-app list

# Show status of all jobs in workspace
jobctl --workspace my-app status

# Show status of specific job
jobctl --workspace my-app status backup-data

# Run job immediately
jobctl --workspace my-app run backup-data

# Kill running job
jobctl --workspace my-app kill long-running-task
```

## Standalone Jobs

Standalone jobs are defined in separate JSON files in the `jobs/` directory:

### Example: System Maintenance Job
**File: `jobs/cleanup-temp.json`**
```json
{
  "name": "cleanup-temp",
  "type": "script",
  "schedule": "0 2 * * *",
  "script": "#!/bin/bash\necho 'Cleaning up temporary files...'\nfind /tmp -type f -name '*.tmp' -mtime +1 -delete\nfind /tmp -type f -name 'core.*' -mtime +7 -delete\necho 'Cleanup completed'",
  "timeout": "10m",
  "enabled": true,
  "description": "Daily cleanup of temporary files and core dumps",
  "tags": ["maintenance", "cleanup"]
}
```

### Example: Health Monitoring Job
**File: `jobs/system-health.json`**
```json
{
  "name": "system-health",
  "type": "script",
  "schedule": ["0 */6 * * *", "0 0 * * *"],
  "script": "#!/bin/bash\necho '=== Disk Usage ==='\ndf -h\necho '\n=== Memory Usage ==='\nfree -h\necho '\n=== System Uptime ==='\nuptime\necho 'Health check completed'",
  "timeout": "5m",
  "enabled": true,
  "description": "System health check every 6 hours and daily",
  "tags": ["monitoring", "health"]
}
```

### Example: Configuration Backup Job
**File: `jobs/backup-configs.json`**
```json
{
  "name": "backup-configs",
  "type": "script",
  "schedule": "0 3 * * 0",
  "script": "#!/bin/bash\nset -e\n\nBACKUP_DIR=\"/var/backups/provisioner-$(date +%Y%m%d)\"\nmkdir -p \"$BACKUP_DIR\"\n\necho 'Creating configuration backup...'\ncp -r /etc/provisioner/workspaces \"$BACKUP_DIR/\"\ncp -r /var/lib/provisioner/templates \"$BACKUP_DIR/\"\n\necho 'Creating tarball...'\ntar -czf \"$BACKUP_DIR.tar.gz\" -C \"$(dirname \"$BACKUP_DIR\")\" \"$(basename \"$BACKUP_DIR\")\"\nrm -rf \"$BACKUP_DIR\"\n\necho \"Backup completed: $BACKUP_DIR.tar.gz\"\n\n# Keep only last 4 backups\nfind /var/backups -name 'provisioner-*.tar.gz' -type f | sort -r | tail -n +5 | xargs rm -f",
  "environment": {
    "PATH": "/usr/local/bin:/usr/bin:/bin"
  },
  "timeout": "30m",
  "enabled": true,
  "description": "Weekly backup of provisioner configurations and templates",
  "tags": ["backup", "maintenance"]
}
```

### Managing Standalone Jobs

```bash
# List all standalone jobs
jobctl list

# Show status of all standalone jobs
jobctl status

# Show status of specific job
jobctl status cleanup-temp

# Run job immediately
jobctl run cleanup-temp

# Kill running job
jobctl kill long-running-task
```

## Scheduling

### CRON Expression Support

The job system supports enhanced CRON expressions with:

- **Basic format**: `minute hour day month weekday`
- **Ranges**: `1-5` (Monday through Friday)
- **Lists**: `1,3,5` (Monday, Wednesday, Friday)
- **Intervals**: `*/15` (every 15 minutes)
- **Mixed combinations**: `1-5,0` (weekdays plus Sunday)

### Multiple Schedules

Jobs can have multiple schedules using arrays:

```json
{
  "schedule": ["0 8 * * 1-5", "0 20 * * 1-5", "0 14 * * 0"]
}
```

This runs the job at:
- 8 AM on weekdays
- 8 PM on weekdays
- 2 PM on Sundays

### Schedule Examples

| Schedule | Description |
|----------|-------------|
| `0 * * * *` | Every hour at minute 0 |
| `*/15 * * * *` | Every 15 minutes |
| `0 2 * * *` | Daily at 2 AM |
| `0 9 * * 1-5` | Weekdays at 9 AM |
| `0 0 1 * *` | First day of every month |
| `0 6 * * 0` | Sundays at 6 AM |

## Environment Variables

Jobs have access to built-in environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `WORKSPACE_ID` | Workspace identifier (or "_standalone_") | `my-app` |
| `JOB_NAME` | Name of the executing job | `backup-data` |
| `PATH` | System PATH variable | `/usr/bin:/bin` |

Plus any custom variables defined in the job configuration.

## Working Directory

Jobs execute in specific working directories:

- **Workspace jobs**: `/var/lib/provisioner/deployments/{workspace}/`
- **Standalone jobs**: `/var/lib/provisioner/deployments/_standalone_/`
- **Custom**: Override with `working_dir` field

## Job State and Monitoring

### Job States

| State | Description |
|-------|-------------|
| `pending` | Job scheduled but not yet run |
| `running` | Job currently executing |
| `success` | Job completed successfully |
| `failed` | Job failed with error |
| `timeout` | Job exceeded timeout limit |

### Execution Tracking

The system tracks for each job:
- **Run Count**: Total number of executions
- **Success Count**: Number of successful runs
- **Failure Count**: Number of failed runs
- **Last Run**: Timestamp of most recent execution
- **Last Success**: Timestamp of most recent success
- **Last Failure**: Timestamp of most recent failure
- **Last Error**: Error message from most recent failure
- **Next Run**: Calculated next execution time

### Viewing Job Status

```bash
# Standalone job status
jobctl status system-health

# Output:
# Job: system-health
# Type: standalone
# Status: success
# Run Count: 15
# Success Count: 14
# Failure Count: 1
# Last Run: 2025-09-27 12:00:01
# Last Success: 2025-09-27 12:00:01
# Last Failure: 2025-09-26 18:00:01
# Last Error: Command failed: exit status 1
# Next Run: 2025-09-27 18:00:00
```

## Use Cases

### System Administration

**Daily Maintenance**
```json
{
  "name": "daily-maintenance",
  "type": "script",
  "schedule": "0 3 * * *",
  "script": "#!/bin/bash\napt update\napt autoremove -y\njournalctl --vacuum-time=30d\ndocker system prune -f",
  "timeout": "1h",
  "enabled": true,
  "description": "Daily system maintenance tasks"
}
```

**Log Rotation**
```json
{
  "name": "log-rotation",
  "type": "script",
  "schedule": "0 1 * * *",
  "script": "#!/bin/bash\nfind /var/log -name '*.log' -size +100M -exec gzip {} \\;\nfind /var/log -name '*.log.gz' -mtime +30 -delete",
  "timeout": "30m",
  "enabled": true,
  "description": "Compress and clean old log files"
}
```

### Monitoring and Alerting

**Service Health Checks**
```json
{
  "name": "service-health",
  "type": "command",
  "schedule": "*/5 * * * *",
  "command": "systemctl is-active provisioner || systemctl restart provisioner",
  "timeout": "2m",
  "enabled": true,
  "description": "Ensure provisioner service is running"
}
```

### Development Workflows

**Database Backup** (Workspace Job)
```json
{
  "name": "db-backup",
  "type": "script",
  "schedule": "0 1 * * *",
  "script": "#!/bin/bash\npg_dump myapp > /backup/myapp-$(date +%Y%m%d).sql\nfind /backup -name 'myapp-*.sql' -mtime +7 -delete",
  "environment": {
    "PGPASSWORD": "{{database_password}}"
  },
  "timeout": "20m",
  "enabled": true,
  "description": "Nightly database backup"
}
```

**Code Deployment**
```json
{
  "name": "deploy-code",
  "type": "script",
  "schedule": "0 */2 * * *",
  "script": "#!/bin/bash\ncd /app\ngit pull origin main\nnpm install\nnpm run build\nsudo systemctl reload myapp",
  "timeout": "15m",
  "enabled": true,
  "description": "Pull and deploy latest code every 2 hours"
}
```

## Best Practices

### Job Design
1. **Idempotent Operations**: Design jobs to be safely re-runnable
2. **Error Handling**: Include proper error handling in scripts
3. **Timeouts**: Set appropriate timeouts for job execution
4. **Resource Limits**: Consider system resource usage

### Scheduling
1. **Avoid Overlap**: Ensure long-running jobs don't overlap
2. **Off-Peak Hours**: Schedule resource-intensive jobs during low usage
3. **Stagger Jobs**: Distribute job start times to avoid system load spikes

### Security
1. **Least Privilege**: Run jobs with minimal required permissions
2. **Secret Management**: Avoid hardcoding secrets in job configurations
3. **Input Validation**: Validate any external inputs in job scripts

### Monitoring
1. **Regular Status Checks**: Monitor job execution status
2. **Error Alerting**: Set up notifications for job failures
3. **Log Management**: Ensure job logs are collected and retained appropriately

## Troubleshooting

### Common Issues

**Job Not Running**
1. Check if job is enabled: `jobctl status job-name`
2. Verify schedule syntax is correct
3. Check system time and timezone settings

**Job Failing**
1. Review error message: `jobctl status job-name`
2. Check job permissions and working directory
3. Verify required dependencies are available
4. Test job script manually

**Performance Issues**
1. Review job timeout settings
2. Check for resource-intensive operations
3. Consider breaking large jobs into smaller ones
4. Monitor system resources during job execution

### Debugging Commands

```bash
# Check job configuration
jobctl list

# View detailed job status
jobctl status job-name

# View recent executions
journalctl -u provisioner | grep "JOB job-name"

# Test job manually (workspace jobs)
jobctl --workspace workspace-name run job-name

# Test job manually (standalone jobs)
jobctl run job-name
```