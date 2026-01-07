#!/bin/bash

# 3x-ui 中文一键安装脚本

echo "================================================"
echo "       3x-UI 中文版安装脚本"
echo "================================================"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 检查 root 权限
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}错误: 此脚本需要 root 权限运行${NC}"
    exit 1
fi

# 安装原版 3x-ui
install_original() {
    echo -e "${BLUE}步骤 1/3: 安装原版 3x-ui...${NC}"
    bash <(curl -Ls https://raw.githubusercontent.com/MHSanaei/3x-ui/main/install.sh)
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}原版安装成功${NC}"
    else
        echo -e "${RED}原版安装失败${NC}"
        exit 1
    fi
}

# 下载中文脚本
download_chinese_script() {
    echo -e "${BLUE}步骤 2/3: 下载中文管理脚本...${NC}"
    
    # 下载中文版 x-ui.sh
    echo "下载中文管理脚本..."
    wget -q -O /usr/local/x-ui/x-ui.sh https://raw.githubusercontent.com/sinian-liu/Original-3x-ui/main/x-ui.sh
    chmod +x /usr/local/x-ui/x-ui.sh
    
    # 替换 /usr/bin/x-ui 链接
    ln -sf /usr/local/x-ui/x-ui.sh /usr/bin/x-ui
    
    echo -e "${GREEN}中文脚本下载完成${NC}"
}

# 设置中文环境
setup_chinese_env() {
    echo -e "${BLUE}步骤 3/3: 设置中文环境...${NC}"
    
    # 设置面板默认语言为中文
    if [ -f "/usr/local/x-ui/config.json" ]; then
        echo "设置面板默认语言为中文..."
        sed -i 's/"language": "en"/"language": "zh-CN"/g' /usr/local/x-ui/config.json
        
        # 如果配置中没有 language 字段，添加它
        if ! grep -q "language" /usr/local/x-ui/config.json; then
            sed -i 's/"panelSettings": {/"panelSettings": {\n    "language": "zh-CN",/g' /usr/local/x-ui/config.json
        fi
    fi
    
    # 重启服务使配置生效
    echo "重启 3x-ui 服务..."
    systemctl restart x-ui
    
    echo -e "${GREEN}中文环境设置完成${NC}"
}

# 显示完成信息
show_completion() {
    echo ""
    echo "================================================"
    echo -e "${GREEN}       3x-UI 中文版安装完成！${NC}"
    echo "================================================"
    echo ""
    echo -e "${YELLOW}重要信息:${NC}"
    echo -e "面板地址: ${GREEN}http://你的服务器IP:54321${NC}"
    echo -e "默认用户名: ${GREEN}admin${NC}"
    echo -e "默认密码: ${GREEN}admin${NC}"
    echo ""
    echo -e "${YELLOW}使用方法:${NC}"
    echo -e "输入 ${GREEN}x-ui${NC} 打开中文管理菜单"
    echo ""
    echo -e "${YELLOW}管理命令:${NC}"
    echo -e "启动面板: ${GREEN}systemctl start x-ui${NC}"
    echo -e "停止面板: ${GREEN}systemctl stop x-ui${NC}"
    echo -e "重启面板: ${GREEN}systemctl restart x-ui${NC}"
    echo -e "查看状态: ${GREEN}systemctl status x-ui${NC}"
    echo ""
    echo -e "${BLUE}请及时登录面板修改默认密码！${NC}"
    echo "================================================"
}

# 主安装流程
main() {
    install_original
    download_chinese_script
    setup_chinese_env
    show_completion
}

# 执行安装
main
