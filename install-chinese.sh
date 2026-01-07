#!/bin/bash

# 3x-ui 中文版安装脚本

echo "================================================"
echo "       3x-UI 中文版安装"
echo "================================================"
echo ""

# 1. 用原版安装
echo "正在安装 3x-ui..."
bash <(curl -Ls https://raw.githubusercontent.com/MHSanaei/3x-ui/main/install.sh)

# 2. 替换为中文管理脚本
echo "正在安装中文管理菜单..."
wget -q -O /usr/local/x-ui/x-ui.sh https://raw.githubusercontent.com/你的用户名/3x-ui/main/x-ui.sh
chmod +x /usr/local/x-ui/x-ui.sh

# 3. 链接到系统命令
ln -sf /usr/local/x-ui/x-ui.sh /usr/bin/x-ui

# 4. 设置默认语言
if [ -f "/usr/local/x-ui/config.json" ]; then
    sed -i 's/"language": "en"/"language": "zh-CN"/g' /usr/local/x-ui/config.json
    systemctl restart x-ui
fi

echo ""
echo "================================================"
echo "       安装完成！"
echo "================================================"
echo ""
echo "✅ 现在输入以下命令使用中文菜单："
echo ""
echo "     x-ui"
echo ""
echo "✅ 网页面板会自动显示中文"
echo "✅ 所有功能与原版完全一样"
echo ""
echo "管理命令："
echo "  x-ui          # 打开中文管理菜单"
echo "  x-ui start    # 启动面板"
echo "  x-ui stop     # 停止面板"
echo "  x-ui restart  # 重启面板"
echo "  x-ui status   # 查看状态"
echo ""
