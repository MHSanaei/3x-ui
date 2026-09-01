[Русский](/README.md) | [English](/README.en_US.md)

<p align="center">
  <img alt="3x-ui-awg" src="./media/3x-ui-awg-logo.png">
</p>

<p align="center">
  <a href="https://github.com/kuzzrus/3x-ui-awg/actions"><img src="https://img.shields.io/github/actions/workflow/status/kuzzrus/3x-ui-awg/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/kuzzrus/3x-ui-awg.svg" alt="GO Version"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
</p>

<p align="center">
  <a href="https://www.tbank.ru/cf/2qxNvGa3fSX"><img src="https://img.shields.io/badge/%E2%9D%A4%EF%B8%8F_Support_this_fork-Donate_via_T--Bank-FFDD2D?style=for-the-badge&labelColor=1a1a1a" alt="Donate via T-Bank"></a>
</p>

**3x-ui-awg** is a panel for running your own VPN/proxy servers, built around not getting fingerprinted by DPI and not looking like a VPN at all. Under the hood it's a fork of [3X-UI](https://github.com/MHSanaei/3x-ui) — everything 3X-UI already did (multi-protocol inbounds, per-client traffic accounting, subscriptions, multi-node, the Telegram bot) is unchanged and still works exactly as upstream. But since then it's grown a native protocol, two independent traffic-camouflage systems, another way out, and a pile of fixes found on real servers, not in theory — so from here on this reads as its own project, not a patch-notes list on someone else's.

> [!IMPORTANT]
> This project is intended for personal use only. Please do not use it for illegal purposes or in a production environment.

## AmneziaWG — native, not bolted on

[AmneziaWG](https://github.com/amnezia-vpn/amneziawg-linux-kernel-module) is WireGuard with an added obfuscation layer (junk packets, randomized padding, magic-header rewriting) designed to defeat DPI-based protocol fingerprinting — the same tunnel, but one that doesn't look like a tunnel on the wire.

- **Embedded in the panel process, not a kernel module.** Runs over a userspace network stack ([amneziawg-go](https://github.com/amnezia-vpn/amneziawg-go)) — no DKMS build, no Secure Boot conflict, no privileged sidecar container, nothing to install on the host at all.
- **A first-class protocol alongside the rest.** Lives in the same `Inbound` table as everything else — bulk operations, the QR/config-download modal, and subscription links all work out of the box.
- **Full 2.0 obfuscation** — Jc/Jmin/Jmax (junk packets), S1–S4 (padding), H1–H4 (magic headers), and all 5 signature-packet slots I1–I5, each generatable with one click as a DNS/STUN/SIP/QUIC-mimicry packet, a real Chrome/Firefox/Safari TLS ClientHello, or pure randomness — all editable per inbound, plus a 1.x-compatible fallback for older clients.
- **Opt-in 3.0 and 3.1** — `HeaderProtectionKey`/`ContentPaddingAddition` and `RandomTrailers`/`DisableCookies`, each a deliberate, explicit choice that doesn't change wire behavior on its own.
- **Every client's traffic already goes through Xray** — no TPROXY, no extra bridge: each inbound relays straight into its own loopback Xray SOCKS5 inbound, so stats, online status, sniffing, and routing rules all work exactly like any other protocol.
- **Real `vpn://` share links** that the official AmneziaVPN app actually imports, not an invented format.
- **Distinct per-client IPv6 identity and port-forwarding** — also on the embedded engine.

This didn't stay an internal fork detail — [PR #6105](https://github.com/MHSanaei/3x-ui/pull/6105) carrying this implementation is merged into MHSanaei/3x-ui's main branch.

## Camouflage: two independent ways to hide the panel

Anyone probing your `:443` without the right secret/path shouldn't be able to tell a panel lives there at all — everywhere they look should be an ordinary, working site.

- **A real AdGuard Home as the decoy.** Not an imitation — the panel installs, configures, and keeps a genuine AdGuard Home alive and serves it as the decoy content: someone probing without the secret gets a working DNS filter with a real admin UI and DoH, not a prop. AdGuard Home's login/password can be changed right from the panel.
- **7 interactive login-mock decoys** — AdGuard Home, Portainer, Pi-hole, OMV, Jellyfin, Home Assistant, Uptime Kuma — with real attempt-lockout behavior, not just form screenshots.
- **A built-in reverse proxy** (**Settings → Reverse proxy**) routes by path itself: the panel's path to the panel, the subscription path to the subscription server, everything else to the decoy. Point a REALITY inbound's fallback `target` at it. Certificates are either your own files or auto-issued via Let's Encrypt. Replaces the hand-written Nginx config this used to require; if something goes wrong, `x-ui setting -disableFrontProxy` turns it off over SSH.

## One more way out: Tor

A one-click Tor outbound — the panel installs and keeps a `tor` process alive, traffic goes out through its SOCKS5. Slower than WARP/NordVPN, but maximum anonymity where that matters more than speed.

## Subscriptions and routing

- **RoscomVPN routing presets** for Happ and Incy — DEFAULT/JSONSUB/WHITELIST/Custom from one selector, the ruleset itself updates live from an external source.
- **Routing-rule autocomplete** — the IP/Domain fields suggest geosite/geoip categories straight from whatever `.dat` files are actually installed in Xray's bin folder, including custom ones.
- **Stable subscription tags** — refreshing a subscription can no longer silently repoint another server's stable tag.

## Built and battle-tested on real servers

Every fix below was found and confirmed against live infrastructure, not hypothesized:

- **Keepalive for tunnel clients** — `PersistentKeepalive` can be set explicitly now, not just left at whatever default.
- **Live speed for AmneziaWG and MTProto** — the Speed column used to show `--` for both, even though cumulative traffic totals were correct.
- **Custom files in `bin/` survive updates** — an update used to wipe the whole `bin/` folder, now it backs up first and restores only what the new release doesn't ship.
- Plus closed races and bugs in TPROXY/firewall rules, per-inbound peer address binding, MTU headroom under large S4 padding, and IPv6 aliasing — each found and fixed against reality, not in theory.

## Features (full list)

- **Multi-protocol inbounds** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, **AmneziaWG**, MTProto, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel, and TUN.
- **Modern transports & security** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade, and XHTTP, secured with TLS, XTLS, and REALITY.
- **Fallbacks** — serve multiple protocols on one port (e.g. VLESS and Trojan on 443).
- **Outbounds** — WARP, NordVPN, **Tor**, custom routing rules, load balancers, outbound proxy chaining.
- **Camouflage** — reverse proxy with a decoy (a real AdGuard Home, or login-mock pages).
- **Per-client management** — traffic quotas, expiry dates, IP limits, live online status, one-click share links/QR/subscriptions.
- **Traffic statistics** — per inbound, per client, and per outbound, with reset controls.
- **Multi-node support** — manage and scale across servers from one panel.
- **Built-in subscription server** with [custom page templates](docs/custom-subscription-templates.md).
- **Telegram bot** for remote monitoring and management.
- **RESTful API** with in-panel Swagger documentation.
- **SQLite or PostgreSQL**, your choice.
- **Russian and English UI**, dark and light themes.
- **Fail2ban integration** for per-client IP limits.

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
curl -fsSL https://raw.githubusercontent.com/kuzzrus/3x-ui-awg/main/install.sh | bash
```

To install a specific version, append its tag (e.g. `v3.7.0-awg.18`):

```bash
curl -fsSL https://raw.githubusercontent.com/kuzzrus/3x-ui-awg/main/install.sh | bash -s v3.7.0-awg.18
```

To install the rolling **dev** build (latest per-commit pre-release from `main`, not a stable release), pass `dev`:

```bash
curl -fsSL https://raw.githubusercontent.com/kuzzrus/3x-ui-awg/main/install.sh | bash -s dev
```

This fork's own stable releases are tagged `<upstream base version>-awg.N` (e.g. `v3.7.0-awg.18`, built on top of what upstream calls `v3.7.0`) — never a bare `vX.Y.Z` — so they're never mistaken for a real upstream MHSanaei/3x-ui release of the same number.

During installation a random username, password, and access path are generated. After installation, run `x-ui` to open the management menu, where you can start/stop the service, view or reset your login credentials, manage SSL certificates, and more.

For general panel documentation beyond what's in this README, see the upstream [project Wiki](https://github.com/MHSanaei/3x-ui/wiki) — none of it is fork-specific, so it still applies.

### Unattended install

The installer also runs **non-interactively** for cloud-init. Set `XUI_NONINTERACTIVE=1` (or pipe with no TTY) and it installs end-to-end with zero prompts, generating random credentials and writing them to `/etc/x-ui/install-result.env`. See [`deploy/`](deploy/) for:

- [Cloud-init user-data](deploy/cloud-init/) — unattended install on any cloud (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Hetzner Cloud notes](deploy/marketplace/hetzner/)

## Supported Platforms

**Operating systems:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), and Alpine. (Upstream also publishes a Windows build; this fork's CI doesn't — everything here targets Linux servers/routers.)

**Architectures:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

AmneziaWG is embedded in the panel binary itself — no kernel module, no separate install step, no distro-specific setup.

## Database Options

- **SQLite** (default) — a single file at `/etc/x-ui/x-ui.db`. Zero setup, ideal for small and medium deployments.
- **PostgreSQL** — recommended for high client counts or multi-node setups. The installer can install PostgreSQL locally for you, or accept a DSN to an existing server.

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Migrating an existing SQLite install to PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
systemctl restart x-ui
```

The source SQLite file is left untouched; remove it manually once you have verified the new backend.

## Environment Variables

| Variable | Description | Default |
| --- | --- | --- |
| `XUI_DB_TYPE` | Database backend: `sqlite` or `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL connection string | — |
| `XUI_DB_FOLDER` | Directory for the SQLite database file | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Maximum open connections (PostgreSQL pool) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Maximum idle connections | — |
| `XUI_INIT_WEB_BASE_PATH` | The initial URI path for the web panel | `/` |
| `XUI_ENABLE_FAIL2BAN` | Enable Fail2ban-based IP-limit enforcement | `true` |
| `XUI_LOG_LEVEL` | Log verbosity | `info` |
| `XUI_DEBUG` | Enable debug mode | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Enable the tunnel health monitor (a restart drops all clients) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Proxy the probe is sent through (e.g. `socks5://127.0.0.1:1080`) | — |
| `XUI_TUNNEL_HEALTH_URL` | URL probed for tunnel health | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Interval between probes | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Per-probe timeout | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Consecutive failures before a restart is triggered | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Minimum delay between consecutive restarts | `5m` |

## Supported Languages

Russian and English, dark and light themes.

## Developer notes

This is a personal fork and isn't looking for outside contributors, but [CONTRIBUTING.md](/CONTRIBUTING.md) still has accurate, useful local dev-setup instructions (Go/Node versions, the C compiler CGo needs, build/lint/test commands) if you're working on this codebase yourself.

## Credit and acknowledgment

Built on top of [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — the panel's underlying architecture is their work. The AmneziaWG implementation draws on:

- [amnezia-vpn/amneziawg-go](https://github.com/amnezia-vpn/amneziawg-go) — the userspace AmneziaWG implementation over [gVisor](https://gvisor.dev/), embedded directly in the panel process.
- [MHSanaei/3x-ui#6086](https://github.com/MHSanaei/3x-ui/pull/6086) — the original AmneziaWG PR against upstream (Docker-sidecar approach); this fork reuses its frontend schema/UI structure.
- [coinman-dev/3ax-ui](https://github.com/coinman-dev/3ax-ui) — an independent fork already running native AmneziaWG in production; the fork's early kernel-module manager and 2.0 obfuscation-parameter generator were ported from its `awg/` package before the move to the embedded engine.

If this fork is useful to you, donations are welcome: [Donate via T-Bank](https://www.tbank.ru/cf/2qxNvGa3fSX).

Also used:

- [alireza0](https://github.com/alireza0/)
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (**GPL-3.0**): enhanced v2ray/xray routing rules with built-in Iranian domains, focused on security and adblocking.
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (**GPL-3.0**): automatically updated routing rules for domains/addresses blocked in Russia.

## Community Tools

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (**MIT**): manage inbounds, clients, panel settings, and Xray configuration as code with Terraform / OpenTofu.
