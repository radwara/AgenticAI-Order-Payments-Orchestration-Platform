{{/* Chart name, truncated to 63 chars. */}}
{{- define "service-base.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Fully qualified resource name. */}}
{{- define "service-base.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "service-base.selectorLabels" -}}
app.kubernetes.io/name: {{ include "service-base.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "service-base.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "service-base.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Chart.AppVersion }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
{{- end -}}

{{- define "service-base.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "service-base.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Image reference. A digest takes precedence over the tag (digest pinning);
otherwise the tag, defaulting to the chart appVersion. No implicit "latest".
*/}}
{{- define "service-base.image" -}}
{{- $repo := required "image.repository is required" .Values.image.repository -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" $repo .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" $repo (required "image.tag or chart appVersion is required" (default .Chart.AppVersion .Values.image.tag)) -}}
{{- end -}}
{{- end -}}

{{/* Secure defaults. User values are merged over these, never replace them wholesale. */}}
{{- define "service-base.defaultPodSecurityContext" -}}
runAsNonRoot: true
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{- define "service-base.defaultContainerSecurityContext" -}}
runAsNonRoot: true
readOnlyRootFilesystem: true
allowPrivilegeEscalation: false
capabilities:
  drop:
    - ALL
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{/* Default resources and probes. Used when values set them to null/empty, so the baseline cannot be dropped by omission. */}}
{{- define "service-base.defaultResources" -}}
requests:
  cpu: 100m
  memory: 128Mi
limits:
  memory: 256Mi
{{- end -}}

{{- define "service-base.defaultProbes" -}}
startup:
  httpGet: {path: /healthz/startup, port: http}
  periodSeconds: 5
  failureThreshold: 30
liveness:
  httpGet: {path: /healthz/live, port: http}
  periodSeconds: 10
  failureThreshold: 3
readiness:
  httpGet: {path: /healthz/ready, port: http}
  periodSeconds: 5
  failureThreshold: 3
{{- end -}}

{{/*
Guard against values that weaken the security baseline (PRD 9.2). The values the service supplied
are inspected directly, because zero values (`false`, `0`) are not reliably carried by a merge.
There is intentionally no opt-out: a workload that genuinely needs more privilege must not use
this library chart.
*/}}
{{- define "service-base.validateSecurity" -}}
{{- $scopes := dict "podSecurityContext" (default dict .Values.podSecurityContext) "securityContext" (default dict .Values.securityContext) -}}
{{- range $scope, $sc := $scopes -}}
{{- if and (hasKey $sc "privileged") $sc.privileged -}}
{{- fail (printf "%s.privileged=true is not allowed by the service-base security baseline" $scope) -}}
{{- end -}}
{{- if and (hasKey $sc "runAsNonRoot") (not $sc.runAsNonRoot) -}}
{{- fail (printf "%s.runAsNonRoot=false is not allowed by the service-base security baseline" $scope) -}}
{{- end -}}
{{- if and (hasKey $sc "runAsUser") (eq (toString $sc.runAsUser) "0") -}}
{{- fail (printf "%s.runAsUser=0 (root) is not allowed by the service-base security baseline" $scope) -}}
{{- end -}}
{{- if and (hasKey $sc "readOnlyRootFilesystem") (not $sc.readOnlyRootFilesystem) -}}
{{- fail (printf "%s.readOnlyRootFilesystem=false is not allowed; use extraVolumes/extraVolumeMounts for writable paths" $scope) -}}
{{- end -}}
{{- if and (hasKey $sc "allowPrivilegeEscalation") $sc.allowPrivilegeEscalation -}}
{{- fail (printf "%s.allowPrivilegeEscalation=true is not allowed by the service-base security baseline" $scope) -}}
{{- end -}}
{{- if hasKey $sc "seccompProfile" -}}
{{- $type := toString (default dict $sc.seccompProfile).type -}}
{{- if not (has $type (list "RuntimeDefault" "Localhost")) -}}
{{- fail (printf "%s.seccompProfile.type=%s is not allowed; use RuntimeDefault (or Localhost)" $scope $type) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- with (default dict .Values.securityContext).capabilities -}}
{{- if and .drop (not (has "ALL" .drop)) -}}
{{- fail "securityContext.capabilities.drop must include ALL" -}}
{{- end -}}
{{- range (default list .add) -}}
{{- if has (toString . | upper) (list "ALL" "SYS_ADMIN") -}}
{{- fail (printf "securityContext.capabilities.add %s is not allowed by the service-base security baseline" .) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Database env vars (optional `database` values). Always secretKeyRef to the Secret produced by the
ExternalSecret in deploy/postgres; a plaintext value is never rendered, and a hand-written env entry
with the same name fails the render instead of silently shadowing the secret.
*/}}
{{- define "service-base.databaseEnv" -}}
{{- $secret := required "database.secretName is required when database.enabled is true" .Values.database.secretName -}}
{{- range $name := list "DB_URL" "DB_USER" "DB_PASSWORD" }}
{{- range $.Values.env }}
{{- if eq .name $name }}{{ fail (printf "env %s conflicts with database.enabled; the value must come from the secret" $name) }}{{ end }}
{{- end }}
- name: {{ $name }}
  valueFrom:
    secretKeyRef:
      name: {{ $secret | quote }}
      key: {{ $name }}
{{- end }}
{{- end -}}
