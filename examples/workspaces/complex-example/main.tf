terraform {
  required_version = ">= 1.0"

  required_providers {
    local = {
      source  = "hashicorp/local"
      version = "~> 2.0"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.9"
    }
  }
}

# Generate deployment timestamp
resource "time_static" "deployment_time" {}

# Create workspace marker file
resource "local_file" "workspace_marker" {
  content = templatefile("${path.module}/templates/workspace_info.tpl", {
    workspace_name = var.workspace_name
    deployment_time  = time_static.deployment_time.rfc3339
    instance_count   = var.instance_count
    workspace_type = var.workspace_type
    tags             = var.tags
  })
  filename = "${var.output_directory}/${var.workspace_name}_deployment.txt"
}

# Create configuration summary
resource "local_file" "config_summary" {
  content = jsonencode({
    workspace = var.workspace_name
    type        = var.workspace_type
    instances   = var.instance_count
    deployed_at = time_static.deployment_time.rfc3339
    tags        = var.tags
    outputs = {
      marker_file   = local_file.workspace_marker.filename
      summary_file  = "${var.output_directory}/${var.workspace_name}_config.json"
    }
  })
  filename = "${var.output_directory}/${var.workspace_name}_config.json"
}

# Create individual instance files if specified
resource "local_file" "instance_files" {
  count = var.create_instance_files ? var.instance_count : 0

  content = templatefile("${path.module}/templates/instance_info.tpl", {
    workspace_name = var.workspace_name
    instance_id      = count.index + 1
    instance_name    = "${var.workspace_name}-instance-${count.index + 1}"
    deployment_time  = time_static.deployment_time.rfc3339
    tags             = var.tags
  })
  filename = "${var.output_directory}/${var.workspace_name}_instance_${count.index + 1}.txt"
}