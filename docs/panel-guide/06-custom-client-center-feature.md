# 06. Custom Client-Center Feature

## Goal

Add a true operator-friendly client-first page while preserving 3x-ui compatibility and minimizing break risk.

## Implemented design

A new orchestration layer was added:
- Master client profile table
- Master client <-> inbound assignment mapping table
- Sync logic that creates/updates/removes native inbound clients automatically

## Added backend pieces

Models:
- `MasterClient`
- `MasterClientInbound`

Service:
- `web/service/client_center.go`

Controller/API:
- `web/controller/client_center.go`
- Mounted in `web/controller/api.go` under `/panel/api/clients/*`

## Added frontend pieces

- Route: `/panel/clients`
- Page: `web/html/clients.html`
- Sidebar item: `web/html/component/aSidebar.html`

## API surface

- `GET /panel/api/clients/list`
- `GET /panel/api/clients/inbounds`
- `POST /panel/api/clients/add`
- `POST /panel/api/clients/update/:id`
- `POST /panel/api/clients/del/:id`

## Behavior notes

- Supports multi-client protocols (vless/vmess/trojan/shadowsocks).
- Assignment email is generated uniquely per inbound.
- Master profile fields are synced to assigned inbound client entries.
- Detach can fail if an inbound would be left with zero clients (inherited safety rule).

## Stability fix included

`xray/api.go` now guards nil handler client and returns clear errors in:
- `AddInbound`
- `DelInbound`
- `AddUser`
- `RemoveUser`

This prevents nil-pointer panics during local/dev conditions.

## Files touched (feature)

- `database/model/model.go`
- `database/db.go`
- `web/service/client_center.go`
- `web/controller/client_center.go`
- `web/controller/api.go`
- `web/controller/xui.go`
- `web/html/clients.html`
- `web/html/component/aSidebar.html`
- `xray/api.go`
