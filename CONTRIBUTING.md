# Contributing

Thanks for taking the time to contribute to 3x-ui. This guide gets a development panel running on your machine in a few minutes.

## Prerequisites

- **Go 1.26+** (the version in `go.mod`)
- **Node.js 22+** and npm (for the Vue frontend)
- **Git**
- **A C compiler** — required by the CGo SQLite driver (`github.com/mattn/go-sqlite3`). Linux/macOS already ship one; on Windows see below.

### Windows: MinGW-w64

`go build` on Windows will fail with `cgo: C compiler "gcc" not found` until you install a GCC toolchain. Two options — pick whichever fits.

**Option A — standalone zip (fastest, no package manager)**

1. Grab the latest build from **<https://github.com/niXman/mingw-builds-binaries/releases>**. For most setups you want a release named like:
   ```
   x86_64-<version>-release-posix-seh-ucrt-rt_<n>-rev<m>.7z
   ```
   (64-bit, POSIX threads, SEH exceptions, UCRT runtime — matches the modern Windows defaults.)
2. Extract it somewhere stable, e.g. `C:\mingw64\`.
3. Add `C:\mingw64\bin` to your **Windows** `PATH` (System Properties → Environment Variables → Path → New).
4. Open a fresh terminal and confirm:
   ```powershell
   gcc --version
   ```

**Option B — MSYS2 (if you also want a Unix-y shell)**

1. Install MSYS2 from <https://www.msys2.org/>.
2. Open the **MSYS2 UCRT64** shell from the Start menu and update once:
   ```bash
   pacman -Syu
   ```
3. Install the UCRT64 toolchain:
   ```bash
   pacman -S --needed mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkg-config
   ```
4. Add `C:\msys64\ucrt64\bin` to your Windows `PATH`.
5. Verify with `gcc --version` in a fresh terminal.

After either, `go build ./...` and `go run .` work normally.

> Why MinGW-w64 over MSVC: `mattn/go-sqlite3` officially supports GCC, builds are faster on Windows, and the toolchain doesn't lock you into a Visual Studio install. If you already have Visual Studio Build Tools installed it works too — just make sure `CC=cl` is **not** set in your environment.

The Linux SQLite cross-build from Windows (or vice versa) needs an extra cross-compiler — out of scope here; build natively on the target OS.

## First-time setup

```bash
git clone https://github.com/MHSanaei/3x-ui.git
cd 3x-ui

cp .env.example .env

mkdir x-ui

go mod download

cd frontend
npm install
npm run build
cd ..
```

`.env.example` ships with sane defaults that point the database, logs, and xray binary at the local `x-ui/` folder so nothing escapes the project directory:

```
XUI_DEBUG=true
XUI_DB_FOLDER=x-ui
XUI_LOG_FOLDER=x-ui
XUI_BIN_FOLDER=x-ui
```

You need to drop the xray binary (`xray-windows-amd64.exe` on Windows, `xray-linux-amd64` on Linux, etc.) plus the matching `geoip.dat` / `geosite.dat` files into `x-ui/`. The easiest source is a [released Xray-core build](https://github.com/XTLS/Xray-core/releases). On Windows you also want `wintun.dll` if you plan to test TUN inbounds.

## Running

```bash
go run .
```

Open [http://localhost:2053](http://localhost:2053) and log in with `admin` / `admin`. You will be prompted to change the credentials on first login.

### Inside VS Code

The repo ships a launch profile in `.vscode/launch.json` (gitignored — copy from the snippet below if it is missing):

```jsonc
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Run 3x-ui (Debug)",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}",
      "cwd": "${workspaceFolder}",
      "env": {
        "XUI_DEBUG": "true",
        "XUI_DB_FOLDER": "x-ui",
        "XUI_LOG_FOLDER": "x-ui",
        "XUI_BIN_FOLDER": "x-ui"
      },
      "console": "integratedTerminal"
    }
  ]
}
```

## Working on the frontend

The panel UI is a Vue 3 + Ant Design Vue 4 app under `frontend/`. A few things worth knowing before you dive in.

### Architecture in one paragraph

It's a **multi-page app**, not a SPA. Every panel route (`/panel`, `/panel/inbounds`, `/panel/clients`, `/panel/xray`, `/panel/settings`, `/panel/sub`, `/panel/api-docs`, plus `login`) has its own HTML entry under `frontend/*.html` and its own bootstrap in `src/entries/<page>.js`. Vite builds them into `web/dist/`, and the Go binary embeds that directory at compile time with `embed.FS`. Each navigation triggers a real document load — but each page's bundle is small, so it stays snappy. There's no Vue Router and no central store; Vuex/Pinia were rejected as overkill for the panel's surface area.

### State and data flow

- **No global store.** State lives where it's used. Cross-page data (settings, current user, theme) is re-fetched on each page load — the backend is on the same box and responses are cheap.
- **Composables** in `src/composables/` carry reactive logic worth sharing inside a page (theme switching, status polling, node lists). Reach for one before adding a new global.
- **Domain classes** in `src/models/` (`Inbound`, `DBInbound`, `Outbound`, `Status`, …) own the protocol-specific logic — link generation, settings JSON shape, TLS/Reality stream handling. The Vue components stay dumb; they ask the model "what's my link?" and render the answer.
- **HTTP** goes through `src/utils/index.js`'s `HttpUtil`, which is a thin Axios wrapper with CSRF, response toast handling, and a `silent: true` opt-out for bulk operations that would otherwise spam toasts.

### i18n

Locale strings live in `web/translation/<locale>.json`, not under `frontend/`. The Go side embeds the same JSON and serves it to both backend templates and `vue-i18n`. When you add a new English key, add it to **every** non-English locale too — missing keys don't fail the build, they just render the raw key in the UI.

### Two dev workflows

| When you want… | Use |
|----------------|-----|
| To iterate on UI tweaks fast | `cd frontend && npm run dev` (Vite on `:5173`, proxies `/panel/*` and `/api/*` to the Go panel on `:2053`). Start the Go panel first. |
| To test what users actually see | `cd frontend && npm run build`, then `go run .`. The Go binary serves the built bundle either embedded (release mode) or from disk (debug mode). |

The Vite dev proxy auto-rewrites the sidebar's production-style links (`/panel`, `/panel/inbounds`, `/panel/clients`, etc.) to the matching Vite-served HTML, so the navigation feels identical to prod without round-tripping through Go. The route allowlist lives in `MIGRATED_ROUTES` in `vite.config.js` — if you add a new page, register it there too.

> **`XUI_DEBUG=true` gotcha** — in debug mode the panel serves HTML out of the embedded FS (frozen at the last `go build` / `go run`) but JS/CSS off disk. Re-running `npm run build` without restarting Go leaves the embedded HTML pointing at the *old* hashed asset names → blank page with 404s in the browser console. Always restart `go run .` after a frontend rebuild.

### Adding a new page

1. Create `frontend/<page>.html` (copy an existing one and adjust the title + the imported entry).
2. Create `src/entries/<page>.js` — `createApp(Page).use(antd).use(i18n).mount('#app')`.
3. Create the page component under `src/pages/<page>/<Page>.vue` (kebab-case folder, PascalCase component).
4. Register the entry in `rollupOptions.input` inside `vite.config.js`.
5. If the page is reachable from the sidebar at `/panel/<route>`, add `<route>` to `MIGRATED_ROUTES` so dev-mode navigation works.
6. Wire a Go controller route that calls `serveDistPage(c, "<page>.html")` to serve the embedded HTML in prod.

### Conventions

- **Ant Design Vue** is the only UI kit — no Tailwind, no shadcn. A previous attempt to migrate was rolled back as ugly + bloated. Small targeted UX tweaks beat sweeping rewrites; if a section *really* needs new visual language, raise it first.
- **Composition API** (`<script setup>`) everywhere. Options API survives only in components nobody has touched yet.
- **No `//` line comments** in committed JS/Vue. HTML `<!-- ... -->` is fine for template structure. Identifiers should carry the meaning; if you need a comment to explain *what* code does, rename the variable. Comments are for the *why* and only when surprising.
- **Persian / Arabic users matter.** RTL is supported via `ConfigProvider` + `dir="rtl"`. When you write Persian text in toasts or labels, keep prose clean — isolate code/identifiers on their own lines so the RTL reading flows.
- **Don't break links.** Share-link generation has two paths: the **inbounds page** (`InboundsPage.vue` → `checkFallback()`) and the **clients page** (`/panel/api/clients/subLinks/:subId` → backend `GetSubs`). Exercise both whenever you touch URL generation, fallback projection, or TLS handling.

### Project layout

```
frontend/
├── *.html                — Vite entry HTML, one per panel route
├── eslint.config.js      — ESLint 10 flat config (vue3-recommended)
├── vite.config.js
└── src/
    ├── entries/          — per-page bootstrap (createApp + mount)
    ├── pages/            — one folder per route (index, login, inbounds, clients, xray, settings, sub, api-docs)
    ├── components/       — cross-page Vue components (DateTimePicker, FinalMaskForm, …)
    ├── composables/      — reusable reactive logic (useTheme, useStatus, useNodeList, …)
    ├── api/              — Axios setup + CSRF interceptor + WebSocket client
    ├── i18n/             — vue-i18n bootstrap (the JSON lives in web/translation/)
    ├── models/           — Inbound, DBInbound, Outbound, Status, reality-targets, …
    └── utils/            — HttpUtil, ObjectUtil, LanguageManager, RandomUtil, SizeFormatter, …
```

Lint with `cd frontend && npm run lint`. The deeper reference is [`frontend/README.md`](frontend/README.md).

## Project layout

| Path | What lives there |
|------|------------------|
| `main.go` | Process entry point, CLI subcommands, signal handling |
| `web/` | Gin HTTP server, controllers, services, embedded frontend |
| `frontend/` | Vue 3 + Ant Design source for the panel UI |
| `database/` | GORM models, migrations, seeders (SQLite / PostgreSQL) |
| `xray/` | Xray-core process lifecycle + gRPC API client |
| `sub/` | Subscription endpoints (raw, JSON, Clash) |
| `config/` | Environment-var helpers, paths, defaults |
| `x-ui/` | **Runtime data** — db, logs, xray binary, geo files (gitignored) |

## Sending a pull request

1. Branch off `main` (e.g. `feat/short-description`).
2. Keep the diff focused — separate refactors from feature work.
3. Run the relevant builds before pushing:
   - `go build ./...`
   - `go test ./...` (if you touched Go code)
   - `cd frontend && npm run build` (if you touched the Vue side)
4. Commit messages follow the existing pattern in `git log` — `<area>: short imperative summary`, then a body explaining the *why*.
5. Open the PR against `main` with a brief description of what changed and how to test it.

## Useful environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `XUI_DEBUG` | `false` | Verbose logs + Gin debug mode + serve `/assets` from disk |
| `XUI_LOG_LEVEL` | `info` | `debug` / `info` / `notice` / `warning` / `error` |
| `XUI_DB_FOLDER` | platform default | Where `x-ui.db` lives |
| `XUI_LOG_FOLDER` | platform default | Where `3xui.log` lives |
| `XUI_BIN_FOLDER` | `bin` | Where the xray binary + geo files + xray `config.json` live |
| `XUI_DB_TYPE` | `sqlite` | Set to `postgres` to use PostgreSQL via `XUI_DB_DSN` |
| `XUI_DB_DSN` | — | PostgreSQL DSN when `XUI_DB_TYPE=postgres` |

## Issues and discussion

- Bug reports and feature requests: [GitHub Issues](https://github.com/MHSanaei/3x-ui/issues)
- General questions and ideas: [GitHub Discussions](https://github.com/MHSanaei/3x-ui/discussions)

Before filing a bug, please include your OS, Go version, panel version (`/panel/api/server/status` or the dashboard footer), and the relevant excerpt from `x-ui/3xui.log`.
