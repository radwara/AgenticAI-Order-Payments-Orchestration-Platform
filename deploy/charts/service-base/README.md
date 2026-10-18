# service-base

Helm library chart (type `library`) encoding the platform workload standards. Consume it from a service chart:

```yaml
dependencies:
  - name: service-base
    version: 0.1.0
    repository: file://../service-base
```

and add one template per resource, e.g. `{{ include "service-base.deployment" . }}`.
Available: `deployment`, `service`, `pdb`, `hpa`, `networkpolicy`, `serviceaccount`.

Defaults live in the consuming chart's `values.yaml` (see `example-service`). The pod and container
securityContext are merged over secure defaults (runAsNonRoot, read-only rootfs, drop ALL, seccomp RuntimeDefault),
so omitting them from values never weakens the baseline.
