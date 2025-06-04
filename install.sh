#!/bin/bash

# Color definitions
red='[0;31m'
green='[0;32m'
blue='[0;34m'
yellow='[0;33m'
plain='[0m'

# Global variables
ARCH=$(uname -m)
OS_RELEASE_ID=""
OS_RELEASE_VERSION_ID=""
INSTALL_DIR="/opt/3x-ui-docker"
REPO_DIR_NAME="3x-ui-source"
REPO_URL="https://github.com/MHSanaei/3x-ui.git"

# --- Utility Functions ---
detect_os() {
    if [[ -f /etc/os-release ]]; then
        source /etc/os-release
        OS_RELEASE_ID=$ID
        OS_RELEASE_VERSION_ID=$VERSION_ID
    elif [[ -f /usr/lib/os-release ]]; then
        source /usr/lib/os-release
        OS_RELEASE_ID=$ID
        OS_RELEASE_VERSION_ID=$VERSION_ID
    else
        echo -e "${red}Failed to detect the operating system!${plain}" >&2
        exit 1
    fi
    echo -e "${blue}Detected OS: $OS_RELEASE_ID $OS_RELEASE_VERSION_ID${plain}"
}

check_root() {
    [[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: Please run this script with root privilege.${plain}" && exit 1
}

print_colored() {
    local color="$1"
    local message="$2"
    echo -e "${color}${message}${plain}"
}

check_command() {
    command -v "$1" >/dev/null 2>&1
}

# --- Installation Functions ---
install_dependencies() {
    print_colored "$blue" "Installing essential dependencies (curl, tar, git)..."
    local pkgs_to_install=""
    if ! check_command curl; then pkgs_to_install+="curl "; fi
    if ! check_command tar; then pkgs_to_install+="tar "; fi
    if ! check_command git; then pkgs_to_install+="git "; fi

    if [[ -n "$pkgs_to_install" ]]; then
        case "$OS_RELEASE_ID" in
        ubuntu | debian | armbian)
            apt-get update -y && apt-get install -y $pkgs_to_install
            ;;
        centos | almalinux | rocky | ol)
            yum install -y $pkgs_to_install
            ;;
        fedora)
            dnf install -y $pkgs_to_install
            ;;
        arch | manjaro)
            pacman -Syu --noconfirm $pkgs_to_install
            ;;
        *)
            print_colored "$yellow" "Unsupported OS for automatic dependency installation: $OS_RELEASE_ID. Please install curl, tar, and git manually."
            ;;
        esac
        if ! check_command curl || ! check_command tar || ! check_command git; then
             print_colored "$red" "Failed to install essential dependencies. Please install them manually and re-run."
             exit 1
        fi
    else
        print_colored "$green" "Essential dependencies are already installed."
    fi
}

install_docker() {
    print_colored "$blue" "Checking for Docker..."
    if check_command docker; then
        print_colored "$green" "Docker is already installed: $(docker --version)"
        return 0
    fi

    print_colored "$yellow" "Docker not found. Attempting to install Docker..."
    if [[ "$OS_RELEASE_ID" == "ubuntu" || "$OS_RELEASE_ID" == "debian" || "$OS_RELEASE_ID" == "armbian" ]]; then
        print_colored "$blue" "Attempting Docker installation for Debian-based system..."
        apt-get update -y
        apt-get install -y apt-transport-https ca-certificates software-properties-common gnupg lsb-release
        mkdir -p /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/${OS_RELEASE_ID}/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        echo           "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/${OS_RELEASE_ID}           $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
        apt-get update -y
        apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin || {
            print_colored "$yellow" "Docker installation via apt failed. Trying convenience script..."
            curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh
            rm -f get-docker.sh
        }
    elif [[ "$OS_RELEASE_ID" == "centos" || "$OS_RELEASE_ID" == "almalinux" || "$OS_RELEASE_ID" == "rocky" || "$OS_RELEASE_ID" == "ol" ]]; then
        print_colored "$blue" "Attempting Docker installation for RHEL-based system..."
        yum install -y yum-utils
        yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
        yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin || {
            print_colored "$yellow" "Docker installation via yum failed. Trying convenience script..."
            curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh
            rm -f get-docker.sh
        }
    elif [[ "$OS_RELEASE_ID" == "fedora" ]]; then
         print_colored "$blue" "Attempting Docker installation for Fedora..."
         dnf -y install dnf-plugins-core
         dnf config-manager --add-repo https://download.docker.com/linux/fedora/docker-ce.repo
         dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin || {
            print_colored "$yellow" "Docker installation via dnf failed. Trying convenience script..."
            curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh
            rm -f get-docker.sh
         }
    else
        print_colored "$yellow" "Unsupported OS for automatic Docker installation via package manager. Trying convenience script..."
        curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh
        rm -f get-docker.sh
    fi

    if ! check_command docker; then
        print_colored "$red" "Docker installation failed. Please install Docker manually and re-run this script."
        exit 1
    fi

    if ! systemctl is-active --quiet docker; then
      systemctl start docker || print_colored "$yellow" "Failed to start Docker service via systemctl. It might need manual start."
    fi
    systemctl enable docker || print_colored "$yellow" "Failed to enable Docker service via systemctl."
    print_colored "$green" "Docker installed and status confirmed: $(docker --version)"
}

install_docker_compose_plugin() {
    print_colored "$blue" "Checking for Docker Compose plugin (v2)..."
    if docker compose version &> /dev/null; then
        print_colored "$green" "Docker Compose plugin is already installed: $(docker compose version)"
        return 0
    fi

    print_colored "$yellow" "Docker Compose plugin not found."
    print_colored "$blue" "Attempting to install/update docker-compose-plugin..."

    if [[ -x "$(command -v apt-get)" ]]; then
        apt-get install -y docker-compose-plugin || print_colored "$yellow" "apt: docker-compose-plugin not found or error. Trying legacy."
    elif [[ -x "$(command -v yum)" ]]; then
        yum install -y docker-compose-plugin || print_colored "$yellow" "yum: docker-compose-plugin not found or error. Trying legacy."
    elif [[ -x "$(command -v dnf)" ]]; then
        dnf install -y docker-compose-plugin || print_colored "$yellow" "dnf: docker-compose-plugin not found or error. Trying legacy."
    else
        print_colored "$yellow" "No known package manager for docker-compose-plugin. Will try legacy."
    fi

    if docker compose version &> /dev/null; then
        print_colored "$green" "Docker Compose plugin is now available: $(docker compose version)"
        return 0
    fi

    print_colored "$yellow" "Docker Compose plugin (v2) still not found. Attempting to install legacy docker-compose (v1)..."
    if command -v docker-compose &>/dev/null; then
        print_colored "$green" "Legacy docker-compose is already installed: $(docker-compose --version)"
        return 0
    fi

    print_colored "$blue" "Downloading latest legacy docker-compose..."
    LATEST_COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep '"tag_name":' | cut -d'"' -f4)

    if [ -z "$LATEST_COMPOSE_VERSION" ]; then
        print_colored "$red" "Failed to fetch latest docker-compose version from GitHub API. Please install it manually."
        exit 1
    fi
    print_colored "$blue" "Latest legacy docker-compose version: $LATEST_COMPOSE_VERSION"

    COMPOSE_URL="https://github.com/docker/compose/releases/download/$LATEST_COMPOSE_VERSION/docker-compose-$(uname -s)-$(uname -m)"

    INSTALL_PATH="/usr/local/bin/docker-compose"
    print_colored "$blue" "Installing docker-compose to $INSTALL_PATH..."

    curl -SL "$COMPOSE_URL" -o "$INSTALL_PATH"
    if [ $? -ne 0 ]; then
        print_colored "$red" "Download failed from $COMPOSE_URL. Please check the URL or install manually."
        INSTALL_PATH_ALT="/usr/bin/docker-compose"
        print_colored "$blue" "Attempting alternative installation to $INSTALL_PATH_ALT..."
        curl -SL "$COMPOSE_URL" -o "$INSTALL_PATH_ALT"
         if [ $? -ne 0 ]; then
            print_colored "$red" "Download to $INSTALL_PATH_ALT also failed. Please install docker-compose manually."
            exit 1
         fi
         chmod +x "$INSTALL_PATH_ALT"
    else
        chmod +x "$INSTALL_PATH"
    fi

    if command -v docker-compose &>/dev/null; then
        print_colored "$green" "Legacy docker-compose installed successfully: $(docker-compose --version)"
    else
        print_colored "$red" "Installation of legacy docker-compose failed or it's not in PATH. Please install it manually."
        exit 1
    fi
}

# --- Main Installation Logic ---
main() {
    check_root
    detect_os

    print_colored "$blue" "Starting 3X-UI New Frontend Docker-based Installation..."

    install_dependencies
    install_docker # Corrected function name
    install_docker_compose_plugin # Corrected function name

    local user_install_dir # Made variables local where appropriate
    read -rp "Enter installation directory (default: $INSTALL_DIR): " user_install_dir
    INSTALL_DIR=${user_install_dir:-$INSTALL_DIR}

    print_colored "$blue" "Panel will be installed in: $INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"

    local REPO_PATH="$INSTALL_DIR/$REPO_DIR_NAME"
    if [ -d "$REPO_PATH" ] && [ -d "$REPO_PATH/.git" ]; then
        print_colored "$yellow" "Existing repository found at $REPO_PATH. Attempting to update..."
        cd "$REPO_PATH" || { print_colored "$red" "Failed to cd to $REPO_PATH"; exit 1; }
        git fetch --all
        print_colored "$yellow" "Checking out and pulling latest 'main' branch..."
        current_branch=$(git rev-parse --abbrev-ref HEAD)
        if [ "\$current_branch" != "main" ] && [ "\$current_branch" != "master" ]; then
            if git show-ref --verify --quiet refs/heads/main; then
                git checkout main
            elif git show-ref --verify --quiet refs/heads/master; then
                git checkout master
            else
                print_colored "$red" "Could not find main or master branch to checkout."
                # exit 1 # Or proceed with current branch
            fi
        fi
        git reset --hard origin/$(git rev-parse --abbrev-ref HEAD)
        git pull
        cd "$INSTALL_DIR"
    else
        print_colored "$blue" "Cloning repository from $REPO_URL into $REPO_PATH..."
        rm -rf "$REPO_PATH"
        git clone --depth 1 "$REPO_URL" "$REPO_PATH" || { print_colored "$red" "Failed to clone repository."; exit 1; }
    fi

    if [ ! -d "$REPO_PATH" ]; then
        print_colored "$red" "Failed to clone or find repository at $REPO_PATH. Aborting."
        exit 1
    fi
    cd "$REPO_PATH"

    if [ ! -f "docker-compose.yml" ] || [ ! -f "new-frontend/Dockerfile" ] || [ ! -f "Dockerfile.backend" ]; then
        print_colored "$red" "Essential Docker configuration files not found in the repository ($REPO_PATH). Aborting."
        exit 1
    fi

    print_colored "$blue" "Creating data directories (db, cert) if they don't exist in $REPO_PATH..."
    mkdir -p db
    mkdir -p cert

    local DEFAULT_FRONTEND_PORT=3000
    local DEFAULT_BACKEND_PANEL_PORT=2053
    local HOST_FRONTEND_PORT
    local HOST_BACKEND_PANEL_PORT
    local INTERNAL_BACKEND_PORT=2053
    local USER_API_URL
    local NEXT_PUBLIC_API_BASE_URL

    print_colored "$yellow" "Configuring .env file in $REPO_PATH..."
    read -rp "Enter HOST port for Frontend (default: $DEFAULT_FRONTEND_PORT): " HOST_FRONTEND_PORT
    HOST_FRONTEND_PORT=\${HOST_FRONTEND_PORT:-$DEFAULT_FRONTEND_PORT}

    read -rp "Enter HOST port for Backend Panel (default: $DEFAULT_BACKEND_PANEL_PORT): " HOST_BACKEND_PANEL_PORT
    HOST_BACKEND_PANEL_PORT=\${HOST_BACKEND_PANEL_PORT:-$DEFAULT_BACKEND_PANEL_PORT}

    DEFAULT_API_URL="http://backend:$INTERNAL_BACKEND_PORT"
    read -rp "Enter API Base URL for Frontend (default: $DEFAULT_API_URL, press Enter to use default): " USER_API_URL
    NEXT_PUBLIC_API_BASE_URL=\${USER_API_URL:-$DEFAULT_API_URL}

    cat << EOF_ENV > .env
# .env for 3x-ui docker-compose
FRONTEND_HOST_PORT=$HOST_FRONTEND_PORT
BACKEND_HOST_PORT=$HOST_BACKEND_PANEL_PORT
BACKEND_INTERNAL_PORT=$INTERNAL_BACKEND_PORT
NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL
XRAY_VMESS_AEAD_FORCED=false
XUI_ENABLE_FAIL2BAN=true
# XUI_USERNAME=admin
# XUI_PASSWORD=admin
EOF_ENV
    print_colored "$green" ".env file configured in $REPO_PATH."
    print_colored "$yellow" "Note: Frontend will be accessible on host at port $HOST_FRONTEND_PORT."
    print_colored "$yellow" "Backend panel (API) will be accessible on host at port $HOST_BACKEND_PANEL_PORT."

    print_colored "$blue" "Building and starting services with Docker Compose (from $REPO_PATH)..."
    print_colored "$yellow" "This may take a few minutes for the first build..."
    if docker compose up -d --build --remove-orphans; then
        print_colored "$green" "3X-UI services (new frontend & backend) started successfully!"
        print_colored "$green" "Frontend should be accessible at: http://<your_server_ip>:$HOST_FRONTEND_PORT"
        print_colored "$yellow" "Please allow a moment for services to initialize fully."
        print_colored "$blue" "To manage services, navigate to '$REPO_PATH' and use 'docker compose' commands (e.g., docker compose logs -f, docker compose stop)."
    else
        print_colored "$red" "Failed to start services with Docker Compose. Please check logs above and run 'docker compose logs' in '$REPO_PATH' for details."
        exit 1
    fi

    print_colored "$green" "Installation finished."
}

# --- Script Execution ---
main "$@"

exit 0
