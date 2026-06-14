// Input variables for the 3x-ui golden-image build.
// See README.md for usage. Override with -var / -var-file or env (PKR_VAR_*).

variable "xui_version" {
  type        = string
  description = "3x-ui release tag to install, e.g. v3.3.1. 'latest' resolves the newest GitHub release at build time."
  default     = "latest"
}

variable "xui_arch" {
  type        = string
  description = "CPU architecture to build for: amd64 or arm64."
  default     = "amd64"
  validation {
    condition     = contains(["amd64", "arm64"], var.xui_arch)
    error_message = "The xui_arch value must be 'amd64' or 'arm64'."
  }
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
  default     = "eu-central-1"
}

variable "instance_type" {
  type        = string
  description = "EC2 instance type used to build the AMI. Must match xui_arch (e.g. t3.small for amd64, t4g.small for arm64/Graviton)."
  default     = "t3.small"
}

variable "ami_name_prefix" {
  type        = string
  description = "Prefix for the produced AMI name."
  default     = "3x-ui"
}

variable "source_ami_filter_name" {
  type        = string
  description = "Override for the Canonical Ubuntu base AMI name filter. Empty ⇒ derived from xui_arch (latest patched 24.04 LTS for that arch)."
  default     = ""
}

variable "ssh_username" {
  type        = string
  description = "Default SSH user on the base Ubuntu cloud image."
  default     = "ubuntu"
}

// --- qemu (qcow2 / raw) -------------------------------------------------------

variable "qemu_iso_url" {
  type        = string
  description = "Override for the Ubuntu cloud image used as the qemu base disk. Empty ⇒ derived from xui_arch (amd64/arm64 cloud image)."
  default     = ""
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

# --- qemu arm64-only knobs (ignored for amd64) -------------------------------

variable "qemu_cpu" {
  type        = string
  description = "QEMU -cpu model for arm64 builds: 'host' with KVM on an arm64 host, 'max' for TCG emulation."
  default     = "host"
}

variable "qemu_efi_code" {
  type        = string
  description = "Path to the arm64 UEFI code firmware (AAVMF). Only used when xui_arch=arm64."
  default     = "/usr/share/AAVMF/AAVMF_CODE.fd"
}

variable "qemu_efi_vars" {
  type        = string
  description = "Path to the arm64 UEFI vars firmware template (AAVMF). Only used when xui_arch=arm64."
  default     = "/usr/share/AAVMF/AAVMF_VARS.fd"
}
