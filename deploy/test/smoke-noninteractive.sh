#!/usr/bin/env bash
#
# smoke-noninteractive.sh — verify the non-interactive install path.
#
# Runs install.sh inside an Ubuntu container with NO TTY (piped) and
# XUI_NONINTERACTIVE=1, then asserts:
#   * /etc/x-ui/install-result.env exists (mode 600) with random, non-default creds
#   * the panel reports hasDefaultCredential: false (no admin/admin remains)
#   * the panel HTTP server actually serves on the generated port/base path
#
# Requires Docker and network access (install.sh downloads the released binary).
# Usage: bash deploy/test/smoke-noninteractive.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE="${SMOKE_IMAGE:-ubuntu:24.04}"

if ! command -v docker > /dev/null 2>&1; then
    echo "ERROR: docker is required for this smoke test." >&2
    exit 1
fi

echo "== non-interactive install smoke test (image: $IMAGE) =="

docker run --rm \
    -v "${REPO_ROOT}/install.sh:/root/install.sh:ro" \
    -e XUI_NONINTERACTIVE=1 \
    -e XUI_SSL_MODE=none \
    -e DEBIAN_FRONTEND=noninteractive \
    "$IMAGE" bash -euo pipefail -c '
        apt-get update -qq
        apt-get install -y -qq curl tar openssl ca-certificates > /dev/null

        echo "--- running install.sh piped (no TTY) ---"
        # Piping guarantees stdin is not a TTY, exercising the auto non-interactive path.
        cat /root/install.sh | bash

        echo "--- assertions ---"
        RESULT=/etc/x-ui/install-result.env
        test -f "$RESULT" || { echo "FAIL: $RESULT missing"; exit 1; }

        perms=$(stat -c %a "$RESULT")
        [ "$perms" = "600" ] || { echo "FAIL: $RESULT perms=$perms (want 600)"; exit 1; }

        # shellcheck disable=SC1090
        . "$RESULT"
        [ -n "${XUI_USERNAME:-}" ] && [ "$XUI_USERNAME" != "admin" ] \
            || { echo "FAIL: username missing or still admin"; exit 1; }
        [ -n "${XUI_PASSWORD:-}" ] && [ "$XUI_PASSWORD" != "admin" ] \
            || { echo "FAIL: password missing or still admin"; exit 1; }
        [ -n "${XUI_PANEL_PORT:-}" ] || { echo "FAIL: port missing"; exit 1; }

        # No default admin in the DB.
        /usr/local/x-ui/x-ui setting -show | grep -q "hasDefaultCredential: false" \
            || { echo "FAIL: hasDefaultCredential is not false"; exit 1; }

        echo "--- verifying the panel serves HTTP ---"
        cd /usr/local/x-ui
        ./x-ui > /tmp/xui.log 2>&1 &
        xpid=$!
        for _ in $(seq 1 15); do
            code=$(curl -s -o /dev/null -w "%{http_code}" \
                "http://127.0.0.1:${XUI_PANEL_PORT}/${XUI_WEB_BASE_PATH}/" 2>/dev/null || true)
            case "$code" in 200|301|302|307|308) break ;; esac
            sleep 1
        done
        kill "$xpid" 2>/dev/null || true
        echo "panel HTTP status: ${code:-none}"
        case "${code:-}" in
            200|301|302|307|308) : ;;
            *) echo "FAIL: panel did not serve (status ${code:-none})"; tail -n 30 /tmp/xui.log; exit 1 ;;
        esac

        echo "SMOKE_PASS: user=$XUI_USERNAME port=$XUI_PANEL_PORT path=$XUI_WEB_BASE_PATH"
    '

echo "== non-interactive smoke test PASSED =="
