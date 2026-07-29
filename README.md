[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/Kuzz007/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/Kuzz007/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/Kuzz007/3x-ui.svg" alt="GO Version"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
</p>

**This is a personal fork of [3X-UI](https://github.com/MHSanaei/3x-ui)** — the advanced, open-source web control panel for [Xray-core](https://github.com/XTLS/Xray-core) — with one major addition: **native AmneziaWG support**, added as a first-class protocol alongside VLESS, VMess, Trojan, and the rest. Everything else 3X-UI already does (multi-protocol inbounds, per-client traffic accounting, subscriptions, multi-node, the Telegram bot) is unchanged and still works exactly as upstream.

This fork exists to run the author's own routers and servers; it isn't trying to replace or compete with the original project. If you're looking for the general-purpose panel, go to [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — everything below only documents what's different here.

> [!IMPORTANT]
> This project is intended for personal use only. Please do not use it for illegal purposes or in a production environment.

## What's different in this fork: AmneziaWG

[AmneziaWG](https://github.com/amnezia-vpn/amneziawg-linux-kernel-module) is WireGuard with an added obfuscation layer (junk packets, randomized padding, magic-header rewriting) designed to defeat DPI-based protocol fingerprinting — the same tunnel, but one that doesn't look like a tunnel on the wire.

- **Native, not Docker.** AmneziaWG runs as a real kernel interface on the host, brought up and torn down with `awg-quick`/`awg` — the same DKMS kernel module approach as a native `wg0` interface. No privileged sidecar containers.
- **A first-class protocol.** An AmneziaWG inbound lives in the same `Inbound` table as everything else, so it gets bulk operations, the QR/config-download modal, and subscription links for free — nothing bespoke to learn.
- **Full AmneziaWG 2.0 obfuscation** — Jc/Jmin/Jmax (junk packets), S1–S4 (packet padding), H1–H4 (magic headers), and the I1 signature packet, all editable per-inbound with a one-click randomize button, plus a 1.x-compatible fallback for older clients.
- **Native IPv6**, with per-client NDP proxying so each peer gets a directly-reachable IPv6 address — no NAT66.
- **Per-client port-forwarding** — DNAT specific ports/ranges straight to one peer's tunnel address.
- **Routing a client's traffic through Xray** — every AmneziaWG inbound gets its own loopback Xray bridge automatically (no toggle to flip); route any client's traffic through any configured Xray outbound from the panel's existing Routing page, exactly like routing any other protocol.
- **`install.sh` installs the kernel module for you** on Ubuntu/Debian/Armbian (`ppa:amnezia/ppa`), with a fallback for other distros. One thing it can't do for you: **disable Secure Boot** on your VPS/VM first — a DKMS-built module is unsigned and the kernel won't load it while Secure Boot is enforced.
- Reconciled the same way [`internal/mtproto`](internal/mtproto) manages its `mtg` sidecar: a background job keeps the running interface in sync with what's saved in the database, hot-reloading peer changes via `awg syncconf` instead of bouncing the whole interface when it can.
- **Real `vpn://` share links** — the per-client copy-link/QR and the subscription endpoint now emit the actual `vpn://` scheme the official AmneziaVPN app expects (base64url of a plain `.conf`), not an invented URI format it couldn't import.

## Other changes in this fork

Smaller fork-specific improvements beyond AmneziaWG land here as they're added:

- **Routing rule autocomplete** — the Domain/IP fields in the Xray Routing rule editor suggest geosite/geoip categories (e.g. typing "you" suggests `geosite:youtube`) built live from whatever `.dat` files are actually installed in the Xray bin folder, including custom ones added via the Geodata auto-update feature (e.g. `geosite_roscom.dat`). Free-text entry still works exactly as before.
- **Live Speed for AmneziaWG and MTProto** — the Speed column used to show `--` for AmneziaWG and MTProto (`mtg`) inbounds/clients even though cumulative traffic totals were correct, since neither runs inside Xray-core's own runtime and so is invisible to its stats API. Both now broadcast live speed alongside every other protocol.
- **Custom files in `bin/` survive updates** — a reinstall/update used to wipe the whole `bin/` folder before re-extracting the release, silently deleting anything hand-placed there (most commonly a custom geoip/geosite file referenced from a routing rule via `ext:<file>:<code>`) and breaking every inbound at next start. The installer now backs up `bin/` first and restores only what the new release doesn't ship.

## Features

- **Multi-protocol inbounds** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, **AmneziaWG**, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel, and TUN.
- **Modern transports & security** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade, and XHTTP, secured with TLS, XTLS, and REALITY.
- **Fallbacks** — serve multiple protocols on a single port (e.g. VLESS and Trojan on 443) using Xray's fallback support.
- **Per-client management** — traffic quotas, expiry dates, IP limits, live online status, and one-click share links, QR codes, and subscriptions.
- **Traffic statistics** — per inbound, per client, and per outbound, with reset controls.
- **Multi-node support** — manage and scale across multiple servers from a single panel.
- **Outbound & routing** — WARP, NordVPN, custom routing rules, load balancers, and outbound proxy chaining.
- **Built-in subscription server** with multiple output formats and [custom page templates](docs/custom-subscription-templates.md).
- **Telegram bot** for remote monitoring and management.
- **RESTful API** with in-panel Swagger documentation.
- **Flexible storage** — SQLite (default) or PostgreSQL.
- **13 UI languages** with dark and light themes.
- **Fail2ban integration** for enforcing per-client IP limits.

## Screenshots

<details>
<summary>Click to expand</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Overview" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="./media/02-add-inbound-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/03-add-client-dark.png">
  <img alt="Add client" src="./media/03-add-client-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/05-add-nodes-dark.png">
  <img alt="Configs" src="./media/05-add-nodes-light.png">
</picture>

</details>

## Quick Start

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash
```

To install a specific version, append its tag (e.g. `v3.5.0-awg.1`):

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s v3.5.0-awg.1
```

To install the rolling **dev** build (latest per-commit pre-release from `main`, not a stable release), pass `dev`:

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s dev
```

This fork's own stable releases are tagged `<upstream base version>-awg.N` (e.g. `v3.5.0-awg.1`, built on top of what upstream calls `v3.5.0`) — never a bare `vX.Y.Z` — so they're never mistaken for a real upstream MHSanaei/3x-ui release of the same number.

During installation a random username, password, and access path are generated. After installation, run `x-ui` to open the management menu, where you can start/stop the service, view or reset your login credentials, manage SSL certificates, and more.

For general panel documentation beyond what's in this README, see the upstream [project Wiki](https://github.com/MHSanaei/3x-ui/wiki) — none of it is fork-specific, so it still applies.

### Unattended install

The installer also runs **non-interactively** for cloud-init.
Set `XUI_NONINTERACTIVE=1` (or pipe with no TTY) and it installs end-to-end with
zero prompts, generating random credentials and writing them to
`/etc/x-ui/install-result.env`. See [`deploy/`](deploy/) for:

- [Cloud-init user-data](deploy/cloud-init/) — unattended install on any cloud (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Hetzner Cloud notes](deploy/marketplace/hetzner/) — cloud-init deployment on Hetzner

## Supported Platforms

**Operating systems:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), and Alpine. (Upstream also publishes a Windows build; this fork's CI doesn't — everything here targets Linux servers/routers, and AmneziaWG needs a Linux kernel module regardless.)

**Architectures:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

AmneziaWG specifically needs a real Linux kernel with the AmneziaWG DKMS module — it will not come up on Windows, and `install_amneziawg` only automates the kernel-module install on Ubuntu/Debian/Armbian today (see [What's different in this fork](#whats-different-in-this-fork-amneziawg)).

## Database Options

3X-UI supports two backends, chosen during the install:

- **SQLite** (default) — a single file at `/etc/x-ui/x-ui.db`. Zero setup, ideal for small and medium deployments.
- **PostgreSQL** — recommended for high client counts or multi-node setups. The installer can install PostgreSQL locally for you, or accept a DSN to an existing server.

At runtime the backend is selected via environment variables (the installer writes these to `/etc/default/x-ui` for you):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Migrating an existing SQLite install to PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# then set XUI_DB_TYPE and XUI_DB_DSN in /etc/default/x-ui and restart:
systemctl restart x-ui
```

The source SQLite file is left untouched; remove it manually once you have verified the new backend.

## Environment Variables

| Variable | Description | Default |
| --- | --- | --- |
| `XUI_DB_TYPE` | Database backend: `sqlite` or `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL connection string (when `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | Directory for the SQLite database file | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Maximum open connections (PostgreSQL pool) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Maximum idle connections (PostgreSQL pool) | — |
| `XUI_INIT_WEB_BASE_PATH` | The initial URI path for the web panel | `/` |
| `XUI_ENABLE_FAIL2BAN` | Enable Fail2ban-based IP-limit enforcement | `true` |
| `XUI_LOG_LEVEL` | Log verbosity (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_DEBUG` | Enable debug mode | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Enable the tunnel health monitor (probes a URL and restarts xray after repeated failures; a restart drops all clients) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Proxy the probe is sent through; point it at a local xray inbound so the probe tests the tunnel (e.g. `socks5://127.0.0.1:1080`). Empty means the probe only checks host connectivity | — |
| `XUI_TUNNEL_HEALTH_URL` | URL probed for tunnel health | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Interval between probes | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Per-probe timeout | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Consecutive failures before a restart is triggered | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Minimum delay between consecutive restarts | `5m` |

## Supported Languages

The panel UI is available in 13 languages:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## Developer notes

This is a personal fork and isn't looking for outside contributors, but [CONTRIBUTING.md](/CONTRIBUTING.md) still has accurate, useful local dev-setup instructions (Go/Node versions, the C compiler CGo needs, build/lint/test commands) if you're working on this codebase yourself.

## Credit

This fork is built entirely on top of [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — all of the panel, the multi-protocol support, and the underlying architecture is their work; **AmneziaWG support is the only thing added here.** If you find the base project useful, the original author's support links are still the right place for it:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

The native AmneziaWG implementation in this fork was ported from/inspired by:

- [MHSanaei/3x-ui#6086](https://github.com/MHSanaei/3x-ui/pull/6086) — the original AmneziaWG PR against upstream (Docker-sidecar approach); this fork reuses its frontend schema/UI structure but replaces the backend with a native, no-Docker manager.
- [coinman-dev/3ax-ui](https://github.com/coinman-dev/3ax-ui) — an independent fork already running native AmneziaWG in production; this fork's `awg-quick` process management, config generation, and AmneziaWG 2.0 obfuscation parameter generator are ported from its `awg/` package.

## Acknowledgment

- [alireza0](https://github.com/alireza0/)
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**): _Enhanced v2ray/xray and v2ray/xray-clients routing rules with built-in Iranian domains and a focus on security and adblocking._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**): _This repository contains automatically updated V2Ray routing rules based on data on blocked domains and addresses in Russia._

## Community Tools

Tools and integrations built by the community around 3x-ui.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (License: **MIT**): _Manage inbounds, clients, panel settings, and Xray configuration as code with Terraform / OpenTofu._
