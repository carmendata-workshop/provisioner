# Makefile Demo Workspace

This workspace demonstrates the **custom deployment commands** feature, showing how to use a Makefile to orchestrate deployments instead of running OpenTofu commands directly.

## Configuration

The `config.json` specifies custom commands:

```json
{
  "custom_deploy": {
    "init_command": "make init",
    "plan_command": "make plan",
    "apply_command": "make deploy"
  },
  "custom_destroy": {
    "init_command": "make init",
    "destroy_command": "make destroy"
  }
}
```

## Makefile Targets

- **`make init`** - Initializes workspace with validation and environment setup
- **`make plan`** - Runs plan with pre and post validation checks
- **`make deploy`** - Executes deployment with pre/post hooks
- **`make destroy`** - Destroys infrastructure with safety checks and cleanup

## Use Cases

Custom commands are useful for:

- **Complex orchestration**: Multi-step workflows with validation, testing, notifications
- **Pre/post hooks**: Run checks, backups, or notifications before/after OpenTofu
- **Legacy integration**: Integrate with existing Makefile-based workflows
- **Custom tooling**: Use wrapper scripts that handle credentials, logging, etc.
- **Multi-tool deployments**: Combine OpenTofu with other tools (Ansible, kubectl, etc.)

## How It Works

When the provisioner schedules this workspace for deployment:

1. Copies workspace files to `/var/lib/provisioner/deployments/makefile-demo/`
2. Checks for `custom_deploy` configuration
3. Executes `make init` instead of `tofu init`
4. Executes `make plan` instead of `tofu plan`
5. Executes `make deploy` instead of `tofu apply -auto-approve`

## Testing Manually

```bash
# Deploy the workspace immediately
./bin/workspacectl deploy makefile-demo

# Destroy the workspace
./bin/workspacectl destroy makefile-demo

# View deployment status
./bin/workspacectl status makefile-demo
```