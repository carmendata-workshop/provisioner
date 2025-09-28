# DigitalOcean Provisioner Server Deployment
# This configuration creates a DigitalOcean droplet and installs the provisioner service

terraform {
  required_version = ">= 1.0"
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.0"
    }
  }
}

# Configure the DigitalOcean Provider
provider "digitalocean" {
  token = var.digitalocean_token
}

# Get all existing SSH keys from the DigitalOcean account
data "digitalocean_ssh_keys" "all" {}

# Create a new droplet for the provisioner
resource "digitalocean_droplet" "provisioner" {
  image     = var.droplet_image
  name      = var.server_name
  region    = var.droplet_region
  size      = var.droplet_size
  ssh_keys  = data.digitalocean_ssh_keys.all.ssh_keys[*].id

  # Enable monitoring and backups if specified
  monitoring = var.enable_monitoring
  backups    = var.enable_backups

  # Add user-specified tags only
  tags = var.tags

  # User data script to bootstrap the server
  user_data = templatefile("${path.module}/bootstrap.sh", {
    provisioner_version = var.provisioner_version
    github_repo        = var.github_repo
    server_timezone    = var.server_timezone
    auto_start         = var.auto_start_service
  })

  # Allow some time for the droplet to fully initialize
  # Note: SSH connection will use any of the configured SSH keys from your local agent
  provisioner "remote-exec" {
    connection {
      type        = "ssh"
      user        = "root"
      host        = self.ipv4_address
      timeout     = "5m"
      agent       = true
    }

    inline = [
      "while [ ! -f /var/log/cloud-init-output.log ] || ! grep -q 'Cloud-init.*finished' /var/log/cloud-init-output.log; do sleep 10; done",
      "echo 'Cloud-init bootstrap completed'"
    ]
  }
}

# Create a firewall for the provisioner droplet
resource "digitalocean_firewall" "provisioner" {
  name = "${var.server_name}-firewall"

  droplet_ids = [digitalocean_droplet.provisioner.id]

  # SSH access
  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = var.ssh_allowed_ips
  }

  # HTTP/HTTPS if web interface is enabled
  dynamic "inbound_rule" {
    for_each = var.enable_web_interface ? [1] : []
    content {
      protocol         = "tcp"
      port_range       = "80"
      source_addresses = ["0.0.0.0/0", "::/0"]
    }
  }

  dynamic "inbound_rule" {
    for_each = var.enable_web_interface ? [1] : []
    content {
      protocol         = "tcp"
      port_range       = "443"
      source_addresses = ["0.0.0.0/0", "::/0"]
    }
  }

  # Allow all outbound traffic
  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "icmp"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}

# Validate that the domain exists in DigitalOcean DNS
data "digitalocean_domain" "main" {
  name = var.domain_name
}

# Check if a record for the subdomain already exists
data "digitalocean_records" "existing" {
  domain = data.digitalocean_domain.main.id
  filter {
    key    = "name"
    values = [var.subdomain]
  }
  filter {
    key    = "type"
    values = ["A", "AAAA", "CNAME"]
  }
}


# Create DNS record for the provisioner server
# Note: Terraform will manage this record and detect conflicts naturally
resource "digitalocean_record" "provisioner" {

  domain = data.digitalocean_domain.main.id
  type   = "A"
  name   = var.subdomain
  value  = digitalocean_droplet.provisioner.ipv4_address
  ttl    = var.dns_ttl
}

# Optional: Create a volume for persistent data
resource "digitalocean_volume" "provisioner_data" {
  count = var.create_data_volume ? 1 : 0

  region      = var.droplet_region
  name        = "${var.server_name}-data"
  size        = var.data_volume_size
  description = "Persistent data volume for provisioner state and logs"
  tags        = var.tags
}

resource "digitalocean_volume_attachment" "provisioner_data" {
  count = var.create_data_volume ? 1 : 0

  droplet_id = digitalocean_droplet.provisioner.id
  volume_id  = digitalocean_volume.provisioner_data[0].id
}