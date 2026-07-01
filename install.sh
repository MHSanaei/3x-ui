#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

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
        *) echo -e "${green}Unsupported CPU architecture! ${plain}" && exit 1 ;;
    esac
}

echo "Arch: $(arch)"

install_xray() {
    local arch=$(arch)
    local xray_dir="${xui_folder}/bin"
    mkdir -p "$xray_dir"
    echo -e "${green}Installing Xray-core...${plain}"
    local url="https://github.com/XTLS/Xray-core/releases/latest/download/xray-linux-${arch}.zip"
    curl -fLR --retry 5 -o "${xray_dir}/xray.zip" "$url"
    if [ $? -eq 0 ]; then
        cd "$xray_dir" && unzip -o xray.zip > /dev/null && rm xray.zip
        chmod +x xray
    else
        echo -e "${red}Failed to install Xray-core!${plain}"
    fi
}


if [[ "${XUI_NONINTERACTIVE:-0}" == "1" ]] || [[ ! -t 0 ]]; then
    NONINTERACTIVE=1
else
    NONINTERACTIVE=0
fi
export NONINTERACTIVE

# Simple helpers
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

install_base() {
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl unzip
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl unzip
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum -y update && yum install -y cronie curl tar tzdata socat ca-certificates openssl unzip
            else
                dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl unzip
            fi
            ;;
        arch | manjaro | parch)
            pacman -Syu && pacman -Syu --noconfirm cronie curl tar tzdata socat ca-certificates openssl unzip
            ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper refresh && zypper -q install -y cron curl tar timezone socat ca-certificates openssl unzip
            ;;
        alpine)
            apk update && apk add dcron curl tar tzdata socat ca-certificates openssl unzip
            ;;
        *)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl unzip
            ;;
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

install_postgres_local() {
    local pg_user pg_pass
    pg_pass=$(gen_random_string 24)
    local pg_db="xui"
    local pg_host="127.0.0.1"
    local pg_port="5432"

    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update >&2 && apt-get install -y -q postgresql >&2 || return 1
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf install -y -q postgresql-server postgresql-contrib >&2 || return 1
            [[ -d /var/lib/pgsql/data && -f /var/lib/pgsql/data/PG_VERSION ]] || postgresql-setup --initdb >&2 || return 1
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum install -y postgresql-server postgresql-contrib >&2 || return 1
            else
                dnf install -y -q postgresql-server postgresql-contrib >&2 || return 1
            fi
            [[ -d /var/lib/pgsql/data && -f /var/lib/pgsql/data/PG_VERSION ]] || postgresql-setup --initdb >&2 || return 1
            ;;
        arch | manjaro | parch)
            pacman -Syu --noconfirm postgresql >&2 || return 1
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
        systemctl enable --now postgresql >&2 || return 1
    fi

    local i
    for i in 1 2 3 4 5; do
        sudo -u postgres psql -tAc 'SELECT 1' > /dev/null 2>&1 && break
        sleep 1
    done

    local existing_owner=""
    existing_owner=$(sudo -u postgres psql -tAc "SELECT pg_catalog.pg_get_userbyid(datdba) FROM pg_database WHERE datname='${pg_db}'" 2> /dev/null | tr -d '[:space:]')
    if [[ -n "${existing_owner}" && "${existing_owner}" != "postgres" ]]; then
        pg_user="${existing_owner}"
    else
        pg_user=$(gen_random_string 8)
    fi

    sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${pg_user}'" 2> /dev/null | grep -q 1 || sudo -u postgres psql -c "CREATE USER \"${pg_user}\" WITH PASSWORD '${pg_pass}';" >&2 || return 1
    sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${pg_db}'" 2> /dev/null | grep -q 1 || sudo -u postgres psql -c "CREATE DATABASE \"${pg_db}\" OWNER \"${pg_user}\";" >&2 || return 1
    sudo -u postgres psql -c "ALTER USER \"${pg_user}\" WITH PASSWORD '${pg_pass}';" >&2 || return 1

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
        ubuntu | debian | armbian) apt-get update >&2 && apt-get install -y -q postgresql-client >&2 || return 1 ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol) dnf install -y -q postgresql >&2 || return 1 ;;
        centos) if [[ "${VERSION_ID}" =~ ^7 ]]; then yum install -y postgresql >&2 || return 1; else dnf install -y -q postgresql >&2 || return 1; fi ;;
        arch | manjaro | parch) pacman -Sy --noconfirm postgresql >&2 || return 1 ;;
        opensuse-tumbleweed | opensuse-leap) zypper -q install -y postgresql >&2 || return 1 ;;
        alpine) apk add --no-cache postgresql-client >&2 || return 1 ;;
        *) return 1 ;;
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
    local domain="$1" server_ip="$2" existing_port="$3" existing_webBasePath="$4"
    echo -e "${green}Setting up SSL certificate...${plain}"

    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${yellow}Failed to install acme.sh, skipping SSL setup${plain}"
            return 1
        fi
    fi

    local certPath="/root/cert/${domain}"
    mkdir -p "$certPath"
    echo -e "${green}Issuing SSL certificate for ${domain}...${plain}"

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1
    ~/.acme.sh/acme.sh --issue -d ${domain} $(acme_listen_flag) --standalone --httpport 80 --force

    if [ $? -ne 0 ]; then
        echo -e "${yellow}Failed to issue certificate for ${domain}${plain}"
        rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc 2> /dev/null
        rm -rf "$certPath" 2> /dev/null
        return 1
    fi

    ~/.acme.sh/acme.sh --installcert -d ${domain} --key-file /root/cert/${domain}/privkey.pem --fullchain-file /root/cert/${domain}/fullchain.pem --reloadcmd "systemctl restart x-ui" > /dev/null 2>&1
    if [ $? -ne 0 ]; then echo -e "${yellow}Failed to install certificate${plain}"; return 1; fi

    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1
    chmod 600 $certPath/privkey.pem 2> /dev/null
    chmod 644 $certPath/fullchain.pem 2> /dev/null

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

setup_ip_certificate() {
    local ipv4="$1" ipv6="$2"
    echo -e "${green}Setting up Let's Encrypt IP certificate...${plain}"
    
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then echo -e "${red}Failed to install acme.sh${plain}"; return 1; fi
    fi

    if [[ -z "$ipv4" ]] || ! is_ipv4 "$ipv4"; then
        echo -e "${red}Invalid IPv4 address: $ipv4${plain}"
        return 1
    fi

    local certDir="/root/cert/ip"
    mkdir -p "$certDir"
    local domain_args="-d ${ipv4}"
    if [[ -n "$ipv6" ]] && is_ipv6 "$ipv6"; then domain_args="${domain_args} -d ${ipv6}"; fi

    local reloadCmd="systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null || true"
    local WebPort=""
    prompt_or_default WebPort "Port to use for ACME HTTP-01 listener (default 80): " "80" XUI_ACME_HTTP_PORT
    WebPort="${WebPort:-80}"

    while true; do
        if is_port_in_use "${WebPort}"; then
            echo -e "${yellow}Port ${WebPort} is in use.${plain}"
            if [[ "$NONINTERACTIVE" == "1" ]]; then return 1; fi
            read -rp "Enter another port: " alt_port
            alt_port="${alt_port// /}"
            if [[ -n "${alt_port}" ]] && [[ "${alt_port}" =~ ^[0-9]+$ ]] && ((alt_port >= 1 && alt_port <= 65535)); then
                WebPort="${alt_port}"
                continue
            fi
            return 1
        else
            break
        fi
    done

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1
    [[ -n "${XUI_ACME_EMAIL:-}" ]] && ~/.acme.sh/acme.sh --register-account -m "${XUI_ACME_EMAIL}" > /dev/null 2>&1

    ~/.acme.sh/acme.sh --issue ${domain_args} --standalone --server letsencrypt --certificate-profile shortlived --days 6 --httpport ${WebPort} --force
    if [ $? -ne 0 ]; then
        echo -e "${red}Failed to issue IP certificate${plain}"
        rm -rf ~/.acme.sh/${ipv4} ~/.acme.sh/${ipv4}_ecc ~/.acme.sh/${ipv6} ~/.acme.sh/${ipv6}_ecc ${certDir} 2> /dev/null
        return 1
    fi

    ~/.acme.sh/acme.sh --installcert -d ${ipv4} --key-file "${certDir}/privkey.pem" --fullchain-file "${certDir}/fullchain.pem" --reloadcmd "${reloadCmd}" 2>&1 || true

    if [[ ! -f "${certDir}/fullchain.pem" || ! -f "${certDir}/privkey.pem" ]]; then
        echo -e "${red}Certificate files not found after installation${plain}"
        return 1
    fi

    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1
    chmod 600 ${certDir}/privkey.pem 2> /dev/null
    chmod 644 ${certDir}/fullchain.pem 2> /dev/null

    ${xui_folder}/x-ui cert -webCert "${certDir}/fullchain.pem" -webCertKey "${certDir}/privkey.pem"
    echo -e "${green}IP certificate installed and configured successfully!${plain}"
    return 0
}

ssl_cert_issue() {
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep 'webBasePath:' | awk -F': ' '{print $2}' | tr -d '[:space:]' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep 'port:' | awk -F': ' '{print $2}' | tr -d '[:space:]')

    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        echo "acme.sh could not be found. Installing now..."
        cd ~ || return 1
        curl -s https://get.acme.sh | sh
        if [ $? -ne 0 ]; then echo -e "${red}Failed to install acme.sh${plain}"; return 1; fi
    fi

    local domain=""
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        domain="${XUI_DOMAIN// /}"
        if [[ -z "$domain" ]] || ! is_domain "$domain"; then return 1; fi
    else
        while true; do
            read -rp "Please enter your domain name: " domain
            domain="${domain// /}"
            if [[ -z "$domain" ]] || ! is_domain "$domain"; then continue; fi
            break
        done
    fi
    SSL_ISSUED_DOMAIN="${domain}"

    local cert_exists=0
    if ~/.acme.sh/acme.sh --list 2> /dev/null | awk '{print $1}' | grep -Fxq "${domain}"; then
        local acmeCertDir=""
        if [[ -s ~/.acme.sh/${domain}_ecc/fullchain.cer && -s ~/.acme.sh/${domain}_ecc/${domain}.key ]]; then
            acmeCertDir=~/.acme.sh/${domain}_ecc
        elif [[ -s ~/.acme.sh/${domain}/fullchain.cer && -s ~/.acme.sh/${domain}/${domain}.key ]]; then
            acmeCertDir=~/.acme.sh/${domain}
        fi
        if [[ -n "${acmeCertDir}" ]]; then cert_exists=1; else rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc; fi
    fi

    certPath="/root/cert/${domain}"
    mkdir -p "$certPath"

    local WebPort=80
    prompt_or_default WebPort "Please choose which port to use (default is 80): " "80" XUI_ACME_HTTP_PORT
    
    systemctl stop x-ui 2> /dev/null || rc-service x-ui stop 2> /dev/null

    if [[ ${cert_exists} -eq 0 ]]; then
        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
        [[ -n "${XUI_ACME_EMAIL:-}" ]] && ~/.acme.sh/acme.sh --register-account -m "${XUI_ACME_EMAIL}" > /dev/null 2>&1
        ~/.acme.sh/acme.sh --issue -d ${domain} $(acme_listen_flag) --standalone --httpport ${WebPort} --force
        if [ $? -ne 0 ]; then
            rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc
            systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null
            return 1
        fi
    fi

    reloadCmd="systemctl restart x-ui || rc-service x-ui restart"
    local installOutput=""
    installOutput=$(~/.acme.sh/acme.sh --installcert -d ${domain} --key-file /root/cert/${domain}/privkey.pem --fullchain-file /root/cert/${domain}/fullchain.pem --reloadcmd "${reloadCmd}" 2>&1)
    local installRc=$?
    
    local installWroteFiles=0
    if echo "${installOutput}" | grep -q "Installing key to:" && echo "${installOutput}" | grep -q "Installing full chain to:"; then installWroteFiles=1; fi

    if [[ -f "/root/cert/${domain}/privkey.pem" && -f "/root/cert/${domain}/fullchain.pem" && (${installRc} -eq 0 || ${installWroteFiles} -eq 1) ]]; then
        ~/.acme.sh/acme.sh --upgrade --auto-upgrade
        chmod 600 $certPath/privkey.pem 2> /dev/null
        chmod 644 $certPath/fullchain.pem 2> /dev/null
    else
        if [[ ${cert_exists} -eq 0 ]]; then rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc; fi
        systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null
        return 1
    fi

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
    echo -e "${green}1.${plain} Let's Encrypt for Domain (90-day validity, auto-renews)"
    echo -e "${green}2.${plain} Let's Encrypt for IP Address (6-day validity, auto-renews)"
    echo -e "${green}3.${plain} Custom SSL Certificate (Path to existing files)"
    echo -e "${green}4.${plain} Skip SSL (advanced)"
    
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        case "${XUI_SSL_MODE:-none}" in
            domain) ssl_choice="1" ;;
            ip) ssl_choice="2" ;;
            none | "") ssl_choice="4" ;;
            *) ssl_choice="4" ;;
        esac
    else
        read -rp "Choose an option (default 2 for IP): " ssl_choice
        ssl_choice="${ssl_choice// /}"
        if [[ "$ssl_choice" != "1" && "$ssl_choice" != "3" && "$ssl_choice" != "4" ]]; then ssl_choice="2"; fi
    fi

    case "$ssl_choice" in
        1)
            if ssl_cert_issue; then
                SSL_HOST="${SSL_ISSUED_DOMAIN:-$server_ip}"
            else
                SSL_HOST="${server_ip}"
            fi
            ;;
        2)
            local ipv6_addr=""
            prompt_or_default ipv6_addr "Do you have an IPv6 address to include? (leave empty to skip): " "" XUI_SSL_IPV6
            systemctl stop x-ui > /dev/null 2>&1 || rc-service x-ui stop > /dev/null 2>&1
            setup_ip_certificate "${server_ip}" "${ipv6_addr}"
            SSL_HOST="${server_ip}"
            ;;
        3)
            local custom_cert="" custom_key="" custom_domain=""
            read -rp "Please enter domain name certificate issued for: " custom_domain
            read -rp "Input certificate path: " custom_cert
            read -rp "Input private key path: " custom_key
            ${xui_folder}/x-ui cert -webCert "$custom_cert" -webCertKey "$custom_key" > /dev/null 2>&1
            SSL_HOST="${custom_domain:-$server_ip}"
            systemctl restart x-ui > /dev/null 2>&1 || rc-service x-ui restart > /dev/null 2>&1
            ;;
        4)
            SSL_SCHEME="http"
            SSL_HOST="${server_ip}"
            ;;
    esac
}

config_after_install() {
    local existing_hasDefaultCredential=$(${xui_folder}/x-ui setting -show true | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    local existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
    local server_ip=""
    
    server_ip=$(curl -s -w "\n%{http_code}" --max-time 3 "https://v4.api.ipinfo.io/ip" 2> /dev/null | head -n-1 | tr -d '[:space:]"')
    if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then server_ip="${XUI_SERVER_IP:-127.0.0.1}"; fi

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath="${XUI_WEB_BASE_PATH:-$(gen_random_string 18)}"
            local config_username="${XUI_USERNAME:-$(gen_random_string 10)}"
            local config_password="${XUI_PASSWORD:-$(gen_random_string 10)}"
            local config_port="${XUI_PANEL_PORT:-$(shuf -i 1024-62000 -n 1)}"
            local db_label="SQLite"

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

install_x-ui() {
    # 1. Определение версии (по умолчанию v3.4.2 если не удается получить)
    if [ $# == 0 ]; then
        tag_version=$(curl -Ls --retry 5 --retry-delay 3 --connect-timeout 15 --max-time 60 "https://api.github.com/repos/KimaruBs/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${yellow}Failed to fetch latest version via API, falling back to v3.4.2${plain}"
            tag_version="v3.4.2"
        fi
        echo -e "Got x-ui version: ${tag_version}, beginning the installation..."
    else
        tag_version=$1
        echo -e "Beginning to install x-ui ${tag_version}"
    fi

    # 2. Остановка и очистка старой версии
    if [[ -e ${xui_folder}/ ]]; then
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop > /dev/null 2>&1
        else
            systemctl stop x-ui > /dev/null 2>&1
        fi
        pkill -f 'mtg-linux-[^ ]* run ' > /dev/null 2>&1 || true
        rm ${xui_folder}/ -rf
    fi

    mkdir -p ${xui_folder}
    cd ${xui_folder}

    # 3. Прямая загрузка исполняемого файла для вашей архитектуры
    local binary_name="x-ui-linux-$(arch)"
    local url="https://github.com/KimaruBs/3x-ui/releases/download/${tag_version}/${binary_name}"
    
    echo -e "Downloading x-ui executable from: ${url}"
    curl -fLR --retry 5 --retry-delay 3 --connect-timeout 15 --max-time 300 -o x-ui "${url}"
    
    if [[ $? -ne 0 ]]; then
        echo -e "${red}Download x-ui ${tag_version} failed. Make sure ${binary_name} exists on GitHub release page.${plain}"
        exit 1
    fi
    
    chmod +x x-ui
    install_xray

    # 4. Скачивание CLI-скрипта x-ui.sh
    curl -fLRo /usr/bin/x-ui-temp https://raw.githubusercontent.com/KimaruBs/3x-ui/main/x-ui.sh
    if [[ $? -ne 0 ]]; then
        echo -e "${red}Failed to download x-ui.sh${plain}"
        exit 1
    fi
    mv -f /usr/bin/x-ui-temp /usr/bin/x-ui
    chmod +x /usr/bin/x-ui
    
    mkdir -p /var/log/x-ui
    config_after_install

    if [ -d "/etc/.git" ]; then
        if [ -f "/etc/.gitignore" ]; then
            if ! grep -q "x-ui/x-ui.db" "/etc/.gitignore"; then
                echo "x-ui/x-ui.db" >> "/etc/.gitignore"
            fi
        else
            echo "x-ui/x-ui.db" > "/etc/.gitignore"
        fi
    fi

    # 5. Установка Systemd/RC сервисов
    if [[ $release == "alpine" ]]; then
        curl -fLRo /etc/init.d/x-ui https://raw.githubusercontent.com/KimaruBs/3x-ui/main/x-ui.rc
        if [[ $? -ne 0 ]]; then echo -e "${red}Failed to download x-ui.rc${plain}"; exit 1; fi
        chmod +x /etc/init.d/x-ui
        rc-update add x-ui
        rc-service x-ui start
    else
        echo -e "${yellow}Downloading systemd service file from GitHub...${plain}"
        case "${release}" in
            ubuntu | debian | armbian)
                curl -fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/KimaruBs/3x-ui/main/x-ui.service.debian > /dev/null 2>&1
                ;;
            arch | manjaro | parch)
                curl -fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/KimaruBs/3x-ui/main/x-ui.service.arch > /dev/null 2>&1
                ;;
            *)
                curl -fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/KimaruBs/3x-ui/main/x-ui.service.rhel > /dev/null 2>&1
                ;;
        esac

        if [[ $? -ne 0 ]]; then
            echo -e "${red}Failed to install x-ui.service from GitHub${plain}"
            exit 1
        fi

        echo -e "${green}Setting up systemd unit...${plain}"
        chown root:root ${xui_service}/x-ui.service > /dev/null 2>&1
        chmod 644 ${xui_service}/x-ui.service > /dev/null 2>&1
        systemctl daemon-reload
        systemctl enable x-ui
        systemctl start x-ui
    fi

    setup_fail2ban

    echo -e "${green}x-ui ${tag_version}${plain} installation finished, it is running now..."
}

echo -e "${green}Running...${plain}"
install_base
install_x-ui $1
