# Template: dev-server
# Source: file:///provisioner/templates/dev-server
# Ref: main
# Path: 

terraform {
  required_providers {
    local = {
      source  = "hashicorp/local"
      version = "~> 2.0"
    }
  }
}

resource "local_file" "template_marker" {
  content  = "Template: dev-server\nDeployed at: $${timestamp()}\n"
  filename = "/tmp/$${var.workspace_name}_dev-server_deployed.txt"
}

variable "workspace_name" {
  description = "Name of the workspace"
  type        = string
  default     = "template"
}

output "deployment_file" {
  value = local_file.template_marker.filename
}
