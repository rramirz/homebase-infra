{{- define "plane.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "plane.fullname" -}}
{{- default (include "plane.name" .) .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "plane.labels" -}}
app.kubernetes.io/name: {{ include "plane.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
