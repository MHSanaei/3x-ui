#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# Значения по умолчанию для автоматического режима
MODE="git"          # Режим для X-UI: git или build
INSTALL_BOT=0       # Флаг установки бота (0 - нет, 1 - да)

# -------------------------------------------------------------------
# Разбор аргументов командной строки (для автоматизации)
# -------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case $1 in
        --build)
            MODE="build"
            shift
            ;;
        --with-bot)
            INSTALL_BOT=1
            shift
            ;;
        *)
            echo -e "${red}Unknown option: $1${plain}"
            exit 1
            ;;
    esac
done

# Проверка на права root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1

# Определение ОС
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

# Определение архитектуры
arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${green}Unsupported CPU architecture! ${plain}" && exit 1 ;;
    esac
}
echo "Arch: $(arch)"

if [[ "${XUI_NONINTERACTIVE:-0}" == "1" ]] || [[ ! -t 0 ]]; then
    NONINTERACTIVE=1
else
    NONINTERACTIVE=0
fi
export NONINTERACTIVE

# -------------------------------------------------------------------
# Вспомогательные функции
# -------------------------------------------------------------------
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

# -------------------------------------------------------------------
# Установка зависимостей
# -------------------------------------------------------------------
install_base() {
    echo -e "${green}Installing base dependencies...${plain}"
    case "${release}" in
        ubuntu | debian | armbian) apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl unzip git wget ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol) dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl unzip git wget ;;
        centos) if [[ "${VERSION_ID}" =~ ^7 ]]; then yum -y update && yum install -y cronie curl tar tzdata socat ca-certificates openssl unzip git wget; else dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl unzip git wget; fi ;;
        arch | manjaro | parch) pacman -Syu --noconfirm cronie curl tar tzdata socat ca-certificates openssl unzip git wget ;;
        opensuse-tumbleweed | opensuse-leap) zypper refresh && zypper -q install -y cron curl tar timezone socat ca-certificates openssl unzip git wget ;;
        alpine) apk update && apk add dcron curl tar tzdata socat ca-certificates openssl unzip bash git wget ;;
        *) apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl unzip git wget ;;
    esac
}

install_build_deps() {
    echo -e "${green}Installing build tools (Node.js, npm, Go)...${plain}"
    case "${release}" in
        ubuntu | debian | armbian) apt-get install -y -q nodejs npm golang-go ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol | centos) dnf install -y -q nodejs npm golang ;;
        arch | manjaro | parch) pacman -S --noconfirm nodejs npm go ;;
        alpine) apk add nodejs npm go ;;
        *) apt-get install -y -q nodejs npm golang-go ;;
    esac
}

install_bot_deps() {
    echo -e "${green}Installing Python dependencies for Telegram Bot...${plain}"
    case "${release}" in
        ubuntu | debian | armbian) apt-get install -y -q python3 python3-pip python3-venv zip unzip ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol | centos) dnf install -y -q python3 python3-pip zip unzip ;;
        arch | manjaro | parch) pacman -S --noconfirm python python-pip zip unzip ;;
        alpine) apk add python3 py3-pip zip unzip ;;
        *) apt-get install -y -q python3 python3-pip python3-venv zip unzip ;;
    esac
}

# -------------------------------------------------------------------
# Базы данных и Сертификаты
# -------------------------------------------------------------------
install_postgres_local() {
    local pg_user pg_pass
    pg_pass=$(gen_random_string 24)
    local pg_db="xui"
    local pg_host="127.0.0.1"
    local pg_port="5432"

    case "${release}" in
        ubuntu | debian | armbian) apt-get update >&2 && apt-get install -y -q postgresql >&2 || return 1 ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol) dnf install -y -q postgresql-server postgresql-contrib >&2 || return 1; [[ -d /var/lib/pgsql/data && -f /var/lib/pgsql/data/PG_VERSION ]] || postgresql-setup --initdb >&2 || return 1 ;;
        centos) if [[ "${VERSION_ID}" =~ ^7 ]]; then yum install -y postgresql-server postgresql-contrib >&2 || return 1; else dnf install -y -q postgresql-server postgresql-contrib >&2 || return 1; fi; [[ -d /var/lib/pgsql/data && -f /var/lib/pgsql/data/PG_VERSION ]] || postgresql-setup --initdb >&2 || return 1 ;;
        arch | manjaro | parch) pacman -Syu --noconfirm postgresql >&2 || return 1; if [[ ! -f /var/lib/postgres/data/PG_VERSION ]]; then sudo -u postgres initdb -D /var/lib/postgres/data >&2 || return 1; fi ;;
        opensuse-tumbleweed | opensuse-leap) zypper -q install -y postgresql-server postgresql-contrib >&2 || return 1; if [[ ! -f /var/lib/pgsql/data/PG_VERSION ]]; then install -d -o postgres -g postgres -m 700 /var/lib/pgsql/data >&2 || return 1; su - postgres -c "initdb -D /var/lib/pgsql/data" >&2 || return 1; fi ;;
        alpine) apk add --no-cache postgresql postgresql-contrib >&2 || return 1; if [[ ! -f /var/lib/postgresql/data/PG_VERSION ]]; then /etc/init.d/postgresql setup >&2 || return 1; fi; rc-update add postgresql default >&2 2> /dev/null || true; rc-service postgresql start >&2 || return 1 ;;
        *) echo -e "${red}Unsupported distro for automatic PostgreSQL install: ${release}${plain}" >&2; return 1 ;;
    esac

    if [[ "${release}" != "alpine" ]]; then systemctl enable --now postgresql >&2 || return 1; fi

    local i
    for i in 1 2 3 4 5; do sudo -u postgres psql -tAc 'SELECT 1' > /dev/null 2>&1 && break; sleep 1; done

    local existing_owner=""
    existing_owner=$(sudo -u postgres psql -tAc "SELECT pg_catalog.pg_get_userbyid(datdba) FROM pg_database WHERE datname='${pg_db}'" 2> /dev/null | tr -d '[:space:]')
    if [[ -n "${existing_owner}" && "${existing_owner}" != "postgres" ]]; then pg_user="${existing_owner}"; else pg_user=$(gen_random_string 8); fi

    sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${pg_user}'" 2> /dev/null | grep -q 1 || sudo -u postgres psql -c "CREATE USER \"${pg_user}\" WITH PASSWORD '${pg_pass}';" >&2 || return 1
    sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${pg_db}'" 2> /dev/null | grep -q 1 || sudo -u postgres psql -c "CREATE DATABASE \"${pg_db}\" OWNER \"${pg_user}\";" >&2 || return 1
    sudo -u postgres psql -c "ALTER USER \"${pg_user}\" WITH PASSWORD '${pg_pass}';" >&2 || return 1

    local pg_pass_enc
    pg_pass_enc=$(printf '%s' "${pg_pass}" | sed -e 's/%/%25/g' -e 's/:/%3A/g' -e 's/@/%40/g' -e 's|/|%2F|g' -e 's/?/%3F/g' -e 's/#/%23/g')

    echo "postgres://${pg_user}:${pg_pass_enc}@${pg_host}:${pg_port}/${pg_db}?sslmode=disable"
    return 0
}

ensure_pg_client() {
    if command -v pg_dump > /dev/null 2>&1 && command -v pg_restore > /dev/null 2>&1; then return 0; fi
    case "${release}" in
        ubuntu | debian | armbian) apt-get install -y -q postgresql-client >&2 || return 1 ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol) dnf install -y -q postgresql >&2 || return 1 ;;
        centos) if [[ "${VERSION_ID}" =~ ^7 ]]; then yum install -y postgresql >&2 || return 1; else dnf install -y -q postgresql >&2 || return 1; fi ;;
        arch | manjaro | parch) pacman -Sy --noconfirm postgresql >&2 || return 1 ;;
        alpine) apk add --no-cache postgresql-client >&2 || return 1 ;;
        *) return 1 ;;
    esac
}

install_acme() {
    echo -e "${green}Installing acme.sh for SSL certificate management...${plain}"
    cd ~ || return 1
    curl -s https://get.acme.sh | sh > /dev/null 2>&1
    if [ $? -ne 0 ]; then echo -e "${red}Failed to install acme.sh${plain}"; return 1; fi
    echo -e "${green}acme.sh installed successfully${plain}"
    return 0
}

setup_ip_certificate() {
    local ipv4="$1" ipv6="$2"
    echo -e "${green}Setting up Let's Encrypt IP certificate...${plain}"
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then install_acme; if [ $? -ne 0 ]; then return 1; fi; fi
    if [[ -z "$ipv4" ]] || ! is_ipv4 "$ipv4"; then echo -e "${red}Invalid IPv4 address: $ipv4${plain}"; return 1; fi

    local certDir="/root/cert/ip"
    mkdir -p "$certDir"
    local domain_args="-d ${ipv4}"
    if [[ -n "$ipv6" ]] && is_ipv6 "$ipv6"; then domain_args="${domain_args} -d ${ipv6}"; fi

    local reloadCmd="systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null || true"
    local WebPort="${XUI_ACME_HTTP_PORT:-80}"

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1
    ~/.acme.sh/acme.sh --issue ${domain_args} --standalone --server letsencrypt --certificate-profile shortlived --days 6 --httpport ${WebPort} --force
    if [ $? -ne 0 ]; then echo -e "${red}Failed to issue IP certificate${plain}"; return 1; fi

    ~/.acme.sh/acme.sh --installcert -d ${ipv4} --key-file "${certDir}/privkey.pem" --fullchain-file "${certDir}/fullchain.pem" --reloadcmd "${reloadCmd}" 2>&1 || true
    chmod 600 ${certDir}/privkey.pem 2> /dev/null
    chmod 644 ${certDir}/fullchain.pem 2> /dev/null

    ${xui_folder}/x-ui cert -webCert "${certDir}/fullchain.pem" -webCertKey "${certDir}/privkey.pem"
    echo -e "${green}IP certificate installed successfully!${plain}"
    return 0
}

ssl_cert_issue() {
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then install_acme; if [ $? -ne 0 ]; then return 1; fi; fi
    local domain=""
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        domain="${XUI_DOMAIN// /}"
    else
        while true; do
            read -rp "Please enter your domain name: " domain
            domain="${domain// /}"
            if [[ -z "$domain" ]] || ! is_domain "$domain"; then continue; fi
            break
        done
    fi
    SSL_ISSUED_DOMAIN="${domain}"

    certPath="/root/cert/${domain}"
    mkdir -p "$certPath"
    local WebPort="${XUI_ACME_HTTP_PORT:-80}"
    
    systemctl stop x-ui 2> /dev/null || rc-service x-ui stop 2> /dev/null
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
    ~/.acme.sh/acme.sh --issue -d ${domain} $(acme_listen_flag) --standalone --httpport ${WebPort} --force
    
    local reloadCmd="systemctl restart x-ui || rc-service x-ui restart"
    ~/.acme.sh/acme.sh --installcert -d ${domain} --key-file /root/cert/${domain}/privkey.pem --fullchain-file /root/cert/${domain}/fullchain.pem --reloadcmd "${reloadCmd}" 2>&1
    
    chmod 600 $certPath/privkey.pem 2> /dev/null
    chmod 644 $certPath/fullchain.pem 2> /dev/null
    systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null

    local setPanel="y"
    if [[ "$NONINTERACTIVE" != "1" ]]; then read -rp "Would you like to set this certificate for the panel? (y/n): " setPanel; fi
    if [[ "$setPanel" == "y" || "$setPanel" == "Y" ]]; then
        ${xui_folder}/x-ui cert -webCert "/root/cert/${domain}/fullchain.pem" -webCertKey "/root/cert/${domain}/privkey.pem"
        systemctl restart x-ui 2> /dev/null || rc-service x-ui restart 2> /dev/null
    fi
    return 0
}

prompt_and_setup_ssl() {
    local panel_port="$1" web_base_path="$2" server_ip="$3"
    local ssl_choice=""
    SSL_SCHEME="https"

    echo -e "${yellow}Choose SSL certificate setup method:${plain}"
    echo -e "${green}1.${plain} Let's Encrypt for Domain (90-day validity)"
    echo -e "${green}2.${plain} Let's Encrypt for IP Address (6-day validity)"
    echo -e "${green}3.${plain} Custom SSL Certificate"
    echo -e "${green}4.${plain} Skip SSL"
    
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        ssl_choice="${XUI_SSL_MODE:-4}"
    else
        read -rp "Choose an option (default 2 for IP): " ssl_choice
        ssl_choice="${ssl_choice:-2}"
    fi

    case "$ssl_choice" in
        1) if ssl_cert_issue; then SSL_HOST="${SSL_ISSUED_DOMAIN:-$server_ip}"; else SSL_HOST="${server_ip}"; fi ;;
        2) systemctl stop x-ui > /dev/null 2>&1 || rc-service x-ui stop > /dev/null 2>&1; setup_ip_certificate "${server_ip}" ""; SSL_HOST="${server_ip}" ;;
        3) read -rp "Domain: " cdmn; read -rp "Cert path: " ccert; read -rp "Key path: " ckey; ${xui_folder}/x-ui cert -webCert "$ccert" -webCertKey "$ckey" >/dev/null 2>&1; SSL_HOST="${cdmn:-$server_ip}"; systemctl restart x-ui >/dev/null 2>&1 || rc-service x-ui restart >/dev/null 2>&1 ;;
        4) SSL_SCHEME="http"; SSL_HOST="${server_ip}" ;;
    esac
}

config_after_install() {
    local existing_hasDefaultCredential=$(${xui_folder}/x-ui setting -show true | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    local existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
    local server_ip=$(curl -s -w "\n%{http_code}" --max-time 3 "https://v4.api.ipinfo.io/ip" 2> /dev/null | head -n-1 | tr -d '[:space:]"')
    if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then server_ip="${XUI_SERVER_IP:-127.0.0.1}"; fi

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath="${XUI_WEB_BASE_PATH:-$(gen_random_string 18)}"
            local config_username="${XUI_USERNAME:-$(gen_random_string 10)}"
            local config_password="${XUI_PASSWORD:-$(gen_random_string 10)}"
            local config_port="${XUI_PANEL_PORT:-$(shuf -i 1024-62000 -n 1)}"
            
            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"
            prompt_and_setup_ssl "${config_port}" "${config_webBasePath}" "${server_ip}"
            local config_apiToken=$(${xui_folder}/x-ui setting -getApiToken true | grep -Eo 'apiToken: .+' | awk '{print $2}')

            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}Username:    ${config_username}${plain}"
            echo -e "${green}Password:    ${config_password}${plain}"
            echo -e "${green}Port:        ${config_port}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "${green}Access URL:  ${SSL_SCHEME}://${SSL_HOST}:${config_port}/${config_webBasePath}${plain}"
            write_install_result "${config_username}" "${config_password}" "${config_port}" "${config_webBasePath}" "${SSL_SCHEME}" "${SSL_HOST}" "${config_apiToken}" "sqlite"
        else
            local config_webBasePath=$(gen_random_string 18)
            ${xui_folder}/x-ui setting -webBasePath "${config_webBasePath}"
            if [[ -z "${existing_cert}" ]]; then prompt_and_setup_ssl "${existing_port}" "${config_webBasePath}" "${server_ip}"; fi
        fi
    else
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_username="${XUI_USERNAME:-$(gen_random_string 10)}"
            local config_password="${XUI_PASSWORD:-$(gen_random_string 10)}"
            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}"
        fi
        if [[ -z "$existing_cert" ]]; then prompt_and_setup_ssl "${existing_port}" "${existing_webBasePath}" "${server_ip}"; fi
    fi
    ${xui_folder}/x-ui migrate
}

setup_fail2ban() {
    if [[ -x /usr/bin/x-ui ]]; then /usr/bin/x-ui setup-fail2ban; fi
}

# -------------------------------------------------------------------
# ИСПРАВЛЕННАЯ ФУНКЦИЯ УСТАНОВКИ XRAY-CORE
# -------------------------------------------------------------------
install_xray() {
    local arch_type=$(arch)
    local xray_dir="${xui_folder}/bin"
    mkdir -p "$xray_dir"
    echo -e "${green}Installing Xray-core...${plain}"
    
    # ФИКС: подгоняем маску имени под реальные файлы релизов GitHub
    local xray_file="Xray-linux-${arch_type}.zip"
    if [[ "$arch_type" == "arm64" ]]; then
        xray_file="Xray-linux-arm64-v8a.zip"
    elif [[ "$arch_type" == "arm7" || "$arch_type" == "armv7" ]]; then
        xray_file="Xray-linux-arm32-v7a.zip"
    elif [[ "$arch_type" == "386" ]]; then
        xray_file="Xray-linux-32.zip"
    elif [[ "$arch_type" == "amd64" ]]; then
        xray_file="Xray-linux-64.zip"
    fi

    local url="https://github.com/XTLS/Xray-core/releases/latest/download/${xray_file}"
    curl -fLR --retry 5 -o "${xray_dir}/xray.zip" "$url"
    if [ $? -eq 0 ]; then
        cd "$xray_dir" && unzip -o xray.zip > /dev/null && rm xray.zip
        
        # Переименовываем распакованный файл 'Xray' в 'xray' (нижний регистр) под логику панели
        if [[ -f "Xray" ]]; then
            mv Xray xray
        fi
        
        ln -sf xray xray-linux-amd64 # Фикс для панели
        chmod +x xray xray-linux-amd64
        echo -e "${green}Xray-core успешно установлен!${plain}"
    else
        echo -e "${red}Failed to install Xray-core!${plain}"
    fi
}

# -------------------------------------------------------------------
# Логика Установки БОТА (Git или Local)
# -------------------------------------------------------------------
install_xray_bot() {
    local bot_mode=$1
    local bot_dir="/usr/local/x-ui-bot"
    
    echo -e "${green}🤖 Starting Xray Bot installation (${bot_mode} mode)...${plain}"
    install_bot_deps
    
    mkdir -p "$bot_dir"
    
    if [[ "$bot_mode" == "build" ]]; then
        echo -e "${green}🚚 Local mode: Looking for bot files in ${cur_dir}/xray-bot...${plain}"
        if [[ -d "${cur_dir}/xray-bot" ]]; then
            cp -r "${cur_dir}/xray-bot/"* "$bot_dir/"
        else
            echo -e "${yellow}⚠️ Warning: '${cur_dir}/xray-bot' folder not found.${plain}"
            echo -e "${yellow}Switching to Git download for bot...${plain}"
            bot_mode="git"
        fi
    fi
    
    if [[ "$bot_mode" == "git" ]]; then
        echo -e "${green}🌐 Git mode: Cloning Xray Bot from repository...${plain}"
        rm -rf "$bot_dir"
        git clone https://github.com/NidukaA递/x-ui-telegram-bot.git "$bot_dir"
    fi
    
    if [[ -f "${bot_dir}/requirements.txt" ]]; then
        echo -e "${green}🐍 Setting up Python virtual environment...${plain}"
        python3 -m venv "${bot_dir}/venv"
        "${bot_dir}/venv/bin/pip" install --upgrade pip -q
        "${bot_dir}/venv/bin/pip" install -r "${bot_dir}/requirements.txt" -q
        echo -e "${green}✅ Xray Bot installed in ${bot_dir}${plain}"
        echo -e "${yellow}Для настройки бота укажите ваш Telegram TOKEN в файле конфига внутри ${bot_dir}${plain}"
    else
        echo -e "${yellow}Requirements.txt not found. Skiping pip install.${plain}"
    fi
}

# -------------------------------------------------------------------
# Основная функция установки X-UI
# -------------------------------------------------------------------
install_x-ui() {
    install_base

    if [[ -e ${xui_folder}/ ]]; then
        if [[ $release == "alpine" ]]; then rc-service x-ui stop > /dev/null 2>&1; else systemctl stop x-ui > /dev/null 2>&1; fi
        pkill -f 'mtg-linux-[^ ]* run ' > /dev/null 2>&1 || true
        rm ${xui_folder}/ -rf
    fi

    mkdir -p ${xui_folder}
    cd ${xui_folder}

    if [[ "$MODE" == "build" ]]; then
        echo -e "${green}🛠 Local build mode (build)...${plain}"
        install_build_deps
        cd "${cur_dir}"
        if [ ! -d "3x-ui" ]; then echo -e "${red}❌ 3x-ui folder not found! Execute in source folder.${plain}"; exit 1; fi
        
        cd 3x-ui
        chmod +x build.sh
        ./build.sh "$(arch)"
        
        echo -e "${green}🚚 Copying compiled files...${plain}"
        cp x-ui ${xui_folder}/x-ui
        cp x-ui.sh /usr/bin/x-ui
        chmod +x ${xui_folder}/x-ui /usr/bin/x-ui
    else
        echo -e "${green}🌐 Git mode: Downloading latest official release...${plain}"
        local arch_type=$(arch)
        
        # ФИКС: замена старой битой ссылки 3x-ui/3x-ui на рабочую Mauxito/3x-ui
        wget -N --no-check-certificate -O /tmp/x-ui-linux-${arch_type}.tar.gz https://github.com/KimaruBs/3x-ui/releases/latest/download/x-ui-linux-${arch_type}.tar.gz
        tar zxvf /tmp/x-ui-linux-${arch_type}.tar.gz -C /usr/local/
        chmod +x ${xui_folder}/x-ui /usr/bin/x-ui 2>/dev/null || true
        rm -f /tmp/x-ui-linux-${arch_type}.tar.gz
    fi

    # Сначала ставим ядро Xray-core
    install_xray

    # Проверяем условие установки бота
    if [[ "$INSTALL_BOT" == "1" ]]; then
        install_xray_bot "$MODE"
    fi

    # Финальные настройки и генерация данных панели
    config_after_install
    setup_fail2ban
}

# -------------------------------------------------------------------
# Интерактивное Меню
# -------------------------------------------------------------------
show_menu() {
    clear
    echo -e "${blue}===================================${plain}"
    echo -e "${green}    X-UI & Xray Bot Installer     ${plain}"
    echo -e "${blue}===================================${plain}"
    echo -e "1. Install X-UI (${green}From Git${plain})"
    echo -e "2. Install X-UI + Xray Bot (${green}From Git${plain})"
    echo -e "3. Build & Install X-UI (${yellow}Local Build${plain})"
    echo -e "4. Build & Install X-UI + Xray Bot (${yellow}Local Build${plain})"
    echo -e "0. Exit"
    echo -e "${blue}===================================${plain}"
    read -rp "Please choose an option: " menu_choice

    case "$menu_choice" in
        1) MODE="git"; INSTALL_BOT=0; install_x-ui ;;
        2) MODE="git"; INSTALL_BOT=1; install_x-ui ;;
        3) MODE="build"; INSTALL_BOT=0; install_x-ui ;;
        4) MODE="build"; INSTALL_BOT=1; install_x-ui ;;
        0) exit 0 ;;
        *) echo -e "${red}Invalid option!${plain}"; sleep 1; show_menu ;;
    esac
}

# -------------------------------------------------------------------
# Точка входа в скрипт
# -------------------------------------------------------------------
if [[ "$NONINTERACTIVE" == "0" && $# -eq 0 ]]; then
    show_menu
else
    # Если запуск был с флагами типа --build или --with-bot
    install_x-ui
fi
