#!/bin/sh

update_all_geofiles() {
        update_main_geofiles
#        update_ir_geofiles
#        update_ru_geofiles
}

update_main_geofiles() {
        wget -O geoip.dat       https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
        wget -O geosite.dat     https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
}

update_ir_geofiles() {
        wget -O geoip_IR.dat    https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
        wget -O geosite_IR.dat  https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
}

update_ru_geofiles() {
        wget -O geoip_RU.dat    https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat
        wget -O geosite_RU.dat  https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat
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
    unzip "Xray-linux-${ARCH}.zip" -d ./xray-unzip
    cp ./xray-unzip/xray ./"xray-linux-${FNAME}"
    rm -r xray-unzip
    rm "Xray-linux-${ARCH}.zip"
 }

# --- dispatcher: вызываем функции по имени ТОЛЬКО если скрипт запущен как файл ---
# Предполагаем, что файл называется xray-updates.sh
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