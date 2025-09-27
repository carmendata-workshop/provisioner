# Template Management Guide

The OpenTofu Workspace Scheduler includes a comprehensive template management system for sharing and versioning OpenTofu templates across multiple workspaces.

## Overview

Templates allow you to:
- **Share** common infrastructure patterns across multiple workspaces
- **Version control** template changes with Git references
- **Centralize** template storage and management
- **Update** multiple workspaces simultaneously when templates change
- **Customize** templates with workspace-specific variables

## Template Commands

### Add Template

**From GitHub Repository:**
```bash
# Add template from GitHub repository
templatectl add web-app https://github.com/org/terraform-templates --path workspaces/web-app --ref v2.1.0

# Add with description
templatectl add database https://github.com/company/infra-templates --path db/postgres --ref main --description "PostgreSQL database template"
```

**Parameters:**
- `name` - Unique template identifier
- `url` - Git repository URL (GitHub, GitLab, etc.)
- `--path` - Path within repository containing the template files
- `--ref` - Git reference (tag, branch, commit hash)
- `--description` - Optional human-readable description

### List Templates

```bash
templatectl list                    # Basic list
templatectl list --detailed         # Detailed information
```

**Output Example:**
```
NAME         SOURCE                                   REF    DESCRIPTION
web-app      https://github.com/company/infra-templates  v1.2.0  Standard web application template
postgres-db  https://github.com/company/infra-templates  main    PostgreSQL database template
monitoring   https://github.com/ops/monitoring-templates v3.1.0  Monitoring stack template
```

### Show Template Details

```bash
templatectl show web-app
```

**Output Example:**
```
Template: web-app
Source: https://github.com/company/infra-templates
Path: templates/web-app
Reference: v2.1.0
Description: Standard web application template
Created: 2025-01-15 10:30:00
Updated: 2025-01-15 10:30:00
Content Hash: abc123def456...
Files:
  - main.tf
  - variables.tf
  - outputs.tf
```

### Update Templates

```bash
templatectl update web-app          # Update specific template
templatectl update --all            # Update all templates
```

**Update Behavior:**
- Fetches latest content from Git repository
- Compares content hash to detect real changes
- Updates template metadata and content
- Logs changes for workspace impact tracking

### Validate Templates

```bash
templatectl validate web-app        # Validate specific template
templatectl validate --all          # Validate all templates
```

**Validation Checks:**
- OpenTofu syntax validation
- Required files presence
- Variable definitions
- Output definitions
- Template completeness

### Remove Templates

```bash
templatectl remove web-app          # Interactive confirmation
templatectl remove web-app --force  # Skip confirmation
```

**Safety Features:**
- Interactive confirmation by default
- Lists workspaces using the template before removal
- Warns about impact on existing deployments
- Provides `--force` flag for automation

## Template Storage Structure

Templates are stored in `/var/lib/provisioner/templates/`:

```
/var/lib/provisioner/templates/
├── registry.json                 # Template metadata registry
├── web-app-v2/                  # Template content
│   ├── main.tf
│   ├── variables.tf
│   └── outputs.tf
├── database/                    # Another template
│   ├── main.tf
│   └── variables.tf
└── monitoring/                  # Monitoring template
    ├── main.tf
    ├── variables.tf
    ├── outputs.tf
    └── modules/
        └── alerts/
            └── main.tf
```

## Template Registry Format

The template registry (`registry.json`) tracks metadata:

```json
{
  "templates": {
    "web-app-v2": {
      "name": "web-app-v2",
      "source_url": "https://github.com/org/terraform-templates",
      "source_path": "workspaces/web-app",
      "source_ref": "v2.1.0",
      "created_at": "2025-01-15T10:30:00Z",
      "updated_at": "2025-01-15T10:30:00Z",
      "content_hash": "abc123...",
      "description": "Modern web application template",
      "version": "v2.1.0"
    }
  }
}
```

## Using Templates in Workspaces

### Template Reference in config.json

```json
{
  "enabled": true,
  "template": "web-app-v2",
  "deploy_schedule": "0 9 * * 1-5",
  "destroy_schedule": "0 18 * * 1-5",
  "description": "My web application using shared template"
}
```

### Template Resolution Priority

1. **Local `main.tf`**: Always highest priority (allows workspace-specific customization)
2. **Template reference**: Resolved from template registry
3. **Error**: No template found

### Local Customization

You can override template behavior by providing a local `main.tf` file in the workspace directory:

```
workspaces/my-custom-app/
├── config.json         # References template: "web-app-v2"
└── main.tf            # Custom version overrides template
```

The local `main.tf` takes precedence over the template reference, allowing customization while maintaining the template relationship.

## Template Update Behavior

### When a Template is Updated

**Currently deployed workspaces:**
- Continue running with current template version (stable)
- No immediate disruption to running infrastructure

**Next scheduled deployment:**
- Automatically uses updated template
- Workspace redeployed with new template content

**Manual deployment:**
- `workspacectl deploy workspace-name` forces immediate template update
- Useful for testing template changes

**Change detection:**
- Only real content changes trigger workspace updates
- Metadata-only changes don't affect deployments
- Content hashing ensures accurate change detection

### Template Update Example

```bash
# Update template
templatectl update web-app

# Check which workspaces use this template
templatectl show web-app

# Force immediate update of specific workspace
workspacectl deploy my-web-app

# Or wait for next scheduled deployment
```

## Working Directory Management

Each workspace gets its own isolated working directory:

```
/var/lib/provisioner/deployments/
├── my-web-workspace/
│   ├── main.tf                     # Copied from template
│   ├── variables.tf                # Additional template files
│   ├── terraform.tfstate           # Workspace-specific state
│   └── .provisioner-metadata.json # Template tracking
└── another-workspace/
    └── ...
```

**Benefits:**
- **State Isolation**: Each workspace maintains separate Terraform state
- **Template Stability**: Updates don't disrupt running workspaces
- **Change Tracking**: Content hashing detects real template changes
- **Version Control**: Templates track source URL, path, and Git ref

## Template Best Practices

### Template Design

1. **Parameterization**: Use variables for workspace-specific values
2. **Modularity**: Break complex templates into modules
3. **Documentation**: Include clear variable descriptions
4. **Outputs**: Provide useful outputs for dependent resources
5. **Validation**: Include variable validation rules

### Variable Conventions

**Standard Variables:**
```hcl
variable "workspace_name" {
  description = "Name of the workspace"
  type        = string
}

variable "deployment_mode" {
  description = "Deployment mode for scaling"
  type        = string
  default     = "normal"

  validation {
    condition = contains(["hibernation", "normal", "busy"], var.deployment_mode)
    error_message = "Deployment mode must be hibernation, normal, or busy."
  }
}

variable "environment" {
  description = "Environment (dev, staging, prod)"
  type        = string
  default     = "dev"
}
```

### Version Management

1. **Semantic Versioning**: Use semantic version tags (v1.0.0, v1.1.0)
2. **Stable References**: Use specific version tags for production
3. **Development Branches**: Use branch names for development templates
4. **Change Documentation**: Document breaking changes in Git commits

### Template Organization

**Repository Structure:**
```
terraform-templates/
├── web-app/
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── README.md
├── database/
│   ├── main.tf
│   ├── variables.tf
│   └── outputs.tf
└── monitoring/
    ├── main.tf
    ├── variables.tf
    ├── outputs.tf
    └── modules/
        └── alerts/
```

## Mode-Based Templates

Templates can support different deployment modes for dynamic scaling:

```hcl
# variables.tf
variable "deployment_mode" {
  description = "Deployment mode for resource scaling"
  type        = string
  default     = "normal"
}

# main.tf
locals {
  instance_counts = {
    hibernation = 0
    normal      = 2
    busy        = 5
  }

  instance_count = local.instance_counts[var.deployment_mode]
}

resource "aws_instance" "web" {
  count         = local.instance_count
  instance_type = var.deployment_mode == "busy" ? "t3.large" : "t3.micro"
  # ... other configuration
}
```

## Template Examples

### Basic Web Application Template

```hcl
# main.tf
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "workspace_name" {
  description = "Name of the workspace"
  type        = string
}

variable "deployment_mode" {
  description = "Deployment mode"
  type        = string
  default     = "normal"
}

locals {
  instance_sizes = {
    hibernation = "t3.nano"
    normal      = "t3.micro"
    busy        = "t3.small"
  }
}

resource "aws_instance" "web" {
  ami           = "ami-0c02fb55956c7d316"
  instance_type = local.instance_sizes[var.deployment_mode]

  tags = {
    Name           = "${var.workspace_name}-web"
    Workspace      = var.workspace_name
    DeploymentMode = var.deployment_mode
  }
}

output "instance_ip" {
  description = "Public IP address of web instance"
  value       = aws_instance.web.public_ip
}
```

### Database Template

```hcl
# main.tf
variable "workspace_name" {
  description = "Name of the workspace"
  type        = string
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.micro"
}

resource "aws_db_instance" "database" {
  identifier = "${var.workspace_name}-db"

  engine               = "postgres"
  engine_version       = "13.7"
  instance_class       = var.db_instance_class
  allocated_storage    = 20
  max_allocated_storage = 100

  db_name  = replace(var.workspace_name, "-", "_")
  username = "dbadmin"
  password = "changeme123!" # Use AWS Secrets Manager in production

  skip_final_snapshot = true

  tags = {
    Name      = "${var.workspace_name}-database"
    Workspace = var.workspace_name
  }
}

output "database_endpoint" {
  description = "RDS instance endpoint"
  value       = aws_db_instance.database.endpoint
}
```

## Troubleshooting

### Common Issues

**Template not found:**
```bash
templatectl show nonexistent-template
# Error: template 'nonexistent-template' not found
```

**Git repository access issues:**
```bash
templatectl add private-template https://github.com/private/repo
# Error: failed to clone repository: authentication required
```

**Template validation failures:**
```bash
templatectl validate broken-template
# Error: template validation failed: syntax error in main.tf
```

### Debugging Commands

```bash
# Check template registry
cat /var/lib/provisioner/templates/registry.json

# Validate template files manually
cd /var/lib/provisioner/templates/web-app
tofu validate

# Check workspace deployment directory
ls -la /var/lib/provisioner/deployments/my-workspace/
```

## Security Considerations

1. **Repository Access**: Ensure proper authentication for private repositories
2. **Secret Management**: Don't store secrets in templates; use external secret managers
3. **Template Validation**: Validate templates before adding to registry
4. **Access Control**: Restrict who can add/modify templates
5. **Version Pinning**: Use specific version tags rather than branch references for stability