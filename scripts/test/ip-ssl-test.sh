#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
source /dev/stdin < <(awk '/^# check root/ { exit } { print }' "$ROOT/x-ui.sh")
TEMP=$(mktemp -d)
trap 'rm -rf "$TEMP"' EXIT

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
same() { [[ "$1" == "$2" ]] || fail "$3: got [$1], want [$2]"; }
pass() { printf 'PASS: %s\n' "$1"; }

for invalid in 'not:ipv6' ':::' '1::2:' ':1::2' '1:2:3:4:5:6:7' '1:2:3:4:5:6:7:8:9' '2606::4700::1' '2606:4700::12345' '2606:4700::1/64' 'fe80::1%eth0' 'example.org' '2606:4700::1;touch /tmp/injected' '::ffff:192.000.2.1' ''; do
    if is_ipv6 "$invalid"; then fail "malformed IPv6 accepted: $invalid"; fi
done
for valid in '::' '::1' '2606:4700::1111' '2001:4860:4860:0:0:0:0:8888' '::ffff:192.0.2.1' '1:2:3:4:5:6:7::'; do
    is_ipv6 "$valid" || fail "valid IPv6 rejected: $valid"
done
pass 'IPv6 parser rejects malformed literals, CIDRs, zones and injection'

for unsuitable in '::1' '::' 'fe80::1' 'fd00::1' 'ff02::1' '2001:db8::1' '3fff::1' '2002:c000:201::1' '2001::1'; do
    if ip_ssl_public_ipv6 "$unsuitable"; then fail "unsuitable public candidate: $unsuitable"; fi
done
ip_ssl_public_ipv6 '2001:4860::1' || fail 'global IPv6 rejected'
pass 'automatic candidates exclude nonpublic and special-purpose ranges'

ip() {
    if [[ "$*" == '-6 route get '* ]]; then printf '%s\n' "${ROUTE:-}"; return; fi
    [[ "${IP_FAILURE:-0}" == 0 ]] || return 1
    printf '%s\n' "${ADDRESSES:-}"
}
curl() { printf 'unexpected external discovery request\n' >> "$TEMP/curl-calls"; return 7; }
row() { printf '2: eth0 inet6 %s/64 scope global %s valid_lft forever preferred_lft forever\n' "$1" "${2:-}"; }
ADDRESSES=$(row '2001:4860::1')
ROUTE='2606:4700::1111 via fe80::1 dev eth0 src 2001:4860::1 metric 100'
same "$(ip_ssl_detect_ipv6)" $'2001:4860::1\tdefault route' 'one default-route candidate'
ADDRESSES="$(row '2001:4860::1')"$'\n'"$(row '2606:4700::2')"$'\n'"$(row '2001:4860:0:0:0:0:0:1')"
ROUTE='2606:4700::1111 dev eth0 src 2606:4700::2 metric 100'
same "$(ip_ssl_detect_ipv6)" $'2606:4700::2\tdefault route\n2001:4860::1\tstable' 'route ordering and normalized deduplication'
ADDRESSES="$(row '2606:4700::1' temporary)"$'\n'"$(row '2001:4860::1')"$'\n'"$(row '2001:4860::2' deprecated)"$'\n'"$(row '2001:4860::3' tentative)"$'\n'"$(row '2001:4860::4' dadfailed)"$'\n'"$(row '2001:4860::5' optimistic)"
same "$(ip_ssl_detect_ipv6)" $'2001:4860::1\tstable' 'stable address wins over privacy and unusable addresses'
ADDRESSES=$(row '2606:4700::1' temporary)
same "$(ip_ssl_detect_ipv6)" $'2606:4700::1\ttemporary' 'privacy-only host is labelled'
ADDRESSES="$(row '2606:4700::1' temporary)"$'\n'"$(row '2606:4700:0:0:0:0:0:1')"
same "$(ip_ssl_detect_ipv6)" $'2606:4700:0:0:0:0:0:1\tstable' 'stable duplicate beats a temporary observation'
for ADDRESSES in '' "$(row 'fe80::1')" "$(row 'fd00::1')" 'broken output inet6 not:ipv6/64 scope global'; do
    same "$(ip_ssl_detect_ipv6)" '' 'no eligible candidates'
done
IP_FAILURE=1
same "$(ip_ssl_detect_ipv6)" '' 'ip command failure'
IP_FAILURE=0
same "$(unset -f ip; PATH=/nonexistent ip_ssl_detect_ipv6)" '' 'missing ip command'
[[ ! -f "$TEMP/curl-calls" ]] || fail 'local discovery depended on an external echo service'
pass 'discovery handles route preference, deduplication, privacy, missing tools and invalid output'

ADDRESSES=$(row '2001:4860::1')
ROUTE=''
same "$(ip_ssl_select_ipv6 1 '' <<< 'y')" '2001:4860::1' 'accept detected address'
same "$(ip_ssl_select_ipv6 1 '' <<< $'n\n')" '' 'reject and skip'
same "$(ip_ssl_select_ipv6 1 '' <<< $'n\n2606:4700::2')" '2606:4700::2' 'reject and enter manual address'
ADDRESSES="$(row '2001:4860::1')"$'\n'"$(row '2606:4700::2')"
same "$(ip_ssl_select_ipv6 1 '' <<< $'2\ny')" '2606:4700::2' 'select just one of multiple addresses'
ADDRESSES=''
same "$(ip_ssl_select_ipv6 1 '' <<< $'bad:ip\n2606:4700::2' 2>"$TEMP/manual-error")" '2606:4700::2' 'invalid manual input is retried'
grep -q 'Invalid public IPv6' "$TEMP/manual-error" || fail 'invalid manual input was silently skipped'
same "$(ip_ssl_select_ipv6 0 '' </dev/null)" '' 'unattended mode skips detection without explicit configuration'
same "$(ip_ssl_select_ipv6 0 '2606:4700::2' </dev/null)" '2606:4700::2' 'unattended explicit override'
if ip_ssl_select_ipv6 0 'bad:ipv6' </dev/null 2>"$TEMP/env-error"; then fail 'invalid environment override accepted'; fi
grep -q 'Invalid public IPv6' "$TEMP/env-error" || fail 'invalid override has no clear error'
pass 'selection requires consent and retains manual, skip and unattended behavior'
