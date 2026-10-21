#!/bin/sh
set -e
case $1 in
    amd64)
        FNAME="amd64"
        XRAY_FNAME="amd64"
        XRAY_GOARCH="amd64"
        ;;
    i386 | 386)
        FNAME="386"
        XRAY_FNAME="386"
        XRAY_GOARCH="386"
        ;;
    armv8 | arm64 | aarch64)
        FNAME="arm64"
        XRAY_FNAME="arm64"
        XRAY_GOARCH="arm64"
        ;;
    arm)
        case ${2:-v7} in
            v6)
                FNAME="armv6"
                XRAY_FNAME="arm32"
                XRAY_GOARCH="arm"
                XRAY_GOARM="6"
                ;;
            v7 | "")
                FNAME="arm32"
                XRAY_FNAME="arm32"
                XRAY_GOARCH="arm"
                XRAY_GOARM="7"
                ;;
            *)
                echo "DockerInit: unsupported ARM variant: $2" >&2
                exit 1
                ;;
        esac
        ;;
    armv7 | arm32)
        FNAME="arm32"
        XRAY_FNAME="arm32"
        XRAY_GOARCH="arm"
        XRAY_GOARM="7"
        ;;
    armv6)
        FNAME="armv6"
        XRAY_FNAME="arm32"
        XRAY_GOARCH="arm"
        XRAY_GOARM="6"
        ;;
    *)
        FNAME="amd64"
        XRAY_FNAME="amd64"
        XRAY_GOARCH="amd64"
        ;;
esac
MTG_MULTI_VER=$(curl -sfL "https://api.github.com/repos/mhsanaei/mtg-multi/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
if [ -z "$MTG_MULTI_VER" ]; then
    echo "DockerInit: could not resolve the latest mtg-multi release tag" >&2
    exit 1
fi
mkdir -p build/bin
env CGO_ENABLED=0 GOOS=linux GOARCH="${XRAY_GOARCH}" GOARM="${XRAY_GOARM:-7}" go build -mod=readonly -ldflags="-s -w" -o "$(pwd)/build/bin/xray" github.com/xtls/xray-core/main
cd build/bin
rm -f geoip.dat geosite.dat
mv xray "xray-linux-${XRAY_FNAME}"
# mtg-multi (MTProto sidecar) ships prebuilt release binaries for every target
# we package, so download and unpack the matching one instead of compiling.
case $FNAME in
    386) MTGARCH="386" ;;
    arm32) MTGARCH="armv7" ;;
    *) MTGARCH="$FNAME" ;;
esac
MTG_PKG="mtg-multi-${MTG_MULTI_VER#v}-linux-${MTGARCH}"
curl -sfLRO "https://github.com/mhsanaei/mtg-multi/releases/download/${MTG_MULTI_VER}/${MTG_PKG}.tar.gz"
tar -xzf "${MTG_PKG}.tar.gz"
mv "${MTG_PKG}/mtg-multi" "mtg-linux-${FNAME}"
rm -rf "${MTG_PKG}" "${MTG_PKG}.tar.gz"
chmod +x "mtg-linux-${FNAME}"
curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
curl -sfLRo geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
curl -sfLRo geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
curl -sfLRo geoip_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat
curl -sfLRo geosite_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat
cd ../../
