{{- define "service-base.serviceaccount" -}}
{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "service-base.serviceAccountName" . }}
  labels:
    {{- include "service-base.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
automountServiceAccountToken: {{ .Values.automountServiceAccountToken }}
{{- end -}}
{{- end -}}
