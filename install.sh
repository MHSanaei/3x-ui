#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

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
        *) echo -e "${green}Unsupported CPU architecture! ${plain}" && rm -f install.sh && exit 1 ;;
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

install_base() {
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q wget curl tar tzdata
        ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf -y update && dnf install -y -q wget curl tar tzdata
        ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum -y update && yum install -y wget curl tar tzdata
            else
                dnf -y update && dnf install -y -q wget curl tar tzdata
            fi
        ;;
        arch | manjaro | parch)
            pacman -Syu && pacman -Syu --noconfirm wget curl tar tzdata
        ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper refresh && zypper -q install -y wget curl tar timezone
        ;;
        alpine)
            apk update && apk add wget curl tar tzdata
        ;;
        *)
            apt-get update && apt-get install -y -q wget curl tar tzdata
        ;;
    esac
}

gen_random_string() {
    local length="$1"
    local random_string=$(LC_ALL=C tr -dc 'a-zA-Z0-9' </dev/urandom | fold -w "$length" | head -n 1)
    echo "$random_string"
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
    
    # Install socat
    echo -e "${green}Installing socat...${plain}"
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update >/dev/null 2>&1 && apt-get install socat -y >/dev/null 2>&1
        ;;
    fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
        dnf -y update >/dev/null 2>&1 && dnf -y install socat >/dev/null 2>&1
        ;;
    centos)
        if [[ "${VERSION_ID}" =~ ^7 ]]; then
            yum -y update >/dev/null 2>&1 && yum -y install socat >/dev/null 2>&1
        else
            dnf -y update >/dev/null 2>&1 && dnf -y install socat >/dev/null 2>&1
        fi
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm socat >/dev/null 2>&1
        ;;
    opensuse-tumbleweed | opensuse-leap)
        zypper refresh >/dev/null 2>&1 && zypper -q install -y socat >/dev/null 2>&1
        ;;
    alpine)
        apk add socat curl openssl >/dev/null 2>&1
        ;;
    esac
    
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

    # Ensure openssl is available
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update >/dev/null 2>&1 && apt-get install -y openssl >/dev/null 2>&1
        ;;
    fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
        dnf -y update >/dev/null 2>&1 && dnf -y install openssl >/dev/null 2>&1
        ;;
    centos)
        if [[ "${VERSION_ID}" =~ ^7 ]]; then
            yum -y update >/dev/null 2>&1 && yum -y install openssl >/dev/null 2>&1
        else
            dnf -y update >/dev/null 2>&1 && dnf -y install openssl >/dev/null 2>&1
        fi
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm openssl >/dev/null 2>&1
        ;;
    opensuse-tumbleweed | opensuse-leap)
        zypper refresh >/dev/null 2>&1 && zypper -q install -y openssl >/dev/null 2>&1
        ;;
    alpine)
        apk add openssl >/dev/null 2>&1
        ;;
    esac

    mkdir -p "$certDir"

    local sanExt=""
    if is_ip "$name"; then
        sanExt="IP:${name}"
    else
        sanExt="DNS:${name}"
    fi

    # Use -addext if supported; fallback to config file if needed
    openssl req -x509 -nodes -newkey rsa:2048 -days 365 \
        -keyout "${certDir}/privkey.pem" \
        -out "${certDir}/fullchain.pem" \
        -subj "/CN=${name}" \
        -addext "subjectAltName=${sanExt}" >/dev/null 2>&1

    if [[ $? -ne 0 ]]; then
        # Fallback via temporary config file (for older OpenSSL versions)
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

config_after_install() {
    local existing_hasDefaultCredential=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')
    local existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    local existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    # Properly detect empty cert by checking if cert: line exists and has content after it
    local existing_cert=$(/usr/local/x-ui/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
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
        server_ip=$(curl -s --max-time 3 "${ip_address}" 2>/dev/null | tr -d '[:space:]')
        if [[ -n "${server_ip}" ]]; then
            break
        fi
    done
    
    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath=$(gen_random_string 18)
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)
            
            read -rp "Would you like to customize the Panel Port settings? (If not, a random port will be applied) [y/n]: " config_confirm
            if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
                read -rp "Please set up the panel port: " config_port
                echo -e "${yellow}Your Panel Port is: ${config_port}${plain}"
            else
                local config_port=$(shuf -i 1024-62000 -n 1)
                echo -e "${yellow}Generated random port: ${config_port}${plain}"
            fi
            
            /usr/local/x-ui/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"
            
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     SSL Certificate Setup (MANDATORY)     ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}For security, SSL certificate is required for all panels.${plain}"
            echo -e "${yellow}Let's Encrypt requires a domain name (IP certificates are not issued).${plain}"
            echo ""

            local ssl_success=false
            local ssl_domain=""

            # Prompt on fresh install as well
            while [[ "$ssl_success" == false ]]; do
                echo -e "${yellow}Write your domain address or press Enter for self-signed (not trusted).${plain}"
                read -rp "Domain (leave empty for self-signed): " ssl_domain

                if [[ -z "${ssl_domain}" ]]; then
                    ssl_domain="self"
                fi

                if [[ "${ssl_domain}" == "self" || "${ssl_domain}" == "SELF" ]]; then
                    echo -e "${yellow}Using server IP for self-signed certificate: ${server_ip}${plain}"
                    setup_self_signed_certificate "${server_ip}"
                    if [ $? -eq 0 ]; then
                        ssl_domain="${server_ip}"
                        ssl_success=true
                        echo -e "${green}✓ Self-signed SSL configured successfully${plain}"
                    else
                        echo -e "${red}✗ Self-signed SSL setup failed${plain}"
                        read -rp "Press Enter to retry SSL setup..."
                    fi
                    continue
                fi

                if is_ip "${ssl_domain}"; then
                    echo -e "${red}Let's Encrypt does not issue certificates for IP addresses.${plain}"
                    echo -e "${yellow}Please provide a domain, or type 'self' for a self-signed certificate.${plain}"
                    continue
                fi
                if ! is_domain "${ssl_domain}"; then
                    echo -e "${red}Input does not look like a valid domain.${plain}"
                    continue
                fi

                echo -e "${green}Using domain: ${ssl_domain}${plain}"
                echo -e "${yellow}Note: Port 80 must be open and accessible from the internet${plain}"
                read -rp "Press Enter to continue with SSL certificate generation..."

                setup_ssl_certificate "${ssl_domain}" "${server_ip}" "${config_port}" "/${config_webBasePath}"

                if [ $? -eq 0 ]; then
                    ssl_success=true
                    echo -e "${green}✓ SSL certificate configured successfully!${plain}"
                else
                    echo ""
                    echo -e "${red}✗ SSL certificate setup failed${plain}"
                    echo -e "${yellow}Please check:${plain}"
                    echo -e "  - Port 80 is open and not in use"
                    echo -e "  - Server is accessible from the internet"
                    echo -e "  - Domain DNS is correctly configured (A/AAAA record)"
                    echo ""
                    read -rp "Press Enter to retry SSL setup..."
                fi
            done
            
            # Start panel only after SSL is configured
            if [[ $release == "alpine" ]]; then
                rc-service x-ui start >/dev/null 2>&1
            else
                systemctl start x-ui >/dev/null 2>&1
            fi
            
            # Display final credentials and access information
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     Panel Installation Complete!         ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}Username:    ${config_username}${plain}"
            echo -e "${green}Password:    ${config_password}${plain}"
            echo -e "${green}Port:        ${config_port}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "${green}Access URL:  https://${ssl_domain}:${config_port}/${config_webBasePath}${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}⚠ IMPORTANT: Save these credentials securely!${plain}"
            echo -e "${yellow}⚠ SSL Certificate: Enabled and configured${plain}"
        else
            local config_webBasePath=$(gen_random_string 18)
            echo -e "${yellow}WebBasePath is missing or too short. Generating a new one...${plain}"
            /usr/local/x-ui/x-ui setting -webBasePath "${config_webBasePath}"
            echo -e "${green}New WebBasePath: ${config_webBasePath}${plain}"
            echo -e "${green}Access URL: https://${server_ip}:${existing_port}/${config_webBasePath}${plain}"
        fi
    else
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)
            
            echo -e "${yellow}Default credentials detected. Security update required...${plain}"
            /usr/local/x-ui/x-ui setting -username "${config_username}" -password "${config_password}"
            echo -e "Generated new random login credentials:"
            echo -e "###############################################"
            echo -e "${green}Username: ${config_username}${plain}"
            echo -e "${green}Password: ${config_password}${plain}"
            echo -e "###############################################"
        else
            echo -e "${green}Username, Password, and WebBasePath are properly set.${plain}"
        fi

        # Existing install: if no cert configured, prompt user to set domain or self-signed
        # Properly detect empty cert by checking if cert: line exists and has content after it
        existing_cert=$(/usr/local/x-ui/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
        if [[ -z "$existing_cert" ]]; then
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     SSL Certificate Setup (RECOMMENDED)   ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}Let's Encrypt requires a domain name (IP certificates are not issued).${plain}"
            echo ""

            local ssl_success=false
            local ssl_domain=""

            while [[ "$ssl_success" == false ]]; do
                echo -e "${yellow}Write your domain address or press Enter for self-signed (not trusted).${plain}"
                read -rp "Domain (leave empty for self-signed): " ssl_domain

                if [[ -z "${ssl_domain}" ]]; then
                    ssl_domain="self"
                fi

                if [[ "${ssl_domain}" == "self" || "${ssl_domain}" == "SELF" ]]; then
                    # Stop panel if running
                    if [[ $release == "alpine" ]]; then
                        rc-service x-ui stop >/dev/null 2>&1
                    else
                        systemctl stop x-ui >/dev/null 2>&1
                    fi
                    echo -e "${yellow}Using server IP for self-signed certificate: ${server_ip}${plain}"
                    setup_self_signed_certificate "${server_ip}"
                    if [ $? -eq 0 ]; then
                        ssl_domain="${server_ip}"
                        ssl_success=true
                        echo -e "${green}✓ Self-signed SSL configured successfully${plain}"
                    else
                        echo -e "${red}✗ Self-signed SSL setup failed${plain}"
                        read -rp "Press Enter to retry SSL setup..."
                    fi
                    continue
                fi

                if is_ip "${ssl_domain}"; then
                    echo -e "${red}Let's Encrypt does not issue certificates for IP addresses.${plain}"
                    echo -e "${yellow}Please provide a domain, or type 'self' for a self-signed certificate.${plain}"
                    continue
                fi
                if ! is_domain "${ssl_domain}"; then
                    echo -e "${red}Input does not look like a valid domain.${plain}"
                    continue
                fi

                echo -e "${green}Using domain: ${ssl_domain}${plain}"
                echo -e "${yellow}Note: Port 80 must be open and accessible from the internet${plain}"
                read -rp "Press Enter to continue with SSL certificate generation..."

                # Stop panel if running
                if [[ $release == "alpine" ]]; then
                    rc-service x-ui stop >/dev/null 2>&1
                else
                    systemctl stop x-ui >/dev/null 2>&1
                fi

                setup_ssl_certificate "${ssl_domain}" "${server_ip}" "${existing_port}" "/${existing_webBasePath}"

                if [ $? -eq 0 ]; then
                    ssl_success=true
                    echo -e "${green}✓ SSL certificate configured successfully!${plain}"
                else
                    echo ""
                    echo -e "${red}✗ SSL certificate setup failed${plain}"
                    echo -e "${yellow}Please check:${plain}"
                    echo -e "  - Port 80 is open and not in use"
                    echo -e "  - Server is accessible from the internet"
                    echo -e "  - Domain DNS is correctly configured (A/AAAA record)"
                    echo ""
                    read -rp "Press Enter to retry SSL setup..."
                fi
            done

            # Start panel after SSL is configured
            if [[ $release == "alpine" ]]; then
                rc-service x-ui start >/dev/null 2>&1
            else
                systemctl start x-ui >/dev/null 2>&1
            fi

            echo -e "${green}Access URL:  https://${ssl_domain}:${existing_port}/${existing_webBasePath}${plain}"
        else
            echo -e "${green}SSL certificate already configured. No action needed.${plain}"
        fi
    fi
    
    /usr/local/x-ui/x-ui migrate
}

install_x-ui() {
    cd /usr/local/
    
    # Download resources
    if [ $# == 0 ]; then
        tag_version=$(curl -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${yellow}Trying to fetch version with IPv4...${plain}"
            tag_version=$(curl -4 -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
            if [[ ! -n "$tag_version" ]]; then
                echo -e "${red}Failed to fetch x-ui version, it may be due to GitHub API restrictions, please try it later${plain}"
                exit 1
            fi
        fi
        echo -e "Got x-ui latest version: ${tag_version}, beginning the installation..."
        wget --inet4-only -N -O /usr/local/x-ui-linux-$(arch).tar.gz https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Downloading x-ui failed, please be sure that your server can access GitHub ${plain}"
            exit 1
        fi
    else
        tag_version=$1
        tag_version_numeric=${tag_version#v}
        min_version="2.3.5"
        
        if [[ "$(printf '%s\n' "$min_version" "$tag_version_numeric" | sort -V | head -n1)" != "$min_version" ]]; then
            echo -e "${red}Please use a newer version (at least v2.3.5). Exiting installation.${plain}"
            exit 1
        fi
        
        url="https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz"
        echo -e "Beginning to install x-ui $1"
        wget --inet4-only -N -O /usr/local/x-ui-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Download x-ui $1 failed, please check if the version exists ${plain}"
            exit 1
        fi
    fi
    wget --inet4-only -O /usr/bin/x-ui-temp https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.sh
    if [[ $? -ne 0 ]]; then
        echo -e "${red}Failed to download x-ui.sh${plain}"
        exit 1
    fi
    
    # Stop x-ui service and remove old resources
    if [[ -e /usr/local/x-ui/ ]]; then
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop
        else
            systemctl stop x-ui
        fi
        rm /usr/local/x-ui/ -rf
    fi
    
    # Extract resources and set permissions
    tar zxvf x-ui-linux-$(arch).tar.gz
    rm x-ui-linux-$(arch).tar.gz -f
    
    cd x-ui
    chmod +x x-ui
    chmod +x x-ui.sh
    
    # Check the system's architecture and rename the file accordingly
    if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm
        chmod +x bin/xray-linux-arm
    fi
    chmod +x x-ui bin/xray-linux-$(arch)
    
    # Update x-ui cli and se set permission
    mv -f /usr/bin/x-ui-temp /usr/bin/x-ui
    chmod +x /usr/bin/x-ui
    config_after_install
    
    if [[ $release == "alpine" ]]; then
        wget --inet4-only -O /etc/init.d/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.rc
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Failed to download x-ui.rc${plain}"
            exit 1
        fi
        chmod +x /etc/init.d/x-ui
        rc-update add x-ui
        rc-service x-ui start
    else
        cp -f x-ui.service /etc/systemd/system/
        systemctl daemon-reload
        systemctl enable x-ui
        systemctl start x-ui
    fi
    
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
