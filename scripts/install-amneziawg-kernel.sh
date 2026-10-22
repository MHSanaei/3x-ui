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

KERNEL="$(uname -r)"
SRC_DIR="${SRC_DIR:-/usr/local/src}"
MODULE_REPO="https://github.com/amnezia-vpn/amneziawg-linux-kernel-module.git"
TOOLS_REPO="https://github.com/amnezia-vpn/amneziawg-tools.git"

# The module and tools must come from the same 3.x series. In 3.x the H1-H4
# magic headers are u64 ranges; 1.0 tools still send a u32 and `awg setconf`
# fails with "Unable to modify interface: Invalid argument", leaving the
# interface down with no useful diagnostic.
MODULE_TAG_FILTER="${MODULE_TAG_FILTER:-^v3\.1}"
TOOLS_TAG_FILTER="${TOOLS_TAG_FILTER:-^v3\.}"

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

latest_tag() {
	git ls-remote --tags --refs "$1" | awk -F/ '{print $NF}' | grep -E "$2" | sort -V | tail -1
}

MODULE_TAG="$(latest_tag "$MODULE_REPO" "$MODULE_TAG_FILTER")"
TOOLS_TAG="$(latest_tag "$TOOLS_REPO" "$TOOLS_TAG_FILTER")"
[ -n "$MODULE_TAG" ] || die "no module tag matched $MODULE_TAG_FILTER"
[ -n "$TOOLS_TAG" ] || die "no tools tag matched $TOOLS_TAG_FILTER"
log "Module $MODULE_TAG, tools $TOOLS_TAG"

mkdir -p "$SRC_DIR"
cd "$SRC_DIR"
rm -rf amneziawg-linux-kernel-module amneziawg-tools
git clone -q --depth 1 --branch "$MODULE_TAG" "$MODULE_REPO"
git clone -q --depth 1 --branch "$TOOLS_TAG" "$TOOLS_REPO"

MODULE_SRC="$SRC_DIR/amneziawg-linux-kernel-module/src"

# Linux 7.1.5 changed setup_udp_tunnel_sock/udp_tunnel_sock_release to take a
# struct sock* where they used to take a struct socket*. Until the module
# carries the fix upstream, rewrite the four call sites.
if grep -q 'udp_tunnel_sock_release(sock->sk_socket)' "$MODULE_SRC/socket.c"; then
	log "Patching udp_tunnel call sites for kernel >= 7.1.5"
	perl -0777 -i -pe '
		s/setup_udp_tunnel_sock\(net, new([46]), /setup_udp_tunnel_sock(net, new$1->sk, /g;
		s/udp_tunnel_sock_release\(new4\)/udp_tunnel_sock_release(new4->sk)/g;
		s/udp_tunnel_sock_release\(sock->sk_socket\)/udp_tunnel_sock_release(sock)/g;
	' "$MODULE_SRC/socket.c"
	! grep -q 'udp_tunnel_sock_release(sock->sk_socket)' "$MODULE_SRC/socket.c" || die "udp_tunnel patch did not apply"
fi

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
