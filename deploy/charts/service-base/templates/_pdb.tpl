{{- define "service-base.pdb" -}}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "service-base.fullname" . }}
  labels:
    {{- include "service-base.labels" . | nindent 4 }}
spec:
  maxUnavailable: {{ .Values.pdb.maxUnavailable }}
  selector:
    matchLabels:
      {{- include "service-base.selectorLabels" . | nindent 6 }}
{{- end -}}
