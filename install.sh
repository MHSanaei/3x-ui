#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error：${plain} Please run this script with root privilege \n " && exit 1

# check os
if [[ -f /etc/redhat-release ]]; then
    release="centos"
elif cat /etc/issue | grep -Eqi "debian"; then
    release="debian"
elif cat /etc/issue | grep -Eqi "ubuntu"; then
    release="ubuntu"
elif cat /etc/issue | grep -Eqi "centos|red hat|redhat"; then
    release="centos"
elif cat /proc/version | grep -Eqi "debian"; then
    release="debian"
elif cat /proc/version | grep -Eqi "ubuntu"; then
    release="ubuntu"
elif cat /proc/version | grep -Eqi "centos|red hat|redhat"; then
    release="centos"
else
    echo -e "${red} Check system OS failed, please contact the author! ${plain}\n" && exit 1
fi

arch=$(arch)

if [[ $arch == "x86_64" || $arch == "x64" || $arch == "amd64" ]]; then
    arch="amd64"
elif [[ $arch == "aarch64" || $arch == "arm64" ]]; then
    arch="arm64"
elif [[ $arch == "s390x" ]]; then
    arch="s390x"
else
    arch="amd64"
    echo -e "${red} Failed to check system arch, will use default arch: ${arch}${plain}"
fi

echo "arch: ${arch}"

if [ $(getconf WORD_BIT) != '32' ] && [ $(getconf LONG_BIT) != '64' ]; then
    echo "x-ui dosen't support 32-bit(x86) system, please use 64 bit operating system(x86_64) instead, if there is something wrong, please get in touch with me!"
    exit -1
fi

os_version=""

# os version
if [[ -f /etc/os-release ]]; then
    os_version=$(awk -F'[= ."]' '/VERSION_ID/{print $3}' /etc/os-release)
fi
if [[ -z "$os_version" && -f /etc/lsb-release ]]; then
    os_version=$(awk -F'[= ."]+' '/DISTRIB_RELEASE/{print $2}' /etc/lsb-release)
fi

if [[ x"${release}" == x"centos" ]]; then
    if [[ ${os_version} -le 6 ]]; then
        echo -e "${red} Please use CentOS 7 or higher ${plain}\n" && exit 1
    fi
elif [[ x"${release}" == x"ubuntu" ]]; then
    if [[ ${os_version} -lt 16 ]]; then
        echo -e "${red} Please use Ubuntu 16 or higher ${plain}\n" && exit 1
    fi
elif [[ x"${release}" == x"debian" ]]; then
    if [[ ${os_version} -lt 8 ]]; then
        echo -e "${red} Please use Debian 8 or higher ${plain}\n" && exit 1
    fi
fi

install_base() {
    if [[ x"${release}" == x"centos" ]]; then
        yum install wget curl tar -y
    else
        apt install wget curl tar -y
    fi
}

#This function will be called when user installed x-ui out of sercurity
config_after_install() {
    echo -e "${yellow}Install/update finished! For security it's recommended to modify panel settings ${plain}"
    read -p "Do you want to continue with the modification [y/n]? ": config_confirm
    if [[ x"${config_confirm}" == x"y" || x"${config_confirm}" == x"Y" ]]; then
        read -p "Please set up your username:" config_account
        echo -e "${yellow}Your username will be:${config_account}${plain}"
        read -p "Please set up your password:" config_password
        echo -e "${yellow}Your password will be:${config_password}${plain}"
        read -p "Please set up the panel port:" config_port
        echo -e "${yellow}Your panel port is:${config_port}${plain}"
        echo -e "${yellow}Initializing, please wait...${plain}"
        /usr/local/x-ui/x-ui setting -username ${config_account} -password ${config_password}
        echo -e "${yellow}Account name and password set successfully!${plain}"
        /usr/local/x-ui/x-ui setting -port ${config_port}
        echo -e "${yellow}Panel port set successfully!${plain}"
    else
        echo -e "${red}Canceled, will use the default settings.${plain}"
    fi
}

install_x-ui() {
    systemctl stop x-ui
    cd /usr/local/

    if [ $# == 0 ]; then
        last_version=$(curl -Ls "https://api.github.com/repos/mhsanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$last_version" ]]; then
            echo -e "${red}Failed to fetch x-ui version, it maybe due to Github API restrictions, please try it later${plain}"
            exit 1
        fi
        echo -e "Got x-ui latest version: ${last_version}, beginning the installation..."
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-${arch}.tar.gz https://github.com/mhsanaei/3x-ui/releases/download/${last_version}/x-ui-linux-${arch}.tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Downloading x-ui failed, please be sure that your server can access Github ${plain}"
            exit 1
        fi
    else
        last_version=$1
        url="https://github.com/mhsanaei/3x-ui/releases/download/${last_version}/x-ui-linux-${arch}.tar.gz"
        echo -e "Begining to install x-ui $1"
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-${arch}.tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Download x-ui $1 failed,please check the version exists${plain}"
            exit 1
        fi
    fi

    if [[ -e /usr/local/x-ui/ ]]; then
        rm /usr/local/x-ui/ -rf
    fi

    tar zxvf x-ui-linux-${arch}.tar.gz
    rm x-ui-linux-${arch}.tar.gz -f
    cd x-ui
    chmod +x x-ui bin/xray-linux-${arch}
    cp -f x-ui.service /etc/systemd/system/
    wget --no-check-certificate -O /usr/bin/x-ui https://raw.githubusercontent.com/mhsanaei/3x-ui/main/x-ui.sh
    chmod +x /usr/local/x-ui/x-ui.sh
    chmod +x /usr/bin/x-ui
    config_after_install
    #echo -e "If it is a new installation, the default web port is ${green}2053${plain}, The username and password are ${green}admin${plain} by default"
    #echo -e "Please make sure that this port is not occupied by other procedures,${yellow} And make sure that port 2053 has been released${plain}"
    #    echo -e "If you want to modify the 2053 to other ports and enter the x-ui command to modify it, you must also ensure that the port you modify is also released"
    #echo -e ""
    #echo -e "If it is updated panel, access the panel in your previous way"
    #echo -e ""
    systemctl daemon-reload
    systemctl enable x-ui
    systemctl start x-ui
    echo -e "${green}x-ui ${last_version}${plain} installation finished, it is running now..."
    echo -e ""
    echo -e "x-ui control menu usages: "
    echo -e "----------------------------------------------"
    echo -e "x-ui              - Enter     Admin menu"
    echo -e "x-ui start        - Start     x-ui"
    echo -e "x-ui stop         - Stop      x-ui"
    echo -e "x-ui restart      - Restart   x-ui"
    echo -e "x-ui status       - Show      x-ui status"
    echo -e "x-ui enable       - Enable    x-ui on system startup"
    echo -e "x-ui disable      - Disable   x-ui on system startup"
    echo -e "x-ui log          - Check     x-ui logs"
    echo -e "x-ui v2-ui        - Migrate   v2-ui Account data to x-ui"
    echo -e "x-ui update       - Update    x-ui"
    echo -e "x-ui install      - Install   x-ui"
    echo -e "x-ui uninstall    - Uninstall x-ui"
    echo -e "----------------------------------------------"
}

echo -e "${green}Running...${plain}"
install_base
install_x-ui $1
