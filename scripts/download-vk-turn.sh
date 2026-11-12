#!/bin/sh
set -eu

target_os=$1
target_arch=$2
destination=$3
version=srtp-build478
commit=f954cb04ac17f49059f6fe5630a09428ff04c242
source_sha=ff1319a47d65086c864353b5755b255334f1f001e9ba18fd03db9a39450002c6

case "$target_os/$target_arch" in
    linux/amd64) checksum=21b4c5e02c8b6658521d17ecbbc2f25487b2cd9f863a7ff70a0a66783af63e98 ;;
    linux/arm64) checksum=b06bf4c9f40dfd882d85234b85d26773d9503e00d58661fd3bb15d311678ec12 ;;
    linux/386) checksum=20b560406d0d739c2183d4cff79c8c440ef41abfa74a602bc57edecc2faf5fbc ;;
    linux/armv7) target_arch=arm; checksum=15e6c95320102787ff94605ec611ab3cda13a8f2d83eed7d48e9a68fa6971eca ;;
    linux/armv5 | linux/armv6 | linux/s390x) checksum=source ;;
    darwin/amd64) checksum=cfc92d2b8e9627bc2c0ee29c14b64ff74486976e394a1a92918e120f09fb1b77 ;;
    darwin/arm64) checksum=c779043319d026b388dd9974bd43bc0a8c158e2fd26a575086b44abf25410283 ;;
    freebsd/amd64) checksum=4f3986c6f85e4f6aa2a31dae549c3ddef79421c8137404697bb90bf20dd38131 ;;
    *) echo "VK TURN: unsupported target $target_os/$target_arch" >&2; exit 1 ;;
esac

mkdir -p "$(dirname "$destination")"
destination="$(cd "$(dirname "$destination")" && pwd)/$(basename "$destination")"
download_dir=$(mktemp -d "${destination}.download.XXXXXX")
trap 'rm -rf "$download_dir"' EXIT
trap 'exit 1' HUP INT TERM

verify() {
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$1" | cut -d ' ' -f 1)
    else
        actual=$(shasum -a 256 "$1" | cut -d ' ' -f 1)
    fi
    if [ "$actual" != "$2" ]; then
        echo "VK TURN: checksum mismatch for $target_os/$target_arch" >&2
        exit 1
    fi
}

if [ "$checksum" = source ]; then
    curl -fsSL --retry 5 --retry-all-errors --retry-delay 3 \
        "https://codeload.github.com/anton48/vk-turn-proxy/tar.gz/$commit" -o "$download_dir/source.tar.gz"
    verify "$download_dir/source.tar.gz" "$source_sha"
    tar -xzf "$download_dir/source.tar.gz" -C "$download_dir"
    case "$target_arch" in
        armv5 | armv6) export GOARM=${target_arch#armv}; target_arch=arm ;;
    esac
    (
        cd "$download_dir/vk-turn-proxy-$commit"
        CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
            go build -trimpath -ldflags '-s -w -checklinkname=0' -o "$download_dir/server" ./server
    )
else
    curl -fsSL --retry 5 --retry-all-errors --retry-delay 3 \
        "https://github.com/anton48/vk-turn-proxy/releases/download/$version/server-$target_os-$target_arch" \
        -o "$download_dir/server"
    verify "$download_dir/server" "$checksum"
fi

chmod 755 "$download_dir/server"
mv -f "$download_dir/server" "$destination"
