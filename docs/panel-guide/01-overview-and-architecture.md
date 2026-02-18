# 01. Overview and Architecture

## What 3x-ui is

3x-ui is a control panel around Xray-core for:
- Inbound and client management
- Quota/expiry/statistics
- Advanced Xray config/routing editing
- Operational actions (restart, logs, backup/import/export)
- Optional subscription and Telegram integrations

## Core runtime parts

- Web panel server: `web/web.go`
- Subscription server: `sub/sub.go`
- Xray integration service: `web/service/xray.go`
- Binary entrypoint: `main.go`

## Data layer

Default storage is SQLite via GORM.
Models include:
- `User`
- `Inbound`
- `Setting`
- `OutboundTraffics`
- `InboundClientIps`
- `xray.ClientTraffic`
- `HistoryOfSeeders`
- Custom extension: `MasterClient`, `MasterClientInbound`

Files:
- `database/db.go`
- `database/model/model.go`
- `xray/client_traffic.go`

## Auth/session model

- Cookie session with secret from DB settings
- `/panel/*` requires login
- `/panel/api/*` also requires login
- Unauthorized API requests are intentionally hidden with 404 behavior

Files:
- `web/controller/base.go`
- `web/controller/api.go`
- `web/session/session.go`

## Background jobs and periodic logic

Cron-based tasks include:
- Xray process health checks
- Traffic collection from Xray
- Auto disable/renew logic
- Client IP checks and cleanup
- Optional LDAP and Telegram jobs

Files:
- `web/web.go`
- `web/job/*.go`
- `web/service/inbound.go`

## URL topology

UI:
- `/`
- `/panel/`
- `/panel/inbounds`
- `/panel/clients` (custom)
- `/panel/settings`
- `/panel/xray`

API:
- `/panel/api/inbounds/*`
- `/panel/api/server/*`
- `/panel/api/clients/*` (custom)
- `/panel/xray/*`
- `/panel/setting/*`

Subscription:
- Dynamic from settings (`subPath`, `subJsonPath`)
