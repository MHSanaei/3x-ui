#!/bin/bash
#
# Lightsail snapshot provisioning user-data (used by build-snapshot.sh).
#
# Installs the dune panel into a build instance but creates NO database and
# NO credentials, and enables the first-boot unit. The instance is then snapshot
# so that every instance launched from the snapshot generates its own unique
# credentials on first boot (see deploy/firstboot/).
#
# This is the Lightsail equivalent of deploy/packer/scripts/provision.sh. It is
# NOT for end users — use deploy/lightsail/launch-script.sh for a direct install.
set -e
export DEBIAN_FRONTEND=noninteractive

REPO=leto217/DUNE
DUNE_DIR=/usr/local/dune
RAW="https://raw.githubusercontent.com/${REPO}/main"

apt-get update
apt-get install -y --no-install-recommends \
    ca-certificates curl tar tzdata socat openssl cron jq

ARCH=$(dpkg --print-architecture) # amd64 | arm64
VER=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -r .tag_name)
if [ -z "$VER" ] || [ "$VER" = "null" ]; then
    echo "failed to resolve dune version" >&2
    exit 1
fi

tmp=$(mktemp -d)
curl -fL4 --retry 3 -o "${tmp}/x.tar.gz" \
    "https://github.com/${REPO}/releases/download/${VER}/dune-linux-${ARCH}.tar.gz"

systemctl stop dune > /dev/null 2>&1 || true
rm -rf "$DUNE_DIR"
tar -xzf "${tmp}/x.tar.gz" -C /usr/local/
chmod +x "${DUNE_DIR}/dune" "${DUNE_DIR}/dune.sh"
chmod +x "${DUNE_DIR}"/bin/* 2> /dev/null || true
cp -f "${DUNE_DIR}/dune.sh" /usr/bin/dune
chmod +x /usr/bin/dune
mkdir -p /var/log/dune

# Panel + first-boot systemd units.
install -m 644 "${DUNE_DIR}/dune.service.debian" /etc/systemd/system/dune.service
curl -fL4 -o "${DUNE_DIR}/dune-firstboot.sh" "${RAW}/deploy/firstboot/dune-firstboot.sh"
curl -fL4 -o /etc/systemd/system/dune-firstboot.service "${RAW}/deploy/firstboot/dune-firstboot.service"
chmod 755 "${DUNE_DIR}/dune-firstboot.sh"
chmod 644 /etc/systemd/system/dune-firstboot.service

systemctl daemon-reload
systemctl enable dune-firstboot.service
systemctl enable dune.service

# No DB, no creds in the image — first boot generates them per-instance.
rm -f /etc/dune/dune.db /etc/dune/dune.db-* /etc/dune/.firstboot-done 2> /dev/null || true

# Marker that build-snapshot.sh polls for over SSH.
touch /var/lib/dune-provision-done
echo "[snapshot-userdata] provisioned dune ${VER} (${ARCH}); no DB created."
