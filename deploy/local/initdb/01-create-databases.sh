#!/bin/bash
# Runs once on first start of an empty Postgres data dir (docker-entrypoint-initdb.d).
# One database per service.
set -euo pipefail

for db in orders payments inventory pricing; do
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -c "CREATE DATABASE ${db};"
done
