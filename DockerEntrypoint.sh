#!/bin/sh

# Start fail2ban
fail2ban-client -x -f start

# Run x-ui
exec /app/x-ui
