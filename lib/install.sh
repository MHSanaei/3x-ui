#!/bin/bash
# lib/install.sh - Install, update, and uninstall functions

# Include guard
[[ -n "${__X_UI_INSTALL_INCLUDED:-}" ]] && return 0
__X_UI_INSTALL_INCLUDED=1

# Source dependencies
source "${LIB_DIR}/common.sh"
source "${LIB_DIR}/service.sh"

install() {
    bash <(curl -Ls https://raw.githubusercontent.com/MHSanaei/3x-ui/main/install.sh)
    if [[ $? == 0 ]]; then
        if [[ $# == 0 ]]; then
            start
        else
            start 0
        fi
    fi
}

update() {
    confirm "This function will update all x-ui components to the latest version, and the data will not be lost. Do you want to continue?" "y"
    if [[ $? != 0 ]]; then
        LOGE "Cancelled"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 0
    fi
    bash <(curl -Ls https://raw.githubusercontent.com/MHSanaei/3x-ui/main/update.sh)
    if [[ $? == 0 ]]; then
        LOGI "Update is complete, Panel has automatically restarted "
        before_show_menu
    fi
}

update_menu() {
    echo -e "${yellow}Updating Menu${plain}"
    confirm "This function will update the menu to the latest changes." "y"
    if [[ $? != 0 ]]; then
        LOGE "Cancelled"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 0
    fi

    curl -fLRo /usr/bin/x-ui https://raw.githubusercontent.com/MHSanaei/3x-ui/main/x-ui.sh
    chmod +x ${xui_folder}/x-ui.sh
    chmod +x /usr/bin/x-ui

    if [[ $? == 0 ]]; then
        echo -e "${green}Update successful. The panel has automatically restarted.${plain}"
        exit 0
    else
        echo -e "${red}Failed to update the menu.${plain}"
        return 1
    fi
}

legacy_version() {
    echo -n "Enter the panel version (like 2.4.0):"
    read -r tag_version

    if [ -z "$tag_version" ]; then
        echo "Panel version cannot be empty. Exiting."
        exit 1
    fi
    # Use the entered panel version in the download link
    install_command="bash <(curl -Ls "https://raw.githubusercontent.com/mhsanaei/3x-ui/v$tag_version/install.sh") v$tag_version"

    echo "Downloading and installing panel version $tag_version..."
    eval $install_command
}

# Function to handle the deletion of the script file
delete_script() {
    rm "$0" # Remove the script file itself
    exit 1
}

uninstall() {
    confirm "Are you sure you want to uninstall the panel? xray will also uninstalled!" "n"
    if [[ $? != 0 ]]; then
        if [[ $# == 0 ]]; then
            show_menu
        fi
        return 0
    fi

    if [[ $release == "alpine" ]]; then
        rc-service x-ui stop
        rc-update del x-ui
        rm /etc/init.d/x-ui -f
    else
        systemctl stop x-ui
        systemctl disable x-ui
        rm ${xui_service}/x-ui.service -f
        systemctl daemon-reload
        systemctl reset-failed
    fi

    rm /etc/x-ui/ -rf
    rm ${xui_folder}/ -rf

    echo ""
    echo -e "Uninstalled Successfully.\n"
    echo "If you need to install this panel again, you can use below command:"
    echo -e "${green}bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)${plain}"
    echo ""
    # Trap the SIGTERM signal
    trap delete_script SIGTERM
    delete_script
}

update_shell() {
    curl -fLRo /usr/bin/x-ui -z /usr/bin/x-ui https://github.com/MHSanaei/3x-ui/raw/main/x-ui.sh
    if [[ $? != 0 ]]; then
        echo ""
        LOGE "Failed to download script, Please check whether the machine can connect Github"
        before_show_menu
    else
        chmod +x /usr/bin/x-ui
        LOGI "Upgrade script succeeded, Please rerun the script"
        before_show_menu
    fi
}
