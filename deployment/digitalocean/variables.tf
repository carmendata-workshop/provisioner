# DigitalOcean Provisioner Deployment Variables

# Required Variables
variable "digitalocean_token" {
  description = "DigitalOcean API token"
  type        = string
  sensitive   = true
}

# Server Configuration
variable "server_name" {
  description = "Name for the provisioner server"
  type        = string
  default     = "provisioner-server"

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.server_name))
    error_message = "Server name must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "droplet_region" {
  description = "DigitalOcean region for the droplet"
  type        = string
  default     = "lon1"

  validation {
    condition = contains([
      "nyc1", "nyc2", "nyc3", "ams2", "ams3", "sfo1", "sfo2", "sfo3",
      "sgp1", "lon1", "fra1", "tor1", "blr1", "syd1"
    ], var.droplet_region)
    error_message = "Invalid DigitalOcean region specified."
  }
}

variable "droplet_size" {
  description = "DigitalOcean droplet size"
  type        = string
  default     = "s-1vcpu-1gb"

  validation {
    condition = contains([
      "s-1vcpu-1gb", "s-1vcpu-2gb", "s-2vcpu-2gb", "s-2vcpu-4gb",
      "s-4vcpu-8gb", "s-6vcpu-16gb", "s-8vcpu-32gb"
    ], var.droplet_size)
    error_message = "Invalid droplet size specified."
  }
}

variable "droplet_image" {
  description = "DigitalOcean droplet image"
  type        = string
  default     = "ubuntu-22-04-x64"

  validation {
    condition = contains([
      "ubuntu-20-04-x64", "ubuntu-22-04-x64", "ubuntu-24-04-x64",
      "debian-11-x64", "debian-12-x64", "centos-stream-9-x64",
      "fedora-39-x64", "fedora-40-x64"
    ], var.droplet_image)
    error_message = "Unsupported droplet image. Use a supported Linux distribution."
  }
}

# Provisioner Configuration
variable "provisioner_version" {
  description = "Version of provisioner to install (latest, v1.0.0, etc.)"
  type        = string
  default     = "latest"
}

variable "github_repo" {
  description = "GitHub repository for provisioner releases"
  type        = string
  default     = "carmendata-workshop/provisioner"
}

variable "auto_start_service" {
  description = "Whether to automatically start the provisioner service after installation"
  type        = bool
  default     = true
}

variable "server_timezone" {
  description = "Timezone for the server (e.g., 'UTC', 'America/New_York')"
  type        = string
  default     = "UTC"
}

# Security Configuration
variable "ssh_allowed_ips" {
  description = "List of IP addresses/CIDR blocks allowed SSH access"
  type        = list(string)
  default     = ["0.0.0.0/0"]

  validation {
    condition = length(var.ssh_allowed_ips) > 0
    error_message = "At least one SSH allowed IP must be specified."
  }
}

variable "enable_monitoring" {
  description = "Enable DigitalOcean monitoring for the droplet"
  type        = bool
  default     = true
}

variable "enable_backups" {
  description = "Enable automated backups for the droplet"
  type        = bool
  default     = false
}

# Web Interface Configuration
variable "enable_web_interface" {
  description = "Whether to enable HTTP/HTTPS firewall rules (for future web interface)"
  type        = bool
  default     = false
}

# DNS Configuration
variable "domain_name" {
  description = "Domain name managed by DigitalOcean DNS (required)"
  type        = string

  validation {
    condition     = length(var.domain_name) > 0
    error_message = "Domain name is required and cannot be empty."
  }

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9.-]*[a-z0-9]$", var.domain_name))
    error_message = "Domain name must be a valid DNS name (lowercase letters, numbers, dots, and hyphens only)."
  }
}

variable "subdomain" {
  description = "Subdomain for the provisioner server"
  type        = string
  default     = "provisioner"

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]*[a-z0-9]$", var.subdomain)) || var.subdomain == "provisioner"
    error_message = "Subdomain must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "dns_ttl" {
  description = "TTL for DNS record in seconds"
  type        = number
  default     = 300

  validation {
    condition     = var.dns_ttl >= 30 && var.dns_ttl <= 86400
    error_message = "DNS TTL must be between 30 and 86400 seconds."
  }
}

# Storage Configuration
variable "data_volume_size" {
  description = "Size of the data volume in GB for persistent storage"
  type        = number
  default     = 10

  validation {
    condition     = var.data_volume_size >= 1 && var.data_volume_size <= 16384
    error_message = "Data volume size must be between 1 and 16384 GB."
  }
}

# Tagging
variable "tags" {
  description = "List of tags to apply to DigitalOcean resources"
  type        = list(string)
  default     = []

  validation {
    condition = alltrue([
      for tag in var.tags : can(regex("^[a-zA-Z0-9:._-]+$", tag))
    ])
    error_message = "Tags must contain only alphanumeric characters, colons, periods, underscores, and hyphens."
  }
}