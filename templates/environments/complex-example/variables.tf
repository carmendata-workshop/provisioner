variable "environment_name" {
  description = "Name of the environment"
  type        = string
  default     = "complex-example"

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.environment_name))
    error_message = "Environment name must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "environment_type" {
  description = "Type of environment (development, staging, production)"
  type        = string
  default     = "development"

  validation {
    condition     = contains(["development", "staging", "production"], var.environment_type)
    error_message = "Environment type must be development, staging, or production."
  }
}

variable "instance_count" {
  description = "Number of instances to simulate"
  type        = number
  default     = 3

  validation {
    condition     = var.instance_count >= 1 && var.instance_count <= 10
    error_message = "Instance count must be between 1 and 10."
  }
}

variable "output_directory" {
  description = "Directory where output files will be created"
  type        = string
  default     = "/tmp"
}

variable "create_instance_files" {
  description = "Whether to create individual instance files"
  type        = bool
  default     = true
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default = {
    Project     = "OpenTofu Environment Scheduler"
    Environment = "example"
    ManagedBy   = "provisioner"
  }
}