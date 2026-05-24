# Contributing

Thanks for taking the time to contribute to 3x-ui. This guide gets a development panel running locally and explains the conventions the project follows so changes land cleanly.

## Prerequisites

- **Go 1.26+** (the version pinned in `go.mod`)
- **Node.js 22+** and npm 10+ (for the React frontend)
- **Git**
- **A C compiler** — required by the CGo SQLite driver (`github.com/mattn/go-sqlite3`). Linux and macOS already ship one; for Windows see below.

### Windows: MinGW-w64

`go build` on Windows fails with `cgo: C compiler "gcc" not found` until a GCC toolchain is installed. Two options — pick whichever fits.

**Option A — standalone zip (fastest, no package manager)**

1. Download the latest build from <https://github.com/niXman/mingw-builds-binaries/releases>. For most setups, pick a release named:
   ```
   x86_64-<version>-release-posix-seh-ucrt-rt_<n>-rev<m>.7z
   ```
   (64-bit, POSIX threads, SEH exceptions, UCRT runtime — matches modern Windows defaults.)
2. Extract it somewhere stable, e.g. `C:\mingw64\`.
3. Add `C:\mingw64\bin` to the **Windows** `PATH` (System Properties → Environment Variables → Path → New).
4. Open a fresh terminal and confirm:
   ```powershell
   gcc --version
   ```

**Option B — MSYS2 (when a Unix shell is also useful)**

1. Install MSYS2 from <https://www.msys2.org/>.
2. Open the **MSYS2 UCRT64** shell from the Start menu and update once:
   ```bash
   pacman -Syu
   ```
3. Install the UCRT64 toolchain:
   ```bash
   pacman -S --needed mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkg-config
   ```
4. Add `C:\msys64\ucrt64\bin` to the Windows `PATH`.
5. Verify with `gcc --version` in a fresh terminal.

After either path, `go build ./...` and `go run .` work normally.

> **Why MinGW-w64 over MSVC:** `mattn/go-sqlite3` officially supports GCC, builds are faster on Windows, and the toolchain does not require a Visual Studio install. If Visual Studio Build Tools are already present that works too — just make sure `CC=cl` is **not** set in the environment.

Cross-building the Linux SQLite target from Windows (or vice versa) requires a separate cross-compiler and is out of scope here; build natively on the target OS.

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

`.env.example` ships with defaults that keep the database, logs, and xray binary inside the local `x-ui/` folder so nothing escapes the project directory:

```
XUI_DEBUG=true
XUI_DB_FOLDER=x-ui
XUI_LOG_FOLDER=x-ui
XUI_BIN_FOLDER=x-ui
```

Drop the xray binary (`xray-windows-amd64.exe` on Windows, `xray-linux-amd64` on Linux, etc.) plus the matching `geoip.dat` and `geosite.dat` files into `x-ui/`. The easiest source is a [released Xray-core build](https://github.com/XTLS/Xray-core/releases). On Windows, `wintun.dll` is also required for testing TUN inbounds.

## Running

```bash
go run .
```

Open [http://localhost:2053](http://localhost:2053) and log in with `admin` / `admin`. Credentials must be changed on first login.

### Inside VS Code

The repo ships a launch profile in `.vscode/launch.json` (gitignored — copy from the snippet below if absent):

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

The panel UI is a **React 19 + Ant Design 6 + TypeScript** app under `frontend/`, built with Vite 8. The sections below cover the architecture, the conventions, and the two dev workflows.

### Architecture

The frontend is a **multi-page application**, not a SPA. Every panel route (`/panel`, `/panel/inbounds`, `/panel/clients`, `/panel/xray`, `/panel/settings`, `/panel/nodes`, `/panel/api-docs`, `/panel/sub`, plus `login`) has its own HTML entry in `frontend/*.html` and its own bootstrap in `src/entries/<page>.tsx`. Vite emits each entry into `web/dist/`, and the Go binary embeds that directory at compile time via `embed.FS`. Each panel navigation is a real document load, but every per-page bundle is small enough to keep the experience responsive. There is no React Router and no global store; the surface area does not justify either.

### State and data flow

- **No global store.** State lives in the page that owns it. Cross-page data (settings, current user, theme) is re-fetched on each page load — the backend is local and responses are inexpensive.
- **Hooks** in `src/hooks/` encapsulate reactive logic worth sharing inside a page (`useTheme`, `useStatus`, `useNodes`, `useWebSocket`, `useDatepicker`, …). Prefer extending an existing hook over introducing a new global.
- **Domain models** in `src/models/` (`Inbound`, `DBInbound`, `Outbound`, `Status`, …) own the protocol-specific logic — link generation, settings JSON shape, TLS/Reality stream handling. React components stay declarative; they ask the model "what is my link?" and render the answer.
- **HTTP** goes through `src/utils/index.js`'s `HttpUtil`, a thin Axios wrapper that handles CSRF, response toasts, and a `silent: true` opt-out for bulk operations that would otherwise spam toasts. The Axios setup itself lives in `src/api/axios-init.js`.

### i18n

Locale strings live in `web/translation/<locale>.json`, **not** under `frontend/`. The Go binary embeds the same JSON and serves it to both backend templates and `react-i18next` (initialized in `src/i18n/react.ts`). When a new English key is added it must also land in **every** non-English locale — missing keys do not break the build, they just render the raw key in the UI.

### Two dev workflows

| Goal | Command |
|------|---------|
| Iterate on UI changes with HMR | `cd frontend && npm run dev` (Vite on `:5173`, proxies `/panel/*` and `/api/*` to the Go panel on `:2053`). Start the Go panel first. |
| Verify what end users actually see | `cd frontend && npm run build`, then `go run .`. The Go binary serves the built bundle — embedded in release mode, off disk in debug mode. |

The Vite dev proxy rewrites the sidebar's production-style links (`/panel`, `/panel/inbounds`, `/panel/clients`, …) to the matching Vite-served HTML, so navigation behaves identically to production without round-tripping through Go. The allowlist lives in `MIGRATED_ROUTES` in `vite.config.js` — register every new page there.

> **`XUI_DEBUG=true` gotcha** — in debug mode the panel serves HTML from the embedded FS (frozen at the last `go build` / `go run`) but JS/CSS off disk. Re-running `npm run build` without restarting Go leaves the embedded HTML pointing at the *old* hashed asset names, producing a blank page with 404s in the console. Always restart `go run .` after a frontend rebuild.

### Adding a new page

1. Create `frontend/<page>.html` (copy an existing entry and adjust the title and the imported `<script type="module" src="/src/entries/<page>.tsx">`).
2. Create `src/entries/<page>.tsx` — mount the page with `createRoot(document.getElementById('app')!).render(...)`, wrapped in the shared `ConfigProvider` for AntD theming and i18n.
3. Create the page component under `src/pages/<page>/<Page>.tsx` (kebab-case folder, PascalCase component).
4. Register the entry in `rollupOptions.input` inside `vite.config.js`.
5. If the page is reachable from the sidebar at `/panel/<route>`, add `<route>` to `MIGRATED_ROUTES` so dev-mode navigation works.
6. Wire a Go controller route that calls `serveDistPage(c, "<page>.html")` to serve the embedded HTML in production.

### Conventions

- **TypeScript strict mode** — all new code in `.ts` / `.tsx`. Run `npm run typecheck` (`tsc --noEmit`) before pushing. The path alias `@/*` resolves to `src/*`.
- **Ant Design 6** is the only UI kit — no Tailwind, no shadcn. A previous attempt to migrate was rolled back. Small, targeted UX tweaks beat sweeping rewrites; raise broader visual changes for discussion before implementing.
- **Function components + hooks** everywhere. No class components.
- **No `//` line comments** in committed JS/TS/Vue/Go. HTML `<!-- ... -->` is fine for template structure. Names should carry the meaning; rename rather than annotate. Comments are reserved for the *why*, and only when the reason is surprising.
- **RTL is a first-class concern.** Persian and Arabic users matter — RTL is enabled through AntD's `ConfigProvider direction="rtl"`. When writing Persian text in toasts or labels, isolate code identifiers on their own lines so RTL reading flows.
- **Do not break link generation.** Share-link generation has two paths: the **inbounds page** (`InboundsPage.tsx` → `checkFallback()`) and the **clients page** (`/panel/api/clients/subLinks/:subId` → backend `GetSubs`). Exercise both whenever URL generation, fallback projection, or TLS handling changes.
- **Vite is pinned** to `8.0.13`. Do not bump to `8.0.14+` — the esbuild dep-optimizer in those builds breaks i18n loading in dev mode.

### Project layout

```
frontend/
├── *.html                 — Vite entry HTML, one per panel route
├── tsconfig.json          — strict, jsx: "react-jsx", paths "@/*" → "src/*"
├── eslint.config.js       — ESLint 10 flat config (@eslint/js + typescript-eslint + react-hooks)
├── vite.config.js
└── src/
    ├── entries/           — per-page bootstrap (createRoot + render)
    ├── pages/             — one folder per route (index, login, inbounds, clients, xray, nodes, settings, api-docs, sub)
    ├── components/        — cross-page React components (AppSidebar, DateTimePicker, FinalMaskForm, JsonEditor, …)
    ├── hooks/             — reusable hooks (useTheme, useStatus, useNodes, useWebSocket, useDatepicker, …)
    ├── api/               — Axios setup + CSRF interceptor + WebSocket client
    ├── i18n/              — react-i18next bootstrap (JSON lives in web/translation/)
    ├── models/            — Inbound, DBInbound, Outbound, Status, reality-targets, …
    ├── styles/            — shared CSS (page-cards, …)
    └── utils/             — HttpUtil, ObjectUtil, LanguageManager, RandomUtil, SizeFormatter, …
```

For deeper notes on the frontend toolchain see [`frontend/README.md`](frontend/README.md).

## Project layout

| Path | Contents |
|------|----------|
| `main.go` | Process entry point, CLI subcommands, signal handling |
| `web/` | Gin HTTP server, controllers, services, embedded frontend assets |
| `frontend/` | React + Ant Design 6 + TypeScript source for the panel UI |
| `database/` | GORM models, migrations, seeders (SQLite / PostgreSQL) |
| `xray/` | Xray-core process lifecycle and gRPC API client |
| `sub/` | Subscription endpoints (raw, JSON, Clash) |
| `config/` | Environment-variable helpers, paths, defaults |
| `x-ui/` | **Runtime data** — db, logs, xray binary, geo files (gitignored) |

## Sending a pull request

1. Branch off `main` (e.g. `feat/short-description`).
2. Keep the diff focused — separate refactors from feature work.
3. Run the relevant checks before pushing:
   - `go build ./...`
   - `go test ./...` (when Go code changed)
   - `cd frontend && npm run typecheck && npm run lint && npm run build` (when the frontend changed)
4. Commit messages follow the existing pattern in `git log` — `<area>: short imperative summary`, then a body explaining the *why*. Conventional-commit prefixes (`feat`, `fix`, `refactor`, `chore`, `style`, `docs`) are encouraged.
5. Open the PR against `main` with a brief description of what changed and how to test it.

## Useful environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `XUI_DEBUG` | `false` | Verbose logs + Gin debug mode + serve `/assets` from disk |
| `XUI_LOG_LEVEL` | `info` | `debug` / `info` / `notice` / `warning` / `error` |
| `XUI_DB_FOLDER` | platform default | Where `x-ui.db` lives |
| `XUI_LOG_FOLDER` | platform default | Where `3xui.log` lives |
| `XUI_BIN_FOLDER` | `bin` | Where the xray binary, geo files, and xray `config.json` live |
| `XUI_DB_TYPE` | `sqlite` | Set to `postgres` to use PostgreSQL via `XUI_DB_DSN` |
| `XUI_DB_DSN` | — | PostgreSQL DSN when `XUI_DB_TYPE=postgres` |

## Issues and discussion

- Bug reports and feature requests: [GitHub Issues](https://github.com/MHSanaei/3x-ui/issues)
- General questions and ideas: [GitHub Discussions](https://github.com/MHSanaei/3x-ui/discussions)

Before filing a bug, include the OS, Go version, panel version (`/panel/api/server/status` or the dashboard footer), and the relevant excerpt from `x-ui/3xui.log`.
