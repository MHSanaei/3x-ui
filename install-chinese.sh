#!/bin/bash

# 3x-UI 一键静默安装脚本
# 修改自原版 install.sh，自动设置 sinian/sinian，端口5321，路径/a

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
plain='\033[0m'

cur_dir=$(pwd)

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# 预设值
PRESET_USERNAME="sinian"
PRESET_PASSWORD="sinian"
PRESET_PORT="5321"
PRESET_WEB_PATH="a"

# check root
[[ $EUID -ne 0 ]] && echo -e "${RED}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1

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

# 自动确认函数
auto_confirm() {
    return 0
}

# 自动读取函数
auto_read() {
    if [[ "$*" == *"port"* ]] || [[ "$*" == *"端口"* ]]; then
        echo "$PRESET_PORT"
    elif [[ "$*" == *"username"* ]] || [[ "$*" == *"用户名"* ]]; then
        echo "$PRESET_USERNAME"
    elif [[ "$*" == *"password"* ]] || [[ "$*" == *"密码"* ]]; then
        echo "$PRESET_PASSWORD"
    elif [[ "$*" == *"path"* ]] || [[ "$*" == *"路径"* ]]; then
        echo "$PRESET_WEB_PATH"
    elif [[ "$*" == *"[y/n]"* ]]; then
        echo "y"
    elif [[ "$*" == *"Choose"* ]] || [[ "$*" == *"选择"* ]]; then
        echo "2"
    else
        echo ""
    fi
}

# 替换原版函数
confirm() {
    auto_confirm
    return $?
}

# 静默安装基函数
install_base() {
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q curl tar tzdata socat
        ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf -y update && dnf install -y -q curl tar tzdata socat
        ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum -y update && yum install -y curl tar tzdata socat
            else
                dnf -y update && dnf install -y -q curl tar tzdata socat
            fi
        ;;
        arch | manjaro | parch)
            pacman -Syu && pacman -Syu --noconfirm curl tar tzdata socat
        ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper refresh && zypper -q install -y curl tar timezone socat
        ;;
        alpine)
            apk update && apk add curl tar tzdata socat
        ;;
        *)
            apt-get update && apt-get install -y -q curl tar tzdata socat
        ;;
    esac
}

# 静默配置函数
config_after_install() {
    echo -e "${GREEN}正在使用预设配置...${plain}"
    echo -e "${GREEN}用户名: $PRESET_USERNAME${plain}"
    echo -e "${GREEN}密码: $PRESET_PASSWORD${plain}"
    echo -e "${GREEN}端口: $PRESET_PORT${plain}"
    echo -e "${GREEN}访问路径: /$PRESET_WEB_PATH/${plain}"
    
    # 直接设置配置
    ${xui_folder}/x-ui setting -username "$PRESET_USERNAME" -password "$PRESET_PASSWORD" -port "$PRESET_PORT" -webBasePath "$PRESET_WEB_PATH"
    
    # 自动配置 SSL
    echo -e "${GREEN}正在自动配置 SSL 证书...${plain}"
    local server_ip=$(curl -s --max-time 3 https://api.ipify.org || echo "127.0.0.1")
    
    # 停止服务释放端口80
    systemctl stop x-ui 2>/dev/null
    
    # 配置 SSL
    ~/.acme.sh/acme.sh --issue -d ${server_ip} --standalone --httpport 80 --force 2>/dev/null
    if [ $? -eq 0 ]; then
        ~/.acme.sh/acme.sh --installcert -d ${server_ip} \
            --key-file /root/cert/ip/privkey.pem \
            --fullchain-file /root/cert/ip/fullchain.pem \
            --reloadcmd "systemctl restart x-ui"
        
        ${xui_folder}/x-ui cert -webCert /root/cert/ip/fullchain.pem -webCertKey /root/cert/ip/privkey.pem
    fi
    
    # 启动服务
    systemctl start x-ui 2>/dev/null
    
    echo -e "${GREEN}配置完成！${plain}"
}

# 主安装函数
install_x-ui() {
    cd ${xui_folder%/x-ui}/
    
    # 下载最新版本
    tag_version=$(curl -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ ! -n "$tag_version" ]]; then
        tag_version=$(curl -4 -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    fi
    
    echo -e "Got x-ui latest version: ${tag_version}, beginning the installation..."
    curl -4fLRo ${xui_folder}-linux-$(arch).tar.gz https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz
    
    curl -4fLRo /usr/bin/x-ui-temp https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.sh
    
    # 停止旧服务
    if [[ -e ${xui_folder}/ ]]; then
        systemctl stop x-ui 2>/dev/null
        rm ${xui_folder}/ -rf
    fi
    
    # 解压
    tar zxvf x-ui-linux-$(arch).tar.gz
    rm x-ui-linux-$(arch).tar.gz -f
    
    cd x-ui
    chmod +x x-ui bin/xray-linux-$(arch)
    
    # 移动文件
    mv -f /usr/bin/x-ui-temp /usr/bin/x-ui
    chmod +x /usr/bin/x-ui
    
    # 配置
    config_after_install
    
    # 安装服务
    if [[ $release == "alpine" ]]; then
        curl -4fLRo /etc/init.d/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.rc
        chmod +x /etc/init.d/x-ui
        rc-update add x-ui
        rc-service x-ui start
    else
        case "${release}" in
            ubuntu | debian | armbian)
                curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.service.debian
            ;;
            *)
                curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.service.rhel
            ;;
        esac
        
        systemctl daemon-reload
        systemctl enable x-ui
        systemctl start x-ui
    fi
    
    # 安装中文脚本
    curl -sL https://raw.githubusercontent.com/sinian-liu/3x-ui/main/x-ui.sh -o /usr/local/x-ui/x-ui.sh
    chmod +x /usr/local/x-ui/x-ui.sh
    ln -sf /usr/local/x-ui/x-ui.sh /usr/bin/x-ui
    
    # 设置中文
    sed -i 's/"language": "en"/"language": "zh-CN"/g' /usr/local/x-ui/config.json 2>/dev/null
    
    echo -e "${GREEN}x-ui ${tag_version}${plain} installation finished!"
    
    # 显示访问信息
    local server_ip=$(curl -s --max-time 3 https://api.ipify.org || hostname -I | awk '{print $1}')
    echo ""
    echo -e "${GREEN}========================================${plain}"
    echo -e "${GREEN}       安装完成！${plain}"
    echo -e "${GREEN}========================================${plain}"
    echo -e "用户名: ${PRESET_USERNAME}"
    echo -e "密码: ${PRESET_PASSWORD}"
    echo -e "访问地址: https://${server_ip}:${PRESET_PORT}/${PRESET_WEB_PATH}/"
    echo -e "${GREEN}========================================${plain}"
}

# 安装 acme.sh
install_acme() {
    curl -s https://get.acme.sh | sh
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
}

echo -e "${GREEN}开始静默安装...${plain}"
install_base
install_acme
install_x-ui
