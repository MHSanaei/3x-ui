[English](/docs/self-steal-reality.md) | [Русский](/docs/self-steal-reality.ru_RU.md)

# Self-Steal Setup for VLESS+Reality

## Why

REALITY hides the VPN handshake inside the TLS handshake of a real website — the "dest" (e.g.
`www.amazon.com` or `www.apple.com`). Anyone probing the server without a valid REALITY key is
transparently forwarded to that external site and sees its actual content.

This works, but you don't own that site. You have no control over what a determined observer
sees on deeper inspection, no visibility into when its certificate rotates or its TLS
configuration changes, and no logs of who is probing your server without a key — those requests
are physically served by someone else's infrastructure, not yours.

**Self-steal** fixes this by pointing REALITY's dest at an nginx instance running on the same
server. It's the same "steal a TLS identity" mechanism, just aimed at yourself — which gives you
full control over:
- what anyone hitting the domain without a REALITY key actually sees;
- which certificate is served and when it rotates;
- logs of every fallback attempt, with real client IPs instead of a single `127.0.0.1`.

## Prerequisites

- 3x-ui installed and running.
- A working VLESS+REALITY inbound on TCP:443 with a normal external dest.
- A domain pointed at the server, with a Let's Encrypt certificate already issued (acme.sh is
  usually set up alongside the panel).
- Root SSH access.

## Dest requirements — this is enforced, not a suggestion

REALITY doesn't accept an arbitrary site as dest. The panel's own source
(`internal/web/service/reality_scan.go`, function `probeRealityAddr`) checks:

```go
res.Feasible = res.TLS13 && res.H2 && res.X25519 && res.CertValid
```

The dest must simultaneously answer with **TLS 1.3**, **HTTP/2** (ALPN `h2`), an **X25519** key
exchange, and a **valid certificate** matching the requested domain. This isn't arbitrary —
REALITY hides its authentication signature specifically in the X25519 fields of the TLS 1.3
handshake; without that exact combination there's nowhere for the signature to live. Your own
nginx has to meet the same bar — a default install does not guarantee this on its own.

---

## Step 1 — Install nginx from the official repo

The Ubuntu-packaged nginx is usually a couple of stable releases behind nginx.org. For REALITY,
use the current upstream build:

```bash
apt install -y curl gnupg2 ca-certificates lsb-release ubuntu-keyring
curl -s https://nginx.org/keys/nginx_signing.key | gpg --dearmor | tee /usr/share/keyrings/nginx-archive-keyring.gpg >/dev/null
echo "deb [signed-by=/usr/share/keyrings/nginx-archive-keyring.gpg] http://nginx.org/packages/ubuntu $(lsb_release -cs) nginx" > /etc/apt/sources.list.d/nginx.list
printf 'Package: *\nPin: origin nginx.org\nPin-Priority: 900\n' > /etc/apt/preferences.d/99nginx
apt update
apt install -y nginx
```

Confirm your distro codename (`lsb_release -cs`) is actually supported by nginx.org:
`curl -s http://nginx.org/packages/ubuntu/dists/ | grep -oE 'href="[a-z]+/"'`.

> **Trap:** if the server has a leftover distro-packaged nginx (e.g. `nginx-common` sitting in
> `rc` state — installed before, removed incompletely), **do not `apt purge` it on its own**.
> dpkg may consider `/etc/nginx/nginx.conf`, `/etc/nginx/conf.d/default.conf`, and all of
> `/var/log/nginx/` as belonging to that old package and delete them along with it. If this has
> already happened, the fix is a full `apt purge nginx && apt install nginx` cycle — not
> `--reinstall`, which has its own bug where conffiles aren't restored if they're physically
> missing but dpkg's database still thinks they're "unmodified".

## Step 2 — Build a convincing decoy site

This step is easy to underestimate. **Do not use nginx's default placeholder page**
("Welcome to nginx!") — it instantly signals a server that was hastily configured to hide
something, not to run an actual business.

You need a real-looking landing page:
- Real copy, not lorem ipsum, not a placeholder.
- At least 2-3 inner pages with genuine content (`/privacy`, `/terms`) — links to nowhere
  (`href="#"`) are a visible tell under manual inspection.
- A `robots.txt`.
- A neutral theme unrelated to your actual infrastructure. The decoy's contact email should
  **not** share a name with your VPN domain/project — otherwise there's a direct, searchable
  correlation between the "ordinary site" and your VPN infrastructure.

Put it in `/var/www/<node>-decoy/`. For clean URLs without `.html`, use a directory-index
layout — `/privacy/index.html`, `/terms/index.html` — nginx will resolve `/privacy` → redirect →
`/privacy/` → serve `index.html` inside, via the standard `index` + `try_files` combination.

## Step 3 — nginx config

Create `/etc/nginx/conf.d/<node>-selfsteal.conf`:

```nginx
# Plain HTTP -> HTTPS redirect, matching what any real site does. This is
# required from day one: the nginx.org package ships its own conf.d/default.conf
# with "listen 80; server_name localhost;". Without a dedicated block for your
# own domain, that default.conf stays the only listener on port 80 — meaning
# ALL HTTP traffic, including requests to your domain, falls through to it and
# gets the bare "Welcome to nginx!" page instead of your decoy. The mismatch is
# obvious: a fully-built site on 443, an unconfigured-looking server on 80 —
# exactly the kind of inconsistency that stands out on inspection.
server {
    listen 80;
    server_name <DOMAIN>;
    return 301 https://$host$request_uri;
}

server {
    listen 127.0.0.1:8443 ssl proxy_protocol;
    http2 on;
    server_name <DOMAIN>;

    # Xray forwards "stolen" (unauthenticated) connections here with xver=1
    # (PROXY protocol v1). Without this, $remote_addr is always 127.0.0.1 —
    # Xray's own loopback hop, not the real client.
    set_real_ip_from 127.0.0.1;
    real_ip_header proxy_protocol;

    ssl_certificate     /root/cert/<DOMAIN>/fullchain.pem;
    ssl_certificate_key /root/cert/<DOMAIN>/privkey.pem;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ecdh_curve X25519:secp256r1;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 1d;

    access_log /var/log/nginx/<node>-decoy-access.log;
    error_log  /var/log/nginx/<node>-decoy-error.log;

    # nginx's real listen port (8443) is internal-only — never leak it into
    # generated redirects (e.g. the default directory-index redirect
    # /privacy -> /privacy/ builds an absolute Location with the server's
    # port by default, which would give away the proxying setup).
    absolute_redirect off;

    root /var/www/<node>-decoy;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }
}
```

Note that nginx listens on **127.0.0.1:8443**, not the public 443 directly. The public port stays
with Xray — it's Xray that decides, based on the REALITY handshake, whether to forward a
connection to this internal address (unauthenticated clients) or serve it as real VLESS
(clients with a valid key).

**On the certificate:** reuse the domain's existing acme.sh-issued certificate (the same one the
panel or a Hysteria2 inbound already uses) — don't issue a separate one, it's the same domain.

Test and start:
```bash
nginx -t && systemctl enable --now nginx
```

## Step 4 — Make certificate renewal reload nginx too

By default, acme.sh's renewal hook only restarts the panel (x-ui). Extend it so it also reloads
nginx:

```bash
~/.acme.sh/acme.sh --install-cert -d <DOMAIN> --ecc \
  --key-file       /root/cert/<DOMAIN>/privkey.pem \
  --fullchain-file /root/cert/<DOMAIN>/fullchain.pem \
  --reloadcmd 'systemctl restart x-ui || rc-service x-ui restart; systemctl reload nginx'
```

Verify it stuck: `cat ~/.acme.sh/<DOMAIN>_ecc/<DOMAIN>.conf | grep reloadcmd` (the value is
base64-encoded — decode with `base64 -d`).

## Step 5 — Update the inbound through the panel UI

Everything you need is right there in the form — no need to touch the API, let alone SQLite
directly, for a regular one-off setup.

Inbounds → the VLESS inbound → **Security** tab:
- **Target** → `127.0.0.1:8443`;
- **SNI** → your domain only (clear out the whole list of external domains that was there for
  the old dest);
- **Xver** → `1` (enables PROXY protocol v1 — without it nginx only ever sees `127.0.0.1`, never
  the real client IP);
- if mldsa65 was generated at some point (the "Get New Cert" button does this too), clear both
  **mldsa65 Seed** / **mldsa65 Verify** fields right away — see the known bug below.

Save. **You don't need to restart Xray by hand** — the panel runs its own background job that
checks every 30 seconds (`@every 30s`, `internal/web/web.go`) whether a restart-needed flag was
set anywhere, and restarts Xray on its own after any inbound change. Give it half a minute, then
check the live config.

Confirm the change actually took effect (not just that the form saved): read
`/usr/local/x-ui/bin/config.json` on the server and check `xver`/`target`/`serverNames`.

> **Provisioning nodes with scripts instead of by hand?** The same result is reachable through
> the API (Bearer-token auth: Settings → Security → API Token, or already in
> `/etc/x-ui/install-result.env`): `GET /panel/api/inbounds/list` → edit
> `streamSettings.realitySettings.target`/`serverNames`/`xver` in the body → `POST
> /panel/api/inbounds/update/:id` (this replaces the whole object; partial patches aren't
> supported). The same background job picks up the change here too — force
> `POST /panel/api/server/restartXrayService` separately only if you don't want to wait up to
> 30 seconds, e.g. when an automated check runs right after the update.

---

## Known bug: mldsa65 breaks real connections

**Symptom:** a genuine VLESS+REALITY client can't connect — the connection drops with EOF after
a few retries (`common/retry: [EOF] > common/retry: all retry attempts failed`), while the
fallback path (a regular browser without a REALITY key) works fine and gets the decoy site.

**Cause:** a documented Xray-core bug ([issue #5319](https://github.com/XTLS/Xray-core/issues/5319),
closed by maintainers as "not planned"). If the inbound has `mldsa65Seed` (server side,
post-quantum signature) and the matching client-side `mldsa65Verify` populated, real connections
break — regardless of how correctly both values are generated and kept in sync.

**Fix:** on the same **Security** tab, right below the `mldsa65 Seed` / `mldsa65 Verify` fields,
there's a **Clear** button — click it and save the inbound, both fields go empty. Don't try to
configure mldsa65 "correctly" — it's known not to work right now. If the panel auto-generated
these fields when the inbound was created (the "Get New Seed" button does this automatically),
clear them immediately rather than waiting to hit the bug.

> For scripted automation, the same thing via the inbound-update API call:
> ```python
> realitySettings['mldsa65Seed'] = ''
> realitySettings['settings']['mldsa65Verify'] = ''
> ```

---

## Verification — with live requests, not by inspection

Don't trust that "the config looks right." Verify it live.

### 1. Fallback path (no REALITY key), from any external host

```bash
# TLS parameters
echo | openssl s_client -connect <IP>:443 -servername <DOMAIN> -tls1_3 2>&1 | grep -E 'subject=|issuer=|Protocol|Verify return'
echo | openssl s_client -connect <IP>:443 -servername <DOMAIN> -tls1_3 2>&1 | grep -i "temp key"   # should be X25519

# Content
curl -sk --resolve <DOMAIN>:443:<IP> https://<DOMAIN>/ -o /dev/null -w 'HTTP: %{http_code}, version: %{http_version}\n'
```

Expected: TLS 1.3, X25519, a valid certificate for the right domain, HTTP/2, a 200 response.

### 2. The real IP of a probing client shows up in the decoy's logs

Once `xver` + `proxy_protocol` are set, send an external request and check
`tail /var/log/nginx/<node>-decoy-access.log` — it should show the client's actual IP, not
`127.0.0.1`.

### 3. A genuine VLESS+REALITY connection (not just the fallback)

For testing, use **the exact xray binary the panel itself runs**
(`scp root@<node>:/usr/local/x-ui/bin/xray-linux-amd64`), not the latest GitHub release —
version drift alone can produce a misleading EOF unrelated to any actual config problem.

Create a temporary test client through the UI: Clients → **Add Client** → pick the VLESS
inbound, set Flow to `xtls-rprx-vision`, use an email like `selfsteal-test-temp`. Save.

Don't hand-assemble the connection parameters (`publicKey`, `shortId`) from the database or an
API response — the client you just created has a QR/link button in the list that already builds
a complete, correct `vless://` link for you. Pull the values from there (or export the link
directly and import it into any VLESS-compatible client).

You still need a separate xray client process running with these parameters — a `vless`
outbound, `security: reality`, `serverName` set to your domain, `fingerprint: firefox`,
`flow: xtls-rprx-vision`, and **no `mldsa65Verify`**. Route through it via SOCKS5 on
`127.0.0.1:10808` and check:
```bash
curl --socks5-hostname 127.0.0.1:10808 https://ifconfig.me
```
The response should be the node's own IP, not yours.

**Delete the temporary client once you're done testing** — same Clients list, regular delete
button.

---

## Checklist

- [ ] nginx installed from nginx.org, not the distro repo
- [ ] Decoy site is fully built out — real copy, at least two inner pages, `robots.txt`; contact
      email unrelated by name to the VPN infrastructure
- [ ] nginx listens on `127.0.0.1:8443`; the public 443 stays with Xray
- [ ] A dedicated `listen 80` block with a 301 redirect to https is in place — without it, port
      80 serves the stock "Welcome to nginx!" page from `default.conf`
- [ ] `absolute_redirect off` set — otherwise the internal 8443 port leaks into redirects
- [ ] Certificate reused, not separately issued; acme.sh's `--reloadcmd` extended to reload nginx
- [ ] In the panel: Target = `127.0.0.1:8443`, SNI = only your domain, Xver = 1
- [ ] Changes confirmed in the live `config.json` (the background job picks them up within 30
      seconds on its own — forcing a restart manually isn't required)
- [ ] `mldsa65Seed`/`mldsa65Verify` cleared (or never populated)
- [ ] Fallback path verified with live requests (TLS 1.3, X25519, valid certificate, 200)
- [ ] The real client IP shows up in the decoy's logs, not `127.0.0.1`
- [ ] A genuine VLESS+REALITY connection verified with a temporary test client (using the
      panel's own xray binary), and the test client deleted afterward
