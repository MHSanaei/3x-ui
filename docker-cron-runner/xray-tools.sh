#!/bin/sh

safe_download_and_update() {
    url="$1"
    dest="$2"

    # Create a temporary file
    tmp=$(mktemp "${dest}.XXXXXX") || return 1

    # Download file into a temporary location
    if wget -q -O "$tmp" "$url"; then
        # Check that the downloaded file is not empty
        if [ -s "$tmp" ]; then
            # Atomically replace the destination file
            mv "$tmp" "$dest"
            echo "[OK] Downloaded: $dest"
        else
            echo "[ERR] Downloaded file is empty: $url"
            rm -f "$tmp"
            return 1
        fi
    else
        echo "[ERR] Failed to download: $url"
        rm -f "$tmp"
        return 1
    fi
}

update_all_geofiles() {
        update_main_geofiles
        update_ir_geofiles
        update_ru_geofiles
}

update_main_geofiles() {
    safe_download_and_update \
        "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat" \
        "geoip.dat"

    safe_download_and_update \
        "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat" \
        "geosite.dat"
}

update_ir_geofiles() {
    safe_download_and_update \
        "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat" \
        "geoip_IR.dat"

    safe_download_and_update \
        "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat" \
        "geosite_IR.dat"
}

update_ru_geofiles() {
    safe_download_and_update \
        "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat" \
        "geoip_RU.dat"

    safe_download_and_update \
        "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat" \
        "geosite_RU.dat"
}

update_geodata_in_docker() {
    XRAYDIR="$1"
    OLD_DIR=$(pwd)
    trap 'cd "$OLD_DIR"' EXIT

    echo "[$(date)] Running update_geodata"

    if [ ! -d "$XRAYDIR" ]; then
      mkdir -p "$XRAYDIR"
    fi
    cd "$XRAYDIR"

    update_all_geofiles
    echo "[$(date)] All geo files have been updated successfully!"
}


install_xray_core() {
    TARGETARCH="$1"
    XRAYDIR="$2"
    XRAY_VERSION="$3"

    OLD_DIR=$(pwd)
    trap 'cd "$OLD_DIR"' EXIT

    echo "[$(date)] Running install_xray_core"

    case $1 in
      amd64)
          ARCH="64"
          FNAME="amd64"
          ;;
      i386)
          ARCH="32"
          FNAME="i386"
          ;;
      armv8 | arm64 | aarch64)
          ARCH="arm64-v8a"
          FNAME="arm64"
          ;;
      armv7 | arm | arm32)
          ARCH="arm32-v7a"
          FNAME="arm32"
          ;;
      armv6)
          ARCH="arm32-v6"
          FNAME="armv6"
          ;;
      *)
          ARCH="64"
          FNAME="amd64"
          ;;
    esac

    if [ ! -d "$XRAYDIR" ]; then
      mkdir -p "$XRAYDIR"
    fi
    cd "$XRAYDIR"

    wget -q "https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/Xray-linux-${ARCH}.zip"

    # Validate the downloaded zip file
    if [ ! -f "Xray-linux-${ARCH}.zip" ] || [ ! -s "Xray-linux-${ARCH}.zip" ]; then
        echo "[ERR] Failed to download Xray-core zip or file is empty"
        cd "$OLD_DIR"
        return 1
    fi

    unzip -q "Xray-linux-${ARCH}.zip" -d ./xray-unzip

    # Validate the extracted xray binary
    if [ ! -f "./xray-unzip/xray" ] || [ ! -s "./xray-unzip/xray" ]; then
        echo "[ERR] Failed to extract xray binary"
        rm -rf ./xray-unzip
        rm -f "Xray-linux-${ARCH}.zip"
        cd "$OLD_DIR"
        return 1
    fi

    cp ./xray-unzip/xray ./"xray-linux-${FNAME}"
    rm -r xray-unzip
    rm "Xray-linux-${ARCH}.zip"

if [ "${0##*/}" = "xray-tools.sh" ]; then
  cmd="$1"
  shift || true

  case "$cmd" in
    install_xray_core)
      # args: TARGETARCH XRAYDIR XRAY_VERSION
      install_xray_core "$@"
      ;;
    update_geodata_in_docker)
      # args: XRAYDIR
      update_geodata_in_docker "$@"
      ;;
    update_all_geofiles)
      update_all_geofiles
      ;;
    ""|help|-h|--help)
      echo "Usage:"
      echo "  $0 install_xray_core TARGETARCH XRAYDIR XRAY_VERSION"
      echo "  $0 update_geodata_in_docker XRAYDIR"
      exit 1
      ;;
    *)
      echo "Unknown command: $cmd" >&2
      echo "Try: $0 help" >&2
      exit 1
      ;;
  esac
fi