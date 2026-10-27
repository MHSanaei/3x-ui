#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
source "$ROOT/scripts/lib/ip-ssl.sh"
TEMP=$(mktemp -d)
trap 'rm -rf "$TEMP"' EXIT
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
acme_version() { printf 'https://github.com/acmesh-official/acme.sh\nv%s\n' "${VERSION:-3.1.5}"; }
ip_ssl_prepare_listener acme_version 18080 ''
[[ "${IP_SSL_LISTEN_ARGS[*]}" == '--listen-v4' ]] || fail 'IPv4-only flags changed'
VERSION=3.1.4
if ip_ssl_prepare_listener acme_version 18080 '2001:4860::1' 2>"$TEMP/old-version"; then fail 'old acme.sh accepted for dual-stack'; fi
grep -q '3.1.5' "$TEMP/old-version" || fail 'old-version error does not explain the minimum version'
printf 'PASS: IPv4-only and incompatible acme.sh behavior\n'

[[ "${IP_SSL_LINUX_TEST:-0}" == 1 ]] || exit 0
command -v socat > /dev/null || fail 'Linux listener test requires socat'
VERSION=3.1.5
port=$((20000+RANDOM))
ip_ssl_prepare_listener acme_version "$port" '2001:4860::1'
[[ "${IP_SSL_LISTEN_ARGS[*]}" == '--listen-v4 --listen-v6' ]] || fail 'dual-stack flags missing'
if curl --noproxy '*' --silent --fail --connect-timeout 1 --max-time 2 "http://127.0.0.1:$port" > /dev/null; then fail 'probe left its IPv4 listener running'; fi
if curl --noproxy '*' --silent --fail -g --connect-timeout 1 --max-time 2 "http://[::1]:$port" > /dev/null; then fail 'probe left its IPv6 listener running'; fi
printf 'PASS: both real loopback families answered, and probe listeners were cleaned up\n'

socat() {
    if [[ "$*" == *TCP6-LISTEN* ]]; then return 1; fi
    command socat "$@"
}
if ip_ssl_prepare_listener acme_version "$port" '2001:4860::1' 2>"$TEMP/no-ipv6"; then fail 'unavailable IPv6 listener accepted'; fi
grep -q 'IPv4 and IPv6' "$TEMP/no-ipv6" || fail 'dual-stack failure has no clear diagnostic'
printf 'PASS: unavailable IPv6 does not silently fall back to IPv4\n'
