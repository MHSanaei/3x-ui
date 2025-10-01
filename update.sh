#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

# Don't edit this config
b_source="${BASH_SOURCE[0]}"
while [ -h "$b_source" ]; do
	b_dir="$(cd -P "$(dirname "$b_source")" >/dev/null 2>&1 && pwd || pwd -P)"
	b_source="$(readlink "$b_source")"
	[[ $b_source != /* ]] && b_source="$b_dir/$b_source"
done
cur_dir="$(cd -P "$(dirname "$b_source")" >/dev/null 2>&1 && pwd || pwd -P)"
script_name=$(basename "$0")

# Check command exist function
_command_exists() {
	type "$1" &>/dev/null
}

# Fail, log and exit script function
_fail() {
	local msg=${1}
	echo -e "${red}${msg}${plain}"
	exit 2
}

# check root
[[ $EUID -ne 0 ]] && _fail "FATAL ERROR: Please run this script with root privilege."

if _command_exists wget; then
	wget_bin=$(which wget)
else
	_fail "ERROR: Command 'wget' not found."
fi

if _command_exists curl; then
	curl_bin=$(which curl)
else
	_fail "ERROR: Command 'curl' not found."
fi

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
	source /etc/os-release
	release=$ID
elif [[ -f /usr/lib/os-release ]]; then
	source /usr/lib/os-release
	release=$ID
else
	_fail "Failed to check the system OS, please contact the author!"
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
	*) echo -e "${red}Unsupported CPU architecture!${plain}" && rm -f "${cur_dir}/${script_name}" >/dev/null 2>&1 && exit 2;;
	esac
}

echo "Arch: $(arch)"

install_base() {
	echo -e "${green}Updating and install dependency packages...${plain}"
	case "${release}" in
	ubuntu | debian | armbian)
		apt-get update >/dev/null 2>&1 && apt-get install -y -q wget curl tar tzdata >/dev/null 2>&1
		;;
	centos | rhel | almalinux | rocky | ol)
		yum -y update >/dev/null 2>&1 && yum install -y -q wget curl tar tzdata >/dev/null 2>&1
		;;
	fedora | amzn | virtuozzo)
		dnf -y update >/dev/null 2>&1 && dnf install -y -q wget curl tar tzdata >/dev/null 2>&1
		;;
	arch | manjaro | parch)
		pacman -Syu >/dev/null 2>&1 && pacman -Syu --noconfirm wget curl tar tzdata >/dev/null 2>&1
		;;
	opensuse-tumbleweed | opensuse-leap)
		zypper refresh >/dev/null 2>&1 && zypper -q install -y wget curl tar timezone >/dev/null 2>&1
		;;
	alpine)
		apk update >/dev/null 2>&1 && apk add wget curl tar tzdata >/dev/null 2>&1
		;;
	*)
		apt-get update >/dev/null 2>&1 && apt install -y -q wget curl tar tzdata >/dev/null 2>&1
		;;
	esac
}

config_after_update() {
	echo -e "${yellow}x-ui settings:${plain}"
	/usr/local/x-ui/x-ui setting -show true
	/usr/local/x-ui/x-ui migrate
}

update_x-ui() {
	cd /usr/local/

	if [ -f "/usr/local/x-ui/x-ui" ]; then
		current_xui_version=$(/usr/local/x-ui/x-ui -v)
		echo -e "${green}Current x-ui version: ${current_xui_version}${plain}"
	else
		_fail "ERROR: Current x-ui version: unknown"
	fi

	echo -e "${green}Downloading new x-ui version...${plain}"

	tag_version=$(${curl_bin} -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
	if [[ ! -n "$tag_version" ]]; then
		echo -e "${yellow}Trying to fetch version with IPv4...${plain}"
		tag_version=$(${curl_bin} -4 -Ls "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
		if [[ ! -n "$tag_version" ]]; then
			_fail "ERROR: Failed to fetch x-ui version, it may be due to GitHub API restrictions, please try it later"
		fi
	fi
	echo -e "Got x-ui latest version: ${tag_version}, beginning the installation..."
	${wget_bin} -N -O /usr/local/x-ui-linux-$(arch).tar.gz https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz 2>/dev/null
	if [[ $? -ne 0 ]]; then
		echo -e "${yellow}Trying to fetch version with IPv4...${plain}"
		${wget_bin} --inet4-only -N -O /usr/local/x-ui-linux-$(arch).tar.gz https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz 2>/dev/null
		if [[ $? -ne 0 ]]; then
			_fail "ERROR: Failed to download x-ui, please be sure that your server can access GitHub"
		fi
	fi

	if [[ -e /usr/local/x-ui/ ]]; then
		echo -e "${green}Stopping x-ui...${plain}"
		if [[ $release == "alpine" ]]; then
			if [ -f "/etc/init.d/x-ui" ]; then
				rc-service x-ui stop >/dev/null 2>&1
				rc-update del x-ui >/dev/null 2>&1
				echo -e "${green}Removing old service unit version...${plain}"
				rm -f /etc/init.d/x-ui >/dev/null 2>&1
			else
				rm x-ui-linux-$(arch).tar.gz -f >/dev/null 2>&1
				_fail "ERROR: x-ui service unit not installed."
			fi
		else
			if [ -f "/etc/systemd/system/x-ui.service" ]; then
				systemctl stop x-ui >/dev/null 2>&1
				systemctl disable x-ui >/dev/null 2>&1
				echo -e "${green}Removing old systemd unit version...${plain}"
				rm /etc/systemd/system/x-ui.service -f >/dev/null 2>&1
				systemctl daemon-reload >/dev/null 2>&1
			else
				rm x-ui-linux-$(arch).tar.gz -f >/dev/null 2>&1
				_fail "ERROR: x-ui systemd unit not installed."
			fi
		fi
		echo -e "${green}Removing old x-ui version...${plain}"
		rm /usr/bin/x-ui -f >/dev/null 2>&1
		rm /usr/local/x-ui/x-ui.service -f >/dev/null 2>&1
		rm /usr/local/x-ui/x-ui -f >/dev/null 2>&1
		rm /usr/local/x-ui/x-ui.sh -f >/dev/null 2>&1
		echo -e "${green}Removing old xray version...${plain}"
		rm /usr/local/x-ui/bin/xray-linux-amd64 -f >/dev/null 2>&1
		echo -e "${green}Removing old README and LICENSE file...${plain}"
		rm /usr/local/x-ui/bin/README.md -f >/dev/null 2>&1
		rm /usr/local/x-ui/bin/LICENSE -f >/dev/null 2>&1
	else
		rm x-ui-linux-$(arch).tar.gz -f >/dev/null 2>&1
		_fail "ERROR: x-ui not installed."
	fi

	echo -e "${green}Installing new x-ui version...${plain}"
	tar zxvf x-ui-linux-$(arch).tar.gz >/dev/null 2>&1
	rm x-ui-linux-$(arch).tar.gz -f >/dev/null 2>&1
	cd x-ui >/dev/null 2>&1
	chmod +x x-ui >/dev/null 2>&1

	# Check the system's architecture and rename the file accordingly
	if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
		mv bin/xray-linux-$(arch) bin/xray-linux-arm >/dev/null 2>&1
		chmod +x bin/xray-linux-arm >/dev/null 2>&1
	fi

	chmod +x x-ui bin/xray-linux-$(arch) >/dev/null 2>&1

	echo -e "${green}Downloading and installing x-ui.sh script...${plain}"
	${wget_bin} -O /usr/bin/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.sh >/dev/null 2>&1
	if [[ $? -ne 0 ]]; then
		echo -e "${yellow}Trying to fetch x-ui with IPv4...${plain}"
		${wget_bin} --inet4-only -O /usr/bin/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.sh >/dev/null 2>&1
		if [[ $? -ne 0 ]]; then
			_fail "ERROR: Failed to download x-ui.sh script, please be sure that your server can access GitHub"
		fi
	fi

	chmod +x /usr/local/x-ui/x-ui.sh >/dev/null 2>&1
	chmod +x /usr/bin/x-ui >/dev/null 2>&1

	echo -e "${green}Changing owner...${plain}"
	chown -R root:root /usr/local/x-ui >/dev/null 2>&1

	if [ -f "/usr/local/x-ui/bin/config.json" ]; then
		echo -e "${green}Changing on config file permissions...${plain}"
		chmod 640 /usr/local/x-ui/bin/config.json >/dev/null 2>&1
	fi

	if [[ $release == "alpine" ]]; then
		echo -e "${green}Downloading and installing startup unit x-ui.rc...${plain}"
		${wget_bin} -O /etc/init.d/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.rc >/dev/null 2>&1
		if [[ $? -ne 0 ]]; then
			${wget_bin} --inet4-only -O /etc/init.d/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.rc >/dev/null 2>&1
			if [[ $? -ne 0 ]]; then
				_fail "ERROR: Failed to download startup unit x-ui.rc, please be sure that your server can access GitHub"
			fi
		fi
		chmod +x /etc/init.d/x-ui >/dev/null 2>&1
		chown root:root /etc/init.d/x-ui >/dev/null 2>&1
		rc-update add x-ui >/dev/null 2>&1
		rc-service x-ui start >/dev/null 2>&1
	else
		echo -e "${green}Installing systemd unit...${plain}"
		cp -f x-ui.service /etc/systemd/system/ >/dev/null 2>&1
		chown root:root /etc/systemd/system/x-ui.service >/dev/null 2>&1
		systemctl daemon-reload >/dev/null 2>&1
		systemctl enable x-ui >/dev/null 2>&1
		systemctl start x-ui >/dev/null 2>&1
	fi

	config_after_update

	echo -e "${green}x-ui ${tag_version}${plain} updating finished, it is running now..."
	echo -e ""
	echo -e "┌───────────────────────────────────────────────────────┐
│  ${blue}x-ui control menu usages (subcommands):${plain}              │
│                                                       │	
│  ${blue}x-ui${plain}              - Admin Management Script          │
│  ${blue}x-ui start${plain}        - Start                            │
│  ${blue}x-ui stop${plain}         - Stop                             │
│  ${blue}x-ui restart${plain}      - Restart                          │
│  ${blue}x-ui status${plain}       - Current Status                   │
│  ${blue}x-ui settings${plain}     - Current Settings                 │
│  ${blue}x-ui enable${plain}       - Enable Autostart on OS Startup   │
│  ${blue}x-ui disable${plain}      - Disable Autostart on OS Startup  │
│  ${blue}x-ui log${plain}          - Check logs                       │
│  ${blue}x-ui banlog${plain}       - Check Fail2ban ban logs          │
│  ${blue}x-ui update${plain}       - Update                           │
│  ${blue}x-ui legacy${plain}       - legacy version                   │
│  ${blue}x-ui install${plain}      - Install                          │
│  ${blue}x-ui uninstall${plain}    - Uninstall                        │
└───────────────────────────────────────────────────────┘"
}

echo -e "${green}Running...${plain}"
install_base
update_x-ui $1
