# 13. Feature File Map and Decision Log

## Why this exists

To preserve implementation intent and reduce rediscovery cost when modifying the custom client-center extension.

## Decision log (session)

1. Decision:
   Build client-center as orchestration layer instead of replacing inbound-native model.

   Reason:
   Lower break risk with existing email/inbound-centric traffic and client logic.

2. Decision:
   Keep existing inbounds/client APIs intact and add new API namespace.

   Reason:
   Backward compatibility for existing UI and operational scripts.

3. Decision:
   Add panic guards in Xray API integration points.

   Reason:
   Improve resilience in dev/runtime states where handler client may be unavailable.

## File map (custom extension)

Data models:
- `database/model/model.go`
  - `MasterClient`
  - `MasterClientInbound`

Migration registration:
- `database/db.go`

Backend service:
- `web/service/client_center.go`

Controller/API:
- `web/controller/client_center.go`
- `web/controller/api.go` (mounts `/panel/api/clients`)

UI route and page:
- `web/controller/xui.go` (`/panel/clients`)
- `web/html/clients.html`
- `web/html/component/aSidebar.html`

Stability fix:
- `xray/api.go`

Dev tooling:
- `.air.toml`
- `justfile`

## Future extension points

1. Add granular permissions for clients API.
2. Add audit logs table for master-client actions.
3. Add integration tests for sync edge-cases.
4. Add i18n keys for custom page labels/messages.
