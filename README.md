[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) |  [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

[![Release](https://img.shields.io/github/v/release/mhsanaei/3x-ui.svg)](https://github.com/MHSanaei/3x-ui/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg)](https://github.com/MHSanaei/3x-ui/actions)
[![GO Version](https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg)](https://github.com/MHSanaei/3x-ui/releases/latest)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![Go Reference](https://pkg.go.dev/badge/github.com/mhsanaei/3x-ui/v2.svg)](https://pkg.go.dev/github.com/mhsanaei/3x-ui/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/mhsanaei/3x-ui/v2)](https://goreportcard.com/report/github.com/mhsanaei/3x-ui/v2)

**3X-UI** — advanced, open-source web-based control panel designed for managing Xray-core server. It offers a user-friendly interface for configuring and monitoring various VPN and proxy protocols.

> [!IMPORTANT]
> This project is only for personal usage, please do not use it for illegal purposes, and please do not use it in a production environment.

As an enhanced fork of the original X-UI project, 3X-UI provides improved stability, broader protocol support, and additional features.

## Quick Start

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

For full documentation, please visit the [project Wiki](https://github.com/MHSanaei/3x-ui/wiki).

## Database Backends

3X-UI supports both `SQLite` and `PostgreSQL` as interchangeable backends. All application logic is written against [GORM](https://gorm.io/) — Go's database-agnostic ORM — so queries work identically on either engine. You can switch backends at any time through the panel UI without data loss.

### Choosing a Backend

| | SQLite | PostgreSQL |
|---|---|---|
| Setup | Zero config, file-based | Requires a running PG server |
| Best for | Single-node, low traffic | Multi-node, high concurrency |
| Backups | Portable + native file export | Portable export |

### Switching Backends (Panel UI)

1. Open **Settings → General → Database**.
2. Select a backend (`SQLite` or `PostgreSQL`) and fill in connection details.
3. Click **Test Connection** to verify.
4. Click **Switch Database** — the panel will:
   - Save a portable backup of current data automatically.
   - Migrate all data to the new backend.
   - Restart itself.

> The target database must be empty before switching. Use **Test Connection** before switching to catch misconfigurations early.

### Local PostgreSQL (panel-managed)

When selecting **Local (panel-managed)** mode, the panel installs and manages PostgreSQL automatically (Linux, root only):

```bash
# The panel uses postgres-manager.sh internally.
# No manual PostgreSQL setup required.
```

### External PostgreSQL

Point the panel at any existing PostgreSQL 13+ server:

1. Create a dedicated database and user.
2. Enter the connection details in Settings → Database.
3. Use **Test Connection**, then **Switch Database**.

### Environment Variable Override

For Docker and infrastructure-as-code deployments, set these environment variables to control the backend without touching the UI:

```bash
XUI_DB_DRIVER=postgres        # or: sqlite
XUI_DB_HOST=127.0.0.1
XUI_DB_PORT=5432
XUI_DB_NAME=x-ui
XUI_DB_USER=x-ui
XUI_DB_PASSWORD=change-me
XUI_DB_SSLMODE=disable        # or: require, verify-ca, verify-full
XUI_DB_MODE=external          # or: local
XUI_DB_PATH=/etc/x-ui/db/x-ui.db   # SQLite only
```

When any `XUI_DB_*` variable is set, the Database section in the panel UI becomes read-only.

### Backup & Restore

| Format | Works with | When to use |
|---|---|---|
| **Portable** (`.xui-backup`) | SQLite + PostgreSQL | Switching backends, Telegram bot backups, long-term storage |
| **Native SQLite** (`.db`) | SQLite only | Quick raw file backup while on SQLite |

- The Telegram bot sends a **portable backup** automatically — this works regardless of which backend is active.
- Portable backups can be imported back on either SQLite or PostgreSQL.
- Legacy `.db` files from older 3x-ui versions can be imported even while PostgreSQL is active.

## A Special Thanks to

- [alireza0](https://github.com/alireza0/)

## Acknowledgment

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**): _Enhanced v2ray/xray and v2ray/xray-clients routing rules with built-in Iranian domains and a focus on security and adblocking._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**): _This repository contains automatically updated V2Ray routing rules based on data on blocked domains and addresses in Russia._

## Support project

**If this project is helpful to you, you may wish to give it a**:star2:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## Stargazers over Time

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)
