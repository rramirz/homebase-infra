apiVersion: v1
kind: PersistentVolume
metadata:
  name: {{ .Release.Namespace }}-{{ .Release.Name }}-external-photos
spec:
  capacity:
    storage: {{ .Values.externalLibraries.photos.size }}
  accessModes:
    - ReadOnlyMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: {{ .Values.externalLibraries.photos.storageClassName }}
  mountOptions:
    - nfsvers=4.1
  nfs:
    server: {{ .Values.externalLibraries.photos.nfs.server }}
    path: {{ .Values.externalLibraries.photos.nfs.path }}
    readOnly: true
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: immich-external-photos
spec:
  accessModes:
    - ReadOnlyMany
  storageClassName: {{ .Values.externalLibraries.photos.storageClassName }}
  resources:
    requests:
      storage: {{ .Values.externalLibraries.photos.size }}
  volumeName: {{ .Release.Namespace }}-{{ .Release.Name }}-external-photos
