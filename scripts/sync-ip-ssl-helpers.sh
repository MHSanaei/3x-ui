#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
mode='write'
if [[ "${1:-}" == --check ]]; then mode=check; shift; fi
targets=("$@")
if (($# == 0)); then targets=(x-ui.sh install.sh); fi
temp=$(mktemp)
trap 'rm -f "$temp"' EXIT
for target in "${targets[@]}"; do
    case "$target" in x-ui.sh|install.sh) ;; *) printf 'Unknown standalone script: %s\n' "$target" >&2; exit 1;; esac
    awk -v helper="$ROOT/scripts/lib/ip-ssl.sh" '
      {sub(/\r$/, "")}
      /^# BEGIN GENERATED IP SSL HELPERS$/ {
        found++; print; while ((getline line < helper) > 0) print line; close(helper); block=1; next
      }
      /^# END GENERATED IP SSL HELPERS$/ {block=0}
      !block {print}
      END {if(found!=1 || block) exit 1}
    ' "$ROOT/$target" > "$temp"
    if [[ "$mode" == check ]]; then
        if ! cmp -s "$temp" "$ROOT/$target"; then
            printf 'Stale IP SSL helpers in %s; run bash scripts/sync-ip-ssl-helpers.sh\n' "$target" >&2
            exit 1
        fi
    else
        cat "$temp" > "$ROOT/$target"
    fi
done
