#!/bin/bash
# lib/geo.sh - Geo files management

# Include guard
[[ -n "${__X_UI_GEO_INCLUDED:-}" ]] && return 0
__X_UI_GEO_INCLUDED=1

update_all_geofiles() {
    target_folder="${1}"
    update_geofiles "main" "${target_folder}"
    update_geofiles "IR" "${target_folder}"
    update_geofiles "RU" "${target_folder}"
}

update_geofiles() {
    target_folder="${2}"
    case "${1}" in
      "main") dat_files=(geoip geosite); dat_source="Loyalsoldier/v2ray-rules-dat";;
        "IR") dat_files=(geoip_IR geosite_IR); dat_source="chocolate4u/Iran-v2ray-rules" ;;
        "RU") dat_files=(geoip_RU geosite_RU); dat_source="runetfreedom/russia-v2ray-rules-dat";;
    esac
    for dat in "${dat_files[@]}"; do
        # Remove suffix for remote filename (e.g., geoip_IR -> geoip)
        remote_file="${dat%%_*}"
        curl -fLRo ${target_folder}/${dat}.dat -z ${target_folder}/${dat}.dat \
            https://github.com/${dat_source}/releases/latest/download/${remote_file}.dat
    done
}

# CLI entrypoint when script is executed directly
if [[ "${0##*/}" = "geo.sh" ]]; then
    cmd="${1:-}"
    shift || true

    case "$cmd" in
        update_all_geofiles)
            update_all_geofiles "$@"
            ;;
        update_geofiles)
            update_geofiles "$@"
            ;;
        "" | help | -h | --help)
            echo "Usage:"
            echo "  $0 update_all_geofiles <target_folder>           - Update geo files for all regions"
            echo "  $0 update_geofiles <region> <target_folder>      - Update geo files for specific region"
            echo ""
            echo "Available regions: main, IR, RU"
            exit 0
            ;;
        *)
            echo "Unknown command: $cmd" >&2
            echo "Try: $0 help" >&2
            exit 1
            ;;
    esac
fi