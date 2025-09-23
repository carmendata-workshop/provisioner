variable "enable_cloudwatch_logs" {
  description = "Enable CloudWatch logging for all services"
  type        = bool
  default     = true
}

variable "enable_auto_scaling" {
  description = "Enable auto-scaling for web and app tiers"
  type        = bool
  default     = true
}

variable "ssl_certificate_arn" {
  description = "ARN of the SSL certificate for HTTPS"
  type        = string
  default     = ""
}

variable "domain_name" {
  description = "Domain name for the application"
  type        = string
  default     = ""
}

variable "backup_enabled" {
  description = "Enable automated backups for RDS"
  type        = bool
  default     = true
}