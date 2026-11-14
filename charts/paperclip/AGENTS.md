# Paperclip Chart — Agent Constraints

Local-only chart. **Personal (`paperclip-personal`) is live at revision 2 post-claim (`https://paperclip.theramirez.casa`); `work` and `logicbroker` remain dormant.**

## Identity & Overlays

- Base `values.yaml`: `enabled: false` (renders zero resources alone).
- `values-personal.yaml`: bootstrap (`httpRoute.enabled: false`, `PAPERCLIP_PUBLIC_URL=http://localhost:3100`, signup open).
- `values-personal-claimed.yaml`: layered on `values-personal.yaml` post-claim (`httpRoute.enabled: true`, `https://paperclip.theramirez.casa`, signup closed).
- `values-work.yaml` / `values-logicbroker.yaml`: dormant (`enabled: false`).

## Hard Rules

- **Context guard:** `test`/`lint`/`render`/`render-dormant` never contact the cluster. Mutation/cluster tasks (`bootstrap-deploy`/`bootstrap-invite`/`bootstrap-port-forward`/`dry-run`/`upgrade`/`verify`/`rollback`) require `kubectl config current-context == home-base`. Only `personal` has mutation tasks.
- **Secrets:** Real secrets live only in gitignored mode-0600 `values-personal.secret-values.yaml` (`secrets.postgres.password`, `betterAuthSecret`, `toolActionSigningSecret`, `agentJwtSecret` — all >=32 chars, last 3 pairwise distinct; `DATABASE_URL` encodes spaces as `%20`). `secret-values.example.yaml` has `testMode: true` (`static-validation-only-*` placeholders, skips only release-name check); live tasks reject any file with `testMode` via `tests/validate-secret-file.sh`.
- **LAN/VPN-only:** Never add `paperclip*.theramirez.casa` to Cloudflare Tunnel.
- **Isolation & persistence:** Each release has its own namespace, Secret, Postgres 17 `StatefulSet` (retained `volumeClaimTemplate`), and 10Gi `ssd` `/paperclip` PVC. Namespace separation is not hard network isolation (no NetworkPolicy). Never delete the Postgres PVC; Postgres password is immutable after first init without `ALTER ROLE`.
- **Image pins & securityContext:** Images pinned by `sha256` digest in `image-lock.yaml`. Do not drop `CHOWN`/`SETUID` capabilities (entrypoints chown before `gosu`); only `allowPrivilegeEscalation: false`.
- **No real data** until encrypted off-cluster backup and tested restore exist (`ssd` reclaim policy is `Delete`).
- **Probe Host header:** All probes send `Host: <probe.hostHeader>` (`localhost:3100` bootstrap, `paperclip.theramirez.casa` post-claim) so kubelet pod-IP probes don't get HTTP 403.
- **Signup UI caveat:** Upstream commit `d554c47` still renders the `/auth` registration form when `PAPERCLIP_AUTH_DISABLE_SIGN_UP=true`; verify server-side before treating the visible form as open signup.

## Workflow

- Local: `task paperclip:{test,lint,render-dormant}`
- Bootstrap: `task paperclip:bootstrap-deploy` → `task paperclip:bootstrap-port-forward` + `task paperclip:bootstrap-invite` (runs `npx --yes paperclipai@2026.916.1 auth bootstrap-ceo` in pod) → claim admin → `PAPERCLIP_BOOTSTRAP_CONFIRMED=true task paperclip:upgrade` → `task paperclip:verify`.
- Rollback: `PAPERCLIP_REVISION=N task paperclip:rollback` (does not revert DB migrations).
