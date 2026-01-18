#!/bin/bash
# x-ui.sh - 3X-UI Panel Management Script (Entrypoint)
# This is the main entrypoint that sources modular library files

# Resolve the actual script location (handles symlinks)
#SCRIPT_PATH="$(readlink -f "$0" 2>/dev/null || realpath "$0" 2>/dev/null || echo "$0")"
#SCRIPT_DIR="$(dirname "$SCRIPT_PATH")"
#LIB_DIR="${SCRIPT_DIR}/lib"
#
## Fallback for installed location
#[[ ! -d "$LIB_DIR" ]] && LIB_DIR="/usr/local/x-ui/lib"

LIB_DIR="${LIB_DIR:=/usr/local/x-ui/lib}"
# Export LIB_DIR for use by library files
export LIB_DIR

# Source all library files
source "${LIB_DIR}/common.sh"
source "${LIB_DIR}/service.sh"
source "${LIB_DIR}/ssl.sh"
source "${LIB_DIR}/settings.sh"
source "${LIB_DIR}/firewall.sh"
source "${LIB_DIR}/iplimit.sh"
source "${LIB_DIR}/bbr.sh"
source "${LIB_DIR}/geo.sh"
source "${LIB_DIR}/install.sh"
source "${LIB_DIR}/extras.sh"

# Print OS info
echo "The OS release is: $release"

#=============================================================================
# Menu Functions (kept in entrypoint to avoid circular dependencies)
#=============================================================================

confirm_restart() {
    confirm "Restart the panel, Attention: Restarting the panel will also restart xray" "y"
    if [[ $? == 0 ]]; then
        restart
    else
        show_menu
    fi
}

before_show_menu() {
    echo && echo -n -e "${yellow}Press enter to return to the main menu: ${plain}" && read -r temp
    show_menu
}

show_usage() {
    echo -e "┌────────────────────────────────────────────────────────────────┐
│  ${blue}x-ui control menu usages (subcommands):${plain}                       │
│                                                                │
│  ${blue}x-ui${plain}                       - Admin Management Script          │
│  ${blue}x-ui start${plain}                 - Start                            │
│  ${blue}x-ui stop${plain}                  - Stop                             │
│  ${blue}x-ui restart${plain}               - Restart                          │
│  ${blue}x-ui status${plain}                - Current Status                   │
│  ${blue}x-ui settings${plain}              - Current Settings                 │
│  ${blue}x-ui enable${plain}                - Enable Autostart on OS Startup   │
│  ${blue}x-ui disable${plain}               - Disable Autostart on OS Startup  │
│  ${blue}x-ui log${plain}                   - Check logs                       │
│  ${blue}x-ui banlog${plain}                - Check Fail2ban ban logs          │
│  ${blue}x-ui update${plain}                - Update                           │
│  ${blue}x-ui update-all-geofiles${plain}   - Update all geo files             │
│  ${blue}x-ui legacy${plain}                - Legacy version                   │
│  ${blue}x-ui install${plain}               - Install                          │
│  ${blue}x-ui uninstall${plain}             - Uninstall                        │
└────────────────────────────────────────────────────────────────┘"
}

update_geo() {
    echo -e "${green}\t1.${plain} Loyalsoldier (geoip.dat, geosite.dat)"
    echo -e "${green}\t2.${plain} chocolate4u (geoip_IR.dat, geosite_IR.dat)"
    echo -e "${green}\t3.${plain} runetfreedom (geoip_RU.dat, geosite_RU.dat)"
    echo -e "${green}\t4.${plain} All"
    echo -e "${green}\t0.${plain} Back to Main Menu"
    read -rp "Choose an option: " choice

    case "$choice" in
    0)
        show_menu
        ;;
    1)
        update_geofiles "main" "${xui_folder}"/bin
        echo -e "${green}Loyalsoldier datasets have been updated successfully!${plain}"
        restart
        ;;
    2)
        update_geofiles "IR" "${xui_folder}"/bin
        echo -e "${green}chocolate4u datasets have been updated successfully!${plain}"
        restart
        ;;
    3)
        update_geofiles "RU" "${xui_folder}"/bin
        echo -e "${green}runetfreedom datasets have been updated successfully!${plain}"
        restart
        ;;
    4)
        update_all_geofiles "${xui_folder}"/bin
        echo -e "${green}All geo files have been updated successfully!${plain}"
        restart
        ;;
    *)
        echo -e "${red}Invalid option. Please select a valid number.${plain}\n"
        update_geo
        ;;
    esac

    before_show_menu
}

show_menu() {
    echo -e "
╔────────────────────────────────────────────────╗
│   ${green}3X-UI Panel Management Script${plain}                │
│   ${green}0.${plain} Exit Script                               │
│────────────────────────────────────────────────│
│   ${green}1.${plain} Install                                   │
│   ${green}2.${plain} Update                                    │
│   ${green}3.${plain} Update Menu                               │
│   ${green}4.${plain} Legacy Version                            │
│   ${green}5.${plain} Uninstall                                 │
│────────────────────────────────────────────────│
│   ${green}6.${plain} Reset Username & Password                 │
│   ${green}7.${plain} Reset Web Base Path                       │
│   ${green}8.${plain} Reset Settings                            │
│   ${green}9.${plain} Change Port                               │
│  ${green}10.${plain} View Current Settings                     │
│────────────────────────────────────────────────│
│  ${green}11.${plain} Start                                     │
│  ${green}12.${plain} Stop                                      │
│  ${green}13.${plain} Restart                                   │
│  ${green}14.${plain} Check Status                              │
│  ${green}15.${plain} Logs Management                           │
│────────────────────────────────────────────────│
│  ${green}16.${plain} Enable Autostart                          │
│  ${green}17.${plain} Disable Autostart                         │
│────────────────────────────────────────────────│
│  ${green}18.${plain} SSL Certificate Management                │
│  ${green}19.${plain} Cloudflare SSL Certificate                │
│  ${green}20.${plain} IP Limit Management                       │
│  ${green}21.${plain} Firewall Management                       │
│  ${green}22.${plain} SSH Port Forwarding Management            │
│────────────────────────────────────────────────│
│  ${green}23.${plain} Enable BBR                                │
│  ${green}24.${plain} Update Geo Files                          │
│  ${green}25.${plain} Speedtest by Ookla                        │
╚────────────────────────────────────────────────╝
"
    show_status
    echo && read -rp "Please enter your selection [0-25]: " num

    case "${num}" in
    0)
        exit 0
        ;;
    1)
        check_uninstall && install
        ;;
    2)
        check_install && update
        ;;
    3)
        check_install && update_menu
        ;;
    4)
        check_install && legacy_version
        ;;
    5)
        check_install && uninstall
        ;;
    6)
        check_install && reset_user
        ;;
    7)
        check_install && reset_webbasepath
        ;;
    8)
        check_install && reset_config
        ;;
    9)
        check_install && set_port
        ;;
    10)
        check_install && check_config
        ;;
    11)
        check_install && start
        ;;
    12)
        check_install && stop
        ;;
    13)
        check_install && restart
        ;;
    14)
        check_install && status
        ;;
    15)
        check_install && show_log
        ;;
    16)
        check_install && enable
        ;;
    17)
        check_install && disable
        ;;
    18)
        ssl_cert_issue_main
        ;;
    19)
        ssl_cert_issue_CF
        ;;
    20)
        iplimit_main
        ;;
    21)
        firewall_menu
        ;;
    22)
        SSH_port_forwarding
        ;;
    23)
        bbr_menu
        ;;
    24)
        update_geo
        ;;
    25)
        run_speedtest
        ;;
    *)
        LOGE "Please enter the correct number [0-25]"
        ;;
    esac
}

#=============================================================================
# CLI Argument Handling
#=============================================================================

if [[ $# -gt 0 ]]; then
    case $1 in
    "start")
        check_install 0 && start 0
        ;;
    "stop")
        check_install 0 && stop 0
        ;;
    "restart")
        check_install 0 && restart 0
        ;;
    "status")
        check_install 0 && status 0
        ;;
    "settings")
        check_install 0 && check_config 0
        ;;
    "enable")
        check_install 0 && enable 0
        ;;
    "disable")
        check_install 0 && disable 0
        ;;
    "log")
        check_install 0 && show_log 0
        ;;
    "banlog")
        check_install 0 && show_banlog 0
        ;;
    "update")
        check_install 0 && update 0
        ;;
    "legacy")
        check_install 0 && legacy_version 0
        ;;
    "install")
        check_uninstall 0 && install 0
        ;;
    "uninstall")
        check_install 0 && uninstall 0
        ;;
#      TODO: check
    "update-all-geofiles")
        check_install 0 && update_all_geofiles "${xui_folder}"/bin 0 && restart 0
        ;;
    *) show_usage ;;
    esac
else
    show_menu
fi
