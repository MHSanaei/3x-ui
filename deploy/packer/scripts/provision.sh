#!/usr/bin/env bash
#
# provision.sh — install the 3x-ui panel into a golden image (Packer).
#
# Self-contained: mirrors install.sh's download/extract logic but DELIBERATELY
# does NOT run config_after_install and does NOT create a database. The image
# must ship without /etc/x-ui/x-ui.db so that deploy/firstboot generates unique
# per-instance credentials on first boot. Both x-ui.service and
# x-ui-firstboot.service are enabled but NOT started here.
#
# Inputs (from Packer environment_vars):
#   XUI_VERSION  release tag (e.g. v3.3.1) or 'latest'
#   XUI_ARCH     amd64 (default) or arm64
set -euo pipefail

XUI_VERSION="${XUI_VERSION:-latest}"
XUI_ARCH="${XUI_ARCH:-amd64}"
XUI_DIR="/usr/local/x-ui"
REPO="MHSanaei/3x-ui"
export DEBIAN_FRONTEND=noninteractive

echo "[provision] installing base packages..."
apt-get update
apt-get install -y --no-install-recommends \
    ca-certificates curl tar tzdata socat openssl cron jq

echo "[provision] resolving 3x-ui version..."
if [ "$XUI_VERSION" = "latest" ]; then
    XUI_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -r '.tag_name')
fi
if [ -z "$XUI_VERSION" ] || [ "$XUI_VERSION" = "null" ]; then
    echo "[provision] ERROR: could not resolve 3x-ui release tag" >&2
    exit 1
fi
echo "[provision] installing 3x-ui ${XUI_VERSION} (${XUI_ARCH})"

tarball="x-ui-linux-${XUI_ARCH}.tar.gz"
url="https://github.com/${REPO}/releases/download/${XUI_VERSION}/${tarball}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Download the RELEASED binary tarball (no Go build inside the image).
curl -fL4 --retry 3 -o "${tmp}/${tarball}" "$url"

# Extract into /usr/local/ (the tarball contains an x-ui/ directory).
systemctl stop x-ui > /dev/null 2>&1 || true
rm -rf "$XUI_DIR"
tar -xzf "${tmp}/${tarball}" -C /usr/local/
chmod +x "${XUI_DIR}/x-ui" "${XUI_DIR}/x-ui.sh"
chmod +x "${XUI_DIR}"/bin/* 2> /dev/null || true

# Install the x-ui management CLI.
if [ -f "${XUI_DIR}/x-ui.sh" ]; then
    cp -f "${XUI_DIR}/x-ui.sh" /usr/bin/x-ui
else
    curl -fL4 -o /usr/bin/x-ui "https://raw.githubusercontent.com/${REPO}/main/x-ui.sh"
fi
chmod +x /usr/bin/x-ui
mkdir -p /var/log/x-ui

# Panel systemd unit (Ubuntu base => debian variant).
install -m 644 "${XUI_DIR}/x-ui.service.debian" /etc/systemd/system/x-ui.service

# First-boot per-instance credential unit + script (uploaded to /tmp/firstboot).
install -m 755 /tmp/firstboot/x-ui-firstboot.sh "${XUI_DIR}/x-ui-firstboot.sh"
install -m 644 /tmp/firstboot/x-ui-firstboot.service /etc/systemd/system/x-ui-firstboot.service

systemctl daemon-reload
# Enable (start on next boot) but do NOT start now — there is no DB yet.
systemctl enable x-ui-firstboot.service
systemctl enable x-ui.service

# Belt-and-braces: ensure no DB / sentinel was created during provisioning.
rm -f /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db-* /etc/x-ui/.firstboot-done 2> /dev/null || true

echo "[provision] done — panel installed, services enabled, NO database initialized."
