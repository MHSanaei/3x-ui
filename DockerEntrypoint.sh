#!/bin/sh

set -eu

: "${MAX_GEODATA_DIR_WAIT:=30}"
: "${WAIT_INTERVAL:=10}"
: "${GEODATA_DIR:?GEODATA_DIR is required}"

FINISH_FILE="$GEODATA_DIR/cron-job-finished.txt"
ELAPSED=0

while [ ! -f "$FINISH_FILE" ] && [ "$ELAPSED" -lt "$MAX_GEODATA_DIR_WAIT" ]; do
  echo "Waiting for geodata initialization... ($ELAPSED/$MAX_GEODATA_DIR_WAIT seconds)"
  sleep $WAIT_INTERVAL
  ELAPSED=$((ELAPSED + WAIT_INTERVAL))
done

if [ ! -f "$FINISH_FILE" ]; then
  echo "ERROR: Geodata initialization timed out after $MAX_GEODATA_DIR_WAIT seconds"
  echo "Container startup aborted."
  exit 1
fi

# Start fail2ban
[ "$XUI_ENABLE_FAIL2BAN" = "true" ] && fail2ban-client -x start

# Run x-ui
exec /app/x-ui
