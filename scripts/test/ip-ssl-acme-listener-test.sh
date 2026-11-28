#!/usr/bin/env bash
set -eo pipefail
acme_source="$1"
port="$2"
marker="xui-real-acme-$$"
serverproc=''
export Le_HTTPPort="$port" Le_Listen_V4=1 Le_Listen_V6=1
trap '_stopserver "${serverproc:-}" >/dev/null 2>&1 || true; wait 2>/dev/null || true' EXIT
# The unchanged upstream dispatcher can call its listener without an ACME order.
set -- _startserver "$marker" ''
# shellcheck disable=SC1090
source "$acme_source"
for family in 4 6; do
    target=127.0.0.1
    [[ "$family" != 6 ]] || target='[::1]'
    body=$(curl --noproxy '*' "-$family" -g --silent --show-error --fail --connect-timeout 2 --max-time 4 "http://$target:$port/.well-known/acme-challenge/probe")
    [[ "$body" == "$marker" ]] || { printf 'Wrong ACME response over IPv%s\n' "$family" >&2; exit 1; }
done
ss -lnt | awk -v port=":$port" '$4 ~ port {print}'
printf 'PASS: unchanged upstream %s serves IPv4 and IPv6 with both listen flags\n' "$acme_source"
