#!/bin/sh

case $1 in
    amd64)
        ARCH="64"
        FNAME="amd64"
        ;;
    arm64)
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

wget "https://github.com/mhsanaei/xray-core/releases/latest/download/Xray-linux-${ARCH}.zip"
unzip "Xray-linux-${ARCH}.zip"
rm -f "Xray-linux-${ARCH}.zip" geoip.dat geosite.dat iran.dat
mv xray "xray-linux-${FNAME}"

wget "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
wget "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
wget "https://github.com/bootmortis/iran-hosted-domains/releases/latest/download/iran.dat"
