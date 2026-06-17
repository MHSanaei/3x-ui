[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/dune-dark.png">
    <img alt="dune" src="./media/dune-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/leto217/DUNE/releases"><img src="https://img.shields.io/github/v/release/leto217/DUNE" alt="Release"></a>
  <a href="https://github.com/leto217/DUNE/actions"><img src="https://img.shields.io/github/actions/workflow/status/leto217/DUNE/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/leto217/DUNE.svg" alt="GO Version"></a>
  <a href="https://github.com/leto217/DUNE/releases/latest"><img src="https://img.shields.io/github/downloads/leto217/DUNE/total.svg" alt="Downloads"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/leto217/DUNE"><img src="https://pkg.go.dev/badge/github.com/leto217/DUNE.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/leto217/DUNE"><img src="https://goreportcard.com/badge/github.com/leto217/DUNE" alt="Go Report Card"></a>
</p>

**DUNE** is a lightweight fork of [3X-UI](https://github.com/MHSanaei/3x-ui) — an open-source web control panel for managing [Xray-core](https://github.com/XTLS/Xray-core) servers. It keeps the familiar workflows and protocol coverage of 3X-UI while using significantly less CPU and RAM, making it ideal for small VPS instances and other low-resource hosts.

Forked from 3X-UI with a focus on efficiency, DUNE trims background work, tightens memory usage, and streamlines the stack so the panel stays responsive without hogging your server.

> [!IMPORTANT]
> This project is intended for personal use only. Please do not use it for illegal purposes or in a production environment.

## Features

- **Multi-protocol inbounds** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel, and TUN.
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
bash <(curl -Ls https://raw.githubusercontent.com/leto217/DUNE/main/install.sh)
```

During installation a random username, password, and access path are generated. After installation, run `dune` to open the management menu, where you can start/stop the service, view or reset your login credentials, manage SSL certificates, and more.

For full documentation, please visit the [project Wiki](https://github.com/leto217/DUNE/wiki).

### Unattended install & cloud images

The installer also runs **non-interactively** for cloud-init and golden images.
Set `DUNE_NONINTERACTIVE=1` (or pipe with no TTY) and it installs end-to-end with
zero prompts, generating random credentials and writing them to
`/etc/dune/install-result.env`. See [`deploy/`](deploy/) for:

- [Cloud-init user-data](deploy/cloud-init/) — unattended install on any cloud (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Packer golden image](deploy/packer/) — build an AWS EC2 AMI + qcow2 (amd64/arm64) with per-instance credentials generated on first boot
- [Amazon Lightsail](deploy/lightsail/) — launch script + reusable snapshot builder
- [AWS Marketplace checklist](deploy/marketplace/aws/)

## Supported Platforms

**Operating systems:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine, and Windows.

**Architectures:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Database Options

Dune supports two backends, chosen during the install:

- **SQLite** (default) — a single file at `/etc/dune/dune.db`. Zero setup, ideal for small and medium deployments.
- **PostgreSQL** — recommended for high client counts or multi-node setups. The installer can install PostgreSQL locally for you, or accept a DSN to an existing server.

At runtime the backend is selected via environment variables (the installer writes these to `/etc/default/dune` for you):

```
DUNE_DB_TYPE=postgres
DUNE_DB_DSN=postgres://dune:password@127.0.0.1:5432/dune?sslmode=disable
```

### Migrating an existing SQLite install to PostgreSQL

```bash
dune migrate-db --dsn "postgres://dune:password@127.0.0.1:5432/dune?sslmode=disable"
# then set DUNE_DB_TYPE and DUNE_DB_DSN in /etc/default/dune and restart:
systemctl restart dune
```

The source SQLite file is left untouched; remove it manually once you have verified the new backend.

### Docker

The default `docker compose up -d` keeps using SQLite. To run with the bundled PostgreSQL service, uncomment the two `DUNE_DB_*` env lines in `docker-compose.yml` and start with the profile:

```bash
docker compose --profile postgres up -d
```

The image bundles Fail2ban (enabled by default) to enforce per-client **IP limits**. Fail2ban bans offenders with `iptables`, which requires the `NET_ADMIN` capability. `docker-compose.yml` already grants it via `cap_add`; if you start the container with `docker run` instead, add the capabilities yourself, otherwise bans are logged but never applied:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/leto217/DUNE
```

## Environment Variables

| Variable | Description | Default |
| --- | --- | --- |
| `DUNE_DB_TYPE` | Database backend: `sqlite` or `postgres` | `sqlite` |
| `DUNE_DB_DSN` | PostgreSQL connection string (when `DUNE_DB_TYPE=postgres`) | — |
| `DUNE_DB_FOLDER` | Directory for the SQLite database file | `/etc/dune` |
| `DUNE_DB_MAX_OPEN_CONNS` | Maximum open connections (PostgreSQL pool) | — |
| `DUNE_DB_MAX_IDLE_CONNS` | Maximum idle connections (PostgreSQL pool) | — |
| `DUNE_INIT_WEB_BASE_PATH` | The initial URI path for the web panel | `/` |
| `DUNE_ENABLE_FAIL2BAN` | Enable Fail2ban-based IP-limit enforcement | `true` |
| `DUNE_LOG_LEVEL` | Log verbosity (`debug`, `info`, `warning`, `error`) | `info` |
| `DUNE_DEBUG` | Enable debug mode | `false` |

## Supported Languages

The panel UI is available in 13 languages:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## Contributing

Contributions are welcome. Please read the [Contributing Guide](/CONTRIBUTING.md) before opening an issue or pull request.

## A Special Thanks to

- [alireza0](https://github.com/alireza0/)

## Acknowledgment

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**): _Enhanced v2ray/xray and v2ray/xray-clients routing rules with built-in Iranian domains and a focus on security and adblocking._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**): _This repository contains automatically updated V2Ray routing rules based on data on blocked domains and addresses in Russia._

## Community Tools

Tools and integrations built by the community around dune.

- [terraform-provider-dune](https://github.com/batonogov/terraform-provider-threexui) (License: **MIT**): _Manage inbounds, clients, panel settings, and Xray configuration as code with Terraform / OpenTofu._

## Support project

**If this project is helpful to you, you may wish to give it a**:star2:

| Network | Address |
| --- | --- |
| TON | `UQAa5FpNlK8Gp7tO8luJXHD-Sf0pPjJbNHGo8hdkyuUBhWEa` |
| TRON | `TLqtTfYSzPLFm8mtFDkSnXvzucxx7DS5VL` |
| ERC20 and BEP20 | `0x2fe632d70f4612b87670f8a28b4587ea2641452d` |

## Stargazers over Time

[![Stargazers over time](https://starchart.cc/leto217/DUNE.svg?variant=adaptive)](https://starchart.cc/leto217/DUNE)
