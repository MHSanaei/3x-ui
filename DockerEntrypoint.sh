#!/usr/bin/env bash
# Description:
#   1) Handle SIGTERM for graceful shutdown
#   2) Run all scripts from /mnt/sh/beforestart before starting the main application
#   3) Copy binaries from /mnt/bin to /app/bin
#   4) Launch x-ui in the background and capture its PID
#   5) After x-ui has started, run scripts from /mnt/sh/afterstart
#   6) Start fail2ban if enabled
#   7) Wait for x-ui to exit or receive a signal

set -euo pipefail

###############################################################################
# Logging function
###############################################################################
log() {
  local level=$1
  shift
  echo "$(date '+%Y-%m-%d %H:%M:%S') $level $*"
}

###############################################################################
# Function: start_fail2ban_if_enabled
###############################################################################
start_fail2ban_if_enabled() {
  if [ "${XUI_ENABLE_FAIL2BAN:-false}" != "true" ]; then
    log INFO "Fail2Ban is disabled; skipping start."
    return
  fi

  log INFO "Attempting to start Fail2Ban..."
  if fail2ban-client -x start; then
    log INFO "Fail2Ban started successfully."
  else
    log ERROR "Fail2Ban failed to start."
    exit 1
  fi
}

###############################################################################
# 1) Handle SIGTERM for graceful shutdown
###############################################################################
# When a SIGTERM is received, properly stop the child process (x-ui)
term_handler() {
  log INFO "Caught SIGTERM. Stopping x-ui..."
  if [[ -n "${XUI_PID:-}" ]]; then
    kill -TERM "$XUI_PID" 2>/dev/null || true
    wait "$XUI_PID"
  fi
  log INFO "x-ui stopped."
  exit 0
}
trap 'term_handler' SIGTERM

###############################################################################
# 2) Execute scripts from /mnt/sh/beforestart
###############################################################################
if [ -d "/mnt/sh/beforestart" ]; then
  log INFO "Detected /mnt/sh/beforestart directory. Setting permissions..."
  chmod -R 777 /mnt/sh/beforestart

  log INFO "Searching for scripts in /mnt/sh/beforestart..."
  find /mnt/sh/beforestart -type f -name "*.sh" -print0 | sort -z | while IFS= read -r -d '' script; do
    log INFO "Executing script: $script"
    sh "$script"
    if [ $? -ne 0 ]; then
      log ERROR "Script failed: $script. Aborting startup."
      exit 1
    fi
  done

  log INFO "All scripts in /mnt/sh/beforestart executed successfully."
fi

###############################################################################
# 3) Copy binaries from /mnt/bin to /app/bin
###############################################################################
if [ -d "/mnt/bin" ]; then
  log INFO "Detected /mnt/bin directory. Copying contents to /app/bin..."
  if ! cp -r /mnt/bin/* /app/bin/; then
    log ERROR "Failed to copy from /mnt/bin to /app/bin. Aborting startup."
    exit 1
  fi
  log INFO "Binaries copied to /app/bin successfully."
fi

###############################################################################
# 4) Launch the main application x-ui in the background
###############################################################################
log INFO "Launching x-ui in background..."
/app/x-ui &
XUI_PID=$!
log INFO "x-ui PID is $XUI_PID."

# Sleep briefly to ensure the process has started before running afterstart scripts
sleep 10

###############################################################################
# 5) Execute scripts from /mnt/sh/afterstart after x-ui has started
###############################################################################
if [ -d "/mnt/sh/afterstart" ]; then
  log INFO "Detected /mnt/sh/afterstart directory. Setting permissions..."
  chmod -R 777 /mnt/sh/afterstart

  log INFO "Searching for scripts in /mnt/sh/afterstart..."
  find /mnt/sh/afterstart -type f -name "*.sh" -print0 | sort -z | while IFS= read -r -d '' script; do
    log INFO "Executing script: $script"
    sh "$script"
    if [ $? -ne 0 ]; then
      log ERROR "Script failed: $script. Aborting startup."
      # Stop x-ui if an afterstart script fails
      kill -TERM "$XUI_PID" 2>/dev/null || true
      wait "$XUI_PID"
      exit 1
    fi
  done

  log INFO "All scripts in /mnt/sh/afterstart executed successfully."
fi

###############################################################################
# 6) Start Fail2Ban if enabled
###############################################################################
start_fail2ban_if_enabled

###############################################################################
# 7) Wait for x-ui to exit (or receive a signal)
###############################################################################
log INFO "Waiting for x-ui (PID $XUI_PID) to exit..."
wait "$XUI_PID"
EXIT_CODE=$?
log INFO "x-ui exited with code $EXIT_CODE."
exit "$EXIT_CODE"
