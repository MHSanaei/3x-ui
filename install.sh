#!/usr/bin/env bash
#
# 4x-ui Reseller Panel Installer
# Installs the forked 3x-ui panel from Azizpouryan/4x-ui (branch: arena/6412f46a-4x-ui)
# with reseller capability on top of the original mhsanaei/3x-ui v3.8.5 baseline.
#
# Usage:
#   bash <(curl -Ls https://raw.githubusercontent.com/Azizpouryan/4x-ui/arena/6412f46a-4x-ui/install.sh)
#
set -euo pipefail

REPO_OWNER="Azizpouryan"
REPO_NAME="4x-ui"
REPO_BRANCH="arena/6412f46a-4x-ui"
INSTALL_DIR="/opt/4x-ui"
BIN_NAME="xray-ui"
SERVICE_NAME="4x-ui"
PANEL_PORT="5443"

# -----------------------------------------------------------------------------
# Helpers
# -----------------------------------------------------------------------------
log()  { printf '\033[1;32m[%s]\033[0m %s\n' "$(date '+%H:%M:%S')" "$*"; }
warn() { printf '\033[1;33m[WARN]\033[0m %s\n' "$*"; }
err()  { printf '\033[1;31m[ERR]\033[0m %s\n' "$*" >&2; }

require_root() {
    if [[ "$(id -u)" -ne 0 ]]; then
        err "This script must be run as root (use sudo)."
        exit 1
    fi
}

detect_os() {
    if [[ -f /etc/os-release ]]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        OS_ID="${ID}"
        OS_VERSION="${VERSION_ID}"
    else
        err "Cannot detect OS (/etc/os-release missing)."
        exit 1
    fi
}

install_dependencies() {
    log "Updating package index..."
    apt-get update -y

    log "Installing build dependencies..."
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
        curl wget git build-essential gcc gzip tar ca-certificates

    # ---------- Go ----------
    if ! command -v go >/dev/null 2>&1 || [[ "$(go version | awk '{print $3}')" != "go1.22"* ]]; then
        log "Installing Go toolchain..."
        GO_TARBALL="go1.22.5.linux-amd64.tar.gz"
        wget -q "https://go.dev/dl/${GO_TARBALL}" -O "/tmp/${GO_TARBALL}"
        rm -rf /usr/local/go
        tar -C /usr/local -xzf "/tmp/${GO_TARBALL}"
        rm -f "/tmp/${GO_TARBALL}"
        export PATH="/usr/local/go/bin:${PATH}"
        if ! grep -q '/usr/local/go/bin' /etc/profile.d/go.sh 2>/dev/null; then
            echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
        fi
    else
        log "Go already installed: $(go version)"
        export PATH="/usr/local/go/bin:${PATH}"
    fi

    # ---------- Node.js ----------
    if ! command -v node >/dev/null 2>&1 || ! node -v | grep -qE 'v2[0-9]'; then
        log "Installing Node.js 20.x..."
        curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
        DEBIAN_FRONTEND=noninteractive apt-get install -y nodejs
    else
        log "Node.js already installed: $(node -v)"
    fi
}

clone_repo() {
    log "Cloning ${REPO_OWNER}/${REPO_NAME} (branch: ${REPO_BRANCH}) into ${INSTALL_DIR}..."

    if [[ -d "${INSTALL_DIR}" ]]; then
        warn "${INSTALL_DIR} already exists. Updating instead of re-cloning."
        cd "${INSTALL_DIR}"
        git fetch origin
        git checkout "${REPO_BRANCH}"
        git pull origin "${REPO_BRANCH}" || true
    else
        git clone --branch "${REPO_BRANCH}" --single-branch \
            "https://github.com/${REPO_OWNER}/${REPO_NAME}.git" "${INSTALL_DIR}"
        cd "${INSTALL_DIR}"
    fi
}

build_backend() {
    log "Building Go backend..."
    cd "${INSTALL_DIR}"
    export PATH="/usr/local/go/bin:${PATH}"
    go version
    go build -o "${BIN_NAME}" .
    log "Backend binary: ${INSTALL_DIR}/${BIN_NAME}"
}

build_frontend() {
    log "Installing frontend dependencies and building React app..."
    cd "${INSTALL_DIR}/frontend"
    export PATH="/usr/local/go/bin:${PATH}"
    npm install --no-audit --no-fund
    npm run build
    log "Frontend built in ${INSTALL_DIR}/frontend/dist"
}

create_service() {
    log "Creating systemd service ${SERVICE_NAME}..."

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=4x-ui Reseller Panel (${REPO_BRANCH})
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${BIN_NAME}
Restart=on-failure
RestartSec=5
Environment=PATH=/usr/local/go/bin:/usr/bin:/bin

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"
    systemctl restart "${SERVICE_NAME}"

    log "Service ${SERVICE_NAME} enabled and started."
}

show_summary() {
    local ip
    ip="$(curl -s https://api.ipify.org 2>/dev/null || curl -s https://ifconfig.me 2>/dev/null || echo 'YOUR_SERVER_IP')"

    cat <<EOF

===============================================================================
  4x-ui Reseller Panel installation complete!
===============================================================================

  Panel URL : http://${ip}:${PANEL_PORT}/
  Login     : admin / admin   (change after first login)

  Service   : systemctl status ${SERVICE_NAME}
  Logs      : journalctl -u ${SERVICE_NAME} -f
  Directory : ${INSTALL_DIR}
  Branch    : ${REPO_BRANCH}

  Features included:
    - Reseller CRUD (create/list/update/delete)
    - Per-reseller traffic and client quotas
    - Reseller inbound assignment
    - Admin can assign existing clients to a reseller
    - Ownership guards (resellers cannot touch others' clients)
    - Disabled reseller login rejection

===============================================================================

EOF
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------
main() {
    require_root
    detect_os

    log "OS detected: ${OS_ID} ${OS_VERSION}"

    install_dependencies
    clone_repo
    build_backend
    build_frontend
    create_service
    show_summary
}

main "$@"
