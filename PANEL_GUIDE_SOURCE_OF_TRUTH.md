# 3x-ui Panel Deep Guide (Source of Truth)

## Scope and trust model

This document is built from:
- Official project wiki pages cloned from `https://github.com/MHSanaei/3x-ui.wiki.git`.
- Source code in this repository (`MHSanaei/3x-ui`, current local `main`).
- Live panel inspection against your running instance via Playwright.

This is intended to be a practical source of truth for operating the panel.

Important constraints:
- The panel evolves quickly; always re-check against current code/wiki before risky production changes.
- Some behavior differs by version, settings, and enabled modules (Telegram, LDAP, Fail2Ban, subscription JSON).

---

## 1. What 3x-ui is

3x-ui is a web control panel around Xray-core. It manages:
- Inbounds and client accounts.
- Traffic/expiry limits and stats.
- Xray config template editing and advanced routing/outbound controls.
- Operational actions (restart Xray, logs, geofile updates, DB export/import).
- Optional subscription service (links and JSON endpoint).
- Optional Telegram bot features.

---

## 2. Runtime architecture

## 2.1 Core processes

The binary starts two servers:
- Web panel server (UI + panel API).
- Subscription server (separate listener/path logic).

Code references:
- `main.go` boot and signal handling.
- `web/web.go` panel server setup.
- `sub/sub.go` subscription server setup.

## 2.2 Data layer

Persistent data is SQLite (GORM).
Default models auto-migrated at startup:
- `User`
- `Inbound`
- `OutboundTraffics`
- `Setting`
- `InboundClientIps`
- `xray.ClientTraffic`
- `HistoryOfSeeders`

Code references:
- `database/db.go`
- `database/model/model.go`
- `xray/client_traffic.go`

## 2.3 Session/auth model

- Session cookie store is configured with secret from settings.
- Cookie is `HttpOnly`, `SameSite=Lax`, max age based on `sessionMaxAge`.
- Panel routes under `/panel` require login session.
- API routes under `/panel/api` also require session; unauthorized API returns `404` (not `401`) to hide endpoint existence.

Code references:
- `web/web.go` (session setup)
- `web/session/session.go`
- `web/controller/xui.go` and `web/controller/api.go`

## 2.4 Scheduler/background jobs

Panel cron jobs include:
- Xray running check: every `1s`.
- Conditional Xray restart pass: every `30s`.
- Traffic pull from Xray API: every `10s` (after initial delay).
- Client IP checks: every `10s`.
- Log cleanup: daily.
- Periodic traffic reset: daily/weekly/monthly.
- LDAP sync: configurable cron, when enabled.
- Telegram stats notify: configurable cron, when enabled.
- Hash storage cleanup: every `2m` (Telegram callbacks).
- CPU threshold monitor: every `10s` (Telegram feature).

Code references:
- `web/web.go`
- `web/job/*.go`

---

## 3. URL topology and route map

With default base path `/`, main user-facing URLs are:
- Login: `/`
- Panel pages: `/panel/`, `/panel/inbounds`, `/panel/settings`, `/panel/xray`
- Panel APIs: `/panel/api/...`

If `webBasePath` changes, the entire panel path is prefixed.

## 3.1 UI routes

Code references:
- `web/controller/index.go`
- `web/controller/xui.go`

- `GET /` login page (or redirects to `panel/` when already logged in).
- `POST /login`
- `GET /logout`
- `POST /getTwoFactorEnable`
- `GET /panel/`
- `GET /panel/inbounds`
- `GET /panel/settings`
- `GET /panel/xray`

## 3.2 API routes (panel)

Code references:
- `web/controller/api.go`
- `web/controller/inbound.go`
- `web/controller/server.go`
- `web/controller/setting.go`
- `web/controller/xray_setting.go`

Main groups:
- `/panel/api/inbounds/*`
- `/panel/api/server/*`
- `/panel/setting/*`
- `/panel/xray/*`
- `/panel/api/backuptotgbot`

(See sections 8 and 9 for functional behavior per group.)

## 3.3 Subscription service routes

Code references:
- `sub/sub.go`
- `sub/subController.go`

Routes are dynamic based on settings:
- Link path: `{subPath}:subid`
- JSON path (if enabled): `{subJsonPath}:subid`

Defaults are typically `/sub/` and `/json/` style paths.

---

## 4. Live instance findings (from your panel)

Observed in your current running panel:
- Version shown: `v2.8.10`.
- Xray shown running.
- Inbounds page has 2 active VLESS+Reality inbounds on ports `443` and `8443`.
- Security warnings active:
  - Panel connection not secure (TLS missing).
  - Panel path appears default/weak.
  - Subscription path appears default/weak.

This confirms panel behavior matches code/wiki defaults and hardening alerts.

---

## 5. Page-by-page guide

## 5.1 Overview page

Purpose:
- Operational dashboard for server + Xray + quick management actions.

Typical widgets/actions:
- CPU/RAM/Swap/Storage stats.
- Xray state with `Stop` / `Restart` actions.
- Current Xray version and switch/update actions.
- Logs, config, backup shortcuts.
- Uptime and traffic/speed totals.
- IP info, connection stats.

Backed by:
- `server.status` and related API endpoints.
- WebSocket status broadcasts.

Code references:
- `web/controller/server.go`
- `web/service/server.go`
- `web/websocket/*`

## 5.2 Inbounds page

Purpose:
- Manage listeners and user clients.

Core capabilities:
- Add/edit/delete inbounds.
- Enable/disable inbound.
- Add/edit/delete clients.
- Bulk client add.
- Reset traffic (single client, per inbound, global).
- Remove depleted clients.
- Import inbound.
- Export URLs/subscription URLs/inbound template.
- Search/filter and online/last-online checks.

Live menu entries seen in your panel include:
- Edit
- Add Client
- Add Bulk
- Reset Clients Traffic
- Export All URLs
- Export All URLs - Subscription
- Delete Depleted Clients
- Export Inbound
- Reset Traffic
- Clone
- Delete

Implementation notes:
- Changes are persisted to DB, and for enabled inbounds/clients are mirrored via Xray API when possible.
- For certain changes, `needRestart` path is set and picked up by scheduler.
- Traffic accounting is updated from periodic Xray traffic pulls.

Code references:
- `web/controller/inbound.go`
- `web/service/inbound.go`
- `web/service/xray.go`

## 5.3 Panel Settings page

Top tabs (observed + source):
- General
- Authentication
- Telegram Bot
- Subscription

### General

What it controls:
- Listen IP/domain/port for panel.
- Panel URI path (`webBasePath`).
- Session duration.
- Pagination size.
- Language.
- Additional collapsible areas: notifications, certificates, external traffic, date/time, LDAP.

Why critical:
- Port/path/domain/listen are core attack-surface controls.

### Authentication

What it controls:
- Admin credential change (current/new username/password).
- Two-factor authentication section.

Behavior:
- Update verifies old credentials first.
- Passwords are hashed (bcrypt).

### Telegram Bot

What it controls:
- Enable/disable bot.
- Bot token.
- Admin chat IDs.
- Notification cadence/language.
- Additional bot notification/proxy/server settings.

Capabilities documented and implemented:
- Periodic reports.
- Login notify.
- CPU/load notify.
- Backup delivery.
- Client lookup/report flows.

### Subscription

What it controls:
- Subscription service enable.
- JSON subscription endpoint enable.
- Listen IP/domain/port.
- URI paths for link/json endpoints.
- Encryption/show-info/update interval/title/support/announce.
- Routing headers integration settings.

Behavior notes:
- Subscription server can run with separate certs/keys.
- Headers like `Subscription-Userinfo`, profile metadata, and routing hints can be returned.

Code references:
- `web/controller/setting.go`
- `web/service/setting.go`
- `sub/sub.go`
- `sub/subController.go`

## 5.4 Xray Configs page

Top tabs (observed):
- Basics
- Routing Rules
- Outbounds
- Reverse
- Balancers
- DNS
- Advanced

### Basics

Controls global xray behavior such as:
- General runtime strategies.
- Statistics/log/general options.
- Outbound test URL used for test-outbound checks.

### Routing Rules

Defines how traffic is routed (domain/ip/network/rule matching, outbound tags).
Used for use-cases like WARP/TOR steering.

### Outbounds

Manages outbound objects and stats.
Features include:
- Add/edit/delete outbound.
- WARP creation/registration hooks.
- Outbound traffic reset.
- Outbound connectivity test (using server-controlled test URL).

### Reverse/Balancers/DNS

Advanced xray building blocks exposed in UI for complex routing topologies.

### Advanced

Raw template-centric advanced xray config editing.
Final generated config derives from template + panel-managed entities.

Safety note:
- Invalid advanced edits can break xray start; keep recovery path (default/reset) ready.

Code references:
- `web/controller/xray_setting.go`
- `web/service/xray_setting.go`
- `web/service/outbound.go`
- `web/service/warp.go`
- `web/service/xray.go`

---

## 6. Subscription service deep behavior

The subscription server:
- Is independent from panel UI server.
- Reads dedicated subscription settings.
- Supports optional domain enforcement.
- Supports link and JSON endpoints.
- Supports base64 response mode for subscription links.
- Can render HTML subscription page when requested by browser/flags.
- Emits metadata headers (`Subscription-Userinfo`, update interval, profile headers, routing headers).

Code references:
- `sub/sub.go`
- `sub/subController.go`
- `sub/subService.go`
- `sub/subJsonService.go`

---

## 7. Settings catalog (important keys)

Default key space includes panel, xray, telegram, subscription, ldap, and feature toggles.
Key examples:
- Panel: `webListen`, `webDomain`, `webPort`, `webCertFile`, `webKeyFile`, `webBasePath`, `sessionMaxAge`.
- Security/auth: `secret`, `twoFactorEnable`, `twoFactorToken`.
- Xray: `xrayTemplateConfig`, `xrayOutboundTestUrl`.
- Telegram: `tgBotEnable`, `tgBotToken`, `tgBotChatId`, `tgRunTime`, `tgCpu`, etc.
- Subscription: `subEnable`, `subJsonEnable`, `subPort`, `subPath`, `subJsonPath`, `subEncrypt`, `subUpdates`, etc.
- LDAP: multiple `ldap*` keys.

Code references:
- `web/service/setting.go` default map and getters/setters.

---

## 8. API behavior guide

## 8.1 Auth

- Login endpoint creates session cookie.
- API requires valid session.
- Unauthenticated API calls return 404 by design.

Code references:
- `web/controller/index.go`
- `web/controller/api.go`

## 8.2 Inbounds API

Base path: `/panel/api/inbounds`

Functions:
- Read inbounds/client traffics.
- CRUD inbound.
- CRUD inbound clients.
- Reset traffic operations.
- Import inbound.
- Online/last-online queries.
- IP tracking/cleanup.

Code reference:
- `web/controller/inbound.go`

## 8.3 Server API

Base path: `/panel/api/server`

Functions:
- Status and CPU history.
- Xray version listing/install.
- Geofile updates.
- Logs and xray logs retrieval.
- Config and DB export.
- DB import.
- Utility key generators (UUID, X25519, ML-DSA-65, ML-KEM-768, VLESS keys, ECH cert).

Code reference:
- `web/controller/server.go`

## 8.4 Xray settings API

Base path: `/panel/xray`

Functions:
- Read/update xray template + outbound test URL.
- Get xray result text.
- Get/reset outbound traffic.
- Warp actions (`data`, `del`, `config`, `reg`, `license`).
- Test outbound endpoint.

Code reference:
- `web/controller/xray_setting.go`

## 8.5 Panel settings API

Base path: `/panel/setting`

Functions:
- Get all settings.
- Get default settings.
- Update settings.
- Update user credentials.
- Restart panel.
- Get default xray JSON config.

Code reference:
- `web/controller/setting.go`

---

## 9. CLI and admin script operations

## 9.1 Binary subcommands (`x-ui` binary)

From `main.go`:
- `run`
- `migrate`
- `setting` (reset/show/port/user/password/basepath/listenIP/cert flags/telegram flags)
- `cert`

Useful flag examples:
- `x-ui setting -show true`
- `x-ui setting -port 2053`
- `x-ui setting -webBasePath /random-path/`
- `x-ui setting -username <u> -password <p>`
- `x-ui setting -resetTwoFactor true`
- `x-ui cert -webCert <fullchain.pem> -webCertKey <privkey.pem>`

## 9.2 Menu/admin shell script (`x-ui.sh`)

Top-level menu includes:
- Install/update/uninstall/legacy.
- Reset credentials/base path/settings.
- Port change and current settings display.
- Start/stop/restart/status/log management.
- Autostart toggles.
- SSL management (ACME, Cloudflare, set cert paths).
- IP limit/Fail2Ban management.
- Firewall management.
- SSH port-forwarding helper.
- BBR, geofile update, speedtest.

Script subcommands include:
- `x-ui start|stop|restart|status|settings|enable|disable|log|banlog|update|legacy|install|uninstall|update-all-geofiles`

Code reference:
- `x-ui.sh`

---

## 10. Security and hardening guide (critical)

Minimum hardening baseline:
1. Change default admin credentials immediately.
2. Enable two-factor authentication.
3. Change panel URI path from default (`/`) to a long random path ending with `/`.
4. Use TLS certs for panel and subscription endpoints.
5. Prefer binding panel to localhost/private interface and use SSH tunnel for admin access.
6. Restrict firewall rules tightly (panel port not globally open if possible).
7. Enable Fail2Ban/IP limit where relevant.
8. Keep geofiles and xray/panel versions updated.
9. Treat Telegram token as secret; rotate if exposed.
10. Export backups regularly and test restore flow.

Operational warning:
- Many panel settings changes require Save + Restart Panel/Xray to apply.

---

## 11. Common operational workflows

## 11.1 Add a new inbound safely

1. Go to `Inbounds` -> `Add Inbound`.
2. Choose protocol/transport/security (e.g., VLESS + TCP + Reality).
3. Set port/tag/stream settings and enable inbound.
4. Add clients with limits (traffic, expiry, IP limit, reset policy).
5. Save and verify on `Overview` (xray running, no config errors).
6. Export URLs/subscription for distribution.

## 11.2 Rotate admin credentials

1. `Panel Settings` -> `Authentication`.
2. Fill current and new credentials.
3. Confirm and verify new login.
4. Enable/verify 2FA.

## 11.3 Enable subscription links

1. `Panel Settings` -> `Subscription`.
2. Enable Subscription Service and set secure path/domain/port.
3. Optionally enable JSON endpoint and set separate path.
4. Configure TLS cert paths for subscription listener.
5. Save and restart panel/services as required.
6. Validate `/{subPath}{subid}` and JSON endpoint.

## 11.4 Configure WARP routing

1. `Xray Configs` -> `Outbounds` -> WARP create/register flow.
2. Add/verify outbound tag.
3. `Routing Rules` add domain/ip/network rules to send selected traffic to WARP outbound.
4. Save and restart xray.

## 11.5 Restore from DB backup

1. Export DB first before risky operations.
2. Use server API/UI import DB action.
3. Ensure xray restart path completes.
4. Validate inbounds/clients/settings and login after import.

---

## 12. Known caveats and behavior notes

- API unauthorized returns 404 (intentional) and may confuse API clients expecting 401.
- Some warning/help text in wiki is version-sensitive; code behavior wins when mismatch appears.
- Access log and fail2ban/IP-limit behavior depend on proper Xray log setup.
- High bandwidth users may continue briefly after limits due to periodic enforcement cadence.
- If DB is slow/locked, traffic/accounting behavior can appear delayed.

---

## 13. High-value file map (for future deep checks)

- Boot/CLI:
  - `main.go`
  - `x-ui.sh`
- Web server and middleware wiring:
  - `web/web.go`
  - `web/middleware/*`
- Controllers (routes):
  - `web/controller/index.go`
  - `web/controller/xui.go`
  - `web/controller/api.go`
  - `web/controller/inbound.go`
  - `web/controller/server.go`
  - `web/controller/setting.go`
  - `web/controller/xray_setting.go`
- Services (business logic):
  - `web/service/inbound.go`
  - `web/service/server.go`
  - `web/service/setting.go`
  - `web/service/xray.go`
  - `web/service/xray_setting.go`
  - `web/service/outbound.go`
  - `web/service/tgbot.go`
  - `web/service/warp.go`
- Subscription:
  - `sub/sub.go`
  - `sub/subController.go`
  - `sub/subService.go`
  - `sub/subJsonService.go`
- DB/models:
  - `database/db.go`
  - `database/model/model.go`
  - `xray/client_traffic.go`
- Jobs:
  - `web/job/*.go`
- Wiki snapshot used:
  - `../3x-ui.wiki/*.md`

---

## 14. Suggested maintenance routine

Daily:
- Check Overview status and xray health.
- Check logs for abnormal auth attempts/errors.

Weekly:
- Verify backup export/import viability.
- Review depleted/expired clients and cleanup.
- Review fail2ban and firewall rules.

Monthly:
- Rotate sensitive secrets where feasible.
- Review panel path/port exposure and TLS validity.
- Re-audit settings after version updates.

---

## 15. User-perspective panel operating guide

This section is intentionally UI-first: what you see, what to click, and what happens.

Version context:
- Labels below are based on your observed panel (`v2.8.10`, English language).
- If your language/theme differs, icon+position are usually stable even when text changes.

## 15.1 Login screen (`/`)

Visible items:
- `Username` input.
- `Password` input.
- Optional `Code` input (appears when 2FA is enabled).
- `Log In` button.
- Top-right settings button (theme + language).

How to use:
1. Enter username and password.
2. If 2FA enabled, enter the one-time code.
3. Click `Log In`.
4. On success, panel opens `Overview`.

Failure behavior:
- Wrong credentials show toast error.
- Login attempts are logged and may notify Telegram (if enabled).

## 15.2 Sidebar navigation

Main left menu items:
- `Overview`
- `Inbounds`
- `Panel Settings`
- `Xray Configs`
- `Log Out`

Top utility item:
- `Theme` switch.

How to use:
1. Use menu to move between modules.
2. `Log Out` clears your session and returns to login.

## 15.3 Overview page (`/panel/`)

What this page is for:
- Quick health check and service control.

Typical cards/buttons:
- CPU/RAM/Swap/Storage cards.
- Xray status card (`Running`/`Stopped`) with `Stop` and `Restart`.
- Xray version control button.
- Management shortcuts: `Logs`, `Config`, `Backup`.
- System usage, speed, total traffic, connection counts, uptime.

How to operate safely:
1. Check top warning banners first (security/config warnings).
2. Confirm Xray is `Running`.
3. Use `Restart` after major config edits.
4. Use `Logs` when users report failures.
5. Use `Backup` before risky changes.

## 15.4 Inbounds page (`/panel/inbounds`)

This is the main daily operations page.

Top summary cards:
- Total Sent/Received.
- Total Usage.
- All-time Total Usage.
- Total Inbounds.
- Clients count.

Toolbar buttons:
- `Add Inbound`: create a new listener.
- `General Actions`: mass operations across inbounds/clients.
- `Sync` icon: refresh table/state.
- Download icon/menu: export-related actions (depends on build/UI state).
- Filter toggle + Search box.

Inbound row controls:
- Expand button: opens nested client details for that inbound.
- Row `...` menu: per-inbound actions.
- Enable switch: on/off without deleting row.

Observed row `...` actions and intent:
- `Edit`: modify inbound protocol/stream/security params.
- `Add Client`: add one client under this inbound.
- `Add Bulk`: create many clients in one operation.
- `Reset Clients Traffic`: reset client traffic counters in this inbound.
- `Export All URLs`: copy/generate all client connection URLs.
- `Export All URLs - Subscription`: copy/generate subscription-style URLs.
- `Delete Depleted Clients`: remove clients that expired/exhausted traffic.
- `Export Inbound`: export inbound config object.
- `Reset Traffic`: reset inbound-level traffic usage.
- `Clone`: duplicate inbound config as a new inbound.
- `Delete`: remove the inbound.

Recommended working pattern:
1. Create inbound with `Add Inbound`.
2. Immediately add at least one client.
3. Verify inbound switch is enabled.
4. Test with exported URL on a client app.
5. Monitor traffic growth and online status.

## 15.5 Panel Settings page (`/panel/settings`)

This page controls panel behavior and administration.

Global buttons at top:
- `Save`: writes pending changes to DB.
- `Restart Panel`: applies panel-side changes.

Critical rule:
- Save does not always fully apply runtime behavior until restart.

Top tabs:
- `General`
- `Authentication`
- `Telegram Bot`
- `Subscription`

### General tab

Primary fields and what they do:
- `Listen IP`: bind panel server to specific interface.
- `Listen Domain`: optional host/domain validation.
- `Listen Port`: panel TCP port.
- `URI Path`: panel base path (`webBasePath`) must start/end with `/`.
- `Session Duration`: login persistence in minutes.
- `Pagination Size`: rows per page in inbounds table.
- `Language`: panel UI language.

Common change flow:
1. Update field.
2. Click `Save`.
3. Click `Restart Panel`.
4. Reopen panel at updated URL/path if changed.

### Authentication tab

Sections:
- `Admin credentials`:
  - Current username/password.
  - New username/password.
  - `Confirm` button.
- `Two-factor authentication`:
  - Enable/disable/setup 2FA token flow.

Change flow:
1. Fill current and new credentials.
2. Click `Confirm`.
3. Re-login with new credentials to verify.
4. Enable 2FA and test one full logout/login cycle.

### Telegram Bot tab

General section includes:
- `Enable Telegram Bot` switch.
- `Telegram Token`.
- `Admin Chat ID` (comma-separated possible).
- Bot language.

Additional sections:
- Notifications behavior/timing.
- Proxy and API server options.

Use case flow:
1. Enable switch.
2. Paste token from BotFather.
3. Add your chat ID(s).
4. Save and restart panel.
5. Trigger a test action (e.g., login notification/backup) and verify receipt.

### Subscription tab

Core controls:
- `Subscription Service` switch.
- `JSON Subscription` switch.
- `Listen IP`, `Listen Domain`, `Listen Port`.
- `URI Path` (link endpoint path, e.g. `/sub/`).

Common secure setup:
1. Enable subscription.
2. Set non-default path(s).
3. Configure domain/certificates.
4. Save + restart.
5. Validate link endpoint and (if enabled) JSON endpoint.

## 15.6 Xray Configs page (`/panel/xray`)

Global buttons at top:
- `Save`: persist template/config changes.
- `Restart Xray`: apply xray runtime changes.

Critical rule:
- Most xray config edits need both Save and Restart Xray to be active.

Top tabs:
- `Basics`
- `Routing Rules`
- `Outbounds`
- `Reverse`
- `Balancers`
- `DNS`
- `Advanced`

### Basics

What to change here:
- General runtime strategy fields.
- Log/statistics behavior.
- `Outbound Test URL` used by outbound tests.

When to use:
- Initial deployment cleanup.
- Debugging route and DNS behavior.

### Routing Rules

What it controls:
- Match conditions and outbound target per rule.

How to use:
1. Add rule.
2. Set match condition (domain/ip/network/etc.).
3. Select outbound tag destination.
4. Save + restart Xray.
5. Validate by testing target domains/services.

### Outbounds

Common controls:
- Add outbound.
- WARP creation/registration.
- Test outbound connectivity.
- Outbound traffic table and reset actions.

How to use:
1. Add outbound with valid protocol details.
2. Use test action to verify connectivity/latency.
3. Attach routing rule to actually send traffic there.

### Reverse

Purpose:
- Reverse proxy structures in Xray config.

Use only when needed:
- More advanced topologies; easy to misconfigure.

### Balancers

Purpose:
- Group outbounds and balance traffic by strategy.

Practical note:
- Set clear tags and test routes before production use.

### DNS

Purpose:
- Built-in DNS behavior for Xray.
Practical note:
- DNS changes can impact all traffic patterns; test carefully.

### Advanced

Purpose:
- Raw template editing for full-control scenarios.

Safe editing workflow:
1. Export/backup current config first.
2. Make small incremental changes.
3. Save.
4. Restart Xray.
5. Check logs immediately for parse/runtime errors.

## 15.7 Security alert banner behavior

When shown, it is actionable. Common warnings:
- No TLS for panel.
- Default panel port/path.
- Default subscription path.

Treat these as high-priority hardening tasks, not cosmetic notices.

## 15.8 Save/Restart behavior quick table

- Inbounds/client edits:
  - Usually immediate DB change.
  - Runtime update via API/restart logic; verify with status/logs.
- Panel settings edits:
  - Save required.
  - Often requires `Restart Panel`.
- Xray config edits:
  - Save required.
  - Usually requires `Restart Xray`.

## 15.9 “What to click when” quick recipes

Add a user to existing inbound:
1. `Inbounds` -> row `...` -> `Add Client`.
2. Fill client limits/expiry.
3. Save.
4. Export URL and test.

Disable compromised client fast:
1. `Inbounds` -> expand row.
2. Locate client.
3. Disable/delete client.
4. Confirm in online/last-online views.

Rotate panel URL path:
1. `Panel Settings` -> `General` -> `URI Path`.
2. Enter random path ending `/`.
3. Save -> Restart Panel.
4. Reconnect using new URL.

Recover from bad xray edit:
1. `Xray Configs` -> use defaults/reset path.
2. Save.
3. Restart Xray.
4. Check logs and overview status.

## 15.10 User mistakes to avoid

- Editing many settings at once without intermediate tests.
- Forgetting restart after Save.
- Leaving default path/port.
- Enabling Telegram bot but exposing token carelessly.
- Using Advanced tab without backup.

---

## 16. Revision metadata

- Document generated: 2026-02-18 (local time).
- Based on repo: `workspace/open-source/3x-ui` (local main).
- Based on wiki commit: `6408b1c` in local clone `workspace/open-source/3x-ui.wiki`.

---

## 17. Live instance addendum (practical operations and standards)

This section captures validated behavior from hands-on operations in a live 3x-ui panel session.

### 17.1 Client management reality: “single place” vs “inbound-scoped”

Important model:
- In 3x-ui, clients are created under inbounds.
- There is no native global user object automatically attached to all inbounds.
- Expiry/traffic/IP-limit are set per client entry inside each inbound.

Practical way to manage “as one user”:
1. Reuse one stable identifier (`email`) across all inbounds for that user.
2. Optionally reuse same UUID where protocol/client behavior allows (commonly VLESS workflows).
3. Update quota/expiry on each inbound client entry.
4. Use client traffic/search views/API by `email` to track usage consistently.

### 17.2 API endpoints to automate cross-inbound client ops

Useful API patterns for centralized tooling:
- `POST /panel/api/inbounds/addClient`
- `POST /panel/api/inbounds/updateClient/:clientId`
- `GET /panel/api/inbounds/getClientTraffics/:email`

Recommended automation pattern:
- Keep one “master user policy” (days, GB, ip-limit).
- Apply the policy to a list of inbound IDs via API.
- Use `email` as the logical cross-inbound key.

### 17.3 Inbound naming standard (recommended)

Use deterministic names:
- `<protocol>-<transport>-<security>-<port>-<role>`

Examples:
- `vless-reality-tcp-443-main`
- `vless-reality-tcp-8443-alt`
- `vless-tcp-http-18080-test`

Why this works:
- Immediate readability in crowded tables.
- Faster incident response (role and port visible).
- Lower chance of editing the wrong inbound.

### 17.4 Safe cleanup order for production panels

Apply changes in this order:
1. Rename remarks first (zero functional risk).
2. Verify rows after each save (port/protocol/security unchanged).
3. Only then change functional settings (security/transport/sni/etc.) in isolated steps.
4. Keep one known-good main inbound untouched while testing alternatives.

### 17.5 Current validated inbound organization pattern

Observed clean structure pattern:
- Main production inbound on 443 with Reality.
- Secondary fallback Reality inbound on alternate TLS-like port (e.g. 8443).
- Separate explicitly-labeled test inbound for experiments.

Operational guidance:
- Keep test inbound clearly labeled (`-test`) and isolated from production users.
- Remove or disable test inbound when no longer needed.
- Avoid mixing experimental header/obfuscation settings into production remarks.

### 17.6 Security and control-plane warning handling

If panel shows connection-security warning (no TLS on panel access):
- Treat as a control-plane hardening task.
- Do not enter sensitive credentials over untrusted links.
- Prefer TLS-terminated panel access path as soon as possible.

### 17.7 Non-breaking tuning rules for existing inbounds

For active user inbounds:
- Do not change port/security/transport unexpectedly.
- Do not rotate keys/shortIds without a migration window.
- Do not disable inbounds with active clients before notification.

For cleanup that is safe anytime:
- Rename remarks.
- Standardize client email/label patterns.
- Review per-client quota/expiry/IP-limit consistency.

---

## 18. Custom feature: centralized client management (implemented)

This repository now includes a **client-first management layer** that stays compatible with 3x-ui inbound-scoped architecture.

### 18.1 What was added

- New panel page: `/panel/clients`
- New sidebar menu item: `Clients Center`
- New authenticated API group:
  - `GET /panel/api/clients/list`
  - `GET /panel/api/clients/inbounds`
  - `POST /panel/api/clients/add`
  - `POST /panel/api/clients/update/:id`
  - `POST /panel/api/clients/del/:id`
- New database models:
  - `master_clients` (central profile: name/quota/expiry/ip-limit/enable/comment)
  - `master_client_inbounds` (mapping master client -> assigned inbounds + underlying assignment email/client key)

### 18.2 Architectural approach

Why this design:
- Native 3x-ui clients are inbound-scoped.
- Core traffic/ip logic is email-centric and inbound-linked.
- A deep core rewrite is high-risk.

Implemented solution:
- Keep native inbound clients as the execution layer.
- Add master client profiles as orchestration layer.
- On assignment, create/update/remove corresponding inbound clients automatically.

### 18.3 User workflow now

1. Open `Clients Center`.
2. Create a master client with:
   - Display name
   - Email prefix
   - Traffic quota (GB)
   - Expiry date/time
   - IP limit
   - Enabled flag
   - Comment
3. Select one or more inbounds.
4. Save.

Result:
- Assigned inbound client records are created and kept in sync from this master profile.
- Updating a master client syncs limits/expiry/enabled/comment to all assigned inbounds.
- Removing assignments detaches from those inbounds.

### 18.4 Important behavior details

- Assignment emails are generated uniquely per inbound to stay compatible with existing uniqueness checks.
- The feature supports multi-client protocols (VLESS/VMESS/Trojan/Shadowsocks).
- If removing a client would leave an inbound with zero clients, detach can fail by design (safety guard inherited from inbound logic).

### 18.5 Stability improvement included

A panic-safe guard was added in Xray API client methods (`AddInbound`, `DelInbound`, `AddUser`, `RemoveUser`) to return a clean error when handler service is not initialized, instead of nil-pointer panic.

### 18.6 Local validation performed

Environment:
- Local run with SQLite (`XUI_DB_FOLDER` custom path), custom log folder, panel on local port.

Validated:
- Login works.
- `GET /panel/api/clients/list` works.
- `GET /panel/api/clients/inbounds` works.
- `POST /panel/api/clients/add` works.
- `POST /panel/api/clients/update/:id` works (including inbound assignment sync).
- `POST /panel/api/clients/del/:id` works.
- Browser UI route `/panel/clients` renders.
- UI create flow works (form -> save -> row visible in table).

### 18.7 Files touched for this feature

- `database/model/model.go`
- `database/db.go`
- `web/service/client_center.go`
- `web/controller/client_center.go`
- `web/controller/api.go`
- `web/controller/xui.go`
- `web/html/component/aSidebar.html`
- `web/html/clients.html`
- `xray/api.go`

### 18.8 Future hardening suggestions

- Add explicit role/permission checks for client-center APIs in multi-admin scenarios.
- Add audit logs for master-client create/update/delete operations.
- Add automated integration tests for add/update/remove assignment scenarios.

---

## 19. Dev workflow additions (implemented)

This section captures engineering workflow additions made during implementation.

### 19.1 Air live-reload setup

Added:
- `.air.toml`

Behavior:
- Builds dev binary to `tmp/bin/3x-ui-dev`.
- Runs panel with:
  - `XUI_DB_FOLDER=$PWD/tmp/db`
  - `XUI_LOG_FOLDER=$PWD/tmp/logs`
  - `XUI_DEBUG=true`
- Auto-creates `tmp/db`, `tmp/logs`, `tmp/bin`.
- Initializes fresh local DB automatically (admin/admin on port 2099) if DB does not exist.

Run:
1. `air -c .air.toml`
2. Open `http://127.0.0.1:2099`
3. Login `admin/admin` (fresh dev DB case)

### 19.2 Justfile command setup

Added:
- `justfile`

Common commands:
- `just help`
- `just ensure-tmp`
- `just init-dev`
- `just run`
- `just air`
- `just build`
- `just check`
- `just api-login`
- `just api-clients-inbounds`
- `just api-clients-list`
- `just clean-tmp`

All commands are scoped to local `tmp/` so production paths are untouched.

### 19.3 Local test execution summary

Validated locally after implementation:
- Build success with `go build ./...`.
- App starts with local sqlite paths under `tmp/`.
- New route `/panel/clients` renders.
- New APIs respond correctly with authenticated session.
- Client-center create/update/delete flows work end-to-end.
- Assignment sync between master clients and inbounds works.

### 19.4 Stability fix included during testing

In local test conditions where Xray handler client may be unavailable, API operations previously could panic.
Fix applied in `xray/api.go`:
- Guard `AddInbound`, `DelInbound`, `AddUser`, `RemoveUser` against nil handler service client.
- Return explicit error instead of panic.

---

## 20. Guide split map (new focused docs)

This monolithic document is now complemented by focused files under:
- `docs/panel-guide/`

Index:
- `docs/panel-guide/README.md`
- `docs/panel-guide/01-overview-and-architecture.md`
- `docs/panel-guide/02-pages-and-operations.md`
- `docs/panel-guide/03-api-and-cli-reference.md`
- `docs/panel-guide/04-security-and-hardening.md`
- `docs/panel-guide/05-client-management-model.md`
- `docs/panel-guide/06-custom-client-center-feature.md`
- `docs/panel-guide/07-dev-and-testing-workflow.md`
- `docs/panel-guide/08-protocols-and-config-analysis.md`
- `docs/panel-guide/09-xray-protocol-reference.md`
- `docs/panel-guide/10-glossary-and-concepts.md`
- `docs/panel-guide/11-troubleshooting-runbook.md`
- `docs/panel-guide/12-change-management-and-rollout.md`
- `docs/panel-guide/13-feature-file-map-and-decision-log.md`
- `docs/panel-guide/14-marzban-inspired-roadmap.md`
- `docs/panel-guide/99-session-context-transfer-2026-02-18.md`

Use those files for day-to-day operations and implementation reference.
