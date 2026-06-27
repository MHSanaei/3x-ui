# 3x-ui on Hetzner Cloud

Hetzner Cloud does **not** have a third-party image marketplace the way AWS does.
Ship 3x-ui via **cloud-init**: each instance installs non-interactively and
generates unique per-instance credentials (no `admin/admin`, no shared secret).

## cloud-init (no image build)

Use the generic user-data from [`../../cloud-init/`](../../cloud-init/). It installs
3x-ui non-interactively and generates unique per-instance credentials.

Web console: **Create Server → Cloud config** → paste
[`deploy/cloud-init/cloud-init.yaml`](../../cloud-init/cloud-init.yaml).

CLI:

```bash
hcloud server create \
  --name xui-1 \
  --type cx22 \
  --image ubuntu-24.04 \
  --user-data-from-file deploy/cloud-init/cloud-init.yaml
```

After boot, fetch the generated credentials:

```bash
ssh root@<server-ip> 'cat /etc/x-ui/install-result.env'
```

## "App"-style listing

Hetzner's curated apps live in the community repo
[`github.com/hetznercloud/apps`](https://github.com/hetznercloud/apps): each app
is essentially a documented cloud-init config plus metadata. To propose 3x-ui as
a Hetzner app, follow that repo's contribution pattern and base the app's
cloud-config on [`deploy/cloud-init/cloud-init.yaml`](../../cloud-init/cloud-init.yaml).
