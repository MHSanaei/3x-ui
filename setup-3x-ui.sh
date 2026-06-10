#!/bin/bash
#===============================================================================
#  3X-UI Auto Installer — Ubuntu 22.04
#  https://github.com/ndotvpn/3x-ui
#
#  Usage:
#    bash <(curl -s https://raw.githubusercontent.com/ndotvpn/3x-ui/master/install.sh)
#
#  What it does:
#    1. Install 3X-UI (PostgreSQL, localhost-only, port 12797)
#    2. Generate self-signed SSL certificate
#    3. Configure Nginx reverse proxy (panel on port 80)
#    4. Harden server (UFW, fail2ban, sysctl, SSH)
#    5. Setup cron maintenance jobs
#    6. Save credentials to /root/3x-ui-credentials.txt
#===============================================================================

set -u

# -- Config -----------------------------------------------------------------
PANEL_PORT=12797
NGINX_SSL_PORT=8443

SERVER_IP=$(curl -s https://api4.ipify.org 2>/dev/null \
         || curl -s https://ipv4.icanhazip.com 2>/dev/null \
         || curl -s https://checkip.amazonaws.com 2>/dev/null \
         || echo "")

XUI_DIR="/usr/local/x-ui"
XUI_BIN="${XUI_DIR}/x-ui"
XUI_SERVICE="x-ui.service"
LOG_FILE="/var/log/3x-ui-setup.log"
CREDS_FILE="/root/3x-ui-credentials.txt"
SSL_DIR="/etc/nginx/ssl"
SSL_CERT="${SSL_DIR}/xui-self-signed.crt"
SSL_KEY="${SSL_DIR}/xui-self-signed.key"
NGINX_CONF="/etc/nginx/sites-available/3x-ui"
XUI_INSTALL_URL="https://raw.githubusercontent.com/ndotvpn/3x-ui/master/install.sh"
ADMIN_USER=""
ADMIN_PASS=""
XUI_WEB_BASE_PATH=""
XUI_API_TOKEN=""
REDIRECT_DOMAIN=""
PANEL_IP_WHITELIST=""

# -- Colors -----------------------------------------------------------------
setup_colors() {
    if [[ -t 1 ]]; then
        RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'
        BLUE='\033[0;34m'; CYAN='\033[0;36m'; MAG='\033[0;35m'; BOLD='\033[1m'
        DIM='\033[2m'; NC='\033[0m'
    else
        RED=''; GREEN=''; YELLOW=''; BLUE=''; CYAN=''; MAG=''; BOLD=''; DIM=''; NC=''
    fi
}
setup_colors

# -- UI Helpers -------------------------------------------------------------
log()   { echo -e " ${GREEN}◆${NC} ${DIM}$(date '+%H:%M:%S')${NC}  $*"; }
ok()    { echo -e " ${GREEN}✓${NC}  $*"; }
warn()  { echo -e " ${YELLOW}⚠${NC}  ${YELLOW}$*${NC}"; }
error() { echo -e " ${RED}✗${NC}  ${RED}$*${NC}"; }
fail()  { error "$*"; exit 1; }

hr()    { echo -e " ${DIM}────────────────────────────────────────────────────────${NC}"; }
space() { echo; }

section() {
    local n=$1; shift
    echo
    echo -e " ${CYAN}┌─[${NC}${BOLD} Step ${n}${NC}${CYAN}]──────────────────────────────────────────┐${NC}"
    echo -e " ${CYAN}│${NC}  ${BOLD}$*${NC}"
    echo -e " ${CYAN}└────────────────────────────────────────────────${NC}"
    echo
}

spinner() {
    local pid=$1; local msg=$2
    local spin='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    local i=0
    while kill -0 "$pid" 2>/dev/null; do
        printf "\r ${CYAN}%s${NC}  %s" "${spin:$i:1}" "$msg"
        i=$(( (i+1) % ${#spin} ))
        sleep 0.1
    done
    printf "\r ${GREEN}✓${NC}  %s\n" "$msg"
}

run_spinner() {
    local msg=$1; shift
    ("$@" > /dev/null 2>&1) &
    local pid=$!
    spinner "$pid" "$msg"
    wait "$pid"
    return $?
}

banner() {
    echo
    echo -e " ${CYAN}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e " ${CYAN}║${NC}                                                       ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}   ${BOLD}██╗  ██╗███████╗██╗   ██╗██╗${NC}                      ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}   ${BOLD}╚██╗██╔╝██╔════╝██║   ██║██║${NC}                      ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}   ${BOLD} ╚███╔╝ █████╗  ██║   ██║██║${NC}                      ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}   ${BOLD} ██╔██╗ ██╔══╝  ██║   ██║██║${NC}                      ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}   ${BOLD}██╔╝ ██╗███████╗╚██████╔╝██║${NC}                      ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}   ${BOLD}╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚═╝${NC}                      ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}                                                       ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}  ${DIM}Auto Installer — Ubuntu 22.04${NC}                    ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}  ${DIM}Server:${NC} ${SERVER_IP:-detecting...}                          ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}  ${DIM}Date:${NC}   $(date '+%Y-%m-%d %H:%M')                              ${CYAN}║${NC}"
    echo -e " ${CYAN}║${NC}                                                       ${CYAN}║${NC}"
    echo -e " ${CYAN}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo
}

# -- Interactive Prompts ----------------------------------------------------
prompts() {
    echo -e " ${CYAN}┌─[ Setup Configuration ]──────────────────────────────────┐${NC}"

    read -r -p "$(echo -e " ${CYAN}│${NC}  Reality redirect domain ${DIM}(SNI, e.g. www.deepl.com)${NC}: ")" REDIRECT_DOMAIN
    REDIRECT_DOMAIN="${REDIRECT_DOMAIN:-www.deepl.com}"
    log "Reality SNI: ${BOLD}${REDIRECT_DOMAIN}${NC}"

    read -r -p "$(echo -e " ${CYAN}│${NC}  Panel IP whitelist ${DIM}(optional, e.g. 1.2.3.4)${NC}: ")" PANEL_IP_WHITELIST
    if [[ -n "$PANEL_IP_WHITELIST" ]]; then
        log "Panel locked to IP: ${BOLD}${PANEL_IP_WHITELIST}${NC}"
    else
        log "Panel accessible from all IPs"
    fi
    echo -e " ${CYAN}└────────────────────────────────────────────────────────${NC}"
    echo
}

# -- Steps ------------------------------------------------------------------

check_root() {
    [[ $EUID -ne 0 ]] && fail "This script must be run as root."
}

check_os() {
    [[ ! -f /etc/os-release ]] && fail "Cannot detect OS."
    source /etc/os-release
    [[ "$ID" != "ubuntu" ]] && fail "This script is designed for Ubuntu. Detected: $ID"
    log "Detected ${BOLD}$NAME $VERSION_ID${NC}"
}

prepare_system() {
    section 1 "Preparing System"
    export DEBIAN_FRONTEND=noninteractive
    log "Updating package lists..."
    apt-get update -qq || true
    log "Installing required packages..."
    apt-get install -y -qq \
        curl wget unzip tar nginx ufw fail2ban cron \
        openssl ca-certificates python3 jq sudo 2>&1 | tail -1
    ok "System packages ready"
}

install_xui() {
    section 2 "Installing 3X-UI Panel"

    if systemctl is-active --quiet "$XUI_SERVICE" 2>/dev/null; then
        warn "3X-UI already installed — skipping"
        return 0
    fi

    [[ -d "$XUI_DIR" ]] && rm -rf "$XUI_DIR"

    log "Downloading installer..."
    local tmp="/tmp/xui-install.sh"
    curl -4fsSL "$XUI_INSTALL_URL" -o "$tmp" || fail "Download failed"
    chmod +x "$tmp"

    log "Launching installer (PostgreSQL, port ${PANEL_PORT}, localhost only)..."
    echo -e "2\n1\ny\n${PANEL_PORT}\n4\ny\n" | bash "$tmp" 2>&1
    local exit_code=${PIPESTATUS[1]}
    [[ $exit_code -ne 0 ]] && fail "3X-UI installer exited with code $exit_code"

    sleep 3
    if systemctl is-active --quiet "$XUI_SERVICE" 2>/dev/null; then
        ok "3X-UI is running"
    else
        systemctl restart "$XUI_SERVICE" 2>/dev/null || true
        sleep 2
    fi
}

extract_settings() {
    section 3 "Reading Panel Configuration"

    local settings=""
    for i in $(seq 1 10); do
        settings=$("${XUI_BIN}" setting -show true 2>/dev/null) && break
        sleep 2
    done
    [[ -z "$settings" ]] && fail "Could not read panel settings"

    local p
    p=$(echo "$settings" | grep -Eo 'port: .+' | awk '{print $2}' | tr -d ' ')
    [[ -n "$p" ]] && PANEL_PORT="$p"
    log "Panel port: ${BOLD}${PANEL_PORT}${NC}"

    XUI_WEB_BASE_PATH=$(echo "$settings" | grep -Eo 'webBasePath: .+' | awk '{print $2}' | tr -d ' ')
    : "${XUI_WEB_BASE_PATH:=/}"
    log "Web base path: ${BOLD}${XUI_WEB_BASE_PATH}${NC}"

    ADMIN_USER="admin"; ADMIN_PASS="admin"
    "${XUI_BIN}" setting -username "$ADMIN_USER" -password "$ADMIN_PASS" 2>/dev/null && \
        ok "Panel credentials: ${BOLD}${ADMIN_USER}${NC} / ${BOLD}${ADMIN_PASS}${NC}"

    XUI_API_TOKEN=$("${XUI_BIN}" setting -getApiToken true 2>/dev/null | grep -Eo 'apiToken: .+' | awk '{print $2}' || true)
    [[ -n "$XUI_API_TOKEN" ]] && log "API token retrieved"

    systemctl restart "$XUI_SERVICE" 2>/dev/null || true
    sleep 2
}

create_ssl_cert() {
    section 4 "Self-Signed SSL Certificate"

    mkdir -p "$SSL_DIR"
    [[ -f "$SSL_CERT" && -f "$SSL_KEY" ]] && { warn "Certificate exists — skipping"; return 0; }

    log "Generating 4096-bit RSA certificate..."
    openssl req -x509 -nodes -days 3650 -newkey rsa:4096 \
        -keyout "$SSL_KEY" -out "$SSL_CERT" \
        -subj "/CN=${SERVER_IP:-localhost}/O=3X-UI" \
        -addext "subjectAltName=IP:${SERVER_IP:-127.0.0.1},DNS:localhost" 2>&1
    chmod 600 "$SSL_KEY"; chmod 644 "$SSL_CERT"
    ok "Certificate created (${SSL_CERT})"
}

configure_nginx() {
    section 5 "Configuring Nginx"

    rm -f /etc/nginx/sites-enabled/default
    local bp="${XUI_WEB_BASE_PATH%/}"

    log "Installing nginx-extras for header spoofing..."
    apt-get install -y -qq nginx-extras 2>&1 | tail -1

    local whitelist=""
    if [[ -n "$PANEL_IP_WHITELIST" ]]; then
        whitelist="        allow ${PANEL_IP_WHITELIST};
        deny all;"
    fi

    cat > "$NGINX_CONF" << NGINXEOF
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name _;
    server_tokens off;
    more_set_headers "Server: Apache";

    location ${bp}/ {
${whitelist}
        proxy_pass http://127.0.0.1:${PANEL_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Forwarded-Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }

    location / { return 404; }
}
NGINXEOF

    cat > /etc/nginx/sites-available/3x-ui-decoy << DECOYEOF
server {
    listen 8443 ssl;
    listen [::]:8443 ssl;
    server_name ${REDIRECT_DOMAIN};
    server_tokens off;
    more_set_headers "Server: Apache";

    ssl_certificate ${SSL_CERT};
    ssl_certificate_key ${SSL_KEY};

    location / {
        proxy_pass https://${REDIRECT_DOMAIN};
        proxy_ssl_server_name on;
        proxy_ssl_name ${REDIRECT_DOMAIN};
        proxy_set_header Host ${REDIRECT_DOMAIN};
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
DECOYEOF

    ln -sf "$NGINX_CONF" /etc/nginx/sites-enabled/
    ln -sf /etc/nginx/sites-available/3x-ui-decoy /etc/nginx/sites-enabled/
    nginx -t 2>&1 || fail "Nginx config test failed"
    systemctl restart nginx && systemctl enable nginx > /dev/null 2>&1

    log "Port 80   → panel at ${BOLD}${bp}/${NC}"
    log "Port 80   → root  → ${DIM}404 (stealth)${NC}"
    log "Port 8443 → decoy → ${BOLD}${REDIRECT_DOMAIN}${NC} ${DIM}(TLS, self-signed)${NC}"
}

configure_ufw() {
    section 6 "Configuring UFW Firewall"

    ufw --force reset > /dev/null 2>&1
    ufw default deny incoming > /dev/null
    ufw default allow outgoing > /dev/null
    ufw allow ssh > /dev/null
    ufw allow 80/tcp > /dev/null
    ufw allow 443/tcp > /dev/null
    ufw allow 8443/tcp > /dev/null
    ufw --force enable > /dev/null 2>&1

    log "Port ${BOLD}22${NC}   → SSH"
    log "Port ${BOLD}80${NC}   → HTTP (panel)"
    log "Port ${BOLD}443${NC}  → Reality/VLESS"
    log "Port ${BOLD}8443${NC} → TLS decoy → ${REDIRECT_DOMAIN}"
    ok "Firewall active"
}

configure_fail2ban() {
    section 7 "Configuring fail2ban"

    local jl="/etc/fail2ban/jail.local"
    if [[ ! -f "$jl" ]]; then
        cat > "$jl" << 'EOF'
[DEFAULT]
bantime = 3600
findtime = 600
maxretry = 5
ignoreip = 127.0.0.1/8 ::1

[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
bantime = 86400
EOF
    fi
    systemctl enable -q fail2ban 2>/dev/null
    systemctl restart fail2ban
    log "SSH: ${BOLD}3 retries → 24h ban${NC}"
}

harden_sysctl() {
    section 8 "Hardening sysctl"

    cat > /etc/sysctl.d/99-hardening.conf << 'EOF'
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv6.conf.all.accept_redirects = 0
net.ipv6.conf.default.accept_redirects = 0
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0
net.ipv6.conf.all.accept_source_route = 0
net.ipv6.conf.default.accept_source_route = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.send_redirects = 0
net.ipv4.conf.all.secure_redirects = 0
net.ipv4.conf.default.secure_redirects = 0
net.ipv4.tcp_syncookies = 1
net.ipv4.tcp_fastopen = 3
net.core.somaxconn = 65536
net.ipv4.tcp_max_syn_backlog = 65536
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
kernel.kptr_restrict = 2
kernel.dmesg_restrict = 1
kernel.printk = 3 3 3 3
kernel.unprivileged_bpf_disabled = 1
net.core.bpf_jit_harden = 2
net.ipv6.conf.all.disable_ipv6 = 0
net.ipv6.conf.default.disable_ipv6 = 0
EOF
    sysctl -p /etc/sysctl.d/99-hardening.conf > /dev/null 2>&1 || true
    log "BBR congestion control, anti-spoof, kernel hardening"
}

harden_ssh() {
    section 9 "Hardening SSH"

    local cfg="/etc/ssh/sshd_config"
    local bak="${cfg}.bak.$(date +%s)"
    [[ ! -f "$cfg" ]] && fail "sshd_config not found"
    cp "$cfg" "$bak"
    log "Backup: ${DIM}${bak}${NC}"

    mkdir -p /root/.ssh; touch /root/.ssh/authorized_keys

    for s in "PermitRootLogin prohibit-password" \
             "PasswordAuthentication no" \
             "PermitEmptyPasswords no" \
             "ChallengeResponseAuthentication no" \
             "UsePAM yes" \
             "X11Forwarding no" \
             "PrintMotd no" \
             "ClientAliveInterval 300" \
             "ClientAliveCountMax 2" \
             "MaxAuthTries 3" \
             "MaxSessions 10" \
             "Protocol 2"; do
        local key="${s%% *}" val="${s#* }"
        if grep -qs "^\s*${key}\s" "$cfg"; then
            sed -i "s/^\s*${key}\s.*/${key} ${val}/" "$cfg" 2>/dev/null || true
        elif grep -qs "^#\s*${key}\s" "$cfg"; then
            sed -i "s/^#\s*${key}\s.*/${key} ${val}/" "$cfg" 2>/dev/null || true
        else
            echo "${key} ${val}" >> "$cfg"
        fi
    done

    if sshd -t 2>&1; then
        systemctl restart sshd 2>/dev/null || true
        log "Root login: ${BOLD}key-only${NC}"
        log "Password auth: ${BOLD}disabled${NC}"
    else
        warn "SSH config test failed — restoring backup"
        cp "$bak" "$cfg"
        systemctl restart sshd 2>/dev/null || true
    fi

    warn "Open a ${BOLD}new terminal${NC} to verify SSH still works before closing this session"
}

fix_ptr() {
    section 11 "PTR Record"

    local ptr=""
    ptr=$(dig +short -x "${SERVER_IP}" 2>/dev/null) || ptr=$(host "${SERVER_IP}" 2>/dev/null | grep -oP 'domain name pointer \K\S+' || echo "")

    if [[ -z "$ptr" ]]; then
        log "No PTR record found — may not be set yet"
    else
        log "Current PTR: ${BOLD}${ptr}${NC}"
        if echo "$ptr" | grep -qiE "(onecom|cloud\.one|contabo|hetzner|digitalocean|vultr|linode|ovh|aws|googlecloud|azure)"; then
            log "Provider detected from PTR: ${BOLD}$(echo "$ptr" | grep -oE '^[^.]+')${NC}"
        fi
    fi

    echo
    echo -e " ${DIM}  To set a custom PTR record, use your provider's control panel or API:${NC}"
    echo -e " ${DIM}  ─────────────────────────────────────────────────────────────${NC}"
    echo -e " ${DIM}  1. Log into your VPS provider dashboard${NC}"
    echo -e " ${DIM}  2. Find the network/reverse DNS settings for ${SERVER_IP}${NC}"
    echo -e " ${DIM}  3. Set PTR to something generic (e.g. mail.yourdomain.com)${NC}"
    echo -e " ${DIM}  4. Apply — changes propagate within a few hours${NC}"
    echo
    log "PTR hardening printed above"
}

setup_cron() {
    section 10 "Cron Jobs"

    local f="/etc/cron.d/3x-ui-maintenance"
    cat > "$f" << 'EOF'
0 3 1 * * root bash -c 'chmod 600 /etc/nginx/ssl/xui-self-signed.key; chmod 644 /etc/nginx/ssl/xui-self-signed.crt; systemctl reload nginx 2>/dev/null || true'
0 4 * * 0 root systemctl restart x-ui 2>/dev/null || true
*/5 * * * * root bash -c 'systemctl is-active --quiet x-ui || systemctl restart x-ui'
EOF
    chmod 644 "$f"
    log "Weekly x-ui restart  ${DIM}(Sundays 04:00)${NC}"
    log "Health check         ${DIM}(every 5 min)${NC}"
}

save_credentials() {
    section 12 "Saving Credentials"

    local url="http://${SERVER_IP}:80${XUI_WEB_BASE_PATH}"
    local decoy_url="https://${SERVER_IP}:8443"

    {
        echo "╔══════════════════════════════════════════════╗"
        echo "║       3X-UI — Setup Complete                ║"
        echo "║       $(date)         ║"
        echo "╚══════════════════════════════════════════════╝"
        echo ""
        echo "  SERVER"
        echo "    IP       ${SERVER_IP}"
        echo "    Hostname $(hostname)"
        echo "    OS       $(source /etc/os-release && echo "$NAME $VERSION_ID")"
        echo ""
        echo "  PANEL"
        echo "    URL         ${url}"
        echo "    Username    ${ADMIN_USER}"
        echo "    Password    ${ADMIN_PASS}"
        echo "    WebBasePath ${XUI_WEB_BASE_PATH}"
        echo "    API Token   ${XUI_API_TOKEN:-N/A}"
        echo ""
        echo "  REALITY"
        echo "    Redirect/SNI ${REDIRECT_DOMAIN}"
        echo "    Use this as the SNI when creating a Reality inbound"
        echo "    in the panel. Port: 443."
        echo ""
        echo "  PORTS"
        echo "    22/tcp  SSH (key only)"
        echo "    80/tcp  Panel reverse proxy (root → 404)"
        echo "    443/tcp VLESS+Reality (configure in panel)"
        echo "    8443/tcp TLS decoy → ${REDIRECT_DOMAIN} (self-signed)"
        echo ""
        echo "  NGINX"
        echo "    Server header spoofed to: Apache"
        local ipinfo="all IPs"
        [[ -n "$PANEL_IP_WHITELIST" ]] && ipinfo="${PANEL_IP_WHITELIST} only"
        echo "    Panel access:    ${ipinfo}"
        echo ""
        echo "  SECURITY"
        echo "    UFW     Enabled (22,80,443,8443)"
        echo "    fail2ban SSH (3→24h)"
        echo "    SSH     Password disabled, root key-only"
        echo ""
        echo "  FILES"
        echo "    Credentials  ${CREDS_FILE}"
        echo "    Setup log    ${LOG_FILE}"
        echo ""
        echo "  NEXT STEPS"
        echo "    1. Set a custom PTR record via your provider's panel"
        echo "    2. Create a VLESS+Reality inbound in the 3X-UI panel"
        echo "       (Settings → Inbounds → Add Inbound)"
        echo "    3. Set SNI to ${REDIRECT_DOMAIN} in the Reality config"
        echo ""
    } > "$CREDS_FILE"
    chmod 600 "$CREDS_FILE"
    ok "Credentials saved to ${DIM}${CREDS_FILE}${NC}"

    echo
    echo -e " ${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e " ${GREEN}║${NC}              ${BOLD}SETUP COMPLETE${NC}                         ${GREEN}║${NC}"
    echo -e " ${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo
    echo -e " ${BOLD}Panel:${NC}  ${CYAN}${url}${NC}"
    echo -e " ${BOLD}Login:${NC}  ${ADMIN_USER} / ${ADMIN_PASS}"
    echo -e " ${BOLD}Reality:${NC} SNI → ${BOLD}${REDIRECT_DOMAIN}${NC} ${DIM}(set this in the panel inbound)${NC}"
    echo -e " ${BOLD}Decoy:${NC}  ${CYAN}${decoy_url}${NC} ${DIM}(looks like ${REDIRECT_DOMAIN})${NC}"
    [[ -n "$PANEL_IP_WHITELIST" ]] && echo -e " ${BOLD}Access:${NC} Panel locked to ${PANEL_IP_WHITELIST}"
    echo
    echo -e " ${CYAN}Credentials:${NC} ${CREDS_FILE}"
    echo -e " ${CYAN}Log:${NC}        ${LOG_FILE}"
    echo
    echo -e " ${YELLOW}⚠ Verify SSH key login in a NEW terminal before closing!${NC}"
    echo
}

# -- Clean Up ---------------------------------------------------------------
cleanup() {
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        echo
        error "Setup failed (exit ${rc})"
        error "Log: ${LOG_FILE}"
    fi
    rm -f /tmp/xui-install.sh 2>/dev/null || true
}
trap cleanup EXIT

# -- Main -------------------------------------------------------------------
main() {
    banner
    check_root
    check_os
    prompts
    prepare_system
    install_xui
    extract_settings
    create_ssl_cert
    configure_nginx
    configure_ufw
    configure_fail2ban
    harden_sysctl
    harden_ssh
    setup_cron
    fix_ptr
    save_credentials
    echo -e " ${GREEN}All done.${NC}"
}

main "$@"
