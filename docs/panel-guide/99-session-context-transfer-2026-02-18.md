# 99. Session Context Transfer (2026-02-18)

## Purpose

This is a handoff snapshot of all major context and decisions from the collaboration session, so future work can continue without re-discovery.

## Repository and research context

- Repo analyzed: `MHSanaei/3x-ui`
- Wiki reference used: `MHSanaei/3x-ui/wiki`
- Existing long guide maintained and expanded: `PANEL_GUIDE_SOURCE_OF_TRUTH.md`
- Focused guide set added under: `docs/panel-guide/`

## Live panel operational context captured

Panel URL pattern used in session:
- `http://127.0.0.1:2053/panel/`

Observed production-style inbounds in your panel during cleanup:
- `reality-443` (renamed)
- `reality-8443` (renamed)

Additional test inbound created by request:
- VLESS TCP with HTTP obfuscation style (`Host speedtest.com`, path `/`) on `18080`

Final cleaned naming applied in live panel:
- `vless-reality-tcp-443-main`
- `vless-reality-tcp-8443-alt`
- `vless-tcp-http-18080-test`

## Core product-model conclusion

Native 3x-ui is inbound-first:
- Clients are managed per inbound.
- There is no built-in global client object with many-to-many inbound assignment UX.

This drove implementation choice:
- Build a client-center orchestration layer, not a deep replacement of native inbound client model.

## Custom feature delivered

Implemented centralized client-management extension:
- New page: `/panel/clients`
- New API group: `/panel/api/clients/*`
- New DB models for master profile + inbound assignments
- Sync engine to create/update/remove underlying inbound clients

Files created/updated for feature:
- `web/service/client_center.go`
- `web/controller/client_center.go`
- `web/html/clients.html`
- `database/model/model.go`
- `database/db.go`
- `web/controller/api.go`
- `web/controller/xui.go`
- `web/html/component/aSidebar.html`

## Additional engineering improvements

1. Air setup:
- Added `.air.toml` for live-reload development.
- Keeps DB/log/bin in `tmp/`.

2. Justfile setup:
- Added `justfile` with common run/build/air/api helper commands.

3. Runtime stability guard:
- Added nil-handler checks in `xray/api.go` (`AddInbound`, `DelInbound`, `AddUser`, `RemoveUser`) to avoid panic and return explicit errors.

## Local validation that was performed

Local dev run used repo-local paths:
- DB: `tmp/db`
- Logs: `tmp/logs`
- Port: `2099`

Validated flows:
- Build success (`go build ./...`)
- Panel startup in debug/dev mode
- Login and authenticated API access
- Custom clients API CRUD and assignment sync
- `/panel/clients` UI render and create flow via browser automation

## Known caveats from session

- A large set of `.playwright-cli` artifacts exists in working tree; this was intentionally not cleaned to avoid touching unrelated state.
- If Xray handler service is unavailable in dev conditions, inbound operations may return explicit errors (now handled) rather than panic.

## Recommended next actions

1. Add automated integration tests for client-center sync scenarios.
2. Add role/audit controls for client-center APIs in multi-admin environments.
3. Decide whether to keep monolithic guide as archive-only and rely on split docs for active maintenance.
4. If desired, add i18n labels for new `Clients` UI text to align with full localization style.

## Sensitive data handling note

Credentials used during interactive operations were user-provided in chat and are intentionally not persisted in this file.
