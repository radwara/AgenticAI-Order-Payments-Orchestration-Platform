#!/usr/bin/env bash
# Lint the Order API v1 contract and check it against SPEC-02.
#  1. redocly lint (pinned version, run via npx)
#  2. all four operations exist, each with a 2xx and an error response
#  3. Order state enum equals the SPEC-02 states exactly
#  4. Guarded contract rules: 409 on cancel, 422 on create, Money as integer minor
#     units with minimum 0, required Idempotency-Key, problem+json errors, list limit
#     default 50 / max 200 (T-013), server-authoritative prices, quantity >= 1
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

paths = doc.get("paths", {})
schemas = doc.get("components", {}).get("schemas", {})

def resolve(node):
    # the bundle keeps component $refs; resolve local ones
    while isinstance(node, dict) and "$ref" in node:
        cur = doc
        for part in node["$ref"].lstrip("#/").split("/"):
            cur = cur.get(part, {}) if isinstance(cur, dict) else {}
        node = cur
    return node

# status codes
def has_resp(path, method, code):
    return code in paths.get(path, {}).get(method, {}).get("responses", {})
if not has_resp("/v1/orders/{id}/cancel", "post", "409"):
    errors.append("POST /v1/orders/{id}/cancel: 409 response missing")
if not has_resp("/v1/orders", "post", "422"):
    errors.append("POST /v1/orders: 422 response missing")

# every 4xx/5xx response is application/problem+json only
for path, item in paths.items():
    for method, op in item.items():
        if not isinstance(op, dict) or "responses" not in op:
            continue
        for code, r in op["responses"].items():
            if code[0] in "45":
                media = list(resolve(r).get("content", {}))
                if media != ["application/problem+json"]:
                    errors.append(f"{method.upper()} {path} {code}: media types {media}, want ['application/problem+json']")

# Idempotency-Key required header on create
params = [resolve(p) for p in paths.get("/v1/orders", {}).get("post", {}).get("parameters", [])]
idem = [p for p in params if p.get("name") == "Idempotency-Key" and p.get("in") == "header"]
if not idem:
    errors.append("POST /v1/orders: Idempotency-Key header parameter missing")
elif idem[0].get("required") is not True:
    errors.append("POST /v1/orders: Idempotency-Key must be required: true")

# Money: integer minor units (int64, >= 0) plus ISO-4217 currency
money = schemas.get("Money", {})
amt = money.get("properties", {}).get("amount_minor", {})
if amt.get("type") != "integer" or amt.get("format") != "int64":
    errors.append(f"Money.amount_minor must be integer/int64, got {amt.get('type')}/{amt.get('format')}")
if not isinstance(amt.get("minimum"), int) or amt.get("minimum") < 0:
    errors.append("Money.amount_minor must declare minimum >= 0")
cur = money.get("properties", {}).get("currency", {})
if cur.get("type") != "string" or cur.get("pattern") != "^[A-Z]{3}$":
    errors.append("Money.currency must be string with pattern ^[A-Z]{3}$")
if sorted(money.get("required", [])) != ["amount_minor", "currency"]:
    errors.append("Money must require amount_minor and currency")

# list limit: default 50, max 200 (T-013)
lim = [p for p in paths.get("/v1/orders", {}).get("get", {}).get("parameters", [])
       if resolve(p).get("name") == "limit"]
ls = resolve(lim[0]).get("schema", {}) if lim else {}
if (ls.get("default"), ls.get("maximum"), ls.get("minimum")) != (50, 200, 1):
    errors.append(f"list limit default/maximum/minimum must be 50/200/1 (T-013), got {ls.get('default')}/{ls.get('maximum')}/{ls.get('minimum')}")

# server-authoritative prices and quantity bounds
req_item = resolve(schemas.get("CreateOrderRequest", {}).get("properties", {}).get("items", {}).get("items", {}))
if "unit_price" in req_item.get("properties", {}):
    errors.append("create request items must not accept unit_price (server-authoritative pricing)")
if req_item.get("additionalProperties") is not False:
    errors.append("create request item must set additionalProperties: false")
if req_item.get("properties", {}).get("quantity", {}).get("minimum", 0) < 1:
    errors.append("create request item quantity must declare minimum >= 1")
if schemas.get("OrderItem", {}).get("properties", {}).get("quantity", {}).get("minimum", 0) < 1:
    errors.append("OrderItem.quantity must declare minimum >= 1")
up = schemas.get("OrderItem", {}).get("properties", {}).get("unit_price", {})
if up.get("readOnly") is not True:
    errors.append("OrderItem.unit_price must be readOnly: true")

if errors:
    print("\n".join("FAIL: " + e for e in errors), file=sys.stderr)
    sys.exit(1)
print("OK: operations, OrderState enum, error codes, Money, Idempotency-Key, limit and pricing rules")
PY
echo "lint-openapi: PASS"
