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

# Значения по умолчанию для автоматического режима
MODE="git"          # Режим для X-UI: git или build
INSTALL_BOT=0       # Флаг установки бота (0 - нет, 1 - да)

# Сохраняем оригинальное количество аргументов до shift
ORIG_ARGS_COUNT=$#

while [[ $# -gt 0 ]]; do
    case "$1" in
        --build)
            MODE="build"
            shift
            ;;
        --with-bot)
            INSTALL_BOT=1
            shift
            ;;
        *)
            echo -e "${red}Unknown option: $1${plain}"
            exit 1
            ;;
    esac
done

# Определение ОС
DETECTED_OS="linux"
case "$(uname -s)" in
    CYGWIN*|MINGW*|MSYS*|Windows*)
        DETECTED_OS="windows"
        release="windows"
        echo "Detected OS: Windows"
        ;;
    *)
        [[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1
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
        ;;
esac

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
echo "Arch: $(arch)"

if [[ "${XUI_NONINTERACTIVE:-0}" == "1" ]] || [[ ! -t 0 ]]; then
    NONINTERACTIVE=1
else
    NONINTERACTIVE=0
fi
export NONINTERACTIVE

is_ipv4() { [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] && return 0 || return 1; }
is_ipv6() { [[ "$1" =~ : ]] && return 0 || return 1; }
is_ip() { is_ipv4 "$1" || is_ipv6 "$1"; }
is_domain() { [[ "$1" =~ ^([A-Za-z0-9](-*[A-Za-z0-9])*\.)+(xn--[a-z0-9]{2,}|[A-Za-z]{2,})$ ]] && return 0 || return 1; }

gen_random_string() {
    local length="$1"
    openssl rand -base64 $((length * 2)) | tr -dc 'a-zA-Z0-9' | head -c "$length"
}

write_install_result() {
    local u="$1" p="$2" port="$3" wbp="$4" scheme="$5" host="$6" token="$7" dbtype="$8"
    local result_file="/etc/x-ui/install-result.env"
    local url_host="${host:-SERVER_IP_UNKNOWN}"
    install -d -m 755 /etc/x-ui 2> /dev/null
    local prev_umask
    prev_umask=$(umask)
    umask 077
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
    umask "$prev_umask"
    chmod 600 "$result_file" 2> /dev/null
    chown root:root "$result_file" 2> /dev/null || true
    echo -e "${green}Install result written to ${result_file} (mode 600).${plain}"
}

install_base() {
    echo -e "${green}Installing base dependencies...${plain}"
    case "${release}" in
        ubuntu | debian | armbian) apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl unzip git wget ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol) dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl unzip git wget ;;
        centos) if [[ "${VERSION_ID}" =~ ^7 ]]; then yum -y update && yum install -y cronie curl tar tzdata socat ca-certificates openssl unzip git wget; else dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl unzip git wget; fi ;;
        arch | manjaro | parch) pacman -Syu --noconfirm cronie curl tar tzdata socat ca-certificates openssl unzip git wget ;;
        alpine) apk update && apk add dcron curl tar tzdata socat ca-certificates openssl unzip bash git wget ;;
        *) apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl unzip git wget ;;
    esac
}

install_build_deps() {
    echo -e "${green}Installing build tools (Node.js, Go)...${plain}"
    case "${release}" in
        ubuntu | debian | armbian) apt-get install -y -q nodejs golang-go ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol | centos) dnf install -y -q nodejs golang ;;
        arch | manjaro | parch) pacman -S --noconfirm nodejs go ;;
        alpine) apk add nodejs go ;;
        *) apt-get install -y -q nodejs golang-go ;;
    esac
}

install_bot_deps() {
    echo -e "${green}Installing Python dependencies for Telegram Bot...${plain}"
    case "${release}" in
        ubuntu | debian | armbian) apt-get install -y -q python3 python3-pip python3-venv zip unzip ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol | centos) dnf install -y -q python3 python3-pip zip unzip ;;
        arch | manjaro | parch) pacman -S --noconfirm python python-pip zip unzip ;;
        alpine) apk add python3 py3-pip zip unzip ;;
        *) apt-get install -y -q python3 python3-pip python3-venv zip unzip ;;
    esac
}

config_after_install() {
    echo -e "${green}Запуск первоначальной настройки и миграции базы данных...${plain}"
    local existing_hasDefaultCredential=$(${xui_folder}/x-ui setting -show true | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    local server_ip=$(curl -s -w "\n%{http_code}" --max-time 3 "https://v4.api.ipinfo.io/ip" 2> /dev/null | head -n-1 | tr -d '[:space:]"')
    if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then server_ip="${XUI_SERVER_IP:-127.0.0.1}"; fi

    SSL_SCHEME="http"
    SSL_HOST="${server_ip}"

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath="${XUI_WEB_BASE_PATH:-$(gen_random_string 18)}"
            local config_username="${XUI_USERNAME:-$(gen_random_string 10)}"
            local config_password="${XUI_PASSWORD:-$(gen_random_string 10)}"
            local config_port="${XUI_PANEL_PORT:-$(shuf -i 1024-62000 -n 1)}"
            
            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"
            local config_apiToken=$(${xui_folder}/x-ui setting -getApiToken true | grep -Eo 'apiToken: .+' | awk '{print $2}')

            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}Username:    ${config_username}${plain}"
            echo -e "${green}Password:    ${config_password}${plain}"
            echo -e "${green}Port:        ${config_port}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "${green}Access URL:  ${SSL_SCHEME}://${SSL_HOST}:${config_port}/${config_webBasePath}${plain}"
            write_install_result "${config_username}" "${config_password}" "${config_port}" "${config_webBasePath}" "${SSL_SCHEME}" "${SSL_HOST}" "${config_apiToken}" "sqlite"
        else
            local config_webBasePath=$(gen_random_string 18)
            ${xui_folder}/x-ui setting -webBasePath "${config_webBasePath}"
        fi
    fi
    ${xui_folder}/x-ui migrate
}

install_xray() {
    local xray_dir="${xui_folder}/bin"
    # ПОПРАВЛЕНО: Исправная проверка существования и работоспособности ядра Xray
    if [[ -x "${xray_dir}/xray" ]] && "${xray_dir}/xray" -version &>/dev/null; then
        echo -e "${green}Xray-core уже установлен и исправно работает. Пропускаем скачивание.${plain}"
        return 0
    fi

    local arch_type=$(arch)
    mkdir -p "$xray_dir"
    echo -e "${green}Installing Xray-core...${plain}"
    
    local xray_file="Xray-linux-${arch_type}.zip"
    if [[ "$arch_type" == "arm64" ]]; then xray_file="Xray-linux-arm64-v8a.zip"
    elif [[ "$arch_type" == "amd64" ]]; then xray_file="Xray-linux-64.zip"
    fi

    local url="https://github.com/XTLS/Xray-core/releases/latest/download/${xray_file}"
    curl -fLR --retry 5 -o "${xray_dir}/xray.zip" "$url"
    cd "$xray_dir" && unzip -o xray.zip > /dev/null && rm xray.zip
    if [[ -f "Xray" ]]; then mv Xray xray; fi
    ln -sf xray xray-linux-amd64
    chmod +x xray xray-linux-amd64
    echo -e "${green}Xray-core успешно добавлен!${plain}"
}

install_xray_bot() {
    local bot_mode=$1
    local bot_dir="/usr/local/x-ui-bot"
    echo -e "${green}🤖 Installing Xray Bot...${plain}"
    install_bot_deps
    mkdir -p "$bot_dir"
    
    if [[ "$bot_mode" == "build" && -d "${cur_dir}/xray-bot" ]]; then
        cp -r "${cur_dir}/xray-bot/"* "$bot_dir/"
    else
        git clone https://github.com/NidukaA递/x-ui-telegram-bot.git "$bot_dir" || true
    fi
    
    if [[ -f "${bot_dir}/requirements.txt" ]]; then
        python3 -m venv "${bot_dir}/venv"
        "${bot_dir}/venv/bin/pip" install --upgrade pip -q
        "${bot_dir}/venv/bin/pip" install -r "${bot_dir}/requirements.txt" -q
        echo -e "${green}✅ Бот развернут в ${bot_dir}${plain}"
    fi
}

install_x-ui() {
    install_base

    # ПОПРАВЛЕНО: Удаляем всё КРОМЕ папки 'bin', чтобы не стереть уже скачанный рабочий Xray
    if [[ -e ${xui_folder}/ ]]; then
        systemctl stop x-ui > /dev/null 2>&1 || true
        find "${xui_folder}" -mindepth 1 -maxdepth 1 ! -name 'bin' -exec rm -rf {} +
    fi

    mkdir -p ${xui_folder}
    cd ${xui_folder}

    if [[ "$MODE" == "build" ]]; then
        echo -e "${green}🛠 Локальная сборка панели...${plain}"
        install_build_deps
        
        if [ -d "${cur_dir}/3x-ui" ]; then SRC_DIR="${cur_dir}/3x-ui"
        elif [ -f "${cur_dir}/main.go" ] && [ -d "${cur_dir}/frontend" ]; then SRC_DIR="${cur_dir}"
        else
            echo -e "${red}❌ Ошибка: Исходники 3x-ui не найдены!${plain}" && exit 1
        fi
        
        cd "$SRC_DIR"
        chmod +x build.sh
        local current_arch=$(arch)
        ./build.sh "$current_arch"
        
        cp "build/x-ui-linux-${current_arch}" "${xui_folder}/x-ui"
        cp x-ui.sh /usr/bin/x-ui
        chmod +x ${xui_folder}/x-ui /usr/bin/x-ui
    else
        echo -e "${green}🌐 Скачивание готового релиза из Git...${plain}"
        local arch_type=$(arch)
        local ui_file="x-ui-linux-${arch_type}"
        wget -N --no-check-certificate -O "${xui_folder}/x-ui" "https://github.com/KimaruBs/3x-ui/releases/latest/download/${ui_file}"
        chmod +x "${xui_folder}/x-ui"
        ln -sf "${xui_folder}/x-ui" /usr/bin/x-ui
    fi

    # 1. Доставляем ядро Xray (скачается ТОЛЬКО если папка bin пустая или повреждена)
    install_xray

    # 2. Ставим бота, если запросили
    if [[ "$INSTALL_BOT" == "1" ]]; then
        install_xray_bot "$MODE"
    fi

    # 3. Настраиваем базу данных и конфиги (теперь бинарник x-ui вызывается безопасно)
    config_after_install

    # 4. САМЫЙ ПОСЛЕДНИЙ ШАГ: Создание службы и запуск x-ui в системе
    echo -e "${green}Создаем службу автозапуска в systemd и запускаем x-ui...${plain}"
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
    systemctl start x-ui
    
    echo -e "${green}🎉 Установка полностью завершена! Панель наконец запущена службой.${plain}"
}

show_menu() {
    clear
    echo -e "${blue}===================================${plain}"
    echo -e "${green}    X-UI & Xray Bot Installer     ${plain}"
    echo -e "${blue}===================================${plain}"
    echo -e "1. Install X-UI (${green}From Git${plain})"
    echo -e "2. Install X-UI + Xray Bot (${green}From Git${plain})"
    echo -e "3. Build & Install X-UI (${yellow}Local Build${plain})"
    echo -e "4. Build & Install X-UI + Xray Bot (${yellow}Local Build${plain})"
    echo -e "0. Exit"
    echo -e "${blue}===================================${plain}"
    read -rp "Please choose an option: " menu_choice

    case "$menu_choice" in
        1) MODE="git"; INSTALL_BOT=0; install_x-ui ;;
        2) MODE="git"; INSTALL_BOT=1; install_x-ui ;;
        3) MODE="build"; INSTALL_BOT=0; install_x-ui ;;
        4) MODE="build"; INSTALL_BOT=1; install_x-ui ;;
        0) exit 0 ;;
        *) echo -e "${red}Invalid option!${plain}"; sleep 1; show_menu ;;
    esac
}

if [[ "$ORIG_ARGS_COUNT" -eq 0 && "$NONINTERACTIVE" == "0" ]]; then
    show_menu
else
    install_x-ui
fi
