{{- define "decluttarr.config" -}}
general:
  log_level: {{ .Values.config.general.logLevel }}
  test_run: {{ .Values.config.general.testRun }}
  timer: {{ .Values.config.general.timer }}
  private_tracker_handling: {{ .Values.config.general.privateTrackerHandling | quote }}
  public_tracker_handling: {{ .Values.config.general.publicTrackerHandling | quote }}
  obsolete_tag: {{ .Values.config.general.obsoleteTag | quote }}
  protected_tag: {{ .Values.config.general.protectedTag | quote }}

job_defaults:
  max_strikes: {{ .Values.config.jobDefaults.maxStrikes }}
  min_days_between_searches: {{ .Values.config.jobDefaults.minDaysBetweenSearches }}
  max_concurrent_searches: {{ .Values.config.jobDefaults.maxConcurrentSearches }}

jobs:
{{- if .Values.config.jobs.removeBadFiles }}
  remove_bad_files:
{{- end }}
{{- if .Values.config.jobs.removeDoneSeeding }}
  remove_done_seeding:
{{- end }}
{{- if .Values.config.jobs.removeFailedDownloads }}
  remove_failed_downloads:
{{- end }}
{{- if .Values.config.jobs.removeFailedImports }}
  remove_failed_imports:
    message_patterns:
{{- range .Values.config.jobs.failedImportMessagePatterns }}
      - {{ . | quote }}
{{- end }}
{{- end }}
{{- if .Values.config.jobs.removeMetadataMissing }}
  remove_metadata_missing:
{{- end }}
{{- if .Values.config.jobs.removeMissingFiles }}
  remove_missing_files:
{{- end }}
{{- if .Values.config.jobs.removeOrphans }}
  remove_orphans:
{{- end }}
{{- if .Values.config.jobs.removeSlow }}
  remove_slow:
    min_speed: {{ .Values.config.jobDefaults.minSpeed }}
    max_strikes: {{ .Values.config.jobDefaults.maxStrikes }}
{{- end }}
{{- if .Values.config.jobs.removeStalled }}
  remove_stalled:
    max_strikes: {{ .Values.config.jobDefaults.maxStrikes }}
{{- end }}
{{- if .Values.config.jobs.removeUnmonitored }}
  remove_unmonitored:
{{- end }}
{{- if .Values.config.jobs.searchUnmetCutoff }}
  search_unmet_cutoff:
    min_days_between_searches: {{ .Values.config.jobDefaults.minDaysBetweenSearches }}
    max_concurrent_searches: {{ .Values.config.jobDefaults.maxConcurrentSearches }}
{{- end }}
{{- if .Values.config.jobs.searchMissing }}
  search_missing:
    min_days_between_searches: {{ .Values.config.jobDefaults.minDaysBetweenSearches }}
    max_concurrent_searches: {{ .Values.config.jobDefaults.maxConcurrentSearches }}
{{- end }}

instances:
  sonarr:
    - base_url: {{ .Values.config.instances.sonarrBaseUrl | quote }}
      api_key: !ENV SONARR_API_KEY
  radarr:
    - base_url: {{ .Values.config.instances.radarrBaseUrl | quote }}
      api_key: !ENV RADARR_API_KEY

download_clients:
  qbittorrent:
    - base_url: {{ .Values.config.downloadClients.qbittorrentBaseUrl | quote }}
      username: !ENV QBITTORRENT_USERNAME
      password: !ENV QBITTORRENT_PASSWORD
      name: {{ .Values.config.downloadClients.qbittorrentName | quote }}
{{- end -}}
