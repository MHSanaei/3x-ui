#!/bin/bash

RED="\033[31m"
GREEN="\033[32m"
YELLOW="\033[33m"
PLAIN='\033[0m'

red(){
    echo -e "\033[31m\033[01m$1\033[0m"
}

green(){
    echo -e "\033[32m\033[01m$1\033[0m"
}

yellow(){
    echo -e "\033[33m\033[01m$1\033[0m"
}

REGEX=("debian" "ubuntu" "centos|red hat|kernel|oracle linux|alma|rocky" "'amazon linux'" "fedora")
RELEASE=("Debian" "Ubuntu" "CentOS" "CentOS" "Fedora")
PACKAGE_UPDATE=("apt-get update" "apt-get update" "yum -y update" "yum -y update" "yum -y update")
PACKAGE_INSTALL=("apt -y install" "apt -y install" "yum -y install" "yum -y install" "yum -y install")
PACKAGE_REMOVE=("apt -y remove" "apt -y remove" "yum -y remove" "yum -y remove" "yum -y remove")
PACKAGE_UNINSTALL=("apt -y autoremove" "apt -y autoremove" "yum -y autoremove" "yum -y autoremove" "yum -y autoremove")

[[ $EUID -ne 0 ]] && red "Note: Please run the script as the root user" && exit 1

CMD=("$(grep -i pretty_name /etc/os-release 2>/dev/null | cut -d \" -f2)" "$(hostnamectl 2>/dev/null | grep -i system | cut -d : -f2)" "$(lsb_release -sd 2>/dev/null)" "$(grep -i description /etc/lsb-release 2>/dev/null | cut -d \" -f2)" "$(grep . /etc/redhat-release 2>/dev/null)" "$(grep . /etc/issue 2>/dev/null | cut -d \\ -f1 | sed '/^[ ]*$/d')")

for i in "${CMD[@]}"; do
    SYS="$i"
    if [[ -n $SYS ]]; then
        break
    fi
done

for ((int = 0; int < ${#REGEX[@]}; int++)); do
    if [[ $(echo "$SYS" | tr '[:upper:]' '[:lower:]') =~ ${REGEX[int]} ]]; then
        SYSTEM="${RELEASE[int]}"
        if [[ -n $SYSTEM ]]; then
            break
        fi
    fi
done

[[ -z $SYSTEM ]] && red "Does not support the current OS, please use a supported one" && exit 1

back2menu() {
    echo ""
    green "The selected command operation execution is completed"
    read -rp "Please enter 'Y' to exit, or press the any key back to the main menu：" back2menuInput
    case "$back2menuInput" in
        y) exit 1 ;;
        *) menu ;;
    esac
}

install_base(){
    if [[ ! $SYSTEM == "CentOS" ]]; then
        ${PACKAGE_UPDATE[int]}
    fi
    ${PACKAGE_INSTALL[int]} curl wget sudo socat
    if [[ $SYSTEM == "CentOS" ]]; then
        ${PACKAGE_INSTALL[int]} cronie
        systemctl start crond
        systemctl enable crond
    else
        ${PACKAGE_INSTALL[int]} cron
        systemctl start cron
        systemctl enable cron
    fi
}

install_acme(){
    install_base
    read -rp "Please enter the registered email (for example: admin@gmail.com, or leave empty to automatically generate a fake email): " acmeEmail
    if [[ -z $acmeEmail ]]; then
        autoEmail=$(date +%s%N | md5sum | cut -c 1-16)
        acmeEmail=$autoEmail@gmail.com
        yellow "Skipped entering email, using a fake email address: $acmeEmail"
    fi
    curl https://get.acme.sh | sh -s email=$acmeEmail
    source ~/.bashrc
    bash ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    bash ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
    if [[ -n $(~/.acme.sh/acme.sh -v 2>/dev/null) ]]; then
        green "ACME.SH certificate application script installed successfully!"
    else
        red "Sorry, the ACME.SH certificate application script installation failed"
        green "Suggestions:"
        yellow "1. Check the server network connection"
        yellow "2. The script could be outdated. Please open up a issue in Github at https://github.com/NidukaAkalanka/x-ui-english/issues"
    fi
    back2menu
}

check_80(){
        if [[ -z $(type -P lsof) ]]; then
        if [[ ! $SYSTEM == "CentOS" ]]; then
            ${PACKAGE_UPDATE[int]}
        fi
        ${PACKAGE_INSTALL[int]} lsof
    fi
    
    yellow "Checking if the port 80 is in use..."
    sleep 1
    
    if [[  $(lsof -i:"80" | grep -i -c "listen") -eq 0 ]]; then
        green "Good! Port 80 is not in use"
        sleep 1
    else
        red "Port 80 is currently in use, please close the service this service, which is using port 80:"
        lsof -i:"80"
        read -rp "If you need to close this service right now, please press Y. Otherwise, press N to abort SSL issuing [Y/N]: " yn
        if [[ $yn =~ "Y"|"y" ]]; then
            lsof -i:"80" | awk '{print $2}' | grep -v "PID" | xargs kill -9
            sleep 1
        else
            exit 1
        fi
    fi
}

acme_standalone(){
    [[ -z $(~/.acme.sh/acme.sh -v 2>/dev/null) ]] && red "Unpacking ACME.SH, Getting ready..." && exit 1
    check_80
    WARPv4Status=$(curl -s4m8 https://www.cloudflare.com/cdn-cgi/trace -k | grep warp | cut -d= -f2)
    WARPv6Status=$(curl -s6m8 https://www.cloudflare.com/cdn-cgi/trace -k | grep warp | cut -d= -f2)
    if [[ $WARPv4Status =~ on|plus ]] || [[ $WARPv6Status =~ on|plus ]]; then
        wg-quick down wgcf >/dev/null 2>&1
    fi
    
    ipv4=$(curl -s4m8 ip.gs)
    ipv6=$(curl -s6m8 ip.gs)
    
    echo ""
    yellow "When using port 80 application mode, first point your domain name to your server's public IP address. Otherwise the certificate application will be failed!"
    echo ""
    if [[ -n $ipv4 && -n $ipv6 ]]; then
        echo -e "The public IPv4 address of server is: ${GREEN} $ipv4 ${PLAIN}"
        echo -e "The public IPv6 address of server is: ${GREEN} $ipv6 ${PLAIN}"
    elif [[ -n $ipv4 && -z $ipv6 ]]; then
        echo -e "The public IPv4 address of server is: ${GREEN} $ipv4 ${PLAIN}"
    elif [[ -z $ipv4 && -n $ipv6 ]]; then
        echo -e "The public IPv6 address of server is: ${GREEN} $ipv6 ${PLAIN}"
    fi
    echo ""
    read -rp "Please enter the pointed domain / sub-domain name: " domain
    [[ -z $domain ]] && red "Given domain is invalid. Please use example.com / sub.example.com" && exit 1
    green "The given domain name：$domain" && sleep 1
    domainIP=$(curl -sm8 ipget.net/?ip="${domain}")
    
    if [[ $domainIP == $ipv6 ]]; then
        bash ~/.acme.sh/acme.sh --issue -d ${domain} --standalone -k ec-256 --listen-v6 --insecure
    fi
    if [[ $domainIP == $ipv4 ]]; then
        bash ~/.acme.sh/acme.sh --issue -d ${domain} --standalone -k ec-256 --insecure
    fi
    
    if [[ -n $(echo $domainIP | grep nginx) ]]; then
        yellow "The domain name analysis failed, please check whether the domain name is correctly entered, and whether the domain name has been pointed to the server's public IP address"
        exit 1
    elif [[ -n $(echo $domainIP | grep ":") || -n $(echo $domainIP | grep ".") ]]; then
        if [[ $domainIP != $ipv4 ]] && [[ $domainIP != $ipv6 ]]; then
            if [[ -n $(type -P wg-quick) && -n $(type -P wgcf) ]]; then
                wg-quick up wgcf >/dev/null 2>&1
            fi
            green "Domain name ${domain} Currently pointed IP: ($domainIP)"
            red "The current domain name's resolved IP does not match the public IP used of the server"
            green "Suggestions:"
            yellow "1. Please check whether domain is correctly pointed to the server's current public IP"
            yellow "2. Please make sure that Cloudflare Proxy is closed (only DNS)"
            yellow "3. The script could be outdated. Please open up a issue in Github at https://github.com/NidukaAkalanka/x-ui-english/issues"
            exit 1
        fi
    fi
    
    bash ~/.acme.sh/acme.sh --install-cert -d ${domain} --key-file /root/private.key --fullchain-file /root/cert.crt --ecc
    checktls
}

acme_cfapiTLD(){
    [[ -z $(~/.acme.sh/acme.sh -v 2>/dev/null) ]] && red "Unpacking ACME.SH, Getting ready..." && exit 1
    ipv4=$(curl -s4m8 ip.gs)
    ipv6=$(curl -s6m8 ip.gs)
    read -rp "Please enter the domain name to issue certificate (sub.example.com): " domain
    if [[ $(echo ${domain:0-2}) =~ cf|ga|gq|ml|tk ]]; then
        red "Detected a Freenom free domain. Since the Cloudflare API does not support it, it is impossible!"
        back2menu
    fi
    read -rp "Enter CloudFlare Global API Key: " GAK
    [[ -z $GAK ]] && red "Unable to verify Cloudflare Global API Key, unable to perform operations!" && exit 1
    export CF_Key="$GAK"
    read -rp "Enter Cloudflare's registered email: " CFemail
    [[ -z $domain ]] && red "Unable to login with the provided email address and API key. Aborted!" && exit 1
    export CF_Email="$CFemail"
    if [[ -z $ipv4 ]]; then
        bash ~/.acme.sh/acme.sh --issue --dns dns_cf -d "${domain}" -k ec-256 --listen-v6 --insecure
    else
        bash ~/.acme.sh/acme.sh --issue --dns dns_cf -d "${domain}" -k ec-256 --insecure
    fi
    bash ~/.acme.sh/acme.sh --install-cert -d "${domain}" --key-file /root/private.key --fullchain-file /root/cert.crt --ecc
    checktls
}

acme_cfapiNTLD(){
    [[ -z $(~/.acme.sh/acme.sh -v 2>/dev/null) ]] && red "Unpacking ACME.SH, Getting ready..." && exit 1
    ipv4=$(curl -s4m8 ip.gs)
    ipv6=$(curl -s6m8 ip.gs)
    read -rp "Please enter the main domain name that requires the application certificate (input format: example.com): " domain
    [[ -z $domain ]] && red "Given domain is invalid！" && exit 1
    if [[ $(echo ${domain:0-2}) =~ cf|ga|gq|ml|tk ]]; then
        red "Detected a Freenom free domain. Since the Cloudflare API does not support it, it is impossible!"
        back2menu
    fi
    read -rp "Enter CloudFlare Global API Key: " GAK
    [[ -z $GAK ]] && red "Unable to verify Cloudflare Global API Key, unable to perform operations!" && exit 1
    export CF_Key="$GAK"
    read -rp "Enter CloudFlare registered email: " CFemail
    [[ -z $domain ]] && red "Unable to login with the provided email address and API key. Aborted!" && exit 1
    export CF_Email="$CFemail"
    if [[ -z $ipv4 ]]; then
        bash ~/.acme.sh/acme.sh --issue --dns dns_cf -d "*.${domain}" -d "${domain}" -k ec-256 --listen-v6 --insecure
    else
        bash ~/.acme.sh/acme.sh --issue --dns dns_cf -d "*.${domain}" -d "${domain}" -k ec-256 --insecure
    fi
    bash ~/.acme.sh/acme.sh --install-cert -d "*.${domain}" --key-file /root/private.key --fullchain-file /root/cert.crt --ecc
    checktls
}

checktls() {
    if [[ -f /root/cert.crt && -f /root/private.key ]]; then
        if [[ -s /root/cert.crt && -s /root/private.key ]]; then
            if [[ -n $(type -P wg-quick) && -n $(type -P wgcf) ]]; then
                wg-quick up wgcf >/dev/null 2>&1
            fi
            sed -i '/--cron/d' /etc/crontab >/dev/null 2>&1
            echo "0 0 * * * root bash /root/.acme.sh/acme.sh --cron -f >/dev/null 2>&1" >> /etc/crontab
            green "Successful application! certificate.crt and Private.key files have been saved to /root/ folder. Use these to your Panel Settings and V2ray configs"
            yellow "Certificate.crt file path is as follows : /root/cert.crt"
            yellow "Private.key file path is as follows     : /root/private.key"
            back2menu
        else
            if [[ -n $(type -P wg-quick) && -n $(type -P wgcf) ]]; then
                wg-quick up wgcf >/dev/null 2>&1
            fi
            red "Sorry. The certificate application failed"
            green "Suggestions: "
            yellow "1. Check whether the firewall is opened. If the application mode of port 80 is used, please open or release port 80"
            yellow "2. Applying for many times in the same domain name may subject it to the risk control of Let'sEncrypt. Please configure another domain that you own or try switching the provider by choosing 9 from the ACME script menu." 
            yellow "3. Try again with the above used domain after 7 days. "
            yellow "4. The script may not be able to keep up with the times, it is recommended to release screenshots to github issues to inquire "
            back2menu
        fi
    fi
}

view_cert(){
    [[ -z $(~/.acme.sh/acme.sh -v 2>/dev/null) ]] && yellow "Unpacking ACME.SH. Getting ready..." && exit 1
    bash ~/.acme.sh/acme.sh --list
    back2menu
}

revoke_cert() {
    [[ -z $(~/.acme.sh/acme.sh -v 2>/dev/null) ]] && yellow "Unpacking ACME.SH. Getting ready..." && exit 1
    bash ~/.acme.sh/acme.sh --list
    read -rp "Please enter the domain name certificate to be revoked (Enter the sub-domain): " domain
    [[ -z $domain ]] && red "Invalid domain name and cannot perform operations!" && exit 1
    if [[ -n $(bash ~/.acme.sh/acme.sh --list | grep $domain) ]]; then
        bash ~/.acme.sh/acme.sh --revoke -d ${domain} --ecc
        bash ~/.acme.sh/acme.sh --remove -d ${domain} --ecc
        rm -rf ~/.acme.sh/${domain}_ecc
        rm -f /root/cert.crt /root/private.key
        green "Revoking the domain name certificate of $ {domin} successfully"
        back2menu
    else
        red "No domain name certificate for $ {domain}, please check by yourself!"
        back2menu
    fi
}

renew_cert() {
    [[ -z $(~/.acme.sh/acme.sh -v 2>/dev/null) ]] && yellow "Unpacking ACME.SH. Getting ready..." && exit 1
    bash ~/.acme.sh/acme.sh --list
    read -rp "Please enter the domain name for the certificate to be renewed (Enter the sub-domain): " domain
    [[ -z $domain ]] && red "Unable to enter the domain name and cannot perform operations!" && exit 1
    if [[ -n $(bash ~/.acme.sh/acme.sh --list | grep $domain) ]]; then
        bash ~/.acme.sh/acme.sh --renew -d ${domain} --force --ecc
        checktls
        back2menu
    else
        red "No domain name certificate for $ {domain}, please check the domain name input correctly again"
        back2menu
    fi
}

switch_provider(){
    yellow "Please select the certificate provider, apply for the certificate now to issue from the default provider "
    yellow "If the certificate application fails, for example, if there are too many applications requested from LetSencrypt.org within a day, you can choose Buypass.com or Zerossl.com to apply."
    echo -e " ${GREEN}1.${PLAIN} Letsencrypt.org"
    echo -e " ${GREEN}2.${PLAIN} BuyPass.com"
    echo -e " ${GREEN}3.${PLAIN} ZeroSSL.com"
    read -rp "Please select certificate provider [1-3]: " provider
    case $provider in
        2) bash ~/.acme.sh/acme.sh --set-default-ca --server buypass && green "Switched certificate provider to BuyPass.com！" ;;
        3) bash ~/.acme.sh/acme.sh --set-default-ca --server zerossl && green "Switched certificate provider to ZeroSSL.com!" ;;
        *) bash ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt && green "Switched certificate provider to Letsencrypt.org！" ;;
    esac
    back2menu
}

uninstall() {
    [[ -z $(~/.acme.sh/acme.sh -v 2>/dev/null) ]] && yellow "Unpacking ACME.SH. Getting ready...!" && exit 1
    ~/.acme.sh/acme.sh --uninstall
    sed -i '/--cron/d' /etc/crontab >/dev/null 2>&1
    rm -rf ~/.acme.sh
    green "Acme  One-click application certificate script has been completely uninstalled. Bye Bye!"
}

menu() {
    clear
    echo "--------------------------------------------------------------"
    echo -e "____ ____ _  _ ____    ____ ____ ____    _  _    _  _ _ "
    echo -e "|__| |    |\/| |___    |___ |  | |__/     \/  __ |  | | "
    echo -e "|  | |___ |  | |___    |    |__| |  \    _/\_    |__| | "
    echo "--------------------------------------------------------------"
    echo ""
    echo -e " ${GREEN}1.${PLAIN} Install ACME.SH"
    echo -e " ${GREEN}2.${PLAIN} ${RED}Uninstall ACME.SH${PLAIN}"
    echo " -------------"
    echo -e " ${GREEN}3.${PLAIN} Certificate issuing via DNS API - Recommended ${YELLOW}(Port 80 should be open)${PLAIN}"
    echo -e " ${GREEN}4.${PLAIN} Certificate issuing via Cloudflare API for sub-domain ${GREEN}${PLAIN} ${RED}(Not working for Freenom free domains)${PLAIN}"
    echo -e " ${GREEN}5.${PLAIN} Certificate issuing via Cloudflare API for root-domain ${PLAIN}$ ${RED}(Not working for Freenom free domains)${PLAIN}"
    echo " -------------"
    echo -e " ${GREEN}6.${PLAIN} Check the certificate"
    echo -e " ${GREEN}7.${PLAIN} Revoke the certificate"
    echo -e " ${GREEN}8.${PLAIN} Manual renewal of certificate"
    echo -e " ${GREEN}9.${PLAIN} Switch certificate issuer"
    echo " -------------"
    echo -e " ${GREEN}0.${PLAIN} Exit script"
    echo ""
    read -rp "Please enter the option [0-9]: " NumberInput
    case "$NumberInput" in
        1) install_acme ;;
        2) uninstall ;;
        3) acme_standalone ;;
        4) acme_cfapiTLD ;;
        5) acme_cfapiNTLD ;;
        6) view_cert ;;
        7) revoke_cert ;;
        8) renew_cert ;;
        9) switch_provider ;;
        *) exit 1 ;;
    esac
}

menu
