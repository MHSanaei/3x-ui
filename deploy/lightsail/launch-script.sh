#!/bin/bash
#
# Amazon Lightsail launch script for 3x-ui (self-service, per-instance creds).
#
# Use it one of two ways when creating an Ubuntu 24.04 Lightsail instance:
#   * Console: "Add launch script" -> paste this file.
#   * CLI:     aws lightsail create-instances --user-data file://launch-script.sh ...
#
# It installs the latest 3x-ui release non-interactively and generates unique
# random credentials for THIS instance. The full credentials land in
# /etc/x-ui/install-result.env (mode 600); /etc/motd shows only the URL + username.
#
# IMPORTANT (Lightsail firewall): Lightsail only opens 22/80/443 by default. The
# panel listens on a random high port, so after boot read the port from
# /etc/x-ui/install-result.env and open it under the instance's Networking tab
# (IPv4 Firewall), or pin a known port below and pre-open it.
set -e
export DEBIAN_FRONTEND=noninteractive

# --- Non-interactive install knobs ------------------------------------------
export XUI_NONINTERACTIVE=1
export XUI_SSL_MODE="${XUI_SSL_MODE:-none}"
# Pin a known panel port so you can pre-open it in the Lightsail firewall
# (otherwise a random high port is chosen). Username/password stay random:
#   export XUI_PANEL_PORT="54321"
# Other optional pins (unset => secure random):
#   export XUI_USERNAME="admin2"
#   export XUI_PASSWORD="change-me"
#   export XUI_WEB_BASE_PATH="panel"
# Domain TLS instead of plain HTTP:
#   export XUI_SSL_MODE="domain" XUI_DOMAIN="panel.example.com" XUI_ACME_EMAIL="you@example.com"
# ----------------------------------------------------------------------------

curl -fsSL https://raw.githubusercontent.com/MHSanaei/3x-ui/main/install.sh | bash

# /etc/motd is world-readable, so it gets ONLY non-secret info (URL + username);
# the full credentials stay in the root-only /etc/x-ui/install-result.env
# (mode 600) — read them with `sudo cat` over SSH.
if [ -r /etc/x-ui/install-result.env ]; then
    # shellcheck disable=SC1091
    . /etc/x-ui/install-result.env
    {
        echo
        echo "=== 3x-ui panel (generated on first boot) ==="
        echo "URL:      ${XUI_ACCESS_URL:-unknown}"
        echo "Username: ${XUI_USERNAME:-unknown}"
        echo "Password + API token: sudo cat /etc/x-ui/install-result.env"
        echo "Open the panel port in the Lightsail IPv4 firewall, then log in."
        echo "============================================="
    } >> /etc/motd 2>/dev/null || true
fi
