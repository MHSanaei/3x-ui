# Deploying the jrangus/3x-ui fork

This repository is an independently deployed fork. Its installer, updater,
management script, service units, release archives, and checksum sidecars are
all resolved from `jrangus/3x-ui`; deployment does not write to or download
runtime scripts from the fork's upstream repository.

## 1. Publish build artifacts

Open the repository's **Actions** tab, enable workflows if GitHub asks, and run
the release workflow for the commit you intend to deploy. Wait for it to finish
successfully, then verify that the repository's **Releases** page contains:

- `x-ui-linux-<architecture>.tar.gz` for the VPS architecture;
- the matching `.tar.gz.sha256` checksum file;
- a release or rolling `dev-latest` tag.

Do not deploy until both the archive and checksum exist. The installer rejects
an archive whose checksum cannot be verified.

Manual `workflow_dispatch` runs on `main` publish the rolling `dev-latest`
pre-release. The default installer prefers that security-hardened rolling asset
through a direct download probe, avoiding the unauthenticated GitHub API limit.

## 2. Install on a clean VPS

Supported script installations require root and a supported Linux
distribution. Install the latest security-hardened rolling release with:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/jrangus/3x-ui/main/install.sh)
```

The explicit equivalent is:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/jrangus/3x-ui/main/install.sh) dev-latest
```

For cloud-init, use `deploy/cloud-init/cloud-init.yaml`; it points to this fork.
Generated credentials are stored in `/etc/x-ui/install-result.env` with mode
`0600`.

On Linux x86_64/systemd hosts the same command also installs the reviewed,
checksum-pinned exit-pool companion. It binds management and local proxy ports
to `127.0.0.1` and does not alter existing nodes or routing. Set
`XUI_INSTALL_EXIT_POOL=0` to opt out, or `XUI_REQUIRE_EXIT_POOL=1` when an
exit-pool installation failure must make unattended provisioning fail.

### Disk-space planning

Keep at least **1.5 GB free** for a clean installation and **1 GB free** for an
in-place update. These are conservative working-space recommendations that
include the release archive, extraction, package-manager caches and exit-pool
dependencies. A typical amd64 installation uses roughly 250–350 MB for 3X-UI
and another 100–250 MB for the exit engine and its Ubuntu/Debian dependencies;
databases, certificates, logs and the operating system are additional. CentOS
may need more temporary space while it builds the SSTP client locally.

The exit-pool installer is protected by `flock`, so a second install or update
cannot run concurrently. A clean installation starts the companion and then
restarts it once to apply the 3X-UI hardening policy. Subsequent messages about
fetching candidates, restoring tunnels, failed probes or changing exits are
normal runtime health checks, not repeated installation. A new host with no
exit group does not restore another server's exits; an upgraded host restores
its existing configured slots, which can take time when many regions were
selected.

## 3. Verify the installation

```bash
systemctl status x-ui --no-pager
x-ui status
x-ui privacy-status
x-ui exit-pool status
```

Run `x-ui` for the interactive menu. Option **26** manages kernel BBR and shows
the running kernel's capabilities. Option **29** manages privacy/network
status, the DNS egress guard, and explicit UFW/firewalld port rules.

The DNS egress guard intentionally refuses to start until the VPS uses a local
loopback DNS resolver listening on port 53. Configure and test an encrypted
local resolver first, then run:

```bash
x-ui enable-dns-guard
```

Rollback is always available with:

```bash
x-ui disable-dns-guard
```

## 4. Update

After a new release workflow finishes successfully:

```bash
x-ui update
```

For the rolling build, use `x-ui update-dev`. The web updater automatically
tracks `dev-latest` when the running binary is a rolling build. CLI and web
updates remain on this fork, validate the updater script before execution, and
verify the downloaded archive against its published checksum.

For migration from upstream 3X-UI or an older copy of this fork, first check
that `/` has at least 1 GB available. Run each command separately:

```bash
df -h /
sudo cp -a /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.before-upgrade

curl --connect-timeout 10 --max-time 120 --retry 3 -fL \
  'https://raw.githubusercontent.com/jrangus/3x-ui/main/x-ui.sh' \
  -o /tmp/x-ui.sh
test "$(head -n 1 /tmp/x-ui.sh)" = '#!/bin/bash'
bash -n /tmp/x-ui.sh
sudo install -m 755 /tmp/x-ui.sh /usr/bin/x-ui

sudo env XUI_SCRIPT_REF=main x-ui update-dev
sudo x-ui exit-pool update
```

Use `sudo x-ui exit-pool install` instead of the final command if the companion
was not previously installed. The panel database, certificates and existing
exit-pool settings are preserved. Verify once after completion:

```bash
/usr/local/x-ui/x-ui -v
sudo systemctl is-active x-ui xnvgatebox-fanout.service xnvgatebox-pool.timer
sudo ss -lntp | grep xnvgatebox
```

Every xnvgatebox listener must be on `127.0.0.1`. Do not publish ports `8899`
or `17928-17999` through a host firewall, cloud security group, or provider
firewall.

## 5. Firewall and remote-access safety

Keep SSH allowed before enabling a host firewall. The privacy menu changes
ports only through an already-active UFW or firewalld installation and never
starts a firewall with guessed rules. Prefer source-restricted CIDRs for panel
or node-management ports. Do not expose an unauthenticated administrative
shell or grant a network service unrestricted root execution.
