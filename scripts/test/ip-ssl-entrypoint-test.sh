#!/usr/bin/env bash
set -euo pipefail
ROOT="${IP_SSL_ENTRY_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"
source_function() {
    source /dev/stdin < <(awk -v name="$2" '$0==name"() {" {body=1} body {print} body && /^}/ {exit}' "$1")
}
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
TEMP=$(mktemp -d)
trap 'rm -rf "$TEMP"' EXIT
export red='' green='' yellow='' blue='' plain='' release=ubuntu
source /dev/stdin < <(awk '/^# BEGIN GENERATED IP SSL HELPERS/ {block=1;next} /^# END GENERATED IP SSL HELPERS/ {block=0} block {print}' "$ROOT/install.sh")
source_function "$ROOT/install.sh" prompt_or_default
source_function "$ROOT/install.sh" prompt_and_setup_ssl
# Invoked by the real installer function loaded above, before the failure stubs.
# shellcheck disable=SC2317,SC2329
setup_ip_certificate() { captured="${2:-}"; called=1; }
# shellcheck disable=SC2317,SC2329
systemctl() { stopped=1; }
export NONINTERACTIVE=1 XUI_SSL_MODE=ip XUI_SSL_IPV6='not:ipv6'
called=0; stopped=0
if prompt_and_setup_ssl 2053 / 198.51.100.8 > "$TEMP/invalid" 2>&1; then fail 'invalid environment IPv6 accepted by installer'; fi
[[ "$called" == 0 && "$stopped" == 0 ]] || fail 'invalid IPv6 stopped panel or reached issuance'
grep -q 'Invalid public IPv6' "$TEMP/invalid" || fail 'invalid environment value has no clear diagnostic'
unset XUI_SSL_IPV6
called=0
prompt_and_setup_ssl 2053 / 198.51.100.8 </dev/null > "$TEMP/unattended" 2>&1
[[ "$called" == 1 && -z "$captured" ]] || fail 'unattended unset override did not retain IPv4-only issuance'
if grep -q 'Detecting public IPv6' "$TEMP/unattended"; then fail 'unattended installer detected IPv6 without opt-in'; fi
export XUI_SSL_IPV6='2001:4860::1'; called=0
prompt_and_setup_ssl 2053 / 198.51.100.8 </dev/null > /dev/null
[[ "$called" == 1 && "$captured" == '2001:4860::1' ]] || fail 'explicit IPv6 override was lost'
unset XUI_SSL_IPV6
ip() {
    [[ "$*" != '-6 route get '* ]] || return 0
    printf '2: eth0 inet6 2001:4860::1/64 scope global valid_lft forever preferred_lft forever\n'
}
NONINTERACTIVE=0
prompt_and_setup_ssl 2053 / 198.51.100.8 <<< $'\ny\ny' > "$TEMP/accepted" 2>&1
[[ "$captured" == '2001:4860::1' ]] || fail 'interactive installer did not include confirmed detected address'
prompt_and_setup_ssl 2053 / 198.51.100.8 <<< $'\ny\nn\n' > "$TEMP/rejected" 2>&1
[[ -z "$captured" ]] || fail 'interactive installer included a rejected address'
printf 'PASS: installer consent, explicit configuration, no unattended prompts, and validation before service stop\n'

setup_ip_certificate() { return 1; }
systemctl() {
    printf '%s\n' "$*" >> "$TEMP/service-calls"
    [[ "$1" != is-active ]] || [[ "$was_running" == 1 ]]
}
rc-service() {
    printf '%s\n' "$*" >> "$TEMP/service-calls"
    [[ "$2" != status ]] || [[ "$was_running" == 1 ]]
}
NONINTERACTIVE=1
export XUI_SSL_IPV6='2001:4860::1'
for release in ubuntu alpine; do
    for was_running in 0 1; do
        : > "$TEMP/service-calls"
        if prompt_and_setup_ssl 2053 / 198.51.100.8 > /dev/null 2>&1; then fail 'failed IP setup reported success'; fi
        if [[ "$was_running" == 1 ]]; then
            grep -Eq '^(start x-ui|x-ui start)$' "$TEMP/service-calls" || fail "failed IP setup left the previous $release panel stopped"
        elif grep -Eq '^(start x-ui|x-ui start)$' "$TEMP/service-calls"; then
            fail "failed IP setup started an inactive $release panel"
        fi
    done
done
printf 'PASS: failed issuance restores only a previously running panel on systemd/OpenRC\n'
