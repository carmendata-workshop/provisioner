# Simple Example Workspace

This is a basic example workspace that demonstrates the core functionality of the OpenTofu Workspace Scheduler.

## Overview

This example creates a simple local file when deployed, making it perfect for:
- Learning how the provisioner works
- Testing your installation
- Understanding basic workspace structure
- Getting started with scheduled deployments

## Files

- **`config.json`** - Workspace configuration with schedules
- **`main.tf`** - Simple OpenTofu configuration
- **`README.md`** - This documentation

## What This Example Does

When deployed, this workspace:
1. Creates a text file at `/tmp/example_deployed.txt`
2. Includes the workspace name and deployment timestamp
3. Uses only the basic local provider (no external dependencies)

## Configuration

```json
{
  "name": "simple-example",
  "enabled": false,
  "deploy_schedule": "0 7 * * 1-5",
  "destroy_schedule": "30 17 * * 1-5",
  "description": "Example workspace - weekdays 7am-5:30pm (disabled by default)"
}
```

### Schedule Details
- **Deploy**: 7:00 AM, Monday through Friday
- **Destroy**: 5:30 PM, Monday through Friday
- **Status**: Disabled by default for safety

## Getting Started

1. **Copy this example** to create your own workspace:
   ```bash
   cp -r /etc/provisioner/workspaces/simple-example /etc/provisioner/workspaces/my-workspace
   ```

2. **Edit the configuration**:
   ```bash
   sudo nano /etc/provisioner/workspaces/my-workspace/config.json
   ```

3. **Update the name and enable it**:
   ```json
   {
     "name": "my-workspace",
     "enabled": true,
     "deploy_schedule": "0 9 * * 1-5",
     "destroy_schedule": "0 17 * * 1-5",
     "description": "My first workspace"
   }
   ```

4. **The scheduler will automatically detect the changes** (within 30 seconds)

## Customization

To modify what this workspace does:

1. **Edit `main.tf`** to change the deployment behavior
2. **Update variables** like the file path or content
3. **Add more resources** as needed

Example modifications:
```hcl
# Change the output file location
filename = "/tmp/my-custom-file.txt"

# Modify the content
content = "My workspace deployed at ${timestamp()}"

# Add more local files
resource "local_file" "additional_file" {
  content  = "Additional file content"
  filename = "/tmp/additional.txt"
}
```

## Verification

After deployment, check:
```bash
# View the created file
cat /tmp/simple-example_deployed.txt

# Check provisioner logs
sudo journalctl -u provisioner -f

# View workspace status
sudo systemctl status provisioner
```

## Next Steps

Once you're comfortable with this simple example:
1. Try the `complex-example` for advanced features
2. Create your own workspaces with real infrastructure
3. Experiment with different CRON schedules
4. Add more OpenTofu providers and resources

## Troubleshooting

- **Workspace not deploying?** Check that `enabled: true` in config.json
- **File not created?** Verify the deploy schedule has triggered
- **Permission errors?** Ensure the provisioner user can write to `/tmp`
- **Schedule not working?** Check the CRON syntax in config.json