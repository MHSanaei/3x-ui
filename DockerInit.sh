#!/bin/sh
# POSIX-compatible script to download Xray binary and GeoIP/GeoSite databases

set -eu

XRAY_VERSION="v25.6.8"
BASE_URL="https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}"
DAT_DIR="build/bin"
ARCH="${1:-amd64}"  # Default to amd64 if not provided

# Map architecture
case "$ARCH" in
    amd64)    ARCH_SUFFIX="64";        FNAME="amd64"     ;;
    i386)     ARCH_SUFFIX="32";        FNAME="i386"      ;;
    armv8|arm64|aarch64) ARCH_SUFFIX="arm64-v8a"; FNAME="arm64" ;;
    armv7|arm|arm32)     ARCH_SUFFIX="arm32-v7a"; FNAME="arm32" ;;
    armv6)    ARCH_SUFFIX="arm32-v6";  FNAME="armv6"     ;;
    *)        ARCH_SUFFIX="64";        FNAME="amd64"     ;;
esac

echo "Selected architecture: $ARCH → $ARCH_SUFFIX → $FNAME"

# Create directory
mkdir -p "$DAT_DIR"
cd "$DAT_DIR"

# Download and unpack Xray
XRAY_ZIP="Xray-linux-${ARCH_SUFFIX}.zip"
echo "Downloading Xray: $XRAY_ZIP"
wget -q "${BASE_URL}/${XRAY_ZIP}"
unzip -q "$XRAY_ZIP"
rm -f "$XRAY_ZIP" geoip.dat geosite.dat
mv xray "xray-linux-${FNAME}"
chmod +x "xray-linux-${FNAME}"
echo "Xray extracted and renamed"

# Download primary databases
echo "Downloading official geoip/geosite..."
wget -q https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
wget -q https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat

# Download region-specific datasets
download_variant() {
    REGION="$1"
    BASE_URL="$2"
    echo "Downloading $REGION GeoIP/GeoSite..."
    wget -q -O "geoip_${REGION}.dat"    "${BASE_URL}/geoip.dat"
    wget -q -O "geosite_${REGION}.dat"  "${BASE_URL}/geosite.dat"
}

download_variant "IR" "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download"
download_variant "RU" "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download"

echo "All files downloaded successfully."
cd ../../
