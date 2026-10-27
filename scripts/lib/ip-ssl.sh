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
