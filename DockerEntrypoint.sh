#!/bin/sh

# Run sshd
/usr/sbin/sshd

# Start nginx in the background
nginx

# Start fail2ban
## fail2ban-client -x start

# Run x-ui as main process
exec /app/x-ui
