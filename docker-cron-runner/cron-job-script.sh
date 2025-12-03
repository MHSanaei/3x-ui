#!/usr/bin/env sh
set -eu

echo "[$(date)] Starting geodata update..."

FINISHED_FLAG="${SHARED_VOLUME_PATH}/cron-job-finished.txt"

if [ -f "$FINISHED_FLAG" ]; then
  rm -f "$FINISHED_FLAG"
fi

/app/xray-tools.sh update_geodata_in_docker "${SHARED_VOLUME_PATH}"
touch "$FINISHED_FLAG"

echo "[$(date)] Geodata update finished, restarting container..."

HTTP_CODE=$(
  curl -s -X POST \
    "${DOCKER_PROXY_URL}/containers/${TARGET_CONTAINER_NAME}/restart" \
    -o /dev/null -w "%{http_code}"
)

echo "[$(date)] Restart request sent, HTTP status: ${HTTP_CODE}"