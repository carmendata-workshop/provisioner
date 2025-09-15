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
    SERVICE_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/latest/download/provisioner.service"
else
    echo "🔍 Downloading version ${VERSION}..."
    DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/provisioner-${OS}-${ARCH}"
    SERVICE_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/provisioner.service"
fi

echo "⬇️  Downloading binary..."
if ! curl -fsSL "$DOWNLOAD_URL" -o provisioner; then
    echo "❌ Failed to download binary from: $DOWNLOAD_URL"
    echo "   Make sure the release exists and contains provisioner-${OS}-${ARCH}"
    exit 1
fi

echo "⬇️  Downloading service file..."
if ! curl -fsSL "$SERVICE_URL" -o provisioner.service; then
    echo "❌ Failed to download service file from: $SERVICE_URL"
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

# Install binary
echo "📋 Installing binary..."
if [ -f "bin/provisioner" ]; then
    cp bin/provisioner "$INSTALL_DIR/"
elif [ -f "provisioner" ]; then
    cp provisioner "$INSTALL_DIR/"
else
    echo "❌ Binary not found. Run 'make build' first."
    exit 1
fi
chmod +x "$INSTALL_DIR/provisioner"

# Create example environment
echo "📋 Creating example environment..."
mkdir -p "$CONFIG_DIR/environments/everest"

cat > "$CONFIG_DIR/environments/everest/config.json" << 'EOF'
{
  "name": "everest",
  "enabled": false,
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "description": "Example environment - weekdays 9am-6pm (disabled by default)"
}
EOF

cat > "$CONFIG_DIR/environments/everest/main.tf" << 'EOF'
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
  default     = "everest"
}

output "deployment_file" {
  value = local_file.environment_marker.filename
}
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
echo "⚙️  Installing systemd service..."
cp deployment/provisioner.service /etc/systemd/system/
systemctl daemon-reload

# Enable service (but don't start automatically)
echo "🔄 Enabling service..."
systemctl enable "$SERVICE_NAME"

echo "✅ Installation complete!"
echo ""
echo "📁 Binary: $INSTALL_DIR/provisioner"
echo "📝 Example environment created (disabled): $CONFIG_DIR/environments/everest/"
echo ""
echo "Next steps:"
echo "1. Review and configure environments in $CONFIG_DIR/environments/"
echo "2. Enable example environment: edit config.json and set 'enabled': true"
echo "3. Start the service: sudo systemctl start $SERVICE_NAME"
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