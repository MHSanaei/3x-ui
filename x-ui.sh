#!/usr/bin/env bash
set -euo pipefail

panel="${XUI_MAIN_FOLDER:-/usr/local/x-ui}/x-ui"
case "${1:-}" in
    start|stop|restart|status|enable|disable)
        exec systemctl "$1" x-ui.service
        ;;
    restart-xray)
        exec systemctl kill --kill-whom=main --signal=SIGUSR1 x-ui.service
        ;;
    log)
        exec journalctl -u x-ui.service
        ;;
    update)
        if [[ $# -ne 2 || ! -f "$2" ]]; then
            echo "Usage: x-ui update <local-update.zip>" >&2
            exit 2
        fi
        exec "$panel" offline-update "$2" "$panel"
        ;;
    settings)
        exec "$panel" setting -show true
        ;;
    setting|cert|migrate-db)
        exec "$panel" "$@"
        ;;
    *)
        echo "Commands: start, stop, restart, restart-xray, status, enable, disable, log, settings, setting, cert, migrate-db, update <local-update.zip>"
        ;;
esac
