# 3x-ui on Hetzner Cloud

Hetzner Cloud does **not** have a third-party image marketplace the way AWS does.
There are two practical ways to ship 3x-ui on Hetzner.

## Option A — cloud-init (recommended, no image build)

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

## Option B — snapshot from the qcow2 / a configured server

Hetzner lets you create a **snapshot** of a running server and launch new
servers from it. Two ways to get there:

1. **From the Packer qcow2:** Hetzner does not allow direct qcow2 upload via the
   normal API, but you can boot a server, write the image to its disk in rescue
   mode, then take a snapshot — or simply use Option A, which needs no image.
2. **From a configured server:** spin up a server, install via cloud-init
   (Option A), verify, then **delete `/etc/x-ui/x-ui.db` and the first-boot
   sentinel** before snapshotting so clones regenerate their own credentials:

   ```bash
   systemctl stop x-ui
   rm -f /etc/x-ui/x-ui.db /etc/x-ui/.firstboot-done /etc/x-ui/credentials.txt
   # re-enable first-boot regeneration if you installed via Packer:
   systemctl enable x-ui-firstboot 2>/dev/null || true
   ```

   > ⚠️ If you snapshot a server **with** its `x-ui.db`, every clone shares the
   > same admin credentials and session secret. Always remove the DB first.

## "App"-style listing

Hetzner's curated apps live in the community repo
[`github.com/hetznercloud/apps`](https://github.com/hetznercloud/apps): each app
is essentially a documented cloud-init config plus metadata. To propose 3x-ui as
a Hetzner app, follow that repo's contribution pattern and base the app's
cloud-config on [`deploy/cloud-init/cloud-init.yaml`](../../cloud-init/cloud-init.yaml).
