{{- range $extension := list "cube" "earthdistance" "vchord" }}
---
apiVersion: postgresql.sql.crossplane.io/v1alpha1
kind: Extension
metadata:
  name: {{ $.Values.dbName }}-{{ $extension }}
spec:
  deletionPolicy: Delete
  forProvider:
    database: {{ $.Values.dbName }}
    extension: {{ $extension }}
  providerConfigRef:
    name: {{ $.Values.psqlClusterName }}
{{- end }}
