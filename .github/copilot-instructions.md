# 3X-UI Development Guide

## Project Overview
3X-UI is a web-based control panel for managing Xray-core servers. It's a Go application using Gin web framework with embedded static assets and SQLite database. The panel manages VPN/proxy inbounds, monitors traffic, and provides Telegram bot integration.

## Architecture

### Core Components
- **main.go**: Entry point that initializes database, web server, and subscription server. Handles graceful shutdown via SIGHUP/SIGTERM signals
- **web/**: Primary web server with Gin router, HTML templates, and static assets embedded via `//go:embed`
- **xray/**: Xray-core process management and API communication for traffic monitoring
- **database/**: GORM-based SQLite database with models in `database/model/`
- **sub/**: Subscription server running alongside main web server (separate port)
- **web/service/**: Business logic layer containing InboundService, SettingService, TgBot, etc.
- **web/controller/**: HTTP handlers using Gin context (`*gin.Context`)
- **web/job/**: Cron-based background jobs for traffic monitoring, CPU checks, LDAP sync

### Key Architectural Patterns
1. **Embedded Resources**: All web assets (HTML, CSS, JS, translations) are embedded at compile time using `embed.FS`:
   - `web/assets` → `assetsFS`
   - `web/html` → `htmlFS`
   - `web/translation` → `i18nFS`

2. **Dual Server Design**: Main web panel + subscription server run concurrently, managed by `web/global` package

3. **Xray Integration**: Panel generates `config.json` for Xray binary, communicates via gRPC API for real-time traffic stats

4. **Signal-Based Restart**: SIGHUP triggers graceful restart. **Critical**: Always call `service.StopBot()` before restart to prevent Telegram bot 409 conflicts

5. **Database Seeders**: Uses `HistoryOfSeeders` model to track one-time migrations (e.g., password bcrypt migration)

## Development Workflows

### Building & Running
```bash
# Build (creates bin/3x-ui.exe)
go run tasks.json → "go: build" task

# Run with debug logging
XUI_DEBUG=true go run ./main.go
# Or use task: "go: run"

# Test
go test ./...
```

### Command-Line Operations
The main.go accepts flags for admin tasks:
- `-reset` - Reset all panel settings to defaults
- `-show` - Display current settings (port, paths)
- Use these by running the binary directly, not via web interface

### Database Management
- DB path: Configured via `config.GetDBPath()`, typically `/etc/x-ui/x-ui.db`
- Models: Located in `database/model/model.go` - Auto-migrated on startup
- Seeders: Use `HistoryOfSeeders` to prevent re-running migrations
- Default credentials: admin/admin (hashed with bcrypt)

### Telegram Bot Development
- Bot instance in `web/service/tgbot.go` (3700+ lines)
- Uses `telego` library with long polling
- **Critical Pattern**: Must call `service.StopBot()` before any server restart to prevent 409 bot conflicts
- Bot handlers use `telegohandler.BotHandler` for routing
- i18n via embedded `i18nFS` passed to bot startup

## Code Conventions

### Service Layer Pattern
Services inject dependencies (like xray.XrayAPI) and operate on GORM models:
```go
type InboundService struct {
    xrayApi xray.XrayAPI
}

func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error) {
    // Business logic here
}
```

### Controller Pattern
Controllers use Gin context and inherit from BaseController:
```go
func (a *InboundController) getInbounds(c *gin.Context) {
    // Use I18nWeb(c, "key") for translations
    // Check auth via checkLogin middleware
}
```

### Configuration Management
- Environment vars: `XUI_DEBUG`, `XUI_LOG_LEVEL`, `XUI_MAIN_FOLDER`
- Config embedded files: `config/version`, `config/name`
- Use `config.GetLogLevel()`, `config.GetDBPath()` helpers

### Internationalization
- Translation files: `web/translation/translate.*.toml`
- Access via `I18nWeb(c, "pages.login.loginAgain")` in controllers
- Use `locale.I18nType` enum (Web, Api, etc.)

## External Dependencies & Integration

### Xray-core
- Binary management: Download platform-specific binary (`xray-{os}-{arch}`) to bin folder
- Config generation: Panel creates `config.json` dynamically from inbound/outbound settings
- Process control: Start/stop via `xray/process.go`
- gRPC API: Real-time stats via `xray/api.go` using `google.golang.org/grpc`

### Critical External Paths
- Xray binary: `{bin_folder}/xray-{os}-{arch}`
- Xray config: `{bin_folder}/config.json`
- GeoIP/GeoSite: `{bin_folder}/geoip.dat`, `geosite.dat`
- Logs: `{log_folder}/3xipl.log`, `3xipl-banned.log`

### Job Scheduling
Uses `robfig/cron/v3` for periodic tasks:
- Traffic monitoring: `xray_traffic_job.go`
- CPU alerts: `check_cpu_usage.go`
- IP tracking: `check_client_ip_job.go`
- LDAP sync: `ldap_sync_job.go`

Jobs registered in `web/web.go` during server initialization

## Deployment & Scripts

### Installation Script Pattern
Both `install.sh` and `x-ui.sh` follow these patterns:
- Multi-distro support via `$release` variable (ubuntu, debian, centos, arch, etc.)
- Port detection with `is_port_in_use()` using ss/netstat/lsof
- Systemd service management with distro-specific unit files (`.service.debian`, `.service.arch`, `.service.rhel`)

### Docker Build
Multi-stage Dockerfile:
1. **Builder**: CGO-enabled build, runs `DockerInit.sh` to download Xray binary
2. **Final**: Alpine-based with fail2ban pre-configured

### Key File Locations (Production)
- Binary: `/usr/local/x-ui/`
- Database: `/etc/x-ui/x-ui.db`
- Logs: `/var/log/x-ui/`
- Service: `/etc/systemd/system/x-ui.service.*`

## Testing & Debugging
- Set `XUI_DEBUG=true` for detailed logging
- Check Xray process: `x-ui.sh` script provides menu for status/logs
- Database inspection: Direct SQLite access to x-ui.db
- Traffic debugging: Check `3xipl.log` for IP limit tracking
- Telegram bot: Logs show bot initialization and command handling

## Common Gotchas
1. **Bot Restart**: Always stop Telegram bot before server restart to avoid 409 conflict
2. **Embedded Assets**: Changes to HTML/CSS require recompilation (not hot-reload)
3. **Password Migration**: Seeder system tracks bcrypt migration - check `HistoryOfSeeders` table
4. **Port Binding**: Subscription server uses different port from main panel
5. **Xray Binary**: Must match OS/arch exactly - managed by installer scripts
6. **Session Management**: Uses `gin-contrib/sessions` with cookie store
7. **IP Limitation**: Implements "last IP wins" - when client exceeds LimitIP, oldest connections are automatically disconnected via Xray API to allow newest IPs
