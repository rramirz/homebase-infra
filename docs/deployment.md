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
