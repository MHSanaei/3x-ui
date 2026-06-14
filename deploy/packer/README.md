# 3x-ui golden image (Packer)

Builds a cloud image with the 3x-ui panel pre-installed but **not configured**:
the image ships with **no database and no credentials**, and generates a unique
admin account on first boot. This is the **primary** path for AWS Marketplace
and any reusable image.

Two sources, one build:

| Source | Output | For |
| --- | --- | --- |
| `amazon-ebs` | AWS AMI | AWS / Marketplace |
| `qemu` | `qcow2` (+ `raw`) | Hetzner, DigitalOcean, Vultr, GCP, Azure, Oracle, bare metal |

Both sources build for **`amd64` and `arm64`** (select with `-var xui_arch=...`).

## Why no baked DB

3x-ui seeds a hardcoded `admin/admin` user and generates its session secret +
panel GUID the first time it starts. If an image shipped an initialized
`x-ui.db`, **every clone would share the same credentials and secret**. So the
build deliberately:

- installs the panel binary + systemd unit but **never starts it** and **never
  creates a DB** (`scripts/provision.sh`);
- wipes any stray DB/credentials/host-keys at the end (`scripts/cleanup.sh`);
- enables `x-ui-firstboot.service`, which on first boot resets settings, sets a
  random username/password on a random high port, regenerates the secret/GUID,
  and writes the credentials to `/etc/x-ui/credentials.txt` + `/etc/motd`
  (`deploy/firstboot/`).

## Prerequisites

- [Packer](https://developer.hashicorp.com/packer) ≥ 1.9
- For `qemu` amd64: `qemu-system-x86`, `qemu-utils` (and `/dev/kvm` for acceptable speed)
- For `qemu` arm64: `qemu-system-arm`, `qemu-efi-aarch64`, `qemu-utils` — best built on an
  arm64 host (native KVM); cross-building from x86 works but uses slow TCG emulation
- For `amazon-ebs`: AWS credentials with EC2 build permissions (arm64 builds on a Graviton
  instance such as `t4g.small`)

```bash
cd deploy/packer
packer init .
packer fmt -check .      # formatting
packer validate .        # both sources
```

## Build

Build a specific release (recommended) or `latest`:

```bash
# amd64 qcow2 (no cloud account needed)
packer build -only='qemu.x-ui' -var 'xui_version=v3.3.1' -var 'xui_arch=amd64' .

# arm64 qcow2 (run on an arm64 host for native KVM)
packer build -only='qemu.x-ui' -var 'xui_version=v3.3.1' -var 'xui_arch=arm64' .

# amd64 AWS AMI
packer build -only='amazon-ebs.x-ui' \
  -var 'xui_version=v3.3.1' -var 'xui_arch=amd64' -var 'instance_type=t3.small' -var 'region=eu-central-1' .

# arm64 AWS AMI (Graviton)
packer build -only='amazon-ebs.x-ui' \
  -var 'xui_version=v3.3.1' -var 'xui_arch=arm64' -var 'instance_type=t4g.small' -var 'region=eu-central-1' .
```

Outputs (per arch):
- `output-qemu/3x-ui-ubuntu-24.04-<arch>.qcow2` and `.raw`
- the AMI id (also recorded in `packer-manifest.json`)

If `/dev/kvm` is unavailable, add `-var 'qemu_accelerator=tcg'` (much slower).

## Key variables

See [`variables.pkr.hcl`](variables.pkr.hcl) for the full list.

| Variable | Default | Notes |
| --- | --- | --- |
| `xui_version` | `latest` | Release tag to install, e.g. `v3.3.1` |
| `xui_arch` | `amd64` | `amd64` or `arm64` (derives the base AMI / cloud image) |
| `region` | `eu-central-1` | AWS region (amazon-ebs) |
| `instance_type` | `t3.small` | EC2 build instance — must match the arch (`t4g.small` for arm64) |
| `qemu_accelerator` | `kvm` | `kvm` or `tcg` |
| `qemu_cpu` | `host` | arm64 `-cpu` model (`host` with KVM, `max` for TCG) |
| `ubuntu_version` | `24.04` | Base Ubuntu LTS (naming/tags) |

The CI workflow builds both arches automatically: amd64 qcow2 on a standard runner,
arm64 qcow2 on a native `ubuntu-24.04-arm` runner, and both AMIs from a single runner
(the build instance runs in AWS).

## First boot

On the first boot of any instance launched from the image:

1. `x-ui-firstboot.service` runs **before** `x-ui.service`.
2. It generates a unique admin username/password, a random panel port, a random
   base path, and an API token.
3. Credentials are written to `/etc/x-ui/credentials.txt` (root-only) and shown
   in `/etc/motd`. Retrieve them with `sudo cat /etc/x-ui/credentials.txt`.
4. The panel then starts on the random port. `admin/admin` never exists.

## CI

`.github/workflows/image.yml` runs this build on `release: published` (and via
`workflow_dispatch`), attaching the compressed `qcow2` to the release and
building the AMI when AWS credentials are configured.

## A note on host firewalls

`scripts/harden.sh` intentionally does **not** enable a restrictive host
firewall. 3x-ui opens Xray inbound ports on admin-chosen ports at runtime, which
a host firewall would block. Use your cloud provider's security groups/firewall
instead, and open the panel port + your inbound ports there. If you still want a
host firewall, add `ufw` rules in `harden.sh` allowing SSH, the panel port and
your inbound ports.
