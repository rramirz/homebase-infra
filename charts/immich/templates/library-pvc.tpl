apiVersion: v1
kind: PersistentVolume
metadata:
  name: {{ .Release.Namespace }}-{{ .Release.Name }}-library
spec:
  capacity:
    storage: {{ .Values.library.size }}
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: {{ .Values.library.storageClassName }}
  mountOptions:
    - nfsvers=4.1
  nfs:
    server: {{ .Values.library.nfs.server }}
    path: {{ .Values.library.nfs.path }}
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: immich-library
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: {{ .Values.library.storageClassName }}
  resources:
    requests:
      storage: {{ .Values.library.size }}
  volumeName: {{ .Release.Namespace }}-{{ .Release.Name }}-library
