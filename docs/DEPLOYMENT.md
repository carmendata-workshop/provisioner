# Deployment Guide

This guide covers installation, deployment, and service management for the OpenTofu Workspace Scheduler.

## Quick Installation (Recommended)

### Install Latest Version
```bash
curl -fsSL https://raw.githubusercontent.com/carmendata-workshop/provisioner/main/scripts/install.sh | sudo bash
```

### Install Specific Version
```bash
curl -fsSL https://raw.githubusercontent.com/carmendata-workshop/provisioner/main/scripts/install.sh | sudo bash -s v0.1.0
```

### Start the Service
```bash
sudo systemctl start provisioner
sudo systemctl status provisioner
```

### View Logs
```bash
sudo journalctl -u provisioner -f
```

## Manual Installation

### 1. Create User and Directories

```bash
sudo useradd --system --home-dir /var/lib/provisioner --shell /bin/false provisioner
sudo mkdir -p /opt/provisioner /etc/provisioner/workspaces /var/lib/provisioner /var/log/provisioner
```

### 2. Copy Binary and Set Permissions

```bash
sudo cp provisioner /opt/provisioner/
sudo chown root:root /opt/provisioner/provisioner
sudo chown -R provisioner:provisioner /etc/provisioner /var/lib/provisioner /var/log/provisioner
```

### 3. Install Systemd Service

```bash
sudo cp deployment/provisioner.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable provisioner
```

## Local Development Installation

### Build the Application
```bash
make build
```

### Install Locally
```bash
make install
```

### Development Workflow
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

### Cross-Platform Builds
```bash
make build-all      # Build for all platforms (Linux, macOS, ARM64, AMD64)
```

### Version Information
```bash
make version                     # Show build version info
make next-version               # Calculate next semantic version from commits
./bin/provisioner --version     # Show runtime version
./bin/provisioner --version-full # Show detailed version info
./bin/provisioner --help        # Show command line help
```

### Conventional Commits
```bash
make validate-commits # Validate commit message format

# Commit message examples:
git commit -m "feat: add automatic workspace scheduling"     # → Minor bump (v0.1.0 → v0.2.0)
git commit -m "fix: resolve CRON parsing issue"               # → Patch bump (v0.1.0 → v0.1.1)
git commit -m "feat!: change configuration API structure"     # → Major bump (v0.1.0 → v1.0.0)
git commit -m "docs: update installation instructions"        # → No bump
```

## Service Management

### Systemd Commands
```bash
# Start service
sudo systemctl start provisioner

# Stop service
sudo systemctl stop provisioner

# Restart service
sudo systemctl restart provisioner

# Check status
sudo systemctl status provisioner

# Enable auto-start on boot
sudo systemctl enable provisioner

# Disable auto-start
sudo systemctl disable provisioner
```

### Log Management
```bash
# View logs
sudo journalctl -u provisioner -f

# View recent logs
sudo journalctl -u provisioner --since "1 hour ago"

# View logs from specific date
sudo journalctl -u provisioner --since "2025-01-15 09:00:00"

# Export logs to file
sudo journalctl -u provisioner --since "today" > provisioner.log
```

## File System Layout

### Production (FHS Compliant)

When installed via the installer, files are organized according to the Linux Filesystem Hierarchy Standard:

```
/opt/provisioner/             # Application binaries
├── provisioner              # Main scheduler daemon
├── workspacectl             # Workspace management CLI
├── templatectl              # Template management CLI
└── jobctl                   # Job management CLI

/etc/provisioner/             # Configuration files
├── workspaces/              # Workspace configurations
│   ├── example/
│   │   ├── main.tf          # OpenTofu template
│   │   └── config.json      # Workspace configuration
│   └── web-app/
│       └── config.json      # Template-based workspace
└── jobs/                    # Standalone job configurations
    ├── cleanup-temp.json    # Daily cleanup job
    ├── system-health.json   # Health monitoring job
    └── backup-configs.json  # Configuration backup job

/var/lib/provisioner/         # State and template data
├── scheduler.json           # Workspace state persistence
├── jobs.json               # Job execution state and history
├── templates/              # Template storage
│   ├── registry.json       # Template metadata registry
│   ├── web-app-v2/         # Template content
│   │   ├── main.tf
│   │   └── variables.tf
│   └── database/
│       └── main.tf
└── deployments/            # Workspace working directories
    ├── my-web-workspace/
    │   ├── main.tf         # Copied from template
    │   ├── terraform.tfstate # Workspace state
    │   └── .provisioner-metadata.json
    ├── another-workspace/
    └── _standalone_/       # Standalone job working directory
        └── job-execution-files

/var/log/provisioner/        # Application logs
├── (log files if file logging enabled)

Systemd logs: journalctl -u provisioner
```

### Development

```
provisioner/
├── main.go                 # Application entry point
├── go.mod                  # Go dependencies
├── Makefile                # Build and development commands
├── pkg/                    # Go packages
├── workspaces/           # Workspace configurations
│   ├── example/
│   │   ├── main.tf         # Local OpenTofu template
│   │   └── config.json     # Workspace configuration with jobs
│   └── template-example/
│       └── config.json     # Workspace using template reference
├── jobs/                   # Standalone job configurations
│   ├── cleanup-temp.json   # Daily cleanup job
│   ├── system-health.json  # Health monitoring job
│   └── backup-configs.json # Configuration backup job
├── state/                  # State persistence
│   ├── scheduler.json      # Scheduler state
│   └── jobs.json           # Job execution state
└── bin/                    # Built binaries
    ├── provisioner         # Main scheduler daemon
    ├── workspacectl        # Workspace management CLI
    ├── templatectl         # Template management CLI
    └── jobctl              # Job management CLI
```

## Environment Variables

### Configuration Directories

```bash
# Set custom configuration directory
export PROVISIONER_CONFIG_DIR="/custom/config/path"

# Set custom state directory
export PROVISIONER_STATE_DIR="/custom/state/path"

# Set custom log directory
export PROVISIONER_LOG_DIR="/custom/log/path"
```

### Default Values

- `PROVISIONER_CONFIG_DIR` - Configuration directory (default: `/etc/provisioner`)
- `PROVISIONER_STATE_DIR` - State directory (default: `/var/lib/provisioner`)
- `PROVISIONER_LOG_DIR` - Log directory (default: `/var/log/provisioner`)

## Dependencies

### Required
- **Go 1.25.1+** - For building the application
- **OpenTofu binary** - Automatically downloaded if not in PATH
- **systemd** - For service management on Linux

### Go Dependencies
- **github.com/opentofu/tofudl** - OpenTofu binary management (only external dependency)
- **Standard library only** - All other functionality uses Go standard library

## Systemd Service Configuration

### Service File (`/etc/systemd/system/provisioner.service`)

```ini
[Unit]
Description=OpenTofu Workspace Scheduler
After=network.target

[Service]
Type=simple
User=provisioner
Group=provisioner
ExecStart=/opt/provisioner/provisioner
Restart=always
RestartSec=5
Environment=PROVISIONER_CONFIG_DIR=/etc/provisioner
Environment=PROVISIONER_STATE_DIR=/var/lib/provisioner
Environment=PROVISIONER_LOG_DIR=/var/log/provisioner

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/provisioner /var/log/provisioner
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### Service Security Features

- **Dedicated User**: Runs as `provisioner` system user
- **No New Privileges**: Prevents privilege escalation
- **Protected Filesystem**: Read-only access to system directories
- **Private Temp**: Isolated temporary directory
- **Restart Policy**: Automatic restart on failure

## Configuration Validation

### Startup Checks

The provisioner performs validation on startup:

1. **Configuration Directory**: Ensures config directory exists and is readable
2. **Workspace Validation**: Validates all workspace configurations
3. **Template Validation**: Checks template references and availability
4. **Job Validation**: Validates job configurations and schedules
5. **CRON Expressions**: Validates all CRON schedule syntax
6. **Permissions**: Checks required file/directory permissions

### Pre-Installation Validation

```bash
# Check Go version
go version

# Verify system requirements
uname -a

# Check available disk space
df -h /var/lib /etc

# Verify systemd availability
systemctl --version
```

## Backup and Recovery

### Configuration Backup

```bash
# Backup configuration files
sudo tar -czf provisioner-config-backup.tar.gz -C /etc provisioner/

# Backup state data
sudo tar -czf provisioner-state-backup.tar.gz -C /var/lib provisioner/
```

### State Recovery

```bash
# Stop service
sudo systemctl stop provisioner

# Restore configuration
sudo tar -xzf provisioner-config-backup.tar.gz -C /etc/

# Restore state
sudo tar -xzf provisioner-state-backup.tar.gz -C /var/lib/

# Fix permissions
sudo chown -R provisioner:provisioner /etc/provisioner /var/lib/provisioner

# Start service
sudo systemctl start provisioner
```

## Monitoring and Health Checks

### Service Health

```bash
# Check service status
sudo systemctl is-active provisioner

# Check service health
sudo systemctl status provisioner

# Monitor resource usage
sudo systemctl show provisioner --property=MainPID
ps -p $(sudo systemctl show provisioner --property=MainPID --value) -o pid,ppid,cmd,%mem,%cpu
```

### Log Monitoring

```bash
# Monitor for errors
sudo journalctl -u provisioner | grep -i error

# Monitor workspace operations
sudo journalctl -u provisioner | grep -E "(DEPLOY|DESTROY)"

# Check job execution
sudo journalctl -u provisioner | grep "JOB"
```

### Health Check Script

```bash
#!/bin/bash
# provisioner-health-check.sh

# Check if service is running
if ! systemctl is-active --quiet provisioner; then
    echo "ERROR: Provisioner service is not running"
    exit 1
fi

# Check if configuration directory exists
if [ ! -d "/etc/provisioner/workspaces" ]; then
    echo "ERROR: Configuration directory not found"
    exit 1
fi

# Check if state directory is writable
if [ ! -w "/var/lib/provisioner" ]; then
    echo "ERROR: State directory is not writable"
    exit 1
fi

echo "Provisioner health check passed"
exit 0
```

## Troubleshooting

### Common Issues

**Service won't start:**
```bash
# Check service status
sudo systemctl status provisioner

# Check logs for errors
sudo journalctl -u provisioner --since "10 minutes ago"

# Verify binary permissions
ls -la /opt/provisioner/provisioner
```

**Permission errors:**
```bash
# Fix ownership
sudo chown -R provisioner:provisioner /etc/provisioner /var/lib/provisioner /var/log/provisioner

# Check SELinux context (if applicable)
ls -Z /opt/provisioner/provisioner
```

**Configuration errors:**
```bash
# Validate configuration manually
/opt/provisioner/provisioner --help

# Check workspace configurations
find /etc/provisioner/workspaces -name "*.json" -exec echo "Checking {}" \; -exec cat {} \;
```

**OpenTofu issues:**
```bash
# Check OpenTofu installation
which tofu

# Verify OpenTofu version
tofu version

# Check download directory
ls -la /var/lib/provisioner/.tofu/
```

### Getting Help

1. **Check logs**: `sudo journalctl -u provisioner -f`
2. **Validate configuration**: Review workspace and job configurations
3. **Test manually**: Use CLI tools to test individual operations
4. **Check permissions**: Ensure proper file ownership and permissions
5. **Resource limits**: Check available disk space and memory

### Support Information

When reporting issues, include:

- **Version**: `./bin/provisioner --version-full`
- **System Info**: `uname -a`
- **Service Status**: `sudo systemctl status provisioner`
- **Recent Logs**: `sudo journalctl -u provisioner --since "1 hour ago"`
- **Configuration**: Workspace and job configurations (redact sensitive data)