#!/bin/bash

# OpenTofu Environment Provisioner Installation Script
set -e

# Configuration
INSTALL_DIR="/opt/provisioner"
CONFIG_DIR="/etc/provisioner"
STATE_DIR="/var/lib/provisioner"
LOG_DIR="/var/log/provisioner"
SERVICE_NAME="provisioner"
USER_NAME="provisioner"
REPO_OWNER="carmendata-workshop"
REPO_NAME="provisioner"
VERSION="${1:-latest}"  # Allow version override as first argument

echo "🚀 Installing OpenTofu Environment Provisioner (${VERSION})..."

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "❌ This script must be run as root (use sudo)"
   exit 1
fi

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    armv7l) ARCH="arm" ;;
    *) echo "❌ Unsupported architecture: $ARCH"; exit 1 ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')

echo "📋 Detected platform: ${OS}/${ARCH}"

# Create temporary directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

cd "$TEMP_DIR"

# Download release
if [ "$VERSION" = "latest" ]; then
    echo "🔍 Finding latest release..."
    DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/latest/download/provisioner-${OS}-${ARCH}"
else
    echo "🔍 Downloading version ${VERSION}..."
    DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/provisioner-${OS}-${ARCH}"
fi

echo "⬇️  Downloading binary..."
if ! curl -fsSL "$DOWNLOAD_URL" -o provisioner; then
    echo "❌ Failed to download binary from: $DOWNLOAD_URL"
    echo "   Make sure the release exists and contains provisioner-${OS}-${ARCH}"
    exit 1
fi

# Create user if doesn't exist
if ! id "$USER_NAME" &>/dev/null; then
    echo "📝 Creating user: $USER_NAME"
    useradd --system --home-dir /var/lib/"$USER_NAME" --shell /bin/false "$USER_NAME"
fi

# Create directories following FHS standards
echo "📁 Creating directories..."
mkdir -p "$INSTALL_DIR"              # /opt/provisioner - binary
mkdir -p "$CONFIG_DIR/environments"  # /etc/provisioner/environments - configs
mkdir -p "$STATE_DIR"                # /var/lib/provisioner - state data
mkdir -p "$LOG_DIR"                  # /var/log/provisioner - log files

# Check if service is running and stop it for binary update
SERVICE_WAS_RUNNING=false
if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    echo "🛑 Stopping existing service for update..."
    if systemctl stop "$SERVICE_NAME"; then
        SERVICE_WAS_RUNNING=true
        echo "✅ Service stopped successfully"
    else
        echo "⚠️  Warning: Failed to stop service, but continuing with installation..."
    fi
fi

# Install binary
echo "📋 Installing binary..."
if [ -f "bin/provisioner" ]; then
    if cp bin/provisioner "$INSTALL_DIR/"; then
        echo "✅ Binary installed successfully"
    else
        echo "❌ Failed to copy binary from bin/provisioner"
        exit 1
    fi
elif [ -f "provisioner" ]; then
    if cp provisioner "$INSTALL_DIR/"; then
        echo "✅ Binary installed successfully"
    else
        echo "❌ Failed to copy binary from provisioner"
        exit 1
    fi
else
    echo "❌ Binary not found. Expected 'provisioner' or 'bin/provisioner'"
    exit 1
fi
chmod +x "$INSTALL_DIR/provisioner"

# Create symlink in /usr/local/bin for system-wide access
echo "🔗 Creating system-wide command access..."
if ln -sf "$INSTALL_DIR/provisioner" /usr/local/bin/provisioner; then
    echo "✅ Created symlink: /usr/local/bin/provisioner -> $INSTALL_DIR/provisioner"
else
    echo "⚠️  Warning: Failed to create symlink in /usr/local/bin"
    echo "   You can manually add $INSTALL_DIR to your PATH or create the symlink later"
fi

# Create example environment
echo "📋 Creating example environment..."
mkdir -p "$CONFIG_DIR/environments/example"

cat > "$CONFIG_DIR/environments/example/config.json" << 'EOF'
{{EXAMPLE_CONFIG_JSON}}
EOF

cat > "$CONFIG_DIR/environments/example/main.tf" << 'EOF'
{{EXAMPLE_MAIN_TF}}
EOF

# Set ownership and permissions
echo "🔐 Setting ownership and permissions..."
chown root:root "$INSTALL_DIR/provisioner"
chown -R "$USER_NAME:$USER_NAME" "$CONFIG_DIR"
chown -R "$USER_NAME:$USER_NAME" "$STATE_DIR"
chown -R "$USER_NAME:$USER_NAME" "$LOG_DIR"

# Set proper permissions
chmod 755 "$INSTALL_DIR/provisioner"
chmod 755 "$CONFIG_DIR"
chmod 750 "$STATE_DIR"
chmod 750 "$LOG_DIR"

# Install systemd service
echo "⚙️  Creating systemd service..."
cat > /etc/systemd/system/provisioner.service << 'EOF'
{{SYSTEMD_SERVICE}}
EOF
systemctl daemon-reload

# Enable and start service
echo "🔄 Enabling service..."
systemctl enable "$SERVICE_NAME"

if [ "$SERVICE_WAS_RUNNING" = true ]; then
    echo "🔄 Restarting service..."
    systemctl start "$SERVICE_NAME"
    echo "✅ Service updated and restarted"
else
    echo "🔄 Starting service..."
    systemctl start "$SERVICE_NAME"
    echo "✅ Service started"
fi

# Check service status
echo "📊 Service status:"
systemctl status "$SERVICE_NAME" --no-pager -l

echo ""
if [ "$SERVICE_WAS_RUNNING" = true ]; then
    echo "✅ Update complete! Service has been restarted with the new binary."
else
    echo "✅ Installation complete! Service has been started."
fi
echo ""
echo "📁 Binary: $INSTALL_DIR/provisioner"
echo "📝 Example environment created (disabled): $CONFIG_DIR/environments/example/"
echo ""
echo "Next steps:"
echo "1. Review and configure environments in $CONFIG_DIR/environments/"
echo "2. Enable example environment: edit config.json and set 'enabled': true"
echo "3. Restart service to pick up changes: sudo systemctl restart $SERVICE_NAME"
echo "4. Check status: sudo systemctl status $SERVICE_NAME"
echo "5. View logs: sudo journalctl -u $SERVICE_NAME -f"
echo ""
echo "Service management commands:"
echo "  sudo systemctl start $SERVICE_NAME"
echo "  sudo systemctl stop $SERVICE_NAME"
echo "  sudo systemctl restart $SERVICE_NAME"
echo "  sudo systemctl status $SERVICE_NAME"
echo ""
echo "🔧 File locations (FHS compliant):"
echo "  - Binary: $INSTALL_DIR/"
echo "  - Configuration: $CONFIG_DIR/"
echo "  - State data: $STATE_DIR/"
echo "  - Log files: $LOG_DIR/"
echo "  - System logs: journalctl -u $SERVICE_NAME"
echo ""
echo "💻 Command access:"
echo "  - System-wide: provisioner --help"
echo "  - Direct path: $INSTALL_DIR/provisioner --help"