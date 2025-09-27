# Demo workspace for job scheduling functionality
# This workspace demonstrates how jobs can be scheduled alongside infrastructure

terraform {
  required_providers {
    local = {
      source  = "hashicorp/local"
      version = "~> 2.0"
    }
  }
}

# Simple local file resource to demonstrate workspace deployment
resource "local_file" "demo_marker" {
  content  = "Job Demo Workspace deployed at: ${timestamp()}\n"
  filename = "/tmp/job-demo-workspace-deployed.txt"
}

# Output that jobs can access
output "workspace_info" {
  value = {
    name         = "job-demo"
    deployed_at  = timestamp()
    marker_file  = local_file.demo_marker.filename
  }
}