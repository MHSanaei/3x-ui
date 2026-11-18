#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 || ! -f "$1" || ! -x "$2" ]]; then
    echo "Usage: $0 <local-update.zip> <built-offline-x-ui-binary>" >&2
    exit 2
fi

repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
case "$(uname -s)" in
    Linux) ;;
    *) echo "Offline installation requires Linux and systemd." >&2; exit 2 ;;
esac
. /etc/os-release
case "${ID:-}" in
    debian|ubuntu|armbian) template="$repo/x-ui.service.debian" ;;
    arch|manjaro) template="$repo/x-ui.service.arch" ;;
    *) template="$repo/x-ui.service.rhel" ;;
esac
"$2" offline-install "$1" "$template" "$repo/x-ui.sh"
