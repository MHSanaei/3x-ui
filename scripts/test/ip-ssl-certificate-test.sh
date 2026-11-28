#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
# shellcheck disable=SC1090
source "${IP_SSL_TEST_HELPERS:-$ROOT/scripts/lib/ip-ssl.sh}"
TEMP=$(mktemp -d)
trap 'rm -rf "$TEMP"' EXIT
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
export ACME_LOG="$TEMP/arguments" RELOAD_LOG="$TEMP/reloads"
export ISSUED_CERT="$TEMP/new.pem" ISSUED_KEY="$TEMP/new.key"
export ACME_ISSUE_FAIL=0 ACME_INSTALL_FAIL=0 ACME_RELOAD_FAIL=0
for name in old new v4 dns; do
    san='IP:198.51.100.8,IP:2001:4860::1'
    [[ "$name" != v4 ]] || san='IP:198.51.100.8'
    [[ "$name" != dns ]] || san='DNS:198.51.100.8,DNS:2001:4860::1'
    openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 2 -subj /CN=ip-test -addext "subjectAltName=$san" -keyout "$TEMP/$name.key" -out "$TEMP/$name.pem" > /dev/null 2>&1
done
mkdir "$TEMP/cert's directory"
cert_dir="$TEMP/cert's directory"
cp "$TEMP/old.pem" "$cert_dir/fullchain.pem"
cp "$TEMP/old.key" "$cert_dir/privkey.pem"
cat > "$TEMP/mock acme" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf 'CALL\n' >> "$ACME_LOG"
printf 'arg=%s\n' "$@" >> "$ACME_LOG"
case "$1" in
    --version) printf 'v3.1.5\n' ;;
    --issue) [[ "$ACME_ISSUE_FAIL" != 1 ]] ;;
    --upgrade|--register-account) : ;;
    --installcert)
        key=''; cert=''; reload=''
        while (($#)); do
            case "$1" in --key-file) key="$2"; shift;; --fullchain-file) cert="$2"; shift;; --reloadcmd) reload="$2"; shift;; esac
            shift
        done
        if [[ "$ACME_INSTALL_FAIL" == 1 ]]; then printf 'broken certificate\n' > "$cert"; exit 1; fi
        cp "$ISSUED_KEY" "$key"; cp "$ISSUED_CERT" "$cert"
        printf '%s' "$reload" > "$RELOAD_LOG.command"
        sh -c "$reload"
        [[ "$ACME_RELOAD_FAIL" != 1 ]]
        ;;
    *) printf 'Unexpected ACME operation: %s\n' "$1" >&2; exit 1 ;;
esac
MOCK
chmod +x "$TEMP/mock acme"
acme="$TEMP/mock acme"
reload="printf 'reloaded\\n' >> '$RELOAD_LOG'"
port=$((20000+RANDOM))

ACME_ISSUE_FAIL=1
if ip_ssl_issue_and_install "$acme" 198.51.100.8 '' "$port" "$cert_dir" "$reload"; then fail 'ACME failure reported success'; fi
cmp "$TEMP/old.pem" "$cert_dir/fullchain.pem" || fail 'issuance failure removed existing certificate'
cmp "$TEMP/old.key" "$cert_dir/privkey.pem" || fail 'issuance failure removed existing key'
[[ ! -f "$RELOAD_LOG" ]] || fail 'issuance failure reloaded panel'
printf 'PASS: ACME failure preserves existing certificate and key\n'
ACME_ISSUE_FAIL=0

ip_ssl_issue_and_install "$acme" 198.51.100.8 '2001:4860::1' "$port" "$cert_dir" "$reload"
cmp "$TEMP/new.pem" "$cert_dir/fullchain.pem" || fail 'new certificate not installed'
[[ "$(stat -c %a "$cert_dir/privkey.pem")" == 600 ]] || fail 'private key permissions changed'
[[ "$(stat -c %a "$cert_dir/fullchain.pem")" == 644 ]] || fail 'certificate permissions changed'
openssl verify -trusted "$cert_dir/fullchain.pem" -partial_chain -verify_ip 198.51.100.8 "$cert_dir/fullchain.pem" > /dev/null
openssl verify -trusted "$cert_dir/fullchain.pem" -partial_chain -verify_ip 2001:4860::1 "$cert_dir/fullchain.pem" > /dev/null
if ! grep -qx 'arg=--listen-v4' "$ACME_LOG" || ! grep -qx 'arg=--listen-v6' "$ACME_LOG"; then fail 'dual-stack flags absent'; fi
[[ "$(grep -cx 'arg=2001:4860::1' "$ACME_LOG")" == 1 ]] || fail 'IPv6 SAN duplicated or missing'
[[ "$(cat "$RELOAD_LOG")" == reloaded ]] || fail 'valid certificate did not reload'
sh -c "$(cat "$RELOAD_LOG.command")"
[[ "$(wc -l < "$RELOAD_LOG")" == 2 ]] || fail 'saved renewal reload command failed'
printf 'PASS: both IP SANs, quoting, install permissions and saved reload command\n'

cp "$TEMP/v4.pem" "$cert_dir/fullchain.pem"
cp "$TEMP/v4.key" "$cert_dir/privkey.pem"
if sh -c "$(cat "$RELOAD_LOG.command")" > /dev/null 2>&1; then fail 'renewal guard accepted a missing IPv6 SAN'; fi
[[ "$(wc -l < "$RELOAD_LOG")" == 2 ]] || fail 'invalid renewed certificate reloaded the panel'
cp "$TEMP/new.pem" "$cert_dir/fullchain.pem"
cp "$TEMP/new.key" "$cert_dir/privkey.pem"
printf 'PASS: saved renewal guard blocks reload after a missing IPv6 SAN\n'

for bad in v4 dns; do
    ISSUED_CERT="$TEMP/$bad.pem"; ISSUED_KEY="$TEMP/$bad.key"
    if ip_ssl_issue_and_install "$acme" 198.51.100.8 '2001:4860::1' "$port" "$cert_dir" "$reload"; then fail 'missing IP SAN accepted'; fi
    cmp "$TEMP/new.pem" "$cert_dir/fullchain.pem" || fail 'bad SAN replaced working certificate'
    cmp "$TEMP/new.key" "$cert_dir/privkey.pem" || fail 'bad SAN replaced working key'
    [[ "$(wc -l < "$RELOAD_LOG")" == 2 ]] || fail 'invalid IP SAN reloaded panel'
done
ISSUED_CERT="$TEMP/new.pem"; ISSUED_KEY="$TEMP/old.key"
if ip_ssl_issue_and_install "$acme" 198.51.100.8 '' "$port" "$cert_dir" "$reload"; then fail 'mismatched private key accepted'; fi
cmp "$TEMP/new.key" "$cert_dir/privkey.pem" || fail 'mismatched key was not rolled back'
ACME_INSTALL_FAIL=1
if ip_ssl_issue_and_install "$acme" 198.51.100.8 '' "$port" "$cert_dir" "$reload"; then fail 'partial installation accepted'; fi
cmp "$TEMP/new.pem" "$cert_dir/fullchain.pem" || fail 'partial install was not rolled back'
ACME_INSTALL_FAIL=0
ISSUED_CERT="$TEMP/old.pem"; ISSUED_KEY="$TEMP/old.key"; ACME_RELOAD_FAIL=1
ip_ssl_issue_and_install "$acme" 198.51.100.8 '' "$port" "$cert_dir" "$reload"
cmp "$TEMP/old.pem" "$cert_dir/fullchain.pem" || fail 'reload error discarded valid newly installed cert'
printf 'PASS: SAN/key mismatch and partial-install rollback; valid copy survives reload errors\n'

before=$(wc -l < "$ACME_LOG")
if ip_ssl_issue_and_install "$acme" 198.51.100.8 "2001:4860::1;touch $TEMP/injected" "$port" "$cert_dir" "$reload"; then fail 'injection accepted'; fi
[[ ! -e "$TEMP/injected" && "$(wc -l < "$ACME_LOG")" == "$before" ]] || fail 'invalid IP reached shell or ACME'
printf 'PASS: invalid SAN never reaches ACME or a shell command\n'

source_function() {
    source /dev/stdin < <(awk -v name="$2" '$0==name"() {" {body=1} body {print} body && /^}/ {exit}' "$1")
}
export HOME="$TEMP/home"
mkdir -p "$HOME/.acme.sh" "$TEMP/panel" "$TEMP/mockbin"
cp "$acme" "$HOME/.acme.sh/acme.sh"
cat > "$TEMP/panel/x-ui" <<'PANEL'
#!/usr/bin/env bash
if [[ "$1" == setting ]]; then printf 'webBasePath: /test/\nport: 2053\n'; else printf 'configured\n' >> "$RELOAD_LOG.panel"; fi
PANEL
cat > "$TEMP/mockbin/systemctl" <<'SERVICE'
#!/usr/bin/env bash
printf 'reloaded\n' >> "$RELOAD_LOG"
SERVICE
chmod +x "$TEMP/panel/x-ui" "$TEMP/mockbin/systemctl"
export PATH="$TEMP/mockbin:$PATH"
export xui_folder="$TEMP/panel" release=ubuntu
export red='' green='' yellow='' blue='' plain=''
LOGI() { printf '%s\n' "$*"; }
LOGE() { printf '%s\n' "$*" >&2; }
apt-get() { :; }
restart() { printf 'restarted\n' >> "$RELOAD_LOG.panel"; }
curl() {
    if [[ "$*" == *https://* ]]; then printf '198.51.100.8\n200'; else command curl "$@"; fi
}
ip() {
    [[ "$*" != '-6 route get '* ]] || return 0
    printf '2: eth0 inet6 2001:4860::1/64 scope global valid_lft forever preferred_lft forever\n'
}
ACME_RELOAD_FAIL=0; ISSUED_CERT="$TEMP/new.pem"; ISSUED_KEY="$TEMP/new.key"
source_function "$ROOT/x-ui.sh" confirm
source_function "$ROOT/x-ui.sh" ssl_cert_issue_for_ip
if ! ssl_cert_issue_for_ip "$TEMP/menu-cert" <<< "$(printf 'y\ny\n%s\ny\n' "$port")" > "$TEMP/menu-output" 2>&1; then
    cat "$TEMP/menu-output"; fail 'management menu issuance failed'
fi
grep -Fq 'https://[2001:4860::1]:2053/test/' "$TEMP/menu-output" || fail 'IPv6 access URL is not bracketed'
if ! grep -q configured "$RELOAD_LOG.panel" || ! grep -q restarted "$RELOAD_LOG.panel"; then fail 'menu did not configure and restart panel after consent'; fi
source_function "$ROOT/install.sh" prompt_or_default
source_function "$ROOT/install.sh" setup_ip_certificate
export NONINTERACTIVE=1 XUI_ACME_HTTP_PORT="$port" XUI_ACME_EMAIL=ssl-test@example.invalid
if ! setup_ip_certificate 198.51.100.8 '2001:4860::1' "$TEMP/installer-cert" </dev/null > "$TEMP/install-output" 2>&1; then
    cat "$TEMP/install-output"; fail 'installer issuance failed'
fi
openssl verify -trusted "$TEMP/installer-cert/fullchain.pem" -partial_chain -verify_ip 2001:4860::1 "$TEMP/installer-cert/fullchain.pem" > /dev/null
grep -qx 'arg=--upgrade' "$ACME_LOG" || fail 'auto-upgrade registration lost'
grep -qx 'arg=--register-account' "$ACME_LOG" || fail 'configured account email ignored'
printf 'PASS: real menu and installer functions install both IP SANs, configure panel paths and retain renewal setup\n'
