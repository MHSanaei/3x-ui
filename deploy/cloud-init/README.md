# 3x-ui via cloud-init

A single [`cloud-init.yaml`](cloud-init.yaml) user-data file that installs 3x-ui
non-interactively on a fresh Ubuntu/Debian VM and generates **unique random
credentials per instance**. It works on any cloud-init platform.

## How it works

1. The VM boots a stock Ubuntu/Debian cloud image.
2. cloud-init writes and runs `/opt/xui-bootstrap.sh`, which exports
   `XUI_NONINTERACTIVE=1` and pipes the project's `install.sh` into `bash`.
3. `install.sh` runs end-to-end with **zero prompts**, picking secure random
   values for any credential you didn't pin.
4. The generated credentials are written to `/etc/x-ui/install-result.env`
   (mode 600), echoed to the **serial console**, and appended to `/etc/motd`.

Retrieve them after boot with either:

```bash
sudo cat /etc/x-ui/install-result.env     # over SSH
```

…or read the provider's **serial console** output (handy before you have SSH).

## Customising

Edit the `export XUI_*` lines inside the `write_files` block of
[`cloud-init.yaml`](cloud-init.yaml). All knobs are optional; unset ⇒ random/secure default.

| Env var | Default | Meaning |
| --- | --- | --- |
| `XUI_SSL_MODE` | `none` | `none` (plain HTTP), `ip` (Let's Encrypt IP cert), `domain` |
| `XUI_USERNAME` | random | Admin username |
| `XUI_PASSWORD` | random | Admin password |
| `XUI_PANEL_PORT` | random high port | Panel listen port |
| `XUI_WEB_BASE_PATH` | random | Panel base path (obscures the URL) |
| `XUI_DOMAIN` | — | Required when `XUI_SSL_MODE=domain` |
| `XUI_ACME_EMAIL` | — | Let's Encrypt account email (domain mode) |
| `XUI_DB_TYPE` / `XUI_DB_DSN` | `sqlite` | Set `postgres` + DSN to use PostgreSQL |

> **TLS note:** `none` serves the panel over plain HTTP on a random high port —
> fine behind a reverse proxy or an SSH tunnel, but put TLS in front of it before
> exposing the panel publicly. `domain` mode needs a public DNS A record pointing
> at the box and port 80 reachable at install time.

## Per-provider usage

- **Hetzner Cloud** — *Create Server → Cloud config*: paste the file. Or CLI:
  `hcloud server create --image ubuntu-24.04 --user-data-from-file cloud-init.yaml ...`
- **AWS EC2** — *Advanced details → User data*: paste the file. Or
  `aws ec2 run-instances --user-data file://cloud-init.yaml ...`
- **DigitalOcean** — *Create Droplet → Advanced options → Add Initialization
  scripts (user data)*: paste the file. Or `doctl compute droplet create --user-data-file cloud-init.yaml ...`
- **Vultr** — *Deploy → Additional Features → Cloud-Init User-Data*: paste the file.
- **Google Cloud (GCE)** — `gcloud compute instances create xui \
  --image-family ubuntu-2404-lts-amd64 --image-project ubuntu-os-cloud \
  --metadata-from-file user-data=cloud-init.yaml`
- **Azure** — `az vm create --image Ubuntu2404 --custom-data cloud-init.yaml ...`
- **Oracle Cloud (OCI)** — *Create Instance → Show advanced options →
  Management → Cloud-init script*: paste (or base64-upload) the file.

## Validate before you deploy

```bash
cloud-init schema --config-file deploy/cloud-init/cloud-init.yaml
```
