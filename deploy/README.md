# Cloud deployment (unattended install)

Tooling to ship the 3x-ui panel via unattended install, with **per-instance
credentials generated on first boot** (never `admin/admin`, never a shared
session secret). Works on amd64 and arm64.

| Path | What it is | Use when |
| --- | --- | --- |
| [`cloud-init/`](cloud-init/) | Generic cloud-init user-data (unattended `install.sh`) | Any cloud, no image build |
| [`marketplace/hetzner/`](marketplace/hetzner/) | Hetzner Cloud notes | Hetzner deployments |
| [`test/`](test/) | Container smoke test | Verifying the install path |

## How it works

`install.sh` runs unattended when `XUI_NONINTERACTIVE=1` or stdin is not a TTY.
Each instance installs and configures itself with random credentials. See
[`cloud-init/README.md`](cloud-init/README.md).

## Unattended install knobs

`install.sh` reads these env vars in non-interactive mode (all optional; unset ⇒
secure random / default):

`XUI_USERNAME`, `XUI_PASSWORD`, `XUI_PANEL_PORT`, `XUI_WEB_BASE_PATH`,
`XUI_SSL_MODE` (`none`|`ip`|`domain`, default `none`), `XUI_DOMAIN`,
`XUI_ACME_EMAIL`, `XUI_ACME_HTTP_PORT` (ACME HTTP-01 listener port, default `80`),
`XUI_SSL_IPV6` (explicit public IPv6 literal to add to an `ip`-mode cert),
`XUI_SERVER_IP` (fallback IP for the displayed access URL when auto-detection fails),
`XUI_DB_TYPE` (`sqlite`|`postgres`), `XUI_DB_DSN`.

The resulting credentials are written to `/etc/x-ui/install-result.env` (mode 600).

### IPv6 in IP certificates

Unattended installation never discovers or includes IPv6 unless you explicitly
set `XUI_SSL_IPV6` to one public IPv6 address. Unset or empty means IPv4 only;
`auto`, a subnet (`/64`), a zone ID (`%eth0`), and malformed or nonpublic addresses
are rejected. An invalid explicit value fails the SSL setup before stopping the
panel or requesting a certificate.

Interactive installation discovers locally assigned global IPv6 addresses and
offers confirmation, numbered selection, manual entry, or skipping. Stable
addresses are preferred to temporary privacy addresses, and a default-route
source is labelled when available. A routed prefix is not a certificate address.

Dual-stack issuance requires acme.sh **3.1.5+**, socat, and working local IPv4 and
IPv6 listeners. The installer checks both families on the chosen listener port
and sends both listen flags so acme.sh saves them for renewal. External port
**80 must reach that listener over both addresses**, including when
`XUI_ACME_HTTP_PORT` selects another internal port. A local probe cannot verify
the provider firewall or external forwarding; the CA validates each IP SAN.
Failure never silently drops the configured IPv6 or deletes the prior certificate.
