#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "Failed to check the system OS, please contact the author!" >&2
    exit 1
fi
echo "The OS release is: $release"

arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${green}Unsupported CPU architecture! ${plain}" && rm -f "$(realpath "$0")" && exit 1 ;;
    esac
}

echo "Arch: $(arch)"

# Non-interactive mode: triggered explicitly via XUI_NONINTERACTIVE=1, or
# implicitly when stdin is not a TTY (e.g. `curl ... | bash`, cloud-init).
# In this mode every prompt below is replaced by an env var or a sane default.
if [[ "${XUI_NONINTERACTIVE:-0}" == "1" ]] || [[ ! -t 0 ]]; then
    NONINTERACTIVE=1
else
    NONINTERACTIVE=0
fi
export NONINTERACTIVE

# Simple helpers
is_ipv4() {
    [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] && return 0 || return 1
}
# BEGIN GENERATED IP SSL HELPERS
# shellcheck shell=bash
# Shared IP-certificate helpers, embedded into both standalone entry points.
# Keep this file free of top-level work and optional runtime dependencies.

ip_ssl_ipv4() {
    local value="$1" octet
    local -a octets
    [[ "$value" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
    IFS=. read -r -a octets <<< "$value"
    for octet in "${octets[@]}"; do
        [[ "$octet" =~ ^(0|[1-9][0-9]{0,2})$ ]] && ((10#$octet <= 255)) || return 1
    done
}

ip_ssl_expand_ipv6() {
    local value="$1" prefix tail group count missing i
    local -a octets left=() right=() groups=()
    if [[ "$value" == *.* ]]; then
        tail="${value##*:}"
        ip_ssl_ipv4 "$tail" || return 1
        [[ "$value" == *:* ]] || return 1
        IFS=. read -r -a octets <<< "$tail"
        printf -v tail '%x:%x' "$((10#${octets[0]} * 256 + 10#${octets[1]}))" "$((10#${octets[2]} * 256 + 10#${octets[3]}))"
        value="${value%:*}:$tail"
    fi
    [[ "$value" =~ ^[[:xdigit:]:]+$ && "$value" != *:::* ]] || return 1
    [[ "$value" != :* || "$value" == ::* ]] || return 1
    [[ "$value" != *: || "$value" == *:: ]] || return 1
    if [[ "$value" == *::* ]]; then
        prefix="${value%%::*}"
        tail="${value#*::}"
        [[ "$tail" != *::* ]] || return 1
        [[ -z "$prefix" ]] || IFS=: read -r -a left <<< "$prefix"
        [[ -z "$tail" ]] || IFS=: read -r -a right <<< "$tail"
        count=$((${#left[@]} + ${#right[@]}))
        ((count < 8)) || return 1
        missing=$((8 - count))
        groups=("${left[@]}")
        for ((i = 0; i < missing; i++)); do groups+=(0); done
        groups+=("${right[@]}")
    else
        [[ "$value" != :* && "$value" != *: ]] || return 1
        IFS=: read -r -a groups <<< "$value"
        ((${#groups[@]} == 8)) || return 1
    fi
    for group in "${groups[@]}"; do
        [[ "$group" =~ ^[[:xdigit:]]{1,4}$ ]] || return 1
    done
    for ((i = 0; i < 8; i++)); do
        ((i == 0)) || printf ':'
        printf '%04x' "$((16#${groups[i]}))"
    done
    printf '\n'
}

is_ipv6() { ip_ssl_expand_ipv6 "$1" > /dev/null; }

ip_ssl_public_ipv6() {
    local expanded first second
    expanded=$(ip_ssl_expand_ipv6 "$1") || return 1
    first="${expanded%%:*}"
    second="${expanded#*:}"; second="${second%%:*}"
    ((16#$first >= 0x2000 && 16#$first <= 0x3fff)) || return 1
    # Exclude protocol assignments, documentation and deprecated 6to4 space.
    [[ "$first" != 2002 ]] || return 1
    [[ "$first:$second" != 2001:0db8 ]] || return 1
    [[ "$first" != 2001 ]] || ((16#$second >= 0x0200)) || return 1
    [[ "$first" != 3fff ]] || ((16#$second >= 0x1000)) || return 1
}

ip_ssl_detect_ipv6() {
    command -v ip > /dev/null 2>&1 || return 0
    local source preferred='' line cidr address expanded label rank i
    local -a fields stable=() privacy=() chosen=()
    local -A seen=()
    # This consults the kernel route table; it sends no packet to the target.
    source=$(ip -6 route get 2606:4700:4700::1111 2> /dev/null | awk '{for(i=1;i<NF;i++)if($i=="src"){print $(i+1);exit}}') || source=''
    if [[ -n "$source" ]]; then preferred=$(ip_ssl_expand_ipv6 "$source") || preferred=''; fi
    while IFS= read -r line; do
        [[ " $line " == *' scope global '* ]] || continue
        case " $line " in
            *' tentative '*|*' optimistic '*|*' deprecated '*|*' dadfailed '*|*' detached '*|*' preferred_lft 0sec '*) continue ;;
        esac
        IFS=' ' read -r -a fields <<< "$line"
        cidr=''
        for ((i=0; i+1<${#fields[@]}; i++)); do
            if [[ "${fields[i]}" == inet6 ]]; then cidr="${fields[i+1]}"; break; fi
        done
        [[ "$cidr" =~ ^([^/]+)/([0-9]{1,3})$ ]] || continue
        ((10#${BASH_REMATCH[2]} <= 128)) || continue
        address="${BASH_REMATCH[1]}"
        ip_ssl_public_ipv6 "$address" || continue
        expanded=$(ip_ssl_expand_ipv6 "$address") || continue
        if [[ -n "${seen[$expanded]:-}" ]]; then
            [[ "${seen[$expanded]}" == temporary && " $line " != *' temporary '* ]] || continue
        fi
        label=stable; rank=1
        if [[ "$expanded" == "$preferred" ]]; then label='default route'; rank=0; fi
        if [[ " $line " == *' temporary '* ]]; then
            seen[$expanded]=temporary
            privacy+=("$rank"$'\t'"$address"$'\t'"temporary")
        else
            seen[$expanded]=stable
            stable+=("$rank"$'\t'"$address"$'\t'"$label")
        fi
    done < <(ip -o -6 addr show up scope global 2> /dev/null || true)
    if ((${#stable[@]})); then chosen=("${stable[@]}"); else chosen=("${privacy[@]}"); fi
    ((${#chosen[@]})) || return 0
    printf '%s\n' "${chosen[@]}" | LC_ALL=C sort -t $'\t' -k1,1n -k2,2 | cut -f2-
}

ip_ssl_select_ipv6() {
    local interactive="$1" configured="${2:-}" row address answer selection i
    local -a candidates=() labels=()
    if [[ -n "$configured" ]]; then
        if ! ip_ssl_public_ipv6 "$configured"; then
            printf 'Invalid public IPv6 in XUI_SSL_IPV6: %s\n' "$configured" >&2
            return 1
        fi
        printf '%s\n' "$configured"
        return 0
    fi
    [[ "$interactive" == 1 ]] || return 0
    printf 'Detecting public IPv6 addresses...\n' >&2
    while IFS=$'\t' read -r address row; do
        [[ -n "$address" ]] || continue
        candidates+=("$address"); labels+=("$row")
    done < <(ip_ssl_detect_ipv6)
    address=''
    if ((${#candidates[@]} == 1)); then
        address="${candidates[0]}"
        printf '\nDetected IPv6 (%s):\n%s\n' "${labels[0]}" "$address" >&2
    elif ((${#candidates[@]} > 1)); then
        printf '\nDetected IPv6 candidates:\n' >&2
        for ((i=0;i<${#candidates[@]};i++)); do printf '%d) %s (%s)\n' "$((i+1))" "${candidates[i]}" "${labels[i]}" >&2; done
        while true; do
            printf 'Choose a number, m for manual entry, or Enter to skip: ' >&2
            IFS= read -r selection || selection=''
            [[ -n "$selection" ]] || return 0
            [[ "$selection" != m && "$selection" != M ]] || break
            if [[ "$selection" =~ ^[0-9]{1,5}$ ]] && ((10#$selection >= 1 && 10#$selection <= ${#candidates[@]})); then
                address="${candidates[10#$selection-1]}"; break
            fi
            printf 'Invalid selection.\n' >&2
        done
    else
        printf 'No suitable public IPv6 address was detected.\n' >&2
    fi
    if [[ -n "$address" ]]; then
        if [[ "${labels[*]}" == *temporary* ]]; then
            printf 'Temporary IPv6 addresses can change; renewal requires this address to remain reachable.\n' >&2
        fi
        while true; do
            printf 'Include %s in the SSL certificate? [y/N]: ' "$address" >&2
            IFS= read -r answer || answer=''
            case "$answer" in
                y|Y) printf '%s\n' "$address"; return 0 ;;
                n|N|'') break ;;
                *) printf 'Please answer y or n.\n' >&2 ;;
            esac
        done
    fi
    while true; do
        printf 'Enter a public IPv6 address manually or press Enter to skip: ' >&2
        IFS= read -r address || address=''
        address="${address#"${address%%[![:space:]]*}"}"
        address="${address%"${address##*[![:space:]]}"}"
        [[ -n "$address" ]] || return 0
        if ip_ssl_public_ipv6 "$address"; then printf '%s\n' "$address"; return 0; fi
        printf 'Invalid public IPv6 address. Enter one address, without a subnet or zone ID.\n' >&2
    done
}

ip_ssl_dual_stack_probe() (
    local port="$1" probe_dir pid4='' pid6='' attempt body4 body6 serve
    local marker="xui-acme-$$-$RANDOM"
    [[ "$port" =~ ^[0-9]{1,5}$ ]] && ((10#$port >= 1 && 10#$port <= 65535)) || return 1
    command -v socat > /dev/null 2>&1 && command -v curl > /dev/null 2>&1 || return 1
    probe_dir=$(mktemp -d) || return 1
    trap '[[ -z "$pid4" ]] || { kill "$pid4" 2>/dev/null || true; wait "$pid4" 2>/dev/null || true; }; [[ -z "$pid6" ]] || { kill "$pid6" 2>/dev/null || true; wait "$pid6" 2>/dev/null || true; }; rm -rf -- "$probe_dir"' EXIT
    serve="sleep 0.1; echo 'HTTP/1.0 200 OK'; echo 'Content-Length\: ${#marker}'; echo ''; printf '%s' '$marker'"
    socat "TCP4-LISTEN:$port,crlf,reuseaddr,fork" "SYSTEM:$serve" > /dev/null 2>"$probe_dir/v4.log" &
    pid4=$!
    socat "TCP6-LISTEN:$port,crlf,reuseaddr,fork,ipv6only=1" "SYSTEM:$serve" > /dev/null 2>"$probe_dir/v6.log" &
    pid6=$!
    for ((attempt=0;attempt<10;attempt++)); do
        body4=$(curl --noproxy '*' -4 --silent --fail --connect-timeout 1 --max-time 2 "http://127.0.0.1:$port/" 2> /dev/null) || body4=''
        body6=$(curl --noproxy '*' -6 --silent --fail -g --connect-timeout 1 --max-time 2 "http://[::1]:$port/" 2> /dev/null) || body6=''
        [[ "$body4" != "$marker" || "$body6" != "$marker" ]] || return 0
        if ! kill -0 "$pid4" 2> /dev/null || ! kill -0 "$pid6" 2> /dev/null; then break; fi
        sleep 0.1
    done
    return 1
)

ip_ssl_prepare_listener() {
    local acme="$1" port="$2" ipv6="${3:-}" line version='' major minor patch
    IP_SSL_LISTEN_ARGS=(--listen-v4)
    [[ -n "$ipv6" ]] || return 0
    while IFS= read -r line; do
        if [[ "$line" =~ ^v?([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})($|[^0-9]) ]]; then
            major="${BASH_REMATCH[1]}"; minor="${BASH_REMATCH[2]}"; patch="${BASH_REMATCH[3]}"
            if ((10#$major > 3 || (10#$major == 3 && (10#$minor > 1 || (10#$minor == 1 && 10#$patch >= 5))))); then version=1; fi
            break
        fi
    done < <("$acme" --version 2> /dev/null)
    if [[ -z "$version" ]]; then
        printf 'Dual-stack IP certificates require acme.sh 3.1.5 or newer. Upgrade acme.sh and retry; the confirmed IPv6 was not omitted.\n' >&2
        return 1
    fi
    if ! ip_ssl_dual_stack_probe "$port"; then
        printf 'Cannot serve HTTP-01 over both IPv4 and IPv6 on port %s. Check socat, IPv6 support and port conflicts. No certificate request was sent.\n' "$port" >&2
        return 1
    fi
    # Consumed as an array by the standalone installer and menu entry points.
    # shellcheck disable=SC2034
    IP_SSL_LISTEN_ARGS=(--listen-v4 --listen-v6)
}

ip_ssl_shell_quote() {
    local value="$1"
    value="${value//\'/\'\\\'\'}"
    printf "'%s'" "$value"
}

ip_ssl_issue_and_install() (
    local acme="$1" ipv4="$2" ipv6="$3" port="$4" cert_dir="$5" reload="$6"
    local stage='' attempted=0 installed=0 code=0 file restored
    local quoted_cert quoted_key validation check_key reload_guard
    local -a domain_args=(-d "$ipv4")
    if ! ip_ssl_ipv4 "$ipv4" || { [[ -n "$ipv6" ]] && ! ip_ssl_public_ipv6 "$ipv6"; }; then
        printf 'Invalid IP certificate identifier; no ACME request was sent.\n' >&2
        return 1
    fi
    [[ "$port" =~ ^[0-9]{1,5}$ ]] && ((10#$port >= 1 && 10#$port <= 65535)) || return 1
    [[ -z "$ipv6" ]] || domain_args+=(-d "$ipv6")
    ip_ssl_prepare_listener "$acme" "$port" "$ipv6" || return 1
    if ! "$acme" --issue "${domain_args[@]}" --standalone "${IP_SSL_LISTEN_ARGS[@]}" \
        --server letsencrypt --certificate-profile shortlived --days 6 --httpport "$port" --force; then
        printf 'IP certificate issuance failed. Existing certificates and ACME data were preserved. Check external port 80 on every selected address.\n' >&2
        return 1
    fi

    umask 077
    mkdir -p "$cert_dir" || return 1
    stage=$(mktemp -d "$cert_dir/.xui-install.XXXXXX") || return 1
    # shellcheck disable=SC2329
    ip_ssl_install_cleanup() {
        restored=1
        if ((attempted && !installed)); then
            for file in fullchain.pem privkey.pem; do
                if [[ -f "$stage/$file" ]]; then
                    cp -p -- "$stage/$file" "$cert_dir/$file" || restored=0
                else
                    rm -f -- "$cert_dir/$file" || restored=0
                fi
            done
        fi
        if ((restored)); then
            rm -rf -- "$stage"
        else
            printf 'Could not restore certificate files; recovery copies remain in %s\n' "$stage" >&2
        fi
    }
    trap ip_ssl_install_cleanup EXIT
    for file in fullchain.pem privkey.pem; do
        [[ ! -e "$cert_dir/$file" ]] || cp -p -- "$cert_dir/$file" "$stage/$file" || return 1
    done
    quoted_cert=$(ip_ssl_shell_quote "$cert_dir/fullchain.pem")
    quoted_key=$(ip_ssl_shell_quote "$cert_dir/privkey.pem")
    # Trust the leaf only for local identity checks, not as a new system CA.
    validation="openssl verify -trusted $quoted_cert -partial_chain -verify_ip '$ipv4' $quoted_cert"
    [[ -z "$ipv6" ]] || validation+=" && openssl verify -trusted $quoted_cert -partial_chain -verify_ip '$ipv6' $quoted_cert"
    # Expansion belongs to the later reload shell, including cron renewals.
    # shellcheck disable=SC2016
    printf -v check_key 'xui_cert_pub=$(openssl x509 -in %s -pubkey -noout) && xui_key_pub=$(openssl pkey -in %s -passin pass: -pubout) && [ -n "$xui_cert_pub" ] && [ "$xui_cert_pub" = "$xui_key_pub" ]' "$quoted_cert" "$quoted_key"
    validation+=" && $check_key"
    # acme.sh persists this self-contained guard for later cron renewals too.
    reload_guard="$validation && { $reload; }"
    attempted=1
    "$acme" --installcert --force -d "$ipv4" --key-file "$cert_dir/privkey.pem" \
        --fullchain-file "$cert_dir/fullchain.pem" --reloadcmd "$reload_guard" || code=$?
    if ! sh -c "$validation" > /dev/null 2>&1; then
        printf 'Installed certificate failed IP SAN, validity or private-key checks; restoring previous files.\n' >&2
        return 1
    fi
    if ((code != 0)) && [[ -f "$stage/fullchain.pem" ]] && [[ "$(cat "$stage/fullchain.pem")" == "$(cat "$cert_dir/fullchain.pem")" ]]; then
        printf 'Certificate installation failed without replacing the previous certificate.\n' >&2
        return 1
    fi
    chmod 600 "$cert_dir/privkey.pem" && chmod 644 "$cert_dir/fullchain.pem" || return 1
    installed=1
    if ((code != 0)); then printf 'Certificate files are valid, but the reload command reported an error.\n' >&2; fi
)
# END GENERATED IP SSL HELPERS
is_ip() {
    is_ipv4 "$1" || is_ipv6 "$1"
}
is_domain() {
    [[ "$1" =~ ^([A-Za-z0-9](-*[A-Za-z0-9])*\.)+(xn--[a-z0-9]{2,}|[A-Za-z]{2,})$ ]] && return 0 || return 1
}

# acme.sh's standalone server binds IPv4 by default; --listen-v6 makes it
# v6-only, which breaks HTTP-01 validation when the domain's A record points
# at this host's IPv4 (#4994). Only force IPv6 when the host has no global
# IPv4 address at all.
acme_listen_flag() {
    if ip -4 addr show scope global 2> /dev/null | grep -q "inet "; then
        echo ""
    else
        echo "--listen-v6"
    fi
}

# Port helpers
is_port_in_use() {
    local port="$1"
    if command -v ss > /dev/null 2>&1; then
        ss -ltn 2> /dev/null | awk -v p=":${port}$" '$4 ~ p {exit 0} END {exit 1}'
        return
    fi
    if command -v netstat > /dev/null 2>&1; then
        netstat -lnt 2> /dev/null | awk -v p=":${port} " '$4 ~ p {exit 0} END {exit 1}'
        return
    fi
    if command -v lsof > /dev/null 2>&1; then
        lsof -nP -iTCP:${port} -sTCP:LISTEN > /dev/null 2>&1 && return 0
    fi
    return 1
}

install_base() {
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf makecache -y && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum makecache -y && yum install -y cronie curl tar tzdata socat ca-certificates openssl
            else
                dnf makecache -y && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl
            fi
            ;;
        arch | manjaro | parch)
            pacman -Sy --noconfirm cronie curl tar tzdata socat ca-certificates openssl
            ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper refresh && zypper -q install -y cron curl tar timezone socat ca-certificates openssl
            ;;
        alpine)
            apk update && apk add dcron curl tar tzdata socat ca-certificates openssl
            ;;
        *)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl
            ;;
    esac
}

gen_random_string() {
    local length="$1"
    openssl rand -base64 $((length * 2)) \
        | tr -dc 'a-zA-Z0-9' \
        | head -c "$length"
}

# prompt_or_default VARNAME "prompt text" "default" [ENV_NAME]
# Interactive: read into VARNAME. Non-interactive: VARNAME = ${ENV_NAME:-default}.
# ENV_NAME defaults to VARNAME when omitted. Keeps every interactive prompt
# string byte-for-byte identical to the original `read -rp`.
prompt_or_default() {
    local __var="$1" __prompt="$2" __default="$3" __env="${4:-$1}"
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        printf -v "$__var" '%s' "${!__env:-$__default}"
    else
        # shellcheck disable=SC2229
        read -rp "$__prompt" "$__var"
    fi
}

# write_install_result <user> <pass> <port> <webpath> <scheme> <host> <token> <dbtype>
# Persists a parseable, root-only credentials file consumed by cloud-init/MOTD.
# Values are written with printf '%q' so a pinned password/username containing
# spaces, quotes, $(...) or backticks is shell-escaped and the file stays safely
# source-able (consumers do '. install-result.env'). For the alphanumeric random
# values gen_random_string emits, %q is a no-op. This is a DIFFERENT file from the
# Postgres env file (/etc/default/x-ui).
write_install_result() {
    local u="$1" p="$2" port="$3" wbp="$4" scheme="$5" host="$6" token="$7" dbtype="$8"
    local result_file="/etc/x-ui/install-result.env"
    local url_host="${host:-SERVER_IP_UNKNOWN}"
    install -d -m 700 /etc/x-ui 2> /dev/null
    local prev_umask
    prev_umask=$(umask)
    umask 077
    if ! {
        printf 'XUI_USERNAME=%q\n' "$u"
        printf 'XUI_PASSWORD=%q\n' "$p"
        printf 'XUI_PANEL_PORT=%q\n' "$port"
        printf 'XUI_WEB_BASE_PATH=%q\n' "$wbp"
        printf 'XUI_ACCESS_URL=%q\n' "${scheme}://${url_host}:${port}/${wbp}"
        printf 'XUI_API_TOKEN=%q\n' "$token"
        printf 'XUI_DB_TYPE=%q\n' "$dbtype"
    } > "$result_file"; then
        umask "$prev_umask"
        echo -e "${yellow}Warning: failed to write ${result_file}.${plain}" >&2
        return 1
    fi
    umask "$prev_umask"
    chmod 600 "$result_file" 2> /dev/null
    chown root:root "$result_file" 2> /dev/null || true
    echo -e "${green}Install result written to ${result_file} (mode 600).${plain}"
}

# RHEL-family initdb writes pg_hba.conf host rules with ident auth, which
# compares the OS username against the Postgres role and always rejects the
# randomly generated panel role over TCP (#5806). Prepend password-auth rules
# scoped to the panel database; first match wins, and md5 also accepts
# scram-sha-256-stored verifiers, so this works on every supported distro.
pg_ensure_hba_password_auth() {
    local pg_db="$1"
    local hba_file
    hba_file=$(sudo -u postgres psql -tAc 'SHOW hba_file' 2> /dev/null | tr -d '[:space:]')
    [[ -n "${hba_file}" && -f "${hba_file}" ]] || return 0
    grep -Eq "^host[[:space:]]+${pg_db}[[:space:]]" "${hba_file}" && return 0
    local tmp
    tmp=$(mktemp) || return 1
    {
        echo "# Added by 3x-ui: allow password logins for the panel database."
        echo "host    ${pg_db}    all    127.0.0.1/32    md5"
        echo "host    ${pg_db}    all    ::1/128         md5"
        cat "${hba_file}"
    } > "${tmp}" || {
        rm -f "${tmp}"
        return 1
    }
    cat "${tmp}" > "${hba_file}" || {
        rm -f "${tmp}"
        return 1
    }
    rm -f "${tmp}"
    sudo -u postgres psql -tAc 'SELECT pg_reload_conf()' > /dev/null 2>&1 || true
}

# New installs get this PostgreSQL major from the official PGDG repository; distros or
# CPU architectures PGDG does not build for fall back to the distribution's own package.
PG_TARGET_MAJOR=18

pg_has_debian_cluster() {
    command -v pg_lsclusters > /dev/null 2>&1 && [[ -n "$(pg_lsclusters -h 2> /dev/null)" ]]
}

# Installs postgresql-${PG_TARGET_MAJOR} from apt.postgresql.org. On failure it removes the
# repository it added, so an unsupported release cannot break later `apt-get update` runs.
pg_apt_install_pgdg_server() {
    local list added_lists=()
    for list in /etc/apt/sources.list.d/pgdg.list /etc/apt/sources.list.d/pgdg.sources; do
        [[ -e "${list}" ]] || added_lists+=("${list}")
    done
    if apt-get install -y -q postgresql-common ca-certificates gnupg >&2 \
        && /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y >&2 \
        && apt-get install -y -q "postgresql-${PG_TARGET_MAJOR}" >&2; then
        return 0
    fi
    for list in "${added_lists[@]}"; do
        rm -f "${list}"
    done
    apt-get update >&2 || true
    echo -e "${yellow}PostgreSQL ${PG_TARGET_MAJOR} is not available for this system; installing the distribution's PostgreSQL instead.${plain}" >&2
    return 1
}

# Installs postgresql${PG_TARGET_MAJOR}-server from the PGDG yum repository (EL 8+ only); on
# failure it undoes the repository and module changes it made. The caller runs initdb.
pg_el_install_pgdg_server() {
    local elver
    elver=$(rpm -E %rhel 2> /dev/null)
    [[ "${elver}" =~ ^[0-9]+$ && "${elver}" -ge 8 ]] && command -v dnf > /dev/null 2>&1 || return 1
    [[ -f /var/lib/pgsql/data/PG_VERSION ]] && return 1
    local added_repo=false
    if ! rpm -q pgdg-redhat-repo > /dev/null 2>&1; then
        if dnf install -y -q "https://download.postgresql.org/pub/repos/yum/reporpms/EL-${elver}-$(uname -m)/pgdg-redhat-repo-latest.noarch.rpm" >&2; then
            added_repo=true
            dnf -qy module disable postgresql > /dev/null 2>&1 || true
        fi
    fi
    if rpm -q pgdg-redhat-repo > /dev/null 2>&1 \
        && dnf install -y -q "postgresql${PG_TARGET_MAJOR}-server" "postgresql${PG_TARGET_MAJOR}-contrib" >&2; then
        return 0
    fi
    if [[ "${added_repo}" == "true" ]]; then
        dnf -qy module reset postgresql > /dev/null 2>&1 || true
        dnf remove -y -q pgdg-redhat-repo >&2 || true
    fi
    echo -e "${yellow}PostgreSQL ${PG_TARGET_MAJOR} is not available for this system; installing the distribution's PostgreSQL instead.${plain}" >&2
    return 1
}

install_postgres_local() {
    local pg_user pg_pass
    pg_pass=$(gen_random_string 24)
    local pg_db="xui"
    local pg_host="127.0.0.1"
    local pg_port="5432"

    local pg_service="postgresql"
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update >&2 || return 1
            if pg_has_debian_cluster || ! pg_apt_install_pgdg_server; then
                apt-get install -y -q postgresql >&2 || return 1
            fi
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol | centos)
            if [[ "${release}" != "fedora" && "${release}" != "amzn" ]] && pg_el_install_pgdg_server; then
                pg_service="postgresql-${PG_TARGET_MAJOR}"
                [[ -f "/var/lib/pgsql/${PG_TARGET_MAJOR}/data/PG_VERSION" ]] \
                    || "/usr/pgsql-${PG_TARGET_MAJOR}/bin/postgresql-${PG_TARGET_MAJOR}-setup" initdb >&2 || return 1
            else
                if [[ "${release}" == "centos" && "${VERSION_ID}" =~ ^7 ]]; then
                    yum install -y postgresql-server postgresql-contrib >&2 || return 1
                else
                    dnf install -y -q postgresql-server postgresql-contrib >&2 || return 1
                fi
                [[ -d /var/lib/pgsql/data && -f /var/lib/pgsql/data/PG_VERSION ]] || postgresql-setup --initdb >&2 || return 1
            fi
            ;;
        arch | manjaro | parch)
            pacman -Sy --noconfirm postgresql >&2 || return 1
            if [[ ! -f /var/lib/postgres/data/PG_VERSION ]]; then
                sudo -u postgres initdb -D /var/lib/postgres/data >&2 || return 1
            fi
            ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper -q install -y postgresql-server postgresql-contrib >&2 || return 1
            if [[ ! -f /var/lib/pgsql/data/PG_VERSION ]]; then
                install -d -o postgres -g postgres -m 700 /var/lib/pgsql/data >&2 || return 1
                su - postgres -c "initdb -D /var/lib/pgsql/data" >&2 || return 1
            fi
            ;;
        alpine)
            apk add --no-cache postgresql postgresql-contrib >&2 || return 1
            if [[ ! -f /var/lib/postgresql/data/PG_VERSION ]]; then
                /etc/init.d/postgresql setup >&2 || return 1
            fi
            rc-update add postgresql default >&2 2> /dev/null || true
            rc-service postgresql start >&2 || return 1
            ;;
        *)
            echo -e "${red}Unsupported distro for automatic PostgreSQL install: ${release}${plain}" >&2
            return 1
            ;;
    esac

    if [[ "${release}" != "alpine" ]]; then
        systemctl enable --now "${pg_service}" >&2 || return 1
    fi

    # Wait briefly for the server to accept connections.
    local i
    for i in 1 2 3 4 5; do
        sudo -u postgres psql -tAc 'SELECT 1' > /dev/null 2>&1 && break
        sleep 1
    done

    local existing_owner=""
    existing_owner=$(sudo -u postgres psql -tAc \
        "SELECT pg_catalog.pg_get_userbyid(datdba) FROM pg_database WHERE datname='${pg_db}'" 2> /dev/null \
        | tr -d '[:space:]')
    if [[ -n "${existing_owner}" && "${existing_owner}" != "postgres" ]]; then
        pg_user="${existing_owner}"
    else
        pg_user=$(gen_random_string 8)
    fi

    # Idempotent role/db creation. Identifiers are double-quoted because a
    # random username may start with a digit, which Postgres rejects unquoted.
    sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${pg_user}'" 2> /dev/null \
        | grep -q 1 \
        || sudo -u postgres psql -c "CREATE USER \"${pg_user}\" WITH PASSWORD '${pg_pass}';" >&2 || return 1

    sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${pg_db}'" 2> /dev/null \
        | grep -q 1 \
        || sudo -u postgres psql -c "CREATE DATABASE \"${pg_db}\" OWNER \"${pg_user}\";" >&2 || return 1

    sudo -u postgres psql -c "ALTER USER \"${pg_user}\" WITH PASSWORD '${pg_pass}';" >&2 || return 1

    pg_ensure_hba_password_auth "${pg_db}" \
        || echo -e "${yellow}Warning: could not update pg_hba.conf; PostgreSQL may reject the panel's TCP login (ident auth).${plain}" >&2

    local pg_pass_enc
    pg_pass_enc=$(printf '%s' "${pg_pass}" | sed -e 's/%/%25/g' -e 's/:/%3A/g' -e 's/@/%40/g' -e 's|/|%2F|g' -e 's/?/%3F/g' -e 's/#/%23/g')

    if [[ -n "${PG_CRED_FILE:-}" ]]; then
        local prev_umask
        prev_umask=$(umask)
        umask 077
        if ! cat > "${PG_CRED_FILE}" << EOF; then
PG_USER=${pg_user}
PG_PASS=${pg_pass}
PG_HOST=${pg_host}
PG_PORT=${pg_port}
PG_DB=${pg_db}
EOF
            umask "${prev_umask}"
            echo -e "${red}Failed to write PostgreSQL credentials to ${PG_CRED_FILE}${plain}" >&2
            return 1
        fi
        umask "${prev_umask}"
    fi

    echo "postgres://${pg_user}:${pg_pass_enc}@${pg_host}:${pg_port}/${pg_db}?sslmode=disable"
    return 0
}

ensure_pg_client() {
    if command -v pg_dump > /dev/null 2>&1 && command -v pg_restore > /dev/null 2>&1; then
        return 0
    fi
    echo -e "${yellow}Installing PostgreSQL client tools (pg_dump/pg_restore) for in-panel backup...${plain}" >&2
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update >&2 && apt-get install -y -q postgresql-client >&2 || return 1
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf install -y -q postgresql >&2 || return 1
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum install -y postgresql >&2 || return 1
            else
                dnf install -y -q postgresql >&2 || return 1
            fi
            ;;
        arch | manjaro | parch)
            pacman -Sy --noconfirm postgresql >&2 || return 1
            ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper -q install -y postgresql >&2 || return 1
            ;;
        alpine)
            apk add --no-cache postgresql-client >&2 || return 1
            ;;
        *)
            return 1
            ;;
    esac
    command -v pg_dump > /dev/null 2>&1 && command -v pg_restore > /dev/null 2>&1
}

install_acme() {
    echo -e "${green}Installing acme.sh for SSL certificate management...${plain}"
    cd ~ || return 1
    curl -s https://get.acme.sh | sh > /dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${red}Failed to install acme.sh${plain}"
        return 1
    else
        echo -e "${green}acme.sh installed successfully${plain}"
    fi
    return 0
}

setup_ssl_certificate() {
    local domain="$1"
    local server_ip="$2"
    local existing_port="$3"
    local existing_webBasePath="$4"

    echo -e "${green}Setting up SSL certificate...${plain}"

    # Check if acme.sh is installed
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${yellow}Failed to install acme.sh, skipping SSL setup${plain}"
            return 1
        fi
    fi

    # Create certificate directory
    local certPath="/root/cert/${domain}"
    mkdir -p "$certPath"

    # Issue certificate
    echo -e "${green}Issuing SSL certificate for ${domain}...${plain}"
    echo -e "${yellow}Note: Port 80 must be open and accessible from the internet${plain}"

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1
    ~/.acme.sh/acme.sh --issue -d ${domain} $(acme_listen_flag) --standalone --httpport 80 --force

    if [ $? -ne 0 ]; then
        echo -e "${yellow}Failed to issue certificate for ${domain}${plain}"
        echo -e "${yellow}Please ensure port 80 is open and try again later with: x-ui${plain}"
        rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc 2> /dev/null
        rm -rf "$certPath" 2> /dev/null
        return 1
    fi

    # Install certificate
    ~/.acme.sh/acme.sh --installcert --force -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem \
        --reloadcmd "systemctl restart x-ui" > /dev/null 2>&1

    if [ $? -ne 0 ]; then
        echo -e "${yellow}Failed to install certificate${plain}"
        return 1
    fi

    # Enable auto-renew
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1
    # Secure permissions: private key readable only by owner
    chmod 600 $certPath/privkey.pem 2> /dev/null
    chmod 644 $certPath/fullchain.pem 2> /dev/null

    # Set certificate for panel
    local webCertFile="/root/cert/${domain}/fullchain.pem"
    local webKeyFile="/root/cert/${domain}/privkey.pem"

    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
        ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile" > /dev/null 2>&1
        echo -e "${green}SSL certificate installed and configured successfully!${plain}"
        return 0
    else
        echo -e "${yellow}Certificate files not found${plain}"
        return 1
    fi
}

# Issue Let's Encrypt IP certificate with shortlived profile (~6 days validity)
# Requires acme.sh and port 80 open for HTTP-01 challenge
setup_ip_certificate() {
    local ipv4="$1"
    local ipv6="${2:-}"
    local certDir="${3:-/root/cert/ip}"
    if ! ip_ssl_ipv4 "$ipv4" || { [[ -n "$ipv6" ]] && ! ip_ssl_public_ipv6 "$ipv6"; }; then
        printf 'Invalid IP certificate identifier.\n' >&2
        return 1
    fi

    echo -e "${green}Setting up Let's Encrypt IP certificate (shortlived profile)...${plain}"
    echo -e "${yellow}Note: IP certificates are valid for ~6 days and will auto-renew.${plain}"
    echo -e "${yellow}Default listener is port 80. If you choose another port, ensure external port 80 forwards to it.${plain}"

    # Check for acme.sh
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${red}Failed to install acme.sh${plain}"
            return 1
        fi
    fi

    # Set reload command for auto-renewal (add || true so it doesn't fail during first install)
    local reloadCmd="systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null || true"

    # Choose port for HTTP-01 listener (default 80, prompt override)
    local WebPort=""
    prompt_or_default WebPort "Port to use for ACME HTTP-01 listener (default 80): " "80" XUI_ACME_HTTP_PORT
    WebPort="${WebPort:-80}"
    if ! [[ "${WebPort}" =~ ^[0-9]+$ ]] || ((WebPort < 1 || WebPort > 65535)); then
        echo -e "${red}Invalid port provided. Falling back to 80.${plain}"
        WebPort=80
    fi
    echo -e "${green}Using port ${WebPort} for standalone validation.${plain}"
    if [[ "${WebPort}" -ne 80 ]]; then
        echo -e "${yellow}Reminder: Let's Encrypt still connects on port 80; forward external port 80 to ${WebPort}.${plain}"
    fi

    # Ensure chosen port is available
    while true; do
        if is_port_in_use "${WebPort}"; then
            echo -e "${yellow}Port ${WebPort} is in use.${plain}"

            local alt_port=""
            if [[ "$NONINTERACTIVE" == "1" ]]; then
                echo -e "${red}Port ${WebPort} is busy; cannot proceed in non-interactive mode.${plain}"
                return 1
            fi
            read -rp "Enter another port for acme.sh standalone listener (leave empty to abort): " alt_port
            alt_port="${alt_port// /}"
            if [[ -z "${alt_port}" ]]; then
                echo -e "${red}Port ${WebPort} is busy; cannot proceed.${plain}"
                return 1
            fi
            if ! [[ "${alt_port}" =~ ^[0-9]+$ ]] || ((alt_port < 1 || alt_port > 65535)); then
                echo -e "${red}Invalid port provided.${plain}"
                return 1
            fi
            WebPort="${alt_port}"
            continue
        else
            echo -e "${green}Port ${WebPort} is free and ready for standalone validation.${plain}"
            break
        fi
    done

    if [[ -n "${XUI_ACME_EMAIL:-}" ]]; then
        ~/.acme.sh/acme.sh --register-account --server letsencrypt -m "${XUI_ACME_EMAIL}" || return 1
    fi
    if ! ip_ssl_issue_and_install "$HOME/.acme.sh/acme.sh" "$ipv4" "$ipv6" "$WebPort" "$certDir" "$reloadCmd"; then
        return 1
    fi
    echo -e "${green}Certificate files installed and verified successfully${plain}"
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1

    # Configure panel to use the certificate
    echo -e "${green}Setting certificate paths for the panel...${plain}"
    ${xui_folder}/x-ui cert -webCert "${certDir}/fullchain.pem" -webCertKey "${certDir}/privkey.pem"

    if [ $? -ne 0 ]; then
        echo -e "${yellow}Warning: Could not set certificate paths automatically${plain}"
        echo -e "${yellow}Certificate files are at:${plain}"
        echo -e "  Cert: ${certDir}/fullchain.pem"
        echo -e "  Key:  ${certDir}/privkey.pem"
    else
        echo -e "${green}Certificate paths configured successfully${plain}"
    fi

    echo -e "${green}IP certificate installed and configured successfully!${plain}"
    echo -e "${green}Certificate valid for ~6 days, auto-renews via acme.sh cron job.${plain}"
    echo -e "${yellow}acme.sh will automatically renew and reload x-ui before expiry.${plain}"
    return 0
}

# Comprehensive manual SSL certificate issuance via acme.sh
ssl_cert_issue() {
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep 'webBasePath:' | awk -F': ' '{print $2}' | tr -d '[:space:]' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep 'port:' | awk -F': ' '{print $2}' | tr -d '[:space:]')

    # check for acme.sh first
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        echo "acme.sh could not be found. Installing now..."
        cd ~ || return 1
        curl -s https://get.acme.sh | sh
        if [ $? -ne 0 ]; then
            echo -e "${red}Failed to install acme.sh${plain}"
            return 1
        else
            echo -e "${green}acme.sh installed successfully${plain}"
        fi
    fi

    # get the domain here, and we need to verify it
    local domain=""
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        domain="${XUI_DOMAIN// /}"
        if [[ -z "$domain" ]] || ! is_domain "$domain"; then
            echo -e "${red}XUI_SSL_MODE=domain requires a valid XUI_DOMAIN (got: '${XUI_DOMAIN:-}').${plain}"
            return 1
        fi
    else
        while true; do
            read -rp "Please enter your domain name: " domain
            domain="${domain// /}" # Trim whitespace

            if [[ -z "$domain" ]]; then
                echo -e "${red}Domain name cannot be empty. Please try again.${plain}"
                continue
            fi

            if ! is_domain "$domain"; then
                echo -e "${red}Invalid domain format: ${domain}. Please enter a valid domain name.${plain}"
                continue
            fi

            break
        done
    fi
    echo -e "${green}Your domain is: ${domain}, checking it...${plain}"
    SSL_ISSUED_DOMAIN="${domain}"

    # detect existing certificate and reuse it only if its files are actually
    # present and non-empty. acme.sh stores ECC certs under ${domain}_ecc and RSA
    # certs under ${domain}; a failed issuance can leave a domain entry in --list
    # with no usable cert files, which must not be reused (it produces a 0-byte
    # fullchain.pem). Broken partial state is cleaned up so issuance can proceed.
    local cert_exists=0
    if ~/.acme.sh/acme.sh --list 2> /dev/null | awk '{print $1}' | grep -Fxq "${domain}"; then
        local acmeCertDir=""
        if [[ -s ~/.acme.sh/${domain}_ecc/fullchain.cer && -s ~/.acme.sh/${domain}_ecc/${domain}.key ]]; then
            acmeCertDir=~/.acme.sh/${domain}_ecc
        elif [[ -s ~/.acme.sh/${domain}/fullchain.cer && -s ~/.acme.sh/${domain}/${domain}.key ]]; then
            acmeCertDir=~/.acme.sh/${domain}
        fi
        if [[ -n "${acmeCertDir}" ]]; then
            cert_exists=1
            local certInfo=$(~/.acme.sh/acme.sh --list 2> /dev/null | grep -F "${domain}")
            echo -e "${yellow}Existing certificate found for ${domain}, will reuse it.${plain}"
            [[ -n "${certInfo}" ]] && echo "$certInfo"
        else
            echo -e "${yellow}Found incomplete acme.sh state for ${domain} (no valid certificate files); cleaning it up and re-issuing.${plain}"
            rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc
        fi
    fi
    if [[ ${cert_exists} -eq 0 ]]; then
        echo -e "${green}Your domain is ready for issuing certificates now...${plain}"
    fi

    # create a directory for the certificate
    certPath="/root/cert/${domain}"
    if [ ! -d "$certPath" ]; then
        mkdir -p "$certPath"
    else
        rm -rf "$certPath"
        mkdir -p "$certPath"
    fi

    # get the port number for the standalone server
    local WebPort=80
    prompt_or_default WebPort "Please choose which port to use (default is 80): " "80" XUI_ACME_HTTP_PORT
    if [[ -z ${WebPort} ]]; then
        WebPort=80
    elif [[ ! ${WebPort} =~ ^[1-9][0-9]*$ || ${WebPort} -gt 65535 ]]; then
        echo -e "${yellow}Your input ${WebPort} is invalid, will use default port 80.${plain}"
        WebPort=80
    fi
    echo -e "${green}Will use port: ${WebPort} to issue certificates. Please make sure this port is open.${plain}"

    # Stop panel temporarily
    echo -e "${yellow}Stopping panel temporarily...${plain}"
    systemctl stop x-ui 2> /dev/null || rc-service x-ui stop 2> /dev/null

    if [[ ${cert_exists} -eq 0 ]]; then
        # issue the certificate
        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
        [[ -n "${XUI_ACME_EMAIL:-}" ]] && ~/.acme.sh/acme.sh --register-account -m "${XUI_ACME_EMAIL}" > /dev/null 2>&1
        ~/.acme.sh/acme.sh --issue -d ${domain} $(acme_listen_flag) --standalone --httpport ${WebPort} --force
        if [ $? -ne 0 ]; then
            echo -e "${red}Issuing certificate failed, please check logs.${plain}"
            rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc
            systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null
            return 1
        else
            echo -e "${green}Issuing certificate succeeded, installing certificates...${plain}"
        fi
    else
        echo -e "${green}Using existing certificate, installing certificates...${plain}"
    fi

    # Setup reload command
    reloadCmd="systemctl restart x-ui || rc-service x-ui restart"
    echo -e "${green}Default --reloadcmd for ACME is: ${yellow}systemctl restart x-ui || rc-service x-ui restart${plain}"
    echo -e "${green}This command will run on every certificate issue and renew.${plain}"
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        setReloadcmd="n"
    else
        read -rp "Would you like to modify --reloadcmd for ACME? (y/n): " setReloadcmd
    fi
    if [[ "$setReloadcmd" == "y" || "$setReloadcmd" == "Y" ]]; then
        echo -e "\n${green}\t1.${plain} Preset: systemctl reload nginx ; systemctl restart x-ui"
        echo -e "${green}\t2.${plain} Input your own command"
        echo -e "${green}\t0.${plain} Keep default reloadcmd"
        read -rp "Choose an option: " choice
        case "$choice" in
            1)
                echo -e "${green}Reloadcmd is: systemctl reload nginx ; systemctl restart x-ui${plain}"
                reloadCmd="systemctl reload nginx ; systemctl restart x-ui"
                ;;
            2)
                echo -e "${yellow}It's recommended to put x-ui restart at the end${plain}"
                read -rp "Please enter your custom reloadcmd: " reloadCmd
                echo -e "${green}Reloadcmd is: ${reloadCmd}${plain}"
                ;;
            *)
                echo -e "${green}Keeping default reloadcmd${plain}"
                ;;
        esac
    fi

    # install the certificate
    local installOutput=""
    installOutput=$(~/.acme.sh/acme.sh --installcert --force -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem --reloadcmd "${reloadCmd}" 2>&1)
    local installRc=$?
    echo "${installOutput}"

    local installWroteFiles=0
    if echo "${installOutput}" | grep -q "Installing key to:" && echo "${installOutput}" | grep -q "Installing full chain to:"; then
        installWroteFiles=1
    fi

    if [[ -f "/root/cert/${domain}/privkey.pem" && -f "/root/cert/${domain}/fullchain.pem" && (${installRc} -eq 0 || ${installWroteFiles} -eq 1) ]]; then
        echo -e "${green}Installing certificate succeeded, enabling auto renew...${plain}"
    else
        echo -e "${red}Installing certificate failed, exiting.${plain}"
        if [[ ${cert_exists} -eq 0 ]]; then
            rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc
        fi
        systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null
        return 1
    fi

    # enable auto-renew
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        echo -e "${yellow}Auto renew setup had issues, certificate details:${plain}"
        ls -lah /root/cert/${domain}/
        # Secure permissions: private key readable only by owner
        chmod 600 $certPath/privkey.pem 2> /dev/null
        chmod 644 $certPath/fullchain.pem 2> /dev/null
    else
        echo -e "${green}Auto renew succeeded, certificate details:${plain}"
        ls -lah /root/cert/${domain}/
        # Secure permissions: private key readable only by owner
        chmod 600 $certPath/privkey.pem 2> /dev/null
        chmod 644 $certPath/fullchain.pem 2> /dev/null
    fi

    # start panel
    systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null

    # Prompt user to set panel paths after successful certificate installation
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        setPanel="y"
    else
        read -rp "Would you like to set this certificate for the panel? (y/n): " setPanel
    fi
    if [[ "$setPanel" == "y" || "$setPanel" == "Y" ]]; then
        local webCertFile="/root/cert/${domain}/fullchain.pem"
        local webKeyFile="/root/cert/${domain}/privkey.pem"

        if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
            ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
            echo -e "${green}Certificate paths set for the panel${plain}"
            echo -e "${green}Certificate File: $webCertFile${plain}"
            echo -e "${green}Private Key File: $webKeyFile${plain}"
            echo ""
            echo -e "${green}Access URL: https://${domain}:${existing_port}/${existing_webBasePath}${plain}"
            echo -e "${yellow}Panel will restart to apply SSL certificate...${plain}"
            systemctl restart x-ui 2> /dev/null || rc-service x-ui restart 2> /dev/null
        else
            echo -e "${red}Error: Certificate or private key file not found for domain: $domain.${plain}"
        fi
    else
        echo -e "${yellow}Skipping panel path setting.${plain}"
    fi

    return 0
}

# Reusable interactive SSL setup (domain or IP)
# Sets global `SSL_HOST` to the chosen domain/IP for Access URL usage
prompt_and_setup_ssl() {
    local panel_port="$1"
    local web_base_path="$2"
    local server_ip="$3"

    local ssl_choice=""
    SSL_SCHEME="https"

    echo -e "${yellow}Choose SSL certificate setup method:${plain}"
    echo -e "${green}1.${plain} Let's Encrypt for Domain (90-day validity, auto-renews)"
    echo -e "${green}2.${plain} Let's Encrypt for IP Address (6-day validity, auto-renews)"
    echo -e "${green}3.${plain} Custom SSL Certificate (Path to existing files)"
    echo -e "${green}4.${plain} Skip SSL (advanced — behind reverse proxy / SSH tunnel only)"
    echo -e "${blue}Note:${plain} Options 1 & 2 require port 80 open. Option 3 requires manual paths."
    echo -e "${blue}Note:${plain} Option 4 serves the panel over plain HTTP — only safe behind nginx/Caddy or an SSH tunnel."
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        case "${XUI_SSL_MODE:-none}" in
            domain) ssl_choice="1" ;;
            ip) ssl_choice="2" ;;
            none | "") ssl_choice="4" ;;
            *)
                echo -e "${yellow}Unknown XUI_SSL_MODE='${XUI_SSL_MODE}', defaulting to none (HTTP).${plain}"
                ssl_choice="4"
                ;;
        esac
    else
        read -rp "Choose an option (default 2 for IP): " ssl_choice
        ssl_choice="${ssl_choice// /}" # Trim whitespace

        # Default to 2 (IP cert) if input is empty or invalid (not 1, 3 or 4)
        if [[ "$ssl_choice" != "1" && "$ssl_choice" != "3" && "$ssl_choice" != "4" ]]; then
            ssl_choice="2"
        fi
    fi

    case "$ssl_choice" in
        1)
            # User chose Let's Encrypt domain option
            echo -e "${green}Using Let's Encrypt for domain certificate...${plain}"
            if ssl_cert_issue; then
                local cert_domain="${SSL_ISSUED_DOMAIN}"
                if [[ -z "${cert_domain}" ]]; then
                    cert_domain=$(~/.acme.sh/acme.sh --list 2> /dev/null | tail -1 | awk '{print $1}')
                fi

                if [[ -n "${cert_domain}" ]]; then
                    SSL_HOST="${cert_domain}"
                    echo -e "${green}✓ SSL certificate configured successfully with domain: ${cert_domain}${plain}"
                else
                    echo -e "${yellow}SSL setup may have completed, but domain extraction failed${plain}"
                    SSL_HOST="${server_ip}"
                fi
            else
                echo -e "${red}SSL certificate setup failed for domain mode.${plain}"
                SSL_HOST="${server_ip}"
            fi
            ;;
        2)
            # User chose Let's Encrypt IP certificate option
            echo -e "${green}Using Let's Encrypt for IP certificate (shortlived profile)...${plain}"

            # Confirm the auto-detected IP before issuing for it: with asymmetric
            # routing / multi-WAN the echo services can return a transit address.
            if [[ "$NONINTERACTIVE" != "1" ]]; then
                local ip_confirm=""
                read -rp "Is ${server_ip} the correct incoming public IPv4 address for this server? [Default y]: " ip_confirm
                if [[ -n "$ip_confirm" && "$ip_confirm" != "y" && "$ip_confirm" != "Y" ]]; then
                    server_ip=""
                    while [[ -z "$server_ip" ]]; do
                        read -rp "Please enter your server's public IPv4 address: " server_ip
                        server_ip="${server_ip// /}"
                        if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                            echo -e "${red}Invalid IPv4 address. Please try again.${plain}"
                            server_ip=""
                        fi
                    done
                fi
            fi

            local ipv6_addr
            ipv6_addr=$(ip_ssl_select_ipv6 "$((1 - NONINTERACTIVE))" "${XUI_SSL_IPV6:-}") || return 1

            local panel_was_running=0
            # Remember the prior state so failed issuance does not leave it down.
            if [[ $release == "alpine" ]]; then
                if rc-service x-ui status > /dev/null 2>&1; then panel_was_running=1; fi
                rc-service x-ui stop > /dev/null 2>&1
            else
                if systemctl is-active --quiet x-ui > /dev/null 2>&1; then panel_was_running=1; fi
                systemctl stop x-ui > /dev/null 2>&1
            fi

            if setup_ip_certificate "${server_ip}" "${ipv6_addr}"; then
                SSL_HOST="${server_ip}"
                echo -e "${green}✓ Let's Encrypt IP certificate configured successfully${plain}"
            else
                echo -e "${red}✗ IP certificate setup failed. Please check port 80 on both selected addresses.${plain}"
                SSL_HOST="${server_ip}"
                if ((panel_was_running)); then
                    if [[ $release == "alpine" ]]; then
                        rc-service x-ui start || echo -e "${red}Could not restore the previously running panel.${plain}"
                    else
                        systemctl start x-ui || echo -e "${red}Could not restore the previously running panel.${plain}"
                    fi
                fi
                return 1
            fi
            ;;
        3)
            # User chose Custom Paths (User Provided) option
            echo -e "${green}Using custom existing certificate...${plain}"
            local custom_cert=""
            local custom_key=""
            local custom_domain=""

            # 3.1 Request Domain to compose Panel URL later
            read -rp "Please enter domain name certificate issued for: " custom_domain
            custom_domain="${custom_domain// /}" # Remove spaces

            # 3.2 Loop for Certificate Path
            while true; do
                read -rp "Input certificate path (keywords: .crt / fullchain): " custom_cert
                # Strip quotes if present
                custom_cert=$(echo "$custom_cert" | tr -d '"' | tr -d "'")

                if [[ -f "$custom_cert" && -r "$custom_cert" && -s "$custom_cert" ]]; then
                    break
                elif [[ ! -f "$custom_cert" ]]; then
                    echo -e "${red}Error: File does not exist! Try again.${plain}"
                elif [[ ! -r "$custom_cert" ]]; then
                    echo -e "${red}Error: File exists but is not readable (check permissions)!${plain}"
                else
                    echo -e "${red}Error: File is empty!${plain}"
                fi
            done

            # 3.3 Loop for Private Key Path
            while true; do
                read -rp "Input private key path (keywords: .key / privatekey): " custom_key
                # Strip quotes if present
                custom_key=$(echo "$custom_key" | tr -d '"' | tr -d "'")

                if [[ -f "$custom_key" && -r "$custom_key" && -s "$custom_key" ]]; then
                    break
                elif [[ ! -f "$custom_key" ]]; then
                    echo -e "${red}Error: File does not exist! Try again.${plain}"
                elif [[ ! -r "$custom_key" ]]; then
                    echo -e "${red}Error: File exists but is not readable (check permissions)!${plain}"
                else
                    echo -e "${red}Error: File is empty!${plain}"
                fi
            done

            # 3.4 Apply Settings via x-ui binary
            ${xui_folder}/x-ui cert -webCert "$custom_cert" -webCertKey "$custom_key" > /dev/null 2>&1

            # Set SSL_HOST for composing Panel URL
            if [[ -n "$custom_domain" ]]; then
                SSL_HOST="$custom_domain"
            else
                SSL_HOST="${server_ip}"
            fi

            echo -e "${green}✓ Custom certificate paths applied.${plain}"
            echo -e "${yellow}Note: You are responsible for renewing these files externally.${plain}"

            systemctl restart x-ui > /dev/null 2>&1 || rc-service x-ui restart > /dev/null 2>&1
            ;;
        4)
            echo ""
            echo -e "${red}⚠ Panel will be installed WITHOUT SSL/TLS.${plain}"
            echo -e "${yellow}Login credentials and cookies will travel as plain HTTP.${plain}"
            echo -e "${yellow}Only safe when:${plain}"
            echo -e "${yellow}  • A reverse proxy (nginx, Caddy, Traefik) terminates TLS for you, or${plain}"
            echo -e "${yellow}  • You access the panel exclusively via SSH tunnel${plain}"
            echo ""

            SSL_SCHEME="http"
            SSL_HOST="${server_ip}"

            local bind_local=""
            if [[ "$NONINTERACTIVE" == "1" ]]; then
                # Cloud images must stay reachable on their public interface.
                bind_local="n"
            else
                read -rp "Bind the panel to 127.0.0.1 only? (recommended — forces SSH tunnel / reverse-proxy access) [y/N]: " bind_local
            fi
            if [[ "$bind_local" == "y" || "$bind_local" == "Y" ]]; then
                ${xui_folder}/x-ui setting -listenIP "127.0.0.1" > /dev/null 2>&1
                SSL_HOST="127.0.0.1"
                echo -e "${green}✓ Panel bound to 127.0.0.1 only. It is now unreachable from the public internet.${plain}"
                echo ""
                echo -e "${green}SSH Port Forwarding — open the panel from your local machine via:${plain}"
                echo -e "  Standard SSH command:"
                echo -e "  ${yellow}ssh -L 2222:127.0.0.1:${panel_port} root@${server_ip}${plain}"
                echo -e "  If using an SSH key:"
                echo -e "  ${yellow}ssh -i <sshkeypath> -L 2222:127.0.0.1:${panel_port} root@${server_ip}${plain}"
                echo -e "  Then open in your browser:"
                echo -e "  ${yellow}http://localhost:2222/${web_base_path}${plain}"
                echo ""
                echo -e "${yellow}Alternative: point a reverse proxy (nginx/Caddy) at 127.0.0.1:${panel_port} and let it terminate TLS.${plain}"
            else
                echo -e "${yellow}Panel will listen on all interfaces over plain HTTP. Make sure something else is terminating TLS in front of it.${plain}"
            fi

            systemctl restart x-ui > /dev/null 2>&1 || rc-service x-ui restart > /dev/null 2>&1
            echo -e "${green}✓ SSL setup skipped.${plain}"
            ;;
        *)
            echo -e "${red}Invalid option. Skipping SSL setup.${plain}"
            SSL_HOST="${server_ip}"
            ;;
    esac
}

config_after_install() {
    local existing_hasDefaultCredential=$(${xui_folder}/x-ui setting -show true | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    # Properly detect empty cert by checking if cert: line exists and has content after it
    local existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
    local URL_lists=(
        "https://api4.ipify.org"
        "https://ipv4.icanhazip.com"
        "https://v4.api.ipinfo.io/ip"
        "https://ipv4.myexternalip.com/raw"
        "https://4.ident.me"
        "https://check-host.net/ip"
    )
    local server_ip=""
    for ip_address in "${URL_lists[@]}"; do
        local response=$(curl -s -w "\n%{http_code}" --max-time 3 "${ip_address}" 2> /dev/null)
        local http_code=$(echo "$response" | tail -n1)
        local ip_result=$(echo "$response" | head -n-1 | tr -d '[:space:]"')
        if [[ "${http_code}" == "200" && "${ip_result}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            server_ip="${ip_result}"
            break
        fi
    done

    if [[ -z "$server_ip" ]]; then
        if [[ "$NONINTERACTIVE" == "1" ]]; then
            # Panel binds 0.0.0.0 regardless; the IP is only used to compose the
            # displayed access URL. Fall back to XUI_SERVER_IP or leave blank.
            server_ip="${XUI_SERVER_IP:-}"
        else
            echo -e "${yellow}Could not auto-detect server IP from any provider.${plain}"
            while [[ -z "$server_ip" ]]; do
                read -rp "Please enter your server's public IPv4 address: " server_ip
                server_ip="${server_ip// /}"
                if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                    echo -e "${red}Invalid IPv4 address. Please try again.${plain}"
                    server_ip=""
                fi
            done
        fi
    fi

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath="${XUI_WEB_BASE_PATH:-$(gen_random_string 18)}"
            local config_username="${XUI_USERNAME:-$(gen_random_string 10)}"
            local config_password="${XUI_PASSWORD:-$(gen_random_string 10)}"
            local config_port=""

            local db_label="SQLite (/etc/x-ui/x-ui.db)"
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     Database Selection                    ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "  1) SQLite     (default — recommended for < 500 clients)"
            echo -e "  2) PostgreSQL (recommended for high client counts / many nodes)"
            if [[ "$NONINTERACTIVE" == "1" ]]; then
                if [[ "${XUI_DB_TYPE:-sqlite}" == "postgres" ]]; then
                    db_choice="2"
                else
                    db_choice="1"
                fi
            else
                read -rp "Choose [1]: " db_choice
                db_choice="${db_choice:-1}"
            fi
            if [[ "$db_choice" == "2" ]]; then
                local xui_env_file
                case "${release}" in
                    ubuntu | debian | armbian)
                        xui_env_file="/etc/default/x-ui"
                        ;;
                    arch | manjaro | parch | alpine)
                        xui_env_file="/etc/conf.d/x-ui"
                        ;;
                    *)
                        xui_env_file="/etc/sysconfig/x-ui"
                        ;;
                esac

                local xui_dsn=""
                local pg_mode=""
                local pg_local_installed=0
                while [[ -z "$xui_dsn" ]]; do
                    if [[ "$NONINTERACTIVE" == "1" ]]; then
                        if [[ -n "${XUI_DB_DSN:-}" ]]; then
                            xui_dsn="${XUI_DB_DSN}"
                            db_label="PostgreSQL (external)"
                            break
                        fi
                        echo -e "${yellow}Installing PostgreSQL locally (non-interactive)...${plain}"
                        local pg_cred_file
                        pg_cred_file=$(mktemp 2> /dev/null) || pg_cred_file=$(mktemp -t x-ui-pg-creds.XXXXXXXX)
                        if [[ -n "${pg_cred_file}" ]] && xui_dsn=$(PG_CRED_FILE="${pg_cred_file}" install_postgres_local); then
                            pg_local_installed=1
                            if [[ -r "${pg_cred_file}" ]]; then
                                # shellcheck disable=SC1090
                                source "${pg_cred_file}"
                            fi
                            rm -f "${pg_cred_file}"
                            db_label="PostgreSQL (${PG_USER}@${PG_HOST}:${PG_PORT}/${PG_DB})"
                            break
                        fi
                        rm -f "${pg_cred_file}"
                        echo -e "${red}PostgreSQL installation failed in non-interactive mode; aborting.${plain}"
                        echo -e "${yellow}Set XUI_DB_DSN to use an existing server, or XUI_DB_TYPE=sqlite.${plain}"
                        exit 1
                    fi
                    echo ""
                    echo -e "  1) Install PostgreSQL locally and create a dedicated user/db (recommended)"
                    echo -e "  2) Use an existing PostgreSQL server (enter DSN)"
                    read -rp "Choose [1]: " pg_mode
                    pg_mode="${pg_mode:-1}"
                    if [[ "$pg_mode" == "2" ]]; then
                        while [[ -z "$xui_dsn" ]]; do
                            read -rp "Enter PostgreSQL DSN (postgres://user:pass@host:port/dbname?sslmode=disable): " xui_dsn
                            xui_dsn="${xui_dsn// /}"
                        done
                        db_label="PostgreSQL (external)"
                    else
                        echo -e "${yellow}Installing PostgreSQL — this may take a moment...${plain}"
                        local pg_cred_file
                        pg_cred_file=$(mktemp 2> /dev/null) || pg_cred_file=$(mktemp -t x-ui-pg-creds.XXXXXXXX)
                        if [[ -z "${pg_cred_file}" ]]; then
                            echo -e "${red}Failed to create temporary credentials file.${plain}"
                            xui_dsn=""
                            continue
                        fi
                        if xui_dsn=$(PG_CRED_FILE="${pg_cred_file}" install_postgres_local); then
                            pg_local_installed=1
                            if [[ -r "${pg_cred_file}" ]]; then
                                # shellcheck disable=SC1090
                                source "${pg_cred_file}"
                            fi
                            rm -f "${pg_cred_file}"
                            db_label="PostgreSQL (${PG_USER}@${PG_HOST}:${PG_PORT}/${PG_DB})"
                        else
                            rm -f "${pg_cred_file}"
                            echo ""
                            echo -e "${red}PostgreSQL installation failed.${plain}"
                            echo -e "  1) Retry local install"
                            echo -e "  2) Enter an external DSN instead"
                            echo -e "  3) Abort install"
                            echo -e "  4) Fall back to SQLite"
                            read -rp "Choose [1]: " pg_fail
                            pg_fail="${pg_fail:-1}"
                            case "$pg_fail" in
                                2) pg_mode="2" ;;
                                3)
                                    echo -e "${red}Install aborted.${plain}"
                                    exit 1
                                    ;;
                                4)
                                    db_choice="1"
                                    xui_dsn=""
                                    break
                                    ;;
                                *) xui_dsn="" ;;
                            esac
                        fi
                    fi
                done
                if [[ -n "$xui_dsn" ]]; then
                    install -d -m 755 "$(dirname "$xui_env_file")"
                    umask 077
                    cat > "$xui_env_file" << EOF
XUI_DB_TYPE=postgres
XUI_DB_DSN=${xui_dsn}
EOF
                    chmod 600 "$xui_env_file"
                    umask 022
                    export XUI_DB_TYPE=postgres
                    export XUI_DB_DSN="${xui_dsn}"
                    ensure_pg_client || echo -e "${yellow}⚠ Could not install pg_dump/pg_restore. In-panel database backup/restore will be unavailable until you install the postgresql-client package.${plain}"
                fi
            fi

            if [[ "$NONINTERACTIVE" == "1" ]]; then
                if [[ -n "${XUI_PANEL_PORT:-}" ]]; then
                    config_port="${XUI_PANEL_PORT}"
                    echo -e "${yellow}Your Panel Port is: ${config_port}${plain}"
                else
                    config_port=$(shuf -i 1024-62000 -n 1)
                    echo -e "${yellow}Generated random port: ${config_port}${plain}"
                fi
            else
                read -rp "Would you like to customize the Panel Port settings? (If not, a random port will be applied) [y/n]: " config_confirm
                if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
                    read -rp "Please set up the panel port: " config_port
                    echo -e "${yellow}Your Panel Port is: ${config_port}${plain}"
                else
                    config_port=$(shuf -i 1024-62000 -n 1)
                    echo -e "${yellow}Generated random port: ${config_port}${plain}"
                fi
            fi

            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"

            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     SSL Certificate Setup (RECOMMENDED)   ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}SSL is strongly recommended. Skip only if a reverse proxy${plain}"
            echo -e "${yellow}or SSH tunnel handles TLS for you.${plain}"
            echo -e "${yellow}Let's Encrypt now supports both domains and IP addresses!${plain}"
            echo ""

            prompt_and_setup_ssl "${config_port}" "${config_webBasePath}" "${server_ip}" || return 1

            # Retrieve the API token for display
            local config_apiToken=$(${xui_folder}/x-ui setting -getApiToken | grep -Eo 'apiToken: .+' | awk '{print $2}')

            # Display final credentials and access information
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     Panel Installation Complete!         ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}Username:    ${config_username}${plain}"
            echo -e "${green}Password:    ${config_password}${plain}"
            echo -e "${green}Port:        ${config_port}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "${green}Database:    ${db_label}${plain}"
            echo -e "${green}Access URL:  ${SSL_SCHEME}://${SSL_HOST}:${config_port}/${config_webBasePath}${plain}"
            echo -e "${green}API Token:   ${config_apiToken}${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}⚠ IMPORTANT: Save these credentials securely!${plain}"
            if [[ "$SSL_SCHEME" == "https" ]]; then
                echo -e "${yellow}⚠ SSL Certificate: Enabled and configured${plain}"
            else
                echo -e "${yellow}⚠ SSL Certificate: Skipped — panel is HTTP-only. Use a reverse proxy or SSH tunnel.${plain}"
            fi

            if [[ "$db_choice" == "2" ]]; then
                echo ""
                echo -e "${green}PostgreSQL backup & restore is built into the panel:${plain}"
                echo -e "  ${blue}${SSL_SCHEME}://${SSL_HOST}:${config_port}/${config_webBasePath}${plain} → Backup & Restore"
                echo -e "${yellow}  Back Up downloads a pg_dump .dump file; Restore reloads it via pg_restore.${plain}"
            fi

            if [[ "$db_choice" == "2" && "$pg_local_installed" == "1" ]]; then
                echo ""
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${green}     PostgreSQL Credentials               ${plain}"
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${green}DB Name:    ${PG_DB}${plain}"
                echo -e "${green}Username:   ${PG_USER}${plain}"
                echo -e "${green}Password:   ${PG_PASS}${plain}"
                echo -e "${green}Host:       ${PG_HOST}${plain}"
                echo -e "${green}Port:       ${PG_PORT}${plain}"
                echo -e "${green}DSN:        ${xui_dsn}${plain}"
                echo -e "${green}Env file:   ${xui_env_file}${plain}"
                echo -e "${green}-------------------------------------------${plain}"
                echo -e "${green}Connect from this server:${plain}"
                echo -e "  ${blue}sudo -u postgres psql -d ${PG_DB}${plain}      (as the postgres superuser)"
                echo -e "  ${blue}PGPASSWORD='${PG_PASS}' psql -h ${PG_HOST} -p ${PG_PORT} -U ${PG_USER} -d ${PG_DB}${plain}"
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${yellow}⚠ The panel reads these credentials from ${xui_env_file}.${plain}"
                echo -e "${yellow}⚠ Save the password — it is not stored anywhere else in plain text.${plain}"
                unset PG_USER PG_PASS PG_HOST PG_PORT PG_DB
            fi

            # Persist a machine-parseable credentials file for cloud-init / MOTD.
            : "${SSL_SCHEME:=https}"
            : "${SSL_HOST:=${server_ip}}"
            local db_type_out="sqlite"
            [[ "$db_choice" == "2" ]] && db_type_out="postgres"
            write_install_result "${config_username}" "${config_password}" "${config_port}" \
                "${config_webBasePath}" "${SSL_SCHEME}" "${SSL_HOST}" "${config_apiToken}" "${db_type_out}"
        else
            local config_webBasePath=$(gen_random_string 18)
            echo -e "${yellow}WebBasePath is missing or too short. Generating a new one...${plain}"
            ${xui_folder}/x-ui setting -webBasePath "${config_webBasePath}"
            echo -e "${green}New WebBasePath: ${config_webBasePath}${plain}"

            # If the panel is already installed but no certificate is configured, prompt for SSL now
            if [[ -z "${existing_cert}" ]]; then
                echo ""
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${green}     SSL Certificate Setup (RECOMMENDED)   ${plain}"
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${yellow}Let's Encrypt now supports both domains and IP addresses!${plain}"
                echo ""
                prompt_and_setup_ssl "${existing_port}" "${config_webBasePath}" "${server_ip}" || return 1
                echo -e "${green}Access URL:  ${SSL_SCHEME}://${SSL_HOST}:${existing_port}/${config_webBasePath}${plain}"
            else
                # If a cert already exists, just show the access URL
                echo -e "${green}Access URL: https://${server_ip}:${existing_port}/${config_webBasePath}${plain}"
            fi
        fi
    else
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_username="${XUI_USERNAME:-$(gen_random_string 10)}"
            local config_password="${XUI_PASSWORD:-$(gen_random_string 10)}"

            echo -e "${yellow}Default credentials detected. Security update required...${plain}"
            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}"
            echo -e "Generated new random login credentials:"
            echo -e "###############################################"
            echo -e "${green}Username: ${config_username}${plain}"
            echo -e "${green}Password: ${config_password}${plain}"
            echo -e "###############################################"

            # Persist a machine-parseable credentials file for cloud-init / MOTD.
            local config_apiToken
            config_apiToken=$(${xui_folder}/x-ui setting -getApiToken | grep -Eo 'apiToken: .+' | awk '{print $2}')
            : "${SSL_SCHEME:=https}"
            : "${SSL_HOST:=${server_ip}}"
            write_install_result "${config_username}" "${config_password}" "${existing_port}" \
                "${existing_webBasePath}" "${SSL_SCHEME}" "${SSL_HOST}" "${config_apiToken}" "${XUI_DB_TYPE:-sqlite}"
        else
            echo -e "${green}Username, Password, and WebBasePath are properly set.${plain}"
        fi

        # Existing install: if no cert configured, prompt user for SSL setup
        # Properly detect empty cert by checking if cert: line exists and has content after it
        existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
        if [[ -z "$existing_cert" ]]; then
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     SSL Certificate Setup (RECOMMENDED)   ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}Let's Encrypt now supports both domains and IP addresses!${plain}"
            echo ""
            prompt_and_setup_ssl "${existing_port}" "${existing_webBasePath}" "${server_ip}" || return 1
            echo -e "${green}Access URL:  ${SSL_SCHEME}://${SSL_HOST}:${existing_port}/${existing_webBasePath}${plain}"
        else
            echo -e "${green}SSL certificate already configured. No action needed.${plain}"
        fi
    fi

    ${xui_folder}/x-ui migrate
}

# setup_fail2ban auto-installs and configures fail2ban for the IP Limit feature
# by invoking the freshly installed x-ui CLI. IP Limit is load-bearing on
# fail2ban (without it the panel disables the limitIp field and zeroes existing
# limits), so a fresh install should make it work out of the box, just like the
# Docker image already does. Non-fatal by design: a fail2ban failure must never
# abort the panel install.
setup_fail2ban() {
    if [[ -n "${XUI_ENABLE_FAIL2BAN+x}" && "${XUI_ENABLE_FAIL2BAN}" != "true" ]]; then
        echo -e "${yellow}XUI_ENABLE_FAIL2BAN=${XUI_ENABLE_FAIL2BAN}, skipping Fail2ban auto-setup.${plain}"
        return 0
    fi

    if [[ ! -x /usr/bin/x-ui ]]; then
        echo -e "${yellow}x-ui CLI not found; skipping Fail2ban auto-setup.${plain}"
        return 0
    fi

    # Scripts older than v3.4.0 have no setup-fail2ban and exit 0 from the
    # usage banner, which would read as success here.
    if ! grep -q '"setup-fail2ban")' /usr/bin/x-ui; then
        echo -e "${yellow}This x-ui.sh predates 'x-ui setup-fail2ban'; skipping Fail2ban auto-setup.${plain}"
        return 0
    fi

    echo -e "${green}Setting up Fail2ban for the IP Limit feature...${plain}"
    if /usr/bin/x-ui setup-fail2ban; then
        echo -e "${green}Fail2ban setup complete.${plain}"
    else
        echo -e "${yellow}Fail2ban setup did not finish; IP Limit stays disabled until you run 'x-ui' and open the IP Limit menu. Continuing.${plain}"
    fi
    return 0
}

# Major version of the local systemd, 0 when it cannot be determined. The
# SystemCallFilter=@system-service group only exists from systemd 239 on (other
# @-named groups exist since 231); on older versions an unknown group is not
# ignored safely, the filter stays in force and leaves a whitelist the panel
# cannot run under.
_xui_systemd_major_version() {
    local version=""
    if command -v systemctl > /dev/null 2>&1; then
        version="$(systemctl --version 2>/dev/null | awk 'NR == 1 {print $2}')"
    fi
    if [[ ! "$version" =~ ^[0-9]+$ ]]; then
        echo 0
        return 0
    fi
    echo "$version"
}

# The shipped units list hardening that older systemd does not know: the
# directive is logged and ignored at load time rather than rejected, so the
# panel still starts, only without that protection. Each entry is the systemd
# release that introduced the directive (systemd.exec(5)); everything else in
# the unit predates the oldest systemd install.sh supports (CentOS 7 has 219).
# SystemCallFilter= is listed because the drop-in only writes it from 239 on.
_xui_warn_unsupported_hardening() {
    local version entry missing=""
    version="$(_xui_systemd_major_version)"
    [[ "$version" -gt 0 ]] || return 0
    for entry in RestrictRealtime:231 ReadWritePaths:231 ProtectKernelTunables:232 \
        ProtectKernelModules:232 RestrictNamespaces:233 LockPersonality:235 \
        SystemCallFilter:239 ProtectHostname:242 RestrictSUIDSGID:242 \
        ProtectKernelLogs:244 ProtectClock:245; do
        if [[ "$version" -lt "${entry##*:}" ]]; then
            missing="${missing:+$missing, }${entry%%:*} (${entry##*:})"
        fi
    done
    [[ -n "$missing" ]] || return 0
    echo -e "${yellow}Note: systemd ${version} ignores part of the hardening in x-ui.service; the panel still starts.${plain}"
    echo "      Not applied, needs a newer systemd: ${missing}."
    if [[ "$version" -lt 231 ]]; then
        echo "      The panel's folders stay writable through ReadWriteDirectories=, the alias this script installs."
    fi
    echo "      The rest of the hardening is in force. Upgrade systemd to apply the above."
    return 0
}

# ProtectSystem=full makes /usr, /boot, /efi and /etc read-only. ProtectSystem=
# strict would make the whole hierarchy read-only (only the kernel API
# filesystems stay as they are), and that would break the panel's own use of
# /tmp. The panel's stores are configurable (XUI_DB_FOLDER, XUI_LOG_FOLDER,
# XUI_BIN_FOLDER), and XUI_MAIN_FOLDER is the folder install.sh/update.sh place
# the files in -- the unit's WorkingDirectory on a stock install, and what a
# relative XUI_BIN_FOLDER is resolved against. So a hard-coded list in the unit
# either misses a relocated store -- the panel then cannot write its own SQLite
# database and sits in a Restart=on-failure loop -- or forces the operator to
# edit a file that every install/update overwrites from the release tarball.
# install.sh and update.sh therefore regenerate the drop-in from the folders
# actually in use, and the unit's own ReadWritePaths only carry the
# plain-install defaults. A relocated store means re-running install or update:
# the drop-in is only written here.
_xui_service_write_paths_dropin() {
    # $1 is the env file to resolve the XUI_* folders from; callers pass nothing
    # and get the OS-specific path the unit itself uses.
    local env_file="${1:-}"
    local dropin_dir dropin temp_file
    local db_folder log_folder bin_folder main_folder
    local path line="" whitespace_paths="" seen_paths="" escaped_path

    if [[ -z "$env_file" ]]; then
        case "${release}" in
            ubuntu | debian | armbian)
                env_file="/etc/default/x-ui"
                ;;
            arch | manjaro | parch | alpine)
                env_file="/etc/conf.d/x-ui"
                ;;
            *)
                env_file="/etc/sysconfig/x-ui"
                ;;
        esac
    fi
    if [[ -r "$env_file" ]]; then
        set -a
        # shellcheck disable=SC1090
        source "$env_file"
        set +a
    fi

    # XUI_* wins over the script's own default: the unit hands that same env
    # file to the panel through EnvironmentFile=, so these are the folders it
    # will actually use.
    main_folder="${XUI_MAIN_FOLDER:-${xui_folder}}"
    db_folder="${XUI_DB_FOLDER:-/etc/x-ui}"
    log_folder="${XUI_LOG_FOLDER:-/var/log/x-ui}"
    # An empty XUI_BIN_FOLDER resolves to "bin" relative to the panel's working
    # directory, which the unit sets to the main folder.
    bin_folder="${XUI_BIN_FOLDER:-bin}"
    if [[ "$bin_folder" != /* ]]; then
        bin_folder="${main_folder%/}/${bin_folder#./}"
    fi

    for path in "$db_folder" "$log_folder" "$bin_folder" "$main_folder"; do
        [[ "$path" == /* ]] || continue
        # ReadWritePaths= is a whitespace-separated list, and a folder whose
        # name contains whitespace cannot be written into it without relying on
        # quoting. A wrong entry makes systemd reject the whole drop-in and the
        # panel would not start, so leave such a folder out and say so instead.
        if [[ "$path" != "${path//[[:space:]]/}" ]]; then
            whitespace_paths="${whitespace_paths:+$whitespace_paths }$path"
            continue
        fi
        case " $seen_paths " in
            *" $path "*) continue ;;
        esac
        seen_paths="${seen_paths}${seen_paths:+ }$path"
        # systemd expands %-specifiers in unit files, so a folder name carrying
        # a literal % has to be written as %%, or the entry stops naming the
        # folder systemd is meant to keep writable.
        escaped_path="${path//%/%%}"
        line="${line} -${escaped_path}"
    done
    if [[ -n "$whitespace_paths" ]]; then
        echo "Warning: these folders contain whitespace and were left out of" >&2
        echo "         10-xui-sandbox.conf: $whitespace_paths" >&2
        echo "         The panel cannot write to them under the unit's sandbox." >&2
    fi
    line="${line# }"
    [[ -n "$line" ]] || return 1

    dropin_dir="${xui_service}/x-ui.service.d"
    dropin="${dropin_dir}/10-xui-sandbox.conf"
    temp_file="${dropin}.tmp.$$"

    mkdir -p "$dropin_dir" || return 1
    cat > "$temp_file" << EOF
# Regenerated by install.sh/update.sh on every install and update: edits here
# are lost, and the list only reflects the XUI_* variables read from
# ${env_file} at that moment. Re-run install/update after moving a store.
# It lists the folders the panel writes to. Put local additions in their own
# drop-in, for example 20-x-ui-local.conf, which nothing here touches.
[Service]
ReadWritePaths=${line}
ReadWriteDirectories=${line}
EOF
    if [[ "$(_xui_systemd_major_version)" -ge 239 ]]; then
        cat >> "$temp_file" << 'EOF'
# @system-service needs systemd >= 239; on older versions the unknown group
# would leave the panel with a filter it cannot start under (x-ui.service.*).
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
EOF
    fi
    if [[ ! -s "$temp_file" ]]; then
        rm -f "$temp_file"
        return 1
    fi
    chmod 644 "$temp_file"
    mv -f "$temp_file" "$dropin" || { rm -f "$temp_file"; return 1; }
    if command -v systemctl > /dev/null 2>&1; then
        systemctl daemon-reload > /dev/null 2>&1 || true
    fi
    return 0
}

# Lands a systemd unit file at ${xui_service}/x-ui.service via a temp file +
# atomic mv, so a failed cp/curl or an interrupted mv never leaves a
# truncated unit file at the live path -- systemd would then fail to parse
# it on the next daemon-reload/start. Same pattern already used for
# /usr/bin/x-ui elsewhere in this script. source_is_url picks cp (from a
# file already extracted from the release tarball) vs curl (GitHub fallback).
_install_xui_service_unit() {
    local source="$1"
    local source_is_url="$2"
    local dest="${xui_service}/x-ui.service"
    local temp_file="${dest}.tmp.$$"

    rm -f "$temp_file"
    if [[ "$source_is_url" == "true" ]]; then
        curl -fLRo "$temp_file" "$source" > /dev/null 2>&1
    else
        cp -f "$source" "$temp_file" > /dev/null 2>&1
    fi
    if [[ $? -ne 0 ]]; then
        rm -f "$temp_file"
        return 1
    fi
    if [[ ! -s "$temp_file" ]]; then
        rm -f "$temp_file"
        return 1
    fi
    mv -f "$temp_file" "$dest"
    if [[ $? -ne 0 ]]; then
        rm -f "$temp_file"
        return 1
    fi
    if ! _xui_service_write_paths_dropin; then
        echo -e "${yellow}Warning: could not refresh ${xui_service}/x-ui.service.d/10-xui-sandbox.conf.${plain}"
        echo -e "${yellow}If XUI_DB_FOLDER or XUI_LOG_FOLDER points outside /etc/x-ui and /var/log/x-ui, the panel may not be able to write to it under ProtectSystem=full.${plain}"
    fi
    _xui_warn_unsupported_hardening
    return 0
}

# resolve_latest_tag prints the latest stable release tag. It prefers the web
# releases/latest redirect, which is not subject to the unauthenticated API's
# 60 req/h-per-IP limit that trips shared CI/CGNAT addresses (the install then
# fails with "Failed to fetch x-ui version"), and falls back to the API.
resolve_latest_tag() {
    local url tag
    url=$(curl -sSLI -o /dev/null -w '%{url_effective}' --retry 5 --retry-delay 3 --connect-timeout 15 --max-time 60 "https://github.com/MHSanaei/3x-ui/releases/latest" 2>/dev/null)
    tag=${url##*/tag/}
    if [[ "$tag" != "$url" && -n "$tag" && "$tag" != "latest" ]]; then
        echo "$tag"
        return 0
    fi
    curl -Ls --retry 5 --retry-delay 3 --connect-timeout 15 --max-time 60 "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
}

# Releases publish <asset>.sha256 next to each archive. A mismatch or a failed
# sidecar download aborts the install; only a 404 (releases predating the
# sidecar) is tolerated with a warning.
verify_release_checksum() {
    local url="$1" file="$2" sums="$2.sha256" code expected actual
    rm -f "${sums}"
    code=$(curl -sL --retry 3 --retry-delay 3 --connect-timeout 15 --max-time 60 -o "${sums}" -w '%{http_code}' "${url}.sha256")
    if [[ "${code}" == "404" ]]; then
        rm -f "${sums}"
        echo -e "${yellow}No checksum published for this release, skipping verification${plain}"
        return 0
    fi
    if [[ "${code}" != "200" ]]; then
        rm -f "${sums}" "${file}"
        echo -e "${red}Failed to download the checksum for $(basename "${file}") (HTTP ${code})${plain}"
        exit 1
    fi
    expected=$(awk 'NR == 1 {print $1}' "${sums}")
    actual=$(sha256sum "${file}" | awk '{print $1}')
    rm -f "${sums}"
    if [[ ! "${expected}" =~ ^[0-9a-f]{64}$ || "${expected}" != "${actual}" ]]; then
        rm -f "${file}"
        echo -e "${red}Checksum mismatch for $(basename "${file}"): expected ${expected:-<none>}, got ${actual}${plain}"
        exit 1
    fi
    echo -e "${green}Checksum verified: ${actual}${plain}"
}

# Older tags predate some of these files (x-ui.rc arrived in v2.8.4). Serving
# main's copy against an old binary is the mismatch this pinning exists to
# prevent, so probe before anything is stopped or removed and refuse the tag.
require_repo_files() {
    local ref="$1" name status
    shift
    [[ "${ref}" == "main" ]] && return 0
    for name in "$@"; do
        status=$(curl -sIL --retry 3 --connect-timeout 15 -o /dev/null -w '%{http_code}' "https://raw.githubusercontent.com/MHSanaei/3x-ui/${ref}/${name}")
        if [[ "${status}" != "200" ]]; then
            echo -e "${red}${name} is not available for ${ref} (HTTP ${status})${plain}"
            echo -e "${red}Install a release that ships it, or 'dev' for the rolling build. Your existing installation has not been touched.${plain}"
            exit 1
        fi
    done
}

install_x-ui() {
    cd ${xui_folder%/x-ui}/

    # Download resources
    if [ $# == 0 ]; then
        tag_version=$(resolve_latest_tag)
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${red}Failed to fetch x-ui version, it may be due to GitHub API restrictions, please try it later${plain}"
            exit 1
        fi
        echo -e "Got x-ui latest version: ${tag_version}, beginning the installation..."
        curl -fLR --retry 5 --retry-delay 3 --connect-timeout 15 --speed-limit 1 --speed-time 300 -o ${xui_folder}-linux-$(arch).tar.gz https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Downloading x-ui failed, please be sure that your server can access GitHub ${plain}"
            exit 1
        fi
        if [[ ! -s ${xui_folder}-linux-$(arch).tar.gz ]]; then
            rm ${xui_folder}-linux-$(arch).tar.gz -f
            echo -e "${red}Downloaded x-ui release archive is empty${plain}"
            exit 1
        fi
        verify_release_checksum "https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz" "${xui_folder}-linux-$(arch).tar.gz"
    else
        tag_version=$1
        # The rolling dev channel ships under a fixed, non-semver tag that is
        # force-moved to the latest main commit on every push. Accept `dev` as a
        # convenient alias and skip the numeric floor check for it.
        if [[ "$tag_version" == "dev" || "$tag_version" == "dev-latest" ]]; then
            tag_version="dev-latest"
            echo -e "${yellow}Installing the rolling dev build (tag: dev-latest). This is a per-commit pre-release, not a stable version.${plain}"
        else
            tag_version_numeric=${tag_version#v}
            min_version="2.3.5"

            if [[ "$(printf '%s\n' "$min_version" "$tag_version_numeric" | sort -V | head -n1)" != "$min_version" ]]; then
                echo -e "${red}Please use a newer version (at least v2.3.5). Exiting installation.${plain}"
                exit 1
            fi
        fi

        url="https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz"
        echo -e "Beginning to install x-ui ${tag_version}"
        curl -fLR --retry 5 --retry-delay 3 --connect-timeout 15 --speed-limit 1 --speed-time 300 -o ${xui_folder}-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Download x-ui ${tag_version} failed, please check if the version exists ${plain}"
            exit 1
        fi
        if [[ ! -s ${xui_folder}-linux-$(arch).tar.gz ]]; then
            rm ${xui_folder}-linux-$(arch).tar.gz -f
            echo -e "${red}Downloaded x-ui release archive is empty${plain}"
            exit 1
        fi
        verify_release_checksum "${url}" "${xui_folder}-linux-$(arch).tar.gz"
    fi
    # x-ui.sh, x-ui.rc and the unit files must come from the same release as
    # the binary; only the rolling dev build tracks main.
    local script_ref="${tag_version}"
    if [[ "${tag_version}" == "dev-latest" ]]; then
        script_ref="main"
    fi
    # The unit files are only fetched when the release tarball lacks them, so
    # they are checked at that point instead of here.
    local required_files=("x-ui.sh")
    [[ $release == "alpine" ]] && required_files+=("x-ui.rc")
    require_repo_files "${script_ref}" "${required_files[@]}"
    local xui_script_temp="/usr/bin/x-ui-temp.$$"
    rm -f "${xui_script_temp}"
    curl -fLRo "${xui_script_temp}" "https://raw.githubusercontent.com/MHSanaei/3x-ui/${script_ref}/x-ui.sh"
    if [[ $? -ne 0 ]]; then
        rm -f "${xui_script_temp}"
        echo -e "${red}Failed to download x-ui.sh${plain}"
        exit 1
    fi
    if [[ ! -s "${xui_script_temp}" ]]; then
        rm -f "${xui_script_temp}"
        echo -e "${red}Downloaded x-ui.sh is empty${plain}"
        exit 1
    fi

    # Stop x-ui service and remove old resources
    local custom_bin_backup=""
    if [[ -e ${xui_folder}/ ]]; then
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop
        else
            systemctl stop x-ui
        fi
        # Kill any leftover mtg (MTProto) sidecars. x-ui runs them outside its own
        # lifecycle, so on Linux a stale one can survive the stop and keep holding
        # an inbound port with an outdated secret, silently breaking new clients.
        # The freshly installed panel respawns a clean mtg per inbound on start.
        pkill -f 'mtg-linux-[^ ]* run ' > /dev/null 2>&1 || true
        pkill -f 'tuic-server.*-c .*bin/tuic/tuic_[0-9]+\.json' > /dev/null 2>&1 || true

        # bin/ is about to be wiped wholesale by the tar extraction below. The
        # release only ships known assets (xray/mtg binaries, the bundled
        # geoip*/geosite*.dat sets) -- anything else in bin/ was placed there
        # by the admin (e.g. a hand-added custom geoip/geosite file referenced
        # from a routing rule via "ext:<file>:<code>") and would otherwise be
        # silently deleted on every update, breaking Xray at next start with
        # "failed to open <file>: no such file or directory" for any routing
        # rule that references it. Moved aside rather than copied: a rename
        # on the same filesystem is atomic (no truncated file if disk space
        # runs out mid-copy, unlike `cp`) and keeps the snapshot under
        # /usr/local rather than a separate, possibly small/tmpfs $TMPDIR.
        if [[ -d "${xui_folder}/bin" ]]; then
            custom_bin_backup="${xui_folder%/x-ui}/x-ui-bin-backup.$$"
            rm -rf "${custom_bin_backup}"
            if ! mv "${xui_folder}/bin" "${custom_bin_backup}"; then
                custom_bin_backup=""
                echo -e "${yellow}Could not back up bin/ -- custom files there will not be preserved across this update${plain}"
            fi
        fi
        # Sole cleanup path for the backup from here on -- covers both the
        # two `exit 1`s below (extraction/binary-missing failures) and an
        # interrupted update (Ctrl-C, signal) before the restore runs.
        # Cleared once the restore below finishes normally.
        trap '[[ -n "${custom_bin_backup}" ]] && rm -rf "${custom_bin_backup}"' EXIT INT TERM
        rm ${xui_folder}/ -rf
    fi

    # Extract resources and set permissions
    tar zxvf x-ui-linux-$(arch).tar.gz
    if [[ $? -ne 0 ]]; then
        rm x-ui-linux-$(arch).tar.gz -f
        rm -f "${xui_script_temp}"
        echo -e "${red}Failed to extract the x-ui release archive -- the previous installation has already been removed, so the panel will not start until this is fixed; try running the installer again${plain}"
        exit 1
    fi
    rm x-ui-linux-$(arch).tar.gz -f

    cd x-ui
    if [[ $? -ne 0 || ! -s x-ui ]]; then
        rm -f "${xui_script_temp}"
        echo -e "${red}Extracted x-ui archive is missing the x-ui binary -- the previous installation has already been removed, so the panel will not start until this is fixed; try running the installer again${plain}"
        exit 1
    fi
    chmod +x x-ui
    chmod +x x-ui.sh

    # Check the system's architecture and rename the file accordingly.
    # The panel binary maps GOARCH=arm to "arm32" (internal/xray/process.go),
    # so the Xray binary must be named xray-linux-arm32; mtg keeps plain "arm".
    if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm32
        chmod +x bin/xray-linux-arm32
        if [[ -f bin/mtg-linux-$(arch) ]]; then
            mv bin/mtg-linux-$(arch) bin/mtg-linux-arm
            chmod +x bin/mtg-linux-arm
        fi
    fi
    chmod +x x-ui bin/xray-linux-$(arch)
    if [[ -f bin/mtg-linux-arm ]]; then
        chmod +x bin/mtg-linux-arm
    elif [[ -f bin/mtg-linux-$(arch) ]]; then
        chmod +x bin/mtg-linux-$(arch)
    fi

    # Restore anything from the old bin/ that the fresh release doesn't ship
    # (custom geoip/geosite files, or anything else an admin hand-placed
    # there) -- never overwrites a same-named file the new release provides,
    # so bundled assets (geoip.dat, geoip_RU.dat, ...) still get the fresh
    # per-release copy. Runs after the arch-rename above so xray-linux-arm32/
    # mtg-linux-arm already exist under their final names there and aren't
    # mistaken for custom files needing a restore. Skips paths the panel
    # itself regenerates at runtime (config.json, mtproto/*.toml -- see
    # internal/xray/process.go, internal/mtproto/manager.go): those aren't
    # admin-placed, and restoring a stale one only resurrects dead state (an
    # orphaned mtg config for a since-deleted inbound) or the wrong
    # directory permissions.
    if [[ -n "${custom_bin_backup}" ]]; then
        local restored_custom_bin=()
        while IFS= read -r -d '' f; do
            local rel="${f#"${custom_bin_backup}"/}"
            case "${rel}" in
                config.json | mtproto | mtproto/* | tuic | tuic/* | tuic-server | tuic-server-*) continue ;;
            esac
            if [[ ! -e "bin/${rel}" ]]; then
                mkdir -p "bin/$(dirname "${rel}")"
                cp -a "${f}" "bin/${rel}"
                restored_custom_bin+=("${rel}")
            fi
        done < <(find "${custom_bin_backup}" \( -type f -o -type l \) -print0)
        rm -rf "${custom_bin_backup}"
        custom_bin_backup=""
        if [[ ${#restored_custom_bin[@]} -gt 0 ]]; then
            echo -e "${green}Restored custom file(s) in bin/ not shipped by this release: ${restored_custom_bin[*]}${plain}"
        fi
    fi
    trap - EXIT INT TERM

    rm -f bin/tuic-server bin/tuic-server-* > /dev/null 2>&1 || true
    rm -rf bin/tuic > /dev/null 2>&1 || true

    # Update x-ui cli and se set permission
    mv -f "${xui_script_temp}" /usr/bin/x-ui
    if [[ $? -ne 0 ]]; then
        rm -f "${xui_script_temp}"
        echo -e "${red}Failed to install x-ui.sh${plain}"
        exit 1
    fi
    chmod +x /usr/bin/x-ui
    mkdir -p /var/log/x-ui
    config_after_install || return 1

    # Etckeeper compatibility
    if [ -d "/etc/.git" ]; then
        if [ -f "/etc/.gitignore" ]; then
            if ! grep -q "x-ui/x-ui.db" "/etc/.gitignore"; then
                echo "" >> "/etc/.gitignore"
                echo "x-ui/x-ui.db" >> "/etc/.gitignore"
                echo -e "${green}Added x-ui.db to /etc/.gitignore for etckeeper${plain}"
            fi
        else
            echo "x-ui/x-ui.db" > "/etc/.gitignore"
            echo -e "${green}Created /etc/.gitignore and added x-ui.db for etckeeper${plain}"
        fi
    fi

    if [[ $release == "alpine" ]]; then
        xui_rc_temp="/etc/init.d/x-ui.tmp.$$"
        rm -f "${xui_rc_temp}"
        curl -fLRo "${xui_rc_temp}" "https://raw.githubusercontent.com/MHSanaei/3x-ui/${script_ref}/x-ui.rc"
        if [[ $? -ne 0 ]]; then
            rm -f "${xui_rc_temp}"
            echo -e "${red}Failed to download x-ui.rc${plain}"
            exit 1
        fi
        if [[ ! -s "${xui_rc_temp}" ]]; then
            rm -f "${xui_rc_temp}"
            echo -e "${red}Downloaded x-ui.rc is empty${plain}"
            exit 1
        fi
        mv -f "${xui_rc_temp}" /etc/init.d/x-ui
        if [[ $? -ne 0 ]]; then
            rm -f "${xui_rc_temp}"
            echo -e "${red}Failed to install x-ui.rc${plain}"
            exit 1
        fi
        chmod +x /etc/init.d/x-ui
        rc-update add x-ui
        rc-service x-ui start
    else
        # Install systemd service file
        service_installed=false

        if [ -f "x-ui.service" ]; then
            echo -e "${green}Found x-ui.service in extracted files, installing...${plain}"
            if _install_xui_service_unit "x-ui.service" "false"; then
                service_installed=true
            fi
        fi

        if [ "$service_installed" = false ]; then
            case "${release}" in
                ubuntu | debian | armbian)
                    if [ -f "x-ui.service.debian" ]; then
                        echo -e "${green}Found x-ui.service.debian in extracted files, installing...${plain}"
                        if _install_xui_service_unit "x-ui.service.debian" "false"; then
                            service_installed=true
                        fi
                    fi
                    ;;
                arch | manjaro | parch)
                    if [ -f "x-ui.service.arch" ]; then
                        echo -e "${green}Found x-ui.service.arch in extracted files, installing...${plain}"
                        if _install_xui_service_unit "x-ui.service.arch" "false"; then
                            service_installed=true
                        fi
                    fi
                    ;;
                *)
                    if [ -f "x-ui.service.rhel" ]; then
                        echo -e "${green}Found x-ui.service.rhel in extracted files, installing...${plain}"
                        if _install_xui_service_unit "x-ui.service.rhel" "false"; then
                            service_installed=true
                        fi
                    fi
                    ;;
            esac
        fi

        # If service file not found in tar.gz, download from GitHub
        if [ "$service_installed" = false ]; then
            echo -e "${yellow}Service files not found in tar.gz, downloading from GitHub...${plain}"
            case "${release}" in
                ubuntu | debian | armbian)
                    service_unit_url="https://raw.githubusercontent.com/MHSanaei/3x-ui/${script_ref}/x-ui.service.debian"
                    ;;
                arch | manjaro | parch)
                    service_unit_url="https://raw.githubusercontent.com/MHSanaei/3x-ui/${script_ref}/x-ui.service.arch"
                    ;;
                *)
                    service_unit_url="https://raw.githubusercontent.com/MHSanaei/3x-ui/${script_ref}/x-ui.service.rhel"
                    ;;
            esac

            if ! _install_xui_service_unit "$service_unit_url" "true"; then
                echo -e "${red}Failed to install x-ui.service from GitHub (${script_ref}) -- the release tarball did not ship one either${plain}"
                exit 1
            fi
            service_installed=true
        fi

        if [ "$service_installed" = true ]; then
            echo -e "${green}Setting up systemd unit...${plain}"
            chown root:root ${xui_service}/x-ui.service > /dev/null 2>&1
            chmod 644 ${xui_service}/x-ui.service > /dev/null 2>&1
            systemctl daemon-reload
            systemctl enable x-ui
            systemctl start x-ui
        else
            echo -e "${red}Failed to install x-ui.service file${plain}"
            exit 1
        fi
    fi

    # IP Limit relies on fail2ban; install + configure it now so the feature
    # works out of the box (no-op when XUI_ENABLE_FAIL2BAN=false). Never fatal.
    setup_fail2ban

    echo -e "${green}x-ui ${tag_version}${plain} installation finished, it is running now..."
    echo -e ""
    echo -e "┌───────────────────────────────────────────────────────┐
│  ${blue}x-ui control menu usages (subcommands):${plain}              │
│                                                       │
│  ${blue}x-ui${plain}              - Admin Management Script          │
│  ${blue}x-ui start${plain}        - Start                            │
│  ${blue}x-ui stop${plain}         - Stop                             │
│  ${blue}x-ui restart${plain}      - Restart                          │
│  ${blue}x-ui status${plain}       - Current Status                   │
│  ${blue}x-ui settings${plain}     - Current Settings                 │
│  ${blue}x-ui enable${plain}       - Enable Autostart on OS Startup   │
│  ${blue}x-ui disable${plain}      - Disable Autostart on OS Startup  │
│  ${blue}x-ui log${plain}          - Check logs                       │
│  ${blue}x-ui banlog${plain}       - Check Fail2ban ban logs          │
│  ${blue}x-ui update${plain}       - Update                           │
│  ${blue}x-ui legacy${plain}       - Legacy version                   │
│  ${blue}x-ui install${plain}      - Install                          │
│  ${blue}x-ui uninstall${plain}    - Uninstall                        │
└───────────────────────────────────────────────────────┘"
}

echo -e "${green}Running...${plain}"
install_base
install_x-ui $1
