#!/usr/bin/env bash
set -euo pipefail

# Package current repo into a timestamped zip bundle.
#
# Usage:
#   ./scripts/package_local.sh [archive_prefix]
# Example:
#   ./scripts/package_local.sh 3x-ui-custom
#
# Output:
#   Prints the created archive file name on the last line.

ARCHIVE_PREFIX="${1:-3x-ui-custom}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=scripts/common.sh
. "${SCRIPT_DIR}/common.sh"

require_cmd zip

TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
ARCHIVE_NAME="${ARCHIVE_PREFIX}-${TIMESTAMP}.zip"
ARCHIVE_PATH="${REPO_ROOT}/${ARCHIVE_NAME}"

cd "${REPO_ROOT}"

echo "Creating archive: ${ARCHIVE_PATH}"
zip -r "${ARCHIVE_PATH}" . \
  -x "./.git/*" \
  -x "./.github/*" \
  -x "./.opencode/*" \
  -x "./.playwright-cli/*" \
  -x "./.tmpdb/*" \
  -x "./.tmplogs/*" \
  -x "./node_modules/*" \
  -x "./tmp/*" \
  -x "./output/*" \
  -x "./backups/*" \
  -x "./*.zip" \
  -x "./.DS_Store"

echo "Created: ${ARCHIVE_NAME}"
echo "${ARCHIVE_NAME}"
