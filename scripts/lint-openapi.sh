#!/usr/bin/env bash
# Lint the Order API v1 contract and check it against SPEC-02.
#  1. redocly lint (pinned version, run via npx)
#  2. all four operations exist, each with a 2xx and an error response
#  3. Order state enum equals the SPEC-02 states exactly
set -euo pipefail

REDOCLY_VERSION="2.64.1"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SPEC_FILE="$ROOT/contracts/order/v1/openapi.yaml"
SPEC02="$ROOT/specs/SPEC-02-order.md"

[ -f "$SPEC_FILE" ] || { echo "FAIL: $SPEC_FILE not found" >&2; exit 1; }

echo "==> redocly lint"
npx --yes "@redocly/cli@${REDOCLY_VERSION}" lint "$SPEC_FILE"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
npx --yes "@redocly/cli@${REDOCLY_VERSION}" bundle "$SPEC_FILE" --ext json -o "$tmp/openapi.json" >/dev/null

echo "==> contract checks"
python3 -I - "$tmp/openapi.json" "$SPEC02" <<'PY'
import json, re, sys

doc = json.load(open(sys.argv[1]))
spec02 = open(sys.argv[2]).read()
errors = []

required = [
    ("/v1/orders", "post"),
    ("/v1/orders/{id}", "get"),
    ("/v1/orders", "get"),
    ("/v1/orders/{id}/cancel", "post"),
]
for path, method in required:
    op = doc.get("paths", {}).get(path, {}).get(method)
    if op is None:
        errors.append(f"missing operation {method.upper()} {path}")
        continue
    codes = list(op.get("responses", {}))
    if not any(c.startswith("2") for c in codes):
        errors.append(f"{method.upper()} {path}: no 2xx response")
    if not any(c[0] in "45" for c in codes):
        errors.append(f"{method.upper()} {path}: no error response")

m = re.search(r"^- States:\s*(.+?)\.\s*Transitions", spec02, re.M)
if not m:
    errors.append("could not find States line in SPEC-02")
else:
    spec_states = [s.strip() for s in m.group(1).split(",")]
    enum = doc.get("components", {}).get("schemas", {}).get("OrderState", {}).get("enum")
    if enum is None:
        errors.append("components.schemas.OrderState.enum missing")
    elif enum != spec_states:
        errors.append(f"OrderState enum {enum} != SPEC-02 states {spec_states}")

if errors:
    print("\n".join("FAIL: " + e for e in errors), file=sys.stderr)
    sys.exit(1)
print("OK: operations present, OrderState enum matches SPEC-02")
PY
echo "lint-openapi: PASS"
