#!/bin/bash
set -e

host="$1"
port="$2"
user="$3"
database="$4"
shift 4
cmd="$@"

until PGPASSWORD="$DB_PASSWORD" psql -h "$host" -p "$port" -U "$user" -d "$database" -c '\q'; do
  >&2 echo "PostgreSQL is unavailable - sleeping"
  sleep 1
done

>&2 echo "PostgreSQL is up - executing command"
exec $cmd 