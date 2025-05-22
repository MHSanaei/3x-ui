#!/bin/sh

# Function to create database environment file
create_db_env() {
    mkdir -p /etc/x-ui
    cat > /etc/x-ui/db.env << EOF
DB_TYPE=${DB_TYPE:-sqlite}
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_NAME=${DB_NAME:-x_ui}
DB_USER=${DB_USER:-x_ui}
DB_PASSWORD=${DB_PASSWORD}
DB_SSLMODE=${DB_SSLMODE:-disable}
DB_TIMEZONE=${DB_TIMEZONE:-UTC}
EOF
    chmod 600 /etc/x-ui/db.env
}

# Function to wait for PostgreSQL
wait_for_postgres() {
    if [ "$DB_TYPE" = "postgres" ]; then
        echo "Waiting for PostgreSQL to be ready..."
        /app/wait-for-postgres.sh "$DB_HOST" "$DB_PORT" "$DB_USER" "$DB_NAME"
        echo "PostgreSQL is ready!"
    fi
}

# Function to test database connection
test_db_connection() {
    if [ "$DB_TYPE" = "postgres" ]; then
        echo "Testing PostgreSQL connection..."
        if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" > /dev/null 2>&1; then
            echo "PostgreSQL connection successful!"
        else
            echo "ERROR: Cannot connect to PostgreSQL database!"
            echo "Please check your database configuration:"
            echo "  DB_HOST: $DB_HOST"
            echo "  DB_PORT: $DB_PORT"
            echo "  DB_NAME: $DB_NAME"
            echo "  DB_USER: $DB_USER"
            exit 1
        fi
    else
        echo "Using SQLite database"
    fi
}

# Create database environment file
create_db_env

# Wait for PostgreSQL if needed
wait_for_postgres

# Test database connection
test_db_connection

# Start fail2ban
[ "$XUI_ENABLE_FAIL2BAN" = "true" ] && fail2ban-client -x start

# Run x-ui
exec /app/x-ui
