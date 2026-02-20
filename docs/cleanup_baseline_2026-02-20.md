# Cleanup Baseline (2026-02-20)

This file captures the cleanup baseline and post-refactor validation checkpoints used for the DRY/KISS consolidation effort.

## Baseline Snapshot (before implementation)
- `go build ./...`: PASS
- `go test ./...`: no test files in repository
- Go source files: 71
- Go test files: 0
- Total Go LOC: 19,882
- `gocyclo` average complexity: 4.99
- Highest hotspots:
  - `web/service/tgbot.go` (~3,840 LOC)
  - `web/service/inbound.go` (~2,510 LOC)
  - `sub/subService.go` (~1,199 LOC)

## Current Snapshot (after implementation)
- `go test ./...`: PASS
- `go vet ./...`: PASS
- `go build ./...`: PASS
- `staticcheck ./...`: PASS
- Go source files: 84
- Go test files: 6
- Total Go LOC: 20,434
- `gocyclo` average complexity: 4.87

## Structural Changes Captured
- Added typed single-row settings model: `app_settings`.
- Added startup migration path from legacy key/value settings to typed settings.
- Kept legacy key/value reads as compatibility fallback and shadow writes for one release window.
- Added shared subscription URL builder used by both sub server and Telegram bot flows.
- Split large service files into focused units:
  - `web/service/tgbot_subscription.go`
  - `web/service/inbound_client_mutation.go`
  - `web/service/settings_repository.go`
- Unified duplicated TLS listener wrapping between web/sub servers via `serverutil`.

## Public Surface Guardrails
- CLI commands kept stable:
  - `run`
  - `migrate`
  - `setting`
  - `cert`
- Route families kept stable:
  - `/panel/*`
  - `/panel/api/*`
  - subscription paths (`subPath`, `subJsonPath`) remain configurable via settings

## Regression Guardrails Added
- Settings migration tests.
- SettingService typed storage + shadow-write behavior tests.
- Shared subscription URL generation tests.
- Inbound client mutation helper tests.
- Route/auth smoke tests.
- Command-level smoke checks with temp DB/log folders:
  - `go run . setting ...`
  - `go run . setting -show`
  - `go run . migrate`
- Runtime HTTP smoke (temp DB, app process started/stopped):
  - `GET /` -> `200`
  - `GET /panel/` -> `307`
  - `GET /panel/inbounds` -> `307`
  - `GET /panel/settings` -> `307`
  - `GET /panel/xray` -> `307`
  - `GET /sub/non-existent-id` on panel port (`2099`) -> `404` (expected; route is on sub server)
  - `GET /sub/non-existent-id` on sub port (`2096`) -> `400`
