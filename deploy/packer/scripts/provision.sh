#!/usr/bin/env bash
#
# provision.sh — install the dune panel into a golden image (Packer).
#
# Self-contained: mirrors install.sh's download/extract logic but DELIBERATELY
# does NOT run config_after_install and does NOT create a database. The image
# must ship without /etc/dune/dune.db so that deploy/firstboot generates unique
# per-instance credentials on first boot. Both dune.service and
# dune-firstboot.service are enabled but NOT started here.
#
# Inputs (from Packer environment_vars):
#   DUNE_VERSION  release tag (e.g. v3.3.1) or 'latest'
#   DUNE_ARCH     amd64 (default) or arm64
set -euo pipefail

DUNE_VERSION="${DUNE_VERSION:-latest}"
DUNE_ARCH="${DUNE_ARCH:-amd64}"
DUNE_DIR="/usr/local/dune"
REPO="leto217/DUNE"
export DEBIAN_FRONTEND=noninteractive

echo "[provision] installing base packages..."
apt-get update
apt-get install -y --no-install-recommends \
    ca-certificates curl tar tzdata socat openssl cron jq

echo "[provision] resolving dune version..."
if [ "$DUNE_VERSION" = "latest" ]; then
    DUNE_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -r '.tag_name')
fi
if [ -z "$DUNE_VERSION" ] || [ "$DUNE_VERSION" = "null" ]; then
    echo "[provision] ERROR: could not resolve dune release tag" >&2
    exit 1
fi
echo "[provision] installing dune ${DUNE_VERSION} (${DUNE_ARCH})"

tarball="dune-linux-${DUNE_ARCH}.tar.gz"
url="https://github.com/${REPO}/releases/download/${DUNE_VERSION}/${tarball}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Download the RELEASED binary tarball (no Go build inside the image).
curl -fL4 --retry 3 -o "${tmp}/${tarball}" "$url"

# Extract into /usr/local/ (the tarball contains an dune/ directory).
systemctl stop dune > /dev/null 2>&1 || true
rm -rf "$DUNE_DIR"
tar -xzf "${tmp}/${tarball}" -C /usr/local/
chmod +x "${DUNE_DIR}/dune" "${DUNE_DIR}/dune.sh"
chmod +x "${DUNE_DIR}"/bin/* 2> /dev/null || true

# Install the dune management CLI.
if [ -f "${DUNE_DIR}/dune.sh" ]; then
    cp -f "${DUNE_DIR}/dune.sh" /usr/bin/dune
else
    curl -fL4 -o /usr/bin/dune "https://raw.githubusercontent.com/${REPO}/main/dune.sh"
fi
chmod +x /usr/bin/dune
mkdir -p /var/log/dune

# Panel systemd unit (Ubuntu base => debian variant).
install -m 644 "${DUNE_DIR}/dune.service.debian" /etc/systemd/system/dune.service

# First-boot per-instance credential unit + script (uploaded to /tmp/firstboot).
install -m 755 /tmp/firstboot/dune-firstboot.sh "${DUNE_DIR}/dune-firstboot.sh"
install -m 644 /tmp/firstboot/dune-firstboot.service /etc/systemd/system/dune-firstboot.service

systemctl daemon-reload
# Enable (start on next boot) but do NOT start now — there is no DB yet.
systemctl enable dune-firstboot.service
systemctl enable dune.service

# Belt-and-braces: ensure no DB / sentinel was created during provisioning.
rm -f /etc/dune/dune.db /etc/dune/dune.db-* /etc/dune/.firstboot-done 2> /dev/null || true

echo "[provision] done — panel installed, services enabled, NO database initialized."
