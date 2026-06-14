#!/bin/bash
#
# Lightsail snapshot provisioning user-data (used by build-snapshot.sh).
#
# Installs the 3x-ui panel into a build instance but creates NO database and
# NO credentials, and enables the first-boot unit. The instance is then snapshot
# so that every instance launched from the snapshot generates its own unique
# credentials on first boot (see deploy/firstboot/).
#
# This is the Lightsail equivalent of deploy/packer/scripts/provision.sh. It is
# NOT for end users — use deploy/lightsail/launch-script.sh for a direct install.
set -e
export DEBIAN_FRONTEND=noninteractive

REPO=MHSanaei/3x-ui
XUI_DIR=/usr/local/x-ui
RAW="https://raw.githubusercontent.com/${REPO}/main"

apt-get update
apt-get install -y --no-install-recommends \
    ca-certificates curl tar tzdata socat openssl cron jq

ARCH=$(dpkg --print-architecture) # amd64 | arm64
VER=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -r .tag_name)
if [ -z "$VER" ] || [ "$VER" = "null" ]; then
    echo "failed to resolve 3x-ui version" >&2
    exit 1
fi

tmp=$(mktemp -d)
curl -fL4 --retry 3 -o "${tmp}/x.tar.gz" \
    "https://github.com/${REPO}/releases/download/${VER}/x-ui-linux-${ARCH}.tar.gz"

systemctl stop x-ui > /dev/null 2>&1 || true
rm -rf "$XUI_DIR"
tar -xzf "${tmp}/x.tar.gz" -C /usr/local/
chmod +x "${XUI_DIR}/x-ui" "${XUI_DIR}/x-ui.sh"
chmod +x "${XUI_DIR}"/bin/* 2> /dev/null || true
cp -f "${XUI_DIR}/x-ui.sh" /usr/bin/x-ui
chmod +x /usr/bin/x-ui
mkdir -p /var/log/x-ui

# Panel + first-boot systemd units.
install -m 644 "${XUI_DIR}/x-ui.service.debian" /etc/systemd/system/x-ui.service
curl -fL4 -o "${XUI_DIR}/x-ui-firstboot.sh" "${RAW}/deploy/firstboot/x-ui-firstboot.sh"
curl -fL4 -o /etc/systemd/system/x-ui-firstboot.service "${RAW}/deploy/firstboot/x-ui-firstboot.service"
chmod 755 "${XUI_DIR}/x-ui-firstboot.sh"
chmod 644 /etc/systemd/system/x-ui-firstboot.service

systemctl daemon-reload
systemctl enable x-ui-firstboot.service
systemctl enable x-ui.service

# No DB, no creds in the image — first boot generates them per-instance.
rm -f /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db-* /etc/x-ui/.firstboot-done 2> /dev/null || true

# Marker that build-snapshot.sh polls for over SSH.
touch /var/lib/3xui-provision-done
echo "[snapshot-userdata] provisioned 3x-ui ${VER} (${ARCH}); no DB created."
