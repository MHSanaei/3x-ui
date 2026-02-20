#!/usr/bin/env bash
set -euo pipefail

# Upload bundle + deploy script to remote server.
#
# Usage:
#   ./scripts/upload_to_server.sh <archive_name_or_path> <remote_user> <remote_host> [remote_port] [remote_base_dir]
# Example:
#   ./scripts/upload_to_server.sh 3x-ui-custom-20260219-120000.zip root 203.0.113.10 22 /opt/3x-ui-deploy

ARCHIVE_INPUT="${1:-}"
REMOTE_USER="${2:-}"
REMOTE_HOST="${3:-}"
REMOTE_PORT="${4:-22}"
REMOTE_BASE_DIR="${5:-/home/$REMOTE_USER/panel}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEPLOY_SCRIPT_LOCAL="${SCRIPT_DIR}/deploy_on_server.sh"
COMMON_SCRIPT_LOCAL="${SCRIPT_DIR}/common.sh"
# shellcheck source=scripts/common.sh
. "${COMMON_SCRIPT_LOCAL}"

usage() {
  echo "Usage: $0 <archive_name_or_path> <remote_user> <remote_host> [remote_port] [remote_base_dir]"
  echo "Example: $0 3x-ui-custom-20260219-120000.zip root 203.0.113.10 22 /opt/3x-ui-deploy"
}

if [[ "${ARCHIVE_INPUT}" == "-h" || "${ARCHIVE_INPUT}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ -z "${ARCHIVE_INPUT}" || -z "${REMOTE_USER}" || -z "${REMOTE_HOST}" ]]; then
  echo "Error: archive, remote_user, and remote_host are required."
  usage
  exit 1
fi

if [[ ! -f "${DEPLOY_SCRIPT_LOCAL}" ]]; then
  echo "Error: ${DEPLOY_SCRIPT_LOCAL} not found."
  exit 1
fi

require_cmd scp
require_cmd ssh

if [[ "${ARCHIVE_INPUT}" = /* ]]; then
  ARCHIVE_PATH="${ARCHIVE_INPUT}"
else
  ARCHIVE_PATH="${REPO_ROOT}/${ARCHIVE_INPUT}"
fi

if [[ ! -f "${ARCHIVE_PATH}" ]]; then
  echo "Error: archive not found: ${ARCHIVE_PATH}"
  exit 1
fi

ARCHIVE_NAME="$(basename "${ARCHIVE_PATH}")"

echo "Ensuring remote directory exists: ${REMOTE_BASE_DIR}"
ssh -p "${REMOTE_PORT}" "${REMOTE_USER}@${REMOTE_HOST}" "mkdir -p '${REMOTE_BASE_DIR}'"

echo "Uploading archive: ${ARCHIVE_NAME}"
scp -P "${REMOTE_PORT}" "${ARCHIVE_PATH}" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_BASE_DIR}/"

echo "Uploading deploy script..."
scp -P "${REMOTE_PORT}" "${DEPLOY_SCRIPT_LOCAL}" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_BASE_DIR}/"
echo "Uploading common script..."
scp -P "${REMOTE_PORT}" "${COMMON_SCRIPT_LOCAL}" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_BASE_DIR}/"

echo
echo "Upload complete."
echo "Next, run on server:"
echo "ssh -p ${REMOTE_PORT} ${REMOTE_USER}@${REMOTE_HOST} 'cd ${REMOTE_BASE_DIR} && chmod +x deploy_on_server.sh common.sh && ./deploy_on_server.sh ${ARCHIVE_NAME}'"
