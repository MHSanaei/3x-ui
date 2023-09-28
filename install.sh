#!/bin/bash

export LANG=en_US.UTF-8

RED="\033[31m"
GREEN="\033[32m"
YELLOW="\033[33m"
PLAIN="\033[0m"

red() {
    echo -e "\033[31m\033[01m$1\033[0m"
}

green() {
    echo -e "\033[32m\033[01m$1\033[0m"
}

yellow() {
    echo -e "\033[33m\033[01m$1\033[0m"
}

# Kiến trúc hiện tại
archxui(){
    case "$(uname -m)" in
        x86_64 | x64 | amd64 ) echo 'amd64' ;;
        armv6* | armv7* | armv7l* ) echo 'armv7' ;;
        armv8* | arm64 | aarch64 ) echo 'arm64' ;;
        s390x ) echo 's390x' ;;
        * ) red "Unsupported CPU architecture! " && rm -f install.sh && exit 1 ;;
    esac
}

# URL tải về X-UI dựa trên kiến trúc
xui_download_url=""
if [ "$(archxui)" == "amd64" ]; then
    xui_download_url="https://github.com/quydang04/x-ui/releases/download/latest/x-ui-linux-amd64.tar.gz"
elif [ "$(archxui)" == "armv7" ]; then
    xui_download_url="https://github.com/quydang04/x-ui/releases/download/latest/x-ui-linux-armv7.tar.gz"
elif [ "$(archxui)" == "arm64" ]; then
    xui_download_url="https://github.com/quydang04/x-ui/releases/download/latest/x-ui-linux-arm64.tar.gz"
else
    red "Unsupported CPU architecture! " && rm -f install.sh && exit 1
fi

# Sử dụng URL tải về X-UI để tải về phiên bản phù hợp
wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(archxui).tar.gz "$xui_download_url"

# Current Directory
cur_dir=$(pwd)

# Check root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} this script must be run as root user " && exit 1

# Check OS and set release variable
REGEX=("debian" "ubuntu" "centos|red hat|kernel|oracle linux|alma|rocky" "'amazon linux'" "fedora", "alpine", "arch", "manjaro", "armbian")
RELEASE=("Debian" "Ubuntu" "CentOS" "CentOS" "Fedora" "Alpine", "ArchLinux", "Manjaro", "Armbian")
PACKAGE_UPDATE=("apt-get update" "apt-get update" "yum -y update" "yum -y update" "yum -y update" "apk update -f", "pacman -Syu", "pacman -Syu", "apt update")
PACKAGE_INSTALL=("apt -y install" "apt -y install" "yum -y install" "yum -y install" "yum -y install" "apk add -f", "pacman -S", "pacman -S", "apt -y install")
PACKAGE_REMOVE=("apt -y remove" "apt -y remove" "yum -y remove" "yum -y remove" "yum -y remove" "apk del -f", "pacman -Rns", "pacman -Rns", "apt -y remove")
PACKAGE_UNINSTALL=("apt -y autoremove" "apt -y autoremove" "yum -y autoremove" "yum -y autoremove" "yum -y autoremove" "apk del -f", "apt -y autoremove")

[[ $EUID -ne 0 ]] && red "This script must be run as root user！" && exit 1

CMD=("$(grep -i pretty_name /etc/os-release 2>/dev/null | cut -d \" -f2)" "$(hostnamectl 2>/dev/null | grep -i system | cut -d : -f2)" "$(lsb_release -sd 2>/dev/null)" "$(grep -i description /etc/lsb-release 2>/dev/null | cut -d \" -f2)" "$(grep . /etc/redhat-release 2>/dev/null)" "$(grep . /etc/issue 2>/dev/null | cut -d \\ -f1 | sed '/^[ ]*$/d')")

for i in "${CMD[@]}"; do
    SYS="$i" && [[ -n $SYS ]] && break
done

for ((int = 0; int < ${#REGEX[@]}; int++)); do
    [[ $(echo "$SYS" | tr '[:upper:]' '[:lower:]') =~ ${REGEX[int]} ]] && SYSTEM="${RELEASE[int]}" && [[ -n $SYSTEM ]] && break
done

[[ -z $SYSTEM ]] && red "Script doesn't support your system. Please use a supported one" && exit 1

cur_dir=$(pwd)
os_version=$(grep -i version_id /etc/os-release | cut -d \" -f2 | cut -d . -f1)

[[ $SYSTEM == "CentOS" && ${os_version} -lt 7 ]] && echo -e "Please use the system 7 or higher version of the system!" && exit 1
[[ $SYSTEM == "Fedora" && ${os_version} -lt 30 ]] && echo -e "Please use Fedora 30 or higher version system!" && exit 1
[[ $SYSTEM == "Ubuntu" && ${os_version} -lt 20 ]] && echo -e "Please use Ubuntu 20 or higher version system!" && exit 1
[[ $SYSTEM == "Debian" && ${os_version} -lt 10 ]] && echo -e "Please use Debian 10 or higher version system!" && exit 1
[[ $SYSTEM == "ArchLinux" ]] && echo -e "Your OS is ArchLinux!"
[[ $SYSTEM == "Manjaro" ]] && echo -e "Your OS is Manjaro!"

# Các hàm và phần còn lại của mã đã được giữ nguyên

# Show info system
info_sys(){
    echo -e "${GREEN} ---------------------------------------- ${PLAIN}"
    echo -e "${GREEN}   __   __           _    _ _____         ${PLAIN}"
    echo -e "${GREEN}   \ \ / /          | |  | |_   _|        ${PLAIN}"
    echo -e "${GREEN}    \ V /   ______  | |  | | | |          ${PLAIN}"
    echo -e "${GREEN}     > <   |______| | |  | | | |          ${PLAIN}"
    echo -e "${GREEN}    / . \           | |__| |_| |_         ${PLAIN}"
    echo -e "${GREEN}   /_/ \_\           \____/|_____|        ${PLAIN}"
    echo -e "${GREEN} ----------------------------------------- ${PLAIN}"
    echo ""
    echo -e "Your system is running: ${GREEN} ${CMD} ${PLAIN}"
    echo ""
    sleep 5
}

# Check system status
check_status(){
    yellow "Checking the IP configuration, please patient..." && sleep 5
    WgcfIPv4Status=$(curl -s4m8 https://www.cloudflare.com/cdn-cgi/trace -k | grep warp | cut -d= -f2)
    WgcfIPv6Status=$(curl -s6m8 https://www.cloudflare.com/cdn-cgi/trace -k | grep warp | cut -d= -f2)
    if [[ $WgcfIPv4Status =~ "on"|"plus" ]] || [[ $WgcfIPv6Status =~ "on"|"plus" ]]; then
        wg-quick down wgcf >/dev/null 2>&1
        v6=$(curl -s6m8 ip.gs -k)
        v4=$(curl -s4m8 ip.gs -k)
        wg-quick up wgcf >/dev/null 2>&1
    else
        v6=$(curl -s6m8 ip.gs -k)
        v4=$(curl -s4m8 ip.gs -k)
        if [[ -z $v4 && -n $v6 ]]; then
            yellow "IPv6 only is detected. So the DNS64 parsing server has been added automatically"
            echo -e "nameserver 2606:4700:4700::1111" > /etc/resolv.conf
        fi
    fi
    sleep 1
}

# Install base of X-UI
install_base(){
    if [[ ! $SYSTEM == "CentOS" ]]; then
        ${PACKAGE_UPDATE[int]}
    fi
    if [[ -z $(type -P curl) ]]; then
        ${PACKAGE_INSTALL[int]} curl
    fi
    if [[ -z $(type -P tar) ]]; then
        ${PACKAGE_INSTALL[int]} tar
    fi   
    check_status
}

# This function will be called when user installed x-ui out of security
config_after_install() {
    echo -e "${yellow}Install/update finished! For security it's recommended to modify panel settings ${plain}"
    read -p "Do you want to continue with the modification[y/n]": config_confirm
    if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
        read -p "Please set up your username: " config_account
        echo -e "${yellow}Your username will be: ${config_account}${plain}"
        read -p "Please set up your password: " config_password
        echo -e "${yellow}Your password will be: ${config_password}${plain}"
        read -p "Please set up the panel port: " config_port
        echo -e "${yellow}Your panel port is: ${config_port}${plain}"
        echo -e "${yellow}Initializing, please wait...${plain}"
        /usr/local/x-ui/x-ui setting -username ${config_account} -password ${config_password}
        echo -e "${yellow}Account name and password set successfully!${plain}"
        /usr/local/x-ui/x-ui setting -port ${config_port}
        echo -e ""
        echo -e "${yellow}Panel port set successfully!\n${plain}"
    else
        echo -e "${red}Cancelling...${plain}"
        if [[ ! -f "/etc/x-ui/x-ui.db" ]]; then
            local usernameTemp=$(head -c 6 /dev/urandom | base64)
            local passwordTemp=$(head-c 6 /dev/urandom | base64)
            /usr/local/x-ui/x-ui setting -username ${usernameTemp} -password ${passwordTemp}
            echo -e "This is a fresh installation,will generate random login info for security concerns:"
            echo -e "###############################################"
            echo -e "${green}Username:${usernameTemp}${plain}"
            echo-e "${green}Password:${passwordTemp}${plain}"
            echo -e "###############################################"
            echo -e "${red}If you forgot your login info,you can type x-ui and then type 7 to check after installation${plain}"
        else
            echo -e "${red} This is your upgrade,will keep old settings,if you forgot your login info,you can type x-ui and then type 7 to check${plain}"
        fi
    fi
    /usr/local/x-ui/x-ui migrate
}

install_x-ui() {
    info_sys
    
    cd /usr/local/

    if [ $# == 0 ]; then
        last_version=$(curl -Ls "https://api.github.com/repos/quydang04/x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$last_version" ]]; then
            echo -e "${red}Failed to fetch x-ui version, it maybe due to Github API restrictions, please try it later${plain}"
            exit 1
        fi
        echo -e "Got x-ui latest version: ${last_version}, beginning the installation..."
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(archxui).tar.gz https://github.com/quydang04/x-ui/releases/download/${last_version}/x-ui-linux-$(archxui).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Downloading x-ui failed, please be sure that your server can access Github ${plain}"
            exit 1
        fi
    else
        last_version=$1
        url="https://github.com/quydang04/x-ui/releases/download/${last_version}/x-ui-linux-$(archxui).tar.gz"
        echo -e "Beginning to install x-ui $1"
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(archxui).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Download x-ui $1 failed,please check the version exists ${plain}"
            exit 1
        fi
    fi

    if [[ -e /usr/local/x-ui/ ]]; then
        systemctl stop x-ui
        rm /usr/local/x-ui/ -rf
    fi

    tar zxvf x-ui-linux-$(archxui).tar.gz
    rm x-ui-linux-$(archxui).tar.gz -f
    cd x-ui
    chmod +x x-ui bin/xray-linux-$(archxui)
    cp -f x-ui.service /etc/systemd/system/
    wget --no-check-certificate -O /usr/bin/x-ui https://raw.githubusercontent.com/quydang04/x-ui/main/x-ui.sh
    chmod +x /usr/local/x-ui/x-ui.sh
    chmod +x /usr/bin/x-ui
    config_after_install
    systemctl daemon-reload
    systemctl enable x-ui
    systemctl start x-ui
    rm -f install.sh
    echo -e "${green}X-UI ${last_version}${plain} installation finished, it's running now..."
    echo -e "------------------------------------------------------------------------------"
    echo -e "X-UI SCRIPT USAGE: "
    echo -e "------------------------------------------------------------------------------"
    echo -e "x-ui              - Show the management menu"
    echo -e "x-ui start        - Start X-UI panel"
    echo -e "x-ui stop         - Stop X-UI panel"
    echo -e "x-ui restart      - Restart X-UI panel"
    echo -e "x-ui status       - View X-UI status"
    echo -e "x-ui enable       - Set X-UI boot self-starting"
    echo -e "x-ui disable      - Cancel X-UI boot self-starting"
    echo -e "x-ui log          - View x-ui log"
    echo -e "x-ui v2-ui        - Migrate V2-UI to X-UI"
    echo -e "x-ui update       - Update X-UI panel"
    echo -e "x-ui install      - Install X-UI panel"
    echo -e "x-ui uninstall    - Uninstall X-UI panel"
    echo -e "------------------------------------------------------------------------------"
    echo -e "Please do consider supporting authors"
    echo -e "------------------------------------------------------------------------------"
    echo -e "vaxilu            - https://github.com/vaxilu" 
    echo -e "MHSanaei          - https://github.com/MHSanaei/"  
    echo -e "Hossin Asaadi     - https://github.com/hossinasaadi"
    echo -e "NidukaAkalanka    - https://github.com/NidukaAkalanka" 
    echo -e "--------------------------------------------------------------------------------"
    show_login_info
    yellow "If you can't access X-UI, please check your firewall or accept the ports you have set while installing X-UI."
}

show_login_info(){
    if [[ -n $v4 && -z $v6 ]]; then
        echo -e "Panel IPv4 login address is: ${GREEN}http://$v4:$config_port ${PLAIN}"
    elif [[ -n $v6 && -z $v4 ]]; then
        echo -e "Panel IPv6 login address is: ${GREEN}http://[$v6]:$config_port ${PLAIN}"
    elif [[ -n $v4 && -n $v6 ]]; then
        echo -e "IPv4 login address is: ${GREEN}http://$v4:$config_port ${PLAIN}"
        echo -e "IPv6 login address is: ${GREEN}http://[$v6]:$config_port ${PLAIN}"
    fi
    echo -e "Your username: ${GREEN}$config_account ${PLAIN}"
    echo -e "Your password: ${GREEN}$config_password ${PLAIN}"
}

clear
echo -e "${green}This script will check and update your system before installing x-ui, checking...${plain}"
install_base
install_x-ui $1
