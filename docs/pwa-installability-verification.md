# PWA installability verification

This change adds a network-only PWA surface to the login and panel pages. It
does not cache panel data, API responses, credentials, or WebSocket traffic.

## Local checks

Run these commands from the repository root after installing the pinned Node
and Go toolchains:

```text
cd frontend
npm run typecheck
npm run lint
npx vitest run --project unit
npx vitest run --project components
npm run build
cd ..
go test ./...
go build ./...
```

The built binary must serve these paths beneath the configured `webBasePath`:

- `manifest.webmanifest`
- `pwa-register.js`
- `service-worker.js`
- `icons/3x-ui-16.png`
- `icons/3x-ui-24.png`
- `icons/3x-ui-32.png`
- `icons/3x-ui-64.png`
- `icons/3x-ui-192.png`
- `icons/3x-ui-512.png`

The login and panel HTML must contain a manifest link and registration script
whose URLs begin with the same runtime base path. The manifest must contain
`display: "standalone"`, relative `start_url` and `scope`, and all six icon
entries.

## Live rollout checks

Before replacing a server binary, record the current x-ui binary checksum and
create a timestamped copy of the binary and `/etc/x-ui/x-ui.db`. Restart only
the `x-ui` service after the candidate is staged. Because x-ui manages Xray as
a child process, the restart can briefly interrupt VPN connections.

After the restart, verify:

1. `x-ui` is active and its child Xray process is running.
2. The existing panel URL serves HTML with the PWA manifest link.
3. The manifest, registration script, worker, and all six icons return `200`.
4. Login, authenticated API requests, panel navigation, logout, and the panel
   WebSocket all work.
5. At least one VPN client can complete a fresh connection cycle.

If any check fails, restore the exact binary backup, restart x-ui once, and
repeat the checks against the original build.
