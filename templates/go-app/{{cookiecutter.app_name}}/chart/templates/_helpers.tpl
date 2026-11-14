{% raw %}{{- define "app.name" -}}{{ .Chart.Name }}{{- end }}
{{- define "app.dbName" -}}{{ .Values.db.name | default .Release.Name }}{{- end }}
{{- define "app.dbSecret" -}}{{ .Values.db.psqlClusterName }}-db-{{ include "app.dbName" . }}{{- end }}
{% endraw %}
