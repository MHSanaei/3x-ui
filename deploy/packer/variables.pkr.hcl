// Input variables for the 3x-ui golden-image build.
// See README.md for usage. Override with -var / -var-file or env (PKR_VAR_*).

variable "xui_version" {
  type        = string
  description = "3x-ui release tag to install, e.g. v3.3.1. 'latest' resolves the newest GitHub release at build time."
  default     = "latest"
}

variable "xui_arch" {
  type        = string
  description = "CPU architecture of the released tarball to install (amd64 or arm64)."
  default     = "amd64"
}

variable "ubuntu_version" {
  type        = string
  description = "Ubuntu LTS version label, used only for image naming/tags."
  default     = "24.04"
}

// --- amazon-ebs (AMI) ---------------------------------------------------------

variable "region" {
  type        = string
  description = "AWS region the AMI is built in."
  default     = "us-east-1"
}

variable "instance_type" {
  type        = string
  description = "EC2 instance type used to build the AMI."
  default     = "t3.small"
}

variable "ami_name_prefix" {
  type        = string
  description = "Prefix for the produced AMI name."
  default     = "3x-ui"
}

variable "source_ami_filter_name" {
  type        = string
  description = "Name filter for the Canonical Ubuntu base AMI (resolves the latest patched LTS)."
  default     = "ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"
}

variable "ssh_username" {
  type        = string
  description = "Default SSH user on the base Ubuntu cloud image."
  default     = "ubuntu"
}

// --- qemu (qcow2 / raw) -------------------------------------------------------

variable "qemu_iso_url" {
  type        = string
  description = "Ubuntu cloud image (qcow2) used as the qemu base disk."
  default     = "https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img"
}

variable "qemu_iso_checksum" {
  type        = string
  description = "Checksum for the qemu base disk. 'file:<SHA256SUMS url>' auto-fetches; 'none' skips verification."
  default     = "file:https://cloud-images.ubuntu.com/releases/24.04/release/SHA256SUMS"
}

variable "qemu_accelerator" {
  type        = string
  description = "QEMU accelerator: 'kvm' when /dev/kvm is available, else 'tcg' (slow software emulation)."
  default     = "kvm"
}

variable "qemu_headless" {
  type        = bool
  description = "Run QEMU without a display (required on CI runners)."
  default     = true
}

variable "qemu_build_password" {
  type        = string
  description = "Temporary password injected via cloud-init for Packer's build-time SSH. Locked/removed before the image is finalized."
  default     = "packer-build-temp-pw"
  sensitive   = true
}
