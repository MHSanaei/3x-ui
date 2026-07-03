#!/bin/bash
set -euo pipefail

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# Temporary file cleanup
temp_files=()
cleanup() {
    for f in "${temp_files[@]:+${temp_files[@]}}"; do
        rm -f "$f" 2>/dev/null || true
    done
}
trap cleanup EXIT

add_temp_file() {
    temp_files+=("$1")
}

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege." && exit 1

# Ensure curl is installed
ensure_curl() {
    if ! command -v curl &>/dev/null; then
        echo -e "${yellow}curl not found, installing...${plain}"
        case "${release}" in
            ubuntu|debian|armbian) apt-get update >/dev/null 2>&1 && apt-get install -y -q curl >/dev/null 2>&1 ;;
            fedora|amzn|virtuozzo|rhel|almalinux|rocky|ol) dnf install -y -q curl >/dev/null 2>&1 ;;
            centos)
                if [[ "${VERSION_ID}" =~ ^7 ]]; then
                    yum install -y -q curl >/dev/null 2>&1
                else
                    dnf install -y -q curl >/dev/null 2>&1
                fi
                ;;
            arch|manjaro|parch) pacman -Sy --noconfirm curl >/dev/null 2>&1 ;;
            opensuse-tumbleweed|opensuse-leap) zypper -q install -y curl >/dev/null 2>&1 ;;
            alpine) apk add --no-cache curl >/dev/null 2>&1 ;;
            *) apt-get update >/dev/null 2>&1 && apt-get install -y -q curl >/dev/null 2>&1 ;;
        esac
        if ! command -v curl &>/dev/null; then
            echo -e "${red}Failed to install curl. Please install curl manually and try again.${plain}"
            exit 1
        fi
        echo -e "${green}curl installed successfully${plain}"
    fi
}

# Rest of the original update.sh continues...
