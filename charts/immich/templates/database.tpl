apiVersion: theramirez.casa/v1alpha1
kind: DB
metadata:
  name: {{ .Release.Name }}-db
spec:
  dbName: {{ .Values.dbName }}
  psqlClusterName: {{ .Values.psqlClusterName }}
  encoding: {{ .Values.dbEncoding }}
  lcCType: {{ .Values.dbLcCType }}
  lcCollate: {{ .Values.dbLcCollate }}
