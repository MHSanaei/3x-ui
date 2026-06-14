# Publishing 3x-ui to the AWS Marketplace (AMI)

This is the checklist for turning the Packer-built AMI into an AWS Marketplace
listing. It assumes you have already built an AMI with
[`../../packer/`](../../packer/) (locally or via `.github/workflows/image.yml`).

> Do **not** commit AMI IDs, AWS account numbers, or credentials. The AMI ID is
> printed to the workflow job summary at build time.

## 1. Seller registration (one-time)

1. Sign in to the [AWS Marketplace Management Portal](https://aws.amazon.com/marketplace/management/)
   with the AWS account that will own the listing.
2. Complete **seller registration** (legal entity, bank, tax interview). Required
   before any product can be submitted.

## 2. Build a compliant AMI

Build in the seller account (or share the AMI into it):

```bash
cd deploy/packer
packer init .
# amd64
packer build -only='amazon-ebs.x-ui' \
  -var 'xui_version=vX.Y.Z' -var 'xui_arch=amd64' -var 'instance_type=t3.small' -var 'region=eu-central-1' .
# arm64 (Graviton)
packer build -only='amazon-ebs.x-ui' \
  -var 'xui_version=vX.Y.Z' -var 'xui_arch=arm64' -var 'instance_type=t4g.small' -var 'region=eu-central-1' .
```

You can list both AMIs (amd64 + arm64) as architectures of a single Marketplace
product, or as separate products.

The image already satisfies the Marketplace AMI policies enforced by `harden.sh`
+ `cleanup.sh`:

- ✅ `PasswordAuthentication no`, `PermitRootLogin prohibit-password`
- ✅ no default OS account passwords (all locked)
- ✅ no baked `authorized_keys`, no SSH host keys (regenerated on boot)
- ✅ base OS = current Ubuntu 24.04 LTS, patched at build time
- ✅ no application default credentials — the panel admin is generated on first
  boot on a random high port (no `admin/admin`, no shipped `x-ui.db`)

## 3. Run the self-service AMI scan

1. In the Management Portal: **Server products → AMIs → Upload/scan an AMI**.
2. Share the AMI with the AWS Marketplace scanning account when prompted
   (the portal gives you the exact account id and the `modify-image-attribute`
   command, or share it from the EC2 console).
3. Start the scan. It checks SSH config, default credentials, open ports, and
   for malware. Fix any finding and re-scan.

Common scan findings and where they're handled:

| Finding | Fix (already in the build) |
| --- | --- |
| Password authentication enabled | `harden.sh` sshd drop-in |
| Root login with password | `harden.sh` `PermitRootLogin prohibit-password` |
| Default user password set | `harden.sh` `passwd -l` on all accounts |
| Authorized keys present | `cleanup.sh` removes them |
| Out-of-date packages | base image is the latest LTS; `provision.sh` runs `apt-get update` |

## 4. Create the product (limited / private first)

1. **Server products → Create new product → AMI** (or AMI + CloudFormation).
2. Add title, description, categories, pricing (free or paid), regions, the AMI
   id, recommended instance types, and the **usage instructions** (tell buyers
   to read `/etc/x-ui/credentials.txt` / MOTD after first boot for the generated
   admin login, then change the password).
3. Submit as a **Limited** (private) listing first. AWS publishes it with
   restricted visibility so only your account / allow-listed accounts see it.

## 5. Preview & launch test

1. From the limited listing, **subscribe and launch** a test instance.
2. SSH in, `sudo cat /etc/x-ui/credentials.txt`, open the panel URL, log in,
   confirm the panel works and the credentials are unique to that instance.
3. Launch a second instance and confirm its credentials differ (no shared
   secrets).

## 6. Go public

1. Once the scan passes and the preview looks correct, request **public
   visibility** (move from Limited to Public) in the listing.
2. AWS does a final review before the listing goes live.

## References

- AWS Marketplace seller guide: <https://docs.aws.amazon.com/marketplace/latest/userguide/>
- AMI-based product requirements: <https://docs.aws.amazon.com/marketplace/latest/userguide/product-and-ami-policies.html>
- Self-service AMI scanning: <https://docs.aws.amazon.com/marketplace/latest/userguide/product-submission.html>
