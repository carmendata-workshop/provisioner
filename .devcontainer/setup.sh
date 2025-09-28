#!/bin/bash
# .devcontainer/setup.sh
# Development environment setup script for provisioner project

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[SETUP]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   error "This script should not be run as root"
   exit 1
fi

log "Starting provisioner development environment setup..."

# Install OpenTofu
install_opentofu() {
    local version="1.10.6"
    local arch="amd64"

    # Detect architecture
    case $(uname -m) in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        arm*) arch="arm" ;;
        *) warn "Unsupported architecture $(uname -m), defaulting to amd64" ;;
    esac

    log "Installing OpenTofu ${version} for ${arch}..."

    # Download and install OpenTofu
    local download_url="https://github.com/opentofu/opentofu/releases/download/v${version}/tofu_${version}_linux_${arch}.tar.gz"
    local temp_dir=$(mktemp -d)

    cd "$temp_dir"
    curl -fsSL "$download_url" -o tofu.tar.gz
    tar -xzf tofu.tar.gz

    # Install to user's local bin (if exists) or create it
    local install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"
    mv tofu "$install_dir/"
    chmod +x "$install_dir/tofu"

    # Clean up
    cd - > /dev/null
    rm -rf "$temp_dir"

    # Check if ~/.local/bin is in PATH
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
        warn "~/.local/bin is not in PATH. Adding to ~/.bashrc"
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
        export PATH="$HOME/.local/bin:$PATH"
    fi

    # Verify installation
    if command -v tofu >/dev/null 2>&1; then
        local installed_version=$(tofu version | head -n1 | awk '{print $2}')
        log "OpenTofu ${installed_version} installed successfully"
    else
        error "OpenTofu installation failed"
        exit 1
    fi
}

# Install Ansible
install_ansible() {
    log "Installing Ansible..."

    # Create virtual environment for project dependencies
    # Note: Python should already be installed via devcontainer features
    local venv_dir="$HOME/.venv/provisioner"
    if [[ ! -d "$venv_dir" ]]; then
        log "Creating Python virtual environment at $venv_dir"
        python3 -m venv "$venv_dir"
    fi

    # Activate virtual environment and install requirements
    source "$venv_dir/bin/activate"

    # Upgrade pip
    pip install --upgrade pip

    # Install from requirements.txt if it exists
    if [[ -f "/workspaces/provisioner/requirements.txt" ]]; then
        pip install -r /workspaces/provisioner/requirements.txt
        log "Installed Python packages from requirements.txt"
    else
        # Install basic Ansible
        pip install ansible>=6.0.0 jinja2>=3.1.0 pyyaml>=6.0
        log "Installed basic Ansible packages"
    fi

    # Create activation script for easy access
    cat > ~/.activate_provisioner << 'EOF'
#!/bin/bash
# Activate provisioner development environment
source ~/.venv/provisioner/bin/activate
echo "provisioner development environment activated"
echo "OpenTofu version: $(tofu version | head -n1)"
echo "Ansible version: $(ansible --version | head -n1)"
EOF
    chmod +x ~/.activate_provisioner

    log "Created activation script at ~/.activate_provisioner"
}

# Install additional tools
install_tools() {
    log "Installing additional development tools..."

    # Update package list first
    sudo apt-get update

    # Install common tools
    sudo apt-get install -y \
        curl \
        wget \
        git \
        jq \
        tree \
        vim \
        htop \
        net-tools \
        dnsutils \
        nmap \
        telnet \
        netcat-openbsd \
        iputils-ping

    # Install doctl (DigitalOcean CLI)
    install_doctl

    # Install pnpm for Node.js package management
    install_pnpm
}

install_doctl() {
    log "Installing doctl (DigitalOcean CLI)..."

    local version="1.104.0"
    local arch="amd64"

    case $(uname -m) in
        aarch64|arm64) arch="arm64" ;;
    esac

    local download_url="https://github.com/digitalocean/doctl/releases/download/v${version}/doctl-${version}-linux-${arch}.tar.gz"
    local temp_dir=$(mktemp -d)

    cd "$temp_dir"
    curl -fsSL "$download_url" -o doctl.tar.gz
    tar -xzf doctl.tar.gz

    mv doctl "$HOME/.local/bin/"
    chmod +x "$HOME/.local/bin/doctl"

    cd - > /dev/null
    rm -rf "$temp_dir"

    log "doctl installed successfully"
}

install_pnpm() {
    log "Installing pnpm..."

    # Node.js should already be installed via devcontainer features
    if ! command -v node >/dev/null 2>&1; then
        error "Node.js not found - should be installed via devcontainer features"
        return 1
    fi

    # Install pnpm using npm
    npm install -g pnpm

    log "pnpm installed successfully"
}

# Setup shell aliases and functions
setup_shell() {
    log "Setting up shell aliases and functions..."

    # Add helpful aliases to bashrc
    cat >> ~/.bashrc << 'EOF'

# provisioner Project Aliases
alias tofu='tofu'
alias tf='tofu'
alias provisioner-activate='source ~/.activate_provisioner'
alias provisioner='cd /workspaces/provisioner'
alias provisioner-plan='cd /workspaces/provisioner && tofu plan'
alias provisioner-apply='cd /workspaces/provisioner && tofu apply'
alias provisioner-destroy='cd /workspaces/provisioner && tofu destroy'
alias provisioner-status='cd /workspaces/provisioner && tofu show'

# OpenTofu aliases
alias tfi='tofu init'
alias tfp='tofu plan'
alias tfa='tofu apply'
alias tfd='tofu destroy'
alias tfv='tofu validate'
alias tff='tofu fmt'
alias tfo='tofu output'

# Ansible aliases
alias ap='ansible-playbook'
alias av='ansible-vault'
alias ai='ansible-inventory'

EOF

    log "Shell aliases added to ~/.bashrc"
}

# Create development configuration
create_dev_config() {
    log "Creating development configuration files..."

    # Create .env template if it doesn't exist
    if [[ ! -f "/workspaces/provisioner/.env" && -f "/workspaces/provisioner/.env.example" ]]; then
        cp /workspaces/provisioner/.env.example /workspaces/provisioner/.env
        warn "Created .env file from .env.example - please update with your credentials"
    fi

    # Create ansible vault password file location (project-specific)
    PROJECT_NAME=$(basename "$PWD")
    VAULT_PASSWORD_FILE="$HOME/.ansible_vault_$PROJECT_NAME"
    touch "$VAULT_PASSWORD_FILE"
    chmod 600 "$VAULT_PASSWORD_FILE"
    warn "Created $VAULT_PASSWORD_FILE - please add your Ansible vault password"

    log "Development configuration setup complete"
}

# Verify installation
verify_installation() {
    log "Verifying installation..."

    local errors=0

    # Check OpenTofu
    if command -v tofu >/dev/null 2>&1; then
        info "✓ OpenTofu: $(tofu version | head -n1)"
    else
        error "✗ OpenTofu not found"
        ((errors++))
    fi

    # Check Ansible (in virtual environment)
    source ~/.venv/provisioner/bin/activate 2>/dev/null
    if command -v ansible >/dev/null 2>&1; then
        info "✓ Ansible: $(ansible --version | head -n1)"
    else
        error "✗ Ansible not found"
        ((errors++))
    fi

    # Check doctl
    if command -v doctl >/dev/null 2>&1; then
        info "✓ doctl: $(doctl version | head -n1)"
    else
        error "✗ doctl not found"
        ((errors++))
    fi

    # Check pnpm
    if command -v pnpm >/dev/null 2>&1; then
        info "✓ pnpm: $(pnpm --version)"
    else
        error "✗ pnpm not found"
        ((errors++))
    fi

    # Check netcat
    if command -v nc >/dev/null 2>&1; then
        info "✓ netcat: Available"
    else
        warn "? netcat not found (telnet available as alternative)"
    fi

    # Check core testing tools
    local testing_tools=("dig" "ssh" "telnet" "ss")
    for tool in "${testing_tools[@]}"; do
        if command -v "$tool" >/dev/null 2>&1; then
            info "✓ $tool: Available"
        else
            error "✗ $tool not found (required for testing)"
            ((errors++))
        fi
    done

    if [[ $errors -eq 0 ]]; then
        log "All tools installed successfully!"
        info ""
        info "To get started:"
        info "1. Run 'source ~/.bashrc' to load new aliases"
        info "2. Run 'provisioner-activate' to activate the Python environment"
        info "3. Set environment variables (DIGITALOCEAN_TOKEN, TF_VAR_environment, etc.)"
        info "4. Add your Ansible vault password to $VAULT_PASSWORD_FILE"
        info ""
        info "Useful aliases:"
        info "  provisioner         - Go to project directory"
        info "  provisioner-plan    - Plan infrastructure changes"
        info "  provisioner-apply   - Apply infrastructure changes"
        info "  tf/tofu     - OpenTofu commands"
        info ""
    else
        error "Installation completed with $errors errors"
        exit 1
    fi
}

# Main execution
main() {
    info "Development Environment Setup"
    info "===================================="

    install_opentofu
    install_ansible
    install_tools
    setup_shell
    create_dev_config
    verify_installation

    log "Setup completed successfully!"
}

# Run main function
main "$@"