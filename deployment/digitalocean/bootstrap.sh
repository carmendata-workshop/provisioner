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

# Configure data volume (always present)
log "Configuring persistent data volume..."

# Wait for volume to be attached (DigitalOcean volumes are attached at /dev/disk/by-id/scsi-0DO_Volume_*)
VOLUME_DEVICE=""
VOLUME_MOUNT="/mnt/provisioner-data"
TIMEOUT=60
COUNTER=0

log "Waiting for data volume to be attached..."
while [ $COUNTER -lt $TIMEOUT ]; do
    # Look for DigitalOcean volume by ID pattern
    VOLUME_DEVICE=$(find /dev/disk/by-id -name "scsi-0DO_Volume_*" 2>/dev/null | head -1)
    if [ -n "$VOLUME_DEVICE" ]; then
        log "Found data volume: $VOLUME_DEVICE"
        break
    fi
    sleep 1
    COUNTER=$((COUNTER + 1))
done

if [ -z "$VOLUME_DEVICE" ]; then
    error "Data volume not found after $TIMEOUT seconds. Cannot continue without persistent storage."
    exit 1
fi

# Create mount point
mkdir -p "$VOLUME_MOUNT"

# Check if volume is already formatted (look for existing filesystem)
if ! blkid "$VOLUME_DEVICE" >/dev/null 2>&1; then
    log "Formatting new data volume..."
    mkfs.ext4 -F "$VOLUME_DEVICE"
    log "Data volume formatted successfully"
else
    log "Data volume already has filesystem, skipping format"
fi

# Mount the volume
log "Mounting data volume..."
mount "$VOLUME_DEVICE" "$VOLUME_MOUNT"

# Add to fstab for persistent mounting (use UUID for reliability)
VOLUME_UUID=$(blkid -s UUID -o value "$VOLUME_DEVICE")
log "Adding volume to fstab with UUID: $VOLUME_UUID"
# Remove any existing entry for this mount point
sed -i "\|$VOLUME_MOUNT|d" /etc/fstab
echo "UUID=$VOLUME_UUID $VOLUME_MOUNT ext4 defaults,nofail 0 2" >> /etc/fstab

# Setup persistent SSH host keys
log "Configuring persistent SSH host keys..."
SSH_KEYS_DIR="$VOLUME_MOUNT/ssh-host-keys"
mkdir -p "$SSH_KEYS_DIR"

# If this is a new volume or no SSH keys exist, generate them
if [ ! -f "$SSH_KEYS_DIR/ssh_host_rsa_key" ]; then
    log "Generating new SSH host keys for persistence..."
    ssh-keygen -t rsa -b 4096 -f "$SSH_KEYS_DIR/ssh_host_rsa_key" -N ""
    ssh-keygen -t ecdsa -f "$SSH_KEYS_DIR/ssh_host_ecdsa_key" -N ""
    ssh-keygen -t ed25519 -f "$SSH_KEYS_DIR/ssh_host_ed25519_key" -N ""
    chmod 600 "$SSH_KEYS_DIR"/ssh_host_*_key
    chmod 644 "$SSH_KEYS_DIR"/ssh_host_*_key.pub
    log "New SSH host keys generated and stored on persistent volume"
else
    log "Using existing SSH host keys from persistent volume"
fi

# Replace system SSH host keys with persistent ones
systemctl stop ssh || systemctl stop sshd || true
rm -f /etc/ssh/ssh_host_*_key*
cp "$SSH_KEYS_DIR"/ssh_host_* /etc/ssh/
chown root:root /etc/ssh/ssh_host_*
chmod 600 /etc/ssh/ssh_host_*_key
chmod 644 /etc/ssh/ssh_host_*_key.pub
systemctl start ssh || systemctl start sshd || true
log "SSH host keys installed from persistent storage"

# Create directory structure on data volume
log "Setting up directory structure on data volume..."
mkdir -p "$VOLUME_MOUNT"/{state,logs,templates,deployments,configs}

# Setup symlinks for standard filesystem compliance
log "Creating symlinks for standard filesystem compliance..."

# Stop provisioner service if running
systemctl stop provisioner 2>/dev/null || true

# Backup existing data if present
if [ -d "/var/lib/provisioner" ] && [ ! -L "/var/lib/provisioner" ]; then
    log "Backing up existing provisioner state data..."
    cp -r /var/lib/provisioner/* "$VOLUME_MOUNT/state/" 2>/dev/null || true
    mv /var/lib/provisioner /var/lib/provisioner.backup
fi

if [ -d "/var/log/provisioner" ] && [ ! -L "/var/log/provisioner" ]; then
    log "Backing up existing provisioner log data..."
    cp -r /var/log/provisioner/* "$VOLUME_MOUNT/logs/" 2>/dev/null || true
    mv /var/log/provisioner /var/log/provisioner.backup
fi

if [ -d "/etc/provisioner" ] && [ ! -L "/etc/provisioner" ]; then
    log "Backing up existing provisioner configuration..."
    cp -r /etc/provisioner/* "$VOLUME_MOUNT/configs/" 2>/dev/null || true
    mv /etc/provisioner /etc/provisioner.backup
fi

# Create symlinks to persistent storage
ln -sf "$VOLUME_MOUNT/state" /var/lib/provisioner
ln -sf "$VOLUME_MOUNT/logs" /var/log/provisioner
ln -sf "$VOLUME_MOUNT/configs" /etc/provisioner

# Set proper ownership
chown -R provisioner:provisioner "$VOLUME_MOUNT"/{state,logs,templates,deployments}
chown -R root:root "$VOLUME_MOUNT/configs"

# Create provisioner group and user if they don't exist (they should from install script)
getent group provisioner >/dev/null || groupadd --system provisioner
getent passwd provisioner >/dev/null || useradd --system --gid provisioner --home-dir /var/lib/provisioner --shell /usr/sbin/nologin provisioner

log "Persistent data volume configured successfully"
log "  Mount point: $VOLUME_MOUNT"
log "  Device: $VOLUME_DEVICE"
log "  UUID: $VOLUME_UUID"
log "  Symlinks created for: /var/lib/provisioner, /var/log/provisioner, /etc/provisioner"
log "  SSH host keys: Persistent across server recreations"

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