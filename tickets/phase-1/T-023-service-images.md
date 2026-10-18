---
id: T-023
title: "[T-023] Container images for pricing, order and payment"
labels: phase:1, status:needs-approval, area:platform, size:S
---
## Why
The Phase 1 end-to-end test and Kubernetes deploys need runnable images. Inventory's image is in T-007. PRD 9.2 Images.

## What
Add multi-stage `Dockerfile` for each of `services/pricing` (Go), `services/order` and `services/payment` (Java 21): build stage, then distroless non-root runtime stage, read-only-rootfs compatible (writable paths via emptyDir only), `HEALTHCHECK`-free (Kubernetes probes used). Add `make images` building all service images tagged with git SHA, and add the three services to `deploy/local/docker-compose.yml` (profile `apps`).

## Constraints
Base images pinned by digest. Run as UID 65532. No shell in runtime image. No secrets in layers. JVM flags: `-XX:MaxRAMPercentage=75`.

## Out of scope
Registry push, image signing and scanning pipeline, Helm values.

## Acceptance criteria (validations)
- [ ] Each image builds from a clean checkout with `make images`
- [ ] `docker inspect` shows non-root user for all three images (script test)
- [ ] Container starts with `--read-only` and passes `/readyz` (script test using compose)
- [ ] Image sizes reported by a script and Go image < 40 MB

## Files in scope
`services/pricing/Dockerfile`, `services/order/Dockerfile`, `services/payment/Dockerfile`, `deploy/local/docker-compose.yml`, `Makefile`, `scripts/test-images.sh`

## Depends on
T-006, T-011, T-016
