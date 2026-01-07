#!/bin/bash

# 3x-ui 中文版安装脚本
# 作者: 你的用户名

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}       3x-UI 中文版安装脚本${NC}"
echo -e "${GREEN}================================================${NC}"
echo ""

# 检查 root 权限
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}错误: 此脚本需要 root 权限运行${NC}"
    exit 1
fi

# 1. 安装原版 3x-ui
echo -e "${BLUE}步骤 1/3: 安装原版 3x-ui...${NC}"
bash <(curl -Ls https://raw.githubusercontent.com/MHSanaei/3x-ui/main/install.sh)

if [ $? -ne 0 ]; then
    echo -e "${RED}原版安装失败，请检查网络连接${NC}"
    exit 1
fi

echo -e "${GREEN}原版安装成功${NC}"

# 2. 下载中文管理脚本
echo -e "${BLUE}步骤 2/3: 下载中文管理脚本...${NC}"

# 创建备份
if [ -f "/usr/local/x-ui/x-ui.sh" ]; then
    cp /usr/local/x-ui/x-ui.sh /usr/local/x-ui/x-ui.sh.backup
    echo "已备份原版脚本"
fi

# 下载中文脚本
wget -q -O /usr/local/x-ui/x-ui.sh https://raw.githubusercontent.com/sinian-liu/3x-ui/main/x-ui.sh

if [ $? -ne 0 ]; then
    echo -e "${YELLOW}下载中文脚本失败，使用原版英文脚本${NC}"
    if [ -f "/usr/local/x-ui/x-ui.sh.backup" ]; then
        cp /usr/local/x-ui/x-ui.sh.backup /usr/local/x-ui/x-ui.sh
    fi
else
    echo -e "${GREEN}中文脚本下载成功${NC}"
    chmod +x /usr/local/x-ui/x-ui.sh
    
    # 替换系统命令
    ln -sf /usr/local/x-ui/x-ui.sh /usr/bin/x-ui
    chmod +x /usr/bin/x-ui
fi

# 3. 设置默认中文
echo -e "${BLUE}步骤 3/3: 设置中文环境...${NC}"

if [ -f "/usr/local/x-ui/config.json" ]; then
    # 备份原配置
    cp /usr/local/x-ui/config.json /usr/local/x-ui/config.json.backup
    
    # 设置中文语言
    if grep -q '"language":' /usr/local/x-ui/config.json; then
        sed -i 's/"language": "en"/"language": "zh-CN"/g' /usr/local/x-ui/config.json
        echo "已修改语言设置为中文"
    else
        # 如果没有 language 字段，添加它
        sed -i 's/"panelSettings": {/"panelSettings": {\n    "language": "zh-CN",/g' /usr/local/x-ui/config.json
        echo "已添加语言设置为中文"
    fi
    
    # 重启服务
    systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null
    echo "服务已重启"
else
    echo -e "${YELLOW}配置文件不存在，跳过语言设置${NC}"
fi

# 显示完成信息
echo ""
echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}       3x-UI 中文版安装完成！${NC}"
echo -e "${GREEN}================================================${NC}"
echo ""
echo -e "${YELLOW}📢 重要信息：${NC}"
echo ""
echo -e "面板访问地址：${GREEN}http://你的服务器IP:54321${NC}"
echo -e "默认用户名：${GREEN}admin${NC}"
echo -e "默认密码：${GREEN}admin${NC}"
echo ""
echo -e "${YELLOW}🚀 使用方法：${NC}"
echo ""
echo -e "输入 ${GREEN}x-ui${NC} 打开中文管理菜单"
echo ""
echo -e "${YELLOW}🔧 常用命令：${NC}"
echo ""
echo -e "${GREEN}x-ui${NC}              # 打开管理菜单"
echo -e "${GREEN}x-ui start${NC}        # 启动面板"
echo -e "${GREEN}x-ui stop${NC}         # 停止面板"
echo -e "${GREEN}x-ui restart${NC}      # 重启面板"
echo -e "${GREEN}x-ui status${NC}       # 查看状态"
echo -e "${GREEN}x-ui log${NC}          # 查看日志"
echo -e "${GREEN}x-ui update${NC}       # 更新面板"
echo ""
echo -e "${YELLOW}⚠️  安全提示：${NC}"
echo -e "1. 请立即登录面板修改默认密码"
echo -e "2. 建议设置 SSL 证书（使用菜单选项 18）"
echo -e "3. 定期备份配置文件"
echo ""
echo -e "${GREEN}================================================${NC}"
