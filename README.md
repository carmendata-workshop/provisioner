# OpenTofu Workspace Scheduler (Provisioner)

**MVP Status: v0.x** - An automated workspace scheduler that deploys and destroys development workspaces based on CRON schedules using OpenTofu.

## Overview

The Provisioner automatically manages OpenTofu workspaces on a schedule, allowing you to:
- Deploy workspaces at specified times (e.g., start of business day)
- Destroy workspaces to save costs (e.g., end of business day)
- Run scheduled jobs within workspaces for automation and maintenance
- Manage standalone jobs for system-wide operations
- Run multiple workspaces with overlapping schedules
- Track workspace state across application restarts

## Features

- **Job Scheduling System** - Run scheduled tasks within workspaces and standalone operations
- **Mode-based deployments** - Deploy workspaces in different resource configurations (hibernation, busy, maintenance) with automatic mode transitions
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

2. **Create a workspace:**
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

   # Job management
   ./bin/jobctl list                          # List standalone jobs
   ./bin/jobctl --workspace example list      # List workspace jobs

   # Template management
   ./bin/templatectl list                     # List all templates
   ./bin/templatectl add web-app https://github.com/org/templates --path web --ref v1.0
   ```

## Documentation

- **[Configuration Guide](./docs/CONFIGURATION.md)** - Complete configuration reference for workspaces, jobs, and scheduling
- **[CRON Scheduling](./docs/CRON_SCHEDULING.md)** - Detailed guide to CRON expressions and scheduling patterns
- **[CLI Commands](./docs/CLI_COMMANDS.md)** - Complete reference for all CLI tools and commands
- **[Template Management](./docs/TEMPLATES.md)** - Guide to template creation, management, and usage
- **[Job System](./docs/JOB_SYSTEM.md)** - Complete documentation for the job scheduling system
- **[Deployment Guide](./docs/DEPLOYMENT.md)** - Installation, deployment, and service management

## Directory Structure

```
provisioner/
├── main.go                   # Application entry point
├── go.mod                    # Go module dependencies
├── pkg/
│   ├── scheduler/           # CRON scheduling and state management
│   ├── workspace/           # Workspace configuration loading
│   ├── template/            # Template management system
│   ├── job/                 # Job scheduling and execution system
│   ├── opentofu/           # OpenTofu CLI wrapper
│   ├── logging/            # Dual logging (systemd + file)
│   └── version/            # Build information and versioning
├── workspaces/             # Workspace configurations
│   ├── example/
│   │   ├── main.tf         # Local OpenTofu template
│   │   └── config.json     # Workspace configuration
│   └── template-example/
│       └── config.json     # Workspace using template reference
├── jobs/                   # Standalone job configurations
│   ├── cleanup-temp.json   # Daily cleanup job
│   └── system-health.json  # Health monitoring job
├── state/                  # State persistence
│   ├── scheduler.json      # Workspace state tracking
│   └── jobs.json          # Job execution state
└── bin/                   # Built binaries
    ├── provisioner        # Main scheduler daemon
    ├── workspacectl       # Workspace management CLI
    ├── templatectl        # Template management CLI
    └── jobctl             # Job management CLI
```

## Core Features

### Workspace Scheduling

Configure workspaces with flexible CRON scheduling:

```json
{
  "enabled": true,
  "deploy_schedule": ["0 9 * * 1-5", "0 14 * * 6"],
  "destroy_schedule": "0 18 * * 1-5",
  "description": "Deploy weekdays at 9am and Saturdays at 2pm"
}
```

### Mode-Based Scaling

Deploy workspaces in different resource configurations:

```json
{
  "enabled": true,
  "template": "web-app",
  "mode_schedules": {
    "hibernation": "0 23 * * 1-5",
    "busy": ["0 8 * * 1-5", "0 13 * * 1-5"],
    "maintenance": "0 2 * * 0"
  }
}
```

### Job System

Schedule tasks within workspaces or as standalone operations:

```json
{
  "jobs": [
    {
      "name": "backup-data",
      "type": "script",
      "schedule": "0 2 * * *",
      "script": "#!/bin/bash\necho 'Creating backup...'\nmkdir -p /backup\ncp -r ./data /backup/",
      "timeout": "30m",
      "enabled": true
    }
  ]
}
```

### Template Management

Share and version control OpenTofu templates:

```bash
# Add template from repository
./bin/templatectl add web-app https://github.com/org/templates --path web --ref v1.0

# Use in workspace configuration
{
  "enabled": true,
  "template": "web-app",
  "deploy_schedule": "0 9 * * 1-5"
}
```

## Key Commands

### Development
```bash
make build          # Build the binary
make run            # Build and run the scheduler daemon
make dev            # Full development workflow (fmt, lint, test, build)
make test           # Run tests
make test-coverage  # Run tests with coverage (local development only)
make lint           # Run golangci-lint (or go vet/fmt if not installed)
make fmt            # Format code
```

### Workspace Management
```bash
./bin/workspacectl deploy my-app      # Deploy workspace immediately
./bin/workspacectl destroy test-ws   # Destroy workspace immediately
./bin/workspacectl status             # Show all workspace status
./bin/workspacectl list               # List all configured workspaces
./bin/workspacectl logs my-app        # Show recent logs for workspace
```

### Job Management
```bash
./bin/jobctl list                     # List all standalone jobs
./bin/jobctl status cleanup-temp      # Show status of specific job
./bin/jobctl run cleanup-temp         # Run job immediately
./bin/jobctl --workspace my-app list  # List jobs in workspace
```

### Template Management
```bash
./bin/templatectl list                # List all templates
./bin/templatectl add web-app https://github.com/org/templates --path web --ref v1.0
./bin/templatectl update web-app      # Update template
```

## Installation

### Quick Installation (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/carmendata-workshop/provisioner/main/scripts/install.sh | sudo bash
sudo systemctl start provisioner
```

### Local Development

```bash
make build
make install
```

See the **[Deployment Guide](./docs/DEPLOYMENT.md)** for complete installation instructions.

## Workspace States

- **deployed** - Workspace is currently active
- **destroyed** - Workspace is currently inactive
- **deploying** - Deployment in progress
- **destroying** - Destruction in progress
- **pending** - Initial state, never deployed

## Dependencies

- **Go 1.25.1+** - For building the application
- **github.com/opentofu/tofudl** - OpenTofu binary management (only external dependency)
- **OpenTofu binary** - Automatically downloaded if not in PATH
- **systemd** - For service management on Linux

## Technical Details

- **MVP Design** - No retry logic, simple error handling, single instance only
- **File-based state** - Uses JSON for persistence
- **Standard library focus** - Minimal external dependencies
- **Hybrid state management** - Uses OpenTofu state files as source of truth with managed metadata
- **Hot reload** - Configuration changes detected automatically
- **Automatic versioning** - Based on conventional commit messages

## Contributing

This is an MVP implementation focused on core functionality. When extending:
1. Follow existing package structure
2. Maintain minimal external dependencies
3. Use structured logging for operations
4. Test with simple OpenTofu templates first
5. Follow conventional commit format for automatic versioning

## Limitations (MVP)

- No retry logic for failed operations
- No web UI or API
- No multi-instance coordination
- No advanced error recovery
- Single cloud provider per workspace

---

For detailed documentation on specific features, see the files in the `docs/` directory.