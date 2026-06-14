#!/usr/bin/env bash
#
# x-ui-firstboot.sh — generate per-instance 3x-ui panel credentials on first boot.
#
# A golden image (AMI / qcow2) MUST ship without an initialized x-ui.db: the
# panel seeds a hardcoded admin/admin user and generates its session secret +
# panel GUID on first start, so a baked DB would make every clone share the same
# credentials and secret. This script runs ONCE, before x-ui.service starts, and
# replaces the default admin with fresh random credentials on a random high port.
#
# Idempotent: a sentinel file guards against re-running. If a non-default admin
# already exists (operator pre-configured the box), regeneration is skipped.
#
# Wired up by deploy/packer/scripts/provision.sh; ordered Before=x-ui.service.

set -u

SENTINEL="/etc/x-ui/.firstboot-done"
CRED_FILE="/etc/x-ui/credentials.txt"
MOTD_FILE="/etc/motd"
XUI_DIR="${XUI_MAIN_FOLDER:-/usr/local/x-ui}"
XUI_BIN="${XUI_DIR}/x-ui"

log() { echo "[x-ui-firstboot] $*"; }

# Already provisioned — nothing to do (idempotent on re-run / re-image).
if [ -f "$SENTINEL" ]; then
    log "sentinel $SENTINEL present; skipping."
    exit 0
fi

if [ ! -x "$XUI_BIN" ]; then
    log "ERROR: x-ui binary not found at $XUI_BIN"
    exit 1
fi

# Inherit DB configuration (sqlite default; postgres via XUI_DB_TYPE/XUI_DB_DSN)
# from the same env files the systemd unit loads, so the binary talks to the
# same database the panel will use.
for ef in /etc/default/x-ui /etc/conf.d/x-ui /etc/sysconfig/x-ui; do
    if [ -r "$ef" ]; then
        set -a
        # shellcheck disable=SC1090
        . "$ef"
        set +a
    fi
done

install -d -m 755 /etc/x-ui 2> /dev/null || true

# Defense-in-depth: make sure the panel is not running while we mutate the DB.
if command -v systemctl > /dev/null 2>&1; then
    systemctl stop x-ui > /dev/null 2>&1 || true
fi

gen_random_string() {
    local length="$1"
    openssl rand -base64 $((length * 2)) | tr -dc 'a-zA-Z0-9' | head -c "$length"
}

# Best-effort public IPv4 for the displayed access URL (cosmetic only — the
# panel binds 0.0.0.0). Falls back to the primary local IP, then a placeholder.
detect_ip() {
    local ip=""
    local url
    for url in https://api4.ipify.org https://ipv4.icanhazip.com https://4.ident.me; do
        ip=$(curl -fsS4 --max-time 3 "$url" 2> /dev/null | tr -d '[:space:]')
        if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "$ip"
            return 0
        fi
    done
    ip=$(hostname -I 2> /dev/null | awk '{print $1}')
    if [ -n "$ip" ]; then
        echo "$ip"
        return 0
    fi
    echo "<server-ip>"
}

# Detect whether the seeded admin/admin default is still in place.
default_creds=$("$XUI_BIN" setting -show true 2> /dev/null | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')

# The parse MUST yield exactly "true" or "false". If the command failed or its
# output format changed, refuse to proceed: do NOT write the sentinel, so the
# next boot retries instead of silently leaving admin/admin in place.
if [ "$default_creds" != "true" ] && [ "$default_creds" != "false" ]; then
    log "ERROR: could not determine credential state (hasDefaultCredential='${default_creds}'); not writing sentinel, will retry next boot."
    exit 1
fi

if [ "$default_creds" = "false" ]; then
    log "non-default admin already configured; skipping credential regeneration."
    {
        echo "3x-ui first-boot: a non-default admin account already exists on this"
        echo "instance, so credentials were left unchanged."
    } > "$MOTD_FILE" 2> /dev/null || true
    : > "$SENTINEL" 2> /dev/null || true
    chmod 600 "$SENTINEL" 2> /dev/null || true
    exit 0
fi

log "generating per-instance credentials..."

NEW_USER="${XUI_USERNAME:-$(gen_random_string 10)}"
NEW_PASS="${XUI_PASSWORD:-$(gen_random_string 16)}"
NEW_PATH="${XUI_WEB_BASE_PATH:-$(gen_random_string 18)}"
NEW_PORT="${XUI_PANEL_PORT:-$(shuf -i 1024-62000 -n 1)}"

# Clean settings slate: drops any baked port/webBasePath and forces the panel
# to regenerate its session secret + panel GUID on next start (per-instance).
"$XUI_BIN" setting -reset > /dev/null 2>&1 || true

# Apply fresh random identity. UpdateFirstUser renames the seeded admin row and
# rehashes the password, so admin/admin no longer exists after this call.
if ! "$XUI_BIN" setting -username "$NEW_USER" -password "$NEW_PASS" -port "$NEW_PORT" -webBasePath "$NEW_PATH" > /dev/null 2>&1; then
    log "ERROR: failed to apply new panel settings."
    exit 1
fi

API_TOKEN=$("$XUI_BIN" setting -getApiToken true 2> /dev/null | grep -Eo 'apiToken: .+' | awk '{print $2}')
SERVER_IP=$(detect_ip)
ACCESS_URL="http://${SERVER_IP}:${NEW_PORT}/${NEW_PATH}"

# Persist credentials for the operator (root-only). Values are shell-escaped
# with %q so the file stays safe to `source` even if a value contains shell
# metacharacters (the smoke test and operators source this file).
umask 077
{
    echo "# 3x-ui per-instance credentials (generated on first boot)"
    printf 'XUI_USERNAME=%q\n' "$NEW_USER"
    printf 'XUI_PASSWORD=%q\n' "$NEW_PASS"
    printf 'XUI_PANEL_PORT=%q\n' "$NEW_PORT"
    printf 'XUI_WEB_BASE_PATH=%q\n' "$NEW_PATH"
    printf 'XUI_ACCESS_URL=%q\n' "$ACCESS_URL"
    printf 'XUI_API_TOKEN=%q\n' "$API_TOKEN"
} > "$CRED_FILE"
chmod 600 "$CRED_FILE" 2> /dev/null || true

# Friendly login banner shown on SSH / console before the panel is reachable.
# /etc/motd is world-readable, so it MUST NOT contain the password or API token;
# those secrets live only in ${CRED_FILE} (mode 600). Show non-secret info only.
cat > "$MOTD_FILE" 2> /dev/null << EOF

========================================================================
  3x-ui panel — per-instance credentials (generated on first boot)
========================================================================
  Access URL : ${ACCESS_URL}
  Username   : ${NEW_USER}

  The password and API token are NOT shown here (this banner is
  world-readable). Read them as root with:
      sudo cat ${CRED_FILE}

  Change the password after login. If no public IP is shown above,
  replace <server-ip> with the address you reach this server on.
========================================================================

EOF

# Mark complete so we never regenerate on subsequent boots.
: > "$SENTINEL" 2> /dev/null || true
chmod 600 "$SENTINEL" 2> /dev/null || true

log "done. Panel will start on port ${NEW_PORT} with a unique admin account."
exit 0
