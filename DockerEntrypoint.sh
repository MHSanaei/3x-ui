#!/bin/sh

FINISH_FILE="$GEODATA_DIR/cron-job-finished.txt"

while [ ! -f "$FINISH_FILE" ]; do
  echo "Still waiting... (looking for $FINISH_FILE)"
  sleep 10
done

# Start fail2ban
[ "$XUI_ENABLE_FAIL2BAN" = "true" ] && fail2ban-client -x start

# Run x-ui
exec /app/x-ui
