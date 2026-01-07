#!/bin/bash

# 3x-UI 中文全自动安装脚本
# 预设：用户名=sinian，密码=sinian，端口=5321，路径=/a
# 自动安装 socat，自动申请 SSL 证书

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}       3x-UI 中文全自动安装脚本${NC}"
echo -e "${GREEN}================================================${NC}"
echo -e "${BLUE}预设配置：${NC}"
echo -e "用户名：${GREEN}sinian${NC}"
echo -e "密  码：${GREEN}sinian${NC}"
echo -e "端  口：${GREEN}5321${NC}"
echo -e "访问路径：${GREEN}/a${NC}"
echo -e "${BLUE}将自动安装 SSL 证书${NC}"
echo ""

# 检查 root 权限
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}错误: 此脚本需要 root 权限运行${NC}"
    exit 1
fi

# 获取服务器 IP（自动检测）
get_server_ip() {
    local ip=""
    # 尝试多个 IP 服务
    local services=(
        "https://api.ipify.org"
        "https://4.ident.me"
        "https://ifconfig.me"
        "https://icanhazip.com"
        "https://checkip.amazonaws.com"
    )
    
    for service in "${services[@]}"; do
        ip=$(curl -s --max-time 3 "$service" 2>/dev/null)
        if [[ -n "$ip" && "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "$ip"
            return 0
        fi
    done
    
    # 如果 API 都失败，尝试本地获取
    ip=$(ip route get 1 | awk '{print $7}' | head -1)
    echo "$ip"
}

# 修复 Ubuntu 软件源（针对 Ubuntu 24.10）
fix_ubuntu_sources() {
    if grep -q "Ubuntu 24.10" /etc/os-release 2>/dev/null; then
        echo -e "${YELLOW}检测到 Ubuntu 24.10，修复软件源...${NC}"
        
        # 备份原文件
        cp /etc/apt/sources.list /etc/apt/sources.list.backup.3xui
        
        # 使用稳定的 Ubuntu 22.04 源
        cat > /etc/apt/sources.list << 'EOF'
deb http://archive.ubuntu.com/ubuntu jammy main restricted universe multiverse
deb http://archive.ubuntu.com/ubuntu jammy-updates main restricted universe multiverse
deb http://archive.ubuntu.com/ubuntu jammy-backports main restricted universe multiverse
deb http://security.ubuntu.com/ubuntu jammy-security main restricted universe multiverse
EOF
        
        # 更新源
        apt-get update >/dev/null 2>&1
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}软件源修复成功${NC}"
        else
            echo -e "${YELLOW}软件源修复失败，继续安装...${NC}"
            # 恢复备份
            mv /etc/apt/sources.list.backup.3xui /etc/apt/sources.list
        fi
    fi
}

# 自动安装 socat（支持多系统）
install_socat_auto() {
    echo -e "${BLUE}检测并安装 socat...${NC}"
    
    # 检查是否已安装
    if command -v socat &>/dev/null; then
        echo -e "${GREEN}socat 已安装${NC}"
        return 0
    fi
    
    # 检测系统类型并安装
    if command -v apt-get &>/dev/null; then
        echo -e "${YELLOW}检测到 Debian/Ubuntu 系统，使用 apt 安装${NC}"
        apt-get update >/dev/null 2>&1
        apt-get install -y socat curl >/dev/null 2>&1
        
    elif command -v yum &>/dev/null; then
        echo -e "${YELLOW}检测到 CentOS/RHEL 系统，使用 yum 安装${NC}"
        yum install -y socat curl >/dev/null 2>&1
        
    elif command -v dnf &>/dev/null; then
        echo -e "${YELLOW}检测到 Fedora 系统，使用 dnf 安装${NC}"
        dnf install -y socat curl >/dev/null 2>&1
        
    elif command -v apk &>/dev/null; then
        echo -e "${YELLOW}检测到 Alpine 系统，使用 apk 安装${NC}"
        apk add socat curl >/dev/null 2>&1
        
    elif command -v pacman &>/dev/null; then
        echo -e "${YELLOW}检测到 Arch 系统，使用 pacman 安装${NC}"
        pacman -Sy --noconfirm socat curl >/dev/null 2>&1
        
    elif command -v zypper &>/dev/null; then
        echo -e "${YELLOW}检测到 openSUSE 系统，使用 zypper 安装${NC}"
        zypper install -y socat curl >/dev/null 2>&1
        
    else
        echo -e "${RED}无法检测包管理器，请手动安装 socat${NC}"
        echo -e "${YELLOW}安装命令参考：${NC}"
        echo -e "Debian/Ubuntu: apt install socat curl"
        echo -e "CentOS/RHEL: yum install socat curl"
        echo -e "Alpine: apk add socat curl"
        return 1
    fi
    
    # 验证安装
    if command -v socat &>/dev/null; then
        echo -e "${GREEN}socat 安装成功${NC}"
        return 0
    else
        echo -e "${RED}socat 安装失败${NC}"
        return 1
    fi
}

# 修改原版 install.sh 的安装过程（静默安装）
install_xui_silent() {
    echo -e "${BLUE}开始安装 3x-UI...${NC}"
    
    # 创建临时安装脚本
    cat > /tmp/install-3xui.sh << 'EOF'
#!/bin/bash

# 静默安装函数
install_xui() {
    # 下载并安装
    bash <(curl -Ls https://raw.githubusercontent.com/MHSanaei/3x-ui/main/install.sh) >/tmp/xui-install.log 2>&1
    
    # 等待安装完成
    sleep 5
    
    # 设置固定配置
    if [ -f "/usr/local/x-ui/x-ui" ]; then
        /usr/local/x-ui/x-ui setting -username "sinian" -password "sinian" -port 5321 -webBasePath "a"
        return 0
    else
        return 1
    fi
}

# 尝试安装
for i in {1..3}; do
    echo "安装尝试 $i/3..."
    if install_xui; then
        echo "安装成功"
        exit 0
    fi
    sleep 2
done

echo "安装失败，请检查日志：/tmp/xui-install.log"
exit 1
EOF
    
    chmod +x /tmp/install-3xui.sh
    /tmp/install-3xui.sh
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}3x-UI 安装成功${NC}"
        return 0
    else
        echo -e "${RED}3x-UI 安装失败${NC}"
        return 1
    fi
}

# 自动申请 SSL 证书
auto_setup_ssl() {
    echo -e "${BLUE}自动申请 SSL 证书...${NC}"
    
    local server_ip=$(get_server_ip)
    
    if [[ -z "$server_ip" ]] || ! [[ "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo -e "${RED}无法获取有效的服务器 IP，跳过 SSL 配置${NC}"
        return 1
    fi
    
    echo -e "${GREEN}服务器 IP: ${server_ip}${NC}"
    
    # 停止面板释放端口 80
    systemctl stop x-ui >/dev/null 2>&1
    sleep 2
    
    # 检查端口 80 是否被占用
    if ss -tulpn | grep -q ":80 "; then
        echo -e "${YELLOW}端口 80 被占用，尝试释放...${NC}"
        # 杀死占用端口 80 的进程（除了必要服务）
        lsof -ti:80 | xargs kill -9 >/dev/null 2>&1
        sleep 2
    fi
    
    # 检查是否已安装 acme.sh
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        echo -e "${YELLOW}安装 acme.sh...${NC}"
        curl -s https://get.acme.sh | sh >/dev/null 2>&1
        ~/.acme.sh/acme.sh --upgrade --auto-upgrade >/dev/null 2>&1
    fi
    
    # 创建证书目录
    mkdir -p /root/cert/ip
    
    echo -e "${YELLOW}正在为 IP ${server_ip} 申请 SSL 证书...${NC}"
    echo -e "${YELLOW}这可能需要几分钟，请稍候...${NC}"
    
    # 申请证书
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt >/dev/null 2>&1
    ~/.acme.sh/acme.sh --issue \
        -d ${server_ip} \
        --standalone \
        --server letsencrypt \
        --certificate-profile shortlived \
        --days 6 \
        --httpport 80 \
        --force >/dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}SSL 证书申请成功${NC}"
        
        # 安装证书
        ~/.acme.sh/acme.sh --installcert -d ${server_ip} \
            --key-file /root/cert/ip/privkey.pem \
            --fullchain-file /root/cert/ip/fullchain.pem \
            --reloadcmd "systemctl restart x-ui" >/dev/null 2>&1
        
        # 设置证书路径
        if [ -f "/usr/local/x-ui/x-ui" ]; then
            /usr/local/x-ui/x-ui cert \
                -webCert /root/cert/ip/fullchain.pem \
                -webCertKey /root/cert/ip/privkey.pem >/dev/null 2>&1
            
            echo -e "${GREEN}SSL 证书配置完成${NC}"
            
            # 启动面板
            systemctl start x-ui >/dev/null 2>&1
            sleep 2
            return 0
        fi
    else
        echo -e "${RED}SSL 证书申请失败${NC}"
        echo -e "${YELLOW}可能原因：${NC}"
        echo -e "1. 端口 80 未开放"
        echo -e "2. 服务器 IP 无法从外部访问"
        echo -e "3. 网络问题"
        echo -e "${YELLOW}安装后可使用命令手动配置：x-ui 18 6${NC}"
        
        # 启动面板（即使 SSL 失败）
        systemctl start x-ui >/dev/null 2>&1
        return 1
    fi
}

# 安装中文管理脚本
install_chinese_script() {
    echo -e "${BLUE}安装中文管理脚本...${NC}"
    
    # 下载中文脚本
    local script_url="https://raw.githubusercontent.com/sinian-liu/3x-ui/main/x-ui.sh"
    
    if curl -s --head "$script_url" | head -n 1 | grep -q "200"; then
        # 备份原脚本
        if [ -f "/usr/local/x-ui/x-ui.sh" ]; then
            cp /usr/local/x-ui/x-ui.sh /usr/local/x-ui/x-ui.sh.backup
        fi
        
        # 下载中文脚本
        curl -sL "$script_url" -o /usr/local/x-ui/x-ui.sh
        
        if [ $? -eq 0 ]; then
            chmod +x /usr/local/x-ui/x-ui.sh
            ln -sf /usr/local/x-ui/x-ui.sh /usr/bin/x-ui
            
            # 设置默认中文
            if [ -f "/usr/local/x-ui/config.json" ]; then
                sed -i 's/"language": "en"/"language": "zh-CN"/g' /usr/local/x-ui/config.json 2>/dev/null
            fi
            
            echo -e "${GREEN}中文脚本安装成功${NC}"
            return 0
        fi
    fi
    
    echo -e "${YELLOW}中文脚本下载失败，使用原版脚本${NC}"
    return 1
}

# 显示安装结果
show_installation_result() {
    echo ""
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}       3x-UI 安装完成！${NC}"
    echo -e "${GREEN}================================================${NC}"
    
    local server_ip=$(get_server_ip)
    local panel_info=""
    
    # 尝试获取面板信息
    if [ -f "/usr/local/x-ui/x-ui" ]; then
        panel_info=$(/usr/local/x-ui/x-ui setting -show true 2>/dev/null)
    fi
    
    # 提取端口和路径
    local port="5321"
    local path="a"
    
    if echo "$panel_info" | grep -q "port:"; then
        port=$(echo "$panel_info" | grep "port:" | awk '{print $2}' | tr -d ',')
    fi
    
    if echo "$panel_info" | grep -q "webBasePath:"; then
        path=$(echo "$panel_info" | grep "webBasePath:" | awk '{print $2}' | tr -d '",')
    fi
    
    # 检查 SSL 证书
    local ssl_status="未配置"
    local protocol="http"
    
    if [ -f "/root/cert/ip/fullchain.pem" ] && [ -f "/root/cert/ip/privkey.pem" ]; then
        ssl_status="已配置"
        protocol="https"
    fi
    
    echo -e "${YELLOW}📋 安装摘要：${NC}"
    echo ""
    echo -e "${BLUE}登录信息：${NC}"
    echo -e "用户名：${GREEN}sinian${NC}"
    echo -e "密  码：${GREEN}sinian${NC}"
    echo ""
    echo -e "${BLUE}访问地址：${NC}"
    
    if [[ -n "$server_ip" ]]; then
        echo -e "${GREEN}${protocol}://${server_ip}:${port}/${path}/${NC}"
        
        # 显示二维码（如果支持）
        if command -v qrencode &>/dev/null; then
            echo ""
            echo -e "${BLUE}访问二维码：${NC}"
            qrencode -t ANSI "${protocol}://${server_ip}:${port}/${path}/"
        elif command -v curl &>/dev/null; then
            echo ""
            echo -e "${BLUE}生成访问链接：${NC}"
            echo "复制上方链接到浏览器访问"
        fi
    else
        echo -e "${GREEN}${protocol}://你的服务器IP:${port}/${path}/${NC}"
    fi
    
    echo ""
    echo -e "${BLUE}SSL 证书：${NC}${ssl_status}"
    if [ "$ssl_status" = "已配置" ]; then
        echo -e "有效期：约6天（自动续期）"
    fi
    
    echo ""
    echo -e "${BLUE}管理命令：${NC}"
    echo -e "${GREEN}x-ui${NC}              # 打开中文管理菜单"
    echo -e "${GREEN}x-ui status${NC}       # 查看状态"
    echo -e "${GREEN}x-ui restart${NC}      # 重启面板"
    echo -e "${GREEN}x-ui 10${NC}           # 查看当前设置"
    echo -e "${GREEN}x-ui 18 6${NC}         # 重新配置 SSL 证书"
    echo ""
    
    if [ "$ssl_status" = "未配置" ]; then
        echo -e "${YELLOW}⚠️ SSL 证书未配置${NC}"
        echo -e "运行命令配置：${GREEN}x-ui 18 6${NC}"
        echo -e "需要确保端口 80 开放"
    fi
    
    echo -e "${BLUE}安全提示：${NC}"
    echo -e "1. 首次登录后立即修改密码"
    echo -e "2. 建议设置防火墙规则"
    echo -e "3. 定期备份配置"
    
    echo ""
    echo -e "${GREEN}================================================${NC}"
}

# 主安装流程
main() {
    echo -e "${BLUE}[1/5] 准备安装环境...${NC}"
    fix_ubuntu_sources
    
    echo -e "${BLUE}[2/5] 安装必要组件...${NC}"
    install_socat_auto
    
    echo -e "${BLUE}[3/5] 安装 3x-UI 面板...${NC}"
    if ! install_xui_silent; then
        echo -e "${RED}面板安装失败，退出安装${NC}"
        exit 1
    fi
    
    echo -e "${BLUE}[4/5] 配置 SSL 证书...${NC}"
    auto_setup_ssl
    
    echo -e "${BLUE}[5/5] 安装中文管理脚本...${NC}"
    install_chinese_script
    
    # 等待服务启动
    sleep 3
    
    # 显示安装结果
    show_installation_result
    
    # 清理临时文件
    rm -f /tmp/install-3xui.sh /tmp/xui-install.log 2>/dev/null
}

# 显示使用说明
show_usage() {
    echo -e "${GREEN}使用说明：${NC}"
    echo ""
    echo -e "一键安装命令："
    echo -e "${GREEN}bash <(curl -Ls https://raw.githubusercontent.com/sinian-liu/Original-3x-ui/main/install-chinese.sh)${NC}"
    echo ""
    echo -e "${BLUE}功能特点：${NC}"
    echo -e "✅ 全自动安装，无需人工干预"
    echo -e "✅ 预设用户名密码：sinian/sinian"
    echo -e "✅ 固定端口：5321，访问路径：/a"
    echo -e "✅ 自动安装 socat（支持多系统）"
    echo -e "✅ 自动申请 SSL 证书"
    echo -e "✅ 自动获取服务器 IP 并显示访问链接"
    echo -e "✅ 中文管理界面"
    echo ""
    echo -e "${YELLOW}系统支持：${NC}"
    echo -e "Ubuntu/Debian/CentOS/RHEL/Fedora/Alpine/Arch/openSUSE"
    echo ""
    echo -e "${RED}注意：${NC}"
    echo -e "1. 需要 root 权限运行"
    echo -e "2. 需要开放端口 80（SSL 证书申请）"
    echo -e "3. 需要开放端口 5321（面板访问）"
}

# 检查是否显示使用说明
if [[ "$1" == "--help" ]] || [[ "$1" == "-h" ]]; then
    show_usage
    exit 0
fi

# 执行安装
main
