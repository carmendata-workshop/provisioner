# OpenTofu Workspace Scheduler (Provisioner)

**MVP Status: v0.x** - An automated workspace scheduler that deploys and destroys development workspaces based on CRON schedules using OpenTofu.

## Overview

The Provisioner automatically manages OpenTofu workspaces on a schedule, allowing you to:
- Deploy workspaces at specified times (e.g., start of business day)
- Destroy workspaces to save costs (e.g., end of business day)
- Run multiple workspaces with overlapping schedules
- Track workspace state across application restarts

## Features

- **Enhanced CRON scheduling** - Supports ranges, lists, intervals, and mixed combinations
- **Multiple schedule support** - Deploy/destroy workspaces at different times using arrays of CRON expressions
- **Template management system** - Centralized template storage with version control and sharing
- **Multiple workspaces** - Manage multiple workspaces simultaneously
- **State persistence** - Workspace status survives application restarts
- **Configuration hot-reload** - Automatically detects changes to config files and templates
- **CLI monitoring and control** - Status checking, workspace listing, log viewing, and manual operations
- **Mountain-themed naming** - Workspace names follow mountain theme (everest, kilimanjaro, etc.)
- **Automatic OpenTofu management** - Downloads and manages OpenTofu binary automatically
- **Comprehensive logging** - All operations and state changes are logged

## Quick Start

1. **Build the applications:**
   ```bash
   make build
   ```

2. **Create an workspace:**
   ```bash
   mkdir -p workspaces/example
   ```

3. **Add OpenTofu template** (`workspaces/example/main.tf`):
   ```hcl
   terraform {
     required_providers {
       local = {
         source  = "hashicorp/local"
         version = "~> 2.0"
       }
     }
   }

   resource "local_file" "workspace_marker" {
     content  = "Workspace: ${var.workspace_name}\nDeployed at: ${timestamp()}\n"
     filename = "/tmp/${var.workspace_name}_deployed.txt"
   }

   variable "workspace_name" {
     description = "Name of the workspace"
     type        = string
     default     = "example"
   }
   ```

4. **Add configuration** (`workspaces/example/config.json`):
   ```json
   {
     "enabled": true,
     "deploy_schedule": "0 9 * * 1-5",
     "destroy_schedule": "0 18 * * 1-5",
     "description": "Development workspace - weekdays 9am-6pm"
   }
   ```

5. **Run the scheduler:**
   ```bash
   make run
   # or
   ./bin/provisioner
   ```

6. **Monitor and manage with CLI tools:**
   ```bash
   # Workspace management
   ./bin/workspacectl list                    # List all workspaces
   ./bin/workspacectl status                  # Show workspace status
   ./bin/workspacectl deploy example          # Deploy workspace manually
   ./bin/workspacectl logs example            # View workspace logs

   # Template management
   ./bin/templatectl list                       # List all templates
   ./bin/templatectl add web-app https://github.com/org/templates --path web --ref v1.0
   ```

## Directory Structure

```
provisioner/
├── main.go                   # Application entry point
├── go.mod                    # Go module dependencies
├── pkg/
│   ├── scheduler/           # CRON scheduling and state management
│   ├── workspace/         # Workspace configuration loading
│   ├── template/            # Template management system
│   ├── opentofu/           # OpenTofu CLI wrapper
│   ├── logging/            # Dual logging (systemd + file)
│   └── version/            # Build information and versioning
├── workspaces/           # Workspace configurations
│   ├── example/
│   │   ├── main.tf         # Local OpenTofu template
│   │   └── config.json     # Workspace configuration
│   └── template-example/
│       └── config.json     # Workspace using template reference
├── state/                  # State persistence
│   ├── scheduler.json      # Workspace state tracking
│   └── templates/          # Template storage
│       ├── registry.json   # Template metadata
│       └── web-app/        # Template content
│           └── main.tf
└── logs/                   # Operation logs
```

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

**Field descriptions:**
- `enabled` - Whether workspace should be processed by scheduler
- `template` - (Optional) Reference to managed template by name
- `deploy_schedule` - CRON expression(s) for deployment times (string or array of strings)
- `destroy_schedule` - CRON expression(s) for destruction times (string, array of strings, or `false` for permanent)
- `description` - Human-readable description

**Template Resolution Priority:**
1. **Local `main.tf`** - Always highest priority (allows customization)
2. **Template reference** - Uses template from template registry
3. **Error** - No template found

**Schedule Behavior:**
- **Single schedule**: Workspace deploys/destroys at the specified time
- **Multiple schedules**: Workspace deploys/destroys when ANY of the schedules match
- **Mixed formats**: Can mix single and multiple schedules (e.g., multiple deploy schedules with single destroy schedule)
- **Permanent deployment**: Use `destroy_schedule: false` to never automatically destroy

### main.tf
Standard OpenTofu/Terraform configuration file with your infrastructure definition.

**Note:** If using a template reference, `main.tf` is optional. Local `main.tf` files override template references for customization.

### State File Format
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

## CRON Schedule Format

Uses standard 5-field CRON format: `minute hour day month day-of-week`

**Field Values:**
- `minute` - 0-59
- `hour` - 0-23
- `day` - 1-31
- `month` - 1-12
- `day-of-week` - 0-6 (Sunday=0)

**Supported Syntax:**
- `*` - Match all values
- `5` - Specific value
- `1-5` - Range of values
- `1,3,5` - List of values
- `*/5` - Every 5th value (intervals)
- `1-3,5` - Mixed ranges and values
- `1,3-5` - Mixed values and ranges
- `1-2,4-5` - Multiple ranges

**Basic Examples:**
- `0 9 * * 1-5` - 9:00 AM, Monday through Friday
- `*/15 * * * *` - Every 15 minutes
- `0 */2 * * *` - Every 2 hours
- `0 0 * * 0` - Midnight every Sunday

**Advanced Examples:**
- `0 9 * * 1,2,4,5` - 9:00 AM, Mon/Tue/Thu/Fri (excluding Wednesday)
- `0 9-17 * * 1-5` - Every hour from 9am-5pm, weekdays
- `30 8,12,17 * * 1-5` - 8:30am, 12:30pm, 5:30pm on weekdays
- `0 */3 * * 1,3,5` - Every 3 hours on Mon/Wed/Fri
- `15 9-17/2 * * 1-5` - 15 minutes past every 2nd hour 9am-5pm, weekdays

## Workspace States

- **deployed** - Workspace is currently active
- **destroyed** - Workspace is currently inactive
- **deploying** - Deployment in progress
- **destroying** - Destruction in progress
- **pending** - Initial state, never deployed

## State Management

Workspace state is automatically saved to `state/scheduler.json` and includes:
- Current status of each workspace
- Last deployment/destruction timestamps
- Error messages from failed operations
- State persistence across application restarts

## Template Usage Examples

### Adding and Using Templates

```bash
# Add a web application template from GitHub
templatectl add web-app https://github.com/company/infra-templates --path templates/web-app --ref v1.2.0 --description "Standard web application template"

# Add a database template
templatectl add postgres-db https://github.com/company/infra-templates --path templates/postgres --ref main --description "PostgreSQL database template"

# List available templates
templatectl list
# Output:
# NAME         SOURCE                                   REF    DESCRIPTION
# web-app      https://github.com/company/infra-templates  v1.2.0  Standard web application template
# postgres-db  https://github.com/company/infra-templates  main    PostgreSQL database template

# Use template in workspace configuration (workspaces/my-app/config.json)
{
  "enabled": true,
  "template": "web-app",
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "description": "My web application using shared template"
}

# Update templates (checks for changes and updates workspaces on next deployment)
templatectl update web-app
```

## Example Workspaces

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

### Extended Hours Workspace (Business Hours)
```json
{
  "enabled": true,
  "deploy_schedule": "0 7 * * 1-5",
  "destroy_schedule": "0 19 * * 1-5",
  "description": "Extended hours - weekdays 7am-7pm"
}
```

### Training Workspace (Tuesday/Thursday Only)
```json
{
  "enabled": true,
  "deploy_schedule": "30 8 * * 2,4",
  "destroy_schedule": "30 16 * * 2,4",
  "description": "Training workspace - Tue/Thu 8:30am-4:30pm"
}
```

### Permanent Workspace (Never Destroyed)
```json
{
  "enabled": true,
  "deploy_schedule": "0 6 * * 1",
  "destroy_schedule": false,
  "description": "Production workspace - permanent deployment"
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

## Deployment

### Quick Installation (Recommended)

**Install latest version:**
```bash
curl -fsSL https://raw.githubusercontent.com/carmendata-workshop/provisioner/main/scripts/install.sh | sudo bash
```

**Install specific version:**
```bash
curl -fsSL https://raw.githubusercontent.com/carmendata-workshop/provisioner/main/scripts/install.sh | sudo bash -s v0.1.0
```

**Start the service:**
```bash
sudo systemctl start provisioner
sudo systemctl status provisioner
```

**View logs:**
```bash
sudo journalctl -u provisioner -f
```

### Local Development Installation

1. **Build the application:**
   ```bash
   make build
   ```

2. **Install locally:**
   ```bash
   make install
   ```

### Development Workflow

**Standard Make targets:**
```bash
make build          # Build the binary
make test           # Run tests
make test-coverage  # Run tests with coverage
make lint           # Run linter
make fmt            # Format code
make clean          # Clean build artifacts
make dev            # Full development workflow (fmt, lint, test, build)
make help           # Show all available targets
```

**Cross-platform builds:**
```bash
make build-all      # Build for all platforms (Linux, macOS, ARM64, AMD64)
```

**Version information:**
```bash
make version                     # Show build version info
make next-version               # Calculate next semantic version from commits
./bin/provisioner --version     # Show runtime version
./bin/provisioner --version-full # Show detailed version info
./bin/provisioner --help        # Show command line help
```

**Conventional Commits:**
```bash
make validate-commits # Validate commit message format

# Commit message examples:
git commit -m "feat: add automatic workspace scheduling"     # → Minor bump (v0.1.0 → v0.2.0)
git commit -m "fix: resolve CRON parsing issue"               # → Patch bump (v0.1.0 → v0.1.1)
git commit -m "feat!: change configuration API structure"     # → Major bump (v0.1.0 → v1.0.0)
git commit -m "docs: update installation instructions"        # → No bump
```

### Manual Installation

1. **Create user and directories:**
   ```bash
   sudo useradd --system --home-dir /var/lib/provisioner --shell /bin/false provisioner
   sudo mkdir -p /opt/provisioner /etc/provisioner/workspaces /var/lib/provisioner /var/log/provisioner
   ```

2. **Copy binary and set permissions:**
   ```bash
   sudo cp provisioner /opt/provisioner/
   sudo chown root:root /opt/provisioner/provisioner
   sudo chown -R provisioner:provisioner /etc/provisioner /var/lib/provisioner /var/log/provisioner
   ```

3. **Install systemd service:**
   ```bash
   sudo cp deployment/provisioner.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable provisioner
   ```

### Service Management

```bash
# Start service
sudo systemctl start provisioner

# Stop service
sudo systemctl stop provisioner

# Restart service
sudo systemctl restart provisioner

# Check status
sudo systemctl status provisioner

# View logs
sudo journalctl -u provisioner -f

# View recent logs
sudo journalctl -u provisioner --since "1 hour ago"
```

## File System Layout (FHS Compliant)

When installed via the installer, files are organized according to the Linux Filesystem Hierarchy Standard:

```
/opt/provisioner/             # Application binary
├── provisioner

/etc/provisioner/             # Configuration files
├── workspaces/
│   └── example/
│       ├── main.tf           # OpenTofu template
│       └── config.json       # Workspace configuration

/var/lib/provisioner/         # State and template data
├── scheduler.json            # Workspace state persistence
├── templates/                # Template storage
│   ├── registry.json         # Template metadata registry
│   ├── web-app-v2/           # Template content
│   │   ├── main.tf
│   │   └── variables.tf
│   └── database/
│       └── main.tf
└── deployments/              # Workspace working directories
    ├── my-web-workspace/
    │   ├── main.tf           # Copied from template
    │   ├── terraform.tfstate # Workspace state
    │   └── .provisioner-metadata.json
    └── another-workspace/

/var/log/provisioner/          # Application logs
├── (log files if file logging enabled)

Systemd logs: journalctl -u provisioner
```

**Workspace Variables:**
- `PROVISIONER_CONFIG_DIR` - Configuration directory (default: `/etc/provisioner`)
- `PROVISIONER_STATE_DIR` - State directory (default: `/var/lib/provisioner`)
- `PROVISIONER_LOG_DIR` - Log directory (default: `/var/log/provisioner`)

## Dependencies

- **Go 1.25.1+** - For building the application
- **github.com/opentofu/tofudl** - OpenTofu binary management
- **OpenTofu binary** - Automatically downloaded if not in PATH
- **systemd** - For service management on Linux

## Versioning and Releases

This project uses **Conventional Commits** with **automatic semantic versioning**.

### Commit Message Format
```
type(scope): description

feat: add new feature           → Minor version bump (1.0.0 → 1.1.0)
fix: resolve bug               → Patch version bump (1.0.0 → 1.0.1)
feat!: breaking change         → Major version bump (1.0.0 → 2.0.0)
docs: update documentation     → No version bump
```

### Automatic Release Process
1. **Commit with conventional format** → Push to main branch
2. **GitHub Actions analyzes commits** → Calculates semantic version
3. **Automatic release created** → Tagged with new version
4. **Multi-platform binaries built** → Available for download
5. **Changelog generated** → Based on commit messages

### Manual Commands
```bash
# Check what version would be next
make next-version

# Validate your commit messages
make validate-commits

# Preview version impact (in PR)
# Automatically shown in GitHub Actions
```

## Technical Details

- **No error handling/retry logic** - Logs errors and continues (MVP design)
- **File-based state** - Uses JSON for persistence
- **Standard library only** - Minimal external dependencies
- **Single instance deployment** - No clustering or scaling
- **Temporary working directories** - Each operation uses isolated working directory
- **Automatic versioning** - Based on conventional commit messages

## Logging

All operations are logged with timestamps:
- Workspace loading and validation
- Schedule checking and execution
- OpenTofu command output
- State changes and errors
- Application lifecycle events

## Template Management

The provisioner includes a comprehensive template management system for sharing and versioning OpenTofu templates across multiple workspaces.

### Template Commands

**Add Template:**
```bash
# Add template from GitHub repository
templatectl add web-app https://github.com/org/terraform-templates --path workspaces/web-app --ref v2.1.0

# Add with description
templatectl add database https://github.com/company/infra-templates --path db/postgres --ref main --description "PostgreSQL database template"
```

**List Templates:**
```bash
templatectl list                    # Basic list
templatectl list --detailed         # Detailed information
```

**Show Template Details:**
```bash
templatectl show web-app
```

**Update Templates:**
```bash
templatectl update web-app          # Update specific template
templatectl update --all            # Update all templates
```

**Validate Templates:**
```bash
templatectl validate web-app        # Validate specific template
templatectl validate --all          # Validate all templates
```

**Remove Templates:**
```bash
templatectl remove web-app          # Interactive confirmation
templatectl remove web-app --force  # Skip confirmation
```

### Template Storage Structure

Templates are stored in `/var/lib/provisioner/templates/`:

```
/var/lib/provisioner/templates/
├── registry.json                 # Template metadata registry
├── web-app-v2/                  # Template content
│   ├── main.tf
│   ├── variables.tf
│   └── outputs.tf
└── database/                    # Another template
    ├── main.tf
    └── variables.tf
```

### Template Registry Format

The template registry (`registry.json`) tracks metadata:

```json
{
  "templates": {
    "web-app-v2": {
      "name": "web-app-v2",
      "source_url": "https://github.com/org/terraform-templates",
      "source_path": "workspaces/web-app",
      "source_ref": "v2.1.0",
      "created_at": "2025-01-15T10:30:00Z",
      "updated_at": "2025-01-15T10:30:00Z",
      "content_hash": "abc123...",
      "description": "Modern web application template",
      "version": "v2.1.0"
    }
  }
}
```

### Template Update Behavior

**When a template is updated:**
- **Currently deployed workspaces**: Continue running with current template version (stable)
- **Next scheduled deployment**: Automatically uses updated template
- **Manual deployment**: `workspacectl deploy workspace-name` forces immediate template update
- **Change detection**: Only real content changes trigger workspace updates

**Template Resolution Priority:**
1. **Local `main.tf`**: Always highest priority (allows workspace-specific customization)
2. **Template reference**: Resolved from template registry
3. **Error**: No template found

### Working Directory Management

Each workspace gets its own isolated working directory:

```
/var/lib/provisioner/deployments/
├── my-web-workspace/
│   ├── main.tf                     # Copied from template
│   ├── variables.tf                # Additional template files
│   ├── terraform.tfstate           # Workspace-specific state
│   └── .provisioner-metadata.json # Template tracking
└── another-workspace/
    └── ...
```

**Benefits:**
- **State Isolation**: Each workspace maintains separate Terraform state
- **Template Stability**: Updates don't disrupt running workspaces
- **Change Tracking**: Content hashing detects real template changes
- **Version Control**: Templates track source URL, path, and Git ref

## CLI Commands

The provisioner provides several commands for managing and monitoring workspaces:

### Workspace Management

**Deploy Workspace**
```bash
workspacectl deploy my-app
```
- Validates workspace exists and is enabled
- Checks workspace is not currently deploying/destroying
- Executes deployment immediately using OpenTofu
- Updates state and provides detailed logging

**Destroy Workspace**
```bash
workspacectl destroy test-workspace
```
- Validates workspace exists and is enabled
- Checks workspace is not currently deploying/destroying
- Executes destruction immediately using OpenTofu
- Updates state and provides detailed logging

### Workspace Monitoring

**Show Workspace Status**
```bash
workspacectl status                  # Show all workspaces
workspacectl status my-app          # Show specific workspace details
```
- Displays current status (deployed, destroyed, deploying, destroying)
- Shows last deployment and destruction timestamps
- Indicates if there are any errors
- For specific workspaces: shows detailed configuration and log file location

**List All Workspaces**
```bash
workspacectl list
```
- Shows all configured workspaces with their schedules
- Displays enabled/disabled status for each workspace
- Shows deploy and destroy CRON schedules
- Supports both single and multiple schedule formats

**View Workspace Logs**
```bash
workspacectl logs my-app
```
- Shows recent logs for the specified workspace
- Displays detailed OpenTofu operation output
- Includes timestamps and operation types (DEPLOY, DESTROY, MANUAL DEPLOY, etc.)
- Shows the full log file path for reference

### Command Examples

```bash
# Check status of all workspaces
workspacectl status
# Output:
# WORKSPACE     STATUS       LAST DEPLOYED        LAST DESTROYED       ERRORS
# -----------     ------       -------------        --------------       ------
# my-app          deployed     2025-09-19 12:04     Never                None
# test-workspace        destroyed    Never                2025-09-19 11:30     None

# Get detailed info about specific workspace
workspacectl status my-app
# Output:
# Workspace: my-app
# Status: deployed
# Enabled: true
# Deploy Schedule: 0 9 * * 1-5
# Destroy Schedule: 0 18 * * 1-5
# Last Deployed: 2025-09-19 12:04:33
# Last Destroyed: Never
# Log File: /var/log/provisioner/my-app.log

# View recent deployment logs
workspacectl logs my-app
# Output:
# === Recent logs for workspace 'my-app' ===
# Log file: /var/log/provisioner/my-app.log
#
# 2025/09/19 12:04:33 MANUAL DEPLOY: Starting manual deployment
# 2025/09/19 12:04:40 MANUAL DEPLOY: Successfully completed
```

### Error Handling
- **Workspace not found**: Returns clear error message
- **Workspace disabled**: Returns error, requires enabling in `config.json`
- **Workspace busy**: Returns error if currently deploying/destroying
- **Operation failures**: Logs detailed errors to workspace-specific log files
- **Invalid arguments**: Shows helpful usage information

## Limitations (MVP)

- No retry logic for failed operations
- No web UI or API
- No multi-instance coordination
- No advanced error recovery
- Single cloud provider per workspace

## Contributing

This is an MVP implementation focused on core functionality. When extending:
1. Follow existing package structure
2. Maintain minimal external dependencies
3. Use structured logging for operations
4. Test with simple OpenTofu templates first