#!/bin/bash

# Check if the file exists
if [ ! -f "/root/x-ui_copied.db" ]; then
  echo "DB file not found. Downloading..."
  curl -L -o /root/x-ui_copied.db "https://drive.google.com/uc?export=download&id=${FILE_ID}"
  curl -L -o /etc/x-ui/x-ui.db "https://drive.google.com/uc?export=download&id=${FILE_ID}"
else
  echo "DB file already exists. Skipping download."
fi


# Run sshd
/usr/sbin/sshd

# Start nginx in the background
nginx

# Start fail2ban
## fail2ban-client -x start

# Run x-ui as main process
exec /app/x-ui
