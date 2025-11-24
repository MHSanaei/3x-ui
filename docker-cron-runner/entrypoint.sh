#!/usr/bin/env sh
set -e

: "${CRON_SCHEDULE:=0 */6 * * *}"
: "${DOCKER_PROXY_URL:?DOCKER_PROXY_URL is required}"
: "${TARGET_CONTAINER_NAME:?TARGET_CONTAINER_NAME is required}"
: "${SHARED_VOLUME_PATH:?SHARED_VOLUME_PATH is required}"

# Скрипт, который будет исполняться по крону
CRON_JOB_SCRIPT="/usr/local/bin/run_update_and_restart.sh"

cat > "$CRON_JOB_SCRIPT" << 'EOF'
#!/usr/bin/env sh
set -e

echo "[$(date)] Starting geodata update..."

# Обновление геоданных
/app/xray-tools.sh update_geodata_in_docker "${SHARED_VOLUME_PATH}"

echo "[$(date)] Geodata update finished, restarting container..."

# Рестарт контейнера через Docker Socket Proxy
curl -s -X POST \
  "${DOCKER_PROXY_URL}/containers/${TARGET_CONTAINER_NAME}/restart" \
  -o /dev/null -w "%{http_code}\n"

echo "[$(date)] Restart request sent."
EOF

chmod +x "$CRON_JOB_SCRIPT"

# Создаём кронтаб
# Важный момент: переменные окружения надо прокинуть в cron.
CRON_ENV_FILE="/env.sh"
env | grep -v '^CRON_SCHEDULE=' | sed 's/^/export /' > "$CRON_ENV_FILE"

# crond не тянет env напрямую, поэтому в крон-строке source env-файла
echo "${CRON_SCHEDULE} . ${CRON_ENV_FILE} && ${CRON_JOB_SCRIPT} >> /var/log/cron.log 2>&1" > /etc/crontabs/root

echo "Starting crond with schedule: ${CRON_SCHEDULE}"
mkdir -p /var/log
touch /var/log/cron.log

bash $CRON_JOB_SCRIPT

# Запускаем crond в foreground
exec crond -f -l 2