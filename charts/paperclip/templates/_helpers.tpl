{{- define "paperclip.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "paperclip.fullname" -}}
{{- default (include "paperclip.name" .) .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "paperclip.labels" -}}
app.kubernetes.io/name: {{ include "paperclip.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "paperclip.checkSecret" -}}
{{- if empty .value -}}
  {{- fail (printf "paperclip: %s is empty" .name) -}}
{{- end -}}
{{- if lt (len .value) 32 -}}
  {{- fail (printf "paperclip: %s must be at least 32 characters (got %d)" .name (len .value)) -}}
{{- end -}}
{{- if .testMode -}}
  {{- if not (hasPrefix "static-validation" .value) -}}
    {{- fail (printf "paperclip: %s must use the static-validation-only-* shape in testMode" .name) -}}
  {{- end -}}
{{- else -}}
  {{- if hasPrefix "static-validation" .value -}}
    {{- fail (printf "paperclip: %s is a static-validation placeholder; provide a real secret" .name) -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{- define "paperclip.validate" -}}
{{- if .Values.enabled -}}
  {{- if not (has .Values.profile (list "personal" "work" "logicbroker")) -}}
    {{- fail (printf "paperclip: unknown or empty profile %q (must be personal, work, or logicbroker)" (.Values.profile | default "")) -}}
  {{- end -}}
  {{- if empty .Values.namespace -}}
    {{- fail "paperclip: namespace is empty - set it in the overlay values file" -}}
  {{- end -}}
  {{- if ne .Release.Namespace .Values.namespace -}}
    {{- fail (printf "paperclip: Helm namespace %q does not match profile namespace %q" .Release.Namespace .Values.namespace) -}}
  {{- end -}}
  {{- if empty .Values.hostname -}}
    {{- fail "paperclip: hostname is empty - set it in the overlay values file" -}}
  {{- end -}}
  {{- $expectedPublicUrl := printf "https://%s" .Values.hostname -}}
  {{- if not .Values.httpRoute.enabled -}}
    {{- $expectedPublicUrl = "http://localhost:3100" -}}
  {{- end -}}
  {{- if ne (.Values.env.PAPERCLIP_PUBLIC_URL | default "") $expectedPublicUrl -}}
    {{- fail (printf "paperclip: PAPERCLIP_PUBLIC_URL %q does not match %q (httpRoute.enabled=%v)" (.Values.env.PAPERCLIP_PUBLIC_URL | default "") $expectedPublicUrl .Values.httpRoute.enabled) -}}
  {{- end -}}
  {{- if not .Values.httpRoute.enabled -}}
    {{- if ne (.Values.probe.hostHeader | default "") "localhost:3100" -}}
      {{- fail (printf "paperclip: probe.hostHeader must be exactly localhost:3100 when the HTTPRoute is disabled (bootstrap); got %q" (.Values.probe.hostHeader | default "")) -}}
    {{- end -}}
  {{- else -}}
    {{- if ne (.Values.probe.hostHeader | default "") .Values.hostname -}}
      {{- fail (printf "paperclip: probe.hostHeader must equal hostname %q when the HTTPRoute is enabled (post-claim); got %q" .Values.hostname (.Values.probe.hostHeader | default "")) -}}
    {{- end -}}
  {{- end -}}
  {{- if not (regexMatch "^sha256:[0-9a-f]{64}$" .Values.image.paperclip.digest) -}}
    {{- fail "paperclip: paperclip image digest is not a valid sha256 hex digest (must match ^sha256:[0-9a-f]{64}$)" -}}
  {{- end -}}
  {{- if not (regexMatch "^sha256:[0-9a-f]{64}$" .Values.image.postgres.digest) -}}
    {{- fail "paperclip: postgres image digest is not a valid sha256 hex digest (must match ^sha256:[0-9a-f]{64}$)" -}}
  {{- end -}}
  {{- $secrets := .Values.secrets | default dict -}}
  {{- $pg := $secrets.postgres | default dict -}}
  {{- $pgPassword := $pg.password | default "" -}}
  {{- $betterAuth := $secrets.betterAuthSecret | default "" -}}
  {{- $toolSigning := $secrets.toolActionSigningSecret | default "" -}}
  {{- $agentJwt := $secrets.agentJwtSecret | default "" -}}
  {{- include "paperclip.checkSecret" (dict "name" "secrets.postgres.password" "value" $pgPassword "testMode" .Values.testMode) -}}
  {{- include "paperclip.checkSecret" (dict "name" "secrets.betterAuthSecret" "value" $betterAuth "testMode" .Values.testMode) -}}
  {{- include "paperclip.checkSecret" (dict "name" "secrets.toolActionSigningSecret" "value" $toolSigning "testMode" .Values.testMode) -}}
  {{- include "paperclip.checkSecret" (dict "name" "secrets.agentJwtSecret" "value" $agentJwt "testMode" .Values.testMode) -}}
  {{- if or (eq $betterAuth $toolSigning) (eq $betterAuth $agentJwt) (eq $toolSigning $agentJwt) -}}
    {{- fail "paperclip: betterAuthSecret, toolActionSigningSecret, and agentJwtSecret must be pairwise distinct" -}}
  {{- end -}}
  {{- if not .Values.testMode -}}
    {{- if or (empty .Values.release) (ne .Release.Name .Values.release) -}}
      {{- fail (printf "paperclip: Helm release name %q does not match profile release %q" .Release.Name (.Values.release | default "")) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- end -}}
