{{- define "cloudflared.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "cloudflared.fullname" -}}
{{- default (include "cloudflared.name" .) .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
