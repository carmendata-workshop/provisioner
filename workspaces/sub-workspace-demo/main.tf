variable "deployment_mode" {
  description = "Deployment mode for scaling"
  type        = string
  default     = "busy"
}

# Main infrastructure resources
resource "null_resource" "main_infrastructure" {
  provisioner "local-exec" {
    command = "echo 'Deploying main infrastructure in ${var.deployment_mode} mode'"
  }

  triggers = {
    deployment_mode = var.deployment_mode
  }
}

# Network infrastructure
resource "null_resource" "network" {
  depends_on = [null_resource.main_infrastructure]

  provisioner "local-exec" {
    command = "echo 'Setting up network infrastructure'"
  }
}

# Security groups
resource "null_resource" "security" {
  depends_on = [null_resource.network]

  provisioner "local-exec" {
    command = "echo 'Configuring security groups and policies'"
  }
}

# Output information for sub-workspaces
output "infrastructure_id" {
  value = "infra-${random_id.infrastructure.hex}"
  description = "Infrastructure identifier for sub-workspaces"
}

output "network_id" {
  value = "network-${random_id.infrastructure.hex}"
  description = "Network identifier for sub-workspaces"
}

resource "random_id" "infrastructure" {
  byte_length = 4
}