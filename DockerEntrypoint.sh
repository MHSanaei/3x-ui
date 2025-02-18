#!/bin/sh

# Start fail2ban
[ $X_UI_ENABLE_FAIL2BAN == "true" ] && fail2ban-client -x start

# Docker Logs
#ln -sf /dev/stdout /app/access.log
#ln -sf /dev/stdout /app/error.log

# Run x-ui
exec /app/x-ui
