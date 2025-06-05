#!/bin/sh

# Start fail2ban
if [ "$XUI_ENABLE_FAIL2BAN" = "true" ]; then
  if command -v fail2ban-client >/dev/null 2>&1; then
    fail2ban-client -x start
  else
    echo "Warning: fail2ban-client not found, but XUI_ENABLE_FAIL2BAN is true."
  fi
fi

# Run x-ui
exec /app/x-ui
