{{- define "actual-budget.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "actual-budget.fullname" -}}
{{- default (include "actual-budget.name" .) .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
