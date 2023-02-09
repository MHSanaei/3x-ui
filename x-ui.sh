#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

#Add some basic function here
function LOGD() {
    echo -e "${yellow}[DEG] $* ${plain}"
}

function LOGE() {
    echo -e "${red}[ERR] $* ${plain}"
}

function LOGI() {
    echo -e "${green}[INF] $* ${plain}"
}
# check root
[[ $EUID -ne 0 ]] && LOGE "ERROR: You must be root to run this script! \n" && exit 1

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
    LOGE "check system OS failed,please contact with author! \n" && exit 1
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
        LOGE "please use CentOS 7 or higher version! \n" && exit 1
    fi
elif [[ x"${release}" == x"ubuntu" ]]; then
    if [[ ${os_version} -lt 16 ]]; then
        LOGE "please use Ubuntu 16 or higher version！\n" && exit 1
    fi
elif [[ x"${release}" == x"debian" ]]; then
    if [[ ${os_version} -lt 8 ]]; then
        LOGE "please use Debian 8 or higher version！\n" && exit 1
    fi
fi

confirm() {
    if [[ $# > 1 ]]; then
        echo && read -p "$1 [Default$2]: " temp
        if [[ x"${temp}" == x"" ]]; then
            temp=$2
        fi
    else
        read -p "$1 [y/n]: " temp
    fi
    if [[ x"${temp}" == x"y" || x"${temp}" == x"Y" ]]; then
        return 0
    else
        return 1
    fi
}

confirm_restart() {
    confirm "Restart the panel, Attention: Restarting the panel will also restart xray" "y"
    if [[ $? == 0 ]]; then
        restart
    else
        show_menu
    fi
}

before_show_menu() {
    echo && echo -n -e "${yellow}Press enter to return to the main menu: ${plain}" && read temp
    show_menu
}

install() {
    bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/main/install.sh)
    if [[ $? == 0 ]]; then
        if [[ $# == 0 ]]; then
            start
        else
            start 0
        fi
    fi
}

update() {
    confirm "This function will forcefully reinstall the latest version, and the data will not be lost. Do you want to continue?" "n"
    if [[ $? != 0 ]]; then
        LOGE "Cancelled"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 0
    fi
    bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/main/install.sh)
    if [[ $? == 0 ]]; then
        LOGI "Update is complete, Panel has automatically restarted "
        exit 0
    fi
}

uninstall() {
    confirm "Are you sure you want to uninstall the panel? xray will also uninstalled!" "n"
    if [[ $? != 0 ]]; then
        if [[ $# == 0 ]]; then
            show_menu
        fi
        return 0
    fi
    systemctl stop x-ui
    systemctl disable x-ui
    rm /etc/systemd/system/x-ui.service -f
    systemctl daemon-reload
    systemctl reset-failed
    rm /etc/x-ui/ -rf
    rm /usr/local/x-ui/ -rf

    echo ""
    echo -e "Uninstalled Successfully，If you want to remove this script，then after exiting the script run ${green}rm /usr/bin/x-ui -f${plain} to delete it."
    echo ""

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

reset_user() {
    confirm "Reset your username and password to admin?" "n"
    if [[ $? != 0 ]]; then
        if [[ $# == 0 ]]; then
            show_menu
        fi
        return 0
    fi
    /usr/local/x-ui/x-ui setting -username admin -password admin
    echo -e "Username and password have been reset to ${green}admin${plain}，Please restart the panel now."
    confirm_restart
}

reset_config() {
    confirm "Are you sure you want to reset all panel settings，Account data will not be lost，Username and password will not change" "n"
    if [[ $? != 0 ]]; then
        if [[ $# == 0 ]]; then
            show_menu
        fi
        return 0
    fi
    /usr/local/x-ui/x-ui setting -reset
    echo -e "All panel settings have been reset to default，Please restart the panel now，and use the default ${green}2053${plain} Port to Access the web Panel"
    confirm_restart
}

check_config() {
    info=$(/usr/local/x-ui/x-ui setting -show true)
    if [[ $? != 0 ]]; then
        LOGE "get current settings error,please check logs"
        show_menu
    fi
    LOGI "${info}"
}

set_port() {
    echo && echo -n -e "Enter port number[1-65535]: " && read port
    if [[ -z "${port}" ]]; then
        LOGD "Cancelled"
        before_show_menu
    else
        /usr/local/x-ui/x-ui setting -port ${port}
        echo -e "The port is set，Please restart the panel now，and use the new port ${green}${port}${plain} to access web panel"
        confirm_restart
    fi
}

start() {
    check_status
    if [[ $? == 0 ]]; then
        echo ""
        LOGI "Panel is running，No need to start again，If you need to restart, please select restart"
    else
        systemctl start x-ui
        sleep 2
        check_status
        if [[ $? == 0 ]]; then
            LOGI "x-ui Started Successfully"
        else
            LOGE "panel Failed to start，Probably because it takes longer than two seconds to start，Please check the log information later"
        fi
    fi

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

stop() {
    check_status
    if [[ $? == 1 ]]; then
        echo ""
        LOGI "Panel stopped，No need to stop again!"
    else
        systemctl stop x-ui
        sleep 2
        check_status
        if [[ $? == 1 ]]; then
            LOGI "x-ui and xray stopped successfully"
        else
            LOGE "Panel stop failed，Probably because the stop time exceeds two seconds，Please check the log information later"
        fi
    fi

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

restart() {
    systemctl restart x-ui
    sleep 2
    check_status
    if [[ $? == 0 ]]; then
        LOGI "x-ui and xray Restarted successfully"
    else
        LOGE "Panel restart failed，Probably because it takes longer than two seconds to start，Please check the log information later"
    fi
    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

status() {
    systemctl status x-ui -l
    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

enable() {
    systemctl enable x-ui
    if [[ $? == 0 ]]; then
        LOGI "x-ui Set to boot automatically on startup successfully"
    else
        LOGE "x-ui Failed to set Autostart"
    fi

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

disable() {
    systemctl disable x-ui
    if [[ $? == 0 ]]; then
        LOGI "x-ui Autostart Cancelled successfully"
    else
        LOGE "x-ui Failed to cancel autostart"
    fi

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

show_log() {
    journalctl -u x-ui.service -e --no-pager -f
    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

migrate_v2_ui() {
    /usr/local/x-ui/x-ui v2-ui

    before_show_menu
}

install_bbr() {
    # temporary workaround for installing bbr
    bash <(curl -L -s https://raw.githubusercontent.com/teddysun/across/master/bbr.sh)
    echo ""
    before_show_menu
}

update_shell() {
    wget -O /usr/bin/x-ui -N --no-check-certificate https://github.com/mhsanaei/3x-ui/raw/main/x-ui.sh
    if [[ $? != 0 ]]; then
        echo ""
        LOGE "Failed to download script，Please check whether the machine can connect Github"
        before_show_menu
    else
        chmod +x /usr/bin/x-ui
        LOGI "Upgrade script succeeded，Please rerun the script" && exit 0
    fi
}

# 0: running, 1: not running, 2: not installed
check_status() {
    if [[ ! -f /etc/systemd/system/x-ui.service ]]; then
        return 2
    fi
    temp=$(systemctl status x-ui | grep Active | awk '{print $3}' | cut -d "(" -f2 | cut -d ")" -f1)
    if [[ x"${temp}" == x"running" ]]; then
        return 0
    else
        return 1
    fi
}

check_enabled() {
    temp=$(systemctl is-enabled x-ui)
    if [[ x"${temp}" == x"enabled" ]]; then
        return 0
    else
        return 1
    fi
}

check_uninstall() {
    check_status
    if [[ $? != 2 ]]; then
        echo ""
        LOGE "Panel installed，Please do not reinstall"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

check_install() {
    check_status
    if [[ $? == 2 ]]; then
        echo ""
        LOGE "Please install the panel first"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

show_status() {
    check_status
    case $? in
    0)
        echo -e "Panel state: ${green}Runing${plain}"
        show_enable_status
        ;;
    1)
        echo -e "Panel state: ${yellow}Not Running${plain}"
        show_enable_status
        ;;
    2)
        echo -e "Panel state: ${red}Not Installed${plain}"
        ;;
    esac
    show_xray_status
}

show_enable_status() {
    check_enabled
    if [[ $? == 0 ]]; then
        echo -e "Start automatically: ${green}Yes${plain}"
    else
        echo -e "Start automatically: ${red}No${plain}"
    fi
}

check_xray_status() {
    count=$(ps -ef | grep "xray-linux" | grep -v "grep" | wc -l)
    if [[ count -ne 0 ]]; then
        return 0
    else
        return 1
    fi
}

show_xray_status() {
    check_xray_status
    if [[ $? == 0 ]]; then
        echo -e "xray state: ${green}Runing${plain}"
    else
        echo -e "xray state: ${red}Not Running${plain}"
    fi
}

ssl_cert_issue() {
    echo -E ""
    LOGD "******Instructions for use******"
    LOGI "This Acme script requires the following data:"
    LOGI "1.Cloudflare Registered e-mail"
    LOGI "2.Cloudflare Global API Key"
    LOGI "3.The domain name that has been resolved dns to the current server by Cloudflare"
    LOGI "4.The script applies for a certificate. The default installation path is /root/cert "
    confirm "Confirmed?[y/n]" "y"
    if [ $? -eq 0 ]; then
        cd ~
        LOGI "Install Acme-Script"
        curl https://get.acme.sh | sh
        if [ $? -ne 0 ]; then
            LOGE "Failed to install acme script"
            exit 1
        fi
        CF_Domain=""
        CF_GlobalKey=""
        CF_AccountEmail=""
        certPath=/root/cert
        if [ ! -d "$certPath" ]; then
            mkdir $certPath
        else
            rm -rf $certPath
            mkdir $certPath
        fi
        LOGD "Please set a domain name:"
        read -p "Input your domain here:" CF_Domain
        LOGD "Your domain name is set to:${CF_Domain}"
        LOGD "Please set the API key:"
        read -p "Input your key here:" CF_GlobalKey
        LOGD "Your API key is:${CF_GlobalKey}"
        LOGD "Please set up registered email:"
        read -p "Input your email here:" CF_AccountEmail
        LOGD "Your registered email address is:${CF_AccountEmail}"
        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
        if [ $? -ne 0 ]; then
            LOGE "Default CA, Lets'Encrypt fail, script exiting..."
            exit 1
        fi
        export CF_Key="${CF_GlobalKey}"
        export CF_Email=${CF_AccountEmail}
        ~/.acme.sh/acme.sh --issue --dns dns_cf -d ${CF_Domain} -d *.${CF_Domain} --log
        if [ $? -ne 0 ]; then
            LOGE "Certificate issuance failed, script exiting..."
            exit 1
        else
            LOGI "Certificate issued Successfully, Installing..."
        fi
        ~/.acme.sh/acme.sh --installcert -d ${CF_Domain} -d *.${CF_Domain} --ca-file /root/cert/ca.cer \
        --cert-file /root/cert/${CF_Domain}.cer --key-file /root/cert/${CF_Domain}.key \
        --fullchain-file /root/cert/fullchain.cer
        if [ $? -ne 0 ]; then
            LOGE "Certificate installation failed, script exiting..."
            exit 1
        else
            LOGI "Certificate installed Successfully,Turning on automatic updates..."
        fi
        ~/.acme.sh/acme.sh --upgrade --auto-upgrade
        if [ $? -ne 0 ]; then
            LOGE "Auto update setup Failed, script exiting..."
            ls -lah cert
            chmod 755 $certPath
            exit 1
        else
            LOGI "The certificate is installed and auto-renewal is turned on, Specific information is as follows"
            ls -lah cert
            chmod 755 $certPath
        fi
    else
        show_menu
    fi
}

show_usage() {
    echo "x-ui control menu usages: "
    echo "------------------------------------------"
    echo "x-ui              - Enter     Admin menu"
    echo "x-ui start        - Start     x-ui"
    echo "x-ui stop         - Stop      x-ui"
    echo "x-ui restart      - Restart   x-ui"
    echo "x-ui status       - Show      x-ui status"
    echo "x-ui enable       - Enable    x-ui on system startup"
    echo "x-ui disable      - Disable   x-ui on system startup"
    echo "x-ui log          - Check     x-ui logs"
    echo "x-ui v2-ui        - Migrate   v2-ui Account data to x-ui"
    echo "x-ui update       - Update    x-ui"
    echo "x-ui install      - Install   x-ui"
    echo "x-ui uninstall    - Uninstall x-ui"
    echo "------------------------------------------"
}

show_menu() {
    echo -e "
  ${green}x-ui Panel Management Script${plain}
  ${green}0.${plain} exit script
————————————————
  ${green}1.${plain} Install x-ui
  ${green}2.${plain} Update x-ui
  ${green}3.${plain} Uninstall x-ui
————————————————
  ${green}4.${plain} Reset username and password
  ${green}5.${plain} Reset panel settings
  ${green}6.${plain} Set panel port
  ${green}7.${plain} View current panel settings
————————————————
  ${green}8.${plain} Start x-ui
  ${green}9.${plain} stop x-ui
  ${green}10.${plain} Reboot x-ui
  ${green}11.${plain} Check x-ui state
  ${green}12.${plain} Check x-ui logs
————————————————
  ${green}13.${plain} set x-ui Autostart
  ${green}14.${plain} Cancel x-ui Autostart
————————————————
  ${green}15.${plain} 一A key installation bbr (latest kernel)
  ${green}16.${plain} 一Apply for an SSL certificate with one click(acme script)
 "
    show_status
    echo && read -p "Please enter your selection [0-16]: " num

    case "${num}" in
    0)
        exit 0
        ;;
    1)
        check_uninstall && install
        ;;
    2)
        check_install && update
        ;;
    3)
        check_install && uninstall
        ;;
    4)
        check_install && reset_user
        ;;
    5)
        check_install && reset_config
        ;;
    6)
        check_install && set_port
        ;;
    7)
        check_install && check_config
        ;;
    8)
        check_install && start
        ;;
    9)
        check_install && stop
        ;;
    10)
        check_install && restart
        ;;
    11)
        check_install && status
        ;;
    12)
        check_install && show_log
        ;;
    13)
        check_install && enable
        ;;
    14)
        check_install && disable
        ;;
    15)
        install_bbr
        ;;
    16)
        ssl_cert_issue
        ;;
    *)
        LOGE "Please enter the correct number [0-16]"
        ;;
    esac
}

if [[ $# > 0 ]]; then
    case $1 in
    "start")
        check_install 0 && start 0
        ;;
    "stop")
        check_install 0 && stop 0
        ;;
    "restart")
        check_install 0 && restart 0
        ;;
    "status")
        check_install 0 && status 0
        ;;
    "enable")
        check_install 0 && enable 0
        ;;
    "disable")
        check_install 0 && disable 0
        ;;
    "log")
        check_install 0 && show_log 0
        ;;
    "v2-ui")
        check_install 0 && migrate_v2_ui 0
        ;;
    "update")
        check_install 0 && update 0
        ;;
    "install")
        check_uninstall 0 && install 0
        ;;
    "uninstall")
        check_install 0 && uninstall 0
        ;;
    *) show_usage ;;
    esac
else
    show_menu
fi
