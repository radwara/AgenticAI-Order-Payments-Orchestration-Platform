#!/usr/bin/env bash
# Validates the Helm charts under deploy/charts (T-018).
#   1. helm lint (service-base, example-service)
#   2. helm template + kubeconform --strict
#   3. STRUCTURAL assertions on the parsed rendered manifests (Deployment pod/container
#      securityContext, PDB, NetworkPolicy policyTypes/rules, ...), not text greps
#   4. override renders (replicas, resources, image tag, image digest, HPA, null values)
#   5. guard renders: insecure values must make `helm template` fail
#   6. self-test: mutate a scratch copy of the charts; every mutation must make this script's
#      own checks fail (proves the assertions are not vacuous)
# Requires: helm 3, kubeconform (pinned below), python3 (stdlib only). Nothing is installed by this script.
set -euo pipefail

# Pinned tool versions. The script fails if the installed tools differ, so CI and laptops agree.
HELM_MAJOR="3"
KUBECONFORM_VERSION="v0.8.0"
KUBERNETES_VERSION="1.29.0"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$ROOT/deploy/charts/service-base"
APP="$ROOT/deploy/charts/example-service"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok:   $*"; }

command -v helm >/dev/null || fail "helm not found (need Helm ${HELM_MAJOR}.x)"
command -v kubeconform >/dev/null || fail "kubeconform not found (need ${KUBECONFORM_VERSION}: https://github.com/yannh/kubeconform/releases/tag/${KUBECONFORM_VERSION})"
command -v python3 >/dev/null || fail "python3 not found (stdlib only is needed)"
helm version --short | grep -q "^v${HELM_MAJOR}\." || fail "Helm ${HELM_MAJOR}.x required, got: $(helm version --short)"
kubeconform -v | grep -q "${KUBECONFORM_VERSION}" || fail "kubeconform ${KUBECONFORM_VERSION} required, got: $(kubeconform -v)"

# Structural YAML checker (python3 stdlib only; no PyYAML dependency). ---------------------
# Parses helm's block-style output into dicts/lists and asserts on specific paths.
#   py baseline            assert the security/reliability baseline on a rendered manifest (stdin)
#   py get KIND PATH [SUF] print the JSON value at PATH of the KIND doc (name ending SUF); exit 1 if absent
#   py absent KIND PATH    exit 0 only if PATH is absent from the KIND doc
#   py mutate FILE N OLD NEW   replace the N-th occurrence of OLD in FILE (exit 1 if not found)
#   py parser-selftest     unit-test the parser
read -r -d '' PYCHECK <<'PYEOF' || true
import json, re, sys

KEY = re.compile(r'^("(?:[^"\\]|\\.)*"|\'[^\']*\'|[^\s:\x22\x27][^:]*?):(?:\s+(.*))?$')

def scalar(v):
    v = v.strip()
    if v[:1] == '"':
        return json.loads(v)
    if v[:1] == "'":
        return v[1:-1].replace("''", "'")
    if v in ('true', 'True'): return True
    if v in ('false', 'False'): return False
    if v in ('null', '~'): return None
    if v == '{}': return {}
    if v == '[]': return []
    if re.fullmatch(r'-?\d+', v): return int(v)
    if re.fullmatch(r'-?\d+\.\d+', v): return float(v)
    if v[0] in '|>&*!{[':
        raise ValueError('unsupported YAML value: ' + v)
    return v

def parse(lines):
    n = len(lines)
    pos = [0]
    def is_seq(t): return t == '-' or t.startswith('- ')
    def block(ind):
        return seq(ind) if is_seq(lines[pos[0]][1]) else mapping(ind)
    def seq(ind):
        out = []
        while pos[0] < n and lines[pos[0]][0] == ind and is_seq(lines[pos[0]][1]):
            t = lines[pos[0]][1]
            rest = t[1:].lstrip()
            if not rest:
                pos[0] += 1
                if pos[0] < n and lines[pos[0]][0] > ind:
                    out.append(block(lines[pos[0]][0]))
                else:
                    out.append(None)
            else:
                off = len(t) - len(rest)
                if KEY.match(rest) or is_seq(rest):
                    lines[pos[0]] = (ind + off, rest)
                    out.append(block(ind + off))
                else:
                    pos[0] += 1
                    out.append(scalar(rest))
        return out
    def mapping(ind):
        d = {}
        while pos[0] < n and lines[pos[0]][0] == ind and not is_seq(lines[pos[0]][1]):
            t = lines[pos[0]][1]
            m = KEY.match(t)
            if not m:
                raise ValueError('cannot parse line: ' + t)
            k = m.group(1)
            if k[:1] in ('"', "'"):
                k = scalar(k)
            if k in d:
                raise ValueError('duplicate key: ' + k)
            v = m.group(2)
            pos[0] += 1
            if v is None or v == '':
                nxt = lines[pos[0]] if pos[0] < n else None
                if nxt and (nxt[0] > ind or (nxt[0] == ind and is_seq(nxt[1]))):
                    d[k] = block(nxt[0])
                else:
                    d[k] = None
            else:
                d[k] = scalar(v)
        return d
    if not n:
        return None
    result = block(lines[0][0])
    if pos[0] != n:
        raise ValueError('unparsed trailing content at: ' + lines[pos[0]][1])
    return result

def parse_docs(text):
    docs = []
    for chunk in re.split(r'(?m)^---\s*$', text):
        lines = []
        for raw in chunk.splitlines():
            if not raw.strip() or raw.lstrip().startswith('#'):
                continue
            if '\t' in raw[:len(raw) - len(raw.lstrip())]:
                raise ValueError('tab indentation')
            lines.append((len(raw) - len(raw.lstrip(' ')), raw.strip()))
        doc = parse(lines)
        if doc is not None:
            docs.append(doc)
    return docs

MISSING = object()

def get(obj, path):
    for part in path.split('.'):
        if isinstance(obj, list) and part.isdigit():
            idx = int(part)
            obj = obj[idx] if idx < len(obj) else MISSING
        elif isinstance(obj, dict):
            obj = obj.get(part, MISSING)
        else:
            return MISSING
        if obj is MISSING:
            return MISSING
    return obj

def find(docs, kind, suffix=''):
    hits = [d for d in docs if isinstance(d, dict) and d.get('kind') == kind
            and str(get(d, 'metadata.name')).endswith(suffix)]
    return hits[0] if hits else None

def baseline(docs):
    bad = []
    def check(desc, ok):
        print(('ok:   ' if ok else 'FAIL: ') + desc)
        if not ok:
            bad.append(desc)
    def eq(doc, path, want):
        return doc is not None and get(doc, path) == want
    dep = find(docs, 'Deployment')
    check('Deployment rendered', dep is not None)
    check('Service rendered', find(docs, 'Service') is not None)
    check('ServiceAccount rendered', find(docs, 'ServiceAccount') is not None)
    check('replicas default is 3', eq(dep, 'spec.replicas', 3))
    pod = 'spec.template.spec.'
    ctr = pod + 'containers.0.'
    check('pod securityContext.runAsNonRoot is true', eq(dep, pod + 'securityContext.runAsNonRoot', True))
    check('pod securityContext.seccompProfile.type is RuntimeDefault', eq(dep, pod + 'securityContext.seccompProfile.type', 'RuntimeDefault'))
    check('container securityContext.runAsNonRoot is true', eq(dep, ctr + 'securityContext.runAsNonRoot', True))
    check('container securityContext.readOnlyRootFilesystem is true', eq(dep, ctr + 'securityContext.readOnlyRootFilesystem', True))
    check('container securityContext.allowPrivilegeEscalation is false', eq(dep, ctr + 'securityContext.allowPrivilegeEscalation', False))
    check('container securityContext.capabilities.drop is [ALL]', eq(dep, ctr + 'securityContext.capabilities.drop', ['ALL']))
    check('container securityContext.seccompProfile.type is RuntimeDefault', eq(dep, ctr + 'securityContext.seccompProfile.type', 'RuntimeDefault'))
    check('container is not privileged', dep is not None and not get(dep, ctr + 'securityContext.privileged') is True)
    check('terminationGracePeriodSeconds is 30', eq(dep, pod + 'terminationGracePeriodSeconds', 30))
    prestop = get(dep, ctr + 'lifecycle.preStop.exec.command') if dep else MISSING
    check('container lifecycle.preStop.exec.command is set', isinstance(prestop, list) and len(prestop) > 0)
    for probe in ('startupProbe', 'livenessProbe', 'readinessProbe'):
        v = get(dep, ctr + probe) if dep else MISSING
        check('container ' + probe + ' is set', isinstance(v, dict) and len(v) > 0)
    for sect in ('requests', 'limits'):
        v = get(dep, ctr + 'resources.' + sect) if dep else MISSING
        check('container resources.' + sect + ' is set', isinstance(v, dict) and len(v) > 0)
    tsc = get(dep, pod + 'topologySpreadConstraints') if dep else MISSING
    zone = [c for c in tsc if isinstance(c, dict) and c.get('topologyKey') == 'topology.kubernetes.io/zone'] if isinstance(tsc, list) else []
    check('topologySpreadConstraints spreads over topology.kubernetes.io/zone', len(zone) == 1)
    check('zone topology spread is DoNotSchedule (hard)', len(zone) == 1 and zone[0].get('whenUnsatisfiable') == 'DoNotSchedule')
    pdb = find(docs, 'PodDisruptionBudget')
    check('PodDisruptionBudget rendered', pdb is not None)
    check('PDB spec.maxUnavailable is 1', eq(pdb, 'spec.maxUnavailable', 1))
    deny = find(docs, 'NetworkPolicy', '-default-deny')
    check('default-deny NetworkPolicy rendered', deny is not None)
    types = get(deny, 'spec.policyTypes') if deny else MISSING
    check('default-deny spec.policyTypes is exactly [Ingress, Egress]',
          isinstance(types, list) and sorted(types) == ['Egress', 'Ingress'])
    check('default-deny selects the workload pods (spec.podSelector.matchLabels)',
          deny is not None and isinstance(get(deny, 'spec.podSelector.matchLabels'), dict))
    check('default-deny has no spec.ingress rules', deny is not None and get(deny, 'spec.ingress') is MISSING)
    check('default-deny has no spec.egress rules', deny is not None and get(deny, 'spec.egress') is MISSING)
    return not bad

def main(argv):
    mode = argv[1]
    if mode == 'mutate':
        path, nth, old, new = argv[2], int(argv[3]), argv[4], argv[5]
        text = open(path).read()
        idx = -1
        for _ in range(nth):
            idx = text.find(old, idx + 1)
            if idx < 0:
                print('mutation target not found: ' + old, file=sys.stderr)
                return 1
        open(path, 'w').write(text[:idx] + new + text[idx + len(old):])
        return 0
    if mode == 'parser-selftest':
        sample = ('apiVersion: v1\nitems:\n- a\n- b: 1\n  c:\n  - x\n  - "y: z"\n  d: {}\n'
                  'e:\n  f: true\n  g: null\nh:\n  - k: v\n    l:\n      m: "q"\n')
        want = {'apiVersion': 'v1', 'items': ['a', {'b': 1, 'c': ['x', 'y: z'], 'd': {}}],
                'e': {'f': True, 'g': None}, 'h': [{'k': 'v', 'l': {'m': 'q'}}]}
        got = parse_docs(sample)[0]
        if got != want:
            print('parser self-test failed: %r' % (got,), file=sys.stderr)
            return 1
        try:
            parse_docs('a: 1\na: 2\n')
            print('parser must reject duplicate keys', file=sys.stderr)
            return 1
        except ValueError:
            pass
        return 0
    docs = parse_docs(sys.stdin.read())
    if mode == 'baseline':
        return 0 if baseline(docs) else 1
    if mode in ('get', 'absent'):
        kind, path = argv[2], argv[3]
        suffix = argv[4] if len(argv) > 4 else ''
        doc = find(docs, kind, suffix)
        if doc is None:
            print('no %s document' % kind, file=sys.stderr)
            return 1
        val = get(doc, path)
        if mode == 'absent':
            return 0 if val is MISSING else 1
        if val is MISSING:
            return 1
        print(json.dumps(val))
        return 0
    print('unknown mode ' + mode, file=sys.stderr)
    return 2

try:
    sys.exit(main(sys.argv))
except ValueError as e:
    print('FAIL: could not parse rendered YAML: %s' % e, file=sys.stderr)
    sys.exit(1)
PYEOF
py() { python3 -I -c "$PYCHECK" "$@"; }

py parser-selftest || fail "YAML checker self-test"
pass "YAML checker parser self-test"

# 1. Lint -------------------------------------------------------------------
helm dependency update "$APP" >/dev/null
helm lint "$LIB" >/dev/null && pass "helm lint service-base"
helm lint "$APP" >/dev/null && pass "helm lint example-service"

render() { helm template rel "$APP" "$@"; }

# 2. Schema validation (strict) --------------------------------------------
render | kubeconform -strict -summary -kubernetes-version "$KUBERNETES_VERSION" - \
  && pass "kubeconform --strict (default values)"
render --set autoscaling.enabled=true | kubeconform -strict -summary -kubernetes-version "$KUBERNETES_VERSION" - \
  && pass "kubeconform --strict (HPA enabled)"
render --set resources=null --set probes=null | kubeconform -strict -summary -kubernetes-version "$KUBERNETES_VERSION" - \
  && pass "kubeconform --strict (resources and probes null)"

# 3. Structural baseline assertions -------------------------------------------
OUT="$(render)"
py baseline <<<"$OUT" || fail "baseline assertions on the rendered default manifest"
pass "structural baseline assertions (securityContext, PDB, NetworkPolicy, probes, ...)"

# Security fields cannot be dropped by omitting them from values.
SPARSE="$(render --set podSecurityContext=null --set securityContext=null)"
py baseline <<<"$SPARSE" >/dev/null || fail "secure default lost when securityContext values omitted"
pass "secure defaults survive empty securityContext values"

# expect <description> <rendered> <Kind> <path> <expected-json> [name-suffix]
expect() {
  local got
  got="$(py get "$3" "$4" ${6:+"$6"} <<<"$2")" || fail "$1 (path $4 missing in $3)"
  [ "$got" = "$5" ] || fail "$1 (got $got, want $5)"
  pass "$1"
}
C0="spec.template.spec.containers.0"

# 4. Overrides ---------------------------------------------------------------
OVR="$(render --set replicaCount=5 --set resources.limits.memory=512Mi --set resources.requests.cpu=250m --set image.tag=9.9.9)"
expect "override replicaCount"  "$OVR" Deployment spec.replicas 5
expect "override memory limit"  "$OVR" Deployment "$C0.resources.limits.memory" '"512Mi"'
expect "override cpu request"   "$OVR" Deployment "$C0.resources.requests.cpu" '"250m"'
expect "override image tag"     "$OVR" Deployment "$C0.image" '"ghcr.io/example/example-service:9.9.9"'

DIGEST="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
DGS="$(render --set image.digest="$DIGEST" --set image.tag=ignored)"
expect "image digest pinning (digest takes precedence over tag)" "$DGS" Deployment "$C0.image" "\"ghcr.io/example/example-service@${DIGEST}\""

HPA="$(render --set autoscaling.enabled=true)"
expect "HPA rendered when enabled" "$HPA" HorizontalPodAutoscaler spec.maxReplicas 10
py absent Deployment spec.replicas <<<"$HPA" || fail "Deployment must not set replicas when HPA enabled"
pass "Deployment omits replicas when HPA enabled"

NP="$(render --set 'networkPolicy.ingress[0].from[0].podSelector.matchLabels.app=caller')"
expect "configurable allow rule" "$NP" NetworkPolicy spec.ingress.0.from.0.podSelector.matchLabels.app '"caller"' -allow

# R1-04: null resources/probes fall back to the baseline defaults instead of rendering null / erroring.
NULLS="$(render --set resources=null --set probes=null)"
expect "resources=null falls back to default requests" "$NULLS" Deployment "$C0.resources.requests.cpu" '"100m"'
expect "probes=null falls back to default readiness"   "$NULLS" Deployment "$C0.readinessProbe.httpGet.path" '"/healthz/ready"'
py baseline <<<"$NULLS" >/dev/null || fail "baseline lost when resources/probes are null"
pass "baseline holds when resources and probes are null"

# R1-03: zone spread is hard (DoNotSchedule) by default, also when a constraint omits whenUnsatisfiable.
OMIT="$(render --set-json 'topologySpreadConstraints=[{"maxSkew":1,"topologyKey":"topology.kubernetes.io/zone"}]')"
expect "whenUnsatisfiable defaults to DoNotSchedule when omitted" "$OMIT" Deployment spec.template.spec.topologySpreadConstraints.0.whenUnsatisfiable '"DoNotSchedule"'
SOFT="$(render --set 'topologySpreadConstraints[0].whenUnsatisfiable=ScheduleAnyway')"
expect "whenUnsatisfiable can be overridden to ScheduleAnyway" "$SOFT" Deployment spec.template.spec.topologySpreadConstraints.0.whenUnsatisfiable '"ScheduleAnyway"'

# 5. Guards: values that weaken the security baseline must fail the render (R1-02) -----------
# guards_hold <chart-dir>: 0 if every insecure override is rejected with its message and a legitimate one renders.
guards_hold() {
  local app="$1" c flag msg out
  local cases=(
    "securityContext.privileged=true|securityContext.privileged=true"
    "podSecurityContext.runAsUser=0|podSecurityContext.runAsUser=0"
    "securityContext.runAsUser=0|securityContext.runAsUser=0"
    "podSecurityContext.runAsNonRoot=false|podSecurityContext.runAsNonRoot=false"
    "securityContext.runAsNonRoot=false|securityContext.runAsNonRoot=false"
    "securityContext.readOnlyRootFilesystem=false|readOnlyRootFilesystem=false"
    "securityContext.allowPrivilegeEscalation=true|allowPrivilegeEscalation=true"
    "securityContext.seccompProfile.type=Unconfined|securityContext.seccompProfile.type=Unconfined"
    "podSecurityContext.seccompProfile.type=Unconfined|podSecurityContext.seccompProfile.type=Unconfined"
    "securityContext.capabilities.add[0]=SYS_ADMIN|capabilities.add SYS_ADMIN"
    "securityContext.capabilities.drop[0]=NET_RAW|capabilities.drop must include ALL"
  )
  for c in "${cases[@]}"; do
    flag="${c%%|*}"
    msg="${c#*|}"
    if out="$(helm template rel "$app" --set "$flag" 2>&1)"; then
      echo "guard missing: --set $flag rendered without error"
      return 1
    fi
    grep -qF -- "$msg" <<<"$out" || { echo "wrong error for --set $flag: $out"; return 1; }
  done
  helm template rel "$app" --set podSecurityContext.runAsUser=20000 >/dev/null || { echo "legitimate runAsUser rejected"; return 1; }
}
guards_hold "$APP" || fail "security guards"
pass "insecure securityContext values fail the render with a clear message (11 cases) and legitimate ones render"

# 6. Self-test: mutate a scratch copy of the charts; every mutation must be caught -------------
# checks_hold <chart-dir>: 0 = all structural + guard checks hold, 1 = a check failed, 2 = chart does not render.
checks_hold() {
  local app="$1" out
  out="$(helm template rel "$app" 2>/dev/null)" || return 2
  py baseline <<<"$out" >/dev/null 2>&1 || return 1
  out="$(helm template rel "$app" --set resources=null --set probes=null 2>/dev/null)" || return 2
  py baseline <<<"$out" >/dev/null 2>&1 || return 1
  out="$(helm template rel "$app" --set podSecurityContext=null --set securityContext=null 2>/dev/null)" || return 2
  py baseline <<<"$out" >/dev/null 2>&1 || return 1
  guards_hold "$app" >/dev/null 2>&1 || return 1
}

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
MUTANTS=0
# mutant <description> <path under deploy/charts> <nth occurrence> <old> <new>; empty path = control (no mutation)
mutant() {
  local rc=0
  rm -rf "$TMP/c"
  mkdir -p "$TMP/c"
  cp -R "$LIB" "$TMP/c/service-base"
  cp -R "$APP" "$TMP/c/example-service"
  rm -rf "$TMP/c/example-service/charts" "$TMP/c/example-service/Chart.lock"
  if [ -n "$2" ]; then
    py mutate "$TMP/c/$2" "$3" "$4" "$5" || fail "self-test: mutation target not found: $1"
  fi
  helm dependency update "$TMP/c/example-service" >/dev/null
  checks_hold "$TMP/c/example-service" || rc=$?
  if [ -z "$2" ]; then
    [ "$rc" -eq 0 ] || fail "self-test control (unmutated copy) did not pass checks (rc=$rc)"
    pass "self-test control: unmutated copy passes all checks"
    return 0
  fi
  [ "$rc" -ne 2 ] || fail "self-test: mutation '$1' broke rendering instead of being caught by an assertion"
  [ "$rc" -eq 1 ] || fail "self-test: mutation SURVIVED (checks still pass): $1"
  MUTANTS=$((MUTANTS + 1))
  pass "mutation caught: $1"
}

B=service-base/templates
mutant "control" "" 0 "" ""
mutant "container capabilities.drop renamed"             $B/_helpers.tpl 1 "  drop:" "  drop-x:"
mutant "container capabilities.drop ALL -> NET_RAW"      $B/_helpers.tpl 1 "    - ALL" "    - NET_RAW"
mutant "NetworkPolicy policyTypes Egress renamed"        $B/_networkpolicy.tpl 1 "    - Egress" "    - Egress-x"
mutant "NetworkPolicy policyTypes Ingress renamed"       $B/_networkpolicy.tpl 1 "    - Ingress" "    - Ingress-x"
mutant "default-deny gains an egress allow rule"         $B/_networkpolicy.tpl 1 $'    - Egress\n{{- if or' $'    - Egress\n  egress:\n    - {}\n{{- if or'
mutant "readOnlyRootFilesystem true -> false"            $B/_helpers.tpl 1 "readOnlyRootFilesystem: true" "readOnlyRootFilesystem: false"
mutant "allowPrivilegeEscalation false -> true"          $B/_helpers.tpl 1 "allowPrivilegeEscalation: false" "allowPrivilegeEscalation: true"
mutant "pod seccomp RuntimeDefault -> Unconfined"        $B/_helpers.tpl 1 "type: RuntimeDefault" "type: Unconfined"
mutant "container seccomp RuntimeDefault -> Unconfined"  $B/_helpers.tpl 2 "type: RuntimeDefault" "type: Unconfined"
mutant "pod runAsNonRoot true -> false"                  $B/_helpers.tpl 1 "runAsNonRoot: true" "runAsNonRoot: false"
mutant "container runAsNonRoot true -> false"            $B/_helpers.tpl 2 "runAsNonRoot: true" "runAsNonRoot: false"
mutant "container defaults not merged under user values" $B/_deployment.tpl 1 'mergeOverwrite (fromYaml (include "service-base.defaultContainerSecurityContext" .)) (deepCopy (default dict .Values.securityContext))' '(default dict .Values.securityContext)'
mutant "PDB maxUnavailable 1 -> 2"                      $B/_pdb.tpl 1 "{{ .Values.pdb.maxUnavailable }}" "2"
mutant "PDB removed"                                     example-service/templates/pdb.yaml 1 '{{ include "service-base.pdb" . }}' ""
mutant "NetworkPolicy removed"                           example-service/templates/networkpolicy.yaml 1 '{{ include "service-base.networkpolicy" . }}' ""
mutant "startupProbe removed"                            $B/_deployment.tpl 1 "startupProbe:" "startupProbe-x:"
mutant "livenessProbe removed"                           $B/_deployment.tpl 1 "livenessProbe:" "livenessProbe-x:"
mutant "readinessProbe removed"                          $B/_deployment.tpl 1 "readinessProbe:" "readinessProbe-x:"
mutant "preStop hook removed"                            $B/_deployment.tpl 1 "preStop:" "preStop-x:"
mutant "terminationGracePeriodSeconds removed"           $B/_deployment.tpl 1 "terminationGracePeriodSeconds: {{" "terminationGracePeriodSecondsX: {{"
mutant "resources removed"                               $B/_deployment.tpl 1 "resources:" "resources-x:"
mutant "null resources no longer fall back to defaults"  $B/_deployment.tpl 1 '(default (fromYaml (include "service-base.defaultResources" .)) .Values.resources)' '.Values.resources'
mutant "null probes no longer fall back to defaults"     $B/_deployment.tpl 1 '(default $defaultProbes.readiness $probes.readiness)' '$probes.readiness'
mutant "zone spread default -> ScheduleAnyway"           example-service/values.yaml 1 "whenUnsatisfiable: DoNotSchedule" "whenUnsatisfiable: ScheduleAnyway"
mutant "topology key no longer zone"                     $B/_deployment.tpl 1 "topologyKey: {{ .topologyKey }}" "topologyKey: kubernetes.io/hostname"
mutant "default replicas 3 -> 1"                         example-service/values.yaml 1 "replicaCount: 3" "replicaCount: 1"
mutant "security guard call removed"                     $B/_deployment.tpl 1 '{{- include "service-base.validateSecurity" . -}}' ""
mutant "guard: privileged check disabled"               $B/_helpers.tpl 1 '(hasKey $sc "privileged")' '(hasKey $sc "privilegedX")'
mutant "guard: runAsUser 0 check disabled"              $B/_helpers.tpl 1 '(eq (toString $sc.runAsUser) "0")' '(eq (toString $sc.runAsUser) "-1")'
mutant "guard: Unconfined seccomp allowed"              $B/_helpers.tpl 1 '(list "RuntimeDefault" "Localhost")' '(list "RuntimeDefault" "Localhost" "Unconfined")'
echo "Self-test: ${MUTANTS} mutations, all caught."

echo "All chart validations passed."
