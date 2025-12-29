#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

# Don't edit this config
b_source="${BASH_SOURCE[0]}"
while [ -h "$b_source" ]; do
    b_dir="$(cd -P "$(dirname "$b_source")" >/dev/null 2>&1 && pwd || pwd -P)"
    b_source="$(readlink "$b_source")"
    [[ $b_source != /* ]] && b_source="$b_dir/$b_source"
done
cur_dir="$(cd -P "$(dirname "$b_source")" >/dev/null 2>&1 && pwd || pwd -P)"
script_name=$(basename "$0")

# Check command exist function
_command_exists() {
    type "$1" &>/dev/null
}

# Fail, log and exit script function
_fail() {
    local msg=${1}
    echo -e "${red}${msg}${plain}"
    exit 2
}

# check root
[[ $EUID -ne 0 ]] && _fail "FATAL ERROR: Please run this script with root privilege."

if _command_exists wget; then
    wget_bin=$(which wget)
else
    _fail "ERROR: Command 'wget' not found."
fi

if _command_exists curl; then
    curl_bin=$(which curl)
else
    _fail "ERROR: Command 'curl' not found."
fi

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
    elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    _fail "Failed to check the system OS, please contact the author!"
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
        *) echo -e "${red}Unsupported CPU architecture!${plain}" && rm -f "${cur_dir}/${script_name}" >/dev/null 2>&1 && exit 2;;
    esac
}

echo "Arch: $(arch)"

# Simple helpers
is_ipv4() {
    [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] && return 0 || return 1
}
is_ipv6() {
    [[ "$1" =~ : ]] && return 0 || return 1
}
is_ip() {
    is_ipv4 "$1" || is_ipv6 "$1"
}
is_domain() {
    [[ "$1" =~ ^([A-Za-z0-9](-*[A-Za-z0-9])*\.)+[A-Za-z]{2,}$ ]] && return 0 || return 1
}

gen_random_string() {
    local length="$1"
    local random_string=$(LC_ALL=C tr -dc 'a-zA-Z0-9' </dev/urandom | fold -w "$length" | head -n 1)
    echo "$random_string"
}

install_base() {
    echo -e "${green}Updating and install dependency packages...${plain}"
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update >/dev/null 2>&1 && apt-get install -y -q wget curl tar tzdata openssl socat >/dev/null 2>&1
        ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf -y update >/dev/null 2>&1 && dnf install -y -q wget curl tar tzdata openssl socat >/dev/null 2>&1
        ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum -y update >/dev/null 2>&1 && yum install -y -q wget curl tar tzdata openssl socat >/dev/null 2>&1
            else
                dnf -y update >/dev/null 2>&1 && dnf install -y -q wget curl tar tzdata openssl socat >/dev/null 2>&1
            fi
        ;;
        arch | manjaro | parch)
            pacman -Syu >/dev/null 2>&1 && pacman -Syu --noconfirm wget curl tar tzdata openssl socat >/dev/null 2>&1
        ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper refresh >/dev/null 2>&1 && zypper -q install -y wget curl tar timezone openssl socat >/dev/null 2>&1
        ;;
        alpine)
            apk update >/dev/null 2>&1 && apk add wget curl tar tzdata openssl socat >/dev/null 2>&1
        ;;
        *)
            apt-get update >/dev/null 2>&1 && apt install -y -q wget curl tar tzdata openssl socat >/dev/null 2>&1
        ;;
    esac
}

install_acme() {
    echo -e "${green}Installing acme.sh for SSL certificate management...${plain}"
    cd ~ || return 1
    curl -s https://get.acme.sh | sh >/dev/null 2>&1
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
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
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
    
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt >/dev/null 2>&1
    ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport 80 --force
    
    if [ $? -ne 0 ]; then
        echo -e "${yellow}Failed to issue certificate for ${domain}${plain}"
        echo -e "${yellow}Please ensure port 80 is open and try again later with: x-ui${plain}"
        rm -rf ~/.acme.sh/${domain} 2>/dev/null
        rm -rf "$certPath" 2>/dev/null
        return 1
    fi
    
    # Install certificate
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem \
        --reloadcmd "systemctl restart x-ui" >/dev/null 2>&1
    
    if [ $? -ne 0 ]; then
        echo -e "${yellow}Failed to install certificate${plain}"
        return 1
    fi
    
    # Enable auto-renew
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade >/dev/null 2>&1
    chmod 755 $certPath/* 2>/dev/null
    
    # Set certificate for panel
    local webCertFile="/root/cert/${domain}/fullchain.pem"
    local webKeyFile="/root/cert/${domain}/privkey.pem"
    
    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
        /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile" >/dev/null 2>&1
        echo -e "${green}SSL certificate installed and configured successfully!${plain}"
        return 0
    else
        echo -e "${yellow}Certificate files not found${plain}"
        return 1
    fi
}

# Fallback: generate a self-signed certificate (not publicly trusted)
setup_self_signed_certificate() {
    local name="$1"   # domain or IP to place in SAN
    local certDir="/root/cert/selfsigned"

    echo -e "${yellow}Generating a self-signed certificate (not publicly trusted)...${plain}"

    mkdir -p "$certDir"

    local sanExt=""
    if is_ip "$name"; then
        sanExt="IP:${name}"
    else
        sanExt="DNS:${name}"
    fi

    # Try -addext; fallback to config if not supported
    openssl req -x509 -nodes -newkey rsa:2048 -days 365 \
        -keyout "${certDir}/privkey.pem" \
        -out "${certDir}/fullchain.pem" \
        -subj "/CN=${name}" \
        -addext "subjectAltName=${sanExt}" >/dev/null 2>&1

    if [[ $? -ne 0 ]]; then
        local tmpCfg="${certDir}/openssl.cnf"
        cat > "$tmpCfg" <<EOF
[req]
distinguished_name=req_distinguished_name
req_extensions=v3_req
[req_distinguished_name]
[v3_req]
subjectAltName=${sanExt}
EOF
        openssl req -x509 -nodes -newkey rsa:2048 -days 365 \
            -keyout "${certDir}/privkey.pem" \
            -out "${certDir}/fullchain.pem" \
            -subj "/CN=${name}" \
            -config "$tmpCfg" -extensions v3_req >/dev/null 2>&1
        rm -f "$tmpCfg"
    fi

    if [[ ! -f "${certDir}/fullchain.pem" || ! -f "${certDir}/privkey.pem" ]]; then
        echo -e "${red}Failed to generate self-signed certificate${plain}"
        return 1
    fi

    chmod 755 ${certDir}/* 2>/dev/null
    /usr/local/x-ui/x-ui cert -webCert "${certDir}/fullchain.pem" -webCertKey "${certDir}/privkey.pem" >/dev/null 2>&1
    echo -e "${yellow}Self-signed certificate configured. Browsers will show a warning.${plain}"
    return 0
}
# Comprehensive manual SSL certificate issuance via acme.sh
ssl_cert_issue() {
    local existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep 'webBasePath:' | awk -F': ' '{print $2}' | tr -d '[:space:]' | sed 's#^/##')
    local existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep 'port:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
    
    # check for acme.sh first
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
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
    while true; do
        read -rp "Please enter your domain name: " domain
        domain="${domain// /}"  # Trim whitespace
        
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
    echo -e "${green}Your domain is: ${domain}, checking it...${plain}"

    # check if there already exists a certificate
    local currentCert=$(~/.acme.sh/acme.sh --list | tail -1 | awk '{print $1}')
    if [ "${currentCert}" == "${domain}" ]; then
        local certInfo=$(~/.acme.sh/acme.sh --list)
        echo -e "${red}System already has certificates for this domain. Cannot issue again.${plain}"
        echo -e "${yellow}Current certificate details:${plain}"
        echo "$certInfo"
        return 1
    else
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
    read -rp "Please choose which port to use (default is 80): " WebPort
    if [[ ${WebPort} -gt 65535 || ${WebPort} -lt 1 ]]; then
        echo -e "${yellow}Your input ${WebPort} is invalid, will use default port 80.${plain}"
        WebPort=80
    fi
    echo -e "${green}Will use port: ${WebPort} to issue certificates. Please make sure this port is open.${plain}"

    # Stop panel temporarily
    echo -e "${yellow}Stopping panel temporarily...${plain}"
    systemctl stop x-ui 2>/dev/null || rc-service x-ui stop 2>/dev/null

    # issue the certificate
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
    ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport ${WebPort} --force
    if [ $? -ne 0 ]; then
        echo -e "${red}Issuing certificate failed, please check logs.${plain}"
        rm -rf ~/.acme.sh/${domain}
        systemctl start x-ui 2>/dev/null || rc-service x-ui start 2>/dev/null
        return 1
    else
        echo -e "${green}Issuing certificate succeeded, installing certificates...${plain}"
    fi

    # Setup reload command
    reloadCmd="systemctl restart x-ui || rc-service x-ui restart"
    echo -e "${green}Default --reloadcmd for ACME is: ${yellow}systemctl restart x-ui || rc-service x-ui restart${plain}"
    echo -e "${green}This command will run on every certificate issue and renew.${plain}"
    read -rp "Would you like to modify --reloadcmd for ACME? (y/n): " setReloadcmd
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
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem --reloadcmd "${reloadCmd}"

    if [ $? -ne 0 ]; then
        echo -e "${red}Installing certificate failed, exiting.${plain}"
        rm -rf ~/.acme.sh/${domain}
        systemctl start x-ui 2>/dev/null || rc-service x-ui start 2>/dev/null
        return 1
    else
        echo -e "${green}Installing certificate succeeded, enabling auto renew...${plain}"
    fi

    # enable auto-renew
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        echo -e "${yellow}Auto renew setup had issues, certificate details:${plain}"
        ls -lah /root/cert/${domain}/
        chmod 755 $certPath/*
    else
        echo -e "${green}Auto renew succeeded, certificate details:${plain}"
        ls -lah /root/cert/${domain}/
        chmod 755 $certPath/*
    fi

    # Restart panel
    systemctl start x-ui 2>/dev/null || rc-service x-ui start 2>/dev/null

    # Prompt user to set panel paths after successful certificate installation
    read -rp "Would you like to set this certificate for the panel? (y/n): " setPanel
    if [[ "$setPanel" == "y" || "$setPanel" == "Y" ]]; then
        local webCertFile="/root/cert/${domain}/fullchain.pem"
        local webKeyFile="/root/cert/${domain}/privkey.pem"

        if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
            /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
            echo -e "${green}Certificate paths set for the panel${plain}"
            echo -e "${green}Certificate File: $webCertFile${plain}"
            echo -e "${green}Private Key File: $webKeyFile${plain}"
            echo ""
            echo -e "${green}Access URL: https://${domain}:${existing_port}/${existing_webBasePath}${plain}"
            echo -e "${yellow}Panel will restart to apply SSL certificate...${plain}"
            systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null
        else
            echo -e "${red}Error: Certificate or private key file not found for domain: $domain.${plain}"
        fi
    else
        echo -e "${yellow}Skipping panel path setting.${plain}"
    fi
    
    return 0
}
# Unified interactive SSL setup (domain or self-signed)
# Sets global `SSL_HOST` to the chosen domain/IP
prompt_and_setup_ssl() {
    local panel_port="$1"
    local web_base_path="$2"   # expected without leading slash
    local server_ip="$3"

    local ssl_choice=""

    echo -e "${yellow}Choose SSL certificate setup method:${plain}"
    echo -e "${green}1.${plain} Let's Encrypt (domain required, recommended)"
    echo -e "${green}2.${plain} Self-signed certificate (for testing/local use)"
    read -rp "Choose an option (default 2): " ssl_choice
    ssl_choice="${ssl_choice// /}"  # Trim whitespace
    
    # Default to 2 (self-signed) if not 1
    if [[ "$ssl_choice" != "1" ]]; then
        ssl_choice="2"
    fi

    case "$ssl_choice" in
    1)
        # User chose Let's Encrypt domain option
        echo -e "${green}Using ssl_cert_issue() for comprehensive domain setup...${plain}"
        ssl_cert_issue
        # Extract the domain that was used from the certificate
        local cert_domain=$(~/.acme.sh/acme.sh --list 2>/dev/null | tail -1 | awk '{print $1}')
        if [[ -n "${cert_domain}" ]]; then
            SSL_HOST="${cert_domain}"
            echo -e "${green}✓ SSL certificate configured successfully with domain: ${cert_domain}${plain}"
        else
            echo -e "${yellow}SSL setup may have completed, but domain extraction failed${plain}"
            SSL_HOST="${server_ip}"
        fi
        ;;
    2)
        # User chose self-signed option
        # Stop panel if running
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop >/dev/null 2>&1
        else
            systemctl stop x-ui >/dev/null 2>&1
        fi
        echo -e "${yellow}Using server IP for self-signed certificate: ${server_ip}${plain}"
        setup_self_signed_certificate "${server_ip}"
        if [ $? -eq 0 ]; then
            SSL_HOST="${server_ip}"
            echo -e "${green}✓ Self-signed SSL configured successfully${plain}"
        else
            echo -e "${red}✗ Self-signed SSL setup failed${plain}"
            SSL_HOST="${server_ip}"
        fi
        # Start panel after SSL is configured
        if [[ $release == "alpine" ]]; then
            rc-service x-ui start >/dev/null 2>&1
        else
            systemctl start x-ui >/dev/null 2>&1
        fi
        ;;
    0)
        echo -e "${yellow}Skipping SSL setup${plain}"
        SSL_HOST="${server_ip}"
        ;;
    *)
        echo -e "${red}Invalid option. Skipping SSL setup.${plain}"
        SSL_HOST="${server_ip}"
        ;;
    esac
}

config_after_update() {
    echo -e "${yellow}x-ui settings:${plain}"
    /usr/local/x-ui/x-ui setting -show true
    /usr/local/x-ui/x-ui migrate
    
    # Properly detect empty cert by checking if cert: line exists and has content after it
    local existing_cert=$(/usr/local/x-ui/x-ui setting -getCert true 2>/dev/null | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
    local existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    local existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##')
    
    # Get server IP
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
        server_ip=$(${curl_bin} -s --max-time 3 "${ip_address}" 2>/dev/null | tr -d '[:space:]')
        if [[ -n "${server_ip}" ]]; then
            break
        fi
    done
    
    # Handle missing/short webBasePath
    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        echo -e "${yellow}WebBasePath is missing or too short. Generating a new one...${plain}"
        local config_webBasePath=$(gen_random_string 18)
        /usr/local/x-ui/x-ui setting -webBasePath "${config_webBasePath}"
        existing_webBasePath="${config_webBasePath}"
        echo -e "${green}New WebBasePath: ${config_webBasePath}${plain}"
    fi
    
    # Check and prompt for SSL if missing
    if [[ -z "$existing_cert" ]]; then
        echo ""
        echo -e "${red}═══════════════════════════════════════════${plain}"
        echo -e "${red}      ⚠ NO SSL CERTIFICATE DETECTED ⚠     ${plain}"
        echo -e "${red}═══════════════════════════════════════════${plain}"
        echo -e "${yellow}For security, SSL certificate is MANDATORY for all panels.${plain}"
        echo -e "${yellow}Let's Encrypt requires a domain name; IP certs are not issued. Use self-signed for IP.${plain}"
        echo ""
        
        if [[ -z "${server_ip}" ]]; then
            echo -e "${red}Failed to detect server IP${plain}"
            echo -e "${yellow}Please configure SSL manually using: x-ui${plain}"
            return
        fi
        
        # Prompt and setup SSL (domain or self-signed)
        prompt_and_setup_ssl "${existing_port}" "${existing_webBasePath}" "${server_ip}"
        
        echo ""
        echo -e "${green}═══════════════════════════════════════════${plain}"
        echo -e "${green}     Panel Access Information              ${plain}"
        echo -e "${green}═══════════════════════════════════════════${plain}"
        echo -e "${green}Access URL: https://${SSL_HOST}:${existing_port}/${existing_webBasePath}${plain}"
        echo -e "${green}═══════════════════════════════════════════${plain}"
        echo -e "${yellow}⚠ SSL Certificate: Enabled and configured${plain}"
    else
        echo -e "${green}SSL certificate is already configured${plain}"
        # Show access URL with existing certificate
        local cert_domain=$(basename "$(dirname "$existing_cert")")
        echo ""
        echo -e "${green}═══════════════════════════════════════════${plain}"
        echo -e "${green}     Panel Access Information              ${plain}"
        echo -e "${green}═══════════════════════════════════════════${plain}"
        echo -e "${green}Access URL: https://${cert_domain}:${existing_port}/${existing_webBasePath}${plain}"
        echo -e "${green}═══════════════════════════════════════════${plain}"
    fi
}

update_x-ui() {
    cd /usr/local/
    
    if [ -f "/usr/local/x-ui/x-ui" ]; then
        current_xui_version=$(/usr/local/x-ui/x-ui -v)
        echo -e "${green}Current x-ui version: ${current_xui_version}${plain}"
    else
        _fail "ERROR: Current x-ui version: unknown"
    fi
    
    echo -e "${green}Downloading new x-ui version...${plain}"
    
    tag_version=$(${curl_bin} -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ ! -n "$tag_version" ]]; then
        echo -e "${yellow}Trying to fetch version with IPv4...${plain}"
        tag_version=$(${curl_bin} -4 -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$tag_version" ]]; then
            _fail "ERROR: Failed to fetch x-ui version, it may be due to GitHub API restrictions, please try it later"
        fi
    fi
    echo -e "Got x-ui latest version: ${tag_version}, beginning the installation..."
    ${wget_bin} -N -O /usr/local/x-ui-linux-$(arch).tar.gz https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz 2>/dev/null
    if [[ $? -ne 0 ]]; then
        echo -e "${yellow}Trying to fetch version with IPv4...${plain}"
        ${wget_bin} --inet4-only -N -O /usr/local/x-ui-linux-$(arch).tar.gz https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz 2>/dev/null
        if [[ $? -ne 0 ]]; then
            _fail "ERROR: Failed to download x-ui, please be sure that your server can access GitHub"
        fi
    fi
    
    if [[ -e /usr/local/x-ui/ ]]; then
        echo -e "${green}Stopping x-ui...${plain}"
        if [[ $release == "alpine" ]]; then
            if [ -f "/etc/init.d/x-ui" ]; then
                rc-service x-ui stop >/dev/null 2>&1
                rc-update del x-ui >/dev/null 2>&1
                echo -e "${green}Removing old service unit version...${plain}"
                rm -f /etc/init.d/x-ui >/dev/null 2>&1
            else
                rm x-ui-linux-$(arch).tar.gz -f >/dev/null 2>&1
                _fail "ERROR: x-ui service unit not installed."
            fi
        else
            if [ -f "/etc/systemd/system/x-ui.service" ]; then
                systemctl stop x-ui >/dev/null 2>&1
                systemctl disable x-ui >/dev/null 2>&1
                echo -e "${green}Removing old systemd unit version...${plain}"
                rm /etc/systemd/system/x-ui.service -f >/dev/null 2>&1
                systemctl daemon-reload >/dev/null 2>&1
            else
                rm x-ui-linux-$(arch).tar.gz -f >/dev/null 2>&1
                _fail "ERROR: x-ui systemd unit not installed."
            fi
        fi
        echo -e "${green}Removing old x-ui version...${plain}"
        rm /usr/bin/x-ui -f >/dev/null 2>&1
        rm /usr/local/x-ui/x-ui.service -f >/dev/null 2>&1
        rm /usr/local/x-ui/x-ui -f >/dev/null 2>&1
        rm /usr/local/x-ui/x-ui.sh -f >/dev/null 2>&1
        echo -e "${green}Removing old xray version...${plain}"
        rm /usr/local/x-ui/bin/xray-linux-amd64 -f >/dev/null 2>&1
        echo -e "${green}Removing old README and LICENSE file...${plain}"
        rm /usr/local/x-ui/bin/README.md -f >/dev/null 2>&1
        rm /usr/local/x-ui/bin/LICENSE -f >/dev/null 2>&1
    else
        rm x-ui-linux-$(arch).tar.gz -f >/dev/null 2>&1
        _fail "ERROR: x-ui not installed."
    fi
    
    echo -e "${green}Installing new x-ui version...${plain}"
    tar zxvf x-ui-linux-$(arch).tar.gz >/dev/null 2>&1
    rm x-ui-linux-$(arch).tar.gz -f >/dev/null 2>&1
    cd x-ui >/dev/null 2>&1
    chmod +x x-ui >/dev/null 2>&1
    
    # Check the system's architecture and rename the file accordingly
    if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm >/dev/null 2>&1
        chmod +x bin/xray-linux-arm >/dev/null 2>&1
    fi
    
    chmod +x x-ui bin/xray-linux-$(arch) >/dev/null 2>&1
    
    echo -e "${green}Downloading and installing x-ui.sh script...${plain}"
    ${wget_bin} -O /usr/bin/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.sh >/dev/null 2>&1
    if [[ $? -ne 0 ]]; then
        echo -e "${yellow}Trying to fetch x-ui with IPv4...${plain}"
        ${wget_bin} --inet4-only -O /usr/bin/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.sh >/dev/null 2>&1
        if [[ $? -ne 0 ]]; then
            _fail "ERROR: Failed to download x-ui.sh script, please be sure that your server can access GitHub"
        fi
    fi
    
    chmod +x /usr/local/x-ui/x-ui.sh >/dev/null 2>&1
    chmod +x /usr/bin/x-ui >/dev/null 2>&1
    
    echo -e "${green}Changing owner...${plain}"
    chown -R root:root /usr/local/x-ui >/dev/null 2>&1
    
    if [ -f "/usr/local/x-ui/bin/config.json" ]; then
        echo -e "${green}Changing on config file permissions...${plain}"
        chmod 640 /usr/local/x-ui/bin/config.json >/dev/null 2>&1
    fi
    
    if [[ $release == "alpine" ]]; then
        echo -e "${green}Downloading and installing startup unit x-ui.rc...${plain}"
        ${wget_bin} -O /etc/init.d/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.rc >/dev/null 2>&1
        if [[ $? -ne 0 ]]; then
            ${wget_bin} --inet4-only -O /etc/init.d/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.rc >/dev/null 2>&1
            if [[ $? -ne 0 ]]; then
                _fail "ERROR: Failed to download startup unit x-ui.rc, please be sure that your server can access GitHub"
            fi
        fi
        chmod +x /etc/init.d/x-ui >/dev/null 2>&1
        chown root:root /etc/init.d/x-ui >/dev/null 2>&1
        rc-update add x-ui >/dev/null 2>&1
        rc-service x-ui start >/dev/null 2>&1
    else
        echo -e "${green}Installing systemd unit...${plain}"
        cp -f x-ui.service /etc/systemd/system/ >/dev/null 2>&1
        chown root:root /etc/systemd/system/x-ui.service >/dev/null 2>&1
        systemctl daemon-reload >/dev/null 2>&1
        systemctl enable x-ui >/dev/null 2>&1
        systemctl start x-ui >/dev/null 2>&1
    fi
    
    config_after_update
    
    echo -e "${green}x-ui ${tag_version}${plain} updating finished, it is running now..."
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
update_x-ui $1
