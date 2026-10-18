#!/usr/bin/env bash
# Verifies that the local dev stack left no ports bound on 127.0.0.1 after `make dev-down`.
# Ports come from .env.example (5432 / 9092 / 6379 by default). Exits non-zero if any is bound.
# Uses lsof if present, else nc, else bash's /dev/tcp (a successful TCP connect means bound).
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
set -a; . "$DIR/.env.example"; set +a

is_bound() {
  local port=$1
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
  elif command -v nc >/dev/null 2>&1; then
    nc -z 127.0.0.1 "$port" >/dev/null 2>&1
  else
    (exec 3<>"/dev/tcp/127.0.0.1/$port") >/dev/null 2>&1
  fi
}

fail=0
for pair in "postgres:$POSTGRES_PORT" "kafka:$KAFKA_PORT" "redis:$REDIS_PORT"; do
  name=${pair%%:*}; port=${pair##*:}
  if is_bound "$port"; then
    echo "FAIL port $port ($name) is still bound; inspect with: lsof -nP -iTCP:$port -sTCP:LISTEN"
    fail=1
  else
    echo "ok   port $port ($name) is free"
  fi
done
exit "$fail"
