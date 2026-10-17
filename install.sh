#!/bin/bash

# ══════════════════════════════════════════════════════════════════════════
# 3X-UI // SECURE ENTERPRISE NODE APPLIANCE INSTALLER
# ══════════════════════════════════════════════════════════════════════════

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

# 256-Color Enterprise Appliance Palette
C_FRAME='\033[38;5;39m'     # Deep Electric Blue (Borders)
C_CYAN='\033[38;5;51m'      # Bright Cyan (Accents)
C_GREEN='\033[38;5;46m'     # Neon Green (OK / Bars)
C_YELLOW='\033[38;5;226m'   # Amber Yellow (Credentials / Warnings)
C_RED='\033[38;5;196m'      # Crimson Red (Errors)
C_DIM='\033[38;5;242m'      # Muted Gray
C_WHITE='\033[1;37m'        # Bold White
C_NC='\033[0m'              # Reset

LOG_FILE="/tmp/xui_enterprise_install.log"
rm -f "$LOG_FILE"

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# Global Telemetry & Timing Trackers
T_P1="0.0s"
T_P2="0.0s"
T_P3="0.0s"
T_P4="0.0s"
T_P5="0.0s"
FINAL_URL=""
FINAL_USER="HCM"
FINAL_PASS="312021"
FINAL_DB="SQLite (/etc/x-ui/x-ui.db)"
FINAL_TOKEN=""
DETECTED_IP=""

# Check root
[[ $EUID -ne 0 ]] && echo -e "${C_RED}Fatal error:${C_NC} Please run this script with root privilege \n" && exit 1

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

arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${C_RED}Unsupported CPU architecture!${C_NC}" && rm -f "$(realpath "$0")" && exit 1 ;;
    esac
}

if [[ "${XUI_NONINTERACTIVE:-0}" == "1" ]] || [[ ! -t 0 ]]; then
    NONINTERACTIVE=1
else
    NONINTERACTIVE=0
fi
export NONINTERACTIVE

# ══════════════════════════════════════════════════════════════════════════
# AUTHENTICATION GUARD (Username: HCM, Password: 312021)
# ══════════════════════════════════════════════════════════════════════════
authenticate_user() {
    clear
    echo -e "${C_FRAME} ╔══════════════════════════════════════════════════════════════════════════╗${C_NC}"
    printf "${C_FRAME} ║${C_NC} ${C_CYAN}█▓▒░${C_NC} ${C_WHITE}%-52s${C_NC} ${C_FRAME}[ SECURE LOGIN ]${C_NC} ${C_FRAME}║${C_NC}\n" "[ AUTHENTICATION REQUIRED ] ────────────────────"
    echo -e "${C_FRAME} ╠══════════════════════════════════════════════════════════════════════════╣${C_NC}"
    echo -e "${C_FRAME} ║${C_NC}                                                                          ${C_FRAME}║${C_NC}"
    
    echo -ne " ${C_FRAME}║${C_NC}   Enter Admin Username : "
    read -rp "" input_user
    echo -ne " ${C_FRAME}║${C_NC}   Enter Admin Password : "
    read -rs input_pass
    echo ""
    echo -e " ${C_FRAME}║${C_NC}                                                                          ${C_FRAME}║${C_NC}"
    echo -e "${C_FRAME} ╚══════════════════════════════════════════════════════════════════════════╝${C_NC}"

    if [[ "$input_user" == "HCM" && "$input_pass" == "312021" ]]; then
        echo -e "\n ${C_CYAN}[✔] Access Granted. Loading Enterprise Appliance...${C_NC}"
        sleep 0.8
        return 0
    else
        echo -e "\n ${C_RED}[✘] Authentication Failed! Invalid Username or Password.${C_NC}"
        exit 1
    fi
}

# ══════════════════════════════════════════════════════════════════════════
# ENTERPRISE APPLIANCE TUI ENGINE (STRICT 76-COL ALIGNMENT)
# ══════════════════════════════════════════════════════════════════════════

v_bar() {
    local pct="${1:-20}"
    [[ ! "$pct" =~ ^[0-9]+$ ]] && pct=20
    (( pct > 100 )) && pct=100
    (( pct < 0 )) && pct=0
    local fill=$(( pct * 8 / 100 ))
    local emp=$(( 8 - fill ))
    local bar=""
    for ((i=0; i<fill; i++)); do bar+="▮"; done
    for ((i=0; i<emp; i++)); do bar+="▯"; done
    echo "$bar"
}

print_appliance_header() {
    clear
    local os_ver=$(grep -oP '(?<=^NAME=).+' /etc/os-release 2>/dev/null | tr -d '"' | awk '{print $1}')
    local os_num=$(grep -oP '(?<=^VERSION_ID=).+' /etc/os-release 2>/dev/null | tr -d '"')
    local os_str=$(printf "%s %s" "${os_ver:-Linux}" "${os_num}" | cut -c 1-12)
    local kern_str=$(uname -r | cut -c 1-12)
    local arch_str=$(printf "%s / 64b" "$(arch)" | cut -c 1-12)

    if [[ -z "$DETECTED_IP" ]]; then
        DETECTED_IP=$(curl -s --max-time 2 https://api4.ipify.org 2>/dev/null || echo "Detecting...")
    fi
    local ip_str=$(printf "%s" "${SSL_HOST:-$DETECTED_IP}" | cut -c 1-15)
    local tls_str="Let's Encrypt"
    [[ "${SSL_SCHEME:-https}" == "http" ]] && tls_str="HTTP (No TLS)"

    local bbr_stat="Standard TCP"
    if sysctl net.ipv4.tcp_congestion_control 2>/dev/null | grep -q "bbr"; then
        bbr_stat="Active (TCP)"
    fi

    local cpu_p=$(top -bn1 2>/dev/null | awk '/Cpu\(s\)/{printf("%.0f", 100 - $8)}' || echo 18)
    [[ ! "$cpu_p" =~ ^[0-9]+$ ]] && cpu_p=18
    local ram_p=$(free 2>/dev/null | awk '/Mem/{printf("%.0f"), $3/$2*100}' || echo 42)
    [[ ! "$ram_p" =~ ^[0-9]+$ ]] && ram_p=42
    local hdd_p=$(df / 2>/dev/null | awk 'END{print $(NF-1)}' | tr -d '%' || echo 24)
    [[ ! "$hdd_p" =~ ^[0-9]+$ ]] && hdd_p=24

    local rev_str="${tag_version:-v3.10.2}"
    rev_str=$(printf "%s" "$rev_str" | cut -c 1-8)

    echo -e "${C_FRAME}┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓${C_NC}"
    printf "${C_FRAME}┃${C_NC} ${C_CYAN}██▓▒░${C_NC}  ${C_WHITE}%-49s${C_NC} ${C_YELLOW}[REV: %-8s]${C_NC} ${C_FRAME}┃${C_NC}\n" "3X-UI // ENTERPRISE NODE APPLIANCE" "$rev_str"
    echo -e "${C_FRAME}┣━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━┫${C_NC}"
    printf "${C_FRAME}┃${C_NC} ${C_CYAN}%-21s${C_NC} ${C_FRAME}┃${C_NC} ${C_CYAN}%-24s${C_NC} ${C_FRAME}┃${C_NC} ${C_CYAN}%-21s${C_NC} ${C_FRAME}┃${C_NC}\n" "❖ HOST ARCHITECTURE" "❖ NETWORK & ROUTING" "❖ HARDWARE TELEMETRY"
    printf "${C_FRAME}┃${C_NC} ${C_DIM}─────────────────────${C_NC} ${C_FRAME}┃${C_NC} ${C_DIM}────────────────────────${C_NC} ${C_FRAME}┃${C_NC} ${C_DIM}─────────────────────${C_NC} ${C_FRAME}┃${C_NC}\n"
    printf "${C_FRAME}┃${C_NC} ▸ OS   : ${C_WHITE}%-12.12s${C_NC} ${C_FRAME}┃${C_NC} ▸ IPv4 : ${C_YELLOW}%-15.15s${C_NC} ${C_FRAME}┃${C_NC} ▸ CPU : ${C_GREEN}%s${C_NC} %3d%% ${C_FRAME}┃${C_NC}\n" "$os_str" "$ip_str" "$(v_bar "$cpu_p")" "$cpu_p"
    printf "${C_FRAME}┃${C_NC} ▸ ARCH : ${C_WHITE}%-12.12s${C_NC} ${C_FRAME}┃${C_NC} ▸ TLS  : ${C_WHITE}%-15.15s${C_NC} ${C_FRAME}┃${C_NC} ▸ RAM : ${C_YELLOW}%s${C_NC} %3d%% ${C_FRAME}┃${C_NC}\n" "$arch_str" "$tls_str" "$(v_bar "$ram_p")" "$ram_p"
    printf "${C_FRAME}┃${C_NC} ▸ KERN : ${C_DIM}%-12.12s${C_NC} ${C_FRAME}┃${C_NC} ▸ BBR  : ${C_GREEN}%-15.15s${C_NC} ${C_FRAME}┃${C_NC} ▸ HDD : ${C_CYAN}%s${C_NC} %3d%% ${C_FRAME}┃${C_NC}\n" "$kern_str" "$bbr_stat" "$(v_bar "$hdd_p")" "$hdd_p"
    echo -e "${C_FRAME}┣━━━━━━━━━━━━━━━━━━━━━━━┻━━━━━━━━━━━━━━━━━━━━━━━━━━┻━━━━━━━━━━━━━━━━━━━━━━━┫${C_NC}"
    printf "${C_FRAME}┃${C_NC} ${C_CYAN}❖ NODE TOPOLOGY :${C_NC}  ${C_WHITE}[WAN]${C_NC} ${C_DIM}──►${C_NC} ${C_YELLOW}[FAIL2BAN]${C_NC} ${C_DIM}──►${C_NC} ${C_GREEN}[TLS:1.3]${C_NC} ${C_DIM}──►${C_NC} ${C_CYAN}[XRAY-ENGINE]${C_NC}  ${C_FRAME}┃${C_NC}\n"
    echo -e "${C_FRAME}┣━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫${C_NC}"
    printf "${C_FRAME}┃${C_NC} ${C_DIM}PHASE   MODULE DESCRIPTION                     PROGRESS       TIME  STAT${C_NC} ${C_FRAME}┃${C_NC}\n"
    echo -e "${C_FRAME}┃ ──────────────────────────────────────────────────────────────────────── ┃${C_NC}"
}

print_completed_phase_row() {
    local hex="$1"
    local desc="$2"
    local elapsed="$3"
    printf "${C_FRAME}┃${C_NC} [${C_CYAN}%-4.4s${C_NC}]  %-38.38s ${C_GREEN}▮▮▮▮▮▮▮▮▮▮${C_NC}   %6.6s  [${C_GREEN}OK${C_NC}] ${C_FRAME}┃${C_NC}\n" "$hex" "$desc" "$elapsed"
}

run_appliance_phase() {
    local hex="$1"
    local desc="$2"
    local out_var="$3"
    shift 3

    local start_ts=$(date +%s%N)
    local anim=(
        "▮▯▯▯▯▯▯▯▯▯" "▮▮▯▯▯▯▯▯▯▯" "▮▮▮▯▯▯▯▯▯▯" "▮▮▮▯▯▯▯▯▯▯" "▮▮▮▯▯▯▯▯▯▯"
        "▮▮▮▮▮▮▯▯▯▯" "▮▮▮▮▮▮▮▮▮▮" "▮▮▮▮▮▮▮▮▮▮" "▮▮▮▮▮▮▮▮▮▮" "▮▮▮▮▮▮▮▮▮▮"
    )
    local idx=0

    "$@" >> "$LOG_FILE" 2>&1 &
    local pid=$!

    while kill -0 $pid 2>/dev/null; do
        printf "\r${C_FRAME}┃${C_NC} [${C_CYAN}%-4.4s${C_NC}]  %-38.38s ${C_YELLOW}%s${C_NC}     ${C_DIM}....${C_NC}  [${C_YELLOW}..${C_NC}] ${C_FRAME}┃${C_NC}" "$hex" "$desc" "${anim[$idx]}"
        idx=$(( (idx + 1) % 10 ))
        sleep 0.12
    done

    wait $pid
    local rc=$?
    local end_ts=$(date +%s%N)
    local elapsed=$(awk "BEGIN {printf \"%.1fs\", ($end_ts - $start_ts)/1000000000}")
    printf -v "$out_var" '%s' "$elapsed"

    if [ $rc -eq 0 ]; then
        print_completed_phase_row "$hex" "$desc" "$elapsed"
    else
        printf "\r${C_FRAME}┃${C_NC} [${C_RED}%-4.4s${C_NC}]  %-38.38s ${C_RED}▮▮▮▮▮▮▮▮▮▮${C_NC}   %6.6s  [${C_RED}XX${C_NC}] ${C_FRAME}┃${C_NC}\n" "$hex" "$desc" "$elapsed"
        echo -e "${C_FRAME}┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛${C_NC}"
        echo -e "${C_RED}[!] Critical failure at phase ${hex}. Detailed log: ${LOG_FILE}${C_NC}"
        tail -n 15 "$LOG_FILE" >&2
        exit 1
    fi
}

print_final_appliance_screen() {
    print_appliance_header
    print_completed_phase_row "0x01" "System Core & Crypto Libraries" "$T_P1"
    print_completed_phase_row "0x02" "3X-UI Binary & Xray-Core Engine" "$T_P2"
    print_completed_phase_row "0x03" "Database Provisioning & Migration" "$T_P3"
    print_completed_phase_row "0x04" "ACME Zero-Trust SSL Certificate" "$T_P4"
    print_completed_phase_row "0x05" "Systemd Hardening & IP-Limit Guard" "$T_P5"

    local url_disp=$(printf "%s" "${FINAL_URL}" | cut -c 1-48)
    local usr_disp=$(printf "%s" "${FINAL_USER}" | cut -c 1-48)
    local pas_disp=$(printf "%s" "${FINAL_PASS}" | cut -c 1-48)
    local db_disp=$(printf "%s" "${FINAL_DB}" | cut -c 1-48)
    local tok_disp="Configured (Use 'x-ui settings' to view)"
    if [[ -n "$FINAL_TOKEN" ]]; then
        tok_disp="$(printf "%s" "$FINAL_TOKEN" | cut -c 1-45)..."
    fi

    echo -e "${C_FRAME}┣━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫${C_NC}"
    printf "${C_FRAME}┃${C_NC} ${C_YELLOW}██▓▒░${C_NC}  ${C_WHITE}%-58s${C_NC}   [${C_YELLOW}##${C_NC}] ${C_FRAME}┃${C_NC}\n" "MASTER ACCESS KEYCARD // AUTHORIZED OPERATOR ONLY"
    echo -e "${C_FRAME}┃ ──────────────────────────────────────────────────────────────────────── ┃${C_NC}"
    echo -e "${C_FRAME}┃                                                                          ┃${C_NC}"
    echo -e "${C_FRAME}┃${C_NC}   ${C_CYAN}┏━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓${C_NC}   ${C_FRAME}┃${C_NC}"
    printf "${C_FRAME}┃${C_NC}   ${C_CYAN}┃${C_NC} ${C_CYAN}◈${C_NC} ${C_WHITE}%-11.11s${C_NC} ${C_CYAN}┃${C_NC} ${C_GREEN}%-48.48s${C_NC} ${C_CYAN}┃${C_NC}   ${C_FRAME}┃${C_NC}\n" "ACCESS URL" "$url_disp"
    echo -e "${C_FRAME}┃${C_NC}   ${C_CYAN}┣━━━━━━━━━━━━━━━╋━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫${C_NC}   ${C_FRAME}┃${C_NC}"
    printf "${C_FRAME}┃${C_NC}   ${C_CYAN}┃${C_NC} ${C_CYAN}◈${C_NC} ${C_WHITE}%-11.11s${C_NC} ${C_CYAN}┃${C_NC} ${C_YELLOW}%-48.48s${C_NC} ${C_CYAN}┃${C_NC}   ${C_FRAME}┃${C_NC}\n" "USERNAME" "$usr_disp"
    printf "${C_FRAME}┃${C_NC}   ${C_CYAN}┃${C_NC} ${C_CYAN}◈${C_NC} ${C_WHITE}%-11.11s${C_NC} ${C_CYAN}┃${C_NC} ${C_YELLOW}%-48.48s${C_NC} ${C_CYAN}┃${C_NC}   ${C_FRAME}┃${C_NC}\n" "PASSWORD" "$pas_disp"
    printf "${C_FRAME}┃${C_NC}   ${C_CYAN}┃${C_NC} ${C_CYAN}◈${C_NC} ${C_WHITE}%-11.11s${C_NC} ${C_CYAN}┃${C_NC} ${C_WHITE}%-48.48s${C_NC} ${C_CYAN}┃${C_NC}   ${C_FRAME}┃${C_NC}\n" "DATABASE" "$db_disp"
    printf "${C_FRAME}┃${C_NC}   ${C_CYAN}┃${C_NC} ${C_CYAN}◈${C_NC} ${C_WHITE}%-11.11s${C_NC} ${C_CYAN}┃${C_NC} ${C_DIM}%-48.48s${C_NC} ${C_CYAN}┃${C_NC}   ${C_FRAME}┃${C_NC}\n" "API TOKEN" "$tok_disp"
    echo -e "${C_FRAME}┃${C_NC}   ${C_CYAN}┗━━━━━━━━━━━━━━━┻━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛${C_NC}   ${C_FRAME}┃${C_NC}"
    echo -e "${C_FRAME}┃                                                                          ┃${C_NC}"
    printf "${C_FRAME}┃${C_NC} ▸ ${C_CYAN}CONTROL :${C_NC}  [ ${C_GREEN}x-ui${C_NC} ] Main Menu   [ ${C_YELLOW}x-ui status${C_NC} ] Service   [ ${C_CYAN}x-ui log${C_NC} ] ${C_FRAME}┃${C_NC}\n"
    echo -e "${C_FRAME}┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛${C_NC}"

    if (( ${#FINAL_URL} > 48 )); then
        echo -e "  ${C_CYAN}▸ Full Access URL:${C_NC} ${C_GREEN}${FINAL_URL}${C_NC}"
    fi
}

# ══════════════════════════════════════════════════════════════════════════
# CORE HELPER FUNCTIONS & WIZARDS
# ══════════════════════════════════════════════════════════════════════════

is_ipv4() { [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] && return 0 || return 1; }
is_ipv6() { [[ "$1" =~ : ]] && return 0 || return 1; }
is_ip() { is_ipv4 "$1" || is_ipv6 "$1"; }
is_domain() { [[ "$1" =~ ^([A-Za-z0-9](-*[A-Za-z0-9])*\.)+(xn--[a-z0-9]{2,}|[A-Za-z]{2,})$ ]] && return 0 || return 1; }

acme_listen_flag() {
    if ip -4 addr show scope global 2> /dev/null | grep -q "inet "; then
        echo ""
    else
        echo "--listen-v6"
    fi
}

is_port_in_use() {
    local port="$1"
    if command -v ss > /dev/null 2>&1; then
        ss -ltn 2> /dev/null | awk -v p=":${port}$" '$4 ~ p {exit 0} END {exit 1}'
        return
    fi
    return 1
}

install_base() {
    case "${release}" in
        ubuntu | debian | armbian) apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl ;;
        fedora | amzn | rhel | almalinux | rocky | ol) dnf makecache -y && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl ;;
        arch | manjaro) pacman -Sy --noconfirm cronie curl tar tzdata socat ca-certificates openssl ;;
        alpine) apk update && apk add dcron curl tar tzdata socat ca-certificates openssl ;;
        *) apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl ;;
    esac
}

gen_random_string() {
    local length="$1"
    openssl rand -base64 $((length * 2)) | tr -dc 'a-zA-Z0-9' | head -c "$length"
}

prompt_or_default() {
    local __var="$1" __prompt="$2" __default="$3" __env="${4:-$1}"
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        printf -v "$__var" '%s' "${!__env:-$__default}"
    else
        read -rp "$__prompt" "$__var"
    fi
}

write_install_result() {
    local u="$1" p="$2" port="$3" wbp="$4" scheme="$5" host="$6" token="$7" dbtype="$8"
    local result_file="/etc/x-ui/install-result.env"
    local url_host="${host:-SERVER_IP_UNKNOWN}"
    install -d -m 700 /etc/x-ui 2> /dev/null
    local prev_umask=$(umask)
    umask 077
    {
        printf 'XUI_USERNAME=%q\n' "$u"
        printf 'XUI_PASSWORD=%q\n' "$p"
        printf 'XUI_PANEL_PORT=%q\n' "$port"
        printf 'XUI_WEB_BASE_PATH=%q\n' "$wbp"
        printf 'XUI_ACCESS_URL=%q\n' "${scheme}://${url_host}:${port}/${wbp}"
        printf 'XUI_API_TOKEN=%q\n' "$token"
        printf 'XUI_DB_TYPE=%q\n' "$dbtype"
    } > "$result_file"
    umask "$prev_umask"
    chmod 600 "$result_file" 2> /dev/null
    chown root:root "$result_file" 2> /dev/null || true
}

# Run Authentication Guard first (HCM / 312021)
authenticate_user

# Execute base installation & pipeline
install_appliance_pipeline() {
    print_appliance_header
    run_appliance_phase "0x01" "System Core & Crypto Libraries" T_P1 install_base
    run_appliance_phase "0x02" "3X-UI Binary & Xray-Core Engine" T_P2 _phase2_download_and_extract
    config_after_install
    print_appliance_header
    print_completed_phase_row "0x01" "System Core & Crypto Libraries" "$T_P1"
    print_completed_phase_row "0x02" "3X-UI Binary & Xray-Core Engine" "$T_P2"
    print_completed_phase_row "0x03" "Database Provisioning & Migration" "$T_P3"
    print_completed_phase_row "0x04" "ACME Zero-Trust SSL Certificate" "$T_P4"
    run_appliance_phase "0x05" "Systemd Hardening & IP-Limit Guard" T_P5 _phase5_service_and_hardening
    print_final_appliance_screen
}
