output "deployment_info" {
  description = "Information about the deployment"
  value = {
    workspace_name = var.workspace_name
    workspace_type = var.workspace_type
    instance_count   = var.instance_count
    deployment_time  = time_static.deployment_time.rfc3339
    tags             = var.tags
  }
}

output "created_files" {
  description = "List of files created by this deployment"
  value = {
    workspace_marker = local_file.workspace_marker.filename
    config_summary     = local_file.config_summary.filename
    instance_files = var.create_instance_files ? [
      for i in range(var.instance_count) :
      "${var.output_directory}/${var.workspace_name}_instance_${i + 1}.txt"
    ] : []
  }
}

output "deployment_summary" {
  description = "Human-readable deployment summary"
  value = "Workspace '${var.workspace_name}' (${var.workspace_type}) deployed at ${time_static.deployment_time.rfc3339} with ${var.instance_count} instances"
}

output "next_steps" {
  description = "Suggested next steps for this workspace"
  value = [
    "Check deployment files in ${var.output_directory}/",
    "Review configuration in ${local_file.config_summary.filename}",
    "Monitor workspace status via provisioner logs",
    "Update config.json to modify schedules if needed"
  ]
}