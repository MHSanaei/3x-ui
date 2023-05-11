#!/bin/sh
if [ $1 == "amd64" ]; then
    ARCH="64";
    FNAME="amd64";
elif [ $1 == "arm64" ]; then
    ARCH="arm64-v8a"
    FNAME="arm64";
else
    ARCH="64";
    FNAME="amd64";
fi
mkdir -p build/bin
cd build/bin
wget "https://github.com/mhsanaei/xray-core/releases/latest/download/Xray-linux-${ARCH}.zip"
unzip "Xray-linux-${ARCH}.zip"
rm -f "Xray-linux-${ARCH}.zip" geoip.dat geosite.dat iran.dat
mv xray "xray-linux-${FNAME}"
wget "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
wget "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
wget "https://github.com/bootmortis/iran-hosted-domains/releases/latest/download/iran.dat"

cd ../../