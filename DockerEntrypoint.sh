#!/bin/sh

# Start fail2ban
fail2ban-client -x start

# Docker Logs
#ln -sf /dev/stdout /app/access.log
#ln -sf /dev/stdout /app/error.log

# Run x-ui
exec /app/x-ui
