#!/usr/bin/env bash
set -Eeuo pipefail

# Replace a Docker-based 3x-ui instance with a custom build from current repo.
# - Backs up db/cert folders
# - Builds a custom image from current source
# - Replaces running container with docker compose
# - Saves rollback info

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
SERVICE_NAME="${SERVICE_NAME:-3xui}"
CONTAINER_NAME="${CONTAINER_NAME:-3xui_app}"
BACKUP_ROOT="${BACKUP_ROOT:-$SCRIPT_DIR/backups}"
USE_COMPOSE_BUILD="${USE_COMPOSE_BUILD:-0}"

timestamp="$(date +%F-%H%M%S)"
git_sha="$(git rev-parse --short HEAD 2>/dev/null || echo no-git)"
new_tag="3xui-custom:${git_sha}-${timestamp}"

log() {
  printf '[%s] %s\n' "$(date '+%F %T')" "$*"
}

die() {
  log "ERROR: $*"
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

need_cmd docker
need_cmd cp
need_cmd mkdir
need_cmd date

if docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD=(docker compose -f "$COMPOSE_FILE")
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD=(docker-compose -f "$COMPOSE_FILE")
else
  die "Neither 'docker compose' nor 'docker-compose' is available"
fi

[ -f "$COMPOSE_FILE" ] || die "Compose file not found: $COMPOSE_FILE"

mkdir -p "$BACKUP_ROOT"
backup_dir="$BACKUP_ROOT/$timestamp"
mkdir -p "$backup_dir"

log "Starting replacement using compose file: $COMPOSE_FILE"
log "Backup directory: $backup_dir"

if [ -d "$SCRIPT_DIR/db" ]; then
  cp -a "$SCRIPT_DIR/db" "$backup_dir/db"
  log "Backed up db to $backup_dir/db"
else
  log "No ./db directory found, skipping db backup"
fi

if [ -d "$SCRIPT_DIR/cert" ]; then
  cp -a "$SCRIPT_DIR/cert" "$backup_dir/cert"
  log "Backed up cert to $backup_dir/cert"
else
  log "No ./cert directory found, skipping cert backup"
fi

old_image="$(docker inspect -f '{{.Config.Image}}' "$CONTAINER_NAME" 2>/dev/null || true)"
if [ -n "$old_image" ]; then
  rollback_tag="3xui-custom:rollback-${timestamp}"
  docker image tag "$old_image" "$rollback_tag"
  log "Tagged current running image for rollback: $rollback_tag (from $old_image)"
else
  rollback_tag=""
  log "No running container named $CONTAINER_NAME found, proceeding as fresh deploy"
fi

if [ "$USE_COMPOSE_BUILD" = "1" ]; then
  log "Building via compose service '$SERVICE_NAME'"
  "${COMPOSE_CMD[@]}" build "$SERVICE_NAME"
else
  log "Building custom image from current repo: $new_tag"
  docker build -t "$new_tag" .
fi

override_file="$backup_dir/docker-compose.override.generated.yml"
if [ "$USE_COMPOSE_BUILD" = "0" ]; then
  cat > "$override_file" <<EOF
services:
  $SERVICE_NAME:
    image: $new_tag
EOF
  COMPOSE_RUN_CMD=("${COMPOSE_CMD[@]}" -f "$override_file")
else
  COMPOSE_RUN_CMD=("${COMPOSE_CMD[@]}")
fi

log "Stopping current stack"
"${COMPOSE_RUN_CMD[@]}" down

log "Starting new stack"
"${COMPOSE_RUN_CMD[@]}" up -d

log "Waiting for container to settle..."
sleep 2

if docker ps --format '{{.Names}}' | grep -qx "$CONTAINER_NAME"; then
  log "Container is running: $CONTAINER_NAME"
else
  die "Container $CONTAINER_NAME is not running after deployment"
fi

log "Recent logs:"
docker logs --tail 60 "$CONTAINER_NAME" || true

meta_file="$backup_dir/deploy-meta.txt"
{
  echo "timestamp=$timestamp"
  echo "git_sha=$git_sha"
  echo "compose_file=$COMPOSE_FILE"
  echo "service_name=$SERVICE_NAME"
  echo "container_name=$CONTAINER_NAME"
  echo "new_image_tag=$new_tag"
  echo "rollback_image_tag=$rollback_tag"
  echo "use_compose_build=$USE_COMPOSE_BUILD"
  echo "override_file=$override_file"
} > "$meta_file"

log "Deployment metadata saved: $meta_file"
log "Replacement completed successfully."
echo
echo "Rollback quick reference:"
if [ -n "$rollback_tag" ]; then
  cat <<EOF
1) Create a temporary override that points to rollback image:
   cat > /tmp/3xui-rollback.yml <<ROLLBACK
services:
  $SERVICE_NAME:
    image: $rollback_tag
ROLLBACK

2) Redeploy rollback:
   docker compose -f "$COMPOSE_FILE" -f /tmp/3xui-rollback.yml down
   docker compose -f "$COMPOSE_FILE" -f /tmp/3xui-rollback.yml up -d
EOF
else
  echo "No prior running image was found, so no rollback tag was created."
fi
