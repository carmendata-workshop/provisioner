variable "deployment_mode" {
  description = "Current deployment mode (hibernation, busy, maintenance)"
  type        = string
  default     = "busy"
}

# Example resource that scales based on deployment mode
resource "null_resource" "app_instances" {
  count = var.deployment_mode == "hibernation" ? 0 : (var.deployment_mode == "busy" ? 3 : 1)

  triggers = {
    mode = var.deployment_mode
  }

  provisioner "local-exec" {
    command = "echo 'Instance ${count.index + 1} running in ${var.deployment_mode} mode'"
  }
}

# Example of mode-specific configuration
resource "null_resource" "monitoring" {
  count = var.deployment_mode == "maintenance" ? 1 : 0

  provisioner "local-exec" {
    command = "echo 'Enhanced monitoring enabled for maintenance mode'"
  }
}

output "deployment_info" {
  value = {
    mode = var.deployment_mode
    instance_count = length(resource.null_resource.app_instances)
    monitoring_enabled = var.deployment_mode == "maintenance"
  }
}