# Example Makefile for custom deployment orchestration
# This demonstrates how workspaces can use Makefiles to orchestrate
# complex deployment workflows beyond simple OpenTofu commands

.PHONY: init plan deploy destroy

# Initialize the workspace
init:
	@echo "Running custom initialization..."
	@echo "- Setting up environment variables"
	@echo "- Validating prerequisites"
	tofu init
	@echo "Initialization complete!"

# Plan the deployment
plan:
	@echo "Running custom plan with validation..."
	@echo "- Pre-plan validation checks"
	tofu validate
	tofu plan
	@echo "- Post-plan analysis"
	@echo "Plan complete!"

# Deploy the infrastructure
deploy:
	@echo "Running custom deployment workflow..."
	@echo "- Pre-deployment checks"
	@echo "- Running tofu apply"
	tofu apply -auto-approve
	@echo "- Post-deployment configuration"
	@echo "- Updating service registry"
	@echo "Deployment complete!"

# Destroy the infrastructure
destroy:
	@echo "Running custom destruction workflow..."
	@echo "- Pre-destroy safety checks"
	@echo "- Backing up state"
	@echo "- Running tofu destroy"
	tofu destroy -auto-approve
	@echo "- Post-destroy cleanup"
	@echo "Destruction complete!"