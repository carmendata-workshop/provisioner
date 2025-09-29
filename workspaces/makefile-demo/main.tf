# Example OpenTofu configuration for custom commands demo
# This is a minimal example that can be deployed with custom Makefile commands

terraform {
  required_version = ">= 1.6"
}

# Simple local file resource for demonstration
resource "local_file" "demo" {
  content  = "Deployed via Makefile at ${timestamp()}"
  filename = "${path.module}/deployed.txt"
}

output "deployment_method" {
  value = "Custom Makefile-based deployment"
}

output "deployed_at" {
  value = timestamp()
}