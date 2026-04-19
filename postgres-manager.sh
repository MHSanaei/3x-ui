#!/bin/bash

set -euo pipefail

release=""
version_id=""

load_os() {
    if [[ -f /etc/os-release ]]; then
        source /etc/os-release
        release="${ID:-}"
        version_id="${VERSION_ID:-}"
        return
    fi
    if [[ -f /usr/lib/os-release ]]; then
        source /usr/lib/os-release
        release="${ID:-}"
        version_id="${VERSION_ID:-}"
        return
    fi
    release="unknown"
    version_id=""
}

load_os

service_name() {
    echo "postgresql"
}

data_dir() {
    case "${release}" in
        arch|manjaro|parch)
            echo "/var/lib/postgres/data"
            ;;
        alpine)
            echo "/var/lib/postgresql/data"
            ;;
        centos|fedora|rhel|rocky|almalinux|ol|amzn|virtuozzo)
            echo "/var/lib/pgsql/data"
            ;;
        *)
            echo "/var/lib/postgresql/data"
            ;;
    esac
}

require_root() {
    if [[ "${EUID}" -ne 0 ]]; then
        echo "This command requires root privileges" >&2
        exit 1
    fi
}

is_installed() {
    command -v psql >/dev/null 2>&1
}

is_running() {
    local service
    service="$(service_name)"
    if command -v systemctl >/dev/null 2>&1; then
        systemctl is-active --quiet "${service}" && return 0 || return 1
    fi
    if command -v rc-service >/dev/null 2>&1; then
        rc-service "${service}" status >/dev/null 2>&1 && return 0 || return 1
    fi
    return 1
}

start_service() {
    local service
    service="$(service_name)"
    if command -v systemctl >/dev/null 2>&1; then
        systemctl daemon-reload >/dev/null 2>&1 || true
        systemctl enable "${service}" >/dev/null 2>&1 || true
        systemctl start "${service}"
        return
    fi
    if command -v rc-service >/dev/null 2>&1; then
        rc-update add "${service}" >/dev/null 2>&1 || true
        rc-service "${service}" start
        return
    fi
    echo "No supported service manager found" >&2
    exit 1
}

run_as_postgres() {
    if command -v runuser >/dev/null 2>&1; then
        runuser -u postgres -- "$@"
        return
    fi
    local quoted=""
    for arg in "$@"; do
        quoted+=" $(printf "%q" "$arg")"
    done
    su - postgres -c "${quoted# }"
}

escape_sql_ident() {
    local value="$1"
    value="${value//\"/\"\"}"
    printf '%s' "${value}"
}

escape_sql_literal() {
    local value="$1"
    value="${value//\'/\'\'}"
    printf '%s' "${value}"
}

install_packages() {
    require_root
    case "${release}" in
        ubuntu|debian|armbian)
            apt-get update
            DEBIAN_FRONTEND=noninteractive apt-get install -y postgresql postgresql-client
            ;;
        fedora|rhel|rocky|almalinux|ol|amzn|virtuozzo)
            dnf -y install postgresql-server postgresql
            ;;
        centos)
            if [[ "${version_id}" =~ ^7 ]]; then
                yum -y install postgresql-server postgresql
            else
                dnf -y install postgresql-server postgresql
            fi
            ;;
        arch|manjaro|parch)
            pacman -Syu --noconfirm postgresql
            ;;
        alpine)
            apk add --no-cache postgresql postgresql-client
            ;;
        *)
            echo "Unsupported OS for automatic PostgreSQL installation: ${release}" >&2
            exit 1
            ;;
    esac
}

init_local() {
    require_root
    local pg_data
    pg_data="$(data_dir)"

    if ! is_installed; then
        install_packages
    fi

    case "${release}" in
        fedora|rhel|rocky|almalinux|ol|amzn|virtuozzo|centos)
            if [[ ! -f "${pg_data}/PG_VERSION" ]]; then
                if command -v postgresql-setup >/dev/null 2>&1; then
                    postgresql-setup --initdb
                elif [[ -x /usr/bin/postgresql-setup ]]; then
                    /usr/bin/postgresql-setup --initdb
                fi
            fi
            ;;
        arch|manjaro|parch|alpine)
            mkdir -p "${pg_data}"
            chown -R postgres:postgres "$(dirname "${pg_data}")" "${pg_data}" >/dev/null 2>&1 || true
            if [[ ! -f "${pg_data}/PG_VERSION" ]]; then
                run_as_postgres initdb -D "${pg_data}"
            fi
            ;;
        *)
            ;;
    esac

    start_service
}

create_db_user() {
    require_root
    local db_user=""
    local db_password=""
    local db_name=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --user)
                db_user="$2"
                shift 2
                ;;
            --password)
                db_password="$2"
                shift 2
                ;;
            --db)
                db_name="$2"
                shift 2
                ;;
            *)
                echo "Unknown argument: $1" >&2
                exit 1
                ;;
        esac
    done

    if [[ -z "${db_user}" || -z "${db_name}" ]]; then
        echo "Both --user and --db are required" >&2
        exit 1
    fi

    init_local

    local safe_user safe_password safe_db
    safe_user="$(escape_sql_ident "${db_user}")"
    safe_password="$(escape_sql_literal "${db_password}")"
    safe_db="$(escape_sql_ident "${db_name}")"

    if [[ -z "$(run_as_postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='$(escape_sql_literal "${db_user}")'")" ]]; then
        if [[ -n "${db_password}" ]]; then
            run_as_postgres psql -v ON_ERROR_STOP=1 -c "CREATE ROLE \"${safe_user}\" LOGIN PASSWORD '${safe_password}';"
        else
            run_as_postgres psql -v ON_ERROR_STOP=1 -c "CREATE ROLE \"${safe_user}\" LOGIN;"
        fi
    elif [[ -n "${db_password}" ]]; then
        run_as_postgres psql -v ON_ERROR_STOP=1 -c "ALTER ROLE \"${safe_user}\" WITH PASSWORD '${safe_password}';"
    fi

    if [[ -z "$(run_as_postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='$(escape_sql_literal "${db_name}")'")" ]]; then
        run_as_postgres psql -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"${safe_db}\" OWNER \"${safe_user}\";"
    fi
}

show_status() {
    if is_installed; then
        echo "installed=true"
    else
        echo "installed=false"
    fi
    if is_running; then
        echo "running=true"
    else
        echo "running=false"
    fi
    echo "service=$(service_name)"
    echo "data_dir=$(data_dir)"
}

command="${1:-status}"
if [[ $# -gt 0 ]]; then
    shift
fi

case "${command}" in
    status)
        show_status
        ;;
    install)
        install_packages
        init_local
        show_status
        ;;
    init-local)
        init_local
        show_status
        ;;
    create-db-user)
        create_db_user "$@"
        show_status
        ;;
    *)
        echo "Unsupported command: ${command}" >&2
        exit 1
        ;;
esac
