#!/bin/sh

case $1 in
    amd64)
        ARCH="64"
        FNAME="amd64"
        ;;
    armv8 | arm64 | aarch64)
        ARCH="arm64-v8a"
        FNAME="arm64"
        ;;
    *)
        ARCH="64"
        FNAME="amd64"
        ;;
esac

mkdir -p build/bin
cd build/bin

wget "https://github.com/XTLS/Xray-core/releases/download/v1.8.4/Xray-linux-${ARCH}.zip"
unzip "Xray-linux-${ARCH}.zip"
rm -f "Xray-linux-${ARCH}.zip" geoip.dat geosite.dat geoip_IR.dat geosite_IR.dat 
mv xray "xray-linux-${FNAME}"

wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
wget -O geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
wget -O geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
