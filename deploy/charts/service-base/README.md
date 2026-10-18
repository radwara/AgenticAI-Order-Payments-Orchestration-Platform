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

## Guard rails

- Values that weaken the baseline make `helm template`/`install` **fail** with a message naming the value:
  `privileged: true`, `runAsNonRoot: false`, `runAsUser: 0`, `readOnlyRootFilesystem: false`,
  `allowPrivilegeEscalation: true`, `seccompProfile.type` other than `RuntimeDefault`/`Localhost`,
  `capabilities.drop` without `ALL`, and `capabilities.add` of `ALL` or `SYS_ADMIN`. There is no opt-out;
  a workload that needs more privilege must not use this chart. Use `extraVolumes` for writable paths.
- `resources` and `probes` set to null/empty fall back to the library defaults (rendered, not `null`).
- Zone topology spread defaults to `whenUnsatisfiable: DoNotSchedule` (hard, "spread across zones").
  Trade-off: on a cluster with fewer zones than replicas the extra pods stay Pending; set
  `whenUnsatisfiable: ScheduleAnyway` on the constraint to accept soft spreading.
