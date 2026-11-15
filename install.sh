#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}致命错误：${plain} 请使用 root 权限运行此脚本\n " && exit 1

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "无法检测系统类型，请联系作者！" >&2
    exit 1
fi
echo "系统发行版是：$release"

arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${green}不支持的 CPU 架构！${plain}" && rm -f "$(realpath "$0")" && exit 1 ;;
    esac
}

echo "架构：$(arch)"

# Non-interactive mode: triggered explicitly via XUI_NONINTERACTIVE=1, or
# implicitly when stdin is not a TTY (e.g. `curl ... | bash`, cloud-init).
# In this mode every prompt below is replaced by an env var or a sane default.
if [[ "${XUI_NONINTERACTIVE:-0}" == "1" ]] || [[ ! -t 0 ]]; then
    NONINTERACTIVE=1
else
    NONINTERACTIVE=0
fi
export NONINTERACTIVE

# Simple helpers
is_ipv4() {
    [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] && return 0 || return 1
}
is_ipv6() {
    [[ "$1" =~ : ]] && return 0 || return 1
}
is_ip() {
    is_ipv4 "$1" || is_ipv6 "$1"
}
is_domain() {
    [[ "$1" =~ ^([A-Za-z0-9](-*[A-Za-z0-9])*\.)+(xn--[a-z0-9]{2,}|[A-Za-z]{2,})$ ]] && return 0 || return 1
}

# acme.sh's standalone server binds IPv4 by default; --listen-v6 makes it
# v6-only, which breaks HTTP-01 validation when the domain's A record points
# at this host's IPv4 (#4994). Only force IPv6 when the host has no global
# IPv4 address at all.
acme_listen_flag() {
    if ip -4 addr show scope global 2> /dev/null | grep -q "inet "; then
        echo ""
    else
        echo "--listen-v6"
    fi
}

# Port helpers
is_port_in_use() {
    local port="$1"
    if command -v ss > /dev/null 2>&1; then
        ss -ltn 2> /dev/null | awk -v p=":${port}$" '$4 ~ p {found = 1} END {exit !found}'
        return
    fi
    if command -v netstat > /dev/null 2>&1; then
        netstat -lnt 2> /dev/null | awk -v p=":${port}$" '$4 ~ p {found = 1} END {exit !found}'
        return
    fi
    if command -v lsof > /dev/null 2>&1; then
        lsof -nP -iTCP:${port} -sTCP:LISTEN > /dev/null 2>&1 && return 0
    fi
    return 1
}

normalize_panel_port() {
    local port="$1"
    [[ "$port" =~ ^[0-9]+$ ]] || return 1
    # Strip leading zeros before bounding the length and doing arithmetic:
    # Bash and the Go flag parser otherwise interpret leading zeros as octal.
    port="${port#"${port%%[!0]*}"}"
    [[ -n "$port" && ${#port} -le 5 ]] || return 1
    ((10#$port <= 65535)) || return 1
    printf '%s\n' "$port"
}

# Print only the selected decimal port to stdout; prompts/notices use stderr.
select_panel_port() {
    local confirm=n port attempt
    if [[ "$NONINTERACTIVE" == 1 ]]; then
        [[ -z "${XUI_PANEL_PORT+x}" ]] || confirm=y
    else
        while true; do
            if ! IFS= read -rp "是否要自定义面板端口设置？（否则将使用随机端口）[y/N]：" confirm; then
                echo '无法读取面板端口确认，正在中止。' >&2
                return 1
            fi
            confirm="${confirm#"${confirm%%[![:space:]]*}"}"
            confirm="${confirm%"${confirm##*[![:space:]]}"}"
            case "${confirm,,}" in
                y | yes) break ;;
                n | no | '') confirm=n; break ;;
                *) echo '请回答 yes 或 no（直接回车默认为 No）。' >&2 ;;
            esac
        done
    fi

    if [[ "$confirm" == n ]]; then
        for ((attempt = 0; attempt < 100; attempt++)); do
            if ! port=$(shuf -i 1024-62000 -n 1); then
                echo '生成随机面板端口失败。' >&2
                return 1
            fi
            if port=$(normalize_panel_port "$port") && ! is_port_in_use "$port"; then
                echo "已生成随机端口：$port" >&2
                printf '%s\n' "$port"
                return 0
            fi
        done
        echo '找不到可用的随机面板端口，正在中止。' >&2
        return 1
    fi

    while true; do
        if [[ "$NONINTERACTIVE" == 1 ]]; then
            port="$XUI_PANEL_PORT"
        elif ! read -rp '请设置面板端口：' port; then
            echo '无法读取自定义面板端口，正在中止。' >&2
            return 1
        fi
        if ! port=$(normalize_panel_port "$port"); then
            echo '端口无效，请输入 1 到 65535 之间的数字。' >&2
        elif is_port_in_use "$port"; then
            echo "端口 $port 已被占用，请选择其他面板端口。" >&2
        else
            echo "你的面板端口是：$port" >&2
            printf '%s\n' "$port"
            return 0
        fi
        [[ "$NONINTERACTIVE" != 1 ]] || return 1
    done
}

install_base() {
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf makecache -y && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum makecache -y && yum install -y cronie curl tar tzdata socat ca-certificates openssl
            else
                dnf makecache -y && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl
            fi
            ;;
        arch | manjaro | parch)
            pacman -Sy --noconfirm cronie curl tar tzdata socat ca-certificates openssl
            ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper refresh && zypper -q install -y cron curl tar timezone socat ca-certificates openssl
            ;;
        alpine)
            apk update && apk add dcron curl tar tzdata socat ca-certificates openssl
            ;;
        *)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl
            ;;
    esac
}

gen_random_string() {
    local length="$1"
    openssl rand -base64 $((length * 2)) \
        | tr -dc 'a-zA-Z0-9' \
        | head -c "$length"
}

# prompt_or_default VARNAME "prompt text" "default" [ENV_NAME]
# Interactive: read into VARNAME. Non-interactive: VARNAME = ${ENV_NAME:-default}.
# ENV_NAME defaults to VARNAME when omitted. Keeps every interactive prompt
# string byte-for-byte identical to the original `read -rp`.
prompt_or_default() {
    local __var="$1" __prompt="$2" __default="$3" __env="${4:-$1}"
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        printf -v "$__var" '%s' "${!__env:-$__default}"
    else
        # shellcheck disable=SC2229
        read -rp "$__prompt" "$__var"
    fi
}

# write_install_result <user> <pass> <port> <webpath> <scheme> <host> <token> <dbtype>
# Persists a parseable, root-only credentials file consumed by cloud-init/MOTD.
# Values are written with printf '%q' so a pinned password/username containing
# spaces, quotes, $(...) or backticks is shell-escaped and the file stays safely
# source-able (consumers do '. install-result.env'). For the alphanumeric random
# values gen_random_string emits, %q is a no-op. This is a DIFFERENT file from the
# Postgres env file (/etc/default/x-ui).
write_install_result() {
    local u="$1" p="$2" port="$3" wbp="$4" scheme="$5" host="$6" token="$7" dbtype="$8"
    local result_file="/etc/x-ui/install-result.env"
    local url_host="${host:-SERVER_IP_UNKNOWN}"
    install -d -m 700 /etc/x-ui 2> /dev/null
    local prev_umask
    prev_umask=$(umask)
    umask 077
    if ! {
        printf 'XUI_USERNAME=%q\n' "$u"
        printf 'XUI_PASSWORD=%q\n' "$p"
        printf 'XUI_PANEL_PORT=%q\n' "$port"
        printf 'XUI_WEB_BASE_PATH=%q\n' "$wbp"
        printf 'XUI_ACCESS_URL=%q\n' "${scheme}://${url_host}:${port}/${wbp}"
        printf 'XUI_API_TOKEN=%q\n' "$token"
        printf 'XUI_DB_TYPE=%q\n' "$dbtype"
    } > "$result_file"; then
        umask "$prev_umask"
        echo -e "${yellow}警告：写入 ${result_file} 失败。${plain}" >&2
        return 1
    fi
    umask "$prev_umask"
    chmod 600 "$result_file" 2> /dev/null
    chown root:root "$result_file" 2> /dev/null || true
    echo -e "${green}安装结果已写入 ${result_file}（权限 600）。${plain}"
}

# RHEL-family initdb writes pg_hba.conf host rules with ident auth, which
# compares the OS username against the Postgres role and always rejects the
# randomly generated panel role over TCP (#5806). Prepend password-auth rules
# scoped to the panel database; first match wins, and md5 also accepts
# scram-sha-256-stored verifiers, so this works on every supported distro.
pg_ensure_hba_password_auth() {
    local pg_db="$1"
    local hba_file
    hba_file=$(sudo -u postgres psql -tAc 'SHOW hba_file' 2> /dev/null | tr -d '[:space:]')
    [[ -n "${hba_file}" && -f "${hba_file}" ]] || return 0
    grep -Eq "^host[[:space:]]+${pg_db}[[:space:]]" "${hba_file}" && return 0
    local tmp
    tmp=$(mktemp) || return 1
    {
        echo "# Added by 3x-ui: allow password logins for the panel database."
        echo "host    ${pg_db}    all    127.0.0.1/32    md5"
        echo "host    ${pg_db}    all    ::1/128         md5"
        cat "${hba_file}"
    } > "${tmp}" || {
        rm -f "${tmp}"
        return 1
    }
    cat "${tmp}" > "${hba_file}" || {
        rm -f "${tmp}"
        return 1
    }
    rm -f "${tmp}"
    sudo -u postgres psql -tAc 'SELECT pg_reload_conf()' > /dev/null 2>&1 || true
}

# New installs get this PostgreSQL major from the official PGDG repository; distros or
# CPU architectures PGDG does not build for fall back to the distribution's own package.
PG_TARGET_MAJOR=18

pg_has_debian_cluster() {
    command -v pg_lsclusters > /dev/null 2>&1 && [[ -n "$(pg_lsclusters -h 2> /dev/null)" ]]
}

# Installs postgresql-${PG_TARGET_MAJOR} from apt.postgresql.org. On failure it removes the
# repository it added, so an unsupported release cannot break later `apt-get update` runs.
pg_apt_install_pgdg_server() {
    local list added_lists=()
    for list in /etc/apt/sources.list.d/pgdg.list /etc/apt/sources.list.d/pgdg.sources; do
        [[ -e "${list}" ]] || added_lists+=("${list}")
    done
    if apt-get install -y -q postgresql-common ca-certificates gnupg >&2 \
        && /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y >&2 \
        && apt-get install -y -q "postgresql-${PG_TARGET_MAJOR}" >&2; then
        return 0
    fi
    for list in "${added_lists[@]}"; do
        rm -f "${list}"
    done
    apt-get update >&2 || true
    echo -e "${yellow}本系统没有 PostgreSQL ${PG_TARGET_MAJOR}，将改用发行版自带的 PostgreSQL。${plain}" >&2
    return 1
}

# Installs postgresql${PG_TARGET_MAJOR}-server from the PGDG yum repository (EL 8+ only); on
# failure it undoes the repository and module changes it made. The caller runs initdb.
pg_el_install_pgdg_server() {
    local elver
    elver=$(rpm -E %rhel 2> /dev/null)
    [[ "${elver}" =~ ^[0-9]+$ && "${elver}" -ge 8 ]] && command -v dnf > /dev/null 2>&1 || return 1
    [[ -f /var/lib/pgsql/data/PG_VERSION ]] && return 1
    local added_repo=false
    if ! rpm -q pgdg-redhat-repo > /dev/null 2>&1; then
        if dnf install -y -q "https://download.postgresql.org/pub/repos/yum/reporpms/EL-${elver}-$(uname -m)/pgdg-redhat-repo-latest.noarch.rpm" >&2; then
            added_repo=true
            dnf -qy module disable postgresql > /dev/null 2>&1 || true
        fi
    fi
    if rpm -q pgdg-redhat-repo > /dev/null 2>&1 \
        && dnf install -y -q "postgresql${PG_TARGET_MAJOR}-server" "postgresql${PG_TARGET_MAJOR}-contrib" >&2; then
        return 0
    fi
    if [[ "${added_repo}" == "true" ]]; then
        dnf -qy module reset postgresql > /dev/null 2>&1 || true
        dnf remove -y -q pgdg-redhat-repo >&2 || true
    fi
    echo -e "${yellow}本系统没有 PostgreSQL ${PG_TARGET_MAJOR}，将改用发行版自带的 PostgreSQL。${plain}" >&2
    return 1
}

install_postgres_local() {
    local pg_user pg_pass
    pg_pass=$(gen_random_string 24)
    local pg_db="xui"
    local pg_host="127.0.0.1"
    local pg_port="5432"

    local pg_service="postgresql"
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update >&2 || return 1
            if pg_has_debian_cluster || ! pg_apt_install_pgdg_server; then
                apt-get install -y -q postgresql >&2 || return 1
            fi
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol | centos)
            if [[ "${release}" != "fedora" && "${release}" != "amzn" ]] && pg_el_install_pgdg_server; then
                pg_service="postgresql-${PG_TARGET_MAJOR}"
                [[ -f "/var/lib/pgsql/${PG_TARGET_MAJOR}/data/PG_VERSION" ]] \
                    || "/usr/pgsql-${PG_TARGET_MAJOR}/bin/postgresql-${PG_TARGET_MAJOR}-setup" initdb >&2 || return 1
            else
                if [[ "${release}" == "centos" && "${VERSION_ID}" =~ ^7 ]]; then
                    yum install -y postgresql-server postgresql-contrib >&2 || return 1
                else
                    dnf install -y -q postgresql-server postgresql-contrib >&2 || return 1
                fi
                [[ -d /var/lib/pgsql/data && -f /var/lib/pgsql/data/PG_VERSION ]] || postgresql-setup --initdb >&2 || return 1
            fi
            ;;
        arch | manjaro | parch)
            pacman -Sy --noconfirm postgresql >&2 || return 1
            if [[ ! -f /var/lib/postgres/data/PG_VERSION ]]; then
                sudo -u postgres initdb -D /var/lib/postgres/data >&2 || return 1
            fi
            ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper -q install -y postgresql-server postgresql-contrib >&2 || return 1
            if [[ ! -f /var/lib/pgsql/data/PG_VERSION ]]; then
                install -d -o postgres -g postgres -m 700 /var/lib/pgsql/data >&2 || return 1
                su - postgres -c "initdb -D /var/lib/pgsql/data" >&2 || return 1
            fi
            ;;
        alpine)
            apk add --no-cache postgresql postgresql-contrib >&2 || return 1
            if [[ ! -f /var/lib/postgresql/data/PG_VERSION ]]; then
                /etc/init.d/postgresql setup >&2 || return 1
            fi
            rc-update add postgresql default >&2 2> /dev/null || true
            rc-service postgresql start >&2 || return 1
            ;;
        *)
            echo -e "${red}不支持在此发行版上自动安装 PostgreSQL：${release}${plain}" >&2
            return 1
            ;;
    esac

    if [[ "${release}" != "alpine" ]]; then
        systemctl enable --now "${pg_service}" >&2 || return 1
    fi

    # Wait briefly for the server to accept connections.
    local i
    for i in 1 2 3 4 5; do
        sudo -u postgres psql -tAc 'SELECT 1' > /dev/null 2>&1 && break
        sleep 1
    done

    local existing_owner=""
    existing_owner=$(sudo -u postgres psql -tAc \
        "SELECT pg_catalog.pg_get_userbyid(datdba) FROM pg_database WHERE datname='${pg_db}'" 2> /dev/null \
        | tr -d '[:space:]')
    if [[ -n "${existing_owner}" && "${existing_owner}" != "postgres" ]]; then
        pg_user="${existing_owner}"
    else
        pg_user=$(gen_random_string 8)
    fi

    # Idempotent role/db creation. Identifiers are double-quoted because a
    # random username may start with a digit, which Postgres rejects unquoted.
    sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${pg_user}'" 2> /dev/null \
        | grep -q 1 \
        || sudo -u postgres psql -c "CREATE USER \"${pg_user}\" WITH PASSWORD '${pg_pass}';" >&2 || return 1

    sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${pg_db}'" 2> /dev/null \
        | grep -q 1 \
        || sudo -u postgres psql -c "CREATE DATABASE \"${pg_db}\" OWNER \"${pg_user}\";" >&2 || return 1

    sudo -u postgres psql -c "ALTER USER \"${pg_user}\" WITH PASSWORD '${pg_pass}';" >&2 || return 1

    pg_ensure_hba_password_auth "${pg_db}" \
        || echo -e "${yellow}警告：无法更新 pg_hba.conf，PostgreSQL 可能会拒绝面板的 TCP 登录（ident 认证）。${plain}" >&2

    local pg_pass_enc
    pg_pass_enc=$(printf '%s' "${pg_pass}" | sed -e 's/%/%25/g' -e 's/:/%3A/g' -e 's/@/%40/g' -e 's|/|%2F|g' -e 's/?/%3F/g' -e 's/#/%23/g')

    if [[ -n "${PG_CRED_FILE:-}" ]]; then
        local prev_umask
        prev_umask=$(umask)
        umask 077
        if ! cat > "${PG_CRED_FILE}" << EOF; then
PG_USER=${pg_user}
PG_PASS=${pg_pass}
PG_HOST=${pg_host}
PG_PORT=${pg_port}
PG_DB=${pg_db}
EOF
            umask "${prev_umask}"
            echo -e "${red}写入 PostgreSQL 凭证到 ${PG_CRED_FILE} 失败${plain}" >&2
            return 1
        fi
        umask "${prev_umask}"
    fi

    echo "postgres://${pg_user}:${pg_pass_enc}@${pg_host}:${pg_port}/${pg_db}?sslmode=disable"
    return 0
}

ensure_pg_client() {
    if command -v pg_dump > /dev/null 2>&1 && command -v pg_restore > /dev/null 2>&1; then
        return 0
    fi
    echo -e "${yellow}正在安装 PostgreSQL 客户端工具（pg_dump/pg_restore），用于面板内备份……${plain}" >&2
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update >&2 && apt-get install -y -q postgresql-client >&2 || return 1
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf install -y -q postgresql >&2 || return 1
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum install -y postgresql >&2 || return 1
            else
                dnf install -y -q postgresql >&2 || return 1
            fi
            ;;
        arch | manjaro | parch)
            pacman -Sy --noconfirm postgresql >&2 || return 1
            ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper -q install -y postgresql >&2 || return 1
            ;;
        alpine)
            apk add --no-cache postgresql-client >&2 || return 1
            ;;
        *)
            return 1
            ;;
    esac
    command -v pg_dump > /dev/null 2>&1 && command -v pg_restore > /dev/null 2>&1
}

install_acme() {
    echo -e "${green}正在安装 acme.sh（用于 SSL 证书管理）……${plain}"
    cd ~ || return 1
    curl -s https://get.acme.sh | sh > /dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${red}安装 acme.sh 失败${plain}"
        return 1
    else
        echo -e "${green}acme.sh 安装成功${plain}"
    fi
    return 0
}

setup_ssl_certificate() {
    local domain="$1"
    local server_ip="$2"
    local existing_port="$3"
    local existing_webBasePath="$4"

    echo -e "${green}正在配置 SSL 证书……${plain}"

    # Check if acme.sh is installed
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${yellow}安装 acme.sh 失败，跳过 SSL 配置${plain}"
            return 1
        fi
    fi

    # Create certificate directory
    local certPath="/root/cert/${domain}"
    mkdir -p "$certPath"

    # Issue certificate
    echo -e "${green}正在为 ${domain} 申请 SSL 证书……${plain}"
    echo -e "${yellow}注意：80 端口必须开放且能从互联网访问${plain}"

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1
    ~/.acme.sh/acme.sh --issue -d ${domain} $(acme_listen_flag) --standalone --httpport 80 --force

    if [ $? -ne 0 ]; then
        echo -e "${yellow}为 ${domain} 申请证书失败${plain}"
        echo -e "${yellow}请确保 80 端口已开放，稍后用 x-ui 命令重试${plain}"
        rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc 2> /dev/null
        rm -rf "$certPath" 2> /dev/null
        return 1
    fi

    # Install certificate
    ~/.acme.sh/acme.sh --installcert --force -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem \
        --reloadcmd "systemctl restart x-ui" > /dev/null 2>&1

    if [ $? -ne 0 ]; then
        echo -e "${yellow}安装证书失败${plain}"
        return 1
    fi

    # Enable auto-renew
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1
    # Secure permissions: private key readable only by owner
    chmod 600 $certPath/privkey.pem 2> /dev/null
    chmod 644 $certPath/fullchain.pem 2> /dev/null

    # Set certificate for panel
    local webCertFile="/root/cert/${domain}/fullchain.pem"
    local webKeyFile="/root/cert/${domain}/privkey.pem"

    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
        ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile" > /dev/null 2>&1
        echo -e "${green}SSL 证书安装配置成功！${plain}"
        return 0
    else
        echo -e "${yellow}未找到证书文件${plain}"
        return 1
    fi
}

# Issue Let's Encrypt IP certificate with shortlived profile (~6 days validity)
# Requires acme.sh and port 80 open for HTTP-01 challenge
setup_ip_certificate() {
    local ipv4="$1"
    local ipv6="$2" # optional

    echo -e "${green}正在配置 Let's Encrypt IP 证书（shortlived 有效期）……${plain}"
    echo -e "${yellow}注意：IP 证书有效期约 6 天，会自动续期。${plain}"
    echo -e "${yellow}默认监听 80 端口。如果选择其他端口，请确保外部 80 端口转发到该端口。${plain}"

    # Check for acme.sh
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${red}安装 acme.sh 失败${plain}"
            return 1
        fi
    fi

    # Validate IP address
    if [[ -z "$ipv4" ]]; then
        echo -e "${red}需要 IPv4 地址${plain}"
        return 1
    fi

    if ! is_ipv4 "$ipv4"; then
        echo -e "${red}IPv4 地址无效：$ipv4${plain}"
        return 1
    fi

    # Create certificate directory
    local certDir="/root/cert/ip"
    mkdir -p "$certDir"

    # Build domain arguments
    local domain_args="-d ${ipv4}"
    if [[ -n "$ipv6" ]] && is_ipv6 "$ipv6"; then
        domain_args="${domain_args} -d ${ipv6}"
        echo -e "${green}包含 IPv6 地址：${ipv6}${plain}"
    fi

    # Set reload command for auto-renewal (add || true so it doesn't fail during first install)
    local reloadCmd="systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null || true"

    # Choose port for HTTP-01 listener (default 80, prompt override)
    local WebPort=""
    prompt_or_default WebPort "ACME HTTP-01 监听使用的端口（默认 80）：" "80" XUI_ACME_HTTP_PORT
    WebPort="${WebPort:-80}"
    if ! [[ "${WebPort}" =~ ^[0-9]+$ ]] || ((WebPort < 1 || WebPort > 65535)); then
        echo -e "${red}提供的端口无效，改用 80 端口。${plain}"
        WebPort=80
    fi
    echo -e "${green}使用端口 ${WebPort} 进行 standalone 验证。${plain}"
    if [[ "${WebPort}" -ne 80 ]]; then
        echo -e "${yellow}提醒：Let's Encrypt 仍通过 80 端口连接，请将外部 80 端口转发到 ${WebPort}。${plain}"
    fi

    # Ensure chosen port is available
    while true; do
        if is_port_in_use "${WebPort}"; then
            echo -e "${yellow}端口 ${WebPort} 已被占用。${plain}"

            local alt_port=""
            if [[ "$NONINTERACTIVE" == "1" ]]; then
                echo -e "${red}端口 ${WebPort} 被占用，非交互模式下无法继续。${plain}"
                return 1
            fi
            read -rp "请为 acme.sh standalone 监听输入另一个端口（留空则中止）：" alt_port
            alt_port="${alt_port// /}"
            if [[ -z "${alt_port}" ]]; then
                echo -e "${red}端口 ${WebPort} 被占用，无法继续。${plain}"
                return 1
            fi
            if ! [[ "${alt_port}" =~ ^[0-9]+$ ]] || ((alt_port < 1 || alt_port > 65535)); then
                echo -e "${red}提供的端口无效。${plain}"
                return 1
            fi
            WebPort="${alt_port}"
            continue
        else
            echo -e "${green}端口 ${WebPort} 空闲，可用于 standalone 验证。${plain}"
            break
        fi
    done

    # Issue certificate with shortlived profile
    echo -e "${green}正在为 ${ipv4} 申请 IP 证书……${plain}"
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1
    [[ -n "${XUI_ACME_EMAIL:-}" ]] && ~/.acme.sh/acme.sh --register-account -m "${XUI_ACME_EMAIL}" > /dev/null 2>&1

    ~/.acme.sh/acme.sh --issue \
        ${domain_args} \
        --standalone \
        --server letsencrypt \
        --certificate-profile shortlived \
        --days 6 \
        --httpport ${WebPort} \
        --force

    if [ $? -ne 0 ]; then
        echo -e "${red}申请 IP 证书失败${plain}"
        echo -e "${yellow}请确保端口 ${WebPort} 可达（或已从外部 80 端口转发）${plain}"
        # Cleanup acme.sh data for both IPv4 and IPv6 if specified
        rm -rf ~/.acme.sh/${ipv4} ~/.acme.sh/${ipv4}_ecc 2> /dev/null
        [[ -n "$ipv6" ]] && rm -rf ~/.acme.sh/${ipv6} ~/.acme.sh/${ipv6}_ecc 2> /dev/null
        rm -rf ${certDir} 2> /dev/null
        return 1
    fi

    echo -e "${green}证书申请成功，正在安装……${plain}"

    # Install certificate
    # Note: acme.sh may report "Reload error" and exit non-zero if reloadcmd fails,
    # but the cert files are still installed. We check for files instead of exit code.
    ~/.acme.sh/acme.sh --installcert --force -d ${ipv4} \
        --key-file "${certDir}/privkey.pem" \
        --fullchain-file "${certDir}/fullchain.pem" \
        --reloadcmd "${reloadCmd}" 2>&1 || true

    # Verify certificate files exist (don't rely on exit code - reloadcmd failure causes non-zero)
    if [[ ! -f "${certDir}/fullchain.pem" || ! -f "${certDir}/privkey.pem" ]]; then
        echo -e "${red}安装后未找到证书文件${plain}"
        # Cleanup acme.sh data for both IPv4 and IPv6 if specified
        rm -rf ~/.acme.sh/${ipv4} ~/.acme.sh/${ipv4}_ecc 2> /dev/null
        [[ -n "$ipv6" ]] && rm -rf ~/.acme.sh/${ipv6} ~/.acme.sh/${ipv6}_ecc 2> /dev/null
        rm -rf ${certDir} 2> /dev/null
        return 1
    fi

    echo -e "${green}证书文件安装成功${plain}"

    # Enable auto-upgrade for acme.sh (ensures cron job runs)
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1

    # Secure permissions: private key readable only by owner
    chmod 600 ${certDir}/privkey.pem 2> /dev/null
    chmod 644 ${certDir}/fullchain.pem 2> /dev/null

    # Configure panel to use the certificate
    echo -e "${green}正在为面板设置证书路径……${plain}"
    ${xui_folder}/x-ui cert -webCert "${certDir}/fullchain.pem" -webCertKey "${certDir}/privkey.pem"

    if [ $? -ne 0 ]; then
        echo -e "${yellow}警告：无法自动设置证书路径${plain}"
        echo -e "${yellow}证书文件位置：${plain}"
        echo -e "  证书：${certDir}/fullchain.pem"
        echo -e "  密钥：${certDir}/privkey.pem"
    else
        echo -e "${green}证书路径配置成功${plain}"
    fi

    echo -e "${green}IP 证书安装配置成功！${plain}"
    echo -e "${green}证书有效期约 6 天，通过 acme.sh 定时任务自动续期。${plain}"
    echo -e "${yellow}acme.sh 会在到期前自动续期并重载 x-ui。${plain}"
    return 0
}

# Comprehensive manual SSL certificate issuance via acme.sh
ssl_cert_issue() {
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep 'webBasePath:' | awk -F': ' '{print $2}' | tr -d '[:space:]' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep 'port:' | awk -F': ' '{print $2}' | tr -d '[:space:]')

    # check for acme.sh first
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        echo "未找到 acme.sh，正在安装……"
        cd ~ || return 1
        curl -s https://get.acme.sh | sh
        if [ $? -ne 0 ]; then
            echo -e "${red}安装 acme.sh 失败${plain}"
            return 1
        else
            echo -e "${green}acme.sh 安装成功${plain}"
        fi
    fi

    # get the domain here, and we need to verify it
    local domain=""
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        domain="${XUI_DOMAIN// /}"
        if [[ -z "$domain" ]] || ! is_domain "$domain"; then
            echo -e "${red}XUI_SSL_MODE=domain 需要有效的 XUI_DOMAIN（当前值：'${XUI_DOMAIN:-}'）。${plain}"
            return 1
        fi
    else
        while true; do
            read -rp "请输入你的域名：" domain
            domain="${domain// /}" # Trim whitespace

            if [[ -z "$domain" ]]; then
                echo -e "${red}域名不能为空，请重试。${plain}"
                continue
            fi

            if ! is_domain "$domain"; then
                echo -e "${red}域名格式无效：${domain}，请输入有效的域名。${plain}"
                continue
            fi

            break
        done
    fi
    echo -e "${green}你的域名是：${domain}，正在检查……${plain}"
    SSL_ISSUED_DOMAIN="${domain}"

    # detect existing certificate and reuse it only if its files are actually
    # present and non-empty. acme.sh stores ECC certs under ${domain}_ecc and RSA
    # certs under ${domain}; a failed issuance can leave a domain entry in --list
    # with no usable cert files, which must not be reused (it produces a 0-byte
    # fullchain.pem). Broken partial state is cleaned up so issuance can proceed.
    local cert_exists=0
    if ~/.acme.sh/acme.sh --list 2> /dev/null | awk '{print $1}' | grep -Fxq "${domain}"; then
        local acmeCertDir=""
        if [[ -s ~/.acme.sh/${domain}_ecc/fullchain.cer && -s ~/.acme.sh/${domain}_ecc/${domain}.key ]]; then
            acmeCertDir=~/.acme.sh/${domain}_ecc
        elif [[ -s ~/.acme.sh/${domain}/fullchain.cer && -s ~/.acme.sh/${domain}/${domain}.key ]]; then
            acmeCertDir=~/.acme.sh/${domain}
        fi
        if [[ -n "${acmeCertDir}" ]]; then
            cert_exists=1
            local certInfo=$(~/.acme.sh/acme.sh --list 2> /dev/null | grep -F "${domain}")
            echo -e "${yellow}找到 ${domain} 的现有证书，将重复使用。${plain}"
            [[ -n "${certInfo}" ]] && echo "$certInfo"
        else
            echo -e "${yellow}发现 ${domain} 的 acme.sh 状态不完整（没有有效的证书文件），正在清理并重新申请。${plain}"
            rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc
        fi
    fi
    if [[ ${cert_exists} -eq 0 ]]; then
        echo -e "${green}你的域名现在可以申请证书了……${plain}"
    fi

    # create a directory for the certificate
    certPath="/root/cert/${domain}"
    if [ ! -d "$certPath" ]; then
        mkdir -p "$certPath"
    else
        rm -rf "$certPath"
        mkdir -p "$certPath"
    fi

    # get the port number for the standalone server
    local WebPort=80
    prompt_or_default WebPort "请选择要使用的端口（默认 80）：" "80" XUI_ACME_HTTP_PORT
    if [[ -z ${WebPort} ]]; then
        WebPort=80
    elif [[ ! ${WebPort} =~ ^[1-9][0-9]*$ || ${WebPort} -gt 65535 ]]; then
        echo -e "${yellow}你输入的 ${WebPort} 无效，将使用默认端口 80。${plain}"
        WebPort=80
    fi
    echo -e "${green}将使用端口 ${WebPort} 申请证书，请确保该端口已开放。${plain}"

    # Stop panel temporarily
    echo -e "${yellow}正在临时停止面板……${plain}"
    systemctl stop x-ui 2> /dev/null || rc-service x-ui stop 2> /dev/null

    if [[ ${cert_exists} -eq 0 ]]; then
        # issue the certificate
        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
        [[ -n "${XUI_ACME_EMAIL:-}" ]] && ~/.acme.sh/acme.sh --register-account -m "${XUI_ACME_EMAIL}" > /dev/null 2>&1
        ~/.acme.sh/acme.sh --issue -d ${domain} $(acme_listen_flag) --standalone --httpport ${WebPort} --force
        if [ $? -ne 0 ]; then
            echo -e "${red}申请证书失败，请查看日志。${plain}"
            rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc
            systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null
            return 1
        else
            echo -e "${green}申请证书成功，正在安装证书……${plain}"
        fi
    else
        echo -e "${green}正在使用现有证书，正在安装……${plain}"
    fi

    # Setup reload command
    reloadCmd="systemctl restart x-ui || rc-service x-ui restart"
    echo -e "${green}ACME 默认 --reloadcmd 是：${yellow}systemctl restart x-ui || rc-service x-ui restart${plain}"
    echo -e "${green}每次申请和续期证书时都会运行此命令。${plain}"
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        setReloadcmd="n"
    else
        read -rp "是否要修改 ACME 的 --reloadcmd？(y/n)：" setReloadcmd
    fi
    if [[ "$setReloadcmd" == "y" || "$setReloadcmd" == "Y" ]]; then
        echo -e "\n${green}\t1.${plain} 预设：systemctl reload nginx ; systemctl restart x-ui"
        echo -e "${green}\t2.${plain} 输入你自己的命令"
        echo -e "${green}\t0.${plain} 保持默认 reloadcmd"
        read -rp "请选择一个选项：" choice
        case "$choice" in
            1)
                echo -e "${green}Reloadcmd 是：systemctl reload nginx ; systemctl restart x-ui${plain}"
                reloadCmd="systemctl reload nginx ; systemctl restart x-ui"
                ;;
            2)
                echo -e "${yellow}建议把 x-ui restart 放在最后${plain}"
                read -rp "请输入你的自定义 reloadcmd：" reloadCmd
                echo -e "${green}Reloadcmd 是：${reloadCmd}${plain}"
                ;;
            *)
                echo -e "${green}保持默认 reloadcmd${plain}"
                ;;
        esac
    fi

    # install the certificate
    local installOutput=""
    installOutput=$(~/.acme.sh/acme.sh --installcert --force -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem --reloadcmd "${reloadCmd}" 2>&1)
    local installRc=$?
    echo "${installOutput}"

    local installWroteFiles=0
    if echo "${installOutput}" | grep -q "Installing key to:" && echo "${installOutput}" | grep -q "Installing full chain to:"; then
        installWroteFiles=1
    fi

    if [[ -f "/root/cert/${domain}/privkey.pem" && -f "/root/cert/${domain}/fullchain.pem" && (${installRc} -eq 0 || ${installWroteFiles} -eq 1) ]]; then
        echo -e "${green}安装证书成功，正在开启自动续期……${plain}"
    else
        echo -e "${red}安装证书失败，正在退出。${plain}"
        if [[ ${cert_exists} -eq 0 ]]; then
            rm -rf ~/.acme.sh/${domain} ~/.acme.sh/${domain}_ecc
        fi
        systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null
        return 1
    fi

    # enable auto-renew
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        echo -e "${yellow}自动续期设置有问题，证书详情：${plain}"
        ls -lah /root/cert/${domain}/
        # Secure permissions: private key readable only by owner
        chmod 600 $certPath/privkey.pem 2> /dev/null
        chmod 644 $certPath/fullchain.pem 2> /dev/null
    else
        echo -e "${green}自动续期设置成功，证书详情：${plain}"
        ls -lah /root/cert/${domain}/
        # Secure permissions: private key readable only by owner
        chmod 600 $certPath/privkey.pem 2> /dev/null
        chmod 644 $certPath/fullchain.pem 2> /dev/null
    fi

    # start panel
    systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null

    # Prompt user to set panel paths after successful certificate installation
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        setPanel="y"
    else
        read -rp "是否要将此证书设置给面板？(y/n)：" setPanel
    fi
    if [[ "$setPanel" == "y" || "$setPanel" == "Y" ]]; then
        local webCertFile="/root/cert/${domain}/fullchain.pem"
        local webKeyFile="/root/cert/${domain}/privkey.pem"

        if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
            ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
            echo -e "${green}已为面板设置证书路径${plain}"
            echo -e "${green}证书文件：$webCertFile${plain}"
            echo -e "${green}私钥文件：$webKeyFile${plain}"
            echo ""
            echo -e "${green}访问地址：https://${domain}:${existing_port}/${existing_webBasePath}${plain}"
            echo -e "${yellow}面板将重启以应用 SSL 证书……${plain}"
            systemctl restart x-ui 2> /dev/null || rc-service x-ui restart 2> /dev/null
        else
            echo -e "${red}错误：未找到域名 $domain 的证书或私钥文件。${plain}"
        fi
    else
        echo -e "${yellow}跳过面板路径设置。${plain}"
    fi

    return 0
}

# Reusable interactive SSL setup (domain or IP)
# Sets global `SSL_HOST` to the chosen domain/IP for Access URL usage
prompt_and_setup_ssl() {
    local panel_port="$1"
    local web_base_path="$2"
    local server_ip="$3"

    local ssl_choice=""
    SSL_SCHEME="https"

    echo -e "${yellow}选择 SSL 证书配置方式：${plain}"
    echo -e "${green}1.${plain} Let's Encrypt 域名证书（90 天有效期，自动续期）"
    echo -e "${green}2.${plain} Let's Encrypt IP 证书（6 天有效期，自动续期）"
    echo -e "${green}3.${plain} 自定义 SSL 证书（使用现有文件路径）"
    echo -e "${green}4.${plain} 跳过 SSL（高级选项——仅用于反向代理 / SSH 隧道后方）"
    echo -e "${blue}注意：${plain}选项 1 和 2 需要开放 80 端口，选项 3 需要手动提供路径。"
    echo -e "${blue}注意：${plain}选项 4 让面板走明文 HTTP——仅在 nginx/Caddy 反代或 SSH 隧道后方才安全。"
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        case "${XUI_SSL_MODE:-none}" in
            domain) ssl_choice="1" ;;
            ip) ssl_choice="2" ;;
            none | "") ssl_choice="4" ;;
            *)
                echo -e "${yellow}未知的 XUI_SSL_MODE='${XUI_SSL_MODE}'，默认为 none（HTTP）。${plain}"
                ssl_choice="4"
                ;;
        esac
    else
        read -rp "请选择一个选项（默认为 2，IP 证书）：" ssl_choice
        ssl_choice="${ssl_choice// /}" # Trim whitespace

        # Default to 2 (IP cert) if input is empty or invalid (not 1, 3 or 4)
        if [[ "$ssl_choice" != "1" && "$ssl_choice" != "3" && "$ssl_choice" != "4" ]]; then
            ssl_choice="2"
        fi
    fi

    case "$ssl_choice" in
        1)
            # User chose Let's Encrypt domain option
            echo -e "${green}正在使用 Let's Encrypt 申请域名证书……${plain}"
            if ssl_cert_issue; then
                local cert_domain="${SSL_ISSUED_DOMAIN}"
                if [[ -z "${cert_domain}" ]]; then
                    cert_domain=$(~/.acme.sh/acme.sh --list 2> /dev/null | tail -1 | awk '{print $1}')
                fi

                if [[ -n "${cert_domain}" ]]; then
                    SSL_HOST="${cert_domain}"
                    echo -e "${green}✓ SSL 证书配置成功，域名：${cert_domain}${plain}"
                else
                    echo -e "${yellow}SSL 配置可能已完成，但域名提取失败${plain}"
                    SSL_HOST="${server_ip}"
                fi
            else
                echo -e "${red}域名模式的 SSL 证书配置失败。${plain}"
                SSL_HOST="${server_ip}"
            fi
            ;;
        2)
            # User chose Let's Encrypt IP certificate option
            echo -e "${green}正在使用 Let's Encrypt 申请 IP 证书（shortlived 有效期）……${plain}"

            # Confirm the auto-detected IP before issuing for it: with asymmetric
            # routing / multi-WAN the echo services can return a transit address.
            if [[ "$NONINTERACTIVE" != "1" ]]; then
                local ip_confirm=""
                read -rp "${server_ip} 是这台服务器正确的入站公网 IPv4 地址吗？[默认 y]：" ip_confirm
                if [[ -n "$ip_confirm" && "$ip_confirm" != "y" && "$ip_confirm" != "Y" ]]; then
                    server_ip=""
                    while [[ -z "$server_ip" ]]; do
                        read -rp "请输入你的服务器公网 IPv4 地址：" server_ip
                        server_ip="${server_ip// /}"
                        if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                            echo -e "${red}IPv4 地址无效，请重试。${plain}"
                            server_ip=""
                        fi
                    done
                fi
            fi

            # Ask for optional IPv6
            local ipv6_addr=""
            prompt_or_default ipv6_addr "是否有要包含的 IPv6 地址？（留空则跳过）：" "" XUI_SSL_IPV6
            ipv6_addr="${ipv6_addr// /}" # Trim whitespace

            # Stop panel if running (port 80 needed)
            if [[ $release == "alpine" ]]; then
                rc-service x-ui stop > /dev/null 2>&1
            else
                systemctl stop x-ui > /dev/null 2>&1
            fi

            setup_ip_certificate "${server_ip}" "${ipv6_addr}"
            if [ $? -eq 0 ]; then
                SSL_HOST="${server_ip}"
                echo -e "${green}✓ Let's Encrypt IP 证书配置成功${plain}"
            else
                echo -e "${red}✗ IP 证书配置失败，请检查 80 端口是否开放。${plain}"
                SSL_HOST="${server_ip}"
            fi
            ;;
        3)
            # User chose Custom Paths (User Provided) option
            echo -e "${green}正在使用自定义现有证书……${plain}"
            local custom_cert=""
            local custom_key=""
            local custom_domain=""

            # 3.1 Request Domain to compose Panel URL later
            read -rp "请输入证书颁发给的域名：" custom_domain
            custom_domain="${custom_domain// /}" # Remove spaces

            # 3.2 Loop for Certificate Path
            while true; do
                read -rp "输入证书路径（关键词：.crt / fullchain）：" custom_cert
                # Strip quotes if present
                custom_cert=$(echo "$custom_cert" | tr -d '"' | tr -d "'")

                if [[ -f "$custom_cert" && -r "$custom_cert" && -s "$custom_cert" ]]; then
                    break
                elif [[ ! -f "$custom_cert" ]]; then
                    echo -e "${red}错误：文件不存在！请重试。${plain}"
                elif [[ ! -r "$custom_cert" ]]; then
                    echo -e "${red}错误：文件存在但不可读（请检查权限）！${plain}"
                else
                    echo -e "${red}错误：文件为空！${plain}"
                fi
            done

            # 3.3 Loop for Private Key Path
            while true; do
                read -rp "输入私钥路径（关键词：.key / privatekey）：" custom_key
                # Strip quotes if present
                custom_key=$(echo "$custom_key" | tr -d '"' | tr -d "'")

                if [[ -f "$custom_key" && -r "$custom_key" && -s "$custom_key" ]]; then
                    break
                elif [[ ! -f "$custom_key" ]]; then
                    echo -e "${red}错误：文件不存在！请重试。${plain}"
                elif [[ ! -r "$custom_key" ]]; then
                    echo -e "${red}错误：文件存在但不可读（请检查权限）！${plain}"
                else
                    echo -e "${red}错误：文件为空！${plain}"
                fi
            done

            # 3.4 Apply Settings via x-ui binary
            ${xui_folder}/x-ui cert -webCert "$custom_cert" -webCertKey "$custom_key" > /dev/null 2>&1

            # Set SSL_HOST for composing Panel URL
            if [[ -n "$custom_domain" ]]; then
                SSL_HOST="$custom_domain"
            else
                SSL_HOST="${server_ip}"
            fi

            echo -e "${green}✓ 自定义证书路径已应用。${plain}"
            echo -e "${yellow}注意：你需要自行在外部续期这些文件。${plain}"

            systemctl restart x-ui > /dev/null 2>&1 || rc-service x-ui restart > /dev/null 2>&1
            ;;
        4)
            echo ""
            echo -e "${red}⚠ 面板将不使用 SSL/TLS 进行安装。${plain}"
            echo -e "${yellow}登录凭证和 Cookie 将以明文 HTTP 传输。${plain}"
            echo -e "${yellow}仅在以下情况才安全：${plain}"
            echo -e "${yellow}  • 由反向代理（nginx、Caddy、Traefik）为你终结 TLS，或者${plain}"
            echo -e "${yellow}  • 你只通过 SSH 隧道访问面板${plain}"
            echo ""

            SSL_SCHEME="http"
            SSL_HOST="${server_ip}"

            local bind_local=""
            if [[ "$NONINTERACTIVE" == "1" ]]; then
                # Cloud images must stay reachable on their public interface.
                bind_local="n"
            else
                read -rp "是否将面板仅绑定到 127.0.0.1？（推荐——强制通过 SSH 隧道 / 反向代理访问）[y/N]：" bind_local
            fi
            if [[ "$bind_local" == "y" || "$bind_local" == "Y" ]]; then
                ${xui_folder}/x-ui setting -listenIP "127.0.0.1" > /dev/null 2>&1
                SSL_HOST="127.0.0.1"
                echo -e "${green}✓ 面板已仅绑定 127.0.0.1，公网无法访问。${plain}"
                echo ""
                echo -e "${green}SSH 端口转发——从你的本地电脑通过以下方式打开面板：${plain}"
                echo -e "  标准 SSH 命令："
                echo -e "  ${yellow}ssh -L 2222:127.0.0.1:${panel_port} root@${server_ip}${plain}"
                echo -e "  如果使用 SSH 密钥："
                echo -e "  ${yellow}ssh -i <sshkeypath> -L 2222:127.0.0.1:${panel_port} root@${server_ip}${plain}"
                echo -e "  然后在浏览器中打开："
                echo -e "  ${yellow}http://localhost:2222/${web_base_path}${plain}"
                echo ""
                echo -e "${yellow}替代方案：将反向代理（nginx/Caddy）指向 127.0.0.1:${panel_port}，由它终结 TLS。${plain}"
            else
                echo -e "${yellow}面板将通过明文 HTTP 监听所有网卡，请确保前方有其他服务在终结 TLS。${plain}"
            fi

            systemctl restart x-ui > /dev/null 2>&1 || rc-service x-ui restart > /dev/null 2>&1
            echo -e "${green}✓ 已跳过 SSL 配置。${plain}"
            ;;
        *)
            echo -e "${red}选项无效，跳过 SSL 配置。${plain}"
            SSL_HOST="${server_ip}"
            ;;
    esac
}

config_after_install() {
    local settings existing_webBasePath
    if ! settings=$("${xui_folder}/x-ui" setting -show true) ||
        ! existing_webBasePath=$(printf '%s\n' "$settings" | awk '
            /^get .* failed, error info:/ { failed = 1 }
            /^hasDefaultCredential: (true|false)$/ { credentials++ }
            /^port: [0-9]+$/ { ports++ }
            /^webBasePath:/ { paths++; sub(/^webBasePath: ?/, ""); path = $0 }
            END {
                if (failed || credentials != 1 || ports != 1 || paths != 1 || (path != "" && path !~ /^\//)) exit 1
                print path
            }'); then
        echo -e "${red}无法读取 webBasePath，保持现有设置不变。${plain}" >&2
        return 1
    fi
    existing_webBasePath=${existing_webBasePath#/}
    local existing_hasDefaultCredential=$(printf '%s\n' "$settings" | awk '/^hasDefaultCredential: / {print $2}')
    local existing_port=$(printf '%s\n' "$settings" | awk '/^port: / {print $2}')
    # Properly detect empty cert by checking if cert: line exists and has content after it
    local existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
    local URL_lists=(
        "https://api4.ipify.org"
        "https://ipv4.icanhazip.com"
        "https://v4.api.ipinfo.io/ip"
        "https://ipv4.myexternalip.com/raw"
        "https://4.ident.me"
        "https://check-host.net/ip"
    )
    local server_ip=""
    for ip_address in "${URL_lists[@]}"; do
        local response=$(curl -s -w "\n%{http_code}" --max-time 3 "${ip_address}" 2> /dev/null)
        local http_code=$(echo "$response" | tail -n1)
        local ip_result=$(echo "$response" | head -n-1 | tr -d '[:space:]"')
        if [[ "${http_code}" == "200" && "${ip_result}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            server_ip="${ip_result}"
            break
        fi
    done

    if [[ -z "$server_ip" ]]; then
        if [[ "$NONINTERACTIVE" == "1" ]]; then
            # Panel binds 0.0.0.0 regardless; the IP is only used to compose the
            # displayed access URL. Fall back to XUI_SERVER_IP or leave blank.
            server_ip="${XUI_SERVER_IP:-}"
        else
            echo -e "${yellow}无法从任何服务商自动检测服务器 IP。${plain}"
            while [[ -z "$server_ip" ]]; do
                read -rp "请输入你的服务器公网 IPv4 地址：" server_ip
                server_ip="${server_ip// /}"
                if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                    echo -e "${red}IPv4 地址无效，请重试。${plain}"
                    server_ip=""
                fi
            done
        fi
    fi

    if [[ -z "$existing_webBasePath" ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath="${XUI_WEB_BASE_PATH:-$(gen_random_string 18)}"
            local config_username="${XUI_USERNAME:-$(gen_random_string 10)}"
            local config_password="${XUI_PASSWORD:-$(gen_random_string 10)}"
            local config_port=""

            local db_label="SQLite (/etc/x-ui/x-ui.db)"
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     数据库选择                            ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "  1) SQLite     （默认——建议客户端少于 500 时使用）"
            echo -e "  2) PostgreSQL（建议客户端数量多 / 节点多时使用）"
            if [[ "$NONINTERACTIVE" == "1" ]]; then
                if [[ "${XUI_DB_TYPE:-sqlite}" == "postgres" ]]; then
                    db_choice="2"
                else
                    db_choice="1"
                fi
            else
                read -rp "请选择 [1]：" db_choice
                db_choice="${db_choice:-1}"
            fi
            if [[ "$db_choice" == "2" ]]; then
                local xui_env_file
                case "${release}" in
                    ubuntu | debian | armbian)
                        xui_env_file="/etc/default/x-ui"
                        ;;
                    arch | manjaro | parch | alpine)
                        xui_env_file="/etc/conf.d/x-ui"
                        ;;
                    *)
                        xui_env_file="/etc/sysconfig/x-ui"
                        ;;
                esac

                local xui_dsn=""
                local pg_mode=""
                local pg_local_installed=0
                while [[ -z "$xui_dsn" ]]; do
                    if [[ "$NONINTERACTIVE" == "1" ]]; then
                        if [[ -n "${XUI_DB_DSN:-}" ]]; then
                            xui_dsn="${XUI_DB_DSN}"
                            db_label="PostgreSQL (external)"
                            break
                        fi
                        echo -e "${yellow}正在本地安装 PostgreSQL（非交互模式）……${plain}"
                        local pg_cred_file
                        pg_cred_file=$(mktemp 2> /dev/null) || pg_cred_file=$(mktemp -t x-ui-pg-creds.XXXXXXXX)
                        if [[ -n "${pg_cred_file}" ]] && xui_dsn=$(PG_CRED_FILE="${pg_cred_file}" install_postgres_local); then
                            pg_local_installed=1
                            if [[ -r "${pg_cred_file}" ]]; then
                                # shellcheck disable=SC1090
                                source "${pg_cred_file}"
                            fi
                            rm -f "${pg_cred_file}"
                            db_label="PostgreSQL (${PG_USER}@${PG_HOST}:${PG_PORT}/${PG_DB})"
                            break
                        fi
                        rm -f "${pg_cred_file}"
                        echo -e "${red}非交互模式下安装 PostgreSQL 失败，正在中止。${plain}"
                        echo -e "${yellow}请设置 XUI_DB_DSN 使用现有服务器，或设置 XUI_DB_TYPE=sqlite。${plain}"
                        exit 1
                    fi
                    echo ""
                    echo -e "  1) 在本地安装 PostgreSQL 并创建专用用户/数据库（推荐）"
                    echo -e "  2) 使用现有 PostgreSQL 服务器（输入 DSN）"
                    read -rp "请选择 [1]：" pg_mode
                    pg_mode="${pg_mode:-1}"
                    if [[ "$pg_mode" == "2" ]]; then
                        while [[ -z "$xui_dsn" ]]; do
                            read -rp "输入 PostgreSQL DSN（postgres://user:pass@host:port/dbname?sslmode=disable）：" xui_dsn
                            xui_dsn="${xui_dsn// /}"
                        done
                        db_label="PostgreSQL (external)"
                    else
                        echo -e "${yellow}正在安装 PostgreSQL——可能需要一点时间……${plain}"
                        local pg_cred_file
                        pg_cred_file=$(mktemp 2> /dev/null) || pg_cred_file=$(mktemp -t x-ui-pg-creds.XXXXXXXX)
                        if [[ -z "${pg_cred_file}" ]]; then
                            echo -e "${red}创建临时凭证文件失败。${plain}"
                            xui_dsn=""
                            continue
                        fi
                        if xui_dsn=$(PG_CRED_FILE="${pg_cred_file}" install_postgres_local); then
                            pg_local_installed=1
                            if [[ -r "${pg_cred_file}" ]]; then
                                # shellcheck disable=SC1090
                                source "${pg_cred_file}"
                            fi
                            rm -f "${pg_cred_file}"
                            db_label="PostgreSQL (${PG_USER}@${PG_HOST}:${PG_PORT}/${PG_DB})"
                        else
                            rm -f "${pg_cred_file}"
                            echo ""
                            echo -e "${red}安装 PostgreSQL 失败。${plain}"
                            echo -e "  1) 重试本地安装"
                            echo -e "  2) 改用外部 DSN"
                            echo -e "  3) 中止安装"
                            echo -e "  4) 回退到 SQLite"
                            read -rp "请选择 [1]：" pg_fail
                            pg_fail="${pg_fail:-1}"
                            case "$pg_fail" in
                                2) pg_mode="2" ;;
                                3)
                                    echo -e "${red}安装已中止。${plain}"
                                    exit 1
                                    ;;
                                4)
                                    db_choice="1"
                                    xui_dsn=""
                                    break
                                    ;;
                                *) xui_dsn="" ;;
                            esac
                        fi
                    fi
                done
                if [[ -n "$xui_dsn" ]]; then
                    install -d -m 755 "$(dirname "$xui_env_file")"
                    umask 077
                    cat > "$xui_env_file" << EOF
XUI_DB_TYPE=postgres
XUI_DB_DSN=${xui_dsn}
EOF
                    chmod 600 "$xui_env_file"
                    umask 022
                    export XUI_DB_TYPE=postgres
                    export XUI_DB_DSN="${xui_dsn}"
                    ensure_pg_client || echo -e "${yellow}⚠ 无法安装 pg_dump/pg_restore。在你安装 postgresql-client 软件包之前，面板内的数据库备份/恢复功能将不可用。${plain}"
                fi
            fi

            config_port=$(select_panel_port) || exit 1

            local settings_output settings_status=0
            settings_output=$("${xui_folder}/x-ui" setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}" 2>&1) || settings_status=$?
            printf '%s\n' "$settings_output"
            # Released CLIs may print a settings/database error and still exit 0.
            # Require all three existing success messages before claiming success.
            if [[ "$settings_status" != 0 ]] ||
                ! grep -Fxq "Port set successfully: ${config_port}" <<< "$settings_output" ||
                ! grep -Fxq 'Username and password updated successfully' <<< "$settings_output" ||
                ! grep -Fxq 'Base URI path set successfully' <<< "$settings_output"; then
                echo '配置面板设置失败，安装已中止。' >&2
                exit 1
            fi

            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     SSL 证书配置（推荐）                  ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}强烈建议使用 SSL。仅当反向代理${plain}"
            echo -e "${yellow}或 SSH 隧道为你处理 TLS 时才跳过。${plain}"
            echo -e "${yellow}Let's Encrypt 现在同时支持域名和 IP 地址！${plain}"
            echo ""

            prompt_and_setup_ssl "${config_port}" "${config_webBasePath}" "${server_ip}"

            # Retrieve the API token for display
            local config_apiToken=$(${xui_folder}/x-ui setting -getApiToken | grep -Eo 'apiToken: .+' | awk '{print $2}')

            # Display final credentials and access information
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     面板安装完成！                       ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}用户名：     ${config_username}${plain}"
            echo -e "${green}密码：       ${config_password}${plain}"
            echo -e "${green}端口：       ${config_port}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "${green}数据库：     ${db_label}${plain}"
            echo -e "${green}访问地址：   ${SSL_SCHEME}://${SSL_HOST}:${config_port}/${config_webBasePath}${plain}"
            echo -e "${green}API Token：  ${config_apiToken}${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}⚠ 重要：请妥善保存这些凭证！${plain}"
            if [[ "$SSL_SCHEME" == "https" ]]; then
                echo -e "${yellow}⚠ SSL 证书：已启用并配置${plain}"
            else
                echo -e "${yellow}⚠ SSL 证书：已跳过——面板仅支持 HTTP，请使用反向代理或 SSH 隧道。${plain}"
            fi

            if [[ "$db_choice" == "2" ]]; then
                echo ""
                echo -e "${green}面板内置了 PostgreSQL 备份与恢复功能：${plain}"
                echo -e "  ${blue}${SSL_SCHEME}://${SSL_HOST}:${config_port}/${config_webBasePath}${plain} → 备份与恢复"
                echo -e "${yellow}  “备份”会下载 pg_dump 的 .dump 文件；“恢复”会通过 pg_restore 重新载入。${plain}"
            fi

            if [[ "$db_choice" == "2" && "$pg_local_installed" == "1" ]]; then
                echo ""
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${green}     PostgreSQL 凭证                      ${plain}"
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${green}数据库名：  ${PG_DB}${plain}"
                echo -e "${green}用户名：    ${PG_USER}${plain}"
                echo -e "${green}密码：      ${PG_PASS}${plain}"
                echo -e "${green}主机：      ${PG_HOST}${plain}"
                echo -e "${green}端口：      ${PG_PORT}${plain}"
                echo -e "${green}DSN：       ${xui_dsn}${plain}"
                echo -e "${green}环境文件：  ${xui_env_file}${plain}"
                echo -e "${green}-------------------------------------------${plain}"
                echo -e "${green}从本服务器连接：${plain}"
                echo -e "  ${blue}sudo -u postgres psql -d ${PG_DB}${plain}      （以 postgres 超级用户身份）"
                echo -e "  ${blue}PGPASSWORD='${PG_PASS}' psql -h ${PG_HOST} -p ${PG_PORT} -U ${PG_USER} -d ${PG_DB}${plain}"
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${yellow}⚠ 面板从 ${xui_env_file} 读取这些凭证。${plain}"
                echo -e "${yellow}⚠ 请保存好密码——它没有以明文保存在其他任何地方。${plain}"
                unset PG_USER PG_PASS PG_HOST PG_PORT PG_DB
            fi

            # Persist a machine-parseable credentials file for cloud-init / MOTD.
            : "${SSL_SCHEME:=https}"
            : "${SSL_HOST:=${server_ip}}"
            local db_type_out="sqlite"
            [[ "$db_choice" == "2" ]] && db_type_out="postgres"
            write_install_result "${config_username}" "${config_password}" "${config_port}" \
                "${config_webBasePath}" "${SSL_SCHEME}" "${SSL_HOST}" "${config_apiToken}" "${db_type_out}"
        else
            local config_webBasePath=$(gen_random_string 18)
            echo -e "${yellow}WebBasePath 缺失或被设为根路径，正在生成新的……${plain}"
            ${xui_folder}/x-ui setting -webBasePath "${config_webBasePath}"
            echo -e "${green}新的 WebBasePath：${config_webBasePath}${plain}"

            # If the panel is already installed but no certificate is configured, prompt for SSL now
            if [[ -z "${existing_cert}" ]]; then
                echo ""
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${green}     SSL 证书配置（推荐）                  ${plain}"
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${yellow}Let's Encrypt 现在同时支持域名和 IP 地址！${plain}"
                echo ""
                prompt_and_setup_ssl "${existing_port}" "${config_webBasePath}" "${server_ip}"
                echo -e "${green}访问地址：   ${SSL_SCHEME}://${SSL_HOST}:${existing_port}/${config_webBasePath}${plain}"
            else
                # If a cert already exists, just show the access URL
                echo -e "${green}访问地址：https://${server_ip}:${existing_port}/${config_webBasePath}${plain}"
            fi
        fi
    else
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_username="${XUI_USERNAME:-$(gen_random_string 10)}"
            local config_password="${XUI_PASSWORD:-$(gen_random_string 10)}"

            echo -e "${yellow}检测到默认凭证，需要进行安全更新……${plain}"
            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}"
            echo -e "已生成新的随机登录凭证："
            echo -e "###############################################"
            echo -e "${green}用户名：${config_username}${plain}"
            echo -e "${green}密码：${config_password}${plain}"
            echo -e "###############################################"

            # Persist a machine-parseable credentials file for cloud-init / MOTD.
            local config_apiToken
            config_apiToken=$(${xui_folder}/x-ui setting -getApiToken | grep -Eo 'apiToken: .+' | awk '{print $2}')
            : "${SSL_SCHEME:=https}"
            : "${SSL_HOST:=${server_ip}}"
            write_install_result "${config_username}" "${config_password}" "${existing_port}" \
                "${existing_webBasePath}" "${SSL_SCHEME}" "${SSL_HOST}" "${config_apiToken}" "${XUI_DB_TYPE:-sqlite}"
        else
            echo -e "${green}用户名、密码和 WebBasePath 均已正确设置。${plain}"
        fi

        # Existing install: if no cert configured, prompt user for SSL setup
        # Properly detect empty cert by checking if cert: line exists and has content after it
        existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
        if [[ -z "$existing_cert" ]]; then
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     SSL 证书配置（推荐）                  ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}Let's Encrypt 现在同时支持域名和 IP 地址！${plain}"
            echo ""
            prompt_and_setup_ssl "${existing_port}" "${existing_webBasePath}" "${server_ip}"
            echo -e "${green}访问地址：   ${SSL_SCHEME}://${SSL_HOST}:${existing_port}/${existing_webBasePath}${plain}"
        else
            echo -e "${green}SSL 证书已配置，无需操作。${plain}"
        fi
    fi

    ${xui_folder}/x-ui migrate
}

# setup_fail2ban auto-installs and configures fail2ban for the IP Limit feature
# by invoking the freshly installed x-ui CLI. IP Limit is load-bearing on
# fail2ban (without it the panel disables the limitIp field and zeroes existing
# limits), so a fresh install should make it work out of the box, just like the
# Docker image already does. Non-fatal by design: a fail2ban failure must never
# abort the panel install.
setup_fail2ban() {
    if [[ -n "${XUI_ENABLE_FAIL2BAN+x}" && "${XUI_ENABLE_FAIL2BAN}" != "true" ]]; then
        echo -e "${yellow}XUI_ENABLE_FAIL2BAN=${XUI_ENABLE_FAIL2BAN}，跳过 Fail2ban 自动配置。${plain}"
        return 0
    fi

    if [[ ! -x /usr/bin/x-ui ]]; then
        echo -e "${yellow}未找到 x-ui 命令行工具，跳过 Fail2ban 自动配置。${plain}"
        return 0
    fi

    # Scripts older than v3.4.0 have no setup-fail2ban and exit 0 from the
    # usage banner, which would read as success here.
    if ! grep -q '"setup-fail2ban")' /usr/bin/x-ui; then
        echo -e "${yellow}当前 x-ui.sh 版本早于 'x-ui setup-fail2ban' 功能，跳过 Fail2ban 自动配置。${plain}"
        return 0
    fi

    echo -e "${green}正在为 IP 限制功能配置 Fail2ban……${plain}"
    if /usr/bin/x-ui setup-fail2ban; then
        echo -e "${green}Fail2ban 配置完成。${plain}"
    else
        echo -e "${yellow}Fail2ban 配置未完成；在你运行 'x-ui' 并打开 IP 限制菜单之前，IP 限制将保持禁用。继续安装。${plain}"
    fi
    return 0
}

# Major version of the local systemd, 0 when it cannot be determined. The
# SystemCallFilter=@system-service group only exists from systemd 239 on (other
# @-named groups exist since 231); on older versions an unknown group is not
# ignored safely, the filter stays in force and leaves a whitelist the panel
# cannot run under.
_xui_systemd_major_version() {
    local version=""
    if command -v systemctl > /dev/null 2>&1; then
        version="$(systemctl --version 2>/dev/null | awk 'NR == 1 {print $2}')"
    fi
    if [[ ! "$version" =~ ^[0-9]+$ ]]; then
        echo 0
        return 0
    fi
    echo "$version"
}

# The shipped units list hardening that older systemd does not know: the
# directive is logged and ignored at load time rather than rejected, so the
# panel still starts, only without that protection. Each entry is the systemd
# release that introduced the directive (systemd.exec(5)); everything else in
# the unit predates the oldest systemd install.sh supports (CentOS 7 has 219).
# SystemCallFilter= is listed because the drop-in only writes it from 239 on.
_xui_warn_unsupported_hardening() {
    local version entry missing=""
    version="$(_xui_systemd_major_version)"
    [[ "$version" -gt 0 ]] || return 0
    for entry in RestrictRealtime:231 ReadWritePaths:231 ProtectKernelTunables:232 \
        ProtectKernelModules:232 RestrictNamespaces:233 LockPersonality:235 \
        SystemCallFilter:239 ProtectHostname:242 RestrictSUIDSGID:242 \
        ProtectKernelLogs:244 ProtectClock:245; do
        if [[ "$version" -lt "${entry##*:}" ]]; then
            missing="${missing:+$missing, }${entry%%:*} (${entry##*:})"
        fi
    done
    [[ -n "$missing" ]] || return 0
    echo -e "${yellow}注意：systemd ${version} 会忽略 x-ui.service 中的部分加固配置，面板仍可正常启动。${plain}"
    echo "      未应用，需要更新的 systemd：${missing}。"
    if [[ "$version" -lt 231 ]]; then
        echo "      面板的文件夹仍可通过 ReadWriteDirectories= 写入，这是本脚本安装的别名。"
    fi
    echo "      其余加固配置已生效。请升级 systemd 以应用上述配置。"
    return 0
}

# ProtectSystem=full makes /usr, /boot, /efi and /etc read-only. ProtectSystem=
# strict would make the whole hierarchy read-only (only the kernel API
# filesystems stay as they are), and that would break the panel's own use of
# /tmp. The panel's stores are configurable (XUI_DB_FOLDER, XUI_LOG_FOLDER,
# XUI_BIN_FOLDER), and XUI_MAIN_FOLDER is the folder install.sh/update.sh place
# the files in -- the unit's WorkingDirectory on a stock install, and what a
# relative XUI_BIN_FOLDER is resolved against. So a hard-coded list in the unit
# either misses a relocated store -- the panel then cannot write its own SQLite
# database and sits in a Restart=on-failure loop -- or forces the operator to
# edit a file that every install/update overwrites from the release tarball.
# install.sh and update.sh therefore regenerate the drop-in from the folders
# actually in use, and the unit's own ReadWritePaths only carry the
# plain-install defaults. A relocated store means re-running install or update:
# the drop-in is only written here.
_xui_service_write_paths_dropin() {
    # $1 is the env file to resolve the XUI_* folders from; callers pass nothing
    # and get the OS-specific path the unit itself uses.
    local env_file="${1:-}"
    local dropin_dir dropin temp_file
    local db_folder log_folder bin_folder main_folder
    local path line="" whitespace_paths="" seen_paths="" escaped_path

    if [[ -z "$env_file" ]]; then
        case "${release}" in
            ubuntu | debian | armbian)
                env_file="/etc/default/x-ui"
                ;;
            arch | manjaro | parch | alpine)
                env_file="/etc/conf.d/x-ui"
                ;;
            *)
                env_file="/etc/sysconfig/x-ui"
                ;;
        esac
    fi
    if [[ -r "$env_file" ]]; then
        set -a
        # shellcheck disable=SC1090
        source "$env_file"
        set +a
    fi

    # XUI_* wins over the script's own default: the unit hands that same env
    # file to the panel through EnvironmentFile=, so these are the folders it
    # will actually use.
    main_folder="${XUI_MAIN_FOLDER:-${xui_folder}}"
    db_folder="${XUI_DB_FOLDER:-/etc/x-ui}"
    log_folder="${XUI_LOG_FOLDER:-/var/log/x-ui}"
    # An empty XUI_BIN_FOLDER resolves to "bin" relative to the panel's working
    # directory, which the unit sets to the main folder.
    bin_folder="${XUI_BIN_FOLDER:-bin}"
    if [[ "$bin_folder" != /* ]]; then
        bin_folder="${main_folder%/}/${bin_folder#./}"
    fi

    for path in "$db_folder" "$log_folder" "$bin_folder" "$main_folder"; do
        [[ "$path" == /* ]] || continue
        # ReadWritePaths= is a whitespace-separated list, and a folder whose
        # name contains whitespace cannot be written into it without relying on
        # quoting. A wrong entry makes systemd reject the whole drop-in and the
        # panel would not start, so leave such a folder out and say so instead.
        if [[ "$path" != "${path//[[:space:]]/}" ]]; then
            whitespace_paths="${whitespace_paths:+$whitespace_paths }$path"
            continue
        fi
        case " $seen_paths " in
            *" $path "*) continue ;;
        esac
        seen_paths="${seen_paths}${seen_paths:+ }$path"
        # systemd expands %-specifiers in unit files, so a folder name carrying
        # a literal % has to be written as %%, or the entry stops naming the
        # folder systemd is meant to keep writable.
        escaped_path="${path//%/%%}"
        line="${line} -${escaped_path}"
    done
    if [[ -n "$whitespace_paths" ]]; then
        echo "警告：以下文件夹包含空格，已从中排除" >&2
        echo "         10-xui-sandbox.conf: $whitespace_paths" >&2
        echo "         在该服务的沙箱下，面板无法写入这些文件夹。" >&2
    fi
    line="${line# }"
    [[ -n "$line" ]] || return 1

    dropin_dir="${xui_service}/x-ui.service.d"
    dropin="${dropin_dir}/10-xui-sandbox.conf"
    temp_file="${dropin}.tmp.$$"

    mkdir -p "$dropin_dir" || return 1
    cat > "$temp_file" << EOF
# Regenerated by install.sh/update.sh on every install and update: edits here
# are lost, and the list only reflects the XUI_* variables read from
# ${env_file} at that moment. Re-run install/update after moving a store.
# It lists the folders the panel writes to. Put local additions in their own
# drop-in, for example 20-x-ui-local.conf, which nothing here touches.
[Service]
ReadWritePaths=${line}
ReadWriteDirectories=${line}
EOF
    if [[ "$(_xui_systemd_major_version)" -ge 239 ]]; then
        cat >> "$temp_file" << 'EOF'
# @system-service needs systemd >= 239; on older versions the unknown group
# would leave the panel with a filter it cannot start under (x-ui.service.*).
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
EOF
    fi
    if [[ ! -s "$temp_file" ]]; then
        rm -f "$temp_file"
        return 1
    fi
    chmod 644 "$temp_file"
    mv -f "$temp_file" "$dropin" || { rm -f "$temp_file"; return 1; }
    if command -v systemctl > /dev/null 2>&1; then
        systemctl daemon-reload > /dev/null 2>&1 || true
    fi
    return 0
}

# Lands a systemd unit file at ${xui_service}/x-ui.service via a temp file +
# atomic mv, so a failed cp/curl or an interrupted mv never leaves a
# truncated unit file at the live path -- systemd would then fail to parse
# it on the next daemon-reload/start. Same pattern already used for
# /usr/bin/x-ui elsewhere in this script. source_is_url picks cp (from a
# file already extracted from the release tarball) vs curl (GitHub fallback).
_install_xui_service_unit() {
    local source="$1"
    local source_is_url="$2"
    local dest="${xui_service}/x-ui.service"
    local temp_file="${dest}.tmp.$$"

    rm -f "$temp_file"
    if [[ "$source_is_url" == "true" ]]; then
        curl -fLRo "$temp_file" "$source" > /dev/null 2>&1
    else
        cp -f "$source" "$temp_file" > /dev/null 2>&1
    fi
    if [[ $? -ne 0 ]]; then
        rm -f "$temp_file"
        return 1
    fi
    if [[ ! -s "$temp_file" ]]; then
        rm -f "$temp_file"
        return 1
    fi
    mv -f "$temp_file" "$dest"
    if [[ $? -ne 0 ]]; then
        rm -f "$temp_file"
        return 1
    fi
    if ! _xui_service_write_paths_dropin; then
        echo -e "${yellow}警告：无法刷新 ${xui_service}/x-ui.service.d/10-xui-sandbox.conf。${plain}"
        echo -e "${yellow}如果 XUI_DB_FOLDER 或 XUI_LOG_FOLDER 指向 /etc/x-ui 和 /var/log/x-ui 之外，在 ProtectSystem=full 下面板可能无法写入。${plain}"
    fi
    _xui_warn_unsupported_hardening
    return 0
}

# resolve_latest_tag prints the latest stable release tag. It prefers the web
# releases/latest redirect, which is not subject to the unauthenticated API's
# 60 req/h-per-IP limit that trips shared CI/CGNAT addresses (the install then
# fails with "Failed to fetch x-ui version"), and falls back to the API.
resolve_latest_tag() {
    local url tag
    url=$(curl -sSLI -o /dev/null -w '%{url_effective}' --retry 5 --retry-delay 3 --connect-timeout 15 --max-time 60 "https://github.com/MHSanaei/3x-ui/releases/latest" 2>/dev/null)
    tag=${url##*/tag/}
    if [[ "$tag" != "$url" && -n "$tag" && "$tag" != "latest" ]]; then
        echo "$tag"
        return 0
    fi
    curl -Ls --retry 5 --retry-delay 3 --connect-timeout 15 --max-time 60 "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
}

# Releases publish <asset>.sha256 next to each archive. A mismatch or a failed
# sidecar download aborts the install; only a 404 (releases predating the
# sidecar) is tolerated with a warning.
verify_release_checksum() {
    local url="$1" file="$2" sums="$2.sha256" code expected actual
    rm -f "${sums}"
    code=$(curl -sL --retry 3 --retry-delay 3 --connect-timeout 15 --max-time 60 -o "${sums}" -w '%{http_code}' "${url}.sha256")
    if [[ "${code}" == "404" ]]; then
        rm -f "${sums}"
        echo -e "${yellow}该版本未发布校验和，跳过验证${plain}"
        return 0
    fi
    if [[ "${code}" != "200" ]]; then
        rm -f "${sums}" "${file}"
        echo -e "${red}下载 $(basename "${file}") 的校验和失败（HTTP ${code}）${plain}"
        exit 1
    fi
    expected=$(awk 'NR == 1 {print $1}' "${sums}")
    actual=$(sha256sum "${file}" | awk '{print $1}')
    rm -f "${sums}"
    if [[ ! "${expected}" =~ ^[0-9a-f]{64}$ || "${expected}" != "${actual}" ]]; then
        rm -f "${file}"
        echo -e "${red}$(basename "${file}") 校验和不匹配：预期 ${expected:-<none>}，实际 ${actual}${plain}"
        exit 1
    fi
    echo -e "${green}校验和验证通过：${actual}${plain}"
}

# Older tags predate some of these files (x-ui.rc arrived in v2.8.4). Serving
# main's copy against an old binary is the mismatch this pinning exists to
# prevent, so probe before anything is stopped or removed and refuse the tag.
require_repo_files() {
    local ref="$1" name status
    shift
    [[ "${ref}" == "main" ]] && return 0
    for name in "$@"; do
        status=$(curl -sIL --retry 3 --connect-timeout 15 -o /dev/null -w '%{http_code}' "https://raw.githubusercontent.com/MHSanaei/3x-ui/${ref}/${name}")
        if [[ "${status}" != "200" ]]; then
            echo -e "${red}${name} 在 ${ref} 中不可用（HTTP ${status}）${plain}"
            echo -e "${red}请安装包含它的发布版本，或使用 'dev' 获取滚动构建。你的现有安装未被改动。${plain}"
            exit 1
        fi
    done
}

install_x-ui() {
    cd ${xui_folder%/x-ui}/

    # Download resources
    if [ $# == 0 ]; then
        tag_version=$(resolve_latest_tag)
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${red}获取 x-ui 版本失败，可能是 GitHub API 限制，请稍后再试${plain}"
            exit 1
        fi
        echo -e "已获取 x-ui 最新版本：${tag_version}，开始安装……"
        curl -fLR --retry 5 --retry-delay 3 --connect-timeout 15 --speed-limit 1 --speed-time 300 -o ${xui_folder}-linux-$(arch).tar.gz https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui 失败，请确保你的服务器可以访问 GitHub${plain}"
            exit 1
        fi
        if [[ ! -s ${xui_folder}-linux-$(arch).tar.gz ]]; then
            rm ${xui_folder}-linux-$(arch).tar.gz -f
            echo -e "${red}下载的 x-ui 发布包为空${plain}"
            exit 1
        fi
        verify_release_checksum "https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz" "${xui_folder}-linux-$(arch).tar.gz"
    else
        tag_version=$1
        # The rolling dev channel ships under a fixed, non-semver tag that is
        # force-moved to the latest main commit on every push. Accept `dev` as a
        # convenient alias and skip the numeric floor check for it.
        if [[ "$tag_version" == "dev" || "$tag_version" == "dev-latest" ]]; then
            tag_version="dev-latest"
            echo -e "${yellow}正在安装滚动开发版（标签：dev-latest），这是每次提交的预发布版本，不是稳定版。${plain}"
        else
            tag_version_numeric=${tag_version#v}
            min_version="2.3.5"

            if [[ "$(printf '%s\n' "$min_version" "$tag_version_numeric" | sort -V | head -n1)" != "$min_version" ]]; then
                echo -e "${red}请使用更新的版本（至少 v2.3.5），正在退出安装。${plain}"
                exit 1
            fi
        fi

        url="https://github.com/MHSanaei/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz"
        echo -e "开始安装 x-ui ${tag_version}"
        curl -fLR --retry 5 --retry-delay 3 --connect-timeout 15 --speed-limit 1 --speed-time 300 -o ${xui_folder}-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui ${tag_version} 失败，请检查该版本是否存在${plain}"
            exit 1
        fi
        if [[ ! -s ${xui_folder}-linux-$(arch).tar.gz ]]; then
            rm ${xui_folder}-linux-$(arch).tar.gz -f
            echo -e "${red}下载的 x-ui 发布包为空${plain}"
            exit 1
        fi
        verify_release_checksum "${url}" "${xui_folder}-linux-$(arch).tar.gz"
    fi
    # x-ui.sh, x-ui.rc and the unit files must come from the same release as
    # the binary; only the rolling dev build tracks main.
    local script_ref="${tag_version}"
    if [[ "${tag_version}" == "dev-latest" ]]; then
        script_ref="main"
    fi
    # The unit files are only fetched when the release tarball lacks them, so
    # they are checked at that point instead of here.
    local required_files=("x-ui.sh")
    [[ $release == "alpine" ]] && required_files+=("x-ui.rc")
    require_repo_files "${script_ref}" "${required_files[@]}"
    local xui_script_temp="/usr/bin/x-ui-temp.$$"
    rm -f "${xui_script_temp}"
    # 中文 fork：菜单脚本改由本 fork 的 main 分支获取，使安装后菜单保持中文；
    # 二进制文件仍从上游 releases 下载，x-ui.rc 与 systemd 单元文件亦然。
    curl -fLRo "${xui_script_temp}" "https://raw.githubusercontent.com/soxlola/3x-ui/main/x-ui.sh"
    if [[ $? -ne 0 ]]; then
        rm -f "${xui_script_temp}"
        echo -e "${red}下载 x-ui.sh 失败${plain}"
        exit 1
    fi
    if [[ ! -s "${xui_script_temp}" ]]; then
        rm -f "${xui_script_temp}"
        echo -e "${red}下载的 x-ui.sh 为空${plain}"
        exit 1
    fi

    # Stop x-ui service and remove old resources
    local custom_bin_backup=""
    if [[ -e ${xui_folder}/ ]]; then
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop
        else
            systemctl stop x-ui
        fi
        # Kill any leftover mtg (MTProto) sidecars. x-ui runs them outside its own
        # lifecycle, so on Linux a stale one can survive the stop and keep holding
        # an inbound port with an outdated secret, silently breaking new clients.
        # The freshly installed panel respawns a clean mtg per inbound on start.
        pkill -f 'mtg-linux-[^ ]* run ' > /dev/null 2>&1 || true
        pkill -f 'tuic-server.*-c .*bin/tuic/tuic_[0-9]+\.json' > /dev/null 2>&1 || true

        # bin/ is about to be wiped wholesale by the tar extraction below. The
        # release only ships known assets (xray/mtg binaries, the bundled
        # geoip*/geosite*.dat sets) -- anything else in bin/ was placed there
        # by the admin (e.g. a hand-added custom geoip/geosite file referenced
        # from a routing rule via "ext:<file>:<code>") and would otherwise be
        # silently deleted on every update, breaking Xray at next start with
        # "failed to open <file>: no such file or directory" for any routing
        # rule that references it. Moved aside rather than copied: a rename
        # on the same filesystem is atomic (no truncated file if disk space
        # runs out mid-copy, unlike `cp`) and keeps the snapshot under
        # /usr/local rather than a separate, possibly small/tmpfs $TMPDIR.
        if [[ -d "${xui_folder}/bin" ]]; then
            custom_bin_backup="${xui_folder%/x-ui}/x-ui-bin-backup.$$"
            rm -rf "${custom_bin_backup}"
            if ! mv "${xui_folder}/bin" "${custom_bin_backup}"; then
                custom_bin_backup=""
                echo -e "${yellow}无法备份 bin/——其中的自定义文件在此次更新中将不会保留${plain}"
            fi
        fi
        # Sole cleanup path for the backup from here on -- covers both the
        # two `exit 1`s below (extraction/binary-missing failures) and an
        # interrupted update (Ctrl-C, signal) before the restore runs.
        # Cleared once the restore below finishes normally.
        trap '[[ -n "${custom_bin_backup}" ]] && rm -rf "${custom_bin_backup}"' EXIT INT TERM
        rm ${xui_folder}/ -rf
    fi

    # Extract resources and set permissions
    tar zxvf x-ui-linux-$(arch).tar.gz
    if [[ $? -ne 0 ]]; then
        rm x-ui-linux-$(arch).tar.gz -f
        rm -f "${xui_script_temp}"
        echo -e "${red}解压 x-ui 发布包失败——旧安装已被删除，在问题解决之前面板无法启动，请重新运行安装脚本${plain}"
        exit 1
    fi
    rm x-ui-linux-$(arch).tar.gz -f

    cd x-ui
    if [[ $? -ne 0 || ! -s x-ui ]]; then
        rm -f "${xui_script_temp}"
        echo -e "${red}解压后的 x-ui 压缩包缺少 x-ui 二进制文件——旧安装已被删除，在问题解决之前面板无法启动，请重新运行安装脚本${plain}"
        exit 1
    fi
    chmod +x x-ui
    chmod +x x-ui.sh

    # Check the system's architecture and rename the file accordingly.
    # The panel binary maps GOARCH=arm to "arm32" (internal/xray/process.go),
    # so the Xray binary must be named xray-linux-arm32; mtg keeps plain "arm".
    if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm32
        chmod +x bin/xray-linux-arm32
        if [[ -f bin/mtg-linux-$(arch) ]]; then
            mv bin/mtg-linux-$(arch) bin/mtg-linux-arm
            chmod +x bin/mtg-linux-arm
        fi
    fi
    chmod +x x-ui bin/xray-linux-$(arch)
    if [[ -f bin/mtg-linux-arm ]]; then
        chmod +x bin/mtg-linux-arm
    elif [[ -f bin/mtg-linux-$(arch) ]]; then
        chmod +x bin/mtg-linux-$(arch)
    fi

    # Restore anything from the old bin/ that the fresh release doesn't ship
    # (custom geoip/geosite files, or anything else an admin hand-placed
    # there) -- never overwrites a same-named file the new release provides,
    # so bundled assets (geoip.dat, geoip_RU.dat, ...) still get the fresh
    # per-release copy. Runs after the arch-rename above so xray-linux-arm32/
    # mtg-linux-arm already exist under their final names there and aren't
    # mistaken for custom files needing a restore. Skips paths the panel
    # itself regenerates at runtime (config.json, mtproto/*.toml -- see
    # internal/xray/process.go, internal/mtproto/manager.go): those aren't
    # admin-placed, and restoring a stale one only resurrects dead state (an
    # orphaned mtg config for a since-deleted inbound) or the wrong
    # directory permissions.
    if [[ -n "${custom_bin_backup}" ]]; then
        local restored_custom_bin=()
        while IFS= read -r -d '' f; do
            local rel="${f#"${custom_bin_backup}"/}"
            case "${rel}" in
                config.json | mtproto | mtproto/* | tuic | tuic/* | tuic-server | tuic-server-*) continue ;;
            esac
            if [[ ! -e "bin/${rel}" ]]; then
                mkdir -p "bin/$(dirname "${rel}")"
                cp -a "${f}" "bin/${rel}"
                restored_custom_bin+=("${rel}")
            fi
        done < <(find "${custom_bin_backup}" \( -type f -o -type l \) -print0)
        rm -rf "${custom_bin_backup}"
        custom_bin_backup=""
        if [[ ${#restored_custom_bin[@]} -gt 0 ]]; then
            echo -e "${green}已恢复本版本未包含的 bin/ 自定义文件：${restored_custom_bin[*]}${plain}"
        fi
    fi
    trap - EXIT INT TERM

    rm -f bin/tuic-server bin/tuic-server-* > /dev/null 2>&1 || true
    rm -rf bin/tuic > /dev/null 2>&1 || true

    # Update x-ui cli and se set permission
    mv -f "${xui_script_temp}" /usr/bin/x-ui
    if [[ $? -ne 0 ]]; then
        rm -f "${xui_script_temp}"
        echo -e "${red}安装 x-ui.sh 失败${plain}"
        exit 1
    fi
    chmod +x /usr/bin/x-ui
    mkdir -p /var/log/x-ui
    config_after_install

    # Etckeeper compatibility
    if [ -d "/etc/.git" ]; then
        if [ -f "/etc/.gitignore" ]; then
            if ! grep -q "x-ui/x-ui.db" "/etc/.gitignore"; then
                echo "" >> "/etc/.gitignore"
                echo "x-ui/x-ui.db" >> "/etc/.gitignore"
                echo -e "${green}已将 x-ui.db 添加到 /etc/.gitignore（用于 etckeeper）${plain}"
            fi
        else
            echo "x-ui/x-ui.db" > "/etc/.gitignore"
            echo -e "${green}已创建 /etc/.gitignore 并添加 x-ui.db（用于 etckeeper）${plain}"
        fi
    fi

    if [[ $release == "alpine" ]]; then
        xui_rc_temp="/etc/init.d/x-ui.tmp.$$"
        rm -f "${xui_rc_temp}"
        curl -fLRo "${xui_rc_temp}" "https://raw.githubusercontent.com/MHSanaei/3x-ui/${script_ref}/x-ui.rc"
        if [[ $? -ne 0 ]]; then
            rm -f "${xui_rc_temp}"
            echo -e "${red}下载 x-ui.rc 失败${plain}"
            exit 1
        fi
        if [[ ! -s "${xui_rc_temp}" ]]; then
            rm -f "${xui_rc_temp}"
            echo -e "${red}下载的 x-ui.rc 为空${plain}"
            exit 1
        fi
        mv -f "${xui_rc_temp}" /etc/init.d/x-ui
        if [[ $? -ne 0 ]]; then
            rm -f "${xui_rc_temp}"
            echo -e "${red}安装 x-ui.rc 失败${plain}"
            exit 1
        fi
        chmod +x /etc/init.d/x-ui
        rc-update add x-ui
        rc-service x-ui start
    else
        # Install systemd service file
        service_installed=false

        if [ -f "x-ui.service" ]; then
            echo -e "${green}在解压文件中找到 x-ui.service，正在安装……${plain}"
            if _install_xui_service_unit "x-ui.service" "false"; then
                service_installed=true
            fi
        fi

        if [ "$service_installed" = false ]; then
            case "${release}" in
                ubuntu | debian | armbian)
                    if [ -f "x-ui.service.debian" ]; then
                        echo -e "${green}在解压文件中找到 x-ui.service.debian，正在安装……${plain}"
                        if _install_xui_service_unit "x-ui.service.debian" "false"; then
                            service_installed=true
                        fi
                    fi
                    ;;
                arch | manjaro | parch)
                    if [ -f "x-ui.service.arch" ]; then
                        echo -e "${green}在解压文件中找到 x-ui.service.arch，正在安装……${plain}"
                        if _install_xui_service_unit "x-ui.service.arch" "false"; then
                            service_installed=true
                        fi
                    fi
                    ;;
                *)
                    if [ -f "x-ui.service.rhel" ]; then
                        echo -e "${green}在解压文件中找到 x-ui.service.rhel，正在安装……${plain}"
                        if _install_xui_service_unit "x-ui.service.rhel" "false"; then
                            service_installed=true
                        fi
                    fi
                    ;;
            esac
        fi

        # If service file not found in tar.gz, download from GitHub
        if [ "$service_installed" = false ]; then
            echo -e "${yellow}在 tar.gz 中未找到服务文件，正在从 GitHub 下载……${plain}"
            case "${release}" in
                ubuntu | debian | armbian)
                    service_unit_url="https://raw.githubusercontent.com/MHSanaei/3x-ui/${script_ref}/x-ui.service.debian"
                    ;;
                arch | manjaro | parch)
                    service_unit_url="https://raw.githubusercontent.com/MHSanaei/3x-ui/${script_ref}/x-ui.service.arch"
                    ;;
                *)
                    service_unit_url="https://raw.githubusercontent.com/MHSanaei/3x-ui/${script_ref}/x-ui.service.rhel"
                    ;;
            esac

            if ! _install_xui_service_unit "$service_unit_url" "true"; then
                echo -e "${red}从 GitHub（${script_ref}）安装 x-ui.service 失败——发布包中也没有提供${plain}"
                exit 1
            fi
            service_installed=true
        fi

        if [ "$service_installed" = true ]; then
            echo -e "${green}正在配置 systemd 服务单元……${plain}"
            chown root:root ${xui_service}/x-ui.service > /dev/null 2>&1
            chmod 644 ${xui_service}/x-ui.service > /dev/null 2>&1
            systemctl daemon-reload
            systemctl enable x-ui
            systemctl start x-ui
        else
            echo -e "${red}安装 x-ui.service 文件失败${plain}"
            exit 1
        fi
    fi

    # IP Limit relies on fail2ban; install + configure it now so the feature
    # works out of the box (no-op when XUI_ENABLE_FAIL2BAN=false). Never fatal.
    setup_fail2ban

    echo -e "${green}x-ui ${tag_version}${plain} 安装完成，正在运行中……"
    echo -e ""
    echo -e "┌───────────────────────────────────────────────────────┐
│  ${blue}x-ui 控制菜单用法（子命令）：                        ${plain}│
│                                                       │
│  ${blue}x-ui${plain}              - 管理脚本                         │
│  ${blue}x-ui start${plain}        - 启动                             │
│  ${blue}x-ui stop${plain}         - 停止                             │
│  ${blue}x-ui restart${plain}      - 重启                             │
│  ${blue}x-ui status${plain}       - 当前状态                         │
│  ${blue}x-ui settings${plain}     - 当前设置                         │
│  ${blue}x-ui enable${plain}       - 开机自启                         │
│  ${blue}x-ui disable${plain}      - 取消开机自启                     │
│  ${blue}x-ui log${plain}          - 查看日志                         │
│  ${blue}x-ui banlog${plain}       - 查看 Fail2ban 封禁日志           │
│  ${blue}x-ui update${plain}       - 更新                             │
│  ${blue}x-ui legacy${plain}       - 旧版本                           │
│  ${blue}x-ui install${plain}      - 安装                             │
│  ${blue}x-ui uninstall${plain}    - 卸载                             │
└───────────────────────────────────────────────────────┘"
}

echo -e "${green}正在运行……${plain}"
install_base
install_x-ui $1
