# DigitalOcean Provisioner Deployment Outputs

output "droplet_id" {
  description = "ID of the created droplet"
  value       = digitalocean_droplet.provisioner.id
}

output "droplet_name" {
  description = "Name of the created droplet"
  value       = digitalocean_droplet.provisioner.name
}

output "droplet_region" {
  description = "Region of the created droplet"
  value       = digitalocean_droplet.provisioner.region
}

output "public_ip" {
  description = "Public IP address of the droplet"
  value       = digitalocean_droplet.provisioner.ipv4_address
}

output "private_ip" {
  description = "Private IP address of the droplet"
  value       = digitalocean_droplet.provisioner.ipv4_address_private
}

output "ssh_command" {
  description = "SSH command to connect to the droplet"
  value       = "ssh root@${digitalocean_droplet.provisioner.ipv4_address}"
}

output "ssh_keys_used" {
  description = "SSH keys configured on the droplet"
  value = {
    count = length(data.digitalocean_ssh_keys.all.ssh_keys)
    names = data.digitalocean_ssh_keys.all.ssh_keys[*].name
  }
}

output "firewall_id" {
  description = "ID of the created firewall"
  value       = digitalocean_firewall.provisioner.id
}

output "dns_record" {
  description = "Created DNS record for the provisioner server"
  value = {
    domain    = var.domain_name
    subdomain = var.subdomain
    fqdn      = "${var.subdomain}.${var.domain_name}"
    ip        = digitalocean_droplet.provisioner.ipv4_address
    record_id = digitalocean_record.provisioner.id
  }
}

output "dns_validation" {
  description = "DNS validation information"
  value = {
    domain_exists = data.digitalocean_domain.main.name
    existing_records_count = length(data.digitalocean_records.existing.records)
    subdomain_was_available = length(data.digitalocean_records.existing.records) == 0
  }
}

output "volume_info" {
  description = "Information about the created data volume (if enabled)"
  value = var.create_data_volume ? {
    volume_id   = digitalocean_volume.provisioner_data[0].id
    volume_name = digitalocean_volume.provisioner_data[0].name
    size_gb     = digitalocean_volume.provisioner_data[0].size
    mount_path  = "/mnt/provisioner-data"
  } : null
}

output "service_status_command" {
  description = "Command to check provisioner service status"
  value       = "ssh root@${digitalocean_droplet.provisioner.ipv4_address} 'systemctl status provisioner'"
}

output "service_logs_command" {
  description = "Command to view provisioner service logs"
  value       = "ssh root@${digitalocean_droplet.provisioner.ipv4_address} 'journalctl -u provisioner -f'"
}

output "installation_info" {
  description = "Information about the provisioner installation"
  value = {
    version           = var.provisioner_version
    config_directory  = "/etc/provisioner"
    state_directory   = "/var/lib/provisioner"
    log_directory     = "/var/log/provisioner"
    binary_location   = "/opt/provisioner"
    service_name      = "provisioner"
  }
}

output "next_steps" {
  description = "Next steps after deployment"
  value = <<-EOT
    1. Connect to server: ssh root@${digitalocean_droplet.provisioner.ipv4_address}
    2. Access via domain: https://${var.subdomain}.${var.domain_name} (when web interface is available)
    3. Check service status: systemctl status provisioner
    4. View logs: journalctl -u provisioner -f
    5. Configure workspaces: /etc/provisioner/workspaces/
    6. Manage workspaces: workspacectl list
    7. Manage templates: templatectl list
    8. Manage jobs: jobctl list
  EOT
}