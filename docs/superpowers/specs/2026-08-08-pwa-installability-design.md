# 3x-ui PWA Installability Design

## Goal

Make the 3x-ui login and administration panel installable as an Android PWA
from the existing panel URL, without an alternate domain, reverse proxy, or
offline application cache.

## Constraints

- The panel may run under a random runtime `webBasePath`.
- The panel's login session, API requests, and WebSocket traffic must remain
  network-backed and must not be cached by the service worker.
- The VPN configuration, Xray lifecycle, database, and panel authentication
  behavior remain unchanged.
- The implementation must be generic enough for an upstream pull request and
  must not contain a user's domain or web base path.
- The generated frontend remains embedded into the Go binary as it is today.

## Architecture

The frontend build will add a small manifest, a service worker that does not
intercept or cache requests, and an external registration script. The Go HTML
renderer will inject the manifest link and registration script using the
runtime base path, so the same build works with every configured panel path.

The Go web layer will serve the embedded PWA files from the current base path
with explicit content types and no-cache headers. The service worker will only
complete installation and claim clients; it will not implement offline
fallbacks, cache storage, request rewriting, or API handling.

## Runtime behavior

- Login and panel documents advertise the manifest.
- Chrome registers the service worker from the panel's base-path scope.
- `start_url` and `scope` are relative to the manifest, keeping the PWA on the
  current panel URL regardless of the random base path.
- Navigating to panel routes remains handled by the existing SPA fallback.
- The service worker never calls `respondWith`, so requests use the normal
  network path and panel auth cookies remain controlled by the browser.

## Testing

- Unit-test base-path HTML injection and embedded asset responses.
- Verify manifest JSON, required fields, MIME types, and no-cache headers.
- Run the repository frontend and Go verification suites.
- Build a release binary and verify the generated files are embedded.
- Run the binary against a disposable panel database and test login, panel
  navigation, API calls, and WebSocket status updates.
- Deploy the same binary first to the Finland VPS and then to the home server,
  with backups and a rollback check. A service restart is expected to briefly
  stop the managed Xray child because systemd uses `KillMode=control-group`.
