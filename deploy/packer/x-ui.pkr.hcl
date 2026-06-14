// 3x-ui golden image — one build, two sources:
//   * amazon-ebs : produces an AWS AMI (Marketplace-scannable)
//   * qemu       : produces a qcow2 (+ raw) for Hetzner/DO/Vultr/GCP/Azure/Oracle
//
// The image ships WITHOUT an initialized x-ui.db and WITHOUT any baked
// credentials. deploy/firstboot/x-ui-firstboot.{sh,service} generates unique
// per-instance credentials on first boot, before x-ui.service starts.
//
// Provisioner order is fixed: provision.sh -> harden.sh -> cleanup.sh.

packer {
  required_plugins {
    amazon = {
      version = ">= 1.3.0"
      source  = "github.com/hashicorp/amazon"
    }
    qemu = {
      version = ">= 1.1.0"
      source  = "github.com/hashicorp/qemu"
    }
  }
}

locals {
  build_stamp = formatdate("YYYYMMDD-hhmmss", timestamp())
  image_name  = "${var.ami_name_prefix}-ubuntu-${var.ubuntu_version}"
}

source "amazon-ebs" "x-ui" {
  region        = var.region
  instance_type = var.instance_type
  ssh_username  = var.ssh_username

  ami_name        = "${local.image_name}-${var.xui_version}-${local.build_stamp}"
  ami_description = "3x-ui panel on Ubuntu ${var.ubuntu_version}. Per-instance credentials are generated on first boot."

  source_ami_filter {
    filters = {
      name                = var.source_ami_filter_name
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    owners      = ["099720109477"] // Canonical
    most_recent = true
  }

  launch_block_device_mappings {
    device_name           = "/dev/sda1"
    volume_size           = 8
    volume_type           = "gp3"
    delete_on_termination = true
  }

  tags = {
    Name       = local.image_name
    Project    = "3x-ui"
    XuiVersion = var.xui_version
    BuildTool  = "packer"
    BaseOS     = "ubuntu-${var.ubuntu_version}"
  }
}

source "qemu" "x-ui" {
  iso_url      = var.qemu_iso_url
  iso_checksum = var.qemu_iso_checksum
  disk_image   = true
  disk_size    = "10G"
  format       = "qcow2"

  accelerator    = var.qemu_accelerator
  headless       = var.qemu_headless
  cpus           = 2
  memory         = 2048
  net_device     = "virtio-net"
  disk_interface = "virtio"

  output_directory = "output-qemu"
  vm_name          = "${local.image_name}.qcow2"

  // Build-time access: a NoCloud seed sets a temporary password for the default
  // user so Packer can SSH in. The seed is a separate CD-ROM (not part of the
  // output disk); the password is locked by harden.sh and state wiped by cleanup.sh.
  cd_label = "cidata"
  cd_content = {
    "meta-data" = ""
    "user-data" = <<-EOT
      #cloud-config
      password: ${var.qemu_build_password}
      chpasswd: { expire: false }
      ssh_pwauth: true
    EOT
  }

  ssh_username = var.ssh_username
  ssh_password = var.qemu_build_password
  ssh_timeout  = "20m"
  boot_wait    = "45s"

  shutdown_command = "sudo shutdown -P now"
}

build {
  name    = "3x-ui"
  sources = ["source.amazon-ebs.x-ui", "source.qemu.x-ui"]

  // Upload the first-boot unit + script so provision.sh can install them.
  provisioner "shell" {
    inline = ["mkdir -p /tmp/firstboot"]
  }
  provisioner "file" {
    source      = "${path.root}/../firstboot/x-ui-firstboot.sh"
    destination = "/tmp/firstboot/x-ui-firstboot.sh"
  }
  provisioner "file" {
    source      = "${path.root}/../firstboot/x-ui-firstboot.service"
    destination = "/tmp/firstboot/x-ui-firstboot.service"
  }

  provisioner "shell" {
    environment_vars = [
      "XUI_VERSION=${var.xui_version}",
      "XUI_ARCH=${var.xui_arch}",
      "DEBIAN_FRONTEND=noninteractive",
    ]
    execute_command = "chmod +x {{ .Path }}; sudo -E bash {{ .Path }}"
    scripts = [
      "${path.root}/scripts/provision.sh",
      "${path.root}/scripts/harden.sh",
      "${path.root}/scripts/cleanup.sh",
    ]
    // give cloud-init time to release apt locks on the very first boot
    pause_before = "10s"
  }

  // Convert the qcow2 to raw for clouds that need it (qemu source only).
  post-processor "shell-local" {
    only   = ["qemu.x-ui"]
    inline = ["qemu-img convert -p -O raw output-qemu/${local.image_name}.qcow2 output-qemu/${local.image_name}.raw"]
  }

  // Record the AMI id / artifacts for CI to surface.
  post-processor "manifest" {
    output     = "packer-manifest.json"
    strip_path = true
  }
}
