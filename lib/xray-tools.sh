#!/bin/bash

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
        exit 1
    fi

    unzip -q "Xray-linux-${ARCH}.zip" -d ./xray-unzip

    # Validate the extracted xray binary
    if [ -f "./xray-unzip/xray" ]; then
      cp ./xray-unzip/xray ./"xray-linux-${FNAME}"
      rm -r xray-unzip
      rm "Xray-linux-${ARCH}.zip"
    else
      echo "[ERR] Failed to extract xray binary"
      exit 1
    fi
}

if [ "${0##*/}" = "xray-tools.sh" ]; then
  cmd="$1"
  shift || true

  case "$cmd" in
    install_xray_core)
      # args: TARGETARCH XRAYDIR XRAY_VERSION
      install_xray_core "$@"
      ;;
#    update_geodata_in_docker)
#      # args: XRAYDIR
#      update_geodata_in_docker "$@"
#      ;;
    ""|help|-h|--help)
      echo "Usage:"
      echo "  $0 install_xray_core TARGETARCH XRAYDIR XRAY_VERSION"
#      echo "  $0 update_geodata_in_docker XRAYDIR"
      exit 0
      ;;
    *)
      echo "Unknown command: $cmd" >&2
      echo "Try: $0 help" >&2
      exit 1
      ;;
  esac
fi