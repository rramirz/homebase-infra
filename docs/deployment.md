# Deployment

## Overview

Deployments are mostly Ansible playbooks and Helm releases applied to Rafael's home Kubernetes/Proxmox environment.

## Local Development

- Preferred command surface: `task --list`
- Default Ansible inventory: `hosts`
- Default SSH user: configured in `ansible.cfg`, with group overrides in inventory/group vars.

## Docker

- Build images for home Kubernetes as `linux/amd64` when building from Apple Silicon.
- Local registry: `registry.theramirez.casa` when used by first-party homelab apps.

## Kubernetes / Infrastructure

- Current kube context: `home-base`.
- Shared ingress uses kgateway Gateway `home` on MetalLB VIP `192.168.1.83`.
- Default StorageClass: OpenEBS local `ssd`.
- Mealie deploy/update: `task mealie:lint`, `task mealie:render`, then `task mealie:upgrade`; release `mealie`, namespace `mealie`, hostname `mealie.theramirez.casa`.
- Plane CE pilot deploy/update: `task plane:lint`, `task plane:render`, `task plane:dry-run` (all read-only/non-mutating), then `task plane:upgrade` (the only mutating step — interactive confirmation prompt, requires context `home-base` and a local `charts/plane/secret-values.yaml`, gitignored). Chart `charts/plane`, release/namespace `plane`, hostnames `plane.theramirez.casa` (app) and `s3.theramirez.casa` (SeaweedFS uploads). **As of 2026-09-01, live revision 5 has all 12 Deployments Available, migrator-5 Complete, both HTTPRoutes Accepted/Resolved, and app HTTP 200.** Resource requests: API `384Mi`/`512Mi`, worker `512Mi`/`768Mi`, beat/live `192Mi`/`256Mi`; total steady-state Deployment requests `1500m` CPU/`2816Mi` memory, below the 3 CPU/10 GiB ceiling. LAN/VPN-only: never add either hostname to the Cloudflare Tunnel. Disposable pilot: no real data until an off-cluster backup/restore path exists (local `ssd` PVCs use reclaim policy `Delete` — namespace/PVC deletion is irreversible). The fixed-name SeaweedFS bucket Job hit its 300-second deadline because `weed shell` needs unexposed gRPC ports `19333`/`18888`; it was deleted after the API confirmed the bucket exists. Fix before the next upgrade; prefer a validated idempotent Filer HTTP API over exposing gRPC solely for the Job. Presigned upload and restore remain unverified.
- Ask before destructive changes, reboots, PVC deletion, storage edits, or Gateway/DNS/firewall changes.
- Family Retirement Planner static checks: `task family-retirement-planner:verify`; PostgreSQL isolation tests: `task family-retirement-planner:integration` (requires Docker). Adjacent chart lives at `apps/family-retirement-planner/chart`. It has not been deployed. LAN/VPN-only and no real data before backup/restore gate.
- Estate inventory shares the same app/database/chart and therefore the same deployment boundary. Never expose it through Cloudflare Tunnel and never store document contents or credentials. Real estate/financial inventory remains prohibited until encrypted off-cluster backup and restore are implemented and tested.
- Paperclip pilot (**deployed** revision 2 post-claim, personal only): chart `charts/paperclip`, one chart with three profiles selected by overlay — `values-personal.yaml` (release/namespace `paperclip-personal`, hostname `paperclip.theramirez.casa`, enabled) plus a post-claim `values-personal-claimed.yaml` overlay, and dormant `values-work.yaml`/`values-logicbroker.yaml` (disabled). Two-phase bootstrap: `task paperclip:bootstrap-deploy` (route-disabled, signup-open) → `task paperclip:bootstrap-port-forward` (one shell) + `task paperclip:bootstrap-invite` (another shell; runs pinned `npx --yes paperclipai@2026.916.1 auth bootstrap-ceo --config <ephemeral /tmp config> --base-url http://localhost:3100` in the app pod (inheriting the pod `DATABASE_URL` env) via an ephemeral `/tmp` config (removed by `trap`, no persistent config mutation), downloading the CLI at runtime and mutating Postgres) → open the generated localhost invite and claim admin → `PAPERCLIP_BOOTSTRAP_CONFIRMED=true task paperclip:upgrade` (post-claim: enables route + disables signup). Local-only checks `task paperclip:test`, `task paperclip:lint` (both phases), `task paperclip:render` (writes `/tmp/paperclip-personal-render.yaml` mode 0600 via `umask 077`, does not print Secret data), `task paperclip:render-dormant`; cluster-facing `task paperclip:dry-run` (`--hide-secret`) → `task paperclip:verify`; rollback `PAPERCLIP_REVISION=N task paperclip:rollback` (warns DB migrations are not reverted). Cluster tasks require context `home-base` + gitignored mode-0600 `charts/paperclip/values-personal.secret-values.yaml` (no `testMode` key). Only the personal profile has mutation tasks. LAN/VPN-only via kgateway `home` `https`; never add to Cloudflare Tunnel. No real employer/Logicbroker/sensitive personal data; no backup/restore yet (local `ssd` reclaim `Delete` — namespace/PVC deletion irreversible); the built-in DB backup currently warns with no recent backup, so the no-real-data rule stays until off-cluster backup + restore exist. Live: release `paperclip-personal` revision 2 (post-claim), both pods Ready/zero restarts, PVCs Bound, HTTPRoute Accepted/Resolved, external-dns A=192.168.1.83, `/api/health` ok; the first install rolled back on a probe `Host` 403 and was fixed with phase-specific probe `Host` headers (`probe.hostHeader`). The pinned authenticated mode prints no startup claim URL; the admin is claimed via the bootstrap invite flow. Namespace separation is not hard isolation (no NetworkPolicy); the Postgres password is immutable after first DB init.

### Kubernetes node time synchronization

Kubernetes nodes must run an enabled, active `chrony` service. The repository
does not manage an external NTP pool; preserve the distribution defaults unless
there is an explicit platform decision to change them. The playbook is
`configure-kubernetes-time-sync.yaml`, targets `home_base`, and uses
`serial: 1` so only one node changes at a time.

Baseline and rollout:

1. Run `task check:kubernetes-time-sync` to confirm the intended inventory
   scope without connecting to hosts.
2. Run `task configure:kubernetes-time-sync-dry-run` and inspect the package
   and service changes. Check mode intentionally skips service start and runtime
   chrony validation because chrony may not exist yet.
3. Abort if inventory is broader than `home_base`, a node is already unhealthy,
   or the dry-run proposes unrelated package/configuration changes.
4. Run the prompt-gated `task configure:kubernetes-time-sync`. It installs
   `chrony` with a one-hour apt-cache validity window, enables/starts the
   service, and validates `systemctl is-active`, `systemctl is-enabled`, and
   `chronyc tracking` after each node.
5. Run `task verify:kubernetes-time-sync` and then the normal Kubernetes health
   checks. No reboot is expected.

If a node has a bad system clock, first prove the RTC is correct and record the
comparison. Only then may an operator use `hwclock --hctosys --utc` as a
one-off repair before rerunning chrony validation. Do not set clocks, reboot,
drain nodes, or use RTC correction as a substitute for proving the time source.
Abort on failed service enablement, failed tracking output, unexpected package
changes, or any Kubernetes readiness regression; leave the next node untouched
until the failed node is diagnosed.

Verified live outcome on 2026-09-10: after proving the RTC correct, the
operator ran `hwclock --hctosys --utc` interactively on home-base5
(`k8s-node-4`, `192.168.1.139`, VM 101 on pve-1). No kubelet restart was
needed; the node became Ready naturally and its lease renewed to current time.
Delayed local-PV workloads rescheduled/started after Flannel recreated
`/run/flannel/subnet.env`; the cluster is now 5/5 Ready. The persistent
`configure-kubernetes-time-sync.yaml` rollout completed on all five `home_base`
nodes. Direct passwordless SSH verification found chrony enabled and active on
all five nodes, valid NTP Reference IDs, `Leap status: Normal`, and system
offsets under 5 ms at verification time. Remaining non-Running pods were
pre-existing and unrelated: ai-memory `qualtrim-sync` missing-secret/
`CreateContainerConfigError`, media-automation `decluttarr` `ContainerCreating`,
and qualtrim-sync `bedrock-trader-paper` `Error`.

### Flannel CNI upgrade

Flannel is a **CRITICAL CNI change**. The repo-managed wrapper at
`charts/flannel` pins the official chart to `v0.28.9`, preserves the live pod
CIDR `10.244.0.0/16`, and leaves the upstream VXLAN default unchanged. Do not
run this workflow during an unrelated cluster incident or maintenance window.
Each Flannel task rebuilds the dependency from `Chart.lock`; the generated
`charts/flannel/charts/` directory is gitignored and is expected to be absent
after a fresh checkout.

Before changing the live release:

1. Keep the cluster at a stable baseline for at least 30 minutes. Confirm all
   nodes are Ready, core DNS and representative workloads are healthy, and
   existing pod-to-pod and pod-to-service netchecks pass.
2. Pre-pull both the current release images and the v0.28.9 images on every
   node. Confirm the new image architecture matches the nodes and that no
   image pull credentials or registry outage will block the DaemonSet.
3. Run the read-only checks in order:
   `task flannel:lint`, `task flannel:render`, `task flannel:diff`, and
   `task flannel:dry-run`.
4. Re-run node, workload, DNS, and pod-to-service netchecks immediately before
   the change. Record the baseline and keep a second operator or console path
   available; CNI failure can break normal cluster access.
5. After the final prompt, run `task flannel:upgrade`. Then run
   `task flannel:verify`, repeat netchecks, and watch all nodes and workloads
   for at least 30 minutes.

Abort before upgrade if any node is NotReady, baseline netchecks fail, the
render/diff contains an unexpected CIDR/backend/resource change, images cannot
be pre-pulled, or the cluster has unrelated instability. Abort after upgrade
if the DaemonSet does not become ready within 10 minutes, any node loses
readiness, or DNS/pod networking fails. Under the documented rollback
procedure, run `task flannel:rollback` to restore Helm revision 1, then run
`task flannel:verify` and repeat the netchecks. Do not use `--atomic`,
`--force`, `--recreate-pods`, or `--reuse-values` for this release.

## CI/CD

- No generic CI/CD pipeline owned by this repo. Routine operations are run locally through Taskfile/Ansible/Helm.
