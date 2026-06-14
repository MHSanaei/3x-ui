# 3x-ui on Amazon Lightsail

Two self-service ways to run 3x-ui on Lightsail, both producing **unique
per-instance credentials** (never `admin/admin`, never a shared secret).

> **Reality check.** The Lightsail *blueprint* list (WordPress, LAMP, GitLab…)
> is curated by AWS — you **cannot** self-publish your panel there, and Lightsail
> **cannot** launch from an arbitrary EC2 AMI. What you *can* do yourself is the
> two paths below. (For a public AWS listing you'd use the EC2 **AMI** +
> Marketplace path in [`../marketplace/aws/`](../marketplace/aws/), which is a
> different product from Lightsail.)

---

## Path A — launch script (simplest, self-service)

Install on a fresh instance at creation time. No image to build.

1. **Create instance** → platform **Linux/Unix** → blueprint **OS Only → Ubuntu 24.04**.
2. **Add launch script** → paste [`launch-script.sh`](launch-script.sh).
3. Create the instance.
4. After it boots, read the credentials:
   ```bash
   ssh ubuntu@<public-ip> 'sudo cat /etc/x-ui/install-result.env'
   ```
5. **Open the panel port** (see the firewall note below) and log in.

CLI equivalent:

```bash
aws lightsail create-instances \
  --instance-names my-3xui \
  --availability-zone eu-central-1a \
  --blueprint-id ubuntu_24_04 \
  --bundle-id small_3_0 \
  --user-data file://deploy/lightsail/launch-script.sh \
  --region eu-central-1
```

By default the panel uses a **random** high port (in `install-result.env`). To
pin a known port so you can pre-open it, set `export XUI_PANEL_PORT=54321` inside
`launch-script.sh`.

---

## Path B — reusable snapshot (your own "ready image")

Build a Lightsail **snapshot** once; launch as many instances from it as you
like, each generating its own credentials on first boot (the golden-image model).

```bash
deploy/lightsail/build-snapshot.sh --region eu-central-1 --panel-port 54321
```

What it does: launches a temporary Ubuntu instance with
[`snapshot-userdata.sh`](snapshot-userdata.sh) (installs the panel, **no DB**,
enables the first-boot unit), strips all state via the shared
[`cleanup.sh`](../packer/scripts/cleanup.sh), then snapshots and deletes the
build instance. Requires `awscli`, `jq`, `ssh` and Lightsail permissions.

Launch instances from the snapshot:

```bash
aws lightsail create-instances-from-snapshot \
  --instance-snapshot-name 3x-ui-ubuntu-24.04-<stamp> \
  --instance-names my-3xui-1 --bundle-id small_3_0 \
  --availability-zone eu-central-1a --region eu-central-1
```

Each launched instance runs `x-ui-firstboot` and writes its unique credentials to
`/etc/x-ui/credentials.txt` + `/etc/motd`. With `--panel-port` the port is the
same across instances (only the credentials differ), so you can pre-open it.

> Lightsail snapshots are **private to your AWS account** (and region). To use one
> elsewhere you can export it to EC2 (`aws lightsail export-snapshot`) and share
> the resulting AMI.

---

## Lightsail firewall note (important)

Lightsail's per-instance firewall only opens **22 / 80 / 443** by default. The
panel runs on a different port, so you must open it:

- Console: instance → **Networking → IPv4 Firewall → Add rule** (TCP, the panel port).
- CLI:
  ```bash
  aws lightsail open-instance-public-ports --region eu-central-1 \
    --instance-name my-3xui \
    --port-info fromPort=54321,toPort=54321,protocol=TCP
  ```

The panel port is in `/etc/x-ui/install-result.env` (Path A) or
`/etc/x-ui/credentials.txt` (Path B), or fixed via `--panel-port` / `XUI_PANEL_PORT`.
