#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# 检查 root 权限
[[ $EUID -ne 0 ]] && echo -e "${red}致命错误：${plain} 请以 root 权限运行此脚本 \n " && exit 1

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "无法检查系统操作系统，请联系作者！" >&2
    exit 1
fi
echo "操作系统版本为：$release"

arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${green}不支持的 CPU 架构！ ${plain}" && rm -f install_zh_cn.sh && exit 1 ;;
    esac
}

echo "架构：$(arch)"

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
    [[ "$1" =~ ^([A-Za-z0-9](-*[A-Za-z0-9])*\.)+(xn--[a-z0-9]{2,}|[A-Za-z]{2,})$ ]] && return 0 || return 1
}

# Port helpers
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
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum -y update && yum install -y cronie curl tar tzdata socat ca-certificates openssl
            else
                dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl
            fi
            ;;
        arch | manjaro | parch)
            pacman -Syu && pacman -Syu --noconfirm cronie curl tar tzdata socat ca-certificates openssl
            ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper refresh && zypper -q install -y cron curl tar timezone socat ca-certificates openssl
            ;;
        alpine)
            apk update && apk add dcron curl tar tzdata socat ca-certificates openssl
            ;;
        *)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl
            ;;
    esac
}

gen_random_string() {
    local length="$1"
    openssl rand -base64 $((length * 2)) \
        | tr -dc 'a-zA-Z0-9' \
        | head -c "$length"
}

install_acme() {
    echo -e "${green}正在安装 acme.sh 以管理 SSL 证书...${plain}"
    cd ~ || return 1
    curl -s https://get.acme.sh | sh > /dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${red}安装 acme.sh 失败${plain}"
        return 1
    else
        echo -e "${green}acme.sh 安装成功${plain}"
    fi
    return 0
}

setup_ssl_certificate() {
    local domain="$1"
    local server_ip="$2"
    local existing_port="$3"
    local existing_webBasePath="$4"

    echo -e "${green}正在设置 SSL 证书...${plain}"

    # 检查 acme.sh 是否已安装
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${yellow}安装 acme.sh 失败，跳过 SSL 设置${plain}"
            return 1
        fi
    fi

    # Create certificate directory
    local certPath="/root/cert/${domain}"
    mkdir -p "$certPath"

    # 颁发证书
    echo -e "${green}正在为 ${domain} 颁发 SSL 证书...${plain}"
    echo -e "${yellow}注意：端口 80 必须开放并可从互联网访问${plain}"

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1
    ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport 80 --force

    if [ $? -ne 0 ]; then
        echo -e "${yellow}为 ${domain} 颁发证书失败${plain}"
        echo -e "${yellow}请确保端口 80 已开放，稍后使用 x-ui 重试${plain}"
        rm -rf ~/.acme.sh/${domain} 2> /dev/null
        rm -rf "$certPath" 2> /dev/null
        return 1
    fi

    # Install certificate
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem \
        --reloadcmd "systemctl restart x-ui" > /dev/null 2>&1

    if [ $? -ne 0 ]; then
        echo -e "${yellow}安装证书失败${plain}"
        return 1
    fi

    # 启用自动续期
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1
    # 安全权限：私钥仅所有者可读
    chmod 600 $certPath/privkey.pem 2> /dev/null
    chmod 644 $certPath/fullchain.pem 2> /dev/null

    # 为面板设置证书
    local webCertFile="/root/cert/${domain}/fullchain.pem"
    local webKeyFile="/root/cert/${domain}/privkey.pem"

    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
        ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile" > /dev/null 2>&1
        echo -e "${green}SSL 证书已成功安装和配置！${plain}"
        return 0
    else
        echo -e "${yellow}未找到证书文件${plain}"
        return 1
    fi
}

# 使用短期配置文件颁发 Let's Encrypt IP 证书（约 6 天有效期）
# 需要 acme.sh 且端口 80 开放用于 HTTP-01 验证
setup_ip_certificate() {
    local ipv4="$1"
    local ipv6="$2" # 可选

    echo -e "${green}正在设置 Let's Encrypt IP 证书（短期配置文件）...${plain}"
    echo -e "${yellow}注意：IP 证书有效期约 6 天，将自动续期。${plain}"
    echo -e "${yellow}默认监听器为端口 80。如果选择其他端口，请确保外部端口 80 转发到该端口。${plain}"

    # 检查 acme.sh
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${red}安装 acme.sh 失败${plain}"
            return 1
        fi
    fi

    # 验证 IP 地址
    if [[ -z "$ipv4" ]]; then
        echo -e "${red}需要 IPv4 地址${plain}"
        return 1
    fi

    if ! is_ipv4 "$ipv4"; then
        echo -e "${red}无效的 IPv4 地址：$ipv4${plain}"
        return 1
    fi

    # 创建证书目录
    local certDir="/root/cert/ip"
    mkdir -p "$certDir"

    # 构建域名参数
    local domain_args="-d ${ipv4}"
    if [[ -n "$ipv6" ]] && is_ipv6 "$ipv6"; then
        domain_args="${domain_args} -d ${ipv6}"
        echo -e "${green}包含 IPv6 地址：${ipv6}${plain}"
    fi

    # 设置自动续期的重载命令（添加 || true 以便在首次安装时不失败）
    local reloadCmd="systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null || true"

    # 选择 HTTP-01 监听器的端口（默认 80，提示覆盖）
    local WebPort=""
    read -rp "用于 ACME HTTP-01 监听器的端口（默认 80）：" WebPort
    WebPort="${WebPort:-80}"
    if ! [[ "${WebPort}" =~ ^[0-9]+$ ]] || ((WebPort < 1 || WebPort > 65535)); then
        echo -e "${red}提供的端口无效。回退到 80。${plain}"
        WebPort=80
    fi
    echo -e "${green}使用端口 ${WebPort} 进行独立验证。${plain}"
    if [[ "${WebPort}" -ne 80 ]]; then
        echo -e "${yellow}提醒：Let's Encrypt 仍然通过端口 80 连接；将外部端口 80 转发到 ${WebPort}。${plain}"
    fi

    # 确保选择的端口可用
    while true; do
        if is_port_in_use "${WebPort}"; then
            echo -e "${yellow}端口 ${WebPort} 正在使用中。${plain}"

            local alt_port=""
            read -rp "输入另一个用于 acme.sh 独立监听器的端口（留空以中止）：" alt_port
            alt_port="${alt_port// /}"
            if [[ -z "${alt_port}" ]]; then
                echo -e "${red}端口 ${WebPort} 繁忙；无法继续。${plain}"
                return 1
            fi
            if ! [[ "${alt_port}" =~ ^[0-9]+$ ]] || ((alt_port < 1 || alt_port > 65535)); then
                echo -e "${red}提供的端口无效。${plain}"
                return 1
            fi
            WebPort="${alt_port}"
            continue
        else
            echo -e "${green}端口 ${WebPort} 空闲，已准备好进行独立验证。${plain}"
            break
        fi
    done

    # 使用短期配置文件颁发证书
    echo -e "${green}正在为 ${ipv4} 颁发 IP 证书...${plain}"
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1

    ~/.acme.sh/acme.sh --issue \
        ${domain_args} \
        --standalone \
        --server letsencrypt \
        --certificate-profile shortlived \
        --days 6 \
        --httpport ${WebPort} \
        --force

    if [ $? -ne 0 ]; then
        echo -e "${red}颁发 IP 证书失败${plain}"
        echo -e "${yellow}请确保端口 ${WebPort} 可访问（或从外部端口 80 转发）${plain}"
        # 清理 acme.sh 数据（如果指定了 IPv4 和 IPv6）
        rm -rf ~/.acme.sh/${ipv4} 2> /dev/null
        [[ -n "$ipv6" ]] && rm -rf ~/.acme.sh/${ipv6} 2> /dev/null
        rm -rf ${certDir} 2> /dev/null
        return 1
    fi

    echo -e "${green}证书颁发成功，正在安装...${plain}"

    # 安装证书
    # 注意：如果 reloadcmd 失败，acme.sh 可能会报告“Reload error”并以非零退出，
    # 但证书文件仍然已安装。我们检查文件而不是退出码。
    ~/.acme.sh/acme.sh --installcert -d ${ipv4} \
        --key-file "${certDir}/privkey.pem" \
        --fullchain-file "${certDir}/fullchain.pem" \
        --reloadcmd "${reloadCmd}" 2>&1 || true

    # 验证证书文件是否存在（不要依赖退出码 - reloadcmd 失败会导致非零）
    if [[ ! -f "${certDir}/fullchain.pem" || ! -f "${certDir}/privkey.pem" ]]; then
        echo -e "${red}安装后未找到证书文件${plain}"
        # 清理 acme.sh 数据（如果指定了 IPv4 和 IPv6）
        rm -rf ~/.acme.sh/${ipv4} 2> /dev/null
        [[ -n "$ipv6" ]] && rm -rf ~/.acme.sh/${ipv6} 2> /dev/null
        rm -rf ${certDir} 2> /dev/null
        return 1
    fi

    echo -e "${green}证书文件安装成功${plain}"

    # 为 acme.sh 启用自动升级（确保 cron 作业运行）
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1

    # 安全权限：私钥仅所有者可读
    chmod 600 ${certDir}/privkey.pem 2> /dev/null
    chmod 644 ${certDir}/fullchain.pem 2> /dev/null

    # 配置面板使用证书
    echo -e "${green}正在为面板设置证书路径...${plain}"
    ${xui_folder}/x-ui cert -webCert "${certDir}/fullchain.pem" -webCertKey "${certDir}/privkey.pem"

    if [ $? -ne 0 ]; then
        echo -e "${yellow}警告：无法自动设置证书路径${plain}"
        echo -e "${yellow}证书文件位于：${plain}"
        echo -e "  证书：${certDir}/fullchain.pem"
        echo -e "  私钥：${certDir}/privkey.pem"
    else
        echo -e "${green}证书路径配置成功${plain}"
    fi

    echo -e "${green}IP 证书已成功安装和配置！${plain}"
    echo -e "${green}证书有效期约 6 天，通过 acme.sh cron 作业自动续期。${plain}"
    echo -e "${yellow}acme.sh 将在到期前自动续期并重载 x-ui。${plain}"
    return 0
}

# 通过 acme.sh 进行全面的 SSL 证书手动颁发
ssl_cert_issue() {
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep 'webBasePath:' | awk -F': ' '{print $2}' | tr -d '[:space:]' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep 'port:' | awk -F': ' '{print $2}' | tr -d '[:space:]')

    # 首先检查 acme.sh
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        echo "未找到 acme.sh。正在安装..."
        cd ~ || return 1
        curl -s https://get.acme.sh | sh
        if [ $? -ne 0 ]; then
            echo -e "${red}安装 acme.sh 失败${plain}"
            return 1
        else
            echo -e "${green}acme.sh 安装成功${plain}"
        fi
    fi

    # 获取域名，需要验证
    local domain=""
    while true; do
        read -rp "请输入您的域名：" domain
        domain="${domain// /}" # 修剪空格

        if [[ -z "$domain" ]]; then
            echo -e "${red}域名不能为空。请重试。${plain}"
            continue
        fi

        if ! is_domain "$domain"; then
            echo -e "${red}无效的域名格式：${domain}。请输入有效的域名。${plain}"
            continue
        fi

        break
    done
    echo -e "${green}您的域名为：${domain}，正在检查...${plain}"
    SSL_ISSUED_DOMAIN="${domain}"

    # 检测现有证书，如果存在则重用
    local cert_exists=0
    if ~/.acme.sh/acme.sh --list 2> /dev/null | awk '{print $1}' | grep -Fxq "${domain}"; then
        cert_exists=1
        local certInfo=$(~/.acme.sh/acme.sh --list 2> /dev/null | grep -F "${domain}")
        echo -e "${yellow}找到 ${domain} 的现有证书，将重用它。${plain}"
        [[ -n "${certInfo}" ]] && echo "$certInfo"
    else
        echo -e "${green}您的域名已准备好颁发证书...${plain}"
    fi

    # 为证书创建目录
    certPath="/root/cert/${domain}"
    if [ ! -d "$certPath" ]; then
        mkdir -p "$certPath"
    else
        rm -rf "$certPath"
        mkdir -p "$certPath"
    fi

    # 获取独立服务器的端口号
    local WebPort=80
    read -rp "请选择要使用的端口（默认 80）：" WebPort
    if [[ ${WebPort} -gt 65535 || ${WebPort} -lt 1 ]]; then
        echo -e "${yellow}您的输入 ${WebPort} 无效，将使用默认端口 80。${plain}"
        WebPort=80
    fi
    echo -e "${green}将使用端口：${WebPort} 颁发证书。请确保此端口已开放。${plain}"

    # 暂时停止面板
    echo -e "${yellow}正在暂时停止面板...${plain}"
    systemctl stop x-ui 2> /dev/null || rc-service x-ui stop 2> /dev/null

    if [[ ${cert_exists} -eq 0 ]]; then
        # 颁发证书
        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
        ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport ${WebPort} --force
        if [ $? -ne 0 ]; then
            echo -e "${red}颁发证书失败，请检查日志。${plain}"
            rm -rf ~/.acme.sh/${domain}
            systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null
            return 1
        else
            echo -e "${green}颁发证书成功，正在安装证书...${plain}"
        fi
    else
        echo -e "${green}使用现有证书，正在安装证书...${plain}"
    fi

    # 设置重载命令
    reloadCmd="systemctl restart x-ui || rc-service x-ui restart"
    echo -e "${green}ACME 的默认 --reloadcmd 为：${yellow}systemctl restart x-ui || rc-service x-ui restart${plain}"
    echo -e "${green}此命令将在每次颁发和续期证书时运行。${plain}"
    read -rp "您是否要修改 ACME 的 --reloadcmd？（y/n）：" setReloadcmd
    if [[ "$setReloadcmd" == "y" || "$setReloadcmd" == "Y" ]]; then
        echo -e "\n${green}\t1.${plain} 预设：systemctl reload nginx ; systemctl restart x-ui"
        echo -e "${green}\t2.${plain} 输入自定义命令"
        echo -e "${green}\t0.${plain} 保持默认 reloadcmd"
        read -rp "选择一个选项：" choice
        case "$choice" in
            1)
                echo -e "${green}Reloadcmd 为：systemctl reload nginx ; systemctl restart x-ui${plain}"
                reloadCmd="systemctl reload nginx ; systemctl restart x-ui"
                ;;
            2)
                echo -e "${yellow}建议将 x-ui restart 放在末尾${plain}"
                read -rp "请输入您的自定义 reloadcmd：" reloadCmd
                echo -e "${green}Reloadcmd 为：${reloadCmd}${plain}"
                ;;
            *)
                echo -e "${green}保持默认 reloadcmd${plain}"
                ;;
        esac
    fi

    # 安装证书
    local installOutput=""
    installOutput=$(~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem --reloadcmd "${reloadCmd}" 2>&1)
    local installRc=$?
    echo "${installOutput}"

    local installWroteFiles=0
    if echo "${installOutput}" | grep -q "Installing key to:" && echo "${installOutput}" | grep -q "Installing full chain to:"; then
        installWroteFiles=1
    fi

    if [[ -f "/root/cert/${domain}/privkey.pem" && -f "/root/cert/${domain}/fullchain.pem" && (${installRc} -eq 0 || ${installWroteFiles} -eq 1) ]]; then
        echo -e "${green}安装证书成功，正在启用自动续期...${plain}"
    else
        echo -e "${red}安装证书失败，退出。${plain}"
        if [[ ${cert_exists} -eq 0 ]]; then
            rm -rf ~/.acme.sh/${domain}
        fi
        systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null
        return 1
    fi

    # 启用自动续期
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        echo -e "${yellow}自动续期设置出现问题，证书详情：${plain}"
        ls -lah /root/cert/${domain}/
        # 安全权限：私钥仅所有者可读
        chmod 600 $certPath/privkey.pem 2> /dev/null
        chmod 644 $certPath/fullchain.pem 2> /dev/null
    else
        echo -e "${green}自动续期成功，证书详情：${plain}"
        ls -lah /root/cert/${domain}/
        # 安全权限：私钥仅所有者可读
        chmod 600 $certPath/privkey.pem 2> /dev/null
        chmod 644 $certPath/fullchain.pem 2> /dev/null
    fi

    # 启动面板
    systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null

    # 证书安装成功后提示用户设置面板路径
    read -rp "您是否要为面板设置此证书？（y/n）：" setPanel
    if [[ "$setPanel" == "y" || "$setPanel" == "Y" ]]; then
        local webCertFile="/root/cert/${domain}/fullchain.pem"
        local webKeyFile="/root/cert/${domain}/privkey.pem"

        if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
            ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
            echo -e "${green}已为面板设置证书路径${plain}"
            echo -e "${green}证书文件：$webCertFile${plain}"
            echo -e "${green}私钥文件：$webKeyFile${plain}"
            echo ""
            echo -e "${green}访问 URL：https://${domain}:${existing_port}/${existing_webBasePath}${plain}"
            echo -e "${yellow}面板将重启以应用 SSL 证书...${plain}"
            systemctl restart x-ui 2> /dev/null || rc-service x-ui restart 2> /dev/null
        else
            echo -e "${red}错误：未找到域名 $domain 的证书或私钥文件。${plain}"
        fi
    else
        echo -e "${yellow}跳过面板路径设置。${plain}"
    fi

    return 0
}

# 可重用的交互式 SSL 设置（域名或 IP）
# 将全局 `SSL_HOST` 设置为所选的域名/IP，用于访问 URL
prompt_and_setup_ssl() {
    local panel_port="$1"
    local web_base_path="$2"
    local server_ip="$3"

    local ssl_choice=""
    SSL_SCHEME="https"

    echo -e "${yellow}选择 SSL 证书设置方法：${plain}"
    echo -e "${green}1.${plain} Let's Encrypt 域名证书（90 天有效期，自动续期）"
    echo -e "${green}2.${plain} Let's Encrypt IP 地址证书（6 天有效期，自动续期）"
    echo -e "${green}3.${plain} 自定义 SSL 证书（现有文件路径）"
    echo -e "${green}4.${plain} 跳过 SSL（高级 — 仅限反向代理 / SSH 隧道后使用）"
    echo -e "${blue}注意：${plain}选项 1 和 2 需要端口 80 开放。选项 3 需要手动指定路径。"
    echo -e "${blue}注意：${plain}选项 4 通过纯 HTTP 提供面板服务 — 仅在 nginx/Caddy 或 SSH 隧道后安全。"
    read -rp "选择一个选项（默认为 2 用于 IP）：" ssl_choice
    ssl_choice="${ssl_choice// /}" # 修剪空格

    # 如果输入为空或无效（不是 1、3 或 4），则默认为 2（IP 证书）
    if [[ "$ssl_choice" != "1" && "$ssl_choice" != "3" && "$ssl_choice" != "4" ]]; then
        ssl_choice="2"
    fi

    case "$ssl_choice" in
        1)
            # 用户选择了 Let's Encrypt 域名选项
            echo -e "${green}使用 Let's Encrypt 颁发域名证书...${plain}"
            if ssl_cert_issue; then
                local cert_domain="${SSL_ISSUED_DOMAIN}"
                if [[ -z "${cert_domain}" ]]; then
                    cert_domain=$(~/.acme.sh/acme.sh --list 2> /dev/null | tail -1 | awk '{print $1}')
                fi

                if [[ -n "${cert_domain}" ]]; then
                    SSL_HOST="${cert_domain}"
                    echo -e "${green}✓ SSL 证书配置成功，域名：${cert_domain}${plain}"
                else
                    echo -e "${yellow}SSL 设置可能已完成，但域名提取失败${plain}"
                    SSL_HOST="${server_ip}"
                fi
            else
                echo -e "${red}域名模式 SSL 证书设置失败。${plain}"
                SSL_HOST="${server_ip}"
            fi
            ;;
        2)
            # 用户选择了 Let's Encrypt IP 证书选项
            echo -e "${green}使用 Let's Encrypt 颁发 IP 证书（短期配置文件）...${plain}"

            # 询问可选的 IPv6
            local ipv6_addr=""
            read -rp "您是否有要包含的 IPv6 地址？（留空以跳过）：" ipv6_addr
            ipv6_addr="${ipv6_addr// /}" # 修剪空格

            # 如果面板正在运行则停止（需要端口 80）
            if [[ $release == "alpine" ]]; then
                rc-service x-ui stop > /dev/null 2>&1
            else
                systemctl stop x-ui > /dev/null 2>&1
            fi

            setup_ip_certificate "${server_ip}" "${ipv6_addr}"
            if [ $? -eq 0 ]; then
                SSL_HOST="${server_ip}"
                echo -e "${green}✓ Let's Encrypt IP 证书配置成功${plain}"
            else
                echo -e "${red}✗ IP 证书设置失败。请检查端口 80 是否开放。${plain}"
                SSL_HOST="${server_ip}"
            fi
            ;;
        3)
            # 用户选择了自定义路径（用户提供）选项
            echo -e "${green}使用自定义现有证书...${plain}"
            local custom_cert=""
            local custom_key=""
            local custom_domain=""

            # 3.1 请求域名以稍后组合面板 URL
            read -rp "请输入证书颁发的域名：" custom_domain
            custom_domain="${custom_domain// /}" # 删除空格

            # 3.2 循环获取证书路径
            while true; do
                read -rp "输入证书路径（关键词：.crt / fullchain）：" custom_cert
                # 如果存在引号则删除
                custom_cert=$(echo "$custom_cert" | tr -d '"' | tr -d "'")

                if [[ -f "$custom_cert" && -r "$custom_cert" && -s "$custom_cert" ]]; then
                    break
                elif [[ ! -f "$custom_cert" ]]; then
                    echo -e "${red}错误：文件不存在！请重试。${plain}"
                elif [[ ! -r "$custom_cert" ]]; then
                    echo -e "${red}错误：文件存在但不可读（检查权限）！${plain}"
                else
                    echo -e "${red}错误：文件为空！${plain}"
                fi
            done

            # 3.3 循环获取私钥路径
            while true; do
                read -rp "输入私钥路径（关键词：.key / privatekey）：" custom_key
                # 如果存在引号则删除
                custom_key=$(echo "$custom_key" | tr -d '"' | tr -d "'")

                if [[ -f "$custom_key" && -r "$custom_key" && -s "$custom_key" ]]; then
                    break
                elif [[ ! -f "$custom_key" ]]; then
                    echo -e "${red}错误：文件不存在！请重试。${plain}"
                elif [[ ! -r "$custom_key" ]]; then
                    echo -e "${red}错误：文件存在但不可读（检查权限）！${plain}"
                else
                    echo -e "${red}错误：文件为空！${plain}"
                fi
            done

            # 3.4 通过 x-ui 二进制应用设置
            ${xui_folder}/x-ui cert -webCert "$custom_cert" -webCertKey "$custom_key" > /dev/null 2>&1

            # 设置 SSL_HOST 以组合面板 URL
            if [[ -n "$custom_domain" ]]; then
                SSL_HOST="$custom_domain"
            else
                SSL_HOST="${server_ip}"
            fi

            echo -e "${green}✓ 自定义证书路径已应用。${plain}"
            echo -e "${yellow}注意：您需要负责外部续期这些文件。${plain}"

            systemctl restart x-ui > /dev/null 2>&1 || rc-service x-ui restart > /dev/null 2>&1
            ;;
        4)
            echo ""
            echo -e "${red}⚠ 面板将在没有 SSL/TLS 的情况下安装。${plain}"
            echo -e "${yellow}登录凭据和 cookie 将以纯 HTTP 传输。${plain}"
            echo -e "${yellow}仅在以下情况安全：${plain}"
            echo -e "${yellow}  • 反向代理（nginx、Caddy、Traefik）为您终止 TLS，或${plain}"
            echo -e "${yellow}  • 您仅通过 SSH 隧道访问面板${plain}"
            echo ""

            SSL_SCHEME="http"
            SSL_HOST="${server_ip}"

            local bind_local=""
            read -rp "将面板绑定到 127.0.0.1 吗？（推荐 — 强制使用 SSH 隧道 / 反向代理访问）[y/N]：" bind_local
            if [[ "$bind_local" == "y" || "$bind_local" == "Y" ]]; then
                ${xui_folder}/x-ui setting -listenIP "127.0.0.1" > /dev/null 2>&1
                SSL_HOST="127.0.0.1"
                echo -e "${green}✓ 面板已绑定到 127.0.0.1。现在无法从公共互联网访问。${plain}"
                echo ""
                echo -e "${green}SSH 端口转发 — 从本地计算机打开面板：${plain}"
                echo -e "  标准 SSH 命令："
                echo -e "  ${yellow}ssh -L 2222:127.0.0.1:${panel_port} root@${server_ip}${plain}"
                echo -e "  如果使用 SSH 密钥："
                echo -e "  ${yellow}ssh -i <sshkeypath> -L 2222:127.0.0.1:${panel_port} root@${server_ip}${plain}"
                echo -e "  然后在浏览器中打开："
                echo -e "  ${yellow}http://localhost:2222/${web_base_path}${plain}"
                echo ""
                echo -e "${yellow}替代方案：将反向代理（nginx/Caddy）指向 127.0.0.1:${panel_port} 并让它终止 TLS。${plain}"
            else
                echo -e "${yellow}面板将通过纯 HTTP 在所有接口上监听。确保前面有其他东西在终止 TLS。${plain}"
            fi

            systemctl restart x-ui > /dev/null 2>&1 || rc-service x-ui restart > /dev/null 2>&1
            echo -e "${green}✓ 已跳过 SSL 设置。${plain}"
            ;;
        *)
            echo -e "${red}无效选项。跳过 SSL 设置。${plain}"
            SSL_HOST="${server_ip}"
            ;;
    esac
}

config_after_install() {
    local existing_hasDefaultCredential=$(${xui_folder}/x-ui setting -show true | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    # 通过检查 cert: 行是否存在且后面有内容来正确检测空证书
    local existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
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
        local response=$(curl -s -w "\n%{http_code}" --max-time 3 "${ip_address}" 2> /dev/null)
        local http_code=$(echo "$response" | tail -n1)
        local ip_result=$(echo "$response" | head -n-1 | tr -d '[:space:]"')
        if [[ "${http_code}" == "200" && "${ip_result}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            server_ip="${ip_result}"
            break
        fi
    done

    if [[ -z "$server_ip" ]]; then
        echo -e "${yellow}无法从任何提供商自动检测服务器 IP。${plain}"
        while [[ -z "$server_ip" ]]; do
            read -rp "请输入您服务器的公共 IPv4 地址：" server_ip
            server_ip="${server_ip// /}"
            if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                echo -e "${red}无效的 IPv4 地址。请重试。${plain}"
                server_ip=""
            fi
        done
    fi

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath=$(gen_random_string 18)
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)

            read -rp "您是否要自定义面板端口设置？（如果否，将应用随机端口）[y/n]：" config_confirm
            if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
                read -rp "请设置面板端口：" config_port
                echo -e "${yellow}您的面板端口为：${config_port}${plain}"
            else
                local config_port=$(shuf -i 1024-62000 -n 1)
                echo -e "${yellow}生成的随机端口：${config_port}${plain}"
            fi

            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"

            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     SSL 证书设置（推荐）   ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}强烈建议使用 SSL。仅在反向代理${plain}"
            echo -e "${yellow}或 SSH 隧道为您处理 TLS 时跳过。${plain}"
            echo -e "${yellow}Let's Encrypt 现在支持域名和 IP 地址！${plain}"
            echo ""

            prompt_and_setup_ssl "${config_port}" "${config_webBasePath}" "${server_ip}"

            # 获取 API 令牌以显示
            local config_apiToken=$(${xui_folder}/x-ui setting -getApiToken true | grep -Eo 'apiToken: .+' | awk '{print $2}')

            # 显示最终凭据和访问信息
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     面板安装完成！         ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}用户名：    ${config_username}${plain}"
            echo -e "${green}密码：    ${config_password}${plain}"
            echo -e "${green}端口：        ${config_port}${plain}"
            echo -e "${green}WebBasePath： ${config_webBasePath}${plain}"
            echo -e "${green}访问 URL：  ${SSL_SCHEME}://${SSL_HOST}:${config_port}/${config_webBasePath}${plain}"
            echo -e "${green}API 令牌：   ${config_apiToken}${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}⚠ 重要：请安全保存这些凭据！${plain}"
            if [[ "$SSL_SCHEME" == "https" ]]; then
                echo -e "${yellow}⚠ SSL 证书：已启用并配置${plain}"
            else
                echo -e "${yellow}⚠ SSL 证书：已跳过 — 面板仅为 HTTP。请使用反向代理或 SSH 隧道。${plain}"
            fi
        else
            local config_webBasePath=$(gen_random_string 18)
            echo -e "${yellow}WebBasePath 缺失或太短。正在生成新的...${plain}"
            ${xui_folder}/x-ui setting -webBasePath "${config_webBasePath}"
            echo -e "${green}新的 WebBasePath：${config_webBasePath}${plain}"

            # 如果面板已安装但未配置证书，现在提示 SSL
            if [[ -z "${existing_cert}" ]]; then
                echo ""
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${green}     SSL 证书设置（推荐）   ${plain}"
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${yellow}Let's Encrypt 现在支持域名和 IP 地址！${plain}"
                echo ""
                prompt_and_setup_ssl "${existing_port}" "${config_webBasePath}" "${server_ip}"
                echo -e "${green}访问 URL：  ${SSL_SCHEME}://${SSL_HOST}:${existing_port}/${config_webBasePath}${plain}"
            else
                # 如果证书已存在，仅显示访问 URL
                echo -e "${green}访问 URL： https://${server_ip}:${existing_port}/${config_webBasePath}${plain}"
            fi
        fi
    else
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)

            echo -e "${yellow}检测到默认凭据。需要安全更新...${plain}"
            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}"
            echo -e "生成了新的随机登录凭据："
            echo -e "###############################################"
            echo -e "${green}用户名：${config_username}${plain}"
            echo -e "${green}密码：${config_password}${plain}"
            echo -e "###############################################"
        else
            echo -e "${green}用户名、密码和 WebBasePath 已正确设置。${plain}"
        fi

        # 现有安装：如果未配置证书，提示用户进行 SSL 设置
        # 通过检查 cert: 行是否存在且后面有内容来正确检测空证书
        existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
        if [[ -z "$existing_cert" ]]; then
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     SSL 证书设置（推荐）   ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}Let's Encrypt 现在支持域名和 IP 地址！${plain}"
            echo ""
            prompt_and_setup_ssl "${existing_port}" "${existing_webBasePath}" "${server_ip}"
            echo -e "${green}访问 URL：  ${SSL_SCHEME}://${SSL_HOST}:${existing_port}/${existing_webBasePath}${plain}"
        else
            echo -e "${green}SSL 证书已配置。无需操作。${plain}"
        fi
    fi

    ${xui_folder}/x-ui migrate
}

install_x-ui() {
    cd ${xui_folder%/x-ui}/

    # 下载资源
    if [ $# == 0 ]; then
        tag_version=$(curl -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${yellow}尝试使用 IPv4 获取版本...${plain}"
            tag_version=$(curl -4 -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
            if [[ ! -n "$tag_version" ]]; then
                echo -e "${red}获取 x-ui 版本失败，可能是由于 GitHub API 限制，请稍后重试${plain}"
                exit 1
            fi
        fi
        echo -e "获取到 x-ui 最新版本：${tag_version}，开始安装..."
        curl -4fLRo ${xui_folder}-linux-$(arch).tar.gz https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui 失败，请确保您的服务器可以访问 GitHub ${plain}"
            exit 1
        fi
    else
        tag_version=$1
        tag_version_numeric=${tag_version#v}
        min_version="2.3.5"

        if [[ "$(printf '%s\n' "$min_version" "$tag_version_numeric" | sort -V | head -n1)" != "$min_version" ]]; then
            echo -e "${red}请使用更新的版本（至少 v2.3.5）。退出安装。${plain}"
            exit 1
        fi

        url="https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz"
        echo -e "开始安装 x-ui $1"
        curl -4fLRo ${xui_folder}-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui $1 失败，请检查版本是否存在 ${plain}"
            exit 1
        fi
    fi
    curl -4fLRo /usr/bin/x-ui-temp https://raw.githubusercontent.com/xsdxq-null/3X-UI-CN/main/x-ui_zh_cn.sh
    if [[ $? -ne 0 ]]; then
        echo -e "${red}下载 x-ui_zh_cn.sh 失败${plain}"
        exit 1
    fi

    # 停止 x-ui 服务并删除旧资源
    if [[ -e ${xui_folder}/ ]]; then
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop
        else
            systemctl stop x-ui
        fi
        rm ${xui_folder}/ -rf
    fi

    # 解压资源并设置权限
    tar zxvf x-ui-linux-$(arch).tar.gz
    rm x-ui-linux-$(arch).tar.gz -f

    cd x-ui
    chmod +x x-ui
    chmod +x x-ui_zh_cn.sh

    # 检查系统架构并相应重命名文件
    if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm
        chmod +x bin/xray-linux-arm
    fi
    chmod +x x-ui bin/xray-linux-$(arch)

    # 更新 x-ui cli 并设置权限
    mv -f /usr/bin/x-ui-temp /usr/bin/x-ui
    chmod +x /usr/bin/x-ui
    mkdir -p /var/log/x-ui
    config_after_install

    # Etckeeper 兼容性
    if [ -d "/etc/.git" ]; then
        if [ -f "/etc/.gitignore" ]; then
            if ! grep -q "x-ui/x-ui.db" "/etc/.gitignore"; then
                echo "" >> "/etc/.gitignore"
                echo "x-ui/x-ui.db" >> "/etc/.gitignore"
                echo -e "${green}已将 x-ui.db 添加到 /etc/.gitignore 以用于 etckeeper${plain}"
            fi
        else
            echo "x-ui/x-ui.db" > "/etc/.gitignore"
            echo -e "${green}已创建 /etc/.gitignore 并添加 x-ui.db 以用于 etckeeper${plain}"
        fi
    fi

    if [[ $release == "alpine" ]]; then
        curl -4fLRo /etc/init.d/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.rc
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui.rc 失败${plain}"
            exit 1
        fi
        chmod +x /etc/init.d/x-ui
        rc-update add x-ui
        rc-service x-ui start
    else
        # 安装 systemd 服务文件
        service_installed=false

        if [ -f "x-ui.service" ]; then
            echo -e "${green}在解压文件中找到 x-ui.service，正在安装...${plain}"
            cp -f x-ui.service ${xui_service}/ > /dev/null 2>&1
            if [[ $? -eq 0 ]]; then
                service_installed=true
            fi
        fi

        if [ "$service_installed" = false ]; then
            case "${release}" in
                ubuntu | debian | armbian)
                    if [ -f "x-ui.service.debian" ]; then
                        echo -e "${green}在解压文件中找到 x-ui.service.debian，正在安装...${plain}"
                        cp -f x-ui.service.debian ${xui_service}/x-ui.service > /dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                    ;;
                arch | manjaro | parch)
                    if [ -f "x-ui.service.arch" ]; then
                        echo -e "${green}在解压文件中找到 x-ui.service.arch，正在安装...${plain}"
                        cp -f x-ui.service.arch ${xui_service}/x-ui.service > /dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                    ;;
                *)
                    if [ -f "x-ui.service.rhel" ]; then
                        echo -e "${green}在解压文件中找到 x-ui.service.rhel，正在安装...${plain}"
                        cp -f x-ui.service.rhel ${xui_service}/x-ui.service > /dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                    ;;
            esac
        fi

        # 如果 tar.gz 中未找到服务文件，从 GitHub 下载
        if [ "$service_installed" = false ]; then
            echo -e "${yellow}在 tar.gz 中未找到服务文件，从 GitHub 下载...${plain}"
            case "${release}" in
                ubuntu | debian | armbian)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.service.debian > /dev/null 2>&1
                    ;;
                arch | manjaro | parch)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.service.arch > /dev/null 2>&1
                    ;;
                *)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.service.rhel > /dev/null 2>&1
                    ;;
            esac

            if [[ $? -ne 0 ]]; then
                echo -e "${red}从 GitHub 安装 x-ui.service 失败${plain}"
                exit 1
            fi
            service_installed=true
        fi

        if [ "$service_installed" = true ]; then
            echo -e "${green}正在设置 systemd 单元...${plain}"
            chown root:root ${xui_service}/x-ui.service > /dev/null 2>&1
            chmod 644 ${xui_service}/x-ui.service > /dev/null 2>&1
            systemctl daemon-reload
            systemctl enable x-ui
            systemctl start x-ui
        else
            echo -e "${red}安装 x-ui.service 文件失败${plain}"
            exit 1
        fi
    fi

    echo -e "${green}x-ui ${tag_version}${plain} 安装完成，正在运行..."
    echo -e ""
    echo -e "╔═══════════════════════════════════════════════════════╗
║  ${blue}x-ui 控制菜单用法（子命令）：${plain}                          ║
║                                                       ║
║  ${blue}x-ui${plain}              - 管理脚本                         ║
║  ${blue}x-ui start${plain}        - 启动                             ║
║  ${blue}x-ui stop${plain}         - 停止                             ║
║  ${blue}x-ui restart${plain}      - 重启                             ║
║  ${blue}x-ui status${plain}       - 当前状态                         ║
║  ${blue}x-ui settings${plain}     - 当前设置                         ║
║  ${blue}x-ui enable${plain}       - 启用开机自启                     ║
║  ${blue}x-ui disable${plain}      - 禁用开机自启                     ║
║  ${blue}x-ui log${plain}          - 查看日志                         ║
║  ${blue}x-ui banlog${plain}       - 查看 Fail2ban 禁止日志           ║
║  ${blue}x-ui update${plain}       - 更新                             ║
║  ${blue}x-ui legacy${plain}       - 旧版本                           ║
║  ${blue}x-ui install${plain}      - 安装                             ║
║  ${blue}x-ui uninstall${plain}    - 卸载                             ║
╚═══════════════════════════════════════════════════════╝"
}

echo -e "${green}正在运行...${plain}"
install_base
install_x-ui $1
