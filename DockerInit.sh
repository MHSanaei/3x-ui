#!/bin/sh
set -e
# $1 = TARGETARCH, $2 = TARGETVARIANT (buildx). buildx sends "386" (not
# "i386") and "arm" + variant "v6"/"v7", never "armv6"/"armv7"; the old
# mapping sent linux/386 to the default amd64 branch and gave arm/v6 an ARMv7
# Xray. The file names must match what the panel looks for at runtime:
#   xray-linux-<GOARCH, with arm -> arm32>   (internal/xray/process.go)
#   mtg-linux-<GOARCH>                       (internal/mtproto/process.go)
VARIANT="${2:-}"
case $1 in
    amd64)
        ARCH="64"
        FNAME="amd64"
        MTGNAME="amd64"
        MTGARCH="amd64"
        ;;
    386 | i386)
        ARCH="32"
        FNAME="386"
        MTGNAME="386"
        MTGARCH="386"
        ;;
    armv8 | arm64 | aarch64)
        ARCH="arm64-v8a"
        FNAME="arm64"
        MTGNAME="arm64"
        MTGARCH="arm64"
        ;;
    arm | arm32 | armv7 | armv6)
        FNAME="arm32"
        MTGNAME="arm"
        if [ "$VARIANT" = "v6" ] || [ "$1" = "armv6" ]; then
            ARCH="arm32-v6"
            MTGARCH="armv6"
        else
            ARCH="arm32-v7a"
            MTGARCH="armv7"
        fi
        ;;
    *)
        echo "DockerInit: unsupported TARGETARCH '$1'" >&2
        exit 1
        ;;
esac
MTG_MULTI_VER=$(curl -sfL "https://api.github.com/repos/mhsanaei/mtg-multi/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
if [ -z "$MTG_MULTI_VER" ]; then
    echo "DockerInit: could not resolve the latest mtg-multi release tag" >&2
    exit 1
fi
mkdir -p build/bin
cd build/bin
curl -sfLRO "https://github.com/XTLS/Xray-core/releases/download/v26.10.10/Xray-linux-${ARCH}.zip"
unzip "Xray-linux-${ARCH}.zip"
rm -f "Xray-linux-${ARCH}.zip" geoip.dat geosite.dat
mv xray "xray-linux-${FNAME}"
# mtg-multi (MTProto sidecar) ships prebuilt release binaries for every target
# we package, so download and unpack the matching one instead of compiling.
MTG_PKG="mtg-multi-${MTG_MULTI_VER#v}-linux-${MTGARCH}"
curl -sfLRO "https://github.com/mhsanaei/mtg-multi/releases/download/${MTG_MULTI_VER}/${MTG_PKG}.tar.gz"
tar -xzf "${MTG_PKG}.tar.gz"
mv "${MTG_PKG}/mtg-multi" "mtg-linux-${MTGNAME}"
rm -rf "${MTG_PKG}" "${MTG_PKG}.tar.gz"
chmod +x "mtg-linux-${MTGNAME}"
curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
curl -sfLRo geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
curl -sfLRo geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
curl -sfLRo geoip_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat
curl -sfLRo geosite_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat
cd ../../
