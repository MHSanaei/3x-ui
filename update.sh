#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" ]]; then
    echo "Usage: $0 <local-update.zip>" >&2
    exit 2
fi

panel="${XUI_MAIN_FOLDER:-/usr/local/x-ui}/x-ui"
exec "$panel" offline-update "$1" "$panel"
