# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development commands

- Build the app: `go build -o bin/3x-ui.exe ./main.go`
- Run locally with debug logging: `XUI_DEBUG=true go run ./main.go`
- Run tests: `go test ./...`
- Run vet: `go vet ./...`
- Run a single package test suite: `go test ./path/to/package`
- Run a single test: `go test ./path/to/package -run TestName`
- Show CLI help / subcommands: `go run ./main.go --help`
- Show version: `go run ./main.go -v`

VS Code tasks mirror the common Go workflows:
- `go: build`
- `go: run`
- `go: test`
- `go: vet`

## Runtime shape

This is a Go monolith for managing Xray-core, with two Gin-based HTTP servers started from `main.go`:
- the main panel server in `web/`
- the subscription server in `sub/`

`main.go` initializes the SQLite database, starts both servers, and handles process signals:
- `SIGHUP` restarts the panel + subscription servers
- `SIGUSR1` restarts xray-core only

Important: before full shutdown or SIGHUP restart, the Telegram bot is stopped explicitly via `service.StopBot()` to avoid Telegram 409 conflicts.

## High-level architecture

### Database and settings

- `database/db.go` initializes GORM with SQLite, runs auto-migrations, seeds the default admin user, and runs one-time seeders.
- Models live in `database/model/model.go`.
- App configuration is heavily database-backed through the `settings` table rather than static config files.
- `HistoryOfSeeders` is used to track one-time migrations such as password hashing changes.

### Web panel

- `web/web.go` builds the main Gin engine, session middleware, gzip, i18n, static asset serving, template loading, websocket hub setup, and background cron jobs.
- Controllers are in `web/controller/`.
- Business logic lives in `web/service/`.
- Background tasks live in `web/job/`.
- The websocket hub is in `web/websocket/` and is wired from `web/web.go`.

### Subscription server

- `sub/sub.go` starts a separate Gin server for subscription links and JSON subscriptions.
- It has its own listen/port/cert settings and can run independently of the main panel routes.
- It reuses embedded templates/assets from `web/` and applies subscription-specific path/domain settings from the database.

### Xray integration

- `xray/` is the bridge to xray-core.
- `xray/process.go` writes `config.json`, launches the platform-specific xray binary, tracks process state, and handles stop/restart behavior.
- `xray/api.go`, `xray/traffic.go`, and related files handle API access and traffic/stat collection.
- The panel treats xray-core as a managed subprocess and periodically monitors/restarts it from cron jobs in `web/web.go`.

### Frontend delivery model

- The UI is server-rendered HTML templates plus embedded static assets under `web/html/` and `web/assets/`.
- In production, templates/assets are embedded with `go:embed` in `web/web.go`.
- In debug mode (`XUI_DEBUG=true`), templates and assets are loaded from disk, so edits under `web/html/` and `web/assets/` are reflected without rebuilding embedded resources.
- Internationalization files live in `web/translation/*.toml` and are initialized by `web/locale`.

## Background jobs and long-running behavior

`web/web.go` registers the operational jobs that keep the panel in sync with xray-core. These include:
- xray process health checks
- deferred/statistical traffic collection
- client IP checks / log maintenance
- periodic traffic reset jobs
- optional LDAP sync
- optional Telegram notification and CPU alert jobs

When changing settings or services that affect runtime behavior, check whether a cron job, websocket update, or xray restart path also needs to change.

## Repo-specific conventions and gotchas

- Default credentials are seeded as `admin` / `admin`, but stored hashed in the DB.
- The app uses DB settings extensively; many behavior changes require updating `SettingService`, not just editing route/controller code.
- The `Inbound` model stores much of the Xray config as JSON strings (`Settings`, `StreamSettings`, `Sniffing`), then converts those into xray config structs.
- The main panel and subscription server have separate listen/port/cert/base-path concepts. Keep them distinct when changing routing or TLS behavior.
- Session handling uses `gin-contrib/sessions` with a cookie store and secret loaded from settings.
- The subscription server intentionally runs Gin in release mode and discards Gin default writers.
- There are currently no `*_test.go` files in the repo, so `go test ./...` mainly validates buildability of packages.

## Important files to orient quickly

- `main.go` — process entrypoint, CLI subcommands, signal handling
- `web/web.go` — main server wiring, embedded assets/templates, cron jobs
- `sub/sub.go` — subscription server wiring
- `database/db.go` — DB init, migrations, seeders
- `database/model/model.go` — core persistent models
- `web/service/setting.go` — central behavior/settings access point
- `web/service/inbound.go` and `web/service/xray.go` — panel logic tied to xray config/runtime
- `xray/process.go` — xray subprocess management

## Existing repo guidance carried forward

From `.github/copilot-instructions.md` and current code structure:
- Treat the project as a Go + Gin + SQLite application with embedded web assets.
- Remember the dual-server design: main panel plus subscription server.
- Preserve the Telegram bot shutdown-before-restart behavior.
- If working on deployment or container behavior, note that Docker support exists via `Dockerfile`, `DockerInit.sh`, `DockerEntrypoint.sh`, and `docker-compose.yml`.
