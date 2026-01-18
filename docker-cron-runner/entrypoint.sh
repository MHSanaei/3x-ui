#!/usr/bin/env sh

set -eu

: "${CRON_SCHEDULE:=0 */6 * * *}"
: "${DOCKER_PROXY_URL:?DOCKER_PROXY_URL is required}"
: "${TARGET_CONTAINER_NAME:?TARGET_CONTAINER_NAME is required}" # required for cron-job-script.sh for container restart
: "${SHARED_VOLUME_PATH:?SHARED_VOLUME_PATH is required}"

CRON_ENV_FILE="/env.sh"

env | grep -v '^CRON_SCHEDULE=' | sed 's/^/export /' > "$CRON_ENV_FILE"
echo "${CRON_SCHEDULE} . ${CRON_ENV_FILE} && /app/cron-job-script.sh >> /var/log/cron.log 2>&1" > /etc/crontabs/root

echo "Starting crond with schedule: ${CRON_SCHEDULE}"

mkdir -p /var/log
touch /var/log/cron.log

mkdir -p "$SHARED_VOLUME_PATH"

exec crond -f -l 2