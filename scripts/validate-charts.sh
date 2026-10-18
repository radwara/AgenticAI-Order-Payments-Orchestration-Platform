#!/usr/bin/env bash
# Validates the Helm charts under deploy/charts (T-018).
#   1. helm lint (service-base, example-service)
#   2. helm template + kubeconform --strict
#   3. assertions on the rendered pod spec (securityContext, PDB, NetworkPolicy, ...)
#   4. override renders (replicas, resources, image tag, image digest, HPA)
# Requires: helm 3, kubeconform (pinned below). Neither is installed by this script.
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
helm version --short | grep -q "^v${HELM_MAJOR}\." || fail "Helm ${HELM_MAJOR}.x required, got: $(helm version --short)"
kubeconform -v | grep -q "${KUBECONFORM_VERSION}" || fail "kubeconform ${KUBECONFORM_VERSION} required, got: $(kubeconform -v)"

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

# 3. Rendered pod spec assertions -------------------------------------------
OUT="$(render)"
assert() { # assert <description> <fixed-string> [haystack]
  local hay="${3:-$OUT}"
  grep -qF -- "$2" <<<"$hay" || fail "$1 (missing: $2)"
  pass "$1"
}

assert "Deployment rendered"           "kind: Deployment"
assert "replicas default is 3"         "replicas: 3"
assert "pod runAsNonRoot"              "runAsNonRoot: true"
assert "readOnlyRootFilesystem"        "readOnlyRootFilesystem: true"
assert "allowPrivilegeEscalation off"  "allowPrivilegeEscalation: false"
assert "capabilities drop ALL"         "- ALL"
assert "seccomp RuntimeDefault"        "type: RuntimeDefault"
assert "terminationGracePeriodSeconds" "terminationGracePeriodSeconds: 30"
assert "preStop hook"                  "preStop:"
assert "startupProbe"                  "startupProbe:"
assert "livenessProbe"                 "livenessProbe:"
assert "readinessProbe"                "readinessProbe:"
assert "resources requests"            "requests:"
assert "resources limits"              "limits:"
assert "zone topology spread"          "topologyKey: topology.kubernetes.io/zone"
assert "PodDisruptionBudget"           "kind: PodDisruptionBudget"
assert "PDB maxUnavailable 1"          "maxUnavailable: 1"
assert "NetworkPolicy"                 "kind: NetworkPolicy"
assert "ServiceAccount"                "kind: ServiceAccount"
assert "Service"                       "kind: Service"

# Both pod-level and container-level security contexts must carry the fields.
for field in "runAsNonRoot: true" "type: RuntimeDefault"; do
  [ "$(grep -cF -- "$field" <<<"$OUT")" -ge 2 ] || fail "'$field' must be set at pod AND container level"
done
pass "securityContext fields present at pod and container level"

# Default-deny policy: no ingress/egress rules in the deny document.
DENY="$(helm template rel "$APP" --show-only templates/networkpolicy.yaml | awk '/-default-deny/{f=1} /^---/{f=0} f' )"
grep -qF -- "policyTypes:" <<<"$DENY" || fail "default-deny NetworkPolicy missing policyTypes"
! grep -qE '^\s*(ingress|egress):' <<<"$DENY" || fail "default-deny NetworkPolicy must not contain allow rules"
pass "default-deny NetworkPolicy has no allow rules"

# Security fields cannot be dropped by omitting them from values.
SPARSE="$(render --set podSecurityContext=null --set securityContext=null)"
for field in "runAsNonRoot: true" "readOnlyRootFilesystem: true" "type: RuntimeDefault" "- ALL"; do
  grep -qF -- "$field" <<<"$SPARSE" || fail "secure default lost when values omitted: $field"
done
pass "secure defaults survive empty securityContext values"

# 4. Overrides ---------------------------------------------------------------
OVR="$(render --set replicaCount=5 --set resources.limits.memory=512Mi --set resources.requests.cpu=250m --set image.tag=9.9.9)"
assert "override replicaCount"   "replicas: 5" "$OVR"
assert "override memory limit"   "memory: 512Mi" "$OVR"
assert "override cpu request"    "cpu: 250m" "$OVR"
assert "override image tag"      'image: "ghcr.io/example/example-service:9.9.9"' "$OVR"

DIGEST="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
DGS="$(render --set image.digest="$DIGEST" --set image.tag=ignored)"
assert "image digest pinning"    "image: \"ghcr.io/example/example-service@${DIGEST}\"" "$DGS"
! grep -qF "ignored" <<<"$DGS" || fail "digest must take precedence over tag"
pass "digest takes precedence over tag"

HPA="$(render --set autoscaling.enabled=true)"
assert "HPA rendered when enabled" "kind: HorizontalPodAutoscaler" "$HPA"
! grep -qE '^\s*replicas:' <<<"$HPA" || fail "Deployment must not set replicas when HPA enabled"
pass "Deployment omits replicas when HPA enabled"

NP="$(render --set 'networkPolicy.ingress[0].from[0].podSelector.matchLabels.app=caller')"
assert "configurable allow rule" "app: caller" "$NP"

echo "All chart validations passed."
