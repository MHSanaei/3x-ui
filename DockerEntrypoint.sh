#!/bin/sh

# Start fail2ban
[ $X_UI_ENABLE_FAIL2BAN == "true" ] && fail2ban-client -x start

# Run x-ui
exec /app/x-ui
