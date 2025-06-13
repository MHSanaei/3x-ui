#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

# ------------------------------------------------------------------------------
# DockerInit.sh — download and prepare Xray binaries and geolocation databases
# ------------------------------------------------------------------------------

# Xray version
readonly XRAY_VERSION="v25.5.16"

# URL template for downloading Xray
readonly XRAY_URL_TEMPLATE="https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/Xray-linux-%s.zip"

# Directories
readonly BUILD_DIR="build/bin"
readonly ROOT_DIR="$(pwd)"

# Check for required utilities
check_dependencies() {
    local deps=(wget unzip)
    for cmd in "${deps[@]}"; do
        if ! command -v "$cmd" &> /dev/null; then
            echo "Error: Required utility '$cmd' is not installed. Please install it and try again." >&2
            exit 1
        fi
    done
}

# Print usage help
usage() {
    cat <<EOF
Usage: $0 <architecture>

Supported architectures:
  amd64    ? 64-bit x86
  i386     ? 32-bit x86
  armv8    ? ARM64-v8a (also accepts arm64, aarch64)
  armv7    ? ARM32-v7a (also accepts arm, arm32)
  armv6    ? ARM32-v6

If no argument is provided or the argument is not recognized, 'amd64' will be used by default.

Example:
  $0 armv7
EOF
    exit 1
}

# Determine ARCH and FNAME based on input argument
detect_arch() {
    local input="$1"

    case "$input" in
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
        "")
            # If argument is empty, default to amd64
            ARCH="64"
            FNAME="amd64"
            ;;
        *)
            echo "Warning: Architecture '$input' not recognized. Defaulting to 'amd64'." >&2
            ARCH="64"
            FNAME="amd64"
            ;;
    esac
}

# Generic function to download a file by URL (with error handling)
download_file() {
    local url="$1"
    local output="$2"

    echo "Downloading: $url"
    if ! wget -q -O "$output" "$url"; then
        echo "Error: Failed to download '$url'" >&2
        exit 1
    fi
}

# Main function: create directory, download and unpack Xray, then geolocation databases
main() {
    # Check dependencies
    check_dependencies

    # Get architecture from argument
    local ARCH_ARG="${1-}"
    detect_arch "$ARCH_ARG"

    # Construct URL for Xray download
    local xray_url
    printf -v xray_url "$XRAY_URL_TEMPLATE" "$ARCH"

    # Create build directory
    echo "Creating directory: $BUILD_DIR"
    mkdir -p "$BUILD_DIR"
    cd "$BUILD_DIR" || exit 1

    # Download and unpack Xray
    local xray_zip="Xray-linux-${ARCH}.zip"
    download_file "$xray_url" "$xray_zip"
    echo "Unpacking $xray_zip"
    unzip -q "$xray_zip"
    rm -f "$xray_zip" geoip.dat geosite.dat

    # Rename binary according to target architecture
    mv xray "xray-linux-${FNAME}"
    chmod +x "xray-linux-${FNAME}"

    # Return to project root
    cd "$ROOT_DIR" || exit 1

    # Download standard GeoIP and GeoSite databases
    echo "Downloading default GeoIP and GeoSite databases"
    download_file "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat" "geoip.dat"
    download_file "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat" "geosite.dat"

    # Download alternative GeoIP/GeoSite for Iran (IR) and Russia (RU)
    echo "Downloading alternative GeoIP/GeoSite for Iran"
    download_file "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat" "geoip_IR.dat"
    download_file "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat" "geosite_IR.dat"

    echo "Downloading alternative GeoIP/GeoSite for Russia"
    download_file "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat" "geoip_RU.dat"
    download_file "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat" "geosite_RU.dat"

    echo "Done."
}

# If -h or --help is passed, show usage
if [[ "${1-}" == "-h" || "${1-}" == "--help" ]]; then
    usage
fi

# Run main with the provided argument (if any)
main "${1-}"
