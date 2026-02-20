# Maintainer Architecture Notes

## Settings
- Authoritative settings storage is now `app_settings` (single typed row).
- Legacy `settings` key/value table remains as temporary compatibility fallback.
- `SettingService` is the single entry point for reads/writes.
- During compatibility window:
  - reads prefer typed storage
  - writes update typed storage and shadow-write legacy key/value rows

## Subscription URL Generation
- Canonical URL generation lives in `web/service/subscription_urls.go`.
- Both flows must use this builder:
  - sub server endpoint rendering (`sub/subService.go`)
  - Telegram bot subscription links (`web/service/tgbot_subscription.go`)
- Avoid introducing URL construction logic directly in controllers/services.

## Service File Layout
- Large services are split by concern while keeping package boundaries stable.
- `web/service/tgbot.go`: lifecycle, command handling core.
- `web/service/tgbot_subscription.go`: subscription link/QR behavior.
- `web/service/inbound.go`: core inbound CRUD/traffic/migration.
- `web/service/inbound_client_mutation.go`: repeated client mutation flows.

## Web/Sub Listener Bootstrapping
- TLS listener wrapping is centralized in `serverutil/listener.go`.
- Both `web/web.go` and `sub/sub.go` must use it to avoid divergence.

## Frontend Path Normalization
- Shared path utilities are in `web/assets/js/util/index.js` (`PathUtil`).
- Use `PathUtil.normalizePath` instead of inline ad-hoc normalization in templates.
