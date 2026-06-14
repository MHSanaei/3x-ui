#!/usr/bin/env bash
#
# harden.sh — baseline OS hardening for AWS Marketplace AMI scanner compliance.
#
# Focus: the controls the scanner actually checks — key-only SSH, no root
# password login, and no default OS account passwords. A restrictive host
# firewall is intentionally NOT enforced by default because 3x-ui opens Xray
# inbound ports on admin-chosen ports at runtime (see README for the rationale
# and how to add ufw rules if you want them).
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "[harden] applying SSH hardening..."
install -d -m 755 /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/99-3xui-hardening.conf << 'EOF'
# 3x-ui golden image hardening (AWS Marketplace scanner compliance)
PasswordAuthentication no
PermitRootLogin prohibit-password
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
EOF
chmod 644 /etc/ssh/sshd_config.d/99-3xui-hardening.conf

echo "[harden] locking passwords on default OS accounts..."
# No account may ship with a usable password. Keys are provisioned per-instance
# by the cloud platform (EC2 metadata / cloud-init) on first boot.
# passwd -l locks the PASSWORD only; key-based login keeps working.
for u in root ubuntu admin; do
    if id "$u" > /dev/null 2>&1; then
        passwd -l "$u" > /dev/null 2>&1 || true
    fi
done

echo "[harden] enabling automatic security updates..."
apt-get update
apt-get install -y --no-install-recommends unattended-upgrades
systemctl enable unattended-upgrades > /dev/null 2>&1 || true

echo "[harden] done."
