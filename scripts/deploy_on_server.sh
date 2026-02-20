#!/usr/bin/env bash
set -euo pipefail

# Server script:
# 1) Unzip uploaded bundle
# 2) Copy code into target app directory
# 3) Rebuild + restart Docker compose stack
#
# Usage:
#   ./deploy_on_server.sh <archive-name.zip> [app_dir]
# Example:
#   ./deploy_on_server.sh 3x-ui-custom-20260219-120000.zip ~/panel

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMMON_SH="${SCRIPT_DIR}/common.sh"
if [[ -f "${COMMON_SH}" ]]; then
  # shellcheck source=scripts/common.sh
  . "${COMMON_SH}"
else
  require_cmd() {
    local cmd="$1"
    if ! command -v "${cmd}" >/dev/null 2>&1; then
      echo "Error: ${cmd} is required but not installed."
      exit 1
    fi
  }
fi

APP_DIR="${2:-$HOME/panel}"

ARCHIVE_NAME="${1:-}"
if [[ -z "${ARCHIVE_NAME}" ]]; then
  echo "Usage: $0 <archive-name.zip>"
  exit 1
fi

if [[ ! -f "${ARCHIVE_NAME}" ]]; then
  echo "Error: archive not found in current directory: ${ARCHIVE_NAME}"
  exit 1
fi

require_cmd unzip
require_cmd docker
require_cmd mktemp

compose_cmd() {
  if docker compose version >/dev/null 2>&1; then
    echo "docker compose"
    return
  fi

  if command -v docker-compose >/dev/null 2>&1; then
    echo "docker-compose"
    return
  fi

  echo ""
}

COMPOSE="$(compose_cmd)"
if [[ -z "${COMPOSE}" ]]; then
  echo "Error: neither 'docker compose' nor 'docker-compose' is available."
  exit 1
fi

WORK_DIR="$(pwd)"
TMP_EXTRACT_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_EXTRACT_DIR}"' EXIT

echo "Extracting bundle to temp dir..."
unzip -oq "${ARCHIVE_NAME}" -d "${TMP_EXTRACT_DIR}"

mkdir -p "${APP_DIR}"

echo "Syncing code to ${APP_DIR}..."
if command -v rsync >/dev/null 2>&1; then
  rsync -a --delete \
    --exclude "db/" \
    --exclude "cert/" \
    "${TMP_EXTRACT_DIR}/" "${APP_DIR}/"
else
  find "${APP_DIR}" -mindepth 1 -maxdepth 1 \
    ! -name "db" \
    ! -name "cert" \
    -exec rm -rf {} +
  cp -a "${TMP_EXTRACT_DIR}/." "${APP_DIR}/"
fi

cd "${APP_DIR}"
mkdir -p db cert

# Stop old container if present, then rebuild and run
${COMPOSE} down || true
${COMPOSE} build
${COMPOSE} up -d

echo "Deployment complete."
${COMPOSE} ps
echo "App directory: ${APP_DIR}"
echo "Archive used: ${WORK_DIR}/${ARCHIVE_NAME}"
