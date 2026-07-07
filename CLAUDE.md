# CLAUDE.md

Operational guide for AI agents working in this repo. Long-form human docs:
`CONTRIBUTING.md` (setup, testing philosophy) and `frontend/README.md`.
Read those before large changes. This file is the short, must-follow version.
For a deep navigation map (request lifecycle, cron-job table, symptom → file
index, layering rules), read `docs/architecture.md` on demand — do not guess
file locations when it can answer in one hop.

## Stack
- Backend: Go 1.26 (`module github.com/mhsanaei/3x-ui/v3`), Gin, GORM.
  Runs Xray-core as a managed child process (`internal/xray/process.go`) and
  imports `github.com/xtls/xray-core` for config types + gRPC stats/handler/router
  API. MTProto inbounds run a second managed child — the `mtg-multi` binary
  (`github.com/mhsanaei/mtg-multi`, a multi-secret fork built from source;
  `internal/mtproto/`) — outside Xray, one process per inbound serving each
  client's FakeTLS secret via the fork's `[secrets]` section (plus per-client
  ad-tags via `[secret-ad-tags]`). Client and ad-tag edits are hot-applied
  through the fork's management API (`PUT /secrets`, bearer-token guarded) so
  connections survive; the manager falls back to a process restart on older
  binaries.
- Storage: SQLite by default (`/etc/x-ui/x-ui.db` on Linux; the executable dir on
  Windows), PostgreSQL optional (`XUI_DB_TYPE` / `XUI_DB_DSN`). The CGo SQLite
  driver (`mattn/go-sqlite3`) needs a C compiler — `CGO_ENABLED=0` builds fail.
- Frontend: React 19 + Ant Design 6 + Vite 8 + TypeScript in `frontend/`,
  built into `internal/web/dist/` (gitignored) and embedded via `embed.FS`.

## Repo map
- `main.go` — entry point + `x-ui` CLI (run, migrate, migrate-db, setting, cert).
- `internal/config/` — env parsing (XUI_DEBUG, XUI_LOG_LEVEL, XUI_LOG_FOLDER,
  XUI_BIN_FOLDER, XUI_SKIP_HSTS, XUI_PORT, XUI_DB_*).
- `internal/database/` + `internal/database/model/` — GORM schema (Inbound,
  Client, Setting, User), inbound Protocol enum, AutoMigrate + hand-written
  migrations in `db.go`.
- `internal/xray/` — Xray child-process lifecycle, config generation, gRPC API.
- `internal/mtproto/` — MTProto inbounds via the bundled `mtg-multi` binary.
- `internal/sub/` — subscription server (raw / JSON / Clash).
- `internal/eventbus/` — in-process pub/sub (outbound/node health, xray.crash,
  cpu.high, memory.high, login.attempt).
- `internal/logger/`, `internal/util/` (link, crypto, sys, ldap, …),
  `internal/tunnelmonitor/` — shared infrastructure.
- `internal/web/` — Gin server (embeds `dist/` + `translation/`).
  - `controller/` — panel + REST API handlers; OpenAPI at /panel/api/openapi.json.
  - `service/` — business logic (InboundService, SettingService, XrayService,
    node sync); subpackages tgbot/, email/, outbound/, panel/, integration/.
  - `job/` — cron jobs (traffic, fail2ban IP-limit, node heartbeat/sync, LDAP).
  - `middleware/`, `entity/`, `global/`, `session/` (CSRF), `network/`,
    `runtime/` (master/sub-node over mTLS), `websocket/`.
  - `locale/` + `translation/` — i18n, 13 embedded locale JSON files.
- `frontend/` — React + TS source (see `frontend/CLAUDE.md`).
- `tools/openapigen/` — Go generator that emits frontend types + Zod/JSON schemas
  into `frontend/src/generated/` from Go structs. The OpenAPI doc itself
  (`frontend/public/openapi.json`) is assembled from those + `endpoints.ts` by
  `frontend/scripts/build-openapi.mjs`.

## Hard rules (non-negotiable)
- NO `//` line comments in committed Go/TS. Names carry meaning; rename instead
  of annotating. Exempt: `//go:build`, `//go:generate`, and other directives.
  HTML `<!-- -->` is fine. (A linter cannot enforce this — you must.)
- New `g.POST`/`g.GET` in `internal/web/controller/` REQUIRES a matching entry
  in `frontend/src/pages/api-docs/endpoints.ts`, then `make gen` (or
  `cd frontend && npm run gen`). It is a hand-maintained registry — nothing checks
  it against the Go routes, so an omitted route silently vanishes from the docs.
- Response examples come from Go struct `example:` tags via `tools/openapigen` —
  never hand-write them. A new struct must be added to openapigen's `StructAllow`
  allowlist (`tools/openapigen/main.go`) or it is silently omitted from
  schemas/examples (and `build-openapi.mjs` then fails on the missing schema).
- A new English i18n key must be added to EVERY locale JSON in
  `internal/web/translation/` (13 files). Missing keys fall back to en-US (or
  render the raw key if absent there too); nothing fails the build, so they are
  easy to miss.
- DB / model changes require a migration in `internal/database/db.go`.
- Conventional-commit prefixes (`feat`, `fix`, `refactor`, `chore`, `docs`,
  `style`): `<area>: short imperative summary`, then a body explaining the why.

## Go conventions
- Stdlib `testing` only (no testify). Table-driven, `t.Run` subtests,
  `t.Helper()` on helpers. Assert the exact value / typed error / emitted
  string, never just `err != nil`. Prefer real deps over mocks: throwaway DB via
  `database.InitDB(filepath.Join(t.TempDir(), "x-ui.db"))` +
  `t.Cleanup(func() { _ = database.CloseDB() })`; `httptest` for HTTP.
  `internal/sub`'s `initSubDB(t)` is the template.
- Code must pass `golangci-lint run` (gofumpt + goimports formatting): `make lint`.

## Frontend conventions (summary; full version in frontend/CLAUDE.md)
- Ant Design 6 only — no Tailwind/shadcn. Targeted tweaks, not rewrites.
- TS strict; `@typescript-eslint/no-explicit-any` is an error. Zod schemas in
  `src/schemas/` are the source of truth; infer types with `z.infer`, never
  hand-write. Do not edit `src/generated/`.
- Editing `frontend/src` does NOT change what users see until the Vite build is
  regenerated into `internal/web/dist/`. In `XUI_DEBUG=true`, HTML is served from
  the frozen embedded FS but JS/CSS off disk — after `npm run build` you MUST
  restart `go run .` or you get a blank page with 404s.
- After touching share-link logic (`src/lib/xray/`), run `npm run test` (golden
  fixtures); regenerate snapshots (`npx vitest run -u`) only for intentional
  output changes, never to make a red test green.

## Build, test, verify
Run `make help` for all targets. The full local gate that mirrors CI:

    make verify

Common targets: `make gen` (regenerate Zod/OpenAPI), `make lint` (Go + frontend),
`make test` (Go `-shuffle=on` + frontend), `make race`, `make build`. See `Makefile`.

## Definition of done (before opening a PR)
1. `make gen` and confirm `git diff` on `frontend/src/generated` +
   `frontend/public/openapi.json` is clean.
2. `make verify` passes.
3. Diff is focused; refactors are separate from feature work.
