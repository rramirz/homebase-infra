# App Database (Crossplane `DB` claim)

Rule: apps deployed to the home cluster use a Helm chart. App DBs on shared Postgres `home-db` (`charts/psql-cluster`, `192.168.1.81`) come from the Crossplane claim `DB` (`theramirez.casa/v1alpha1`, composite `XDB`, Composition `charts/crossplane/templates/xdb-composition.yaml`). Do NOT put CNPG `Cluster` CRDs in app charts unless the app needs isolation (own PG major, extensions, blast radius). Exception example: `charts/mattermost`.

New Go app: `task app:new NAME=my-app [DB=yes|no]` (cookiecutter `templates/go-app` -> `apps/<name>/`, already includes the DB claim and secret wiring).

Third-party app: `task chart:new NAME=<chart>` (`~/homespace/cookiecutter-helm-template`: searches Helm repos, you pick version, generates an umbrella chart in `charts/<name>/` with `values.yaml` seeded from upstream under key `<chart>:`, plus `templates/database.yaml` DB claim(s)). Set `psqlClusterName: home-db` and `databases:` in values; point the app's DB env/secret at `home-db-db-<dbName>`.

## What the Composition creates
- `Role` named `<dbName>` (login, connectionLimit 50) in `home-db`.
- `Database` named `<dbName>`, owner = role.
- Connection secret `<psqlClusterName>-db-<dbName>` in the claim's namespace (written by the Role; keys from provider-sql: `username`, `password`, `endpoint`, `port`).

## Chart template (`templates/database.yaml`)
```yaml
{{- if .Values.db.enabled }}
apiVersion: theramirez.casa/v1alpha1
kind: DB
metadata:
  name: {{ .Release.Name }}-db
  namespace: {{ .Release.Namespace }}
spec:
  dbName: {{ .Values.db.name | default .Release.Name }}
  psqlClusterName: {{ .Values.db.psqlClusterName | default "home-db" }}
  encoding: UTF8
  lcCType: C
  lcCollate: C
{{- end }}
```
`values.yaml`:
```yaml
db:
  enabled: true
  name: ""            # defaults to release name
  psqlClusterName: home-db
```

## Consuming in the Deployment
Secret name = `{{ .Values.db.psqlClusterName }}-db-{{ .Values.db.name | default .Release.Name }}`.
```yaml
env:
  - name: DB_USER
    valueFrom: {secretKeyRef: {name: <secret>, key: username}}
  - name: DB_PASSWORD
    valueFrom: {secretKeyRef: {name: <secret>, key: password}}
  - name: DB_HOST
    value: home-db-rw.postgres.svc  # confirm ns: kubectl get svc -A | grep home-db-rw
```
First install: secret appears after Crossplane reconciles (~seconds). Pod stays `CreateContainerConfigError` until then and self-heals; no hook needed. Verify the secret keys with `kubectl get secret <secret> -o json | jq '.data|keys'` (never print values).

## Verify
`kubectl get db,xdb -A`, `kubectl get database.postgresql.sql,role.postgresql.sql`.

## Known gaps
- The XRD (`xdbs.theramirez.casa`) is not in the repo; only the Composition is. Commit it to `charts/crossplane/templates/` so the claim schema is reviewable.
- Composition hardcodes Role `connectionLimit: 50` and has no password rotation.
