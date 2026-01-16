#!/bin/bash
# lib/geo.sh - Geo files management

# Include guard
[[ -n "${__X_UI_GEO_INCLUDED:-}" ]] && return 0
__X_UI_GEO_INCLUDED=1

# Source dependencies
source "${LIB_DIR}/common.sh"
source "${LIB_DIR}/service.sh"

update_all_geofiles() {
    update_geofiles "main"
    update_geofiles "IR"
    update_geofiles "RU"
}

update_geofiles() {
    case "${1}" in
      "main") dat_files=(geoip geosite); dat_source="Loyalsoldier/v2ray-rules-dat";;
        "IR") dat_files=(geoip_IR geosite_IR); dat_source="chocolate4u/Iran-v2ray-rules" ;;
        "RU") dat_files=(geoip_RU geosite_RU); dat_source="runetfreedom/russia-v2ray-rules-dat";;
    esac
    for dat in "${dat_files[@]}"; do
        # Remove suffix for remote filename (e.g., geoip_IR -> geoip)
        remote_file="${dat%%_*}"
        curl -fLRo ${xui_folder}/bin/${dat}.dat -z ${xui_folder}/bin/${dat}.dat \
            https://github.com/${dat_source}/releases/latest/download/${remote_file}.dat
    done
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
        update_geofiles "main"
        echo -e "${green}Loyalsoldier datasets have been updated successfully!${plain}"
        restart
        ;;
    2)
        update_geofiles "IR"
        echo -e "${green}chocolate4u datasets have been updated successfully!${plain}"
        restart
        ;;
    3)
        update_geofiles "RU"
        echo -e "${green}runetfreedom datasets have been updated successfully!${plain}"
        restart
        ;;
    4)
        update_all_geofiles
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
