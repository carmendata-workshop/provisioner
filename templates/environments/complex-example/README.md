# Complex Example Environment

This is an advanced example environment that demonstrates sophisticated OpenTofu patterns and the full capabilities of the Environment Scheduler.

## Overview

This example showcases:
- **Multiple CRON schedules** for complex timing patterns
- **Variables with validation** for configuration flexibility
- **Outputs** for deployment information
- **Template files** for dynamic content generation
- **Resource iteration** with count and for loops
- **Multiple providers** (local and time)

## Files

- **`config.json`** - Environment configuration with multiple schedules
- **`main.tf`** - Primary infrastructure definition
- **`variables.tf`** - Input variables with validation
- **`outputs.tf`** - Output values and information
- **`templates/`** - Template files for dynamic content
  - `environment_info.tpl` - Environment information template
  - `instance_info.tpl` - Instance information template
- **`README.md`** - This documentation

## What This Example Does

When deployed, this environment:
1. **Creates multiple files** based on configuration
2. **Generates dynamic content** using templates
3. **Simulates instances** with configurable counts
4. **Produces structured outputs** for integration
5. **Validates inputs** to prevent configuration errors

## Configuration

```json
{
  "name": "complex-example",
  "enabled": false,
  "deploy_schedule": ["0 8 * * 1-5", "0 13 * * 1-5"],
  "destroy_schedule": ["0 12 * * 1-5", "0 18 * * 1-5"],
  "description": "Complex example - multiple daily cycles with variables and outputs (disabled by default)"
}
```

### Schedule Details
- **Deploy**: 8:00 AM and 1:00 PM, Monday through Friday (twice daily)
- **Destroy**: 12:00 PM and 6:00 PM, Monday through Friday (twice daily)
- **Pattern**: Creates 4-hour deployment windows (8am-12pm, 1pm-6pm)
- **Status**: Disabled by default for safety

## Variables

This example includes several configurable variables:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `environment_name` | string | "complex-example" | Name of the environment |
| `environment_type` | string | "development" | Type (development/staging/production) |
| `instance_count` | number | 3 | Number of instances to create |
| `output_directory` | string | "/tmp" | Where to create output files |
| `create_instance_files` | bool | true | Whether to create individual instance files |
| `tags` | map(string) | {...} | Tags to apply to resources |

## Outputs

The environment provides several useful outputs:

- **`deployment_info`** - Complete deployment information
- **`created_files`** - List of all files created
- **`deployment_summary`** - Human-readable summary
- **`next_steps`** - Suggested actions after deployment

## Getting Started

1. **Copy this example** to create your own environment:
   ```bash
   cp -r /etc/provisioner/environments/complex-example /etc/provisioner/environments/my-complex-env
   ```

2. **Edit the configuration**:
   ```bash
   sudo nano /etc/provisioner/environments/my-complex-env/config.json
   ```

3. **Customize the environment**:
   ```json
   {
     "name": "my-complex-env",
     "enabled": true,
     "deploy_schedule": ["0 9 * * 1-5"],
     "destroy_schedule": ["0 17 * * 1-5"],
     "description": "My complex environment"
   }
   ```

4. **Optional: Customize variables** by editing `variables.tf` defaults

## Advanced Features

### Variable Validation

The example includes input validation:
```hcl
variable "environment_type" {
  validation {
    condition     = contains(["development", "staging", "production"], var.environment_type)
    error_message = "Environment type must be development, staging, or production."
  }
}
```

### Template Files

Dynamic content generation using templates:
```hcl
resource "local_file" "environment_marker" {
  content = templatefile("${path.module}/templates/environment_info.tpl", {
    environment_name = var.environment_name
    deployment_time  = time_static.deployment_time.rfc3339
    # ... more variables
  })
}
```

### Resource Iteration

Creating multiple resources with count:
```hcl
resource "local_file" "instance_files" {
  count = var.create_instance_files ? var.instance_count : 0
  # ... resource configuration
}
```

## Customization Examples

### Change Instance Count
```hcl
variable "instance_count" {
  default = 5  # Instead of 3
}
```

### Add More File Types
```hcl
resource "local_file" "log_files" {
  count    = var.instance_count
  content  = "Log file for instance ${count.index + 1}"
  filename = "${var.output_directory}/logs/${var.environment_name}_${count.index + 1}.log"
}
```

### Custom Tags
```hcl
variable "tags" {
  default = {
    Project     = "My Project"
    Environment = "production"
    Team        = "DevOps"
    CostCenter  = "Engineering"
  }
}
```

## Multiple Schedules Use Cases

This example uses multiple deploy/destroy schedules for:

1. **Development Cycles**: Morning and afternoon development sessions
2. **Testing Windows**: Deploy for testing, destroy after completion
3. **Cost Optimization**: Multiple short-lived environments instead of long-running ones
4. **Demo Scenarios**: Deploy before meetings, destroy after presentations

## Verification

After deployment, check:
```bash
# View created files
ls -la /tmp/complex-example*

# Check deployment info
cat /tmp/complex-example_deployment.txt

# View JSON summary
cat /tmp/complex-example_config.json | jq

# Check instance files
cat /tmp/complex-example_instance_*.txt

# Monitor provisioner logs
sudo journalctl -u provisioner -f
```

## Integration

The structured outputs make this example suitable for:
- **CI/CD pipelines** - Parse JSON outputs for deployment status
- **Monitoring systems** - Track deployment times and instance counts
- **Automation scripts** - Use outputs to trigger follow-up actions
- **Reporting dashboards** - Display environment status and metrics

## Troubleshooting

- **Validation errors?** Check that variable values meet validation rules
- **Template errors?** Verify template file syntax and variable references
- **File creation fails?** Ensure output directory exists and is writable
- **Multiple schedules not working?** Verify JSON array syntax in config.json
- **Instance files missing?** Check `create_instance_files` variable

## Next Steps

Use this example as a foundation for:
1. **Real infrastructure** with cloud providers (AWS, Azure, GCP)
2. **Complex scheduling patterns** with multiple deployment windows
3. **Variable-driven environments** for different deployment scenarios
4. **Template-based configurations** for dynamic content generation
5. **Output integration** with monitoring and CI/CD systems