# OpenTofu Environment Scheduler (Provisioner)

**MVP Status: v0.x** - An automated environment scheduler that deploys and destroys development environments based on CRON schedules using OpenTofu.

## Overview

The Provisioner automatically manages OpenTofu environments on a schedule, allowing you to:
- Deploy environments at specified times (e.g., start of business day)
- Destroy environments to save costs (e.g., end of business day)
- Run multiple environments with overlapping schedules
- Track environment state across application restarts

## Features

- **Enhanced CRON scheduling** - Supports ranges, lists, intervals, and mixed combinations
- **Multiple schedule support** - Deploy/destroy environments at different times using arrays of CRON expressions
- **Template management system** - Centralized template storage with version control and sharing
- **Multiple environments** - Manage multiple environments simultaneously
- **State persistence** - Environment status survives application restarts
- **Configuration hot-reload** - Automatically detects changes to config files and templates
- **CLI monitoring and control** - Status checking, environment listing, log viewing, and manual operations
- **Mountain-themed naming** - Environment names follow mountain theme (everest, kilimanjaro, etc.)
- **Automatic OpenTofu management** - Downloads and manages OpenTofu binary automatically
- **Comprehensive logging** - All operations and state changes are logged

## Quick Start

1. **Build the applications:**
   ```bash
   make build
   ```

2. **Create an environment:**
   ```bash
   mkdir -p environments/example
   ```

3. **Add OpenTofu template** (`environments/example/main.tf`):
   ```hcl
   terraform {
     required_providers {
       local = {
         source  = "hashicorp/local"
         version = "~> 2.0"
       }
     }
   }

   resource "local_file" "environment_marker" {
     content  = "Environment: ${var.environment_name}\nDeployed at: ${timestamp()}\n"
     filename = "/tmp/${var.environment_name}_deployed.txt"
   }

   variable "environment_name" {
     description = "Name of the environment"
     type        = string
     default     = "example"
   }
   ```

4. **Add configuration** (`environments/example/config.json`):
   ```json
   {
     "enabled": true,
     "deploy_schedule": "0 9 * * 1-5",
     "destroy_schedule": "0 18 * * 1-5",
     "description": "Development environment - weekdays 9am-6pm"
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
   # Environment management
   ./bin/environmentctl list                    # List all environments
   ./bin/environmentctl status                  # Show environment status
   ./bin/environmentctl deploy example          # Deploy environment manually
   ./bin/environmentctl logs example            # View environment logs

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
│   ├── environment/         # Environment configuration loading
│   ├── template/            # Template management system
│   ├── opentofu/           # OpenTofu CLI wrapper
│   ├── logging/            # Dual logging (systemd + file)
│   └── version/            # Build information and versioning
├── environments/           # Environment configurations
│   ├── example/
│   │   ├── main.tf         # Local OpenTofu template
│   │   └── config.json     # Environment configuration
│   └── template-example/
│       └── config.json     # Environment using template reference
├── state/                  # State persistence
│   ├── scheduler.json      # Environment state tracking
│   └── templates/          # Template storage
│       ├── registry.json   # Template metadata
│       └── web-app/        # Template content
│           └── main.tf
└── logs/                   # Operation logs
```

## Environment Configuration

Each environment requires two files in `environments/{name}/`:

### config.json

**Local Template:**
```json
{
  "enabled": true,
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "description": "Environment with local main.tf"
}
```

**Template Reference:**
```json
{
  "enabled": true,
  "template": "web-app-v2",
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "description": "Environment using shared template"
}
```

**Multiple Schedule Support:**
Both `deploy_schedule` and `destroy_schedule` can accept either a single CRON expression (string) or multiple CRON expressions (array):

```json
{
  "name": "multi-schedule-env",
  "enabled": true,
  "deploy_schedule": ["0 7 * * 1,3,5", "0 8 * * 2,4"],
  "destroy_schedule": "30 17 * * 1-5",
  "description": "Multiple deploy schedules, single destroy schedule"
}
```

**Permanent Deployments:**
Set `destroy_schedule` to `false` for environments that should never be automatically destroyed:

```json
{
  "name": "permanent-env",
  "enabled": true,
  "deploy_schedule": "0 6 * * 1",
  "destroy_schedule": false,
  "description": "Permanent deployment - never destroyed"
}
```

**Field descriptions:**
- `enabled` - Whether environment should be processed by scheduler
- `template` - (Optional) Reference to managed template by name
- `deploy_schedule` - CRON expression(s) for deployment times (string or array of strings)
- `destroy_schedule` - CRON expression(s) for destruction times (string, array of strings, or `false` for permanent)
- `description` - Human-readable description

**Template Resolution Priority:**
1. **Local `main.tf`** - Always highest priority (allows customization)
2. **Template reference** - Uses template from template registry
3. **Error** - No template found

**Schedule Behavior:**
- **Single schedule**: Environment deploys/destroys at the specified time
- **Multiple schedules**: Environment deploys/destroys when ANY of the schedules match
- **Mixed formats**: Can mix single and multiple schedules (e.g., multiple deploy schedules with single destroy schedule)
- **Permanent deployment**: Use `destroy_schedule: false` to never automatically destroy

### main.tf
Standard OpenTofu/Terraform configuration file with your infrastructure definition.

**Note:** If using a template reference, `main.tf` is optional. Local `main.tf` files override template references for customization.

### State File Format
The scheduler maintains state in `scheduler.json`:
```json
{
  "environments": {
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

## Environment States

- **deployed** - Environment is currently active
- **destroyed** - Environment is currently inactive
- **deploying** - Deployment in progress
- **destroying** - Destruction in progress
- **pending** - Initial state, never deployed

## State Management

Environment state is automatically saved to `state/scheduler.json` and includes:
- Current status of each environment
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

# Use template in environment configuration (environments/my-app/config.json)
{
  "enabled": true,
  "template": "web-app",
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "description": "My web application using shared template"
}

# Update templates (checks for changes and updates environments on next deployment)
templatectl update web-app
```

## Example Environments

### Development Environment (Business Hours)
```json
{
  "enabled": true,
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "description": "Dev environment - weekdays 9am-6pm"
}
```

### Testing Environment (Twice Daily)
```json
{
  "enabled": true,
  "deploy_schedule": "0 6,14 * * *",
  "destroy_schedule": "0 12,20 * * *",
  "description": "Test environment - 6am-12pm, 2pm-8pm daily"
}
```

### Demo Environment (Weekdays Excluding Wednesday)
```json
{
  "enabled": true,
  "template": "demo-app",
  "deploy_schedule": "0 9 * * 1,2,4,5",
  "destroy_schedule": "30 17 * * 1,2,4,5",
  "description": "Demo environment using shared template - Mon/Tue/Thu/Fri 9am-5:30pm"
}
```

### Extended Hours Environment (Business Hours)
```json
{
  "enabled": true,
  "deploy_schedule": "0 7 * * 1-5",
  "destroy_schedule": "0 19 * * 1-5",
  "description": "Extended hours - weekdays 7am-7pm"
}
```

### Training Environment (Tuesday/Thursday Only)
```json
{
  "enabled": true,
  "deploy_schedule": "30 8 * * 2,4",
  "destroy_schedule": "30 16 * * 2,4",
  "description": "Training environment - Tue/Thu 8:30am-4:30pm"
}
```

### Permanent Environment (Never Destroyed)
```json
{
  "enabled": true,
  "deploy_schedule": "0 6 * * 1",
  "destroy_schedule": false,
  "description": "Production environment - permanent deployment"
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

### Testing Environment (Multiple Daily Cycles)
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
git commit -m "feat: add automatic environment scheduling"     # → Minor bump (v0.1.0 → v0.2.0)
git commit -m "fix: resolve CRON parsing issue"               # → Patch bump (v0.1.0 → v0.1.1)
git commit -m "feat!: change configuration API structure"     # → Major bump (v0.1.0 → v1.0.0)
git commit -m "docs: update installation instructions"        # → No bump
```

### Manual Installation

1. **Create user and directories:**
   ```bash
   sudo useradd --system --home-dir /var/lib/provisioner --shell /bin/false provisioner
   sudo mkdir -p /opt/provisioner /etc/provisioner/environments /var/lib/provisioner /var/log/provisioner
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
├── environments/
│   └── example/
│       ├── main.tf           # OpenTofu template
│       └── config.json       # Environment configuration

/var/lib/provisioner/         # State and template data
├── scheduler.json            # Environment state persistence
├── templates/                # Template storage
│   ├── registry.json         # Template metadata registry
│   ├── web-app-v2/           # Template content
│   │   ├── main.tf
│   │   └── variables.tf
│   └── database/
│       └── main.tf
└── deployments/              # Environment working directories
    ├── my-web-env/
    │   ├── main.tf           # Copied from template
    │   ├── terraform.tfstate # Environment state
    │   └── .provisioner-metadata.json
    └── another-env/

/var/log/provisioner/          # Application logs
├── (log files if file logging enabled)

Systemd logs: journalctl -u provisioner
```

**Environment Variables:**
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
- Environment loading and validation
- Schedule checking and execution
- OpenTofu command output
- State changes and errors
- Application lifecycle events

## Template Management

The provisioner includes a comprehensive template management system for sharing and versioning OpenTofu templates across multiple environments.

### Template Commands

**Add Template:**
```bash
# Add template from GitHub repository
templatectl add web-app https://github.com/org/terraform-templates --path environments/web-app --ref v2.1.0

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
      "source_path": "environments/web-app",
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
- **Currently deployed environments**: Continue running with current template version (stable)
- **Next scheduled deployment**: Automatically uses updated template
- **Manual deployment**: `environmentctl deploy env-name` forces immediate template update
- **Change detection**: Only real content changes trigger environment updates

**Template Resolution Priority:**
1. **Local `main.tf`**: Always highest priority (allows environment-specific customization)
2. **Template reference**: Resolved from template registry
3. **Error**: No template found

### Working Directory Management

Each environment gets its own isolated working directory:

```
/var/lib/provisioner/deployments/
├── my-web-env/
│   ├── main.tf                     # Copied from template
│   ├── variables.tf                # Additional template files
│   ├── terraform.tfstate           # Environment-specific state
│   └── .provisioner-metadata.json # Template tracking
└── another-env/
    └── ...
```

**Benefits:**
- **State Isolation**: Each environment maintains separate Terraform state
- **Template Stability**: Updates don't disrupt running environments
- **Change Tracking**: Content hashing detects real template changes
- **Version Control**: Templates track source URL, path, and Git ref

## CLI Commands

The provisioner provides several commands for managing and monitoring environments:

### Environment Management

**Deploy Environment**
```bash
environmentctl deploy my-app
```
- Validates environment exists and is enabled
- Checks environment is not currently deploying/destroying
- Executes deployment immediately using OpenTofu
- Updates state and provides detailed logging

**Destroy Environment**
```bash
environmentctl destroy test-env
```
- Validates environment exists and is enabled
- Checks environment is not currently deploying/destroying
- Executes destruction immediately using OpenTofu
- Updates state and provides detailed logging

### Environment Monitoring

**Show Environment Status**
```bash
environmentctl status                  # Show all environments
environmentctl status my-app          # Show specific environment details
```
- Displays current status (deployed, destroyed, deploying, destroying)
- Shows last deployment and destruction timestamps
- Indicates if there are any errors
- For specific environments: shows detailed configuration and log file location

**List All Environments**
```bash
environmentctl list
```
- Shows all configured environments with their schedules
- Displays enabled/disabled status for each environment
- Shows deploy and destroy CRON schedules
- Supports both single and multiple schedule formats

**View Environment Logs**
```bash
environmentctl logs my-app
```
- Shows recent logs for the specified environment
- Displays detailed OpenTofu operation output
- Includes timestamps and operation types (DEPLOY, DESTROY, MANUAL DEPLOY, etc.)
- Shows the full log file path for reference

### Command Examples

```bash
# Check status of all environments
environmentctl status
# Output:
# ENVIRONMENT     STATUS       LAST DEPLOYED        LAST DESTROYED       ERRORS
# -----------     ------       -------------        --------------       ------
# my-app          deployed     2025-09-19 12:04     Never                None
# test-env        destroyed    Never                2025-09-19 11:30     None

# Get detailed info about specific environment
environmentctl status my-app
# Output:
# Environment: my-app
# Status: deployed
# Enabled: true
# Deploy Schedule: 0 9 * * 1-5
# Destroy Schedule: 0 18 * * 1-5
# Last Deployed: 2025-09-19 12:04:33
# Last Destroyed: Never
# Log File: /var/log/provisioner/my-app.log

# View recent deployment logs
environmentctl logs my-app
# Output:
# === Recent logs for environment 'my-app' ===
# Log file: /var/log/provisioner/my-app.log
#
# 2025/09/19 12:04:33 MANUAL DEPLOY: Starting manual deployment
# 2025/09/19 12:04:40 MANUAL DEPLOY: Successfully completed
```

### Error Handling
- **Environment not found**: Returns clear error message
- **Environment disabled**: Returns error, requires enabling in `config.json`
- **Environment busy**: Returns error if currently deploying/destroying
- **Operation failures**: Logs detailed errors to environment-specific log files
- **Invalid arguments**: Shows helpful usage information

## Limitations (MVP)

- No retry logic for failed operations
- No web UI or API
- No multi-instance coordination
- No advanced error recovery
- Single cloud provider per environment

## Contributing

This is an MVP implementation focused on core functionality. When extending:
1. Follow existing package structure
2. Maintain minimal external dependencies
3. Use structured logging for operations
4. Test with simple OpenTofu templates first