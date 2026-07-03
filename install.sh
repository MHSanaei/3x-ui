#!/bin/bash

# Остановка скрипта при любой критической ошибке
set -e

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# Значения по умолчанию для меню
MODE="git"          
INSTALL_BOT=0       

# Проверка прав root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1

# Определение ОС и дистрибутива
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

# Определение архитектуры CPU
arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${green}Unsupported CPU architecture! ${plain}" && rm -f install.sh && exit 1 ;;
    esac
}
current_arch=$(arch)
echo "Arch: $current_arch"

# Режим интерактивности
if [[ "${XUI_NONINTERACTIVE:-0}" == "1" ]] || [[ ! -t 0 ]]; then
    NONINTERACTIVE=1
else
    NONINTERACTIVE=0
fi
export NONINTERACTIVE

# Помощники проверки строк
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
    if command -v netstat > /dev/null 2>&1; then
        netstat -lnt 2> /dev/null | awk -v p=":${port} " '$4 ~ p {exit 0} END {exit 1}'
        return
    fi
    if command -v lsof > /dev/null 2>&1; then
        lsof -nP -iTCP:${port} -sTCP:LISTEN > /dev/null 2>&1 && return 0
    fi
    return 1
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
    install -d -m 755 /etc/x-ui 2> /dev/null
    local prev_umask=$(umask); umask 077
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
    umask "$prev_umask"; chmod 600 "$result_file" 2> /dev/null; chown root:root "$result_file" 2> /dev/null || true
    echo -e "${green}Install result written to ${result_file} (mode 600).${plain}"
}

install_base() {
    echo -e "${green}Installing base dependencies...${plain}"
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl unzip git wget python3 python3-pip python3-venv zip
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf makecache -y && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl unzip git wget python3 python3-pip zip
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum makecache -y && yum install -y cronie curl tar tzdata socat ca-certificates openssl unzip git wget python3 python3-pip zip
            else
                dnf makecache -y && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl unzip git wget python3 python3-pip zip
            fi
            ;;
        *)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl unzip git wget python3 python3-pip zip
            ;;
    esac
}

install_build_deps() {
    echo -e "${green}Installing build tools (Node.js, Go)...${plain}"
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get install -y -q nodejs golang-go
            ;;
        *)
            echo "Попытка установить nodejs и golang через пакетный менеджер по умолчанию..."
            if command -v dnf &>/dev/null; then dnf install -y nodejs golang; else apt-get install -y nodejs golang; fi
            ;;
    esac
}

install_postgres_local() {
    local pg_user pg_pass; pg_pass=$(gen_random_string 24)
    local pg_db="xui"; local pg_host="127.0.0.1"; local pg_port="5432"

    echo -e "${green}Installing PostgreSQL locally...${plain}"
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update >&2 && apt-get install -y -q postgresql >&2 || return 1
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol | centos)
            if command -v dnf &>/dev/null; then dnf install -y -q postgresql-server postgresql-contrib >&2 || return 1; else yum install -y postgresql-server postgresql-contrib >&2 || return 1; fi
            [[ -d /var/lib/pgsql/data && -f /var/lib/pgsql/data/PG_VERSION ]] || postgresql-setup --initdb >&2 || return 1
            ;;
        *)
            echo -e "${red}Unsupported distro for automatic PostgreSQL install: ${release}${plain}" >&2
            return 1
            ;;
    esac

    systemctl enable --now postgresql >&2 || return 1

    for i in 1 2 3 4 5; do
        sudo -u postgres psql -tAc 'SELECT 1' > /dev/null 2>&1 && break
        sleep 1
    done

    local existing_owner=""
    existing_owner=$(sudo -u postgres psql -tAc "SELECT pg_catalog.pg_get_userbyid(datdba) FROM pg_database WHERE datname='${pg_db}'" 2> /dev/null | tr -d '[:space:]')
    if [[ -n "${existing_owner}" && "${existing_owner}" != "postgres" ]]; then pg_user="${existing_owner}"; else pg_user=$(gen_random_string 8); fi

    sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${pg_user}'" 2> /dev/null | grep -q 1 || sudo -u postgres psql -c "CREATE USER \"${pg_user}\" WITH PASSWORD '${pg_pass}';" >&2 || return 1
    sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${pg_db}'" 2> /dev/null | grep -q 1 || sudo -u postgres psql -c "CREATE DATABASE \"${pg_db}\" OWNER \"${pg_user}\";" >&2 || return 1
    sudo -u postgres psql -c "ALTER USER \"${pg_user}\" WITH PASSWORD '${pg_pass}';" >&2 || return 1

    local pg_pass_enc=$(printf '%s' "${pg_pass}" | sed -e 's/%/%25/g' -e 's/:/%3A/g' -e 's/@/%40/g' -e 's|/|%2F|g' -e 's/?/%3F/g' -e 's/#/%23/g')

    if [[ -n "${PG_CRED_FILE:-}" ]]; then
        local prev_umask=$(umask); umask 077
        cat > "${PG_CRED_FILE}" << EOF
PG_USER=${pg_user}
PG_PASS=${pg_pass}
PG_HOST=${pg_host}
PG_PORT=${pg_port}
PG_DB=${pg_db}
EOF
        umask "${prev_umask}"
    fi

    echo "postgres://${pg_user}:${pg_pass_enc}@${pg_host}:${pg_port}/${pg_db}?sslmode=disable"
    return 0
}

ensure_pg_client() {
    if command -v pg_dump > /dev/null 2>&1 && command -v pg_restore > /dev/null 2>&1; then return 0; fi
    echo -e "${yellow}Installing PostgreSQL client tools (pg_dump/pg_restore)...${plain}" >&2
    case "${release}" in
        ubuntu | debian | armbian) apt-get update >&2 && apt-get install -y -q postgresql-client >&2 || return 1 ;;
        *) if command -v dnf &>/dev/null; then dnf install -y -q postgresql >&2; else yum install -y postgresql >&2; fi || return 1 ;;
    esac
}

install_acme() {
    echo -e "${green}Installing acme.sh for SSL certificate management...${plain}"
    cd ~ || return 1
    curl -s https://get.acme.sh | sh > /dev/null 2>&1
    return 0
}

setup_ip_certificate() {
    local ipv4="$1"; local ipv6="$2"
    echo -e "${green}Setting up Let's Encrypt IP certificate...${plain}"
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then install_acme; fi

    local certDir="/root/cert/ip"; mkdir -p "$certDir"
    local domain_args="-d ${ipv4}"
    if [[ -n "$ipv6" ]] && is_ipv6 "$ipv6"; then domain_args="${domain_args} -d ${ipv6}"; fi

    local reloadCmd="systemctl restart x-ui 2>/dev/null || true"
    local WebPort="80"
    prompt_or_default WebPort "Port for ACME HTTP-01 (default 80): " "80" XUI_ACME_HTTP_PORT

    while true; do
        if is_port_in_use "${WebPort}"; then
            if [[ "$NONINTERACTIVE" == "1" ]]; then return 1; fi
            read -rp "Port ${WebPort} busy. Enter another port (or empty to abort): " alt_port
            [[ -z "${alt_port}" ]] && return 1
            WebPort="${alt_port}"
            continue
        fi
        break
    done

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1
    [[ -n "${XUI_ACME_EMAIL:-}" ]] && ~/.acme.sh/acme.sh --register-account -m "${XUI_ACME_EMAIL}" > /dev/null 2>&1

    ~/.acme.sh/acme.sh --issue ${domain_args} --standalone --server letsencrypt --certificate-profile shortlived --days 6 --httpport ${WebPort} --force || return 1

    ~/.acme.sh/acme.sh --installcert --force -d ${ipv4} --key-file "${certDir}/privkey.pem" --fullchain-file "${certDir}/fullchain.pem" --reloadcmd "${reloadCmd}" 2>&1 || true
    chmod 600 ${certDir}/privkey.pem 2> /dev/null; chmod 644 ${certDir}/fullchain.pem 2> /dev/null

    ${xui_folder}/x-ui cert -webCert "${certDir}/fullchain.pem" -webCertKey "${certDir}/privkey.pem" > /dev/null 2>&1
    return 0
}

ssl_cert_issue() {
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true 2>/dev/null | grep 'webBasePath:' | awk -F': ' '{print $2}' | tr -d '[:space:]' | sed 's#^/##' || echo "")
    local existing_port=$(${xui_folder}/x-ui setting -show true 2>/dev/null | grep 'port:' | awk -F': ' '{print $2}' | tr -d '[:space:]' || echo "")

    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then install_acme; fi

    local domain=""
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        domain="${XUI_DOMAIN// /}"
    else
        while true; do
            read -rp "Please enter your domain name: " domain
            domain="${domain// /}"
            [[ -n "$domain" ]] && is_domain "$domain" && break
            echo -e "${red}Invalid domain!${plain}"
        done
    fi
    SSL_ISSUED_DOMAIN="${domain}"

    local certPath="/root/cert/${domain}"; mkdir -p "$certPath"
    local WebPort=80
    prompt_or_default WebPort "Please choose ACME port (default 80): " "80" XUI_ACME_HTTP_PORT

    systemctl stop x-ui 2> /dev/null || true

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
    ~/.acme.sh/acme.sh --issue -d ${domain} $(acme_listen_flag) --standalone --httpport ${WebPort} --force

    local reloadCmd="systemctl restart x-ui"
    ~/.acme.sh/acme.sh --installcert --force -d ${domain} --key-file "$certPath/privkey.pem" --fullchain-file "$certPath/fullchain.pem" --reloadcmd "${reloadCmd}" > /dev/null 2>&1
    chmod 600 $certPath/privkey.pem 2> /dev/null; chmod 644 $certPath/fullchain.pem 2> /dev/null

    systemctl start x-ui 2> /dev/null || true

    ${xui_folder}/x-ui cert -webCert "$certPath/fullchain.pem" -webCertKey "$certPath/privkey.pem" > /dev/null 2>&1
    return 0
}

prompt_and_setup_ssl() {
    local panel_port="$1"; local web_base_path="$2"; local server_ip="$3"
    local ssl_choice=""
    SSL_SCHEME="https"

    echo -e "${yellow}Choose SSL certificate setup method:${plain}"
    echo -e "1. Let's Encrypt for Domain\n2. Let's Encrypt for IP Address\n3. Custom SSL Paths\n4. Skip SSL (HTTP)"
    
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        ssl_choice="4"
    else
        read -rp "Choose an option (default 4): " ssl_choice
        ssl_choice="${ssl_choice// /}"
        [[ -z "$ssl_choice" ]] && ssl_choice="4"
    fi

    case "$ssl_choice" in
        1) ssl_cert_issue && SSL_HOST="${SSL_ISSUED_DOMAIN}" || SSL_HOST="${server_ip}" ;;
        2) 
            local ipv6_addr=""
            read -rp "Optional IPv6: " ipv6_addr
            setup_ip_certificate "${server_ip}" "${ipv6_addr}" && SSL_HOST="${server_ip}" || SSL_HOST="${server_ip}"
            ;;
        3)
            read -rp "Cert Path: " c_cert; read -rp "Key Path: " c_key
            ${xui_folder}/x-ui cert -webCert "$c_cert" -webCertKey "$c_key" > /dev/null 2>&1
            SSL_HOST="${server_ip}"
            ;;
        *)
            SSL_SCHEME="http"; SSL_HOST="${server_ip}"
            ;;
    esac
}

config_after_install() {
    echo -e "${green}Инициализация конфигурации...${plain}"
    local existing_hasDefaultCredential=$(${xui_folder}/x-ui setting -show true 2>/dev/null | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}' || echo "true")
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true 2>/dev/null | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##' || echo "")
    local existing_port=$(${xui_folder}/x-ui setting -show true 2>/dev/null | grep -Eo 'port: .+' | awk '{print $2}' || echo "54321")

    local server_ip=$(curl -s --max-time 3 "https://v4.api.ipinfo.io/ip" || echo "127.0.0.1")
    if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then server_ip="127.0.0.1"; fi

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath=$(gen_random_string 18)
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)
            local config_port=$(shuf -i 1024-62000 -n 1)
            local db_label="SQLite"

            echo -e "1) SQLite (Default)\n2) PostgreSQL"
            read -rp "Choose DB [1]: " db_choice
            if [[ "$db_choice" == "2" ]]; then
                local xui_dsn=$(install_postgres_local || echo "")
                if [[ -n "$xui_dsn" ]]; then
                    local xui_env_file="/etc/default/x-ui"
                    mkdir -p "$(dirname "$xui_env_file")"
                    echo -e "XUI_DB_TYPE=postgres\nXUI_DB_DSN=${xui_dsn}" > "$xui_env_file"
                    db_label="PostgreSQL"
                fi
            fi

            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}" > /dev/null 2>&1
            prompt_and_setup_ssl "${config_port}" "${config_webBasePath}" "${server_ip}"

            local config_apiToken=$(${xui_folder}/x-ui setting -getApiToken true 2>/dev/null | grep -Eo 'apiToken: .+' | awk '{print $2}' || echo "")
            
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "Username:    ${config_username}"
            echo -e "Password:    ${config_password}"
            echo -e "Port:        ${config_port}"
            echo -e "WebBasePath: ${config_webBasePath}"
            echo -e "Access URL:  ${SSL_SCHEME}://${SSL_HOST}:${config_port}/${config_webBasePath}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            write_install_result "${config_username}" "${config_password}" "${config_port}" "${config_webBasePath}" "${SSL_SCHEME}" "${SSL_HOST}" "${config_apiToken}" "sqlite"
        fi
    fi
    ${xui_folder}/x-ui migrate > /dev/null 2>&1
}

install_xray() {
    local xray_dir="${xui_folder}/bin"
    if [[ -x "${xray_dir}/xray" ]] && "${xray_dir}/xray" -version &>/dev/null; then
        echo -e "${green}Xray-core уже установлен. Пропускаем.${plain}"
        return 0
    fi
    mkdir -p "$xray_dir"
    local xray_file="Xray-linux-64.zip"
    if [[ "$current_arch" == "arm64" ]]; then xray_file="Xray-linux-arm64-v8a.zip"; fi

    curl -fLR --retry 5 -o "${xray_dir}/xray.zip" "https://github.com/XTLS/Xray-core/releases/latest/download/${xray_file}"
    cd "$xray_dir" && unzip -o xray.zip > /dev/null && rm xray.zip
    [[ -f "Xray" ]] && mv Xray xray
    ln -sf xray xray-linux-${current_arch}
    chmod +x xray xray-linux-${current_arch}
}

install_xray_bot() {
    local bot_dir="${xui_folder}/xray-bot"
    echo -e "${green}🤖 Installing Xray Bot...${plain}"
    
    rm -f /usr/bin/xray-bot; mkdir -p "$bot_dir"
    
    if [[ "$MODE" == "build" ]]; then
        [[ -d "${cur_dir}/xray-bot" ]] && cp -r "${cur_dir}/xray-bot/"* "$bot_dir/"
        [[ -f "${cur_dir}/xray-bot.sh" ]] && cp "${cur_dir}/xray-bot.sh" /usr/bin/xray-bot
    else
        git clone https://github.com/KimaruBs/3x-ui.git "${bot_dir}_tmp"
        cp -r "${bot_dir}_tmp/xray-bot/"* "$bot_dir/"
        wget -N --no-check-certificate -O /usr/bin/xray-bot "https://raw.githubusercontent.com/KimaruBs/3x-ui/main/xray-bot.sh"
        rm -rf "${bot_dir}_tmp"
    fi
    
    [[ ! -f /usr/bin/xray-bot && -f "${bot_dir}/xray-bot.sh" ]] && cp "${bot_dir}/xray-bot.sh" /usr/bin/xray-bot
    chmod +x /usr/bin/xray-bot

    if [[ -f "${bot_dir}/requirements.txt" ]]; then
        python3 -m venv "${bot_dir}/venv"
        "${bot_dir}/venv/bin/pip" install --upgrade pip -q
        "${bot_dir}/venv/bin/pip" install -r "${bot_dir}/requirements.txt" -q
    fi

    cat > /etc/systemd/system/xray-bot.service <<EOF
[Unit]
Description=3x-ui Xray Telegram Bot
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${bot_dir}/src
ExecStart=${bot_dir}/venv/bin/python3 app.py
Restart=on-failure
RestartSec=3s

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload && systemctl enable xray-bot && systemctl restart xray-bot || true
}

start_installation() {
    install_base

    if [[ -e ${xui_folder}/ ]]; then
        systemctl stop x-ui > /dev/null 2>&1 || true
        find "${xui_folder}" -mindepth 1 -maxdepth 1 ! -name 'bin' ! -name 'xray-bot' -exec rm -rf {} +
    fi

    mkdir -p ${xui_folder}

    if [[ "$MODE" == "build" ]]; then
        install_build_deps
        cd "$cur_dir"
        chmod +x build.sh && ./build.sh "$current_arch"
        cp "build/x-ui-linux-${current_arch}" "${xui_folder}/x-ui"
        cp x-ui.sh /usr/bin/x-ui
    else
        wget -N --no-check-certificate -O "${xui_folder}/x-ui" "https://github.com/KimaruBs/3x-ui/releases/latest/download/x-ui-linux-${current_arch}"
        wget -N --no-check-certificate -O /usr/bin/x-ui "https://raw.githubusercontent.com/KimaruBs/3x-ui/main/x-ui.sh"
    fi

    chmod +x "${xui_folder}/x-ui" /usr/bin/x-ui
    install_xray

    [[ "$INSTALL_BOT" == "1" ]] && install_xray_bot

    cat > /etc/systemd/system/x-ui.service <<EOF
[Unit]
Description=3x-ui customized panel
After=network.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/usr/local/x-ui
ExecStart=/usr/local/x-ui/x-ui
Restart=on-failure
RestartSec=3s

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload && systemctl enable x-ui
    config_after_install
    systemctl restart x-ui || true

    echo -e "${yellow}Принудительный запуск Xray-core...${plain}"
    x-ui xray start || true
    echo -e "${green}🎉 Установка завершена!${plain}"
    exec /usr/bin/x-ui
}

show_menu() {
    clear
    echo -e "${blue}===================================${plain}"
    echo -e "${green}        3X-UI Smart Installer      ${plain}"
    echo -e "${blue}===================================${plain}"
    echo -e "1. Install X-UI (${green}Download from Git${plain})"
    echo -e "2. Install X-UI + Xray Bot (${green}Download from Git${plain})"
    echo -e "3. Build & Install X-UI (${yellow}Local Build${plain})"
    echo -e "4. Build & Install X-UI + Xray Bot (${yellow}Local Build${plain})"
    echo -e "0. Exit"
    echo -e "${blue}===================================${plain}"
    read -rp "Please choose an option: " menu_choice

    case "$menu_choice" in
        1) MODE="git";   INSTALL_BOT=0; start_installation ;;
        2) MODE="git";   INSTALL_BOT=1; start_installation ;;
        3) MODE="build"; INSTALL_BOT=0; start_installation ;;
        4) MODE="build"; INSTALL_BOT=1; start_installation ;;
        0) exit 0 ;;
        *) echo -e "${red}Invalid option!${plain}"; sleep 1; show_menu ;;
    esac
}

show_menu
