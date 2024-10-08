#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1

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

echo "arch: $(arch)"

os_version=""
os_version=$(grep "^VERSION_ID" /etc/os-release | cut -d '=' -f2 | tr -d '"' | tr -d '.')

if [[ "${release}" == "arch" ]]; then
    echo "Your OS is Arch Linux"
elif [[ "${release}" == "parch" ]]; then
    echo "Your OS is Parch Linux"
elif [[ "${release}" == "manjaro" ]]; then
    echo "Your OS is Manjaro"
elif [[ "${release}" == "armbian" ]]; then
    echo "Your OS is Armbian"
elif [[ "${release}" == "opensuse-tumbleweed" ]]; then
    echo "Your OS is OpenSUSE Tumbleweed"
elif [[ "${release}" == "openEuler" ]]; then
    if [[ ${os_version} -lt 2203 ]]; then
        echo -e "${red} Please use OpenEuler 22.03 or higher ${plain}\n" && exit 1
    fi
elif [[ "${release}" == "centos" ]]; then
    if [[ ${os_version} -lt 8 ]]; then
        echo -e "${red} Please use CentOS 8 or higher ${plain}\n" && exit 1
    fi
elif [[ "${release}" == "ubuntu" ]]; then
    if [[ ${os_version} -lt 2004 ]]; then
        echo -e "${red} Please use Ubuntu 20 or higher version!${plain}\n" && exit 1
    fi
elif [[ "${release}" == "fedora" ]]; then
    if [[ ${os_version} -lt 36 ]]; then
        echo -e "${red} Please use Fedora 36 or higher version!${plain}\n" && exit 1
    fi
elif [[ "${release}" == "amzn" ]]; then
    if [[ ${os_version} != "2023" ]]; then
        echo -e "${red} Please use Amazon Linux 2023!${plain}\n" && exit 1
    fi
elif [[ "${release}" == "debian" ]]; then
    if [[ ${os_version} -lt 11 ]]; then
        echo -e "${red} Please use Debian 11 or higher ${plain}\n" && exit 1
    fi
elif [[ "${release}" == "almalinux" ]]; then
    if [[ ${os_version} -lt 80 ]]; then
        echo -e "${red} Please use AlmaLinux 8.0 or higher ${plain}\n" && exit 1
    fi
elif [[ "${release}" == "rocky" ]]; then
    if [[ ${os_version} -lt 8 ]]; then
        echo -e "${red} Please use Rocky Linux 8 or higher ${plain}\n" && exit 1
    fi
elif [[ "${release}" == "ol" ]]; then
    if [[ ${os_version} -lt 8 ]]; then
        echo -e "${red} Please use Oracle Linux 8 or higher ${plain}\n" && exit 1
    fi
else
    echo -e "${red}Your operating system is not supported by this script.${plain}\n"
    echo "Please ensure you are using one of the following supported operating systems:"
    echo "- Ubuntu 20.04+"
    echo "- Debian 11+"
    echo "- CentOS 8+"
    echo "- OpenEuler 22.03+"
    echo "- Fedora 36+"
    echo "- Arch Linux"
    echo "- Parch Linux"
    echo "- Manjaro"
    echo "- Armbian"
    echo "- AlmaLinux 8.0+"
    echo "- Rocky Linux 8+"
    echo "- Oracle Linux 8+"
    echo "- OpenSUSE Tumbleweed"
    echo "- Amazon Linux 2023"
    exit 1
fi

install_base() {
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update && apt-get install -y -q wget curl tar tzdata
        ;;
    centos | almalinux | rocky | ol)
        yum -y update && yum install -y -q wget curl tar tzdata
        ;;
    fedora | amzn)
        dnf -y update && dnf install -y -q wget curl tar tzdata
        ;;
    arch | manjaro | parch)
        pacman -Syu && pacman -Syu --noconfirm wget curl tar tzdata
        ;;
    opensuse-tumbleweed)
        zypper refresh && zypper -q install -y wget curl tar timezone
        ;;
    *)
        apt-get update && apt install -y -q wget curl tar tzdata
        ;;
    esac
}

gen_random_string() {
    local length="$1"
    local random_string=$(LC_ALL=C tr -dc 'a-zA-Z0-9' </dev/urandom | fold -w "$length" | head -n 1)
    echo "$random_string"
}

config_after_install() {
    echo -e "${yellow}Install/update finished! For security, it's recommended to modify panel settings ${plain}"
    read -p "Would you like to customize the panel settings? (If not, random settings will be applied) [y/n]: " config_confirm

    local config_webBasePath=$(gen_random_string 10)

    if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then

        read -p "Please set up your username: " config_account
        echo -e "${yellow}Your username will be: ${config_account}${plain}"

        read -p "Please set up your password: " config_password
        echo -e "${yellow}Your password will be: ${config_password}${plain}"

        read -p "Please set up the panel port: " config_port
        echo -e "${yellow}Your panel port is: ${config_port}${plain}"

        echo -e "${yellow}Your web base path will be generated randomly: ${config_webBasePath}${plain}"

        echo -e "${yellow}Initializing, please wait...${plain}"

        /usr/local/x-ui/x-ui setting -username "${config_account}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"
        echo -e "${yellow}Settings applied successfully!${plain}"

        echo -e "###############################################"
        echo -e "${green}Username: ${config_account}${plain}"
        echo -e "${green}Password: ${config_password}${plain}"
        echo -e "${green}Port: ${config_port}${plain}"
        echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
        echo -e "###############################################"

    else

        echo -e "${red}Cancel...${plain}"

        if [[ ! -f "/etc/x-ui/x-ui.db" ]]; then

            local usernameTemp=$(gen_random_string 10)
            local passwordTemp=$(gen_random_string 10)
            local portTemp=$(shuf -i 1024-62000 -n 1)

            /usr/local/x-ui/x-ui setting -username "${usernameTemp}" -password "${passwordTemp}" -port "${portTemp}" -webBasePath "${config_webBasePath}"
            echo -e "This is a fresh installation, generating random login info for security concerns:"
            echo -e "###############################################"
            echo -e "${green}Username: ${usernameTemp}${plain}"
            echo -e "${green}Password: ${passwordTemp}${plain}"
            echo -e "${green}Port: ${portTemp}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "###############################################"
            echo -e "${yellow}If you forgot your login info, you can type 'x-ui settings' to check after installation${plain}"
        else
            echo -e "${yellow}This is your upgrade, keeping old settings. If you forgot your login info, you can type 'x-ui settings' to check${plain}"

            local existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')

            if [[ ${#existing_webBasePath} -lt 4 ]]; then
                echo -e "${yellow}WebBasePath is empty, generating a random one...${plain}"

                /usr/local/x-ui/x-ui setting -webBasePath "${config_webBasePath}"
                echo -e "${green}New webBasePath: ${config_webBasePath}${plain}"
            fi
        fi
    fi

    /usr/local/x-ui/x-ui migrate
}

install_x-ui() {
    cd /usr/local/

    if [ $# == 0 ]; then
        tag_version=$(curl -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${red}Failed to fetch x-ui version, it may be due to GitHub API restrictions, please try it later${plain}"
            exit 1
        fi
        echo -e "Got x-ui latest version: ${tag_version}, beginning the installation..."
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(arch).tar.gz https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Downloading x-ui failed, please be sure that your server can access GitHub ${plain}"
            exit 1
        fi
    else
        tag_version=$1
        tag_version_numeric=${tag_version#v}
        min_version="2.3.5"

        if [[ "$(printf '%s\n' "$min_version" "$tag_version_numeric" | sort -V | head -n1)" != "$min_version" ]]; then
            echo -e "${red}Please use a newer version (at least v2.3.5). Exiting installation.${plain}"
            exit 1
        fi

        url="https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz"
        echo -e "Beginning to install x-ui $1"
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Download x-ui $1 failed, please check if the version exists ${plain}"
            exit 1
        fi
    fi

    if [[ -e /usr/local/x-ui/ ]]; then
        systemctl stop x-ui
        rm /usr/local/x-ui/ -rf
    fi

    tar zxvf x-ui-linux-$(arch).tar.gz
    rm x-ui-linux-$(arch).tar.gz -f
    cd x-ui
    chmod +x x-ui

    # Check the system's architecture and rename the file accordingly
    if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm
        chmod +x bin/xray-linux-arm
    fi

    chmod +x x-ui bin/xray-linux-$(arch)
    cp -f x-ui.service /etc/systemd/system/
    wget --no-check-certificate -O /usr/bin/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.sh
    chmod +x /usr/local/x-ui/x-ui.sh
    chmod +x /usr/bin/x-ui
    config_after_install

    systemctl daemon-reload
    systemctl enable x-ui
    systemctl start x-ui
    echo -e "${green}x-ui ${tag_version}${plain} installation finished, it is running now..."
    echo -e ""
    echo -e "x-ui control menu usages: "
    echo -e "----------------------------------------------"
    echo -e "SUBCOMMANDS:"
    echo -e "x-ui              - Admin Management Script"
    echo -e "x-ui start        - Start"
    echo -e "x-ui stop         - Stop"
    echo -e "x-ui restart      - Restart"
    echo -e "x-ui status       - Current Status"
    echo -e "x-ui settings     - Current Settings"
    echo -e "x-ui enable       - Enable Autostart on OS Startup"
    echo -e "x-ui disable      - Disable Autostart on OS Startup"
    echo -e "x-ui log          - Check logs"
    echo -e "x-ui banlog       - Check Fail2ban ban logs"
    echo -e "x-ui update       - Update"
    echo -e "x-ui custom       - custom version"
    echo -e "x-ui install      - Install"
    echo -e "x-ui uninstall    - Uninstall"
    echo -e "----------------------------------------------"
}

echo -e "${green}Running...${plain}"
install_base
install_x-ui $1
