{{- define "service-base.deployment" -}}
{{- include "service-base.validateSecurity" . -}}
{{- $probes := default dict .Values.probes -}}
{{- $defaultProbes := fromYaml (include "service-base.defaultProbes" .) -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "service-base.fullname" . }}
  labels:
    {{- include "service-base.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "service-base.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "service-base.selectorLabels" . | nindent 8 }}
        {{- with .Values.podLabels }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      {{- with .Values.podAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
    spec:
      serviceAccountName: {{ include "service-base.serviceAccountName" . }}
      automountServiceAccountToken: {{ .Values.automountServiceAccountToken }}
      terminationGracePeriodSeconds: {{ .Values.terminationGracePeriodSeconds }}
      securityContext:
        {{- mergeOverwrite (fromYaml (include "service-base.defaultPodSecurityContext" .)) (deepCopy (default dict .Values.podSecurityContext)) | toYaml | nindent 8 }}
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: {{ .Chart.Name }}
          image: {{ include "service-base.image" . | quote }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          securityContext:
            {{- mergeOverwrite (fromYaml (include "service-base.defaultContainerSecurityContext" .)) (deepCopy (default dict .Values.securityContext)) | toYaml | nindent 12 }}
          ports:
            - name: http
              containerPort: {{ .Values.containerPort }}
              protocol: TCP
          {{- $db := default dict .Values.database }}
          {{- if or .Values.env $db.enabled }}
          env:
            {{- with .Values.env }}
            {{- toYaml . | nindent 12 }}
            {{- end }}
            {{- if $db.enabled }}
            {{- include "service-base.databaseEnv" . | nindent 12 }}
            {{- end }}
          {{- end }}
          startupProbe:
            {{- toYaml (default $defaultProbes.startup $probes.startup) | nindent 12 }}
          livenessProbe:
            {{- toYaml (default $defaultProbes.liveness $probes.liveness) | nindent 12 }}
          readinessProbe:
            {{- toYaml (default $defaultProbes.readiness $probes.readiness) | nindent 12 }}
          lifecycle:
            preStop:
              exec:
                command:
                  {{- toYaml .Values.preStopCommand | nindent 18 }}
          resources:
            {{- toYaml (default (fromYaml (include "service-base.defaultResources" .)) .Values.resources) | nindent 12 }}
          volumeMounts:
            - name: tmp
              mountPath: /tmp
            {{- with .Values.extraVolumeMounts }}
            {{- toYaml . | nindent 12 }}
            {{- end }}
      volumes:
        - name: tmp
          emptyDir: {}
        {{- with .Values.extraVolumes }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      {{- with .Values.topologySpreadConstraints }}
      topologySpreadConstraints:
        {{- range . }}
        - maxSkew: {{ .maxSkew }}
          topologyKey: {{ .topologyKey }}
          whenUnsatisfiable: {{ default "DoNotSchedule" .whenUnsatisfiable }}
          labelSelector:
            matchLabels:
              {{- include "service-base.selectorLabels" $ | nindent 14 }}
        {{- end }}
      {{- end }}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
{{- end -}}
