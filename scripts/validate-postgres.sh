#!/usr/bin/env bash
# Validates the Postgres manifests and chart wiring (T-024): kubeconform --strict (pinned CNPG and External
# Secrets schemas), isolation and ExternalSecret assertions on the parsed CRs, chart secretKeyRef-only
# assertions, docs headings, and a self-test where every mutation of a scratch copy must be detected.
# Requires helm 3, kubeconform v0.8.0, python3 (stdlib). NETWORK is needed on the first run for the CRD
# schemas (pinned by commit); they are cached in $KUBECONFORM_CACHE afterwards.
set -euo pipefail

HELM_MAJOR="3"
KUBECONFORM_VERSION="v0.8.0"
KUBERNETES_VERSION="1.29.0"
# datreeio/CRDs-catalog pinned by commit (CloudNativePG cluster/pooler/database v1, External Secrets v1beta1).
CRD_CATALOG_SHA="501527ab9c40c827144c8f45945b25a023e344ce"
CRD_SCHEMA="https://raw.githubusercontent.com/datreeio/CRDs-catalog/${CRD_CATALOG_SHA}/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json"
KUBECONFORM_CACHE="${KUBECONFORM_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/opop-kubeconform}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PG="$ROOT/deploy/postgres"
APP="$ROOT/deploy/charts/example-service"
LIB="$ROOT/deploy/charts/service-base"
DOC="$ROOT/docs/postgres.md"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok:   $*"; }

command -v helm >/dev/null || fail "helm not found (need Helm ${HELM_MAJOR}.x)"
command -v kubeconform >/dev/null || fail "kubeconform not found (need ${KUBECONFORM_VERSION})"
command -v python3 >/dev/null || fail "python3 not found"
helm version --short | grep -q "^v${HELM_MAJOR}\." || fail "Helm ${HELM_MAJOR}.x required"
kubeconform -v | grep -q "${KUBECONFORM_VERSION}" || fail "kubeconform ${KUBECONFORM_VERSION} required, got: $(kubeconform -v)"
mkdir -p "$KUBECONFORM_CACHE"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# The block-YAML parser is reused from validate-charts.sh (same repo, tested there by its own self-test)
# so there is one parser and no PyYAML dependency. Only the parser section is extracted.
read -r -d '' PYCHECK <<'PYEOF' || true
import re, sys, json, os, glob
src = open(os.path.join(os.environ['ROOT'], 'scripts', 'validate-charts.sh')).read()
start = src.index("KEY = re.compile")
end = src.index("def baseline(")
exec(src[start:end])

def load(directory):
    docs = []
    for f in sorted(glob.glob(os.path.join(directory, '*.yaml'))):
        docs += parse_docs(open(f).read())
    return docs

def of(docs, kind):
    return [d for d in docs if isinstance(d, dict) and d.get('kind') == kind]

def isolation(docs):
    errs = []
    def need(ok, msg):
        if not ok:
            errs.append(msg)
    clusters = of(docs, 'Cluster')
    need(len(clusters) == 1, 'exactly one Cluster expected')
    if len(clusters) != 1:
        return errs
    cl = clusters[0]
    cname = get(cl, 'metadata.name')
    roles = get(cl, 'spec.managed.roles')
    roles = roles if isinstance(roles, list) else []
    names = [r.get('name') for r in roles]
    need(len(names) >= 2 and len(set(names)) == len(names), 'roles must be distinct (one per service)')
    for r in roles:
        n = r.get('name')
        need(r.get('login') is True, 'role %s must login' % n)
        for flag in ('superuser', 'createdb', 'createrole', 'replication'):
            need(r.get(flag) is False, 'role %s must set %s: false' % (n, flag))
        need('inRoles' not in r, 'role %s must not be a member of other roles' % n)
        need(get(r, 'passwordSecret.name') == n + '-db', 'role %s passwordSecret must be %s-db' % (n, n))
    # databases: bootstrap database plus Database CRs; each owned by the role of the same name
    dbs = {}
    boot = get(cl, 'spec.bootstrap.initdb')
    need(isinstance(boot, dict) and boot.get('database') == boot.get('owner'), 'bootstrap database owner must match its name')
    if isinstance(boot, dict):
        dbs[boot.get('database')] = boot.get('owner')
    for d in of(docs, 'Database'):
        need(get(d, 'spec.cluster.name') == cname, 'Database %s must reference Cluster %s' % (get(d, 'metadata.name'), cname))
        need(get(d, 'spec.ensure') == 'present', 'Database %s must be ensure: present' % get(d, 'spec.name'))
        need(get(d, 'spec.name') not in dbs, 'duplicate database %s' % get(d, 'spec.name'))
        dbs[get(d, 'spec.name')] = get(d, 'spec.owner')
    need(sorted(dbs) == sorted(names), 'databases %s must match roles %s one to one' % (sorted(dbs), sorted(names)))
    for db, owner in dbs.items():
        need(db == owner, 'database %s must be owned by its own role, not %s' % (db, owner))
    # pg_hba: TLS-only, exactly one rule per role restricted to its own database; the final rule rejects all else
    hba = get(cl, 'spec.postgresql.pg_hba')
    hba = hba if isinstance(hba, list) else []
    need(bool(hba) and hba[-1].split() == ['host', 'all', 'all', 'all', 'reject'], 'pg_hba must end with "host all all all reject"')
    for n in names:
        mine = [h for h in hba[:-1] if h.split()[1:3] == [n, n]]
        need(len(mine) == 1 and mine[0].split()[0] == 'hostssl' and mine[0].split()[-1] == 'scram-sha-256',
             'pg_hba must allow %s only to database %s over hostssl with scram-sha-256' % (n, n))
    need(len(hba) - 1 == len(names), 'pg_hba may contain only per-role rules before the final reject')
    need(re.search(r':[0-9][\w.-]*$', str(get(cl, 'spec.imageName'))) is not None, 'Cluster imageName must pin a version tag')
    bs = get(boot, 'secret.name') if isinstance(boot, dict) else None
    need(bs == str(get(boot, 'owner')) + '-db', 'bootstrap secret must be the owner role\'s own secret')
    # External secrets: one per service; every templated value is a pure reference to the right own key
    ess = {get(e, 'metadata.name'): e for e in of(docs, 'ExternalSecret')}
    need(sorted(ess) == sorted(n + '-db' for n in names), 'one ExternalSecret <service>-db per service')
    want = {'username': 'user', 'DB_USER': 'user', 'password': 'password', 'DB_PASSWORD': 'password',
            'DB_URL': 'url', 'DB_MIGRATION_URL': 'migration_url'}
    keys, stores = [], set()
    for n, e in ess.items():
        need(get(e, 'spec.target.name') == n, '%s must target Secret %s' % (n, n))
        stores.add(get(e, 'spec.secretStoreRef.name'))
        tpl = get(e, 'spec.target.template.data')
        need(isinstance(tpl, dict) and tpl == {k: '{{ .%s }}' % v for k, v in want.items()},
             '%s template must map each key to exactly its own {{ .key }} reference (no literals, no cross-wiring)' % n)
        need(get(e, 'spec.data') is MISSING, '%s must not carry literal data items' % n)
        ex = get(e, 'spec.dataFrom') or []
        need(len(ex) == 1 and set(ex[0]) == {'extract'} and set(ex[0]['extract']) == {'key'}, '%s must extract exactly one cloud secret' % n)
        keys.append(get(ex[0], 'extract.key') if ex else None)
        need(n[:-3] in str(keys[-1]), '%s cloud secret key must be service specific' % n)
    need(len(set(keys)) == len(ess), 'each service must use its own single cloud secret key (no shared credentials)')
    need(stores == {'cloud-secrets'}, 'all ExternalSecrets must use the cloud-secrets store')
    poolers = of(docs, 'Pooler')
    need(len(poolers) == 1 and get(poolers[0], 'spec.cluster.name') == cname and get(poolers[0], 'spec.type') == 'rw',
         'one rw Pooler for the Cluster expected')
    if len(poolers) == 1:
        po = poolers[0]
        par = get(po, 'spec.pgbouncer.parameters') or {}
        inst, pool = get(po, 'spec.instances'), int(par.get('default_pool_size', 0))
        need(get(po, 'spec.pgbouncer.poolMode') == 'transaction', 'Pooler must use transaction pool mode')
        need(isinstance(inst, int) and inst >= 2, 'Pooler needs at least 2 instances')
        need(1 <= pool <= 50, 'default_pool_size must be between 1 and 50')
        need(int(par.get('max_prepared_statements', 0)) > 0, 'max_prepared_statements must be set (transaction pooling)')
        img = str(get(po, 'spec.template.spec.containers.0.image'))
        ver = re.search(r':(\d+)\.(\d+)', img)
        need(ver is not None and (int(ver.group(1)), int(ver.group(2))) >= (1, 21), 'PgBouncer image must be pinned to >= 1.21')
        for r in roles:
            need(isinstance(r.get('connectionLimit'), int) and isinstance(inst, int) and r['connectionLimit'] > inst * pool,
                 'role %s connectionLimit must exceed pooler instances x default_pool_size' % r.get('name'))
    return errs

def chart(docs, secret, names):
    errs = []
    dep = [d for d in docs if isinstance(d, dict) and d.get('kind') == 'Deployment']
    env = get(dep[0], 'spec.template.spec.containers.0.env') if dep else MISSING
    if not isinstance(env, list):
        return ['no env rendered']
    for name in names:
        hits = [e for e in env if e.get('name') == name]
        if len(hits) != 1:
            errs.append('%s must appear exactly once' % name)
            continue
        ref = get(hits[0], 'valueFrom.secretKeyRef')
        if 'value' in hits[0] or not isinstance(ref, dict):
            errs.append('%s must use secretKeyRef only' % name)
        elif ref.get('name') != secret or ref.get('key') != name:
            errs.append('%s must reference secret %s key %s' % (name, secret, name))
    return errs

def main(argv):
    mode = argv[1]
    if mode == 'mutate':
        path, old, new = argv[2], argv[3], argv[4]
        text = open(path).read()
        if old not in text:
            print('mutation target not found: ' + old, file=sys.stderr)
            return 2
        open(path, 'w').write(text.replace(old, new, 1))
        return 0
    errs = isolation(load(argv[2])) if mode == 'isolation' else chart(parse_docs(sys.stdin.read()), argv[2], argv[3:])
    for e in errs:
        print('FAIL: ' + e)
    return 1 if errs else 0

try:
    sys.exit(main(sys.argv))
except ValueError as e:
    print('FAIL: could not parse YAML: %s' % e, file=sys.stderr)
    sys.exit(1)
PYEOF
py() { ROOT="$ROOT" python3 -I -c "$PYCHECK" "$@"; }

# 1. kubeconform --strict ------------------------------------------------------------------
KC=(kubeconform -strict -summary -kubernetes-version "$KUBERNETES_VERSION" -cache "$KUBECONFORM_CACHE"
    -schema-location default -schema-location "$CRD_SCHEMA")
"${KC[@]}" "$PG"/*.yaml || fail "kubeconform --strict on deploy/postgres (needs network on first run for CRD schemas)"
pass "kubeconform --strict (CloudNativePG + External Secrets manifests)"

# Chart renders go through a scratch copy so helm dependency update leaves the tree untouched.
mkdir -p "$TMP/charts"
cp -R "$LIB" "$APP" "$TMP/charts/"
helm dependency update "$TMP/charts/example-service" >/dev/null
render() { helm template rel "$TMP/charts/example-service" "$@"; }
render --set database.enabled=true --set database.secretName=orders-db | "${KC[@]}" - \
  || fail "kubeconform --strict on chart render with database.enabled"
pass "kubeconform --strict (service-base render with database.enabled)"

# 2. Isolation ---------------------------------------------------------------------------------
py isolation "$PG" || fail "database/role isolation assertions"
pass "one database and one role per service; no cross-database access; no shared or plaintext credentials"

# 3. Chart wiring -----------------------------------------------------------------------------------
for svc in orders payments inventory pricing; do
  render --set database.enabled=true --set "database.secretName=${svc}-db" | py chart "${svc}-db" DB_URL DB_USER DB_PASSWORD \
    || fail "chart DB env for ${svc}"
done
pass "chart renders DB_URL/DB_USER/DB_PASSWORD from secretKeyRef only (all four services)"
render --set database.enabled=true --set database.migration=true --set database.secretName=orders-db \
  | py chart orders-db DB_URL DB_USER DB_PASSWORD DB_MIGRATION_URL || fail "chart DB_MIGRATION_URL env"
pass "database.migration renders DB_MIGRATION_URL from secretKeyRef"
! render | grep -q "DB_URL" || fail "DB env rendered although database is not enabled"
pass "no DB env when database is not enabled"
render --set database.enabled=true >/dev/null 2>&1 && fail "database.enabled without secretName must fail"
render --set database.enabled=true --set database.secretName=x --set 'env[0].name=DB_PASSWORD' --set 'env[0].value=p' >/dev/null 2>&1 \
  && fail "plaintext DB_PASSWORD env must fail the render"
pass "missing secretName and plaintext DB_* env are rejected"

# 4. Docs --------------------------------------------------------------------------------------------
for topic in "managed" "pooler" "migration job" "private network" "backup"; do
  grep -qiE "^#+ .*${topic}" "$DOC" || fail "docs/postgres.md has no heading about: ${topic}"
done
pass "docs/postgres.md covers managed production, pooler, migration job, private networking and backups"

# 5. Self-test: every mutation must be caught ------------------------------------------------------------
mutate_pg() { # <file>|<old>|<new>: each mutation, applied alone to a scratch copy, must be detected
  local f o n
  IFS='|' read -r f o n <<<"$1"
  rm -rf "$TMP/m" && cp -R "$PG" "$TMP/m"
  py mutate "$TMP/m/$f" "$o" "$n" || fail "mutation target missing: $o"
  ! py isolation "$TMP/m" >/dev/null || fail "isolation check missed mutation: $f '$o' -> '$n'"
}
MUTATIONS=(
  $'cluster.yaml|      - hostssl payments payments all scram-sha-256\n|'
  "cluster.yaml|      - host all all all reject|      - host all all all scram-sha-256"
  "cluster.yaml|      - hostssl payments payments all|      - hostssl orders payments all"
  "cluster.yaml|      - hostssl payments payments|      - host payments payments"
  "cluster.yaml|superuser: false|superuser: true"
  "cluster.yaml|        name: orders-db|        name: payments-db"
  "cluster.yaml|postgresql:16.4|postgresql:latest"
  "cluster.yaml|connectionLimit: 30|connectionLimit: 20"
  "databases.yaml|  owner: payments|  owner: orders"
  "databases.yaml|  name: pricing|  name: orders"
  "externalsecrets.yaml|key: opop/dev/payments/db|key: opop/dev/orders/db"
  "externalsecrets.yaml|name: payments-db|name: shared-db"
  "externalsecrets.yaml|username: \"{{ .user }}\"|username: \"hunter2\""
  "externalsecrets.yaml|DB_USER: \"{{ .user }}\"|DB_USER: \"{{ .url }}\""
  "externalsecrets.yaml|DB_MIGRATION_URL: \"{{ .migration_url }}\"|DB_MIGRATION_URL: \"{{ .url }}\""
  "externalsecrets.yaml|name: cloud-secrets|name: other-store"
  "pooler.yaml|type: rw|type: ro"
  "pooler.yaml|poolMode: transaction|poolMode: session"
  "pooler.yaml|instances: 2|instances: 0"
  "pooler.yaml|default_pool_size: \"10\"|default_pool_size: \"500\""
  "pooler.yaml|max_prepared_statements: \"100\"|max_prepared_statements: \"0\""
  "pooler.yaml|pgbouncer:1.24.1|pgbouncer:1.20.1"
)
for m in "${MUTATIONS[@]}"; do mutate_pg "$m"; done
pass "self-test: ${#MUTATIONS[@]} isolation mutations all detected"

BAD="$(render --set database.enabled=true --set database.secretName=orders-db \
  | perl -0pe 's/( *)valueFrom:\n\s+secretKeyRef:\n\s+name: [^\n]+\n\s+key: [^\n]+/$1value: plain/g')"
! py chart orders-db DB_URL DB_USER DB_PASSWORD <<<"$BAD" >/dev/null || fail "chart check missed a plaintext DB env mutation"
pass "self-test: chart secretKeyRef check detects plaintext values"
echo "validate-postgres: all checks passed"
