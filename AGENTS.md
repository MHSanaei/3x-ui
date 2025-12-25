# AGENTS.md (3x-ui)

This repo is **primarily Go**: a Gin-based web panel that manages an **Xray-core** process and stores state in a **SQLite** DB.

## Quick commands (repo root)

- **Run (dev, no root needed)**:
  - Ensure writable paths:
    - `mkdir -p ./x-ui ./bin ./log`
    - `export XUI_DB_FOLDER="$(pwd)/x-ui"`
    - `export XUI_BIN_FOLDER="$(pwd)/bin"`
    - `export XUI_LOG_FOLDER="$(pwd)/log"`
    - `export XUI_DEBUG=true` (loads templates/assets from disk; see `web/AGENTS.md`)
  - Start: `go run .`
  - Panel defaults (fresh DB): `http://localhost:2053/` with **admin/admin**

- **Build**: `go build -ldflags "-w -s" -o build/x-ui main.go`
- **Format**: `gofmt -w .`
- **Tests**: `go test ./...`
- **Basic sanity**: `go vet ./...`

## Docker

- **Compose**: `docker compose up --build`
  - Uses `network_mode: host` and mounts:
    - `./db/` → `/etc/x-ui/` (SQLite DB lives at `/etc/x-ui/x-ui.db`)
    - `./cert/` → `/root/cert/`

## Layout / where things live

- **Entry point**: `main.go` (starts the web server + subscription server; handles signals)
- **Config**: `config/` (env-driven defaults; DB path, bin path, log folder)
- **DB (SQLite via GORM)**: `database/` (+ `database/model/`)
- **Web panel**: `web/` (Gin controllers, templates, embedded assets, i18n)
- **Subscription server**: `sub/`
- **Xray process management**: `xray/` (binary path naming, config/log paths, process wrapper)
- **Operational scripts**: `install.sh`, `update.sh`, `x-ui.sh` (production/admin tooling; be cautious editing)

## Important environment variables

- **`XUI_DEBUG=true`**: enables dev behavior (Gin debug + loads `web/html` and `web/assets` from disk).
- **`XUI_DB_FOLDER`**: DB directory (default: `/etc/x-ui` on non-Windows). DB file is `<folder>/x-ui.db`.
- **`XUI_BIN_FOLDER`**: where Xray binary + `config.json` + `geo*.dat` live (default: `bin`).
- **`XUI_LOG_FOLDER`**: log directory (default: `/var/log` on non-Windows).
- **`XUI_LOG_LEVEL`**: `debug|info|notice|warning|error`.

## Agent workflow guidelines

- **Prefer small, surgical changes**: this is a production-oriented project (panel + system scripts).
- **Don’t run** `install.sh` / `update.sh` in dev automation: they expect **root** and mutate the system.
- **When touching templates/assets**: ensure it works in both **debug** (disk) and **production** (embedded) modes.
- **Security**: treat any change in `web/controller`, `web/service`, and shell scripts as security-sensitive.


