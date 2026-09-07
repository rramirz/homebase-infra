# Plane CE v1.3.1 Disposable Pilot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `subagent-driven-development` to implement this plan task-by-task. Execute only assigned tasks. Do not commit, stage, push, or create a PR unless Rafael later asks.

**Goal:** Deploy a disposable Plane CE v1.3.1 pilot to the `home-base` Kubernetes cluster at `https://plane.theramirez.casa`.

**Architecture:** A local Helm chart reproduces Plane's Community Docker Compose topology in namespace `plane`. Plane's Caddy proxy remains the only app ingress target. SeaweedFS 4.39 replaces MinIO and receives browser uploads through a separate S3 hostname. All state uses local OpenEBS `ssd` PVCs; therefore this pilot must contain no real data until off-cluster backup and restore are implemented and tested.

**Tech Stack:** Kubernetes, Helm, Taskfile, kgateway Gateway API, OpenEBS local PVCs, Plane CE v1.3.1, PostgreSQL 15.7, Valkey 7.2.11, RabbitMQ 3.13.6, SeaweedFS 4.39, Caddy.

## Global Constraints

- Target repo: `/Users/rafael.ramirez/homespace/homebase-infra`.
- Target context: `home-base`; release and namespace: `plane`.
- The repo is materially dirty/untracked. Modify only files listed in this plan. Never reformat or clean unrelated work.
- Disposable pilot only. No real workspaces, tasks, or employer data.
- No MinIO, shared `home-db`, CNPG, Authentik, monitoring changes, n8n/router/bridge, Jira integration, Cloudflare Tunnel, or backup claim.
- Never add `plane.theramirez.casa` or `s3.theramirez.casa` to the Cloudflare Tunnel.
- OpenEBS `ssd` uses node-local storage and reclaim policy `Delete`. Never uninstall the release, delete namespace/PVCs, or run destructive restore tests without explicit approval.
- Pin every runtime image by version and OCI digest. No `latest` or tag-only runtime images.
- Total steady-state resource requests must remain below 3 CPU and 10 GiB memory.
- Use only documented Plane HTTP probes: API `/` and live `/health/`. Use TCP/process probes elsewhere.
- Preserve Plane's official Caddy routing contract. `plane.theramirez.casa` routes only to Caddy; `s3.theramirez.casa` routes only to SeaweedFS.
- Use `USE_MINIO=0`, bucket `plane-uploads`, and a browser-reachable S3 endpoint.
- Do not commit. User authorized deployment, not Git history changes.

---

## Task 1: Preflight and Image Lock

**Files:**
- Create: `charts/plane/image-lock.yaml`

**Produces:** Verified immutable image references and evidence cluster prerequisites exist.

- [ ] Confirm correct cluster and capacity:

  ```bash
  kubectl config current-context
  kubectl get nodes -o wide
  kubectl top nodes
  ```

  Expected: context `home-base`; schedulable amd64 nodes; at least 3 CPU and 10 GiB aggregate headroom.

- [ ] Confirm required platform objects:

  ```bash
  kubectl get storageclass ssd
  kubectl get gateway home -n kgateway-system
  kubectl get secret theramirez-casa-wildcard-tls -n kgateway-system
  task check:kubernetes-health
  ```

  Expected: `ssd` exists; Gateway `home` is Programmed; wildcard TLS Secret exists; health task returns no blocking failure.

- [ ] Resolve and record the multi-arch OCI digest for `chrislusf/seaweedfs:4.39` using `docker buildx imagetools inspect` or `crane digest`. Reject mutable/tag-only deployment.

- [ ] Create `charts/plane/image-lock.yaml` containing these reviewed OCI index digests:

  ```yaml
  plane:
    backend: sha256:2cdcb5f778c6ccacebce0e5a751d39fac4a549a44e049a5b110a7623cfdad139
    frontend: sha256:c178fd85c4588165262cfe748bd103fdeccebbbab827c53e71b9ce32fff84f86
    space: sha256:e08c2c8741ae6f81a9326dc9201e7e09c0c411b87a20198f9ea9e5cf2fae3488
    admin: sha256:ff9219127a2c2c4a4bb066d6a0e25a5fc6a11204cf8484b521dada53d696fe43
    live: sha256:2073b6950a394545ea1db6ed4157e951ad5a6e1881e74f4f9238ace6c35bbf3d
    proxy: sha256:b4f8bb6998052dcd1488171a90d473674cefbc6ec77114d5095b48c805f4ad27
  postgres: sha256:468d34fefd6338031787c7b8e94078975b3aaf4d66c7ead25c39cd3ba46a15c6
  valkey: sha256:10328d00120dc14fbc87b2ed61b7677ddbb0d011e705361b4788329a0ec69a93
  rabbitmq: sha256:611107e29cce05c2acd968325d5dcbde7e2fee404970f1ead75fdb22be2821b3
  seaweedfs: REPLACE_WITH_VERIFIED_SHA256
  ```

- [ ] Verify no placeholder remains before Task 2:

  ```bash
  ! grep -R 'REPLACE_WITH\|latest' charts/plane/image-lock.yaml
  ```

---

## Task 2: Chart Skeleton and Local Secrets

**Files:**
- Create: `charts/plane/Chart.yaml`
- Create: `charts/plane/values.yaml`
- Create: `charts/plane/secret-values.example.yaml`
- Create locally, gitignored: `charts/plane/secret-values.yaml`
- Create: `charts/plane/templates/_helpers.tpl`
- Create: `charts/plane/templates/namespace.yaml`
- Create: `charts/plane/templates/secrets.yaml`
- Create: `charts/plane/templates/configmap.yaml`

**Produces:** Valid chart contract and generated credentials without secret leakage.

- [ ] Define chart metadata: chart `plane` version `0.1.0`, app version `v1.3.1`.
- [ ] Define committed values for domains, images, ports, `ssd` PVC sizes, probes, resource requests/limits, and `disposablePilot: true`.
- [ ] Set Plane configuration:

  ```text
  WEB_URL=https://plane.theramirez.casa
  CORS_ALLOWED_ORIGINS=https://plane.theramirez.casa
  USE_MINIO=0
  AWS_REGION=homebase
  AWS_S3_ENDPOINT_URL=https://s3.theramirez.casa
  AWS_S3_BUCKET_NAME=plane-uploads
  FILE_SIZE_LIMIT=5242880
  ```

- [ ] Generate unique Postgres, RabbitMQ, Plane `SECRET_KEY`, live-server, and SeaweedFS S3 credentials into `secret-values.yaml` with `umask 077`; `chmod 600` the file. Never print values.
- [ ] Confirm ignore behavior:

  ```bash
  git check-ignore charts/plane/secret-values.yaml
  ```

  Expected: path printed and exit 0.

- [ ] Render `DATABASE_URL` and `AMQP_URL` in the Kubernetes Secret, not ConfigMap. Do not rely on shell substitution inside Kubernetes env values.
- [ ] Run:

  ```bash
  helm lint charts/plane -f charts/plane/values.yaml -f charts/plane/secret-values.yaml
  ```

  Expected: `0 chart(s) failed`.

---

## Task 3: Stateful Dependencies

**Files:**
- Create: `charts/plane/templates/postgres.yaml`
- Create: `charts/plane/templates/valkey.yaml`
- Create: `charts/plane/templates/rabbitmq.yaml`
- Create: `charts/plane/templates/seaweedfs.yaml`
- Create: `charts/plane/templates/seaweedfs-bucket-job.yaml`

**Produces:** Four isolated single-instance services with PVC persistence and authenticated S3 uploads.

- [ ] Implement one Deployment, ClusterIP Service, and RWO `ssd` PVC each for Postgres, Valkey, RabbitMQ, and SeaweedFS.
- [ ] Use Recreate strategy for PVC-owning Deployments and explicit requests/limits.
- [ ] Configure native TCP/exec readiness checks: `pg_isready`, Valkey `PING`, RabbitMQ diagnostics, SeaweedFS S3 TCP `8333`.
- [ ] Run SeaweedFS 4.39 all-in-one with filer and S3 enabled, one `/data` PVC, S3 auth config mounted from Secret, and CORS origin exactly `https://plane.theramirez.casa`.
- [ ] Create `plane-uploads` with an idempotent normal Job. It must wait for SeaweedFS and succeed when the bucket already exists.
- [ ] Render and assert exactly four PVCs use `storageClassName: ssd`; inspect generated Secret output without displaying secret values.

---

## Task 4: Plane Application and Migration Gate

**Files:**
- Create: `charts/plane/templates/migrator.yaml`
- Create: `charts/plane/templates/api.yaml`
- Create: `charts/plane/templates/worker.yaml`
- Create: `charts/plane/templates/beat.yaml`
- Create: `charts/plane/templates/web.yaml`
- Create: `charts/plane/templates/space.yaml`
- Create: `charts/plane/templates/admin.yaml`
- Create: `charts/plane/templates/live.yaml`

**Produces:** Full Plane CE app tier that cannot start DB-dependent processes before migrations complete.

- [ ] Implement migrator as a normal idempotent Job, not a pre-install hook. It waits for Postgres and Valkey, then runs Plane's official `docker-entrypoint-migrator.sh`.
- [ ] Add migration init-container gate to API, worker, and beat. Poll `django_migrations` using `psql`; do not start until a row exists.
- [ ] Use official entrypoints unchanged: API, worker, beat, and migrator scripts from Plane backend image.
- [ ] Implement web, space, admin, API, live Services. Worker and beat need no Service.
- [ ] Use API `GET /` and live `GET /health/` probes only. Use TCP probes for web/space/admin; process checks for Celery worker/beat.
- [ ] Render and verify migration gate appears only on API/worker/beat and no `pre-install` hook exists.

---

## Task 5: Caddy and Gateway Routes

**Files:**
- Create: `charts/plane/templates/proxy.yaml`
- Create: `charts/plane/templates/httproutes.yaml`

**Produces:** TLS application and S3 endpoints without reimplementing Plane routing.

- [ ] Deploy pinned Plane proxy image and its official Caddy configuration/entrypoint. kgateway terminates TLS; Caddy listens internally on HTTP.
- [ ] Route `plane.theramirez.casa` only to `plane-proxy`.
- [ ] Route `s3.theramirez.casa` only to SeaweedFS port `8333`.
- [ ] Attach both HTTPRoutes to `kgateway-system/home`, section `https`.
- [ ] Do not add Cloudflare Tunnel resources, Certificate objects, or second-level wildcard hosts.
- [ ] Render and inspect both HTTPRoutes for `Accepted`/`ResolvedRefs`-compatible backend names and ports.

---

## Task 6: Taskfile and Static Validation

**Files:**
- Modify surgically: `Taskfile.yaml`

**Produces:** Repeatable lint, render, dry-run, and deploy commands.

- [ ] Add `plane:lint`, `plane:render`, `plane:dry-run`, and `plane:upgrade` tasks using both values files.
- [ ] Add precondition requiring local `charts/plane/secret-values.yaml`.
- [ ] Make `plane:upgrade` prompt that this is disposable and accepts no real data.
- [ ] Run:

  ```bash
  task plane:lint
  task plane:render
  task plane:dry-run
  ```

  Expected: lint succeeds; rendered YAML contains no unresolved placeholders; server dry-run exits 0 without cluster mutation.

---

## Task 7: Deploy and Verify Disposable Pilot

**Files:** None.

**Produces:** Healthy Plane pilot with persisted throwaway state.

- [ ] Re-run read-only preflight and capture failures before change.
- [ ] Deploy:

  ```bash
  task plane:upgrade
  ```

- [ ] Verify workloads and storage:

  ```bash
  kubectl -n plane get pods,pvc,svc,jobs,httproute -o wide
  kubectl -n plane wait --for=condition=complete job/plane-migrator --timeout=600s
  kubectl -n plane wait --for=condition=complete job/plane-seaweedfs-bucket --timeout=300s
  ```

  Expected: Deployments Ready, four PVCs Bound, Jobs Complete, both HTTPRoutes Accepted and ResolvedRefs.

- [ ] Verify public routes:

  ```bash
  curl -fsS -o /dev/null -w '%{http_code}\n' https://plane.theramirez.casa/
  curl -fsS https://plane.theramirez.casa/api/
  curl -fsS https://plane.theramirez.casa/live/health/
  ```

  Expected: UI 200; API returns Plane response; live health 200.

- [ ] Create only a throwaway workspace/project through UI. Upload a small non-sensitive file. Confirm browser upload targets `https://s3.theramirez.casa/plane-uploads/...`, POST succeeds, CORS permits only Plane origin, and object is retrievable.
- [ ] If boto emits virtual-hosted URL `plane-uploads.s3.theramirez.casa`, stop. Do not add wildcard DNS/cert. First force path-style through supported boto/django-storages configuration in a reviewed follow-up.
- [ ] Restart Postgres and SeaweedFS pods only, not PVCs; verify throwaway workspace and uploaded file survive. Pod deletion is allowed here; PVC/namespace deletion is not.
- [ ] Run `task check:kubernetes-health` after deployment and record any unrelated pre-existing failures separately.

---

## Task 8: Documentation and Memory

**Files:**
- Modify surgically: `AGENTS.md`
- Modify surgically: `docs/architecture.md`
- Modify surgically: `docs/deployment.md`
- Modify surgically: `docs/memory/current-work.md`

**Produces:** Future agents understand deployed state, safety boundary, and next gate.

- [ ] Document Plane components, namespace, domains, storage, exact versions/digests, request totals, and local Helm workflow.
- [ ] Record hard rules: LAN/VPN-only; never Cloudflare Tunnel; no real data; local PVC deletion is irreversible; backup/restore is next blocking task.
- [ ] Record actual validation results and anything not verified. Do not claim backup, HA, SSO, monitoring, or agent routing.
- [ ] Review diffs only for declared paths:

  ```bash
  git status --short
  git diff -- Taskfile.yaml AGENTS.md docs/architecture.md docs/deployment.md docs/memory/current-work.md
  ```

- [ ] Do not commit. Hand Rafael exact changed-file list and live validation evidence.

---

## Completion Gate

Pilot is complete only when:

- Helm lint, render, and server dry-run pass.
- Migrator and bucket Jobs complete.
- All app and dependency workloads are Ready.
- Four `ssd` PVCs are Bound.
- App and S3 HTTPRoutes are Accepted and ResolvedRefs.
- UI, API, and live health checks pass.
- One non-sensitive presigned POST upload succeeds through SeaweedFS.
- Throwaway database row and upload survive pod restarts.
- Documentation states disposable/no-real-data boundary.
- No secrets appear in Git status/diff.
- No unrelated repo files changed.

## Rollback

- Failed upgrade with prior revision: `helm rollback plane <revision> -n plane`.
- Do not run `helm uninstall`, delete namespace, or delete PVCs without explicit approval.
- Because backup is deferred, full teardown permanently destroys pilot state. This is acceptable only while all data remains deliberately disposable.
