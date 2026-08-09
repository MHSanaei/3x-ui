# 3x-ui PWA Installability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a minimal, network-only Android PWA mode to 3x-ui using the existing panel URL and runtime web base path.

**Architecture:** The frontend build emits a manifest, a no-cache service worker, and a registration script. The Go embedded-web layer injects their URLs into login and panel HTML and serves them beneath the configured base path, while the service worker deliberately does not intercept requests. No VPN, Xray, database, or authentication logic changes.

**Tech Stack:** Go/Gin, embedded `embed.FS`, React/Vite frontend, TypeScript/JavaScript, standard Web App Manifest and Service Worker APIs, Go `testing`, Vitest.

---

### Task 1: Add PWA static assets

**Files:**
- Create: `frontend/public/manifest.webmanifest`
- Create: `frontend/public/pwa-register.js`
- Create: `frontend/public/service-worker.js`
- Create: `frontend/public/icons/3x-ui-192.svg`
- Create: `frontend/public/icons/3x-ui-512.svg`

- [ ] Add a generic manifest with `name`, `short_name`, `start_url: "./"`, `scope: "./"`, `display: "standalone"`, theme colors, and 192/512 icons.
- [ ] Add a registration script that derives the base path from its own URL and registers `service-worker.js` within that path only.
- [ ] Add a service worker with install/activate lifecycle handling and an intentionally empty fetch handler; do not use `CacheStorage`, `respondWith`, or API fallbacks.
- [ ] Add simple repository-owned SVG icons with the required dimensions and `purpose: "any maskable"`.

### Task 2: Inject PWA metadata using the runtime base path

**Files:**
- Modify: `internal/web/controller/dist.go`
- Test: `internal/web/controller/dist_test.go`

- [ ] Add a helper that returns the PWA manifest and registration URLs for a normalized runtime base path.
- [ ] Extend `serveDistPage` to inject a manifest link and deferred registration script only for `index.html` and `login.html`.
- [ ] Keep the existing CSP nonce script and CSRF/base-path metadata unchanged.
- [ ] Add tests for `/`, `/panel-secret/`, and a path with a trailing slash, asserting that URLs stay within the runtime base path and `subpage.html` is not changed.

### Task 3: Serve embedded PWA files safely

**Files:**
- Create: `internal/web/controller/pwa.go`
- Test: `internal/web/controller/pwa_test.go`
- Modify: `internal/web/web.go`

- [ ] Add handlers that read only `dist/manifest.webmanifest`, `dist/pwa-register.js`, and `dist/service-worker.js` from the embedded filesystem.
- [ ] Add allowlisted handlers for the two manifest icons without accepting arbitrary filesystem paths.
- [ ] Register the handlers beneath the existing `basePath` group so the current random URL is the only public scope.
- [ ] Set exact content types and `Cache-Control: no-cache, no-store, must-revalidate` for the manifest, registration script, and service worker.
- [ ] Return a hard 404 for unknown PWA asset names and test that no filesystem path is accepted from the request.

### Task 4: Build-focused tests and documentation

**Files:**
- Modify: `frontend/src/test/` or add a focused manifest test if the existing frontend test conventions support it.
- Modify: `frontend/README.md`
- Modify: `docs/architecture.md` only if the repository's architecture index needs the new embedded assets documented.

- [ ] Verify the manifest is valid JSON and contains the two required icon entries.
- [ ] Document that PWA mode is network-only and that no panel/API data is cached.
- [ ] Run `npm run typecheck`, `npm run lint`, `npm run test`, and `npm run build`.
- [ ] Run `make test-go`, `make lint-go`, and `make build` after the frontend build.

### Task 5: Local release verification

**Files:**
- Create: `docs/pwa-installability-verification.md`

- [ ] Build the exact release binary from the feature branch with embedded frontend assets.
- [ ] Start it against a disposable database and verify `/base/`, `/base/manifest.webmanifest`, `/base/pwa-register.js`, and `/base/service-worker.js`.
- [ ] Use a browser test to verify the login page exposes the manifest and that the service worker scope equals the panel base path.
- [ ] Verify login, panel navigation, an authenticated API request, logout, and WebSocket status updates.
- [ ] Record the commands and expected results without recording credentials or session cookies.

### Task 6: Controlled server rollout

**Files:**
- No production files are modified by the repository change.

- [ ] Confirm the Finland and home servers run the same 3x-ui build baseline and that both Xray children are healthy.
- [ ] Back up each current `/usr/local/x-ui/x-ui` binary and `/etc/x-ui/x-ui.db` using timestamped server-side paths.
- [ ] Install and restart the Finland VPS service first, then verify x-ui, Xray, panel URL, login, API, WebSocket, and a VPN client cycle.
- [ ] Repeat the same controlled rollout on the home server only after the Finland check passes.
- [ ] If any check fails, stop using the candidate binary, restore the exact backup, restart, and verify the old panel and VPN state.

### Task 7: Fork and upstream contribution

**Files:**
- No additional source files beyond the implementation tasks.

- [ ] Authenticate GitHub CLI as `korsun009` and create the fork from `MHSanaei/3x-ui`.
- [ ] Set the fork as `origin` and keep `upstream` pointing to `MHSanaei/3x-ui`.
- [ ] Commit the focused implementation with conventional commit messages and push `feat/pwa-installability` to the fork.
- [ ] Open a PR describing runtime base-path support, no-cache behavior, tests, and the fact that Xray/VPN logic is untouched.
- [ ] Keep the local fork branch available for future upstream rebases if the PR is not immediately merged.
