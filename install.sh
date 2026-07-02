#!/bin/bash

# Остановка скрипта при любой критической ошибке
set -e

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# Значения по умолчанию
MODE="git"          
INSTALL_BOT=0       

# Определение ОС
DETECTED_OS="linux"
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    release="unknown_linux"
fi
echo "Detected OS: Linux ($release)"

# Определение архитектуры
arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${green}Unsupported CPU architecture! ${plain}" && exit 1 ;;
    esac
}
current_arch=$(arch)
echo "Arch: $current_arch"

gen_random_string() {
    local length="$1"
    openssl rand -base64 $((length * 2)) | tr -dc 'a-zA-Z0-9' | head -c "$length"
}

write_install_result() {
    local u="$1" p="$2" port="$3" wbp="$4" scheme="$5" host="$6" token="$7" dbtype="$8"
    local result_file="/etc/x-ui/install-result.env"
    local url_host="${host:-SERVER_IP_UNKNOWN}"
    install -d -m 755 /etc/x-ui 2> /dev/null
    local prev_umask=$(umask); umask 077
    if ! {
        printf 'XUI_USERNAME=%q\n' "$u"
        printf 'XUI_PASSWORD=%q\n' "$p"
        printf 'XUI_PANEL_PORT=%q\n' "$port"
        printf 'XUI_WEB_BASE_PATH=%q\n' "$wbp"
        printf 'XUI_ACCESS_URL=%q\n' "${scheme}://${url_host}:${port}/${wbp}"
        printf 'XUI_API_TOKEN=%q\n' "$token"
        printf 'XUI_DB_TYPE=%q\n' "$dbtype"
    } > "$result_file"; then
        umask "$prev_umask"
        echo -e "${yellow}Warning: failed to write ${result_file}.${plain}" >&2
        return 1
    fi
    umask "$prev_umask"; chmod 600 "$result_file" 2> /dev/null; chown root:root "$result_file" 2> /dev/null || true
}

install_base() {
    echo -e "${green}Installing base dependencies...${plain}"
    apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl unzip git wget
}

install_build_deps() {
    echo -e "${green}Installing build tools (Node.js, Go)...${plain}"
    apt-get install -y -q nodejs golang-go
}

install_bot_deps() {
    echo -e "${green}Installing Python dependencies for Telegram Bot...${plain}"
    apt-get install -y -q python3 python3-pip python3-venv zip unzip
}

config_after_install() {
    echo -e "${green}Инициализация базы данных...${plain}"
    local existing_hasDefaultCredential=$(${xui_folder}/x-ui setting -show true 2>/dev/null | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}' || echo "true")
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true 2>/dev/null | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##' || echo "")
    local server_ip=$(curl -s -w "\n%{http_code}" --max-time 3 "https://v4.api.ipinfo.io/ip" 2> /dev/null | head -n-1 | tr -d '[:space:]"')
    if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then server_ip="127.0.0.1"; fi

    SSL_SCHEME="http"
    SSL_HOST="${server_ip}"

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath=$(gen_random_string 18)
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)
            local config_port=$(shuf -i 1024-62000 -n 1)
            
            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}" > /dev/null 2>&1
            local config_apiToken=$(${xui_folder}/x-ui setting -getApiToken true 2>/dev/null | grep -Eo 'apiToken: .+' | awk '{print $2}')

            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}Username:    ${config_username}${plain}"
            echo -e "${green}Password:    ${config_password}${plain}"
            echo -e "${green}Port:        ${config_port}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "${green}Access URL:  ${SSL_SCHEME}://${SSL_HOST}:${config_port}/${config_webBasePath}${plain}"
            write_install_result "${config_username}" "${config_password}" "${config_port}" "${config_webBasePath}" "${SSL_SCHEME}" "${SSL_HOST}" "${config_apiToken}" "sqlite"
        else
            local config_webBasePath=$(gen_random_string 18)
            ${xui_folder}/x-ui setting -webBasePath "${config_webBasePath}" > /dev/null 2>&1
        fi
    fi
    ${xui_folder}/x-ui migrate > /dev/null 2>&1
}

install_xray() {
    local xray_dir="${xui_folder}/bin"
    if [[ -x "${xray_dir}/xray" ]] && "${xray_dir}/xray" -version &>/dev/null; then
        echo -e "${green}Xray-core уже установлен и исправно работает. Пропускаем скачивание.${plain}"
        return 0
    fi

    mkdir -p "$xray_dir"
    echo -e "${green}Installing Xray-core...${plain}"
    
    local xray_file="Xray-linux-64.zip"
    if [[ "$current_arch" == "arm64" ]]; then xray_file="Xray-linux-arm64-v8a.zip"; fi

    local url="https://github.com/XTLS/Xray-core/releases/latest/download/${xray_file}"
    curl -fLR --retry 5 -o "${xray_dir}/xray.zip" "$url"
    cd "$xray_dir" && unzip -o xray.zip > /dev/null && rm xray.zip
    if [[ -f "Xray" ]]; then mv Xray xray; fi
    ln -sf xray xray-linux-amd64
    chmod +x xray xray-linux-amd64
}

install_xray_bot() {
    local bot_dir="${xui_folder}/xray-bot"
    echo -e "${green}🤖 Installing Xray Bot...${plain}"
    install_bot_deps
    
    rm -f /usr/bin/xray-bot
    mkdir -p "$bot_dir"
    
    if [[ "$MODE" == "build" ]]; then
        if [[ -d "${SRC_DIR}/xray-bot" ]]; then
            cp -r "${SRC_DIR}/xray-bot/"* "$bot_dir/"
        elif [[ -d "${cur_dir}/xray-bot" ]]; then
            cp -r "${cur_dir}/xray-bot/"* "$bot_dir/"
        fi

        if [[ -f "${SRC_DIR}/xray-bot.sh" ]]; then
            cp "${SRC_DIR}/xray-bot.sh" /usr/bin/xray-bot
        elif [[ -f "${cur_dir}/xray-bot.sh" ]]; then
            cp "${cur_dir}/xray-bot.sh" /usr/bin/xray-bot
        fi
    else
        git clone https://github.com/KimaruBs/3x-ui.git "${bot_dir}_tmp"
        cp -r "${bot_dir}_tmp/xray-bot/"* "$bot_dir/"
        wget -N --no-check-certificate -O /usr/bin/xray-bot "https://raw.githubusercontent.com/KimaruBs/3x-ui/main/xray-bot.sh"
        rm -rf "${bot_dir}_tmp"
    fi
    
    if [[ ! -f /usr/bin/xray-bot && -f "${bot_dir}/xray-bot.sh" ]]; then
        cp "${bot_dir}/xray-bot.sh" /usr/bin/xray-bot
    fi
    
    chmod +x /usr/bin/xray-bot

    if [[ -f "${bot_dir}/requirements.txt" ]]; then
        python3 -m venv "${bot_dir}/venv"
        "${bot_dir}/venv/bin/pip" install --upgrade pip -q
        "${bot_dir}/venv/bin/pip" install -r "${bot_dir}/requirements.txt" -q
        echo -e "${green}✅ Бот развернут в ${bot_dir}${plain}"
    fi

    echo -e "${green}⚙️ Обновление системной службы для xray-bot...${plain}"
    systemctl stop xray-bot > /dev/null 2>&1 || true
    rm -f /etc/systemd/system/xray-bot.service

    cat > /etc/systemd/system/xray-bot.service <<EOF
[Unit]
Description=3x-ui Xray Telegram Bot
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${bot_dir}/src
ExecStart=${bot_dir}/venv/bin/python3 app.py
Restart=on-failure
RestartSec=3s

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable xray-bot
    systemctl restart xray-bot
    echo -e "${green}✅ Служба xray-bot успешно перезапущена! Управление: xray-bot${plain}"
}

start_installation() {
    install_base

    if [ -d "${cur_dir}/3x-ui" ]; then SRC_DIR="${cur_dir}/3x-ui"
    elif [ -f "${cur_dir}/main.go" ] && [ -d "${cur_dir}/frontend" ]; then SRC_DIR="${cur_dir}"
    else SRC_DIR="${cur_dir}"
    fi

    if [[ -e ${xui_folder}/ ]]; then
        systemctl stop x-ui > /dev/null 2>&1 || true
        find "${xui_folder}" -mindepth 1 -maxdepth 1 ! -name 'bin' ! -name 'xray-bot' -exec rm -rf {} +
    fi

    mkdir -p ${xui_folder}
    rm -f "${xui_folder}/x-ui" /usr/bin/x-ui

    if [[ "$MODE" == "build" ]]; then
        echo -e "${green}🛠 Локальная сборка панели из исходников...${plain}"
        install_build_deps
        
        cd "$SRC_DIR"
        chmod +x build.sh
        ./build.sh "$current_arch"
        
        cp "build/x-ui-linux-${current_arch}" "${xui_folder}/x-ui"
        chmod +x "${xui_folder}/x-ui"
        
        cp x-ui.sh /usr/bin/x-ui
        chmod +x /usr/bin/x-ui
    else
        echo -e "${green}🌐 Скачивание готового релиза из репозитория Git...${plain}"
        local ui_file="x-ui-linux-${current_arch}"
        
        wget -N --no-check-certificate -O "${xui_folder}/x-ui" "https://github.com/KimaruBs/3x-ui/releases/latest/download/${ui_file}"
        chmod +x "${xui_folder}/x-ui"
        
        wget -N --no-check-certificate -O /usr/bin/x-ui "https://raw.githubusercontent.com/KimaruBs/3x-ui/main/x-ui.sh"
        chmod +x /usr/bin/x-ui
    fi

    install_xray

    if [[ "$INSTALL_BOT" == "1" ]]; then
        install_xray_bot
    fi

    cat > /etc/systemd/system/x-ui.service <<EOF
[Unit]
Description=3x-ui customized panel
After=network.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/usr/local/x-ui
ExecStart=/usr/local/x-ui/x-ui
Restart=on-failure
RestartSec=3s

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable x-ui

    config_after_install
    systemctl restart x-ui
    sleep 1

    echo -e "${green}🎉 Установка полностью завершена! Панель успешно запущена.${plain}"
    echo -e "${yellow}Запускаем интерактивное меню управления...${plain}"
    sleep 1
    
    exec /usr/bin/x-ui
}

show_menu() {
    clear
    echo -e "${blue}===================================${plain}"
    echo -e "${green}        3X-UI Smart Installer      ${plain}"
    echo -e "${blue}===================================${plain}"
    echo -e "1. Install X-UI (${green}Download from Git${plain})"
    echo -e "2. Install X-UI + Xray Bot (${green}Download from Git${plain})"
    echo -e "3. Build & Install X-UI (${yellow}Local Build${plain})"
    echo -e "4. Build & Install X-UI + Xray Bot (${yellow}Local Build${plain})"
    echo -e "0. Exit"
    echo -e "${blue}===================================${plain}"
    read -rp "Please choose an option: " menu_choice

    case "$menu_choice" in
        1) MODE="git";   INSTALL_BOT=0; start_installation ;;
        2) MODE="git";   INSTALL_BOT=1; start_installation ;;
        3) MODE="build"; INSTALL_BOT=0; start_installation ;;
        4) MODE="build"; INSTALL_BOT=1; start_installation ;;
        0) exit 0 ;;
        *) echo -e "${red}Invalid option!${plain}"; sleep 1; show_menu ;;
    esac
}

show_menu
