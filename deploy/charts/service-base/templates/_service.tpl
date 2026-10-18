{{- define "service-base.service" -}}
apiVersion: v1
kind: Service
metadata:
  name: {{ include "service-base.fullname" . }}
  labels:
    {{- include "service-base.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - name: http
      port: {{ .Values.service.port }}
      targetPort: http
      protocol: TCP
  selector:
    {{- include "service-base.selectorLabels" . | nindent 4 }}
{{- end -}}
