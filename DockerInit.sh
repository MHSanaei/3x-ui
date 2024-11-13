#!/bin/sh
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
mkdir -p build/bin
cd build/bin
wget "https://github.com/XTLS/Xray-core/releases/download/v24.11.11/Xray-linux-${ARCH}.zip"
unzip "Xray-linux-${ARCH}.zip"
rm -f "Xray-linux-${ARCH}.zip" geoip.dat geosite.dat
mv xray "xray-linux-${FNAME}"
wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
wget https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
wget -O geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
wget -O geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
wget -O geoip_VN.dat https://github.com/vuong2023/vn-v2ray-rules/releases/latest/download/geoip.dat
wget -O geosite_VN.dat https://github.com/vuong2023/vn-v2ray-rules/releases/latest/download/geosite.dat
cd ../../

# Antizapret
case $2 in
    0)
        ANTIZAPRET="0"
        ;;
    1)
        ANTIZAPRET="1"
        ;;
    *)
        ANTIZAPRET="0"
        ;;
esac
if [[ $ANTIZAPRET == "1" ]]; then
    wget https://github.com/warexify/antizapret-xray/archive/refs/heads/main.zip
    unzip main.zip
    mv antizapret-xray-main antizapret-xray
    mkdir -p antizapret-xray/z-i
    cd antizapret-xray/z-i
    wget -O dump.csv https://github.com/zapret-info/z-i/raw/master/dump.csv
    cd ../
    go build
    chmod +x antizapret-xray
    ./antizapret-xray
    mv publish/geosite.dat ../build/bin/geosite_antizapret.dat
    cd ../
    echo "Antizapret: ext:geosite_antizapret.dat:zapretinfo"
else
    echo "Antizapret: disabled"
fi