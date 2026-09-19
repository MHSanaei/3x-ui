#!/usr/bin/env bash
set -euo pipefail

CONFIG="/etc/x-ui/telemt.toml"
CHAIN="TELEMT_MEKO"
MARK="0x400"

log() { printf '%s\n' "[telemt-meko-fix] $*"; }

get_port() {
    awk '
        /^\[server\][[:space:]]*$/ { in_server=1; next }
        /^\[/ { in_server=0 }
        in_server && /^[[:space:]]*port[[:space:]]*=/ {
            gsub(/[[:space:]]/, "", $0)
            sub(/^port=/, "", $0)
            print $0
            exit
        }
    ' "$CONFIG"
}

ensure_u32() {
    if ! iptables -m u32 --help >/dev/null 2>&1; then
        modprobe xt_u32 >/dev/null 2>&1 || true
    fi
    iptables -m u32 --help >/dev/null 2>&1
}

apply() {
    [[ -r "$CONFIG" ]] || { log "Telemt config not found: $CONFIG"; exit 1; }
    command -v iptables >/dev/null 2>&1 || { log "iptables is required"; exit 1; }
    ensure_u32 || { log "xt_u32 is not available; MEKO V3 cannot be enabled"; exit 1; }

    local port
    port="$(get_port)"
    [[ "$port" =~ ^[0-9]+$ ]] || { log "Cannot read [server].port from $CONFIG"; exit 1; }
    (( port >= 1 && port <= 65535 )) || { log "Invalid Telemt port: $port"; exit 1; }

    iptables -t filter -N "$CHAIN" 2>/dev/null || true
    iptables -t filter -F "$CHAIN"

    # Remove the previous MEKO mark rule(s) before recreating it.
    while iptables -t mangle -C PREROUTING -p tcp --dport "$port" -m u32 --u32 \
        "32 & 0x000FFFFF = 0x0002FFFF && 40 & 0xFF000000 = 0x02000000 && 44 & 0xFFFF0000 = 0x01030000 && 48 & 0xFFFFFF00 = 0x01010800 && 60 & 0xFFFFFFFF = 0x04020000" \
        -j MARK --set-mark "$MARK" 2>/dev/null; do
        iptables -t mangle -D PREROUTING -p tcp --dport "$port" -m u32 --u32 \
            "32 & 0x000FFFFF = 0x0002FFFF && 40 & 0xFF000000 = 0x02000000 && 44 & 0xFFFF0000 = 0x01030000 && 48 & 0xFFFFFF00 = 0x01010800 && 60 & 0xFFFFFFFF = 0x04020000" \
            -j MARK --set-mark "$MARK"
    done

    iptables -t mangle -A PREROUTING -p tcp --dport "$port" -m u32 --u32 \
        "32 & 0x000FFFFF = 0x0002FFFF && 40 & 0xFF000000 = 0x02000000 && 44 & 0xFFFF0000 = 0x01030000 && 48 & 0xFFFFFF00 = 0x01010800 && 60 & 0xFFFFFFFF = 0x04020000" \
        -j MARK --set-mark "$MARK"

    # MEKO V3: iOS fingerprint gets no SYN limit; other clients get 54/min/IP,
    # burst 1, then an immediate TCP reset. This is the V3 u32 fingerprint
    # published by MTPROTO_FIX_By_MEKO.
    iptables -t filter -A "$CHAIN" -p tcp --dport "$port" --syn -m mark --mark "$MARK" -j ACCEPT
    iptables -t filter -A "$CHAIN" -p tcp --dport "$port" --syn \
        -m hashlimit --hashlimit-name "telemt_meko_$port" --hashlimit-mode srcip \
        --hashlimit-upto 54/minute --hashlimit-burst 1 \
        --hashlimit-htable-expire 60000 --hashlimit-htable-size 32768 -j ACCEPT
    iptables -t filter -A "$CHAIN" -p tcp --dport "$port" --syn -j REJECT --reject-with tcp-reset

    iptables -t filter -C INPUT -j "$CHAIN" 2>/dev/null || iptables -t filter -I INPUT 2 -j "$CHAIN"
    log "MEKO V3 applied to Telemt port $port"
}

remove() {
    if command -v iptables >/dev/null 2>&1; then
        if iptables -t filter -C INPUT -j "$CHAIN" 2>/dev/null; then
            iptables -t filter -D INPUT -j "$CHAIN"
        fi
        iptables -t filter -F "$CHAIN" 2>/dev/null || true
        iptables -t filter -X "$CHAIN" 2>/dev/null || true

        if [[ -r "$CONFIG" ]]; then
            local port
            port="$(get_port || true)"
            if [[ "$port" =~ ^[0-9]+$ ]]; then
                local rule='32 & 0x000FFFFF = 0x0002FFFF && 40 & 0xFF000000 = 0x02000000 && 44 & 0xFFFF0000 = 0x01030000 && 48 & 0xFFFFFF00 = 0x01010800 && 60 & 0xFFFFFFFF = 0x04020000'
                while iptables -t mangle -C PREROUTING -p tcp --dport "$port" -m u32 --u32 "$rule" -j MARK --set-mark "$MARK" 2>/dev/null; do
                    iptables -t mangle -D PREROUTING -p tcp --dport "$port" -m u32 --u32 "$rule" -j MARK --set-mark "$MARK"
                done
            fi
        fi
    fi
}

case "${1:-apply}" in
    apply) apply ;;
    remove) remove ;;
    *) echo "Usage: $0 {apply|remove}" >&2; exit 2 ;;
esac
