# 3x-ui frontend

React 19 + Ant Design 6 + TypeScript + Vite 8. Multi-page app — one HTML
entry per panel route — built into `../web/dist/` and embedded into the
Go binary via `embed.FS`.

## Dev

```sh
npm install
npm run dev
```

Vite serves on `http://localhost:5173/`. API calls and `/panel/*` routes
proxy to the Go panel at `http://localhost:2053/`, so start the Go panel
first (`go run main.go`) and then Vite.

The proxy auto-rewrites `/panel`, `/panel/settings`, `/panel/inbounds`,
`/panel/xray` to the matching Vite-served HTML in dev mode (see
`MIGRATED_ROUTES` in `vite.config.js`), so the sidebar's
production-style links work without round-tripping through Go.

## Production build

```sh
npm run build
```

Outputs to `../web/dist/` (HTML at the root, hashed JS/CSS under
`assets/`). The Go binary embeds this directory at compile time and
`web/controller/dist.go` serves the per-page HTML.

## Type check and lint

```sh
npm run typecheck
npm run lint
```

`tsc --noEmit` against `tsconfig.json` (strict mode, `jsx: "react-jsx"`,
`@/*` → `src/*` alias). ESLint 10 with `eslint.config.js` (flat config)
— `@eslint/js` recommended plus `typescript-eslint` and
`eslint-plugin-react-hooks` rules.

## Layout

```
frontend/
├── *.html                 # Vite entry HTML, one per panel route
├── tsconfig.json
├── eslint.config.js
├── vite.config.js
└── src/
    ├── entries/           # Per-page bootstrap (createRoot + render)
    ├── pages/             # One folder per route, each with the page
    │   ├── index/         # component + helpers + sub-components
    │   ├── login/
    │   ├── inbounds/
    │   ├── clients/
    │   ├── xray/
    │   ├── nodes/
    │   ├── settings/
    │   ├── api-docs/
    │   └── sub/
    ├── components/        # Cross-page React components
    ├── hooks/             # Reusable hooks (useTheme, useWebSocket, …)
    ├── api/               # Axios setup, CSRF interceptor, WebSocket
    ├── i18n/              # react-i18next init (locales live in web/translation/)
    ├── models/            # Inbound, Outbound, Status, … domain classes
    ├── styles/            # Shared CSS modules (page-cards, …)
    └── utils/             # HttpUtil, ObjectUtil, LanguageManager, …
```

## Adding a new page

1. Add `frontend/<page>.html` referencing `/src/entries/<page>.tsx`.
2. Add `src/entries/<page>.tsx` that imports the page component and
   mounts it with `createRoot(...).render(...)`.
3. Add the page component under `src/pages/<page>/`.
4. Register the entry in `rollupOptions.input` in `vite.config.js`.
5. If the page is reachable from the sidebar at `/panel/<route>`, add
   it to `MIGRATED_ROUTES` so the dev proxy serves the Vite HTML.
6. Wire the Go controller to `serveDistPage(c, "<page>.html")`.
