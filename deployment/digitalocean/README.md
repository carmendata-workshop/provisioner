# DigitalOcean Provisioner Deployment

This directory contains everything needed to deploy the OpenTofu Workspace Provisioner to DigitalOcean using Infrastructure as Code (IaC) with OpenTofu/Terraform and Ansible.

## Overview

The deployment consists of:
- **OpenTofu/Terraform**: Creates DigitalOcean infrastructure (droplet, firewall, DNS, volumes)
- **Ansible**: Configures the server and installs the provisioner service
- **Bootstrap Script**: Cloud-init script for initial server setup
- **Deployment Script**: Orchestrates the entire deployment process

## Quick Start

### 1. Prerequisites

Ensure you have the following tools installed:
- OpenTofu or Terraform
- Ansible
- jq
- SSH client

In the development container, these are automatically installed.

### 2. Configure Deployment

```bash
cd deployment/digitalocean

# Copy the example configuration
cp terraform.tfvars.example terraform.tfvars

# Edit the configuration with your values
vim terraform.tfvars
```

### 3. Set Environment Variables

```bash
# Set your DigitalOcean API token
export DIGITALOCEAN_TOKEN="dop_v1_your_token_here"
```

**SSH Keys**: The deployment will automatically use all SSH keys from your DigitalOcean account. Make sure you have at least one SSH key added to your DO account at: https://cloud.digitalocean.com/account/security

### 4. Deploy

```bash
# Deploy everything (infrastructure + configuration)
./deploy.sh deploy

# Or deploy with custom configuration
./deploy.sh deploy -c production.tfvars
```

### 5. Access Your Server

```bash
# SSH into the server
./deploy.sh ssh

# Check service status
./deploy.sh health

# View service logs
./deploy.sh logs
```

## Configuration

### Required Variables

Edit `terraform.tfvars` with your specific values:

```hcl
# DigitalOcean API token
digitalocean_token = "dop_v1_your_token_here"

# Server configuration
server_name    = "my-provisioner"
droplet_region = "lon1"
droplet_size   = "s-1vcpu-2gb"

# DNS configuration (required)
domain_name = "yourdomain.com"      # Must be managed by DigitalOcean DNS
subdomain   = "provisioner"         # Creates provisioner.yourdomain.com
```

**Important Notes:**
- **SSH keys** are automatically loaded from your DigitalOcean account
- **Domain name** is required and must be managed by DigitalOcean DNS
- The server will be accessible at `provisioner.yourdomain.com`

### Optional Configuration

```hcl
# Security
ssh_allowed_ips = ["203.0.113.0/24"]  # Restrict SSH access
enable_backups  = true                # Enable automated backups

# DNS TTL (optional)
dns_ttl = 300                         # DNS record TTL in seconds

# Persistent storage
create_data_volume = true
data_volume_size   = 20               # GB

# Monitoring
enable_monitoring = true
```

## Usage Examples

### Basic Deployment

```bash
# Plan deployment (dry run)
./deploy.sh plan

# Deploy with confirmation prompt
./deploy.sh deploy

# Deploy without confirmation
./deploy.sh deploy -y
```

### Advanced Deployment

```bash
# Deploy with custom configuration
./deploy.sh deploy -c production.tfvars

# Deploy only infrastructure (skip Ansible)
./deploy.sh deploy --skip-ansible

# Run only Ansible configuration (infrastructure exists)
./deploy.sh deploy --ansible-only

# Verbose output
./deploy.sh deploy -v
```

### Management Commands

```bash
# Show deployment status
./deploy.sh status

# SSH into server
./deploy.sh ssh

# View service logs in real-time
./deploy.sh logs

# Run health check
./deploy.sh health

# Show Terraform outputs
./deploy.sh output

# Destroy everything
./deploy.sh destroy
```

## Architecture

### Infrastructure Components

- **Droplet**: Ubuntu 22.04 LTS server running the provisioner
- **Firewall**: Restricts access to SSH (and optionally HTTP/HTTPS)
- **SSH Key**: Secure access to the server
- **Volume**: Optional persistent storage for data
- **DNS Record**: Required domain name with duplicate prevention

### Directory Structure

```
deployment/digitalocean/
├── main.tf                    # Main Terraform configuration
├── variables.tf               # Variable definitions
├── outputs.tf                 # Output values
├── bootstrap.sh               # Cloud-init bootstrap script
├── terraform.tfvars.example   # Example configuration
├── deploy.sh                  # Deployment orchestration script
├── ansible/
│   ├── playbook.yml          # Ansible configuration playbook
│   ├── inventory.yml         # Dynamic inventory template
│   └── ansible.cfg           # Ansible configuration
└── README.md                 # This file
```

### Security Features

- **Minimal Attack Surface**: Only SSH port open by default
- **Automatic Updates**: Unattended security updates enabled
- **Firewall**: UFW configured with strict rules
- **SSH Keys**: Uses all keys from your DigitalOcean account, password authentication disabled
- **Dedicated User**: Service runs as non-root user
- **File Permissions**: Strict permissions on configuration files

## Configuration Examples

### Development Environment

Minimal cost configuration for testing:

```hcl
server_name     = "provisioner-dev"
droplet_size    = "s-1vcpu-1gb"
droplet_region  = "lon1"
enable_backups  = false
create_data_volume = false
domain_name     = "yourdomain.com"
```

### Production Environment

Robust configuration for production use:

```hcl
server_name        = "provisioner-prod"
droplet_size       = "s-2vcpu-4gb"
droplet_region     = "lon1"
enable_backups     = true
enable_monitoring  = true
create_data_volume = true
data_volume_size   = 50
ssh_allowed_ips    = ["203.0.113.0/24"]  # Your office network
domain_name        = "yourdomain.com"
subdomain          = "provisioner"
```

### High Availability Environment

Enterprise configuration with enhanced features:

```hcl
server_name        = "provisioner-ha"
droplet_size       = "s-4vcpu-8gb"
droplet_region     = "lon1"
enable_backups     = true
enable_monitoring  = true
create_data_volume = true
data_volume_size   = 100
ssh_allowed_ips    = ["203.0.113.0/24", "198.51.100.0/24"]
enable_web_interface = true  # For future web UI
domain_name        = "yourdomain.com"
```

## Troubleshooting

### Common Issues

**Deployment fails with "DigitalOcean token not configured":**
```bash
export DIGITALOCEAN_TOKEN="your_token_here"
# Or set it in terraform.tfvars
```

**Deployment fails with domain validation error:**
```bash
# Make sure your domain is managed by DigitalOcean DNS
# Check at: https://cloud.digitalocean.com/networking/domains
# The domain must exist in your DO account before deployment
```

**Deployment fails with "DNS record already exists" error:**
```bash
# A provisioner instance may already be deployed at this subdomain
# Options:
# 1. Use a different subdomain: subdomain = "provisioner-dev"
# 2. Destroy existing deployment: ./deploy.sh destroy
# 3. Manually remove DNS record if old instance is gone
```

**SSH connection fails:**
```bash
# Check if the SSH key is configured correctly
ssh -i ~/.ssh/provisioner_key root@SERVER_IP

# Check firewall rules
./deploy.sh ssh
sudo ufw status
```

**Service not starting:**
```bash
# Check service status
./deploy.sh ssh
sudo systemctl status provisioner
sudo journalctl -u provisioner -f
```

**Terraform state issues:**
```bash
# Check state file
ls -la terraform.tfstate

# Refresh state
tofu refresh -var-file=terraform.tfvars
```

### Getting Help

1. **Check logs**: `./deploy.sh logs`
2. **Run health check**: `./deploy.sh health`
3. **Check service status**: `sudo systemctl status provisioner`
4. **View deployment status**: `./deploy.sh status`
5. **SSH for manual debugging**: `./deploy.sh ssh`

### Cleanup

To completely remove the deployment:

```bash
# Destroy all infrastructure
./deploy.sh destroy

# Clean up local files
rm -f terraform.tfstate*
rm -f .terraform.lock.hcl
```

## Cost Estimation

Approximate monthly costs (USD):

- **s-1vcpu-1gb**: $6/month (development)
- **s-1vcpu-2gb**: $12/month (small production)
- **s-2vcpu-4gb**: $24/month (production)
- **s-4vcpu-8gb**: $48/month (high availability)

Additional costs:
- **Backups**: +20% of droplet cost
- **Monitoring**: Free
- **Data Volume**: $0.10/GB/month
- **Bandwidth**: First 1TB free

## Next Steps

After deployment:

1. **Configure Workspaces**: Add your workspace configurations in `/etc/provisioner/workspaces/`
2. **Set up Templates**: Use `templatectl` to manage reusable templates
3. **Configure Jobs**: Add standalone jobs for maintenance tasks
4. **Monitor Logs**: Set up log monitoring and alerting
5. **Backup Strategy**: Configure regular backups of state and configuration
6. **Security Hardening**: Review and enhance security settings as needed

For more information, see the main project documentation at the repository root.