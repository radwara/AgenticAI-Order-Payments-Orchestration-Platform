{{/* Default-deny for all ingress and egress of the workload's pods, plus optional allow rules. */}}
{{- define "service-base.networkpolicy" -}}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "service-base.fullname" . }}-default-deny
  labels:
    {{- include "service-base.labels" . | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      {{- include "service-base.selectorLabels" . | nindent 6 }}
  policyTypes:
    - Ingress
    - Egress
{{- if or .Values.networkPolicy.ingress .Values.networkPolicy.egress }}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "service-base.fullname" . }}-allow
  labels:
    {{- include "service-base.labels" . | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      {{- include "service-base.selectorLabels" . | nindent 6 }}
  policyTypes:
    {{- if .Values.networkPolicy.ingress }}
    - Ingress
    {{- end }}
    {{- if .Values.networkPolicy.egress }}
    - Egress
    {{- end }}
  {{- with .Values.networkPolicy.ingress }}
  ingress:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  {{- with .Values.networkPolicy.egress }}
  egress:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- end }}
{{- end -}}
