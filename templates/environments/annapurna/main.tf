terraform {
  required_providers {
    local = {
      source  = "hashicorp/local"
      version = "~> 2.0"
    }
  }
}

resource "local_file" "environment_marker" {
  content  = "Environment: ${var.environment_name}\nDeployed at: ${timestamp()}\nSchedule Type: Multiple Deploy Schedules\n"
  filename = "/tmp/${var.environment_name}_deployed.txt"
}

variable "environment_name" {
  description = "Name of the environment"
  type        = string
  default     = "annapurna"
}