# frontend/CLAUDE.md

Frontend agent guide. Full detail: `frontend/README.md` and the root
`CONTRIBUTING.md` ("Working on the frontend"). This is the short version.

## What this is
React 19 + Ant Design 6 + Vite 8 + TypeScript. The Vite config is
`vite.config.js` (plain JS). Three bundles, each emitted into
`internal/web/dist/` and embedded into the Go binary:
- `index.html` — admin panel SPA (entry `src/main.tsx`; react-router under
  `/panel`, lazy routes).
- `login.html` — login + 2FA (`src/entries/login.tsx`).
- `subpage.html` — public subscription viewer (`src/entries/subpage.tsx`).
The `@` import alias maps to `src/`.

## Data flow
- Server state via TanStack Query (`src/api/`, keys in `src/api/queryKeys.ts`);
  invalidate on mutation. WebSocket pushes feed the cache
  (`src/api/websocketBridge.ts`).
- Local UI state in the page (`useState`); shared concerns via `src/hooks/`.
  Extend an existing hook before adding a global.
- Zod (`src/schemas/`) is the single source of truth for the xray config model.
  Infer types with `z.infer`. Go-side types are mirrored into `src/generated/`
  by `npm run gen:zod` (`go run ./tools/openapigen`) — do not hand-edit that
  folder (every file is marked `DO NOT EDIT`).
- xray domain logic (links, defaults, form<->wire adapters) is pure functions in
  `src/lib/xray/`. HTTP goes through `HttpUtil` in `src/utils/index.ts`.

## Rules
- Ant Design 6 only; no Tailwind/shadcn (a migration was rolled back).
- Function components + hooks only; no class components.
- No `//` line comments in committed TS/TSX. HTML comments are fine.
- TS strict; `no-explicit-any` is an error. Validate form fields with
  `antdRule(Schema.shape.field, t)` from `@/utils/zodForm`, not inline
  `z.string()`.
- New `g.POST`/`g.GET` route => add it to `src/pages/api-docs/endpoints.ts`,
  then `npm run gen`.
- i18n strings live in `internal/web/translation/<locale>.json`, NOT under
  `frontend/`, and are shared with the Go backend. A new English key must be
  added to every locale. Interpolation here uses single braces `{var}`, not the
  i18next default `{{var}}`.
- Persian/Arabic (RTL) users are first-class — isolate code identifiers on their
  own line when writing Persian text in labels/toasts.
- Vite is pinned to an exact version (no `^`) — bump deliberately, then verify
  `npm run dev` AND `npm run build`.

## Adding a panel route
1. `src/pages/<page>/<Page>.tsx` (kebab folder, PascalCase component).
2. Register in `src/routes.tsx` under `/panel` (lazy import).
3. Add a sidebar link in `src/layouts/AppSidebar.tsx` if it needs nav.
Only standalone bundles (login/subpage) need a new `.html` + `src/entries/*` +
`rollupOptions.input` (in `vite.config.js`) + a Go controller route.

## Commands
- `npm run dev` (HMR on :5173, proxies to the Go panel on :2053 — start Go first).
- `npm run typecheck` / `npm run lint` / `npm run test` / `npm run build`.
- `npm run gen` = `gen:zod` (Go → `src/generated/`) + `gen:api`
  (`build-openapi.mjs` → `public/openapi.json`).
- After `npm run build`, RESTART `go run .` (see the XUI_DEBUG gotcha in root
  CLAUDE.md) before checking the panel.
