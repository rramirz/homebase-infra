# Paperclip Chart — Architecture

`charts/paperclip` is a reusable third-party Helm chart that deploys one
isolated [Paperclip](https://github.com/paperclipai/paperclip) instance per
Helm release. It is **implemented but not deployed** as of 2026-09-24.

## Purpose

One chart, three profiles on the same home cluster:

| Profile | Overlay file(s) | Status | Release/namespace | Hostname |
|---|---|---|---|---|
| personal | `values-personal.yaml` (+ `values-personal-claimed.yaml` post-claim) | **enabled** (only deployable profile) | `paperclip-personal` | `paperclip.theramirez.casa` |
| work | `values-work.yaml` | dormant (disabled) | `paperclip-work` | `paperclip-work.theramirez.casa` |
| logicbroker | `values-logicbroker.yaml` | dormant (disabled) | `paperclip-logicbroker` | `paperclip-logicbroker.theramirez.casa` |

The base `values.yaml` has `enabled: false` and produces **zero resources** on
its own. The personal profile is deployed in two phases:

- **bootstrap** (`values-personal.yaml`): HTTPRoute disabled, public URL
  `http://localhost:3100`, signup open — reachable only via a local
  port-forward.
- **post-claim** (`values-personal.yaml` + `values-personal-claimed.yaml`):
  HTTPRoute enabled, public URL `https://paperclip.theramirez.casa`, signup
  closed.

## Components (per enabled release)

- **Namespace** — `helm.sh/resource-policy: keep` plus Pod Security Admission
  labels `pod-security.kubernetes.io/{enforce,audit,warn}=baseline`.
- **Secret** (`<fullname>`) — generated `DATABASE_URL` (URL-encoded, spaces as
  `%20`), plus `POSTGRES_PASSWORD`, `BETTER_AUTH_SECRET`,
  `PAPERCLIP_TOOL_ACTION_SIGNING_SECRET`, and `PAPERCLIP_AGENT_JWT_SECRET`.
  Sourced from a gitignored, mode-0600 `values-personal.secret-values.yaml`.
- **PostgreSQL** — headless `Service` + single-replica `StatefulSet` with a
  release-local `volumeClaimTemplate` (10Gi `ssd` RWO). The template PVC is
  retained by StatefulSet lifecycle behavior (not deleted by `helm uninstall`);
  the task workflow must never delete it.
- **Paperclip app** — single-replica `Deployment` (strategy `Recreate`) with a
  dedicated 10Gi `ssd` RWO PVC mounted at `/paperclip`
  (`helm.sh/resource-policy: keep`), plus a `ClusterIP` `Service` on port 3100.
- **HTTPRoute** — LAN/VPN-only route on the kgateway `home` Gateway
  (`namespace: kgateway-system`, `sectionName: https`), gated by
  `httpRoute.enabled` (absent during bootstrap).

## Data model

- **Postgres 17** (`docker.io/library/postgres:17-alpine@sha256:...`) holds
  Paperclip's relational state (users, boards, data). DB/user are both
  `paperclip`; `PGDATA=/var/lib/postgresql/data/pgdata`.
- **`/paperclip`** is the app's persistent home (`PAPERCLIP_HOME`) holding
  Paperclip's own data on the app PVC.
- **The Postgres password is immutable after the initial DB initialization.**
  `docker-entrypoint.sh` only sets the password on first boot of a fresh data
  dir; later Secret changes are ignored by Postgres. Rotating it requires SQL
  `ALTER ROLE` plus a coordinated Secret + app rollout. Helm rollback does NOT
  revert database migrations.
- Storage is local OpenEBS `ssd` (`Delete` reclaim policy). There is **no
  off-cluster backup/restore path yet** — nothing here is durable.

## Auth and bootstrap (two-phase, no claim race)

`PAPERCLIP_DEPLOYMENT_MODE=authenticated` and
`PAPERCLIP_DEPLOYMENT_EXPOSURE=private`. The pinned authenticated mode does
**not** print a startup claim URL; the admin identity is bootstrapped
explicitly:

1. `task paperclip:bootstrap-deploy` — installs the bootstrap stack (no
   HTTPRoute, signup open, public URL `http://localhost:3100`).
2. `task paperclip:bootstrap-port-forward` — `kubectl port-forward
   svc/paperclip-personal 3100:3100` in one shell.
3. `task paperclip:bootstrap-invite` — in another shell, runs a script inside
   the app pod that writes an **ephemeral** config to `/tmp` (never
   `/paperclip`) and invokes the pinned CLI
   `npx --yes paperclipai@2026.916.1 auth bootstrap-ceo --config <tmp>
   --base-url http://localhost:3100`. The CLI inherits the pod's
   `DATABASE_URL` env (no `--db-url` flag — the CLI resolves the database once
   the valid config loads). The temp config is removed by a `trap`;
   `/paperclip` stays app data only. The script captures combined output,
   extracts the `http://localhost:3100/invite/pcp_bootstrap_*` URL, and exits
   nonzero with a safe diagnostic if it is absent — it never trusts the CLI's
   exit code, which can be 0 without writing an invite.
4. Open the invite, sign up / claim the admin.
5. `PAPERCLIP_BOOTSTRAP_CONFIRMED=true task paperclip:upgrade` — applies the
   post-claim overlay: enables the HTTPRoute and disables signup.

**Why the ephemeral config is required:** the pinned CLI reads `config.json`
before resolving the database, and env-only app startup does not create
`/paperclip/instances/default/config.json`, so the previous direct invocation
returned exit 0 without emitting an invite. The workaround writes a minimal
valid config (`$meta.version=1`, `$meta.source=configure`, valid ISO
`$meta.updatedAt`, `database.mode=postgres`, `logging.mode=file`,
`server.deploymentMode=authenticated`) to `/tmp` and passes it via `--config`,
leaving no persistent config mutation.

`PAPERCLIP_AUTH_DISABLE_SIGN_UP` is `"false"` during bootstrap and `"true"`
after the claim. `PAPERCLIP_AUTH_RATE_LIMIT_ENABLED=true` and
`PAPERCLIP_AGENT_JWT_DISABLE_LEGACY_FALLBACK=true` are always set, with
`PAPERCLIP_AGENT_JWT_SECRET` backed by its own distinct secret.

## Request / data flow

1. **Post-claim:** client resolves `<hostname>` → kgateway Gateway `home` VIP
   `192.168.1.83` (LAN/VPN only); Gateway terminates TLS on the `https`
   listener and forwards to the app `Service:3100` via the HTTPRoute.
2. **Bootstrap:** the operator forwards `localhost:3100` directly to the app
   Service; no HTTPRoute exists.
3. The app serves UI (`SERVE_UI=true`) and API on `:3100`; startup/readiness/
   liveness probe `GET /api/health`. Each probe sends an explicit `Host` header
   (`probe.hostHeader`: `localhost:3100` in bootstrap, the hostname post-claim)
   because Paperclip's authenticated/private mode rejects the pod-IP `Host`
   that kubelet sends by default with HTTP 403.
4. The app reaches Postgres via `DATABASE_URL` at `<fullname>-postgres:5432`.

## Security posture

- `automountServiceAccountToken: false`, `seccompProfile: RuntimeDefault`,
  `allowPrivilegeEscalation: false`. `readOnlyRootFilesystem`/`runAsNonRoot`
  are intentionally **not** forced yet (unverified against the upstream image).
- **Capabilities are NOT dropped** (`capabilities.drop: ["ALL"]` is absent)
  because the pinned Paperclip image's root entrypoint chowns `/paperclip`
  before dropping privileges (chown-then-gosu), and the Postgres entrypoint
  chowns PGDATA before gosu; dropping all caps would remove `CAP_CHOWN` and
  break startup.
- **Namespace separation is NOT hard isolation.** There is no NetworkPolicy,
  so a per-release namespace does not prevent cross-namespace network access.
  `work`/`logicbroker` remain blocked pending a policy-capable network
  isolation decision (e.g. NetworkPolicy); do not treat them as deployable
  until that is resolved.

## Failure modes

- **Wrong namespace / hostname / public-URL / non-SHA256-pinned image** → `helm`
  render fails via `paperclip.validate` regardless of `testMode`. Image digests
  must match `^sha256:[0-9a-f]{64}$`.
- **Wrong release name** → render fails on the strict path only (the
  release-name check is skipped in `testMode` because `helm lint` cannot set
  one).
- **Empty, short (<32 char), or non-distinct secrets** → render fails in both
  modes. The Better Auth, tool-signing, and agent-JWT secrets must be pairwise
  distinct; the Postgres password must be >=32 chars.
- **`testMode`** is static-validation-only: it accepts only the exact
  `static-validation-only-*` placeholder shape and skips only the release-name
  check. The committed example carries `testMode: true`; the live secret file
  must omit the key entirely.
- **`testMode` in a live secret file** → `bootstrap-deploy`/`render`/`dry-run`/
  `upgrade` refuse via `tests/validate-secret-file.sh` (mode 0600 + no
  `testMode` key).
- **State loss** — deleting the namespace or a PVC is irreversible (local `ssd`
  `Delete`). Postgres data survives `helm uninstall` only because of the
  volumeClaimTemplate retention, not because of any backup.

## Deliberate non-goals

- No HA (single control plane, single replica each).
- No backup/restore (blocking gate before any real data).
- No Cloudflare Tunnel / public ingress.
- No NetworkPolicy/monitoring/alerting for this pilot yet (which is why
  namespace separation is not hard isolation).
- No work/Logicbroker deployment task (dormant overlays only).
- No OIDC/Authentik integration yet.

## Bootstrap CLI supply-chain tradeoff

`paperclip:bootstrap-invite` runs `npx --yes paperclipai@2026.916.1`, which
downloads the npm CLI at runtime from the registry on first use. The version is
pinned exactly (`2026.916.1`) to avoid a floating tag, but the download is still
an unpinned-by-digest, runtime supply-chain fetch. Accepting it trades
determinism for not vendoring the CLI into the repo. It is the one runtime
download in the bootstrap path; the app and Postgres images remain
digest-pinned.
