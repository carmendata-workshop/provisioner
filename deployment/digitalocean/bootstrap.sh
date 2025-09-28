#!/bin/bash
# DigitalOcean Droplet Bootstrap Script for Provisioner Installation
# This script is executed via cloud-init user_data

set -euo pipefail

# Configuration from template variables
PROVISIONER_VERSION="${provisioner_version}"
GITHUB_REPO="${github_repo}"
SERVER_TIMEZONE="${server_timezone}"
AUTO_START="${auto_start}"

# Logging setup
LOG_FILE="/var/log/provisioner-bootstrap.log"
exec 1> >(tee -a "$LOG_FILE")
exec 2> >(tee -a "$LOG_FILE" >&2)

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

error() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" >&2
}

log "Starting provisioner bootstrap process..."
log "Configuration:"
log "  Version: $PROVISIONER_VERSION"
log "  Repository: $GITHUB_REPO"
log "  Timezone: $SERVER_TIMEZONE"
log "  Auto-start: $AUTO_START"

# Update system packages
log "Updating system packages..."
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get upgrade -y

# Set timezone
log "Setting timezone to $SERVER_TIMEZONE..."
timedatectl set-timezone "$SERVER_TIMEZONE"

# Install essential packages
log "Installing essential packages..."
apt-get install -y \
    curl \
    wget \
    git \
    jq \
    tree \
    vim \
    htop \
    net-tools \
    systemd \
    ca-certificates \
    gnupg \
    lsb-release \
    unzip \
    tar

# Configure automatic security updates
log "Configuring automatic security updates..."
apt-get install -y unattended-upgrades apt-listchanges
echo 'Unattended-Upgrade::Automatic-Reboot "false";' >> /etc/apt/apt.conf.d/50unattended-upgrades
systemctl enable unattended-upgrades

# Download and install provisioner
log "Downloading and installing provisioner..."
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

# Construct download URL
if [ "$PROVISIONER_VERSION" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/latest/download/install.sh"
    log "Using latest version from: $DOWNLOAD_URL"
else
    DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$PROVISIONER_VERSION/install.sh"
    log "Using specific version from: $DOWNLOAD_URL"
fi

# Download and execute install script
if curl -fsSL "$DOWNLOAD_URL" -o install.sh; then
    log "Install script downloaded successfully"
    chmod +x install.sh

    # Execute install script
    if [ "$PROVISIONER_VERSION" = "latest" ]; then
        ./install.sh
    else
        ./install.sh "$PROVISIONER_VERSION"
    fi

    log "Provisioner installation completed"
else
    error "Failed to download install script from $DOWNLOAD_URL"
    exit 1
fi

# Clean up
cd /
rm -rf "$TEMP_DIR"

# Verify installation
log "Verifying installation..."
if [ -f "/opt/provisioner/provisioner" ]; then
    log "Provisioner binary installed successfully"
    /opt/provisioner/provisioner --version
else
    error "Provisioner binary not found after installation"
    exit 1
fi

# Check if service exists
if systemctl list-unit-files | grep -q "^provisioner.service"; then
    log "Provisioner service installed successfully"
else
    error "Provisioner service not found after installation"
    exit 1
fi

# Configure service startup
if [ "$AUTO_START" = "true" ]; then
    log "Starting provisioner service..."
    systemctl enable provisioner
    systemctl start provisioner

    # Wait a moment and check status
    sleep 5
    if systemctl is-active --quiet provisioner; then
        log "Provisioner service started successfully"
    else
        error "Provisioner service failed to start"
        systemctl status provisioner --no-pager
        journalctl -u provisioner --no-pager -n 20
        exit 1
    fi
else
    log "Auto-start disabled - service ready but not started"
    systemctl enable provisioner
fi

# Create basic monitoring script
log "Creating monitoring script..."
cat > /usr/local/bin/provisioner-health << 'EOF'
#!/bin/bash
# Provisioner health check script

echo "=== Provisioner Health Check ==="
echo "Date: $(date)"
echo

# Service status
echo "Service Status:"
systemctl status provisioner --no-pager -l
echo

# Resource usage
echo "Resource Usage:"
echo "Memory: $(free -h | grep Mem)"
echo "Disk: $(df -h / | tail -1)"
echo "Load: $(uptime)"
echo

# Recent logs
echo "Recent Logs (last 10 lines):"
journalctl -u provisioner --no-pager -n 10
echo

# Workspace status
echo "Workspace Status:"
if command -v workspacectl >/dev/null 2>&1; then
    workspacectl status 2>/dev/null || echo "No workspaces configured or accessible"
else
    echo "workspacectl not in PATH"
fi
EOF

chmod +x /usr/local/bin/provisioner-health

# Create log rotation for provisioner logs
log "Setting up log rotation..."
cat > /etc/logrotate.d/provisioner << 'EOF'
/var/log/provisioner/*.log {
    daily
    missingok
    rotate 7
    compress
    delaycompress
    notifempty
    create 640 provisioner provisioner
    postrotate
        systemctl reload provisioner || true
    endscript
}
EOF

# Setup basic firewall (UFW)
log "Configuring basic firewall..."
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw --force enable

# Create welcome message with useful information
log "Creating welcome message..."
cat > /etc/motd << 'EOF'
================================================================================
                        OpenTofu Workspace Provisioner Server
================================================================================

This server is running the OpenTofu Workspace Provisioner - an automated
workspace scheduler for infrastructure deployments.

Useful Commands:
  systemctl status provisioner    - Check service status
  journalctl -u provisioner -f    - View live logs
  workspacectl list               - List configured workspaces
  workspacectl status             - Show workspace status
  templatectl list                - List available templates
  jobctl list                     - List configured jobs
  provisioner-health              - Run health check

Configuration:
  Workspaces: /etc/provisioner/workspaces/
  Templates:  /var/lib/provisioner/templates/
  State:      /var/lib/provisioner/
  Logs:       /var/log/provisioner/

For documentation and support:
  https://github.com/carmendata-workshop/provisioner

================================================================================
EOF

# Setup data volume if mount point exists (created by Terraform)
if [ -d "/mnt/provisioner-data" ]; then
    log "Configuring data volume..."

    # Format and mount the volume if it's not already mounted
    DEVICE=$(lsblk -ln -o NAME,MOUNTPOINT | grep -v "/" | head -1 | awk '{print $1}')
    if [ -n "$DEVICE" ] && [ ! -d "/mnt/provisioner-data/lost+found" ]; then
        log "Formatting and mounting data volume: /dev/$DEVICE"
        mkfs.ext4 "/dev/$DEVICE"
        mount "/dev/$DEVICE" /mnt/provisioner-data

        # Add to fstab for persistent mounting
        echo "/dev/$DEVICE /mnt/provisioner-data ext4 defaults,nofail 0 2" >> /etc/fstab

        # Create directories and set permissions
        mkdir -p /mnt/provisioner-data/{state,logs,templates,deployments}
        chown -R provisioner:provisioner /mnt/provisioner-data

        # Create symlinks to data volume
        systemctl stop provisioner || true
        mv /var/lib/provisioner /var/lib/provisioner.backup 2>/dev/null || true
        mv /var/log/provisioner /var/log/provisioner.backup 2>/dev/null || true

        ln -sf /mnt/provisioner-data/state /var/lib/provisioner
        ln -sf /mnt/provisioner-data/logs /var/log/provisioner

        # Restore any existing data
        if [ -d "/var/lib/provisioner.backup" ]; then
            cp -r /var/lib/provisioner.backup/* /mnt/provisioner-data/state/ 2>/dev/null || true
        fi
        if [ -d "/var/log/provisioner.backup" ]; then
            cp -r /var/log/provisioner.backup/* /mnt/provisioner-data/logs/ 2>/dev/null || true
        fi

        chown -R provisioner:provisioner /mnt/provisioner-data
        systemctl start provisioner || true

        log "Data volume configured successfully"
    else
        log "Data volume appears to be already configured or device not found"
    fi
fi

# Final system status
log "Bootstrap completed! System status:"
log "  Provisioner version: $(/opt/provisioner/provisioner --version 2>/dev/null || echo 'Version check failed')"
log "  Service status: $(systemctl is-active provisioner)"
log "  Disk usage: $(df -h / | tail -1)"
log "  Memory usage: $(free -h | grep Mem)"

log "Bootstrap process completed successfully!"
log "Server is ready for use. Connect via SSH and run 'provisioner-health' for status."

# Signal completion to cloud-init
echo "Provisioner bootstrap completed at $(date)" > /var/log/bootstrap-complete