# Multi-country exit subscription groups

This optional integration combines 3x-ui with the reviewed
[`jrangus/xnvgatebox`](https://github.com/jrangus/xnvgatebox) fork of
[`hanhaoops/xnvgatebox`](https://github.com/hanhaoops/xnvgatebox). The companion
is installed automatically on supported hosts, but it does not alter existing
clients or routes until an administrator creates an exit-pool subscription group.

## Install the exit engine

The normal 3x-ui one-line installer installs this component automatically on
Linux x86_64/systemd hosts. For an existing installation, or to retry after a
temporary network/dependency failure, run:

```bash
x-ui exit-pool install
```

The installer pins a reviewed xnvgatebox release, its installer commit, and two
independent SHA-256 values stored in 3x-ui. It refuses installation if either
download differs from the reviewed artifacts. Systemd, TUN and network
namespaces are required. Docker deployments are not supported by this helper.

Keep at least 1.5 GB free for a new combined installation, or 1 GB for an
in-place upgrade. Typical final amd64 disk use is about 250–350 MB for 3X-UI
plus 100–250 MB for xnvgatebox and its Ubuntu/Debian dependencies. Temporary
downloads, extraction, package caches, logs and databases require additional
headroom; CentOS can require more while compiling its SSTP package.

Installation does not loop. The upstream installer takes an exclusive file
lock, and 3X-UI restarts the exit service once after installation to apply its
loopback and systemd hardening. Messages such as `restoring tunnels`, failed
candidate probes and exit replacement are runtime health checks. They can
continue for a while when dozens of slots already exist, but they do not mean
the installer is running again. Do not rerun `install` or `secure` in response
to those messages.

Use `x-ui exit-pool access` only when you need to change candidate countries,
the number of exits, probe settings, or third-party SOCKS resources. Day-to-day
node assignment is performed in the 3x-ui panel.

## Create one subscription containing several regions

1. In xnvgatebox, select the required countries and wait until their exits show
   `up`.
2. Open **Xray configuration → Multi-country exit pool** in 3x-ui.
3. Choose one existing local VLESS, VMess or Trojan inbound as the mother node.
4. Select several ready regional exits, enter a group name, and click **Create**.
5. Open the 3x-ui **Clients** page, search for the displayed subscription ID,
   and copy its subscription URL.

## Exit source isolation

The panel treats the source as a routing boundary, not merely as a display
label. Existing reviewed xnvgatebox exits always remain in **xnvgatebox stable
pool**. Optional providers are kept in separate **new residential** and **new
native datacenter** pools. The stable pool is selected by default when creating
or editing a group.

An old group created before source isolation is automatically interpreted as a
stable-pool group; no database rewrite is required. New residential exits are
rejected unless xnvgatebox supplies current `strict_residential` or
`likely_residential` evidence. New native datacenter exits are rejected unless
both the native-country check and the datacenter/business-ISP classification
are present. Unknown, expired or mismatched evidence is never promoted into
either optional pool.

Disabling or removing an optional provider does not delete stable exits, stable
groups, ordinary inbounds, clients or routes. A subscription group must contain
exactly one source category. Both the panel and the server reject a mixed
stable/residential/datacenter selection, so a saved group can never lose its
source boundary. Create separate groups when more than one category is needed.

Generated client names also carry a source token (`xnvgatebox-stable`,
`new-residential`, or `new-native-dc`). This keeps the source visible in client
subscriptions and in multi-region group management even when several exits have
the same measured country.

### Free verified supplemental pool

Open the embedded xnvgatebox console, choose **Proxy resources → Free verified
pool**, and click **Fetch and verify on VPS**. Public SOCKS5 lists are used only
as untrusted candidate input. Every accepted endpoint must independently pass,
from the VPS:

- two HTTPS exit-IP observations with one unchanged public IPv4;
- an HTTPS content and certificate check through remote SOCKS DNS;
- current country and IP-type intelligence;
- `strict_residential` or `likely_residential` classification for the new
  residential pool; or verified native-country evidence plus `datacenter` or
  `business_isp` classification for the new native-datacenter pool.

Unknown, mobile, stale, private, hostname-only and failed candidates are not
published. Candidate labels and claimed countries are never trusted. A
successful refresh is stored atomically and uses deterministic slots in the
reserved `100000-999999` range. A failed or empty refresh preserves the last
unexpired supplemental pool. Once enabled, it refreshes automatically near
expiry. **Disable supplemental pool** cancels and joins an active refresh, then
renames the optional manifest for recovery; it never changes stable tunnels.

Free public proxies are short-lived and operated by unknown third parties.
They are suitable only for disposable, non-sensitive traffic. Do not send
credentials, payments, private messages or unencrypted data through them.

### Promote a verified SSTP resource

The embedded console can turn a current VPS-verified VPN Gate SSTP observation
into a managed VPS exit. Open **Proxy resources → Raw SSTP**, run the VPS
check, then select **Add to VPS exit pool** on an eligible row. Only exits with
fresh residential evidence, or datacenter/business-ISP evidence whose measured
country matches the source country, can be promoted.

The promoted connection does not expose SSTP to v2rayN. xnvgatebox keeps
SSTP/PPP inside that Slot's isolated network namespace and exposes an
authenticated SOCKS listener only on `127.0.0.1`. 3x-ui consumes that listener
through the same VLESS, VMess or Trojan mother inbound used by the stable pool.
The client therefore continues to use an ordinary panel subscription.

Promoted SSTP exits remain in **new residential** or **new native datacenter**;
they never enter **xnvgatebox stable pool**. A failed SSTP session is blocked
and rebuilt without falling back to the VPS address. Server-side PPP uses a
private `tun0` inside the namespace; this does not require TUN mode in the
desktop or mobile client.

3x-ui creates one dedicated client identity per selected region inside the same
inbound. Those identities share one `subId`, so one subscription contains
separate US, JP, SG, or other regional entries while they all use the mother
node's existing public address and port. Xray routes each identity to its own
local xnvgatebox SOCKS exit. No extra inbound or public port is created.

Use **Edit regions** to add or remove countries from an existing group. Deleting
a group removes only its generated clients, routing rules and unused managed
outbounds.

## Network behavior

Managed rules capture both TCP and UDP and do not distinguish IPv4 from IPv6.
TCP, client DNS packets, WebRTC UDP and IPv6 destinations are therefore sent to
the selected SOCKS exit. If that exit does not support UDP or IPv6, the request
fails there; no alternate/direct outbound is configured and it cannot silently
fall back to the VPS public IPv4 or IPv6. This prevents fallback leakage but
does not manufacture IPv6 support. Hysteria2, TUIC and QUIC should be treated
as unsupported unless the selected exit has been independently verified for
the required UDP behavior.

The integration reads xnvgatebox only through its authenticated loopback API.
SOCKS credentials are never returned to the browser.

In embedded 3x-ui mode the xnvgatebox management and subscription addresses are
intentionally loopback-only. The embedded console therefore disables its own
subscription copy buttons instead of presenting a useless `127.0.0.1` URL. Copy
the public client subscription from **3x-ui → Clients** after creating an exit
pool group.

3x-ui fails closed if `listen_addr` is not exactly `127.0.0.1`. The embedded
console cannot change xnvgatebox network settings or invoke its upstream online
updater. A systemd drop-in also applies process hardening while retaining the
network-namespace capabilities required to create exits. The management HTTP
port must not be opened in UFW, firewalld, security groups, or provider firewall
rules.

## Operations

```bash
x-ui exit-pool status
x-ui exit-pool access
x-ui exit-pool update
x-ui exit-pool uninstall
```

`x-ui exit-pool update` reinstalls the exact xnvgatebox build reviewed and
pinned by the installed 3x-ui version. It deliberately does not follow a mutable
"latest" release. Upgrade 3x-ui first when a newer reviewed exit-engine version
is published, then run the command above.

After installing or updating, verify the boundary with:

```bash
x-ui exit-pool secure
systemctl is-active xnvgatebox-fanout.service
sudo ss -lntp | grep xnvgatebox
```

The management listener should appear only on `127.0.0.1`, never `0.0.0.0` or
`[::]`.

Delete exit-pool subscription groups in the panel before uninstalling the exit
engine. VPN Gate exits are operated by third parties, can disappear without
notice, and should not be trusted with sensitive unencrypted traffic.

`xnvgatebox` is GPL-3.0-only and remains a separately distributed component.
The helper downloads its pinned reviewed release rather than embedding the
binary in 3x-ui.
