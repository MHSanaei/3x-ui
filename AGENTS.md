# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Critical Architecture Patterns

**Telegram Bot Restart Pattern**: MUST call `service.StopBot()` before any server restart (SIGHUP or shutdown) to prevent Telegram bot 409 conflicts. This is critical in `main.go` signal handlers (lines 82-84, 120-122).

**Embedded Assets**: All web resources (HTML, CSS, JS, translations in `web/translation/`) are embedded at compile time using `//go:embed`. Changes to these files require full recompilation - no hot-reload available.

**Dual Server Design**: Main web panel and subscription server run concurrently, both managed by `web/global` package. Subscription server uses separate port.

**Database Seeder System**: Uses `HistoryOfSeeders` model to track one-time migrations (e.g., password bcrypt migration). Check this table before running migrations to prevent re-execution.

**Xray Integration**: Panel dynamically generates `config.json` from inbound/outbound settings and communicates via gRPC API (`xray/api.go`) for real-time traffic stats. Xray binary is platform-specific (`xray-{os}-{arch}`) and managed by installer scripts.

**Signal-Based Restart**: SIGHUP triggers graceful restart. Always stop Telegram bot first via `service.StopBot()`, then restart both web and sub servers.

## Build & Development Commands

```bash
# Build (creates bin/3x-ui.exe)
go build -o bin/3x-ui.exe ./main.go

# Run with debug logging
XUI_DEBUG=true go run ./main.go

# Test all packages
go test ./...

# Vet code
go vet ./...
```

**Production Build**: Uses CGO_ENABLED=1 with static linking via Bootlin musl toolchains for cross-platform builds (see `.github/workflows/release.yml`).

## Configuration & Environment

**Environment Variables**:
- `XUI_DEBUG=true` - Enable detailed debug logging
- `XUI_LOG_LEVEL` - Set log level (debug/info/notice/warning/error)
- `XUI_MAIN_FOLDER` - Override default installation folder
- `XUI_BIN_FOLDER` - Override binary folder (default: "bin")
- `XUI_DB_FOLDER` - Override database folder (default: `/etc/x-ui` on Linux)
- `XUI_LOG_FOLDER` - Override log folder (default: `/var/log/x-ui` on Linux)

**Database Path**: `config.GetDBPath()` returns `/etc/x-ui/x-ui.db` on Linux, current directory on Windows. GORM models auto-migrate on startup.

**Listen Address**: If inbound listen field is empty, defaults to `0.0.0.0` for proper dual-stack IPv4/IPv6 binding (see `database/model/model.go` lines 85-87).

## Project-Specific Patterns

**IP Limitation**: Implements "last IP wins" strategy. When client exceeds LimitIP, oldest connections are automatically disconnected via Xray API to allow newest IPs.

**Session Management**: Uses `gin-contrib/sessions` with cookie-based store for authentication.

**Internationalization**: Translation files in `web/translation/translate.*.toml`. Access via `I18nWeb(c, "key")` in controllers using `locale.I18nType` enum.

**Job Scheduling**: Uses `robfig/cron/v3` for periodic tasks (traffic monitoring, CPU checks, LDAP sync, IP tracking). Jobs registered in `web/web.go` during server initialization.

**Service Layer Pattern**: Services inject dependencies (like `xray.XrayAPI`) and operate on GORM models. Example: `InboundService` in `web/service/inbound.go`.

**Controller Pattern**: Controllers use Gin context (`*gin.Context`) and inherit from `BaseController`. Check auth via `checkLogin` middleware.

**Xray Binary Management**: Download platform-specific Xray binary to bin folder during installation. GeoIP/GeoSite rules downloaded from external repositories (Loyalsoldier, chocolate4u, runetfreedom).

## Gotchas

1. **Bot Restart**: Always stop Telegram bot before server restart to avoid 409 conflict
2. **Embedded Assets**: Changes to HTML/CSS/JS require recompilation
3. **Password Migration**: Seeder system tracks bcrypt migration - check `HistoryOfSeeders` table
4. **Port Binding**: Subscription server uses different port from main panel
5. **Xray Binary**: Must match OS/arch exactly - managed by installer scripts
6. **No Test Files**: Project currently has no `_test.go` files, though `go test ./...` is available
