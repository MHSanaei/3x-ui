#!/usr/bin/env bash
#
# smoke-firstboot.sh — verify the first-boot per-instance credential script.
#
# Installs the released x-ui binary into a container WITHOUT a database, runs
# x-ui-firstboot.sh, and asserts:
#   * fresh random credentials are generated (no admin/admin)
#   * /etc/x-ui/credentials.txt (600) and /etc/motd are written
#   * the sentinel is created and a second run is a no-op (creds unchanged)
#
# Requires Docker and network access. Usage: bash deploy/test/smoke-firstboot.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE="${SMOKE_IMAGE:-ubuntu:24.04}"

if ! command -v docker > /dev/null 2>&1; then
    echo "ERROR: docker is required for this smoke test." >&2
    exit 1
fi

echo "== first-boot credential smoke test (image: $IMAGE) =="

docker run --rm \
    -v "${REPO_ROOT}/deploy/firstboot/x-ui-firstboot.sh:/root/x-ui-firstboot.sh:ro" \
    -e DEBIAN_FRONTEND=noninteractive \
    "$IMAGE" bash -euo pipefail -c '
        apt-get update -qq
        apt-get install -y -qq curl tar openssl ca-certificates jq > /dev/null

        echo "--- installing released x-ui binary (no DB, no systemd) ---"
        REPO=MHSanaei/3x-ui
        ARCH=$(dpkg --print-architecture)   # amd64 | arm64
        echo "container arch: $ARCH"
        VER=$(curl --fail --location --silent --show-error \
            --retry 5 --retry-all-errors --retry-delay 3 \
            --connect-timeout 15 --max-time 60 \
            "https://api.github.com/repos/${REPO}/releases/latest" | jq -r .tag_name)
        [ -n "$VER" ] && [ "$VER" != "null" ] || { echo "FAIL: cannot resolve version"; exit 1; }
        tmp=$(mktemp -d)
        # 504s and other transient GitHub/CDN hiccups are retried; a real HTTP
        # failure (e.g. missing arch asset) still aborts after the retries.
        if ! curl -4 --fail --location --silent --show-error \
            --retry 5 --retry-all-errors --retry-delay 3 \
            --connect-timeout 15 --max-time 300 \
            -o "${tmp}/x.tar.gz" \
            "https://github.com/${REPO}/releases/download/${VER}/x-ui-linux-${ARCH}.tar.gz"; then
            echo "FAIL: cannot download x-ui-linux-${ARCH}.tar.gz (${VER})" >&2; exit 1
        fi
        test -s "${tmp}/x.tar.gz" || { echo "FAIL: downloaded tarball is empty"; exit 1; }
        tar -xzf "${tmp}/x.tar.gz" -C /usr/local/
        chmod +x /usr/local/x-ui/x-ui
        install -m 755 /root/x-ui-firstboot.sh /usr/local/x-ui/x-ui-firstboot.sh

        # Guarantee a clean slate (the image must never ship a DB).
        rm -f /etc/x-ui/x-ui.db /etc/x-ui/.firstboot-done

        echo "--- run 1: generate per-instance credentials ---"
        /usr/local/x-ui/x-ui-firstboot.sh

        test -f /etc/x-ui/.firstboot-done || { echo "FAIL: sentinel not created"; exit 1; }
        test -f /etc/x-ui/credentials.txt || { echo "FAIL: credentials.txt missing"; exit 1; }
        perms=$(stat -c %a /etc/x-ui/credentials.txt)
        [ "$perms" = "600" ] || { echo "FAIL: credentials.txt perms=$perms (want 600)"; exit 1; }
        grep -q "3x-ui" /etc/motd || { echo "FAIL: motd not written"; exit 1; }

        # shellcheck disable=SC1090
        . /etc/x-ui/credentials.txt
        [ -n "${XUI_USERNAME:-}" ] && [ "$XUI_USERNAME" != "admin" ] \
            || { echo "FAIL: username missing or still admin"; exit 1; }
        first_user="$XUI_USERNAME"

        /usr/local/x-ui/x-ui setting -show | grep -q "hasDefaultCredential: false" \
            || { echo "FAIL: hasDefaultCredential is not false"; exit 1; }

        echo "--- run 2: must be a no-op (sentinel honored) ---"
        /usr/local/x-ui/x-ui-firstboot.sh
        # shellcheck disable=SC1090
        . /etc/x-ui/credentials.txt
        [ "$XUI_USERNAME" = "$first_user" ] \
            || { echo "FAIL: credentials changed on re-run"; exit 1; }

        echo "SMOKE_PASS: firstboot user=$first_user (stable across re-run)"
    '

echo "== first-boot smoke test PASSED =="
