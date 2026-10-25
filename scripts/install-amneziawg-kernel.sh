#!/usr/bin/env bash
#
# Installs the AmneziaWG kernel module and tools, the prerequisite for an
# AmneziaWG inbound's "kernel mode" (internal/amneziawgkern). Without it the
# panel refuses to enable the setting and says why.
#
# Verified on Debian 13 (trixie), kernel 7.1.11-x64v3-xanmod1, x86_64.
set -eu

log() { printf '\n==> %s\n' "$*"; }
die() { printf '\nERROR: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root"
command -v apt-get >/dev/null || die "this installer supports Debian and Ubuntu; elsewhere install amneziawg-linux-kernel-module and amneziawg-tools 3.x by hand"

KERNEL="$(uname -r)"
SRC_DIR="${SRC_DIR:-/usr/local/src}"
MODULE_REPO="https://github.com/amnezia-vpn/amneziawg-linux-kernel-module.git"
TOOLS_REPO="https://github.com/amnezia-vpn/amneziawg-tools.git"

# Pinned to tested releases and verified by commit, since this runs as root.
# Module and tools must share the 3.x series: 1.0 tools send u32 H1-H4 and
# `awg setconf` then fails with "Invalid argument".
MODULE_TAG="v3.1.20260906"
MODULE_COMMIT="4569c4c67f3a57414969260cafbbd04694fbaae0"
TOOLS_TAG="v3.1.20260812"
TOOLS_COMMIT="ee0f0a9aa34ff0a0da4b3433b9512781cfe02843"

log "Kernel: $KERNEL"
[ -d "/lib/modules/$KERNEL/build" ] || die "kernel headers missing; install linux-headers-$KERNEL"

log "Installing build dependencies"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq dkms build-essential git libmnl-dev pkg-config nftables

# A clang-built kernel makes DKMS invoke the module build with LLVM=1, which
# fails outright when only gcc is present. /proc/version names the compiler.
if grep -qi clang /proc/version; then
	log "Kernel was built with clang; installing the LLVM toolchain to match"
	apt-get install -y -qq clang lld llvm
fi

log "Module $MODULE_TAG, tools $TOOLS_TAG"

mkdir -p "$SRC_DIR"
cd "$SRC_DIR"
rm -rf amneziawg-linux-kernel-module amneziawg-tools
git clone -q --depth 1 --branch "$MODULE_TAG" "$MODULE_REPO"
git clone -q --depth 1 --branch "$TOOLS_TAG" "$TOOLS_REPO"
[ "$(git -C amneziawg-linux-kernel-module rev-parse HEAD)" = "$MODULE_COMMIT" ] || die "module $MODULE_TAG is not commit $MODULE_COMMIT"
[ "$(git -C amneziawg-tools rev-parse HEAD)" = "$TOOLS_COMMIT" ] || die "tools $TOOLS_TAG is not commit $TOOLS_COMMIT"

MODULE_SRC="$SRC_DIR/amneziawg-linux-kernel-module/src"

log "Building the module via DKMS"
cd "$MODULE_SRC"
rm -rf /usr/src/amneziawg-1.0.0
make dkms-install
dkms status -m amneziawg -v 1.0.0 | grep -q . || dkms add -m amneziawg -v 1.0.0
dkms build -m amneziawg -v 1.0.0
dkms install -m amneziawg -v 1.0.0 --force

log "Loading the module"
modprobe amneziawg
lsmod | grep -q '^amneziawg' || die "module did not load"

log "Building amneziawg-tools"
cd "$SRC_DIR/amneziawg-tools/src"
make
make install

log "Done"
printf '  module: %s\n' "$(dkms status -m amneziawg)"
printf '  tools:  %s\n' "$(awg --version)"
printf '\nEnable "Kernel mode" on the AmneziaWG inbound in the panel to use it.\n'
printf 'DKMS rebuilds the module on kernel upgrades; Secure Boot hosts must sign it.\n'
