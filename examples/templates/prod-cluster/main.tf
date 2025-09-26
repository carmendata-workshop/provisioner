# Template: prod-cluster
# Source: file:///provisioner/templates/prod-cluster
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
  content  = "Template: prod-cluster\nDeployed at: $${timestamp()}\n"
  filename = "/tmp/$${var.environment_name}_prod-cluster_deployed.txt"
}

variable "environment_name" {
  description = "Name of the environment"
  type        = string
  default     = "template"
}

output "deployment_file" {
  value = local_file.template_marker.filename
}
