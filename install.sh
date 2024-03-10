#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}致命错误：${plain} 请以 root 权限运行此脚本 \n " && exit 1

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "检查系统操作系统失败，请联系作者！" >&2
    exit 1
fi
echo "操作系统版本为：$release"

arch3xui() {
    case "$(uname -m)" in
    x86_64 | x64 | amd64) echo 'amd64' ;;
    i*86 | x86) echo '386' ;;
    armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
    armv7* | armv7 | arm) echo 'armv7' ;;
    armv6* | armv6) echo 'armv6' ;;
    armv5* | armv5) echo 'armv5' ;;
    *) echo -e "${green}不支持的CPU架构！ ${plain}" && rm -f install.sh && exit 1 ;;
    esac
}

echo "信息: $(arch3xui)"

os_version=""
os_version=$(grep -i version_id /etc/os-release | cut -d \" -f2 | cut -d . -f1)

if [[ "${release}" == "centos" ]]; then
    if [[ ${os_version} -lt 8 ]]; then
        echo -e "${red} 请使用 CentOS 8 或更高版本 ${plain}\n" && exit 1
    fi
elif [[ "${release}" == "ubuntu" ]]; then
    if [[ ${os_version} -lt 20 ]]; then
        echo -e "${red} 请使用 Ubuntu 20 或更高版本！${plain}\n" && exit 1
    fi

elif [[ "${release}" == "fedora" ]]; then
    if [[ ${os_version} -lt 36 ]]; then
        echo -e "${red} 请使用 Fedora 36 或更高版本！${plain}\n" && exit 1
    fi

elif [[ "${release}" == "debian" ]]; then
    if [[ ${os_version} -lt 11 ]]; then
        echo -e "${red} 请使用 Debian 11 或更高版本${plain}\n" && exit 1
    fi

elif [[ "${release}" == "almalinux" ]]; then
    if [[ ${os_version} -lt 9 ]]; then
        echo -e "${red} 请使用 AlmaLinux 9 或更高版本 ${plain}\n" && exit 1
    fi

elif [[ "${release}" == "rocky" ]]; then
    if [[ ${os_version} -lt 9 ]]; then
        echo -e "${red} 请使用 RockyLinux 9 或更高版本 ${plain}\n" && exit 1
    fi
elif [[ "${release}" == "arch" ]]; then
    echo "你的操作系统是 ArchLinux"
elif [[ "${release}" == "manjaro" ]]; then
    echo "您的操作系统是 Manjaro"
elif [[ "${release}" == "armbian" ]]; then
    echo "您的操作系统是 Armbian"

else
    echo -e "${red}检查操作系统版本失败，请联系作者！${plain}" && exit 1
fi

install_base() {
    case "${release}" in
    centos | almalinux | rocky)
        yum -y update && yum install -y -q wget curl tar tzdata
        ;;
    fedora)
        dnf -y update && dnf install -y -q wget curl tar tzdata
        ;;
    arch | manjaro)
        pacman -Syu && pacman -Syu --noconfirm wget curl tar tzdata
        ;;
    *)
        apt-get update && apt install -y -q wget curl tar tzdata
        ;;
    esac
}

# This function will be called when user installed x-ui out of security
config_after_install() {
    echo -e "${yellow}安装/更新完成！为了安全起见，建议修改面板设置${plain}"
    read -p "是否要继续修改 [y/n]?": config_confirm
    if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
        read -p "请设置您的用户名：" config_account
        echo -e "${yellow}您的用户名将是：${config_account}${plain}"
        read -p "请设置您的密码：" config_password
        echo -e "${yellow}您的密码将是：${config_password}${plain}"
        read -p "请设置面板端口：" config_port
        echo -e "${yellow}您的面板端口是：${config_port}${plain}"
        echo -e "${yellow}正在初始化，请稍候...${plain}"
        /usr/local/x-ui/x-ui setting -username ${config_account} -password ${config_password}
        echo -e "${yellow}帐户名和密码设置成功！${plain}"
        /usr/local/x-ui/x-ui setting -port ${config_port}
        echo -e "${yellow}面板端口设置成功！${plain}"
    else
        echo -e "${red}取消...${plain}"
        if [[ ! -f "/etc/x-ui/x-ui.db" ]]; then
            local usernameTemp=$(head -c 6 /dev/urandom | base64)
            local passwordTemp=$(head -c 6 /dev/urandom | base64)
            /usr/local/x-ui/x-ui setting -username ${usernameTemp} -password ${passwordTemp}
            echo -e "这是一个全新的安装，出于安全考虑，会生成随机登录信息："
            echo -e "###############################################"
            echo -e "${green}用户名:${usernameTemp}${plain}"
            echo -e "${green}密码:${passwordTemp}${plain}"
            echo -e "###############################################"
            echo -e "${red}如果您忘记了登录信息，您可以输入 X-UI，然后输入 8 以在安装后进行检查${plain}"
        else
            echo -e "${red} 这是您的升级，将保留旧设置，如果您忘记了登录信息，您可以输入 x-ui，然后输入 8 进行检查${plain}"
        fi
    fi
    /usr/local/x-ui/x-ui migrate
}

install_x-ui() {
    cd /usr/local/

    if [ $# == 0 ]; then
        last_version=$(curl -Ls "https://api.github.com/repos/jiulingyun/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$last_version" ]]; then
            echo -e "${red}无法获取 x-ui 版本，可能是由于 Github API 限制，请稍后再试${plain}"
            exit 1
        fi
        echo -e "获取到 x-ui 最新版本：${last_version}，开始安装..."
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(arch3xui).tar.gz https://github.com/jiulingyun/3x-ui/releases/download/${last_version}/x-ui-linux-$(arch3xui).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui 失败，请确保您的服务器可以访问 Github${plain}"
            exit 1
        fi
    else
        last_version=$1
        url="https://github.com/jiulingyun/3x-ui/releases/download/${last_version}/x-ui-linux-$(arch3xui).tar.gz"
        echo -e "开始安装 x-ui$1"
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(arch3xui).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui $1 失败，请检查版本是否存在${plain}"
            exit 1
        fi
    fi

    if [[ -e /usr/local/x-ui/ ]]; then
        systemctl stop x-ui
        rm /usr/local/x-ui/ -rf
    fi

    tar zxvf x-ui-linux-$(arch3xui).tar.gz
    rm x-ui-linux-$(arch3xui).tar.gz -f
    cd x-ui
    chmod +x x-ui

    # Check the system's architecture and rename the file accordingly
    if [[ $(arch3xui) == "armv5" || $(arch3xui) == "armv6" || $(arch3xui) == "armv7" ]]; then
        mv bin/xray-linux-$(arch3xui) bin/xray-linux-arm
        chmod +x bin/xray-linux-arm
    fi

    chmod +x x-ui bin/xray-linux-$(arch3xui)
    cp -f x-ui.service /etc/systemd/system/
    wget --no-check-certificate -O /usr/bin/x-ui https://raw.githubusercontent.com/jiulingyun/3x-ui/main/x-ui.sh
    chmod +x /usr/local/x-ui/x-ui.sh
    chmod +x /usr/bin/x-ui
    config_after_install

    systemctl daemon-reload
    systemctl enable x-ui
    systemctl start x-ui
    echo -e "${green}x-ui ${last_version}${plain} 安装完成，正在运行..."
    echo -e ""
    echo -e "X-UI 控件菜单用法: "
    echo -e "----------------------------------------------"
    echo -e "x-ui              - 进入管理菜单"
    echo -e "x-ui start        - 启动       x-ui"
    echo -e "x-ui stop         - 停止       x-ui"
    echo -e "x-ui restart      - 重启       x-ui"
    echo -e "x-ui status       - 显示       x-ui 状态"
    echo -e "x-ui enable       - 启用       x-ui 开机自启"
    echo -e "x-ui disable      - 禁用       x-ui 开机自启"
    echo -e "x-ui log          - 查看       x-ui 日志"
    echo -e "x-ui banlog       - 查看       Fail2ban 封禁日志"
    echo -e "x-ui update       - 更新       x-ui"
    echo -e "x-ui install      - 安装       x-ui"
    echo -e "x-ui uninstall    - 卸载       x-ui"
    echo -e "----------------------------------------------"
}

echo -e "${green}运行...${plain}"
install_base
install_x-ui $1
