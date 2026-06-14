#!/usr/bin/env bash
#
# cleanup.sh — strip all instance-specific state and secrets from the image.
#
# Runs LAST. The output image must contain no panel database, no credentials,
# no SSH host keys, and no baked authorized_keys. Fails the build if any of
# those survive.
set -euo pipefail

echo "[cleanup] removing panel database, credentials and first-boot sentinel..."
rm -f /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db-* 2> /dev/null || true
rm -f /etc/x-ui/install-result.env /etc/x-ui/credentials.txt 2> /dev/null || true
rm -f /etc/x-ui/.firstboot-done 2> /dev/null || true

echo "[cleanup] removing SSH host keys (regenerated on first boot)..."
rm -f /etc/ssh/ssh_host_* 2> /dev/null || true

echo "[cleanup] removing any baked authorized_keys..."
rm -f /root/.ssh/authorized_keys 2> /dev/null || true
find /home -maxdepth 3 -name authorized_keys -type f -delete 2> /dev/null || true

echo "[cleanup] resetting machine-id..."
truncate -s 0 /etc/machine-id 2> /dev/null || true
rm -f /var/lib/dbus/machine-id 2> /dev/null || true
ln -sf /etc/machine-id /var/lib/dbus/machine-id 2> /dev/null || true

echo "[cleanup] resetting cloud-init so it re-runs on the real first boot..."
cloud-init clean --logs --seed > /dev/null 2>&1 || rm -rf /var/lib/cloud/* 2> /dev/null || true

echo "[cleanup] truncating logs, history and package caches..."
find /var/log -type f -exec truncate -s 0 {} + 2> /dev/null || true
rm -rf /var/lib/x-ui /var/log/x-ui/* 2> /dev/null || true
apt-get clean || true
rm -rf /var/lib/apt/lists/* 2> /dev/null || true
rm -f /root/.bash_history 2> /dev/null || true
find /home -maxdepth 3 -name .bash_history -type f -delete 2> /dev/null || true
rm -rf /tmp/firstboot 2> /dev/null || true

echo "[cleanup] verifying the image is clean..."
fail=0
for f in /etc/x-ui/x-ui.db /etc/x-ui/credentials.txt /etc/x-ui/install-result.env /etc/x-ui/.firstboot-done; do
    if [ -e "$f" ]; then
        echo "[cleanup] FATAL: $f is present in the image" >&2
        fail=1
    fi
done
if ls /etc/ssh/ssh_host_* > /dev/null 2>&1; then
    echo "[cleanup] FATAL: SSH host keys present in the image" >&2
    fail=1
fi
if [ -e /root/.ssh/authorized_keys ]; then
    echo "[cleanup] FATAL: /root/.ssh/authorized_keys present in the image" >&2
    fail=1
fi
if [ "$fail" -ne 0 ]; then
    exit 1
fi

echo "[cleanup] OK — no DB, no credentials, no host keys, no authorized_keys."
