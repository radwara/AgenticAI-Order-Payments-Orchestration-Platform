---
id: T-018
title: "[T-018] Helm library chart with workload standards"
labels: phase:1, status:needs-approval, area:platform, size:M
---
## Why
PRD §9.2: every service must meet the same security and reliability baseline by default.

## What
Create `deploy/charts/service-base` (Helm library chart) and `deploy/charts/example-service` that consumes it. Templates: Deployment (3 replicas default, topology spread across zones, startup/liveness/readiness probes, resources, securityContext: runAsNonRoot, readOnlyRootFilesystem, drop ALL, seccomp RuntimeDefault, terminationGracePeriodSeconds + preStop), Service, PodDisruptionBudget (maxUnavailable 1), HPA, default-deny NetworkPolicy plus configurable allow rules, ServiceAccount.

## Constraints
Helm 3 only. No operators, no cluster needed: validated with `helm lint`, `helm template`, and `kubeconform` (pinned in a script).

## Out of scope
Argo CD apps, environment overlays, mesh, per-service charts.

## Acceptance criteria (validations)
- [ ] `helm lint` passes for both charts
- [ ] Rendered manifest from example-service validates with kubeconform (strict)
- [ ] A script asserts rendered pod spec has all securityContext fields above and a PDB and a NetworkPolicy
- [ ] Overriding replicas, resources and image works through values (test render)
- [ ] Image reference supports digest pinning

## Files in scope
`deploy/charts/**`, `scripts/validate-charts.sh`

## Depends on
T-001
