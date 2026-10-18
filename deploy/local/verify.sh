#!/usr/bin/env bash
# Verifies the local dev stack: the four databases (psql), Kafka topic list, Redis ping.
# Usage: deploy/local/verify.sh   (stack must be up: `make dev-up`)
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
DC=(docker compose -f "$DIR/docker-compose.yml" --env-file "$DIR/.env.example")
# shellcheck disable=SC1091
set -a; . "$DIR/.env.example"; set +a

fail=0
ok()  { echo "ok   $*"; }
bad() { echo "FAIL $*"; fail=1; }

for db in orders payments inventory pricing; do
  if "${DC[@]}" exec -T postgres psql -U "$POSTGRES_USER" -d "$db" -tAc 'SELECT 1' | grep -q 1; then
    ok "postgres database '$db' exists"
  else
    bad "postgres database '$db' missing"
  fi
done

if "${DC[@]}" exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:29092 --list >/dev/null; then
  ok "kafka topic list"
else
  bad "kafka topic list"
fi

if [ "$("${DC[@]}" exec -T redis redis-cli ping | tr -d '\r')" = "PONG" ]; then
  ok "redis ping"
else
  bad "redis ping"
fi

exit "$fail"
