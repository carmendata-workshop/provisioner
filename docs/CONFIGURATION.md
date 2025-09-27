# Configuration Guide

This guide covers all configuration options for the OpenTofu Workspace Scheduler.

## Workspace Configuration

Each workspace requires two files in `workspaces/{name}/`:

### config.json

**Local Template:**
```json
{
  "enabled": true,
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "description": "Workspace with local main.tf"
}
```

**Template Reference:**
```json
{
  "enabled": true,
  "template": "web-app-v2",
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "description": "Workspace using shared template"
}
```

**Multiple Schedule Support:**
Both `deploy_schedule` and `destroy_schedule` can accept either a single CRON expression (string) or multiple CRON expressions (array):

```json
{
  "name": "multi-schedule-workspace",
  "enabled": true,
  "deploy_schedule": ["0 7 * * 1,3,5", "0 8 * * 2,4"],
  "destroy_schedule": "30 17 * * 1-5",
  "description": "Multiple deploy schedules, single destroy schedule"
}
```

**Mode-Based Scheduling:**
For dynamic resource scaling, use `mode_schedules` instead of `deploy_schedule`:

```json
{
  "enabled": true,
  "template": "web-app-v2",
  "mode_schedules": {
    "hibernation": ["0 23 * * 1-5", "0 0 * * 6,0"],
    "quiet": "0 6 * * 1-5",
    "busy": ["0 8 * * 1-5", "0 13 * * 1-5"],
    "maintenance": "0 2 * * 0"
  },
  "destroy_schedule": "0 1 * * 0",
  "description": "Dynamic scaling with multiple deployment modes"
}
```

**Workspace Jobs Configuration:**
Workspaces can also include scheduled jobs that run within the workspace context:

```json
{
  "enabled": true,
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "jobs": [
    {
      "name": "backup-data",
      "type": "script",
      "schedule": "0 2 * * *",
      "script": "#!/bin/bash\necho 'Creating backup...'\nmkdir -p /tmp/backup\ncp -r ./data /tmp/backup/",
      "timeout": "30m",
      "enabled": true,
      "description": "Daily data backup"
    },
    {
      "name": "health-check",
      "type": "command",
      "schedule": ["0 */6 * * *"],
      "command": "curl -f http://localhost:8080/health",
      "timeout": "5m",
      "enabled": true,
      "description": "Periodic health check"
    },
    {
      "name": "scale-monitoring",
      "type": "template",
      "schedule": "0 1 * * *",
      "template": "monitoring",
      "environment": {
        "MONITORING_LEVEL": "detailed"
      },
      "timeout": "15m",
      "enabled": true,
      "description": "Deploy monitoring infrastructure"
    }
  ],
  "description": "Workspace with scheduled jobs"
}
```

**Permanent Deployments:**
Set `destroy_schedule` to `false` for workspaces that should never be automatically destroyed:

```json
{
  "name": "permanent-workspace",
  "enabled": true,
  "deploy_schedule": "0 6 * * 1",
  "destroy_schedule": false,
  "description": "Permanent deployment - never destroyed"
}
```

### Configuration Fields

- `enabled` - Whether workspace should be processed by scheduler
- `template` - (Optional) Reference to managed template by name
- `deploy_schedule` - CRON expression(s) for deployment times (string or array of strings) - **mutually exclusive with `mode_schedules`**
- `mode_schedules` - Map of deployment modes to CRON schedules for dynamic scaling - **requires `template` field**
- `destroy_schedule` - CRON expression(s) for destruction times (string, array of strings, or `false` for permanent)
- `jobs` - Array of job configurations for workspace-embedded jobs
- `description` - Human-readable description

### Job Configuration Fields

- **name**: Unique job identifier within the workspace
- **type**: Job type (`script`, `command`, or `template`)
- **schedule**: CRON expression(s) for when to run the job
- **script**: Shell script content (for `script` type)
- **command**: Command to execute (for `command` type)
- **template**: Template name to deploy (for `template` type)
- **environment**: Environment variables for job execution
- **working_dir**: Working directory for job execution (optional)
- **timeout**: Maximum execution time (default: 30m)
- **enabled**: Whether the job is active
- **description**: Human-readable description

### Template Resolution Priority

1. **Local `main.tf`** - Always highest priority (allows customization)
2. **Template reference** - Uses template from template registry
3. **Error** - No template found

### Schedule Behavior

- **Traditional scheduling** (`deploy_schedule`): Workspace deploys/destroys at specified times
- **Mode-based scheduling** (`mode_schedules`): Workspace transitions between different resource configurations
- **Single schedule**: Workspace deploys/destroys at the specified time
- **Multiple schedules**: Workspace deploys/destroys when ANY of the schedules match
- **Mixed formats**: Can mix single and multiple schedules (e.g., multiple deploy schedules with single destroy schedule)
- **Permanent deployment**: Use `destroy_schedule: false` to never automatically destroy
- **Mode transitions**: Workspace stays in current mode until another mode schedule triggers or destroy_schedule runs

## main.tf

Standard OpenTofu/Terraform configuration file with your infrastructure definition.

**Note:** If using a template reference, `main.tf` is optional. Local `main.tf` files override template references for customization.

## Standalone Jobs Configuration

Standalone jobs run independently of any workspace and are configured in separate JSON files in the `jobs/` directory:

```json
{
  "name": "cleanup-temp",
  "type": "script",
  "schedule": "0 2 * * *",
  "script": "#!/bin/bash\necho 'Cleaning up temporary files...'\nfind /tmp -type f -name '*.tmp' -mtime +1 -delete\necho 'Cleanup completed'",
  "timeout": "10m",
  "enabled": true,
  "description": "Daily cleanup of temporary files"
}
```

See [Job System Documentation](./JOB_SYSTEM.md) for complete details on job configuration and management.

## State File Format

The scheduler maintains state in `scheduler.json`:

```json
{
  "workspaces": {
    "example": {
      "status": "deployed",
      "last_deployed": "2025-09-15T09:00:00Z",
      "last_destroyed": "2025-09-14T18:00:00Z",
      "last_deploy_error": "",
      "last_destroy_error": ""
    }
  },
  "last_updated": "2025-09-15T10:30:00Z"
}
```

**Status values:** `deployed`, `destroyed`, `pending`, `deploying`, `destroying`

## Environment Variables

The following environment variables configure the provisioner:

- `PROVISIONER_CONFIG_DIR` - Configuration directory (default: `/etc/provisioner`)
- `PROVISIONER_STATE_DIR` - State directory (default: `/var/lib/provisioner`)
- `PROVISIONER_LOG_DIR` - Log directory (default: `/var/log/provisioner`)

## Example Configurations

### Development Workspace (Business Hours)
```json
{
  "enabled": true,
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "description": "Dev workspace - weekdays 9am-6pm"
}
```

### Testing Workspace (Twice Daily)
```json
{
  "enabled": true,
  "deploy_schedule": "0 6,14 * * *",
  "destroy_schedule": "0 12,20 * * *",
  "description": "Test workspace - 6am-12pm, 2pm-8pm daily"
}
```

### Demo Workspace (Weekdays Excluding Wednesday)
```json
{
  "enabled": true,
  "template": "demo-app",
  "deploy_schedule": "0 9 * * 1,2,4,5",
  "destroy_schedule": "30 17 * * 1,2,4,5",
  "description": "Demo workspace using shared template - Mon/Tue/Thu/Fri 9am-5:30pm"
}
```

### Multiple Deploy Schedules (Different Times for Different Days)
```json
{
  "enabled": true,
  "deploy_schedule": ["0 7 * * 1,3,5", "0 8 * * 2,4"],
  "destroy_schedule": "30 17 * * 1-5",
  "description": "Earlier start Mon/Wed/Fri (7am), later start Tue/Thu (8am)"
}
```

### Testing Workspace (Multiple Daily Cycles)
```json
{
  "enabled": true,
  "deploy_schedule": ["0 6 * * 1-5", "0 14 * * 1-5"],
  "destroy_schedule": ["0 12 * * 1-5", "0 18 * * 1-5"],
  "description": "Twice daily - 6am-12pm and 2pm-6pm on weekdays"
}
```