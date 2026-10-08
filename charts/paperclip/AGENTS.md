# Paperclip Chart — Agent Constraints

Local-only chart implementation. **Personal is live at revision 2 post-claim; work
and Logicbroker remain dormant.**

## Identity model

One chart, three profiles selected by overlay file. Personal is deployed in two
phases:

- `values-personal.yaml` → `paperclip-personal` / `paperclip.theramirez.casa`
  (**bootstrap**: HTTPRoute disabled, `PAPERCLIP_PUBLIC_URL=http://localhost:3100`,
  signup open) — the enabled profile.
- `values-personal-claimed.yaml` → applied ON TOP of personal (**post-claim**:
  HTTPRoute enabled, `PAPERCLIP_PUBLIC_URL=https://paperclip.theramirez.casa`,
  signup closed).
- `values-work.yaml` → `paperclip-work` (dormant, disabled).
- `values-logicbroker.yaml` → `paperclip-logicbroker` (dormant, disabled).

Base `values.yaml` has `enabled: false` and renders zero resources alone.

## Hard rules

- **Never contact or mutate the cluster** from `test`/`lint`/`render`/
  `render-dormant`. Cluster-facing tasks (`bootstrap-deploy`/`bootstrap-invite`/
  `bootstrap-port-forward`/`dry-run`/`upgrade`/`verify`/`rollback`) require
  `kubectl config current-context == home-base`.
- **Never commit real secrets.** Real values live only in the gitignored,
  mode-0600 `values-personal.secret-values.yaml`. The committed
  `secret-values.example.yaml` carries `testMode: true` + `static-validation-only-*`
  placeholders and must never be deployed.
- **`testMode` is static-validation-only.** It accepts only the exact
  `static-validation-only-*` placeholder shape and skips ONLY the release-name
  check (`helm lint` cannot set a release name). It does not bypass namespace,
  hostname/public-URL, image-pin, secret-length, or pairwise-distinct
  validation. Live tasks reject any secret file that contains a `testMode` key
  at all (via `tests/validate-secret-file.sh`: mode 0600 + no `testMode` key).
- **Never deploy work or Logicbroker.** No mutation task exists for those
  profiles. Only personal has `bootstrap-deploy`/`dry-run`/`upgrade`/`verify`/
  `rollback`.
- **No Cloudflare Tunnel / public ingress.** The hostname is LAN/VPN-only via
  the kgateway `home` Gateway `https` listener (post-claim only).
- **Namespace separation is NOT hard isolation.** There is no NetworkPolicy.
  Work/Logicbroker remain blocked pending a policy-capable network-isolation
  decision.
- **No shared DB/PVC/credentials/workspace** between releases. Each release gets
  its own namespace, Secret, Postgres StatefulSet, and `/paperclip` PVC.
- **No floating images.** Paperclip `ghcr.io/paperclipai/paperclip:sha-d554c47`
  and Postgres `docker.io/library/postgres:17-alpine` are pinned by linux/amd64
  digest in `image-lock.yaml`; templates refuse non-`sha256:[0-9a-f]{64}`
  digests.
- **Do not drop capabilities.** The pinned images' root entrypoints chown their
  data dirs before gosu (CHOWN is required); only `allowPrivilegeEscalation`
  is disabled.
- **No real data** until encrypted off-cluster backup and tested restore exist.
  Local `ssd` reclaim policy is `Delete`; namespace/PVC deletion is irreversible.
- **Never delete the Postgres volumeClaimTemplate PVC.** It is retained by
  StatefulSet lifecycle behavior and is the only persistence.
- **The Postgres password is immutable after the first DB init.** Rotation needs
  SQL `ALTER ROLE` + a coordinated Secret/app rollout. Helm rollback does not
  revert migrations.
- **Do not force `readOnlyRootFilesystem` or `runAsNonRoot`** until verified
  against the upstream image.

## Required secrets

`secrets.postgres.password` (>=32 chars), `secrets.betterAuthSecret`,
`secrets.toolActionSigningSecret`, `secrets.agentJwtSecret` — the last three
must be pairwise distinct and >=32 chars. `DATABASE_URL` is generated in the
Secret template from the postgres user/password/host, with spaces encoded as
`%20` (not `+`).

## App contract

Port 3100, probes `GET /api/health`, env `PORT=3100`, `SERVE_UI=true`,
`PAPERCLIP_HOME=/paperclip`, `PAPERCLIP_DEPLOYMENT_MODE=authenticated`,
`PAPERCLIP_DEPLOYMENT_EXPOSURE=private`, `PAPERCLIP_MIGRATION_AUTO_APPLY=true`,
`PAPERCLIP_AUTH_RATE_LIMIT_ENABLED=true`,
`PAPERCLIP_AGENT_JWT_DISABLE_LEGACY_FALLBACK=true`,
`PAPERCLIP_AUTH_DISABLE_SIGN_UP` (`false` bootstrap / `true` post-claim),
`PAPERCLIP_PUBLIC_URL` (localhost bootstrap / https post-claim).

Every probe (`startupProbe`/`readinessProbe`/`livenessProbe`) sends an explicit
`Host` header via `httpHeaders: [{name: Host, value: <probe.hostHeader>}]`.
`probe.hostHeader` is phase-specific and validated: `localhost:3100` when the
route is disabled (bootstrap), the hostname when enabled (post-claim). Without
this, kubelet sends the pod IP as `Host`, which Paperclip's
authenticated/private mode rejects with HTTP 403.

## Workflow

Local: `task paperclip:test` → `task paperclip:lint` →
`task paperclip:render-dormant`.

Bootstrap: `task paperclip:bootstrap-deploy` (route-disabled) →
`task paperclip:bootstrap-port-forward` (one shell) →
`task paperclip:bootstrap-invite` (another shell; runs a script in the app pod
that writes an ephemeral `/tmp` config and invokes the pinned
`npx --yes paperclipai@2026.916.1 auth bootstrap-ceo --config <tmp>
--base-url http://localhost:3100`, inheriting the pod `DATABASE_URL` env (no
`--db-url`), downloading the CLI at runtime and mutating Postgres; the temp
config is removed by `trap`, and the script fails if no
`http://localhost:3100/invite/pcp_bootstrap_*` URL is emitted) → open the
generated localhost invite and claim the admin.

Post-claim: `PAPERCLIP_BOOTSTRAP_CONFIRMED=true task paperclip:upgrade` (enables
route + disables signup) → `task paperclip:verify`. Rollback:
`PAPERCLIP_REVISION=N task paperclip:rollback` (warns migrations are not
reverted).

## Signup UI caveat

Post-claim revision 2 sets `PAPERCLIP_AUTH_DISABLE_SIGN_UP=true`, but upstream
commit `d554c4789ed3930f8a53ac9fdf6503b3187097da` still shows `Create one` and
the registration form on unauthenticated `/auth`. This is an upstream UX
mismatch; no valid registration attempt was performed. An empty `{}` signup
POST returned HTTP 400 validation before proving the disable guard. Signup is
intended closed; require server-side verification before treating the visible
form as a security issue.
