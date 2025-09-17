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

# Create environment marker file
resource "local_file" "environment_marker" {
  content = templatefile("${path.module}/templates/environment_info.tpl", {
    environment_name = var.environment_name
    deployment_time  = time_static.deployment_time.rfc3339
    instance_count   = var.instance_count
    environment_type = var.environment_type
    tags             = var.tags
  })
  filename = "${var.output_directory}/${var.environment_name}_deployment.txt"
}

# Create configuration summary
resource "local_file" "config_summary" {
  content = jsonencode({
    environment = var.environment_name
    type        = var.environment_type
    instances   = var.instance_count
    deployed_at = time_static.deployment_time.rfc3339
    tags        = var.tags
    outputs = {
      marker_file   = local_file.environment_marker.filename
      summary_file  = "${var.output_directory}/${var.environment_name}_config.json"
    }
  })
  filename = "${var.output_directory}/${var.environment_name}_config.json"
}

# Create individual instance files if specified
resource "local_file" "instance_files" {
  count = var.create_instance_files ? var.instance_count : 0

  content = templatefile("${path.module}/templates/instance_info.tpl", {
    environment_name = var.environment_name
    instance_id      = count.index + 1
    instance_name    = "${var.environment_name}-instance-${count.index + 1}"
    deployment_time  = time_static.deployment_time.rfc3339
    tags             = var.tags
  })
  filename = "${var.output_directory}/${var.environment_name}_instance_${count.index + 1}.txt"
}