# 3x-ui frontend

Vue 3 + Ant Design Vue 4 + Vite. Builds into `../web/dist/`, which the
Go binary will embed via `embed.FS` once the migration reaches the page
handlers (Phase 4+).

This directory exists alongside the legacy `web/html/` Vue 2 templates
during the migration. Pages will move over one at a time on the
`vue3-migration` branch.

## Dev

```sh
cd frontend
npm install
npm run dev
```

The dev server runs on `http://localhost:5173/` and proxies API calls to
the Go panel at `http://localhost:2053/` — start the Go panel first
(`go run main.go`), then start Vite.

## Production build

```sh
npm run build
```

Outputs to `../web/dist/`. The Go binary picks it up at compile time via
`embed.FS`.

## Where things live

- `src/main.js` — app entrypoint (createApp, install Antd, mount)
- `src/App.vue` — root component (currently a smoke-test placeholder)
- `vite.config.js` — build + dev-server config
- `index.html` — Vite HTML template

## Adding new pages

For each legacy page being migrated, add an entry to
`vite.config.js` `rollupOptions.input`. Each entry produces its own
HTML file in `web/dist/`, which the Go panel route handler will serve.
