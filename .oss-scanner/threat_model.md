# 3x-ui threat model

3x-ui is a self-hosted web control panel for Xray-core. One Go binary stores
inbounds, clients and settings in SQLite (default) or PostgreSQL, renders an
Xray JSON config from that state, supervises Xray-core and the `mtg-multi`
MTProto sidecar as child processes, runs AmneziaWG and TUIC servers in-process,
and serves an embedded React SPA. It is deployed on internet-facing VPS hosts,
usually as root, often in censored regions where a compromise exposes the
people relying on the proxy. Read `docs/architecture.md` in the scanned revision
for the code map; do not assume package names stay fixed.

## Trust boundaries and attackers

Trusted: the panel administrator (one account, or an admin-scope API token).
The admin can already edit the full Xray config, run a panel update, import a
database and set file paths, so admin-to-root is by design and out of scope.

Untrusted, in decreasing order of importance:

1. **Unauthenticated network attacker** reaching the panel port (default 2053)
   or the subscription port. Login (`POST /login`, 2FA, LDAP), CSRF/session
   handling (`internal/web/session/`), the login rate limiter
   (`controller/login_limiter.go`), the `checkAPIAuth` gate in
   `controller/api.go` (session, Bearer token, mTLS client cert),
   `/csrf-token`, PWA/static routes, and the WebSocket `/ws` upgrade.
2. **Subscription end users.** They know only their own `subId` and fetch
   `{subPath}/{subId}`, the JSON/Clash/Mihomo variants and `hwid-status`
   (`internal/sub/`). Each must see only their own clients; check subId
   guessing, cross-client leakage, header injection, forwarded-header trust
   (`internal/sub/forwarded_trust.go`) and the public subscription page.
3. **Limited API tokens.** `monitor` and `node-sync` scopes are restricted by
   explicit allowlists in `controller/api.go` (`enforceTokenScope`). Any way
   for such a token to reach a route outside its allowlist is a vulnerability.
4. **Master/node peers.** A master panel drives child nodes over HTTPS
   (`internal/web/runtime/`, TLS modes verify/skip/pin/mtls). A compromised
   or malicious node must not gain control of the master or of other nodes;
   node responses (inbound lists, traffic, online clients, certificates,
   `descendants`) are untrusted input on the master.
5. **Proxy traffic.** Packets reaching AmneziaWG (`internal/amneziawgnet/`),
   TUIC (`internal/tuic/`) and the loopback SOCKS5 relays into Xray. A peer
   must not escape its per-peer auth, reach another peer's traffic, or crash
   the panel process.
6. **Chat-bot users.** Telegram and Discord bot messages and callback data
   (`internal/web/service/tgbot/`, `discord/`). Non-admin chats may only see
   their own client (bound via the subId invite token); admin commands must
   be unreachable from them.
7. **Imported content** an admin may receive from a third party: share links
   and subscription URLs parsed by the import paths (`FuzzParseLink`), and
   geosite/geoip `.dat` files (`internal/xray/geodata/`).

Also in scope: stored or reflected XSS in the SPA or the subscription page
from client remarks, emails, inbound tags or any other value a non-admin can
influence; SQL injection through GORM raw queries; path traversal in file or
certificate handling; secrets leaking into logs, subscription output, API
responses for limited tokens, or node sync payloads.

## How to exercise it

- Checkout at `/src`; the panel binary is `/usr/local/bin/x-ui` (debug build),
  Xray-core is `/usr/local/bin/xray`, and the sidecars and geo files are in
  `/src/build/bin`.
- Go tests: `cd /src && go test -count=1 ./...`. Real-DB helpers live in
  `internal/database/dbtest` (`dbtest.InitDB(t, path)`); `internal/sub`'s
  `initSubDB(t)` is the template for subscription tests. `httptest` is used
  for HTTP. `XRAY_E2E_BINARY` is set, so the xray gRPC end-to-end tests run.
- Master + node end to end:
  `go build -o /tmp/x-ui . && XUI_NODE_E2E_BINARY=/tmp/x-ui go test -count=1 -v ./internal/nodee2e/`.
- Fuzz targets: `FuzzParseLink`, `FuzzDecodeCertPin` (`go test -fuzz=...`).
- Frontend: `cd /src/frontend && npx vitest run --project unit --project components`.
  The Storybook browser project needs Chromium and is not provisioned.
- Running a live panel: `XUI_DB_FOLDER=/tmp/xui XUI_LOG_FOLDER=/tmp/xui
  XUI_BIN_FOLDER=/src/build/bin x-ui` (logs a generated username and password on
  first start; `x-ui setting -h` changes port, base path and credentials).
- Baseline logs from the image build: `/src/.oss-scanner-go-test.log` and
  `/src/.oss-scanner-vitest.log`. A built image does not prove the tests passed.
- PostgreSQL is not installed: tests gated on `XUI_TEST_PG_DSN` skip, so a
  green run says nothing about the Postgres dialect.

## Severity

- **Critical:** unauthenticated remote code execution; authentication bypass
  to admin; a limited token, subscription user, bot user or node obtaining
  admin control; reading the database or admin credentials unauthenticated.
- **High:** cross-client disclosure of subscription configs or credentials
  without knowing the target's subId; scope escape of a `monitor`/`node-sync`
  token; stored XSS that a non-admin can plant in the admin panel; a
  malicious node compromising its master; SQL injection reachable by a
  non-admin; peer isolation bypass in AmneziaWG/TUIC.
- **Medium:** remotely triggerable crash or resource exhaustion of the panel
  process without authentication (it takes down every proxy it supervises);
  CSRF on a state-changing admin action; login rate-limit bypass; secret
  leakage into logs.
- **Low / informational:** issues that need admin access, local shell access,
  or an attacker already able to modify the database or binaries.

## Reports and patches

Name the scanned commit, the attacker's starting position from the list
above, the configuration needed (default or not), the entry route, and the
full path to impact. Give a deterministic reproducer, ideally a Go test in the
package that owns the bug, using the real DB helpers rather than mocks.
Patches should fix the root cause, follow `CLAUDE.md` in the repository
(comments at most two lines, stdlib `testing`, state-changing inbound/client
operations dispatched through `runtime.Runtime`), and include a regression
test that fails without the fix. Merge findings sharing one root cause.

## Out of scope

- Anything requiring the admin account or an admin-scope token, including
  admin-supplied Xray config, custom geo files, panel update and DB import.
- Vulnerabilities inside Xray-core, mtg-multi, amneziawg-go or other
  dependencies, unless 3x-ui itself makes them reachable in a way upstream
  does not; report the 3x-ui path.
- `skip` TLS mode accepting any node certificate: it is an explicit,
  documented opt-out.
- The `docs/` site, `tools/`, install scripts and CI workflows.
- Missing hardening headers or cookie flags with no demonstrated exploit.
