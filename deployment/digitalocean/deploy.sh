#!/bin/bash
# DigitalOcean Provisioner Deployment Script
# This script orchestrates the deployment of the provisioner to DigitalOcean

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TERRAFORM_DIR="$SCRIPT_DIR"
ANSIBLE_DIR="$SCRIPT_DIR/ansible"
CONFIG_FILE="$SCRIPT_DIR/terraform.tfvars"
STATE_FILE="$SCRIPT_DIR/terraform.tfstate"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

info() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

# Show usage information
usage() {
    cat << EOF
Usage: $0 [COMMAND] [OPTIONS]

Commands:
    deploy              Deploy provisioner to DigitalOcean
    destroy             Destroy provisioner infrastructure
    plan                Show deployment plan without applying
    output              Show deployment outputs
    status              Show current deployment status
    ssh                 SSH into the provisioner server
    logs                Show provisioner service logs
    health              Run health check on the server
    help                Show this help message

Options:
    -c, --config FILE   Terraform variables file (default: terraform.tfvars)
    -s, --state FILE    Terraform state file (default: terraform.tfstate)
    -v, --verbose       Enable verbose output
    -y, --yes           Auto-approve actions without prompting
    --skip-ansible      Skip Ansible configuration (Terraform only)
    --ansible-only      Run only Ansible configuration (skip Terraform)

Examples:
    $0 deploy                               # Deploy with default config
    $0 deploy -c production.tfvars          # Deploy with custom config
    $0 plan                                 # Show deployment plan
    $0 ssh                                  # SSH into deployed server
    $0 logs                                 # View service logs
    $0 destroy -y                           # Destroy without confirmation

Environment Variables:
    DIGITALOCEAN_TOKEN                      # DigitalOcean API token
    TF_VAR_digitalocean_token              # Alternative for Terraform

Configuration:
    Copy terraform.tfvars.example to terraform.tfvars and customize.

EOF
}

# Activate Python virtual environment for Ansible
activate_python_env() {
    local venv_path="$HOME/.venv/provisioner"

    if [ ! -d "$venv_path" ]; then
        error "Python virtual environment not found at $venv_path"
        error "Please run the devcontainer setup script or activate the environment manually:"
        error "  source ~/.activate_provisioner"
        exit 1
    fi

    # Source the virtual environment
    source "$venv_path/bin/activate"
    info "Activated Python virtual environment: $venv_path"
}

# Check prerequisites
check_prerequisites() {
    local missing_tools=()

    # Check for required tools
    if ! command -v tofu >/dev/null 2>&1 && ! command -v terraform >/dev/null 2>&1; then
        missing_tools+=("tofu or terraform")
    fi

    if ! command -v jq >/dev/null 2>&1; then
        missing_tools+=("jq")
    fi

    if ! command -v ssh >/dev/null 2>&1; then
        missing_tools+=("ssh")
    fi

    # Check for Python virtual environment and Ansible
    activate_python_env

    if ! command -v ansible-playbook >/dev/null 2>&1; then
        missing_tools+=("ansible-playbook (in virtual environment)")
    fi

    if [ ${#missing_tools[@]} -ne 0 ]; then
        error "Missing required tools: ${missing_tools[*]}"
        error "Please install the missing tools and try again."
        if [[ " ${missing_tools[*]} " =~ " ansible-playbook " ]]; then
            error "For Ansible, try running: source ~/.activate_provisioner"
        fi
        exit 1
    fi

    # Determine which Terraform binary to use
    if command -v tofu >/dev/null 2>&1; then
        TERRAFORM_CMD="tofu"
    else
        TERRAFORM_CMD="terraform"
    fi

    info "Using Terraform command: $TERRAFORM_CMD"
    info "Using Ansible from virtual environment: $(which ansible-playbook)"
}

# Check configuration
check_config() {
    if [ ! -f "$CONFIG_FILE" ]; then
        error "Configuration file not found: $CONFIG_FILE"
        error "Copy terraform.tfvars.example to terraform.tfvars and customize it."
        exit 1
    fi

    # Check for DigitalOcean token
    local token=""
    if [ -n "${DIGITALOCEAN_TOKEN:-}" ]; then
        token="$DIGITALOCEAN_TOKEN"
    elif [ -n "${TF_VAR_digitalocean_token:-}" ]; then
        token="$TF_VAR_digitalocean_token"
    elif grep -q "^digitalocean_token" "$CONFIG_FILE"; then
        token=$(grep "^digitalocean_token" "$CONFIG_FILE" | cut -d'"' -f2)
    fi

    if [ -z "$token" ] || [ "$token" = "dop_v1_your_digitalocean_token_here" ]; then
        error "DigitalOcean token not configured."
        error "Set DIGITALOCEAN_TOKEN environment variable or configure in $CONFIG_FILE"
        exit 1
    fi

    # Check if user has SSH keys in DigitalOcean account
    info "Note: All SSH keys from your DigitalOcean account will be added to the droplet"
    info "Make sure you have at least one SSH key in your DO account at:"
    info "https://cloud.digitalocean.com/account/security"

    # Extract domain and subdomain for validation messaging
    local domain_name=""
    local subdomain="provisioner"
    if grep -q "^domain_name" "$CONFIG_FILE"; then
        domain_name=$(grep "^domain_name" "$CONFIG_FILE" | cut -d'"' -f2)
    fi
    if grep -q "^subdomain" "$CONFIG_FILE"; then
        subdomain=$(grep "^subdomain" "$CONFIG_FILE" | cut -d'"' -f2)
    fi

    if [ -n "$domain_name" ] && [ "$domain_name" != "example.com" ]; then
        info "DNS: Will create/validate record for ${subdomain}.${domain_name}"
        info "Make sure this subdomain is not already in use"
    else
        warn "Domain name not configured or still set to example.com"
        warn "Make sure to set a valid domain_name in $CONFIG_FILE"
    fi

    log "Configuration validated: $CONFIG_FILE"
}

# Initialize Terraform
terraform_init() {
    log "Initializing Terraform..."
    cd "$TERRAFORM_DIR"
    $TERRAFORM_CMD init
    cd - > /dev/null
}

# Plan deployment
terraform_plan() {
    log "Planning deployment..."
    cd "$TERRAFORM_DIR"
    $TERRAFORM_CMD plan -var-file="$CONFIG_FILE" -state="$STATE_FILE"
    cd - > /dev/null
}

# Check for deployment conflicts before applying
check_deployment_conflicts() {
    # Check if this is a new deployment by looking for existing state
    if [ ! -f "$STATE_FILE" ]; then
        # This is a new deployment, check for DNS conflicts
        local domain_name=""
        local subdomain="provisioner"

        if grep -q "^domain_name" "$CONFIG_FILE"; then
            domain_name=$(grep "^domain_name" "$CONFIG_FILE" | cut -d'"' -f2)
        fi
        if grep -q "^subdomain" "$CONFIG_FILE"; then
            subdomain=$(grep "^subdomain" "$CONFIG_FILE" | cut -d'"' -f2)
        fi

        if [ -n "$domain_name" ] && [ "$domain_name" != "example.com" ]; then
            info "Checking for existing DNS records for new deployment..."

            # Use doctl to check for existing DNS records
            if command -v doctl >/dev/null 2>&1 && [ -n "${DIGITALOCEAN_TOKEN:-}" ]; then
                local existing_records
                existing_records=$(doctl compute domain records list "$domain_name" --format Name,Type,Data --no-header 2>/dev/null | grep "^$subdomain" | head -5)

                if [ -n "$existing_records" ]; then
                    error "DNS record '${subdomain}.${domain_name}' already exists!"
                    error "Found existing record(s):"
                    echo "$existing_records" | while read line; do
                        error "  $line"
                    done
                    error ""
                    error "This suggests a provisioner instance may already be deployed."
                    error "Please either:"
                    error "  1. Use a different subdomain (set subdomain variable)"
                    error "  2. Remove the existing DNS record if the old instance is no longer needed"
                    error "  3. Destroy the existing deployment first"
                    exit 1
                fi
            else
                warn "Cannot check for DNS conflicts (doctl not available or DIGITALOCEAN_TOKEN not set)"
                warn "Proceeding with deployment - Terraform will detect any conflicts"
            fi
        fi
    else
        info "Updating existing deployment (state file found)"
    fi
}

# Apply deployment
terraform_apply() {
    local auto_approve=""
    if [ "${AUTO_APPROVE:-false}" = "true" ]; then
        auto_approve="-auto-approve"
    fi

    # Check for conflicts before applying (only for new deployments)
    check_deployment_conflicts

    log "Applying deployment..."
    cd "$TERRAFORM_DIR"
    $TERRAFORM_CMD apply $auto_approve -var-file="$CONFIG_FILE" -state="$STATE_FILE"
    cd - > /dev/null
}

# Destroy deployment
terraform_destroy() {
    local auto_approve=""
    if [ "${AUTO_APPROVE:-false}" = "true" ]; then
        auto_approve="-auto-approve"
    fi

    warn "This will destroy all provisioner infrastructure in DigitalOcean!"
    if [ "${AUTO_APPROVE:-false}" != "true" ]; then
        read -p "Are you sure you want to continue? (yes/no): " -r
        if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
            log "Destroy cancelled."
            exit 0
        fi
    fi

    log "Destroying deployment..."
    cd "$TERRAFORM_DIR"
    $TERRAFORM_CMD destroy $auto_approve -var-file="$CONFIG_FILE" -state="$STATE_FILE"
    cd - > /dev/null
}

# Get Terraform outputs
get_terraform_outputs() {
    if [ ! -f "$STATE_FILE" ]; then
        error "No deployment state found. Run 'deploy' first."
        exit 1
    fi

    cd "$TERRAFORM_DIR"
    $TERRAFORM_CMD output -json -state="$STATE_FILE"
    cd - > /dev/null
}

# Get server IP from Terraform output
get_server_ip() {
    local outputs
    outputs=$(get_terraform_outputs)
    echo "$outputs" | jq -r '.public_ip.value // empty'
}

# Configure Ansible inventory
configure_ansible() {
    local server_ip="$1"
    local outputs="$2"

    log "Configuring Ansible inventory..."

    # Create dynamic inventory file
    cat > "$ANSIBLE_DIR/inventory.yml" << EOF
all:
  children:
    provisioner_servers:
      hosts:
        provisioner-server:
          ansible_host: "$server_ip"
          ansible_user: root
          # Persistent SSH host keys - no need for StrictHostKeyChecking=no

          # Configuration from Terraform outputs
          server_name: $(echo "$outputs" | jq -r '.droplet_name.value // "provisioner-server"')
          server_region: $(echo "$outputs" | jq -r '.droplet_region.value // "unknown"')
          provisioner_version: "latest"
          github_repo: "carmendata-workshop/provisioner"
          auto_start_service: true
          server_timezone: "UTC"

      vars:
        ansible_python_interpreter: /usr/bin/python3
        ansible_ssh_pipelining: true
        provisioner_install_dir: "/opt/provisioner"
        provisioner_config_dir: "/etc/provisioner"
        provisioner_state_dir: "/var/lib/provisioner"
        provisioner_log_dir: "/var/log/provisioner"
        provisioner_user: "provisioner"
        provisioner_service: "provisioner"
EOF

    log "Ansible inventory configured"
}

# Run Ansible playbook
run_ansible() {
    log "Running Ansible playbook..."

    # Ensure virtual environment is activated
    if [ -z "$VIRTUAL_ENV" ]; then
        info "Virtual environment not active, activating..."
        activate_python_env
    fi

    cd "$ANSIBLE_DIR"

    # Wait for SSH to be available
    local server_ip="$1"
    log "Waiting for SSH to be available on $server_ip..."
    local max_attempts=30
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if ssh -o ConnectTimeout=5 root@"$server_ip" "echo 'SSH Ready'" >/dev/null 2>&1; then
            log "SSH connection established"
            break
        fi

        info "Attempt $attempt/$max_attempts: SSH not ready, waiting..."
        sleep 10
        ((attempt++))
    done

    if [ $attempt -gt $max_attempts ]; then
        error "Failed to establish SSH connection after $max_attempts attempts"
        exit 1
    fi

    # Run the playbook with virtual environment
    info "Running ansible-playbook from: $(which ansible-playbook)"
    ansible-playbook -i inventory.yml playbook.yml

    cd - > /dev/null
}

# SSH into server
ssh_to_server() {
    local server_ip
    server_ip=$(get_server_ip)

    if [ -z "$server_ip" ]; then
        error "Could not determine server IP. Is the deployment active?"
        exit 1
    fi

    log "Connecting to server: $server_ip"
    ssh root@"$server_ip"
}

# Show service logs
show_logs() {
    local server_ip
    server_ip=$(get_server_ip)

    if [ -z "$server_ip" ]; then
        error "Could not determine server IP. Is the deployment active?"
        exit 1
    fi

    log "Showing provisioner logs from: $server_ip"
    ssh root@"$server_ip" "journalctl -u provisioner -f"
}

# Run health check
health_check() {
    local server_ip
    server_ip=$(get_server_ip)

    if [ -z "$server_ip" ]; then
        error "Could not determine server IP. Is the deployment active?"
        exit 1
    fi

    log "Running health check on: $server_ip"
    ssh root@"$server_ip" "provisioner-health"
}

# Show deployment status
show_status() {
    if [ ! -f "$STATE_FILE" ]; then
        warn "No deployment state found."
        return
    fi

    local outputs
    outputs=$(get_terraform_outputs)

    echo "=== Deployment Status ==="
    echo "State File: $STATE_FILE"
    echo "Config File: $CONFIG_FILE"
    echo

    if [ -n "$outputs" ]; then
        echo "Server Details:"
        echo "  Name: $(echo "$outputs" | jq -r '.droplet_name.value // "Unknown"')"
        echo "  IP: $(echo "$outputs" | jq -r '.public_ip.value // "Unknown"')"
        echo "  Region: $(echo "$outputs" | jq -r '.droplet_region.value // "Unknown"')"
        echo

        echo "Connection:"
        echo "  SSH: $(echo "$outputs" | jq -r '.ssh_command.value // "N/A"')"
        echo

        echo "Service Commands:"
        echo "  Status: $(echo "$outputs" | jq -r '.service_status_command.value // "N/A"')"
        echo "  Logs: $(echo "$outputs" | jq -r '.service_logs_command.value // "N/A"')"
    else
        warn "Could not retrieve deployment outputs"
    fi
}

# Main deployment function
deploy() {
    log "Starting DigitalOcean provisioner deployment..."

    check_prerequisites
    check_config

    if [ "${ANSIBLE_ONLY:-false}" != "true" ]; then
        terraform_init
        terraform_apply

        # Get outputs for Ansible
        local outputs
        outputs=$(get_terraform_outputs)
        local server_ip
        server_ip=$(echo "$outputs" | jq -r '.public_ip.value')

        if [ -z "$server_ip" ] || [ "$server_ip" = "null" ]; then
            error "Could not determine server IP from Terraform outputs"
            exit 1
        fi

        log "Server deployed successfully: $server_ip"
    fi

    if [ "${SKIP_ANSIBLE:-false}" != "true" ]; then
        # Configure and run Ansible
        local outputs
        outputs=$(get_terraform_outputs)
        local server_ip
        server_ip=$(echo "$outputs" | jq -r '.public_ip.value')

        configure_ansible "$server_ip" "$outputs"
        run_ansible "$server_ip"

        log "Provisioner configuration completed successfully!"
    fi

    echo
    echo "=== Deployment Complete ==="
    show_status
    echo
    log "Use '$0 ssh' to connect to the server"
    log "Use '$0 logs' to view service logs"
    log "Use '$0 health' to run a health check"
}

# Parse command line arguments
parse_args() {
    COMMAND=""
    AUTO_APPROVE=false
    VERBOSE=false
    SKIP_ANSIBLE=false
    ANSIBLE_ONLY=false

    while [[ $# -gt 0 ]]; do
        case $1 in
            -c|--config)
                CONFIG_FILE="$2"
                shift 2
                ;;
            -s|--state)
                STATE_FILE="$2"
                shift 2
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -y|--yes)
                AUTO_APPROVE=true
                shift
                ;;
            --skip-ansible)
                SKIP_ANSIBLE=true
                shift
                ;;
            --ansible-only)
                ANSIBLE_ONLY=true
                shift
                ;;
            deploy|destroy|plan|output|status|ssh|logs|health|help)
                COMMAND="$1"
                shift
                ;;
            *)
                error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done

    if [ -z "$COMMAND" ]; then
        COMMAND="help"
    fi

    # Enable verbose output if requested
    if [ "$VERBOSE" = "true" ]; then
        set -x
    fi
}

# Main function
main() {
    parse_args "$@"

    case "$COMMAND" in
        deploy)
            deploy
            ;;
        destroy)
            check_prerequisites
            terraform_destroy
            ;;
        plan)
            check_prerequisites
            check_config
            terraform_init
            terraform_plan
            ;;
        output)
            get_terraform_outputs | jq .
            ;;
        status)
            show_status
            ;;
        ssh)
            ssh_to_server
            ;;
        logs)
            show_logs
            ;;
        health)
            health_check
            ;;
        help)
            usage
            ;;
        *)
            error "Unknown command: $COMMAND"
            usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"