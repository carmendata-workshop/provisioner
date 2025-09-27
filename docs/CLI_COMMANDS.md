# CLI Commands Reference

The OpenTofu Workspace Scheduler provides several CLI tools for managing workspaces, templates, and jobs.

## Workspace Management (workspacectl)

### Deploy Workspace
```bash
workspacectl deploy my-app                    # Traditional deployment or interactive mode selection
workspacectl deploy my-app busy               # Deploy in specific mode (mode-based workspaces)
```

**Behavior:**
- Validates workspace exists and is enabled
- For mode-based workspaces: prompts for mode selection if not specified
- For traditional workspaces: deploys using configured deploy_schedule logic
- Checks workspace is not currently deploying/destroying
- Executes deployment immediately using OpenTofu
- Updates state and provides detailed logging

### Change Workspace Mode
```bash
workspacectl mode my-app hibernation          # Change to hibernation mode
workspacectl mode my-app busy                 # Change to busy mode
```

**Behavior:**
- Direct mode change for mode-based workspaces
- Validates mode is available in workspace template
- Confirms mode change if workspace is already deployed in different mode
- Updates deployment mode state tracking

### Destroy Workspace
```bash
workspacectl destroy test-workspace
```

**Behavior:**
- Validates workspace exists and is enabled
- Checks workspace is not currently deploying/destroying
- Executes destruction immediately using OpenTofu
- Updates state and provides detailed logging

### Show Workspace Status
```bash
workspacectl status                  # Show all workspaces
workspacectl status my-app          # Show specific workspace details
```

**Output Example:**
```
# All workspaces
WORKSPACE       STATUS       LAST DEPLOYED        LAST DESTROYED       ERRORS
-----------     ------       -------------        --------------       ------
my-app          deployed     2025-09-19 12:04     Never                None
test-workspace  destroyed    Never                2025-09-19 11:30     None

# Specific workspace
Workspace: my-app
Status: deployed
Enabled: true
Deploy Schedule: 0 9 * * 1-5
Destroy Schedule: 0 18 * * 1-5
Last Deployed: 2025-09-19 12:04:33
Last Destroyed: Never
Log File: /var/log/provisioner/my-app.log
```

### List All Workspaces
```bash
workspacectl list
```

**Output:**
- Shows all configured workspaces with their schedules
- Displays enabled/disabled status for each workspace
- Shows deploy and destroy CRON schedules
- Supports both single and multiple schedule formats

### View Workspace Logs
```bash
workspacectl logs my-app
```

**Output Example:**
```
=== Recent logs for workspace 'my-app' ===
Log file: /var/log/provisioner/my-app.log

2025/09/19 12:04:33 MANUAL DEPLOY: Starting manual deployment
2025/09/19 12:04:40 MANUAL DEPLOY: Successfully completed
```

## Template Management (templatectl)

### Add Template
```bash
# Add template from GitHub repository
templatectl add web-app https://github.com/org/terraform-templates --path workspaces/web-app --ref v2.1.0

# Add with description
templatectl add database https://github.com/company/infra-templates --path db/postgres --ref main --description "PostgreSQL database template"
```

### List Templates
```bash
templatectl list                    # Basic list
templatectl list --detailed         # Detailed information
```

**Output Example:**
```
NAME         SOURCE                                   REF    DESCRIPTION
web-app      https://github.com/company/infra-templates  v1.2.0  Standard web application template
postgres-db  https://github.com/company/infra-templates  main    PostgreSQL database template
```

### Show Template Details
```bash
templatectl show web-app
```

### Update Templates
```bash
templatectl update web-app          # Update specific template
templatectl update --all            # Update all templates
```

### Validate Templates
```bash
templatectl validate web-app        # Validate specific template
templatectl validate --all          # Validate all templates
```

### Remove Templates
```bash
templatectl remove web-app          # Interactive confirmation
templatectl remove web-app --force  # Skip confirmation
```

## Job Management (jobctl)

The `jobctl` command provides unified management for both standalone and workspace jobs.

### Standalone Jobs (Default)

```bash
# List all standalone jobs
jobctl list

# Show status of all standalone jobs
jobctl status

# Show status of specific standalone job
jobctl status cleanup-temp

# Run specific standalone job immediately
jobctl run cleanup-temp

# Kill running standalone job
jobctl kill cleanup-temp
```

### Workspace Jobs

Use the `--workspace` flag to manage jobs within a specific workspace:

```bash
# List all jobs in a workspace
jobctl --workspace my-app list

# Show status of all jobs in workspace
jobctl --workspace my-app status

# Show status of specific job in workspace
jobctl --workspace my-app status backup-db

# Run specific job immediately
jobctl --workspace my-app run backup-db

# Kill running job
jobctl --workspace my-app kill backup-db
```

### Job Status Output Example

```bash
jobctl status system-health

# Output:
Job: system-health
Type: standalone
Status: success
Run Count: 15
Success Count: 14
Failure Count: 1
Last Run: 2025-09-27 12:00:01
Last Success: 2025-09-27 12:00:01
Last Failure: 2025-09-26 18:00:01
Last Error: Command failed: exit status 1
Next Run: 2025-09-27 18:00:00
```

## Scheduler Daemon (provisioner)

### Run Scheduler
```bash
# Start the scheduler daemon
./bin/provisioner

# Run with specific configuration directory
PROVISIONER_CONFIG_DIR=/custom/path ./bin/provisioner
```

### Version Information
```bash
./bin/provisioner --version         # Show runtime version
./bin/provisioner --version-full     # Show detailed version info
./bin/provisioner --help            # Show command line help
```

## Development Commands

### Build and Test
```bash
make build          # Build the binary
make run            # Build and run the scheduler daemon
make dev            # Full development workflow (fmt, lint, test, build)
make test           # Run tests
make test-coverage  # Run tests with coverage (local development only)
make lint           # Run golangci-lint (or go vet/fmt if not installed)
make fmt            # Format code
```

### Version Management
```bash
make version                         # Show build version info
make next-version                    # Calculate next semantic version
make validate-commits                # Validate conventional commit format
```

## Error Handling

### Common Error Scenarios

**Workspace not found:**
```bash
workspacectl deploy nonexistent
# Error: workspace 'nonexistent' not found
```

**Workspace disabled:**
```bash
workspacectl deploy disabled-workspace
# Error: workspace 'disabled-workspace' is disabled
```

**Workspace busy:**
```bash
workspacectl deploy currently-deploying
# Error: workspace 'currently-deploying' is currently deploying
```

**Job not found:**
```bash
jobctl run nonexistent-job
# Error: standalone job 'nonexistent-job' not found
```

### Getting Help

All commands support the `--help` flag:

```bash
workspacectl --help
templatectl --help
jobctl --help
provisioner --help
```

## Command Examples by Use Case

### Daily Operations
```bash
# Check status of all workspaces
workspacectl status

# Deploy workspace for urgent testing
workspacectl deploy test-env

# Check template updates
templatectl list

# Run manual backup job
jobctl run backup-configs
```

### Troubleshooting
```bash
# View logs for problematic workspace
workspacectl logs failing-workspace

# Check job execution history
jobctl status system-health

# Validate workspace template
templatectl validate web-app
```

### Maintenance
```bash
# Update all templates
templatectl update --all

# Check workspace schedules
workspacectl list

# Run maintenance jobs
jobctl run cleanup-temp
jobctl run log-rotation
```

## Environment Variables

Set these variables to customize CLI behavior:

- `PROVISIONER_CONFIG_DIR` - Configuration directory (default: `/etc/provisioner`)
- `PROVISIONER_STATE_DIR` - State directory (default: `/var/lib/provisioner`)
- `PROVISIONER_LOG_DIR` - Log directory (default: `/var/log/provisioner`)

## Integration with Other Tools

### Systemd Service Management
```bash
# Check scheduler service status
sudo systemctl status provisioner

# View scheduler logs
sudo journalctl -u provisioner -f

# Restart scheduler after configuration changes
sudo systemctl restart provisioner
```

### Monitoring Scripts
```bash
# Check all workspace health in script
if workspacectl status | grep -q "ERROR"; then
    echo "Some workspaces have errors"
    workspacectl status
fi

# Check job execution in monitoring
if ! jobctl status backup-configs | grep -q "success"; then
    echo "Backup job failed"
    jobctl status backup-configs
fi
```