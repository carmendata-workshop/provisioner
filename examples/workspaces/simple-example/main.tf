terraform {
  required_providers {
    local = {
      source  = "hashicorp/local"
      version = "~> 2.0"
    }
  }
}

resource "local_file" "workspace_marker" {
  content  = "Workspace: ${var.workspace_name}\nDeployed at: ${timestamp()}\n"
  filename = "/tmp/${var.workspace_name}_deployed.txt"
}

variable "workspace_name" {
  description = "Name of the workspace"
  type        = string
  default     = "example"
}

output "deployment_file" {
  value = local_file.workspace_marker.filename
}