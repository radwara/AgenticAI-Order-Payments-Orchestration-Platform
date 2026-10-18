#!/usr/bin/env bash
# Verifies that the local dev stack left no ports bound on 127.0.0.1 after `make dev-down`.
# Ports come from .env.example (5432 / 9092 / 6379 by default).
#
# - Retries up to 10 x 0.5s per port: Docker Desktop can release published ports a moment
#   after `compose down` returns.
# - If a port is still held, the holder (command and PID, via lsof) is printed.
#   FAIL (exit 1) when the holder is docker/compose related (docker, containerd, vpnkit,
#   gvproxy, rootlesskit, com.docker...) or when the holder cannot be identified (strict by
#   default: unknown is never treated as safe). WARN (exit 0) only when lsof positively
#   identifies a holder that is not docker related, e.g. a locally installed Postgres.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
set -a; . "$DIR/.env.example"; set +a

DOCKER_RE='docker|containerd|vpnkit|gvproxy|rootlesskit|slirp4netns|lima|colima|orbstack|podman'

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

# Prints "<command> (pid <pid>)" lines for listeners on the port; empty if unknown.
holders() {
  command -v lsof >/dev/null 2>&1 || return 0
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -Fpc 2>/dev/null | awk '
    /^p/ { pid = substr($0, 2) }
    /^c/ { print substr($0, 2) " (pid " pid ")" }' | sort -u
}

fail=0
for pair in "postgres:$POSTGRES_PORT" "kafka:$KAFKA_PORT" "redis:$REDIS_PORT"; do
  name=${pair%%:*}; port=${pair##*:}
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    is_bound "$port" || break
    sleep 0.5
  done
  if ! is_bound "$port"; then
    echo "ok   port $port ($name) is free"
    continue
  fi
  who="$(holders "$port")"
  if [ -z "$who" ]; then
    echo "FAIL port $port ($name) is still bound by an unidentified process (lsof unavailable or not permitted)"
    fail=1
  elif printf '%s\n' "$who" | grep -Eiq "$DOCKER_RE"; then
    echo "FAIL port $port ($name) is still held by a docker process: $(printf '%s' "$who" | tr '\n' ' ')"
    fail=1
  else
    echo "WARN port $port ($name) is held by an unrelated process, not the stack: $(printf '%s' "$who" | tr '\n' ' ')"
  fi
done
exit "$fail"
