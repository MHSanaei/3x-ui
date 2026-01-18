#!/usr/bin/env sh
set -eu

echo "[$(date)] Starting geodata update..."

/app/geo.sh update_all_geofiles "${SHARED_VOLUME_PATH}"

echo "[$(date)] Geodata update finished, restarting container..."

HTTP_CODE=$(
  curl -s -X POST \
    "${DOCKER_PROXY_URL}/containers/${TARGET_CONTAINER_NAME}/restart" \
    -o /dev/null -w "%{http_code}"
)

echo "[$(date)] Restart request sent, HTTP status: ${HTTP_CODE}"