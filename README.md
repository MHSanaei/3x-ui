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

## TrustTunnel Support

This fork adds support for [TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) — a fast VPN protocol by AdGuard, written in Rust. TrustTunnel runs as a separate process alongside Xray and can be managed through the same panel UI.

### How it works

- TrustTunnel appears as a protocol option (`trusttunnel`) when creating a new inbound
- Each TrustTunnel inbound runs its own process independently of Xray
- Supports multiple clients with username/password authentication
- Supports HTTP/1.1, HTTP/2, and QUIC (HTTP/3) transports
- Uses its own TLS certificates (configured per inbound)

### Installation

1. Install the TrustTunnel binary:

```bash
curl -fsSL https://raw.githubusercontent.com/TrustTunnel/TrustTunnel/refs/heads/master/scripts/install.sh | sh -s --
```

2. Install/update 3x-ui from this fork:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yelloduxx/3x-ui/master/install.sh)
```

3. In the panel, create a new inbound and select `trusttunnel` as the protocol

### Configuration

When creating a TrustTunnel inbound, you need to specify:

| Field | Description |
|-------|-------------|
| **Port** | Listen port (e.g., 443) |
| **Hostname** | Domain name for TLS SNI matching |
| **Certificate Path** | Path to TLS certificate chain (PEM) |
| **Private Key Path** | Path to TLS private key (PEM) |
| **Transport Protocols** | Enable/disable HTTP/1.1, HTTP/2, QUIC |
| **Clients** | Username/password pairs for authentication |

### Upgrading 3x-ui

TrustTunnel integration is designed for minimal merge conflicts:

- **3 new files** are fully isolated (`trusttunnel/process.go`, `web/service/trusttunnel.go`, `web/html/form/protocol/trusttunnel.html`)
- Changes to existing files are small and clearly marked
- No database schema changes — settings are stored as JSON in the existing `Settings` field

## Quick Start

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yelloduxx/3x-ui/master/install.sh)
```

For full documentation, please visit the [project Wiki](https://github.com/MHSanaei/3x-ui/wiki).

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
