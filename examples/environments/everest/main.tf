terraform {
  required_providers {
    local = {
      source  = "hashicorp/local"
      version = "~> 2.0"
    }
  }
}

resource "local_file" "environment_marker" {
  content  = "Environment: ${var.environment_name}\nDeployed at: ${timestamp()}\n"
  filename = "/tmp/${var.environment_name}_deployed.txt"
}

variable "environment_name" {
  description = "Name of the environment"
  type        = string
  default     = "everest"
}

output "deployment_file" {
  value = local_file.environment_marker.filename
}