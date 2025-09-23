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

# Download all binaries
BINARIES="provisioner environmentctl templatectl"
if [ "$VERSION" = "latest" ]; then
    echo "🔍 Finding latest release..."
    BASE_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/latest/download"
else
    echo "🔍 Downloading version ${VERSION}..."
    BASE_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}"
fi

echo "⬇️  Downloading binaries..."
for binary in $BINARIES; do
    DOWNLOAD_URL="${BASE_URL}/${binary}-${OS}-${ARCH}"
    echo "  Downloading ${binary}..."
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$binary"; then
        echo "❌ Failed to download ${binary} from: $DOWNLOAD_URL"
        echo "   Make sure the release exists and contains ${binary}-${OS}-${ARCH}"
        exit 1
    fi
done

# Create user if doesn't exist
if ! id "$USER_NAME" &>/dev/null; then
    echo "📝 Creating user: $USER_NAME"
    useradd --system --home-dir /var/lib/"$USER_NAME" --shell /bin/false "$USER_NAME"
fi

# Check for existing installation
if [ ! -f "$INSTALL_DIR/provisioner" ] && [ ! -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
    echo "🆕 Detected fresh installation..."
else
    echo "🔄 Detected existing installation - performing update..."
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

# Install binaries
echo "📋 Installing binaries..."
for binary in $BINARIES; do
    if [ -f "$binary" ]; then
        if cp "$binary" "$INSTALL_DIR/"; then
            echo "✅ ${binary} installed successfully"
        else
            echo "❌ Failed to copy ${binary}"
            exit 1
        fi
        chmod +x "$INSTALL_DIR/$binary"
    else
        echo "❌ Binary not found: $binary"
        exit 1
    fi
done

# Create symlinks in /usr/local/bin for system-wide access
echo "🔗 Creating system-wide command access..."
for binary in $BINARIES; do
    if ln -sf "$INSTALL_DIR/$binary" "/usr/local/bin/$binary"; then
        echo "✅ Created symlink: /usr/local/bin/$binary -> $INSTALL_DIR/$binary"
    else
        echo "⚠️  Warning: Failed to create symlink for $binary in /usr/local/bin"
    fi
done

# Install example environments if none exist
ENV_COUNT=$(find "$CONFIG_DIR/environments" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)

if [ "$ENV_COUNT" -eq 0 ]; then
    echo "📋 Installing example environments (no environments found)..."

    # Extract embedded examples archive
    EXAMPLES_ARCHIVE=$(mktemp)
    cat << 'EOF' | base64 -d > "$EXAMPLES_ARCHIVE"
{{EXAMPLES_BASE64}}
EOF

    if tar -xzf "$EXAMPLES_ARCHIVE" -C "$TEMP_DIR" 2>/dev/null; then
        if [ -d "$TEMP_DIR/environments" ]; then
            echo "📦 Installing example environments..."
            cp -r "$TEMP_DIR/environments"/* "$CONFIG_DIR/environments/"
            echo "✅ Example environments installed:"
            ls -1 "$CONFIG_DIR/environments/" | sed 's/^/  - /'
        else
            echo "⚠️  Malformed examples archive, creating basic example..."
            mkdir -p "$CONFIG_DIR/environments/simple-example"
            cat > "$CONFIG_DIR/environments/simple-example/config.json" << 'EOF'
{{EXAMPLE_CONFIG_JSON}}
EOF
            cat > "$CONFIG_DIR/environments/simple-example/main.tf" << 'EOF'
{{EXAMPLE_MAIN_TF}}
EOF
        fi
    else
        echo "⚠️  Failed to extract examples archive, creating basic example..."
        mkdir -p "$CONFIG_DIR/environments/simple-example"
        cat > "$CONFIG_DIR/environments/simple-example/config.json" << 'EOF'
{{EXAMPLE_CONFIG_JSON}}
EOF
        cat > "$CONFIG_DIR/environments/simple-example/main.tf" << 'EOF'
{{EXAMPLE_MAIN_TF}}
EOF
    fi

    rm -f "$EXAMPLES_ARCHIVE"
else
    echo "📋 Skipping example environments (environments already exist)..."
fi

# Install example templates using templatectl
TEMPLATE_COUNT=$(find "$STATE_DIR/templates" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)

if [ "$TEMPLATE_COUNT" -eq 0 ]; then
    echo "📋 Installing example templates (no templates found)..."

    # Extract embedded templates archive
    TEMPLATES_ARCHIVE=$(mktemp)
    cat << 'EOF' | base64 -d > "$TEMPLATES_ARCHIVE"
{{TEMPLATES_BASE64}}
EOF

    if tar -xzf "$TEMPLATES_ARCHIVE" -C "$TEMP_DIR" 2>/dev/null; then
        if [ -d "$TEMP_DIR/templates" ]; then
            echo "📦 Installing example templates..."

            # Install each template using templatectl
            for template_dir in "$TEMP_DIR/templates"/*; do
                if [ -d "$template_dir" ]; then
                    template_name=$(basename "$template_dir")
                    echo "  Installing template: $template_name"

                    # Use templatectl to add the template from file path
                    if "$INSTALL_DIR/templatectl" add "$template_name" "file://$template_dir" 2>/dev/null; then
                        echo "✅ Template '$template_name' installed successfully"
                    else
                        echo "⚠️  Warning: Failed to install template '$template_name'"
                    fi
                fi
            done

            echo "✅ Example templates installation complete"
        else
            echo "⚠️  Templates archive missing templates directory"
        fi
    else
        echo "⚠️  Failed to extract templates archive"
    fi

    rm -f "$TEMPLATES_ARCHIVE"
else
    echo "📋 Skipping example templates (templates already exist)..."
fi

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
    echo "✅ Update complete! Service has been restarted with the new binaries."
else
    echo "✅ Installation complete! Service has been started."
fi
echo ""
echo "📁 Binaries: $INSTALL_DIR/"

# Check if we just installed examples
EXAMPLE_ENVS=$(find "$CONFIG_DIR/environments" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
EXAMPLE_TEMPLATES=$(find "$STATE_DIR/templates" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)

if [ "$EXAMPLE_ENVS" -gt 0 ] || [ "$EXAMPLE_TEMPLATES" -gt 0 ]; then
    echo "📝 Examples installed:"
    [ "$EXAMPLE_ENVS" -gt 0 ] && echo "  - Environments: $CONFIG_DIR/environments/"
    [ "$EXAMPLE_TEMPLATES" -gt 0 ] && echo "  - Templates: available via templatectl"
    echo ""
    echo "Next steps:"
    echo "1. Review examples: environmentctl list && templatectl list"
    echo "2. Enable environments by editing their config.json files"
    echo "3. Create your own environments and templates"
    echo "4. Restart service to pick up changes: sudo systemctl restart $SERVICE_NAME"
    echo "5. View logs: sudo journalctl -u $SERVICE_NAME -f"
else
    echo ""
    echo "Next steps:"
    echo "1. Create environments and templates"
    echo "2. View service logs: sudo journalctl -u $SERVICE_NAME -f"
    echo "3. Check service status: sudo systemctl status $SERVICE_NAME"
fi
echo ""
echo "Service management commands:"
echo "  sudo systemctl start $SERVICE_NAME"
echo "  sudo systemctl stop $SERVICE_NAME"
echo "  sudo systemctl restart $SERVICE_NAME"
echo "  sudo systemctl status $SERVICE_NAME"
echo ""
echo "🔧 File locations (FHS compliant):"
echo "  - Binaries: $INSTALL_DIR/"
echo "  - Configuration: $CONFIG_DIR/"
echo "  - State data: $STATE_DIR/"
echo "  - Log files: $LOG_DIR/"
echo "  - System logs: journalctl -u $SERVICE_NAME"
echo ""
echo "💻 Available commands:"
echo "  - provisioner --help        # Scheduler daemon"
echo "  - environmentctl --help     # Environment management"
echo "  - templatectl --help        # Template management"
echo ""
echo "📖 Quick examples:"
echo "  environmentctl list                    # List environments"
echo "  environmentctl deploy my-app          # Deploy immediately"
echo "  templatectl list                      # List templates"