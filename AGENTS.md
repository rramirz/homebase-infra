# Homebase Infra — Agent Context

## What This Repo Is

`homebase-infra` manages Rafael's home-base infrastructure using Ansible plus in-repo third-party/umbrella Helm charts. Treat the environment like a small production platform: reliability matters, but practical low-maintenance solutions matter more than over-engineered enterprise patterns.

- Kubernetes cluster nodes
- Proxmox hypervisors
- LXC containers
- monitoring exporters
- Plex host updates
- iSCSI, GPU, and node bootstrap tasks
- Third-party/umbrella Helm charts for homelab platform services. First-party app charts should stay with the app code they deploy, not live centrally in this repo.
- First-party homelab apps that live in this repo belong under `apps/<app>/`, with their Helm chart adjacent to source/deployment config.
- `apps/family-retirement-planner` is a Go/PostgreSQL deterministic retirement and estate-planning workbench. Estate MVP added 2026-08-14 with typed household-scoped inventory, primary/contingent designations, dependent-aware readiness, explicit death timing/obligations, eight-stage scenario, and persisted printable results. Read its `ARCHITECTURE.md` and `AGENTS.md`. Static-only, not deployed; LAN/VPN-only; never Cloudflare Tunnel; no real financial or estate data until encrypted off-cluster backup and tested restore exist; never store credentials in digital metadata.
- Cloudflare Tunnel routes: **the `ocean-sounds` tunnel (`251632cb-5abe-4397-8294-1e8e3f6c5b7e`) is a remotely-managed tunnel (`config_src: "cloudflare"`)**. Its ingress rules live in Cloudflare's control plane via the Tunnel Configuration API (`PUT /accounts/{account}/cfd_tunnel/{tunnel}/configurations`), NOT in `charts/cloudflared/values.yaml`/the local `config.yaml` ConfigMap — that local file is mounted into the pod but is **completely ignored** by a remotely-managed tunnel. Keep `values.yaml` in sync for documentation/rollback reference, but any real ingress change must be pushed via the API too, or it silently does nothing (discovered 2026-07-01 after a values.yaml-only change produced 404s at the edge with zero cloudflared logs).

## Cluster Upgrade Visibility (added 2026-07-03)

Two read-only tools answer "what's outdated on the cluster right now?" without any GitOps/Renovate dependency (repo Chart.yaml pins and live cluster state have drifted, so watching git alone would lie).

- **`charts/version-checker`**: umbrella chart wrapping `jetstack/version-checker` (`https://charts.jetstack.io`), deployed as Helm release `version-checker` in `monitoring`. `versionChecker.testAllContainers: true` (no per-pod annotations needed) scans every running container and exposes `version_checker_is_latest_version` / `version_checker_image_failures_total` metrics on `:8080/metrics`, scraped via its own `ServiceMonitor` (labeled `release: prometheus-stack` — required because `kube-prometheus-stack` sets `serviceMonitorSelectorNilUsesHelmValues: true`, same pattern PrometheusRules use). Ships its own Grafana dashboards (`dashboards.enabled: true`, folder `Homelab`) via the chart's built-in `internal.json`/`general-overview.json`, not a hand-rolled dashboard.
  - Deploy/update: `task monitoring:render-version-checker` then `task monitoring:upgrade-version-checker` (or `helm upgrade version-checker charts/version-checker -n monitoring -f values.yaml`).
  - Alert: `charts/prometheus-stack/rules/version-checker.yaml` — `VersionCheckerImageLookupFailures` fires on `increase(version_checker_image_failures_total[15m]) > 0`, **explicitly excludes `image=~"registry.theramirez.casa/.*"`** because first-party images on that private unauthenticated registry can never resolve an upstream "latest" — that failure is permanent/expected, not a real signal. `VersionCheckerDown` covers scrape loss. Apply via `task monitoring:apply-grafana-dashboards-and-rules`.
- **`nova` (FairwindsOps, `brew install nova-fairwinds`)**: on-demand snapshot, not a running service. `task check:cluster-upgrades` (alias `task nova`) runs `nova find --helm --show-old` (installed vs latest chart per Helm release) and `nova find --containers --show-old` (installed vs latest image tag per running container) as plain tables.
- Renovate/GitOps deliberately skipped for now (2026-07-03 decision) — would need the repo to be the source of truth first (Flux/ArgoCD), and it currently isn't (e.g. cluster `crossplane` release is 2.1.1 while repo `Chart.yaml` pinned 1.19.1 at the time this was written).

## Known Version Debt (surfaced 2026-07-03 via `nova`)

- `prometheus-stack` chart itself (`kube-prometheus-stack`) is pinned to 67.4.0, ~20 versions behind; bumping just needs care since the chart also carries Prometheus/Alertmanager/kube-state-metrics versions, not only Grafana.
- Grafana app version was overridden ahead of the chart bump: `charts/prometheus-stack/values.yaml` sets `grafana.image.repository: grafana/grafana` (no `docker.io/` prefix — the chart's own `registry:` field already supplies that; setting the full path there double-prefixes into `docker.io/docker.io/...` and 401s) and `grafana.image.tag: "13.1.0"`. Two majors ahead of the chart's bundled default (11.4.0); watch for dashboard/plugin issues after this jump.
- Home Assistant bumped 2026.5.0 → 2026.7.1 (`charts/home-assistant`), a plain first-party chart (own `templates/`, no Helm dependency). Applied 2026-07-05 as Helm release `home-assit` revision 20 in namespace `home-assistant`; local chart version is `0.2.2`.
- Mealie installed 2026-07-05 via local chart `charts/mealie` as Helm release `mealie` revision 1 in namespace `mealie`. Image `ghcr.io/mealie-recipes/mealie:v3.20.1`, SQLite on PVC `mealie-data` (`ssd`, 10Gi, mounted `/app/data`), single replica/Recreate. Exposed at `https://mealie.theramirez.casa` through kgateway Gateway `home` HTTPS listener with HTTPRoute `mealie`; route accepted/resolved and `/api/app/about` returned HTTP 200. Mealie has no official upstream Helm chart; keep local chart simple rather than pulling community chart drift. Default login from upstream docs is `changeme@example.com` / `MyPassword`; change password after first login. Signup disabled.
- Removed stale leftovers from `charts/home-assistant`: `Chart.lock` + vendored `charts/home-assistant/` subdirectory referenced an old `truecharts` dependency (`19.0.18`) that the chart no longer declares in `Chart.yaml` — the extracted subchart dir was missing its own `Chart.yaml` and was hard-blocking every `helm upgrade` with "error unpacking subchart". Safe to remove since the parent chart has zero declared dependencies now.
- `flannel` (CNI) upgraded 2026-09-01 from official chart `v0.24.2` to `v0.28.9` (Helm release `flannel` revision 2, namespace `kube-flannel`). Repo source-of-truth is now the wrapper chart `charts/flannel` (parent 0.1.0, dependency `flannel` `v0.28.9` from `https://flannel-io.github.io/flannel`); the obsolete raw `v0.12.0` manifest `charts/flannel/deploy.yaml` was deleted. Workflow: `task flannel:{dependency,lint,render,diff,dry-run,upgrade,rollback,verify}` — see `docs/deployment.md` "Flannel CNI upgrade". Images now `ghcr.io/flannel-io/flannel:v0.28.9` + `ghcr.io/flannel-io/flannel-cni-plugin:v1.9.1-flannel3` (moved off docker.io), with health probes on `:8081` and a `NoExecute` toleration added. Pod CIDR `10.244.0.0/16` and VXLAN backend unchanged. Rollback target: Helm revision 1 (`task flannel:rollback`). The `/run/flannel/subnet.env` missing-file errors seen during node reboots are a kubelet-vs-flanneld startup race (upstream issue flannel-io/flannel#2437), transient and self-healing — not fixed or worsened by this upgrade.
- `home-psql-cluster` uses a `bitnami/postgresql` chart from before Bitnami restructured their free images (2025) — confirm this release is still actually in use (CNPG runs most DBs now) before deciding whether to upgrade or retire it.
- Full current backlog: run `task check:cluster-upgrades`.
- **2026-07-01: townhome tunnel cutover is LIVE.** Both `townhome-test.theramirez.casa` and `townhome.theramirez.casa` are proxied CNAMEs (`<tunnel-id>.cfargotunnel.com`) routing to `http://townhome-frontend.townhome.svc.cluster.local:80` via the tunnel's remote config. **Townhome tunnel boundary**: listing frontend + offer/showing submission only (`POST /api/offers`, `POST /api/showings`, `GET /api/health`) — nginx in the app itself blocks `/admin*` and other `/api/*` paths regardless. Seller offer-review/admin uses separate host `townhome-admin.theramirez.casa` via the app repo's `k8s/admin-httproute.yaml` (now applied to the cluster, previously was not) on the internal kgateway `home` Gateway (LAN IP `192.168.1.83`) — reachable only from the home network/VPN, deliberately NOT in the tunnel config. Do not add `townhome-admin.theramirez.casa` to the tunnel's ingress (API or values.yaml); public listing hosts must not route `/admin*` or `/api/admin/*` at all.
- **2026-07-07: Trilium agent-memory gateway deployed, waiting for Trilium first-run ETAPI token.** Standalone app repo at `/Users/rafaelramirez/homespace/agent-memory-gateway` implements Go HTTP gateway + Postgres/pgvector + Trilium ETAPI projection per `/Users/rafaelramirez/Downloads/Trilium Agent Memory Spec.md`. Namespace `ai-memory`; Helm release `agent-memory` revision 3; hosts `trilium.theramirez.casa` and `agent-memory.theramirez.casa` attach to kgateway `home` HTTPS listener and remain LAN/VPN-only. Do **not** add either host to Cloudflare Tunnel. Chart uses pinned `triliumnext/trilium:v0.102.2` (no non-v tag exists), `enableServiceLinks: false` on Trilium to prevent injected `TRILIUM_PORT=tcp://...` crash, `pgvector/pgvector:0.8.0-pg16`, StorageClass `ssd`, bearer-token gateway auth (startup fails without `GATEWAY_API_TOKEN` unless `ALLOW_NO_AUTH=true`), embedded startup migrations, Postgres-first idempotent writes, required confidence/source refs, unknown-context `_Inbox` routing, all-field secret redaction, `trilium_folders` path cache, `PGDATA=/var/lib/postgresql/data/pgdata`, and nightly backup PVC containing pg_dump + Trilium data tarball + non-secret restore note. Live state verified 2026-07-07: `postgres-0`, `agent-memory-gateway`, and `trilium` pods Running; HTTPRoutes Accepted/Resolved; `https://trilium.theramirez.casa` returns HTTP 302 and Trilium logs say DB not initialized; `https://agent-memory.theramirez.casa/v1/health` returns Postgres `ok`, Trilium `error`, status `degraded` because `agent-memory-gateway-env` still has placeholder `TRILIUM_TOKEN`. Gateway image `registry.theramirez.casa/agent-memory-gateway:0.1.0` built/pushed linux/amd64 after fixing Dockerfile builder to Go `1.23.12`; k8s secrets `agent-memory-postgres` and `agent-memory-gateway-env` exist with generated values. Manual next step: open Trilium UI, initialize DB/password, create ETAPI token, patch `agent-memory-gateway-env` key `TRILIUM_TOKEN`, `kubectl -n ai-memory rollout restart deploy/agent-memory-gateway`, then verify `/v1/health`, sample writes, search/context bundle, and backup job.
- **external-dns ownership conflict pattern**: `external-dns` (source `gateway-httproute`, domain-filter `theramirez.casa`/`homeapps.io`/`dipradar.io`, policy `upsert-only`) auto-manages DNS for any HTTPRoute hostname it can see, and will silently revert manual DNS changes to that hostname on its ~60s reconcile loop as long as the HTTPRoute exists. To let a Cloudflare Tunnel CNAME "win" for a hostname that also has a live HTTPRoute (kept as an internal/rollback path), annotate that HTTPRoute with `external-dns.alpha.kubernetes.io/controller: <anything-other-than-dns-controller>` (e.g. `ignore`) — official external-dns behavior, causes the source to skip that resource entirely. Used on the `townhome` HTTPRoute in namespace `townhome` 2026-07-01 to free `townhome.theramirez.casa` for the tunnel while keeping the HTTPRoute/kgateway path intact as rollback. `townhome-admin` HTTPRoute was deliberately left unannotated (still external-dns managed, LAN-only, correct).
- **2026-07-01 incident: Cloudflare API token had been dead for ~17 days**, breaking `external-dns` (`CrashLoopBackOff`, `403 Invalid access token`, code 9109, all DNS automation stalled) and putting the `theramirez-casa-wildcard-tls` cert renewal (due ~2026-08-02) at risk via the same broken `cloudflare-api-token-secret` used by cert-manager's `letsencrypt-cloudflare` ClusterIssuer. Root cause: token expired/revoked at Cloudflare's end; local `~/.cloudflare.env` had also gone stale. Fixed by rotating to a fresh token and updating all three consumers: `cert-manager/cloudflare-api-token-secret` (key `api-token`), `external-dns/cloudflare-credentials` (key `api-token`), and `~/.cloudflare.env`. **Gotcha**: `https://api.cloudflare.com/client/v4/user/tokens/verify` rejected the new, working, narrowly-scoped token with `401 Invalid API Token` even though it worked fine for real calls (`/zones`, DNS edits) — that endpoint is unreliable for scoped tokens; verify with an actual scoped call (e.g. `GET /zones?name=<zone>`) instead. Token scope needed: `Zone:DNS:Edit` for the three zones (cert-manager + external-dns) plus `Account:Cloudflare Tunnel:Edit` (for pushing tunnel ingress config via API). Restart both `external-dns` and `cert-manager` deployments after rotating the secret to force them to pick it up and clear any crash loop.

Default inventory: `hosts`. Default SSH user from `ansible.cfg`: `rramirez`, except groups with explicit `ansible_user=root`.

Task harness: `Taskfile.yaml` is the preferred entrypoint for routine operations. Prefer descriptive task names: `task check:kubernetes-health`, `task check:proxmox-guests`, `task check:nas-and-sync-storage`, `task install:all-node-exporters`, `task install:node-exporters-proxmox-lxcs`, `task update:active-kubernetes-node-os`, `task update:ssh-managed-server-os-no-reboot`, `task update:proxmox-packages-no-reboot`, `task update:kubernetes-version`, `task update:plex-media-server`, `task update:syncthing-lxc`, `task monitoring:render-prometheus-stack`, `task monitoring:upgrade-prometheus-stack`, `task monitoring:apply-grafana-dashboards-and-rules`, `task monitoring:show-syncthing-storage`. Short aliases like `task k8s:status`, `task exporters:all`, and `task syncthing:update` still work.

Proxmox major upgrades: use `docs/runbooks/proxmox-major-upgrade.md` and only `task proxmox:major:{preflight,upgrade,resume,reboot,verify}` with an exact `PVE_HOST` of `pve-1` or `pve-2`. `resume` is only for an interrupted canonical-trixie target upgrade after manual dpkg recovery; it never restores bookworm sources. The workflow supports PVE 8/bookworm → PVE 9/trixie only, is one-host/phase-at-a-time, and requires explicit backup/outage/reboot acknowledgements. It never auto-repairs boot/GRUB/CT100 cgroup/NVIDIA DKMS/sysctl blockers, manages guests, upgrades ZFS features, or claims an in-place downgrade path.

Access context:

- Rafael's SSH key should be present on most or all home infra hosts; use SSH/Ansible for read-only discovery when useful.
- Prefer `ssh -o BatchMode=yes -o ConnectTimeout=8 ...` for quick reachability checks; do not assume key access means permission to make disruptive changes.
- For Proxmox guests, start with read-only commands: `pveversion`, `qm list`, `pct list`, `pvesh get /cluster/resources`, `qm config <vmid>`, `pct config <vmid>`.
- Ask before changing VM/LXC configs, starting/stopping/rebooting guests, migrating guests, editing storage, or changing Proxmox networking/firewall.

Owner context:

- Owner: Rafael Ramirez
- Timezone: America/New_York
- Preferred style: direct, practical, action-oriented
- Technical background: experienced platform/SRE/cloud leader; strong with Kubernetes, AWS, Linux, Terraform, Helm, Go, observability, and operational patterns
- Preferred tooling: simple shell scripts, Taskfiles, Go utilities, Kubernetes-native workflows, easy-to-reason-about YAML
- Avoid vague architecture talk and enterprise-heavy patterns that add maintenance without clear benefit

Default recommendation biases:

- keep Proxmox as the base layer unless Rafael says otherwise
- keep NAS/storage simple and recoverable
- do not put critical storage fully inside Kubernetes without a strong reason
- use Kubernetes for apps, dashboards, automation, and experiments
- use Authentik for centralized auth where it fits
- use Prometheus/Grafana/Alertmanager for monitoring
- use ZFS snapshots and tested backups for important data
- prefer LAN/VPN/private access for admin surfaces
- prefer boring infrastructure over clever infrastructure

## Context Loading

Use repo topic docs to avoid loading all context by default:

- Read `docs/memory/current-work.md` first for active work.
- Read `docs/architecture.md` for architecture/topology changes.
- Read `docs/deployment.md` for Kubernetes, Ansible, Helm, Docker, or release work.
- Read `docs/database.md` only for database/schema/query work.
- Read `docs/memory/decisions.md` before changing established patterns.
- Read `docs/memory/gotchas.md` before debugging.
- Update `docs/memory/current-work.md` after meaningful context changes; use `scripts/update-agent-memory.sh` to ensure memory files exist.

## Topology

### Kubernetes

Group: `home_base`

| Host | IP |
|---|---|
| k8s-master | 192.168.1.135 |
| k8s-node-1 | 192.168.1.136 |
| k8s-node-2 | 192.168.1.137 |
| k8s-node-3 | 192.168.1.138 |
| k8s-node-4 | 192.168.1.139 |

Groups:
- `k8s_master`: `k8s-master`
- `k8s_nodes`: `k8s-node-1` through `k8s-node-4`
- `retired_k8s_nodes`: `k8s-node-5` / `192.168.1.140`, `k8s-node-6` / `192.168.1.141`
- `gpu_nodes`: `k8s-master`, `k8s-node-1`

Observed Kubernetes/Proxmox placement on 2026-06-05:

| K8s node | IP | Proxmox host | VMID | vCPU | RAM |
|---|---|---|---:|---:|---:|
| `home-base1` / inventory `k8s-master` | 192.168.1.135 | `pve-2` / `infra` | 100 | 4 | 16 GB |
| `home-base2` / inventory `k8s-node-1` | 192.168.1.136 | `pve-2` / `infra` | 101 | 4 | 16 GB |
| `home-base3` / inventory `k8s-node-2` | 192.168.1.137 | `pve-2` / `infra` | 102 | 4 | 16 GB |
| `home-base4` / inventory `k8s-node-3` | 192.168.1.138 | `pve-2` / `infra` | 103 | 4 | 16 GB |
| `home-base5` / inventory `k8s-node-4` | 192.168.1.139 | `pve-1` / `files` | 101 | 4 | 8 GB |
| retired `home-base6` / inventory `k8s-node-5` | 192.168.1.140 | `pve-1` / `files` | 102 | 4 | 2 GB |
| retired `home-base7` / inventory `k8s-node-6` | 192.168.1.141 | `pve-1` / `files` | 103 | 4 | 2 GB |

Kubernetes shape observed 2026-06-05:

- Single control-plane node: `home-base1`; all nodes are schedulable and have no taints.
- OpenEBS local PV is active; default StorageClass is `ssd` (`openebs.io/local`, hostpath `/ssd`, `WaitForFirstConsumer`). `nvme` uses hostpath `/nvme`.
- Most bound local PVs are pinned to `home-base2`, `home-base3`, or `home-base4`.
- 2026-06-05 consolidation: `home-base6` and `home-base7` were drained, removed from Kubernetes, shut down, and set `onboot=0`; no PVs were pinned to them at preflight.
- 2026-06-05 consolidation: `home-base5` was drained, resized from 2 GB to 8 GB RAM, restarted, and uncordoned; it remains the one worker on `pve-1`.
- Current optimized split: `pve-2` is primary Kubernetes compute/control-plane with four 16 GB VMs; `pve-1` is primary storage/NAS plus one 8 GB Kubernetes worker.
- Node labels applied 2026-06-05: `topology.theramirez.casa/proxmox=pve-2` on `home-base1..4`, `topology.theramirez.casa/proxmox=pve-1` on `home-base5`, `node-role.theramirez.casa/cluster-primary=true` on `home-base1`, `node-role.theramirez.casa/storage-host-adjacent=true` on `home-base5`.
- Rafael's stated goal for this home Kubernetes cluster: not true HA; wants an optimized, low-maintenance place to run apps/stuff. Prefer fewer, better-sized nodes and simple backup/restore over HA complexity.
- Reactivating retired `home-base6/7` is not just starting VMs: set Proxmox `onboot=1`, start VM(s), rejoin Kubernetes with current kubeadm flow if needed, move inventory hosts from `retired_k8s_nodes` back to `home_base`/`k8s_nodes`, restore Prometheus scrape targets, then verify nodes/PVs/DaemonSets.

### Proxmox

Group: `proxmox`, `ansible_user=root`

| Host | IP |
|---|---|
| pve-1 | 192.168.1.216 |
| pve-2 | 192.168.1.28 |

There are two Proxmox deployments/hypervisors. Rafael deploys VMs and LXCs from these hosts.

- `pve-1` / `192.168.1.216` reports hostname `files`; also primary NAS.
- `pve-2` / `192.168.1.28` reports hostname `infra`.
- Both observed on Proxmox VE `8.4.14` on 2026-06-05.
- SSH as `root` should work by key; inventory already sets `ansible_user=root` for `proxmox`.

### pve-2 freeze remediation, 2026-08-12

- `pve-2` / `infra` (`192.168.1.28`) had abrupt freezes. Updated Proxmox `pve-manager` `8.4.14` -> `8.4.20` and kernel `6.8.12-16-pve` -> `6.8.12-41-pve`; retained old kernel in GRUB. Added Debian `non-free-firmware`; installed `amd64-microcode` `3.20250311.1~deb12u1`, updating early microcode `0x08608103` -> `0x08608108`.
- Removed GRUB `tsc=unstable` and invalid `amd_iommu=on`; IOMMU initializes automatically. Kernel independently marks TSC unstable from 3 ms skew and broken BIOS, then switches to HPET. Do not re-add `tsc=unstable`.
- Host rollback archive: `/root/infra-remediation-20260812-150231.tar.gz`, SHA256 `a8bb40a3757759d54e5c6377d22e02b7f6994451406037e767927472039736bb`. Validated etcd snapshot: revision `460460254`, 295 MB, hash status `f09ddf4a`; SHA256 `67aaf0235f8950239829f09c8fde9402c8e280b02acdf8eb54eed36303404653`. Sensitive off-host copies live under `~/.config/opencode/private-backups/homebase-infra/pve-2-20260812/` with directory mode `0700` and file mode `0600`; treat both as credentials-bearing backups.
- VM `102` was stale/unreachable since 02:39 and force-stopped; VMs `100`/`101`/`103` shut down cleanly. After controlled reboot, all four VMs and all five Kubernetes nodes were Ready; API/etcd ready; pending pods recovered; only pre-existing `qualtrim-sync` failures remained. Both SSD SMART checks passed; storage unchanged, `SSD` VG remains 98.55% provisioned.
- If freezes recur: review backups and BIOS release notes, then update BIOS `SER5H508` dated `2023-12-12`; next diagnostics are power-brick substitution, Memtest86+, and thermal tests. BIOS was not updated during this remediation.
- Post-remediation review caught two gaps, both fixed same day: off-host rollback copies were initially world-readable in a temp path (moved to `~/.config/opencode/private-backups/homebase-infra/pve-2-20260812/` at `0700`/`0600`, see above); and pve-2 had no host/PVE-exporter-down alert despite the NAS having one. Added `InfraHostDown` (`up{job="node-external",instance="192.168.1.28"}`, critical, 5m) and `InfraPveExporterDown` (`up{job="pve",instance="192.168.1.28"}`, warning, 10m) to `charts/prometheus-stack/rules/homelab-storage.yaml` group `homelab-storage.infra`, applied live via `task monitoring:apply-grafana-dashboards-and-rules`.
- **2026-09-01: pve-2 upgraded PVE8/bookworm -> PVE9/trixie**, matching pve-1 (both hosts now on `pve-manager/9.2.11`, kernel `7.0.14-14-pve`). Ran via this repo's own `task proxmox:major:{preflight,upgrade,reboot,verify}` workflow; all phases clean, `failed=0`. Two real bugs found and fixed in `tasks/proxmox-major-technical-preflight.yaml` while running preflight against pve-2 for the first time (pve-1 never exercised these pve-2-only checks): (1) the ZFS health assertion only checked `zpool_status.stdout`, but on a pool-less host `zpool status -x` prints `no pools available` to **stderr** (rc=0) — false-failed every time; fixed to check stdout+stderr combined. (2) the sysctl.conf non-comment-line check used YAML single-quoted `'^\\s*(#|$)'` passed via `argv:` to `grep -E` — YAML single-quotes don't escape backslashes, so grep received a literal double-backslash regex (matches "starts with a literal `\`"), which never matched real content and made `-v` return almost the whole file every time regardless of actual state; fixed to single-backslash `'^\s*(#|$)'`. Also found (real host-state issue, not a script bug) pve-2's `/etc/apt/sources.list` had two non-canonical PVE-repo lines: the `pve-no-subscription` line had a stray trailing ` non-free-firmware` (that component doesn't apply to Proxmox's own repo, confirmed by comparing to pve-1's known-good state) and the `security.debian.org` line was missing its `/debian-security` path (old-format bare-domain URL) — both fixed to match pve-1's canonical format before preflight would pass. Remediated the three documented boot/sysctl blockers per the runbook's own official-fix pointers: `apt remove systemd-boot` (stray package, not pve-2's actual bootloader — GRUB/shim is), `debconf-set-selections grub2/force_efi_extra_removable=true` + `apt install --reinstall grub-efi-amd64` (keeps the removable EFI fallback path in sync), and migrated `/etc/sysctl.conf` (`net.ipv4.ip_forward=1`, `vm.swapiness=0`) to `/etc/sysctl.d/99-pve2-legacy.conf`.
- **2026-09-01: pve-2 has a recurring lockup pattern ("just locks up and have to reboot it"), only partially resolved by the Aug-12 remediation.** `journalctl --list-boots` shows anomalously short boots interspersed with normal multi-day ones since Aug-12 (e.g. Aug24 12:51->17:57 ~5h, Aug26 08:56->11:25 ~2.5h) — the freeze issue never fully went away, it just got less frequent. Confirmed the Sep-1 PVE9 reboot itself was NOT a crash (journal shows a clean graceful shutdown sequence for the boot immediately before it), so today's kernel jump 6.8.12-41-pve->7.0.14-14-pve is a new variable, not the cause. Hardware: AMD Ryzen 7 5700U (mobile APU), board is Beelink SER5 (`dmidecode` manufacturer `AZW`, product `SER`, version `V2.0`), BIOS `SER5H508` dated 2023-12-12, never updated. **No IPMI/BMC** (`ipmi_si: Unable to find any System Interface(s)`) — a true hard lockup has no hardware watchdog reset path, only a manual power-cycle; `watchdog-mux.service` only runs the *software* watchdog, which cannot recover a genuine hang.
  - Oracle consulted 2026-09-01 for a prioritized diagnostic/mitigation plan. Root-cause ranking: BIOS/AGESA power-state firmware defect (high), RAM instability (medium-high), power-brick/VRM (medium), thermal (medium-low, untested), kernel 7.0 itself (low-medium, pattern predates it). **Recommendation: do NOT flash BIOS yet** — no IPMI means a bad flash is a full outage with no recovery path short of physical access; get the exact-model firmware from Beelink for board `SER V2.0` (generic SER5 firmware risks bricking) and only flash after cheaper diagnostics are exhausted or another freeze happens with the new instrumentation in place.
  - **EDAC/RAM error telemetry is a dead end on this hardware**: `modprobe amd64_edac` fails with `No such device` — Ryzen mobile/laptop-class APUs (including the 5700U) generally don't expose ECC memory-controller telemetry the way desktop/server AMD platforms do (this box runs non-ECC SODIMM). `rasdaemon`/`ras-mc-ctl` will never show RAM errors here regardless of configuration; MemTest86+ (>=4 passes, VMs stopped, overnight window) is the only real way to validate RAM health on this platform.
  - **Instrumentation installed and active** (2026-09-01, low-risk, no reboot): `rasdaemon`, `lm-sensors`, `smartmontools` (smartmontools was already present via `pve2`). `sensors` baseline: `k10temp` Tctl ~76°C, NVMe composite ~52-59°C, amdgpu edge ~57°C — worth re-checking against this baseline if another freeze happens (thermal was previously untested). `/etc/sysctl.d/98-lockup-diagnostics.conf` sets `kernel.nmi_watchdog=1` (was already 1), `kernel.hardlockup_panic=1`, `kernel.softlockup_panic=1`, `kernel.panic=30` — a detected soft/hard lockup now panics and auto-reboots after 30s instead of hanging forever silently, since there's no IPMI to force a reset otherwise.
  - **netconsole attempted and abandoned as not worth the effort** (2026-09-01): tried sending kernel `printk` output over UDP from pve-2 to a `socat` listener on pve-1 (`192.168.1.216:6666`) to capture forensic evidence through a future freeze. Tried three configs — static `netconsole=` module param over `vmbr0` (bridge device; local ethernet address resolved as bogus `ff:ff:ff:ff:ff:ff`), the same over the underlying physical `enp1s0` (netpoll refuses: "is a slave device, aborting"), and configfs dynamic reconfig (`/sys/kernel/config/netconsole/`) with the local MAC explicitly and correctly set to the real `b0:41:6f:0f:6f:a6` — all three reported `transmit_errors: 0` at the sender but delivered zero bytes to the pve-1 listener; ruled out `pve-firewall` (disabled) and iptables/nft rules (none) as the cause. This is a known-hard combination (netpoll + software Linux bridge); not pursued further as disproportionate effort for a nice-to-have. Both the pve-2 module/configfs config and the pve-1 `socat` listener service were cleanly removed — nothing half-configured left behind. If freeze-time log capture is wanted later, a serial/USB debug console or `pstore`/`ramoops` (reserved-memory crash log surviving a reboot, no network dependency) are the better next options per Oracle, not netconsole through this bridge.
  - Treat this as an open reliability risk on the host carrying the entire K8s cluster (single point of failure, no HA) until root-caused. Mitigation options discussed but not yet actioned: manual smart-plug/PDU for remote power-cycling (better fit than hardware watchdog add-on for this mini-PC); moving only the K8s control-plane VM to `pve-1`/`home-base5` would improve manageability during an outage but does not make the cluster HA.

Observed Proxmox guest layout on 2026-06-05, verify before acting because guests can move/change:

| Proxmox host | Type | VMID | Name | Status | Memory | Boot disk |
|---|---|---:|---|---|---:|---:|
| `pve-1` / `files` | VM | 101 | `home-base5` | running | 8192 MB | 100 GB |
| `pve-1` / `files` | VM | 102 | `home-base6` | stopped, `onboot=0` | 2048 MB | 100 GB |
| `pve-1` / `files` | VM | 103 | `home-base7` | stopped, `onboot=0` | 2048 MB | 100 GB |
| `pve-1` / `files` | LXC | 100 | `store.theramirez.casa` | running | unknown | unknown |
| `pve-1` / `files` | LXC | 104 | `downloads` | running | unknown | unknown |
| `pve-1` / `files` | LXC | 105 | `sync.theramirez.casa` | running | unknown | unknown |
| `pve-2` / `infra` | VM | 100 | `home-base1` | running | 16384 MB | 100 GB |
| `pve-2` / `infra` | VM | 101 | `home-base2` | running | 16384 MB | 100 GB |
| `pve-2` / `infra` | VM | 102 | `home-base3` | running | 16384 MB | 100 GB |
| `pve-2` / `infra` | VM | 103 | `home-base4` | running | 16384 MB | 100 GB |

Notes:

- `home-base*` VMs appear to back the Kubernetes fleet, but map VM name ↔ Kubernetes hostname/IP before making node-level changes.
- `pve-1` currently hosts the known LXCs from Proxmox's perspective; Ansible inventory currently tracks only `lxc-3` and `lxc-4`, so reconcile inventory before automating every LXC.

### LXC

Group: `lxc`, `ansible_user=root`

| Host | IP |
|---|---|
| lxc-3 | 192.168.1.218 |
| lxc-4 | 192.168.1.225 |

`plex` group targets `lxc-3`.

LXC placement decision confirmed by Rafael on 2026-06-05:

- Keep Plex/Homeflix on `store.theramirez.casa` / LXC `100` because video files live on the NAS paths there; avoid moving Plex into Kubernetes unless there is a strong reason and GPU/storage permissions are revalidated.
- Keep `downloads` LXC because it has VPN setup inside the container; Kubernetes migration would add tun/device/VPN routing complexity for little benefit.
- Keep `sync.theramirez.casa` / Syncthing-style sync in LXC because it syncs to a local directory on the NAS mount; stable filesystem access matters more than Kubernetes lifecycle.
- General rule: keep NAS/media/VPN/sync appliance-style services in LXCs; use Kubernetes for web apps, automation, dashboards, experiments, and services that do not need special host paths/devices/VPN plumbing.

## Monitoring

Central stack runs in Kubernetes `monitoring` namespace using in-repo chart:

- chart: `/Users/rafaelramirez/homespace/homebase-infra/charts/prometheus-stack`
- values: `charts/prometheus-stack/values.yaml`
- dashboards: `charts/prometheus-stack/dashboards`
- rules: `charts/prometheus-stack/rules`

Public endpoints:

- Grafana: `https://grafana.theramirez.casa`
- Prometheus targets: `https://prometheus.theramirez.casa/targets`

Expected Prometheus external targets:

- `node-external`: 10 targets, grouped by `group=k8s|lxc|proxmox`
- `pve`: 2 targets
- `zfs-exporter`: 1 target if ZFS exporter is running on `pve-1`
- 2026-06-05 consolidation follow-up: retired K8s node-exporter targets `192.168.1.140:9100` and `192.168.1.141:9100` were removed from `charts/prometheus-stack/values.yaml`; Helm release `prometheus-stack` in `monitoring` upgraded to revision 17 and live Prometheus config showed only K8s targets `192.168.1.135:9100` through `192.168.1.139:9100`.
- 2026-06-05 sync storage monitoring: added `192.168.1.78:9100` to `node-external`, added `SyncStorageAlmostFull` and `SyncStorageCritical` alerts, added low-noise `Homelab / Essentials` Grafana dashboard, disabled noisy `Node Exporter Full` auto-import label, and upgraded `prometheus-stack` to revision 18.

Exporter ports:

| Target | Port | Purpose |
|---|---:|---|
| node-exporter | 9100 | host OS metrics |
| pve-exporter | 9221 | Proxmox VM/LXC/ZFS/cluster metrics |
| zfs-exporter | 9134 | ZFS internals on `pve-1` |

## Storage / NAS

Rafael considers the home NAS to be `store.theramirez.casa`, implemented as LXC `100` on `pve-1` / `files`. The storage pool itself lives on the Proxmox host; NAS user-facing services run mostly in the LXC.

- Proxmox host/storage layer: `pve-1` / `files` / `192.168.1.216`
- NAS access layer: LXC `100` / `store.theramirez.casa` / `192.168.1.218`
- Direct SSH to LXC works as `root@192.168.1.218` as of 2026-06-05; `store.theramirez.casa` DNS resolves to Gateway VIP `192.168.1.83`, not the LXC IP.
- ZFS pool: `home-storage`, observed `ONLINE` on 2026-06-05
- Pool layout observed 2026-06-05: 4-disk `raidz2-0`, 29.1T raw, 6.11T allocated, 23.0T free, 11% fragmentation, 21% capacity
- Pool disks observed 2026-06-05: 4x Seagate `ST8000VN004-2M2101` ~8TB (`sda` serial `WSD43WC2`, `sdb` `WSD4EXV7`, `sdc` `WSD4EXG1`, `sdd` `WSD4HYT3`); pool uses partition 2 on each disk.
- Last scrub observed 2026-06-05: completed 2026-05-10, repaired `0B`, `0` errors
- Source of truth for ZFS health: `zfs_exporter` on `pve-1:9134`; service active on 2026-06-05
- No ZFS snapshots observed for `home-storage/home-cloud`, `home-storage/k8s-storage`, or `home-storage/vms` on 2026-06-05
- No Proxmox backup jobs observed via `pvesh get /cluster/backup` on 2026-06-05
- Backup path user-reported 2026-06-05: Rafael mounts the NAS share on a Windows machine, and that Windows machine backs share contents up to Backblaze. Treat files placed in the share as covered by Backblaze, but verify client status/last backup before relying on it for restore-sensitive work.

Observed NAS mounts/services on 2026-06-05:

- LXC `100` is unprivileged, `onboot=1`, Debian, 4 cores, 4096 MB RAM, 50 GB rootfs, 512 MB swap
- LXC mount points: `mp0=/home-storage/home-cloud/home-share -> /home-cloud/home-share`, `mp1=/home-storage/home-cloud/rramirez -> /home-cloud/rramirez`
- Samba runs inside LXC `100`; share `[home-cloud]` points at `/home-cloud`, `valid users = rramirez erikami89 test-user`, `force group = home-share`, masks `0770`, `fruit` enabled for macOS clients
- Samba perms observed: `/home-cloud` `rramirez:rramirez 755`, `/home-cloud/home-share` `root:home-share 775`, `/home-cloud/rramirez` `rramirez:rramirez 700`
- NFS server runs on `pve-1`, not in LXC `100`; exports `/home-storage/home-cloud/home-share/Videos` rw and `/home-storage/home-cloud/home-share/Photos` ro to `192.168.1.0/24`, both currently with `no_root_squash`
- Plex/Homeflix appears to run in LXC `100` on `:32400`; LXC config includes NVIDIA device bind mounts
- Node exporter active in LXC `100` on `:9100`; logs show repeated nfsd collector parse errors for `wdeleg_getattr` as of 2026-06-05
- Sync LXC `105` / `192.168.1.78` rootfs is ZFS dataset `home-storage/home-cloud/home-share/subvol-105-disk-0` with `refquota=50G`; observed 2026-06-05 at 48G used / 2.7G free / 95% full, and 2026-06-06 at 40G used / 11G free / 80% full. Node exporter installed inside LXC via `pct exec` and scraped at `192.168.1.78:9100`; manage with `node-exporters-proxmox-lxc.yaml` or `task install:node-exporters-proxmox-lxcs`. Syncthing runs in this LXC as `syncthing@syncthing.service`; updated 2026-06-06 from `1.27.4` to `1.30.0` via apt repo `https://apt.syncthing.net`, after backing up config/database to `/var/backups/syncthing/config-20260606-192227.tgz`. Future updates: use `update-syncthing-lxc.yaml` or `task update:syncthing-lxc`; direct SSH `root@192.168.1.78` also works.

Known storage paths/domains:

- `/home-storage/home-cloud`
- `/home-storage/home-cloud/home-share` — shared between Rafael and Erika
- `/home-storage/home-cloud/rramirez` — Rafael private directory
- Samba target: `\\mnt.homeapps.io\home-cloud` if DNS/client config resolves it
- `store.theramirez.casa` resolves locally to Gateway VIP `192.168.1.83`; repo ingress maps it to Proxmox `pve-1:8006`, not directly to LXC `100`
- `homeflix.theramirez.casa` resolves locally to Gateway VIP `192.168.1.83`; repo ingress maps it to LXC `100:32400`

Storage preferences:

- favor ZFS where appropriate
- explain RAIDZ1 vs RAIDZ2 in terms of usable capacity, resiliency, rebuild risk, and operational complexity
- do not assume TrueNAS is still preferred; check whether target is Proxmox, LXC, VM, or Kubernetes
- avoid designs that make file permissions painful unless there is a strong reason
- Rafael has mounted host directories into LXC containers and used separate mount points for `home-share` and `rramirez`

`zfs_pool_health` mapping:

- `0=ONLINE`
- `1=DEGRADED`
- `2=FAULTED`
- `3=OFFLINE`
- `4=REMOVED`
- `5=UNAVAIL`
- `6=SUSPENDED`

## Common Playbooks

| Playbook | Target | Purpose |
|---|---|---|
| `update-homebase.yaml` | `home_base` | dist-upgrade K8s nodes, reboot if needed |
| `update-k8s.yaml` | `home_base` | interactive Kubernetes version upgrade, master first then workers |
| `add-k8s-node.yaml` | `bootstrap` | bootstrap new node with containerd/K8s packages/swapoff |
| `disable-swap.yaml` | `bootstrap` | persistent swapoff systemd service |
| `setup-iscsi.yaml` | `home_base` | iSCSI initiator setup |
| `setup-gpu.yaml` | `gpu_nodes` | AMD GPU drivers + VAAPI packages |
| `node-exporters.yaml` | `servers` | install node-exporter on home_base + lxc + proxmox |
| `install-pve-exporter.yaml` | `proxmox` | create PVE monitoring API user/token and systemd exporter on `:9221` |
| `update-syncthing-lxc.yaml` | `proxmox_managed_lxc` | back up and update Syncthing in LXC `105` via `pct exec` |
| `update-plex.yaml` | `plex` | update Plex `.deb` on `lxc-3` |

## Media

Rafael hosts or plans media services such as Plex and/or Jellyfin.

Known context:

- Plex should use hardware acceleration when practical to reduce CPU
- AMD integrated graphics → VAAPI
- NVIDIA card discussed: GeForce GTX 1080 / GP104
- GPU passthrough may involve LXC, VM, Docker, Kubernetes, TrueNAS, or Proxmox; identify runtime before final commands
- Jellyfin should join the home-share group when it needs shared media access

Media guidance:

- prefer simplest working media architecture first
- verify device visibility (`/dev/dri` or NVIDIA devices) and confirm transcoding in media dashboard
- account for container permissions, driver version, LXC/VM passthrough boundaries

## Media Automation Stack (added 2026-08-02)

Full pipeline layout:

- k8s namespace `media-automation`: Helm releases `radarr`, `sonarr`, `prowlarr`, `seerr`, `decluttarr`, `recyclarr` — local charts at `charts/<app>` (bjw-s `app-template` 4.6.2 vendored in each `charts/<app>/charts/`). Update flow: bump image `tag` in `charts/<app>/values.yaml`, then `helm upgrade <app> charts/<app> -n media-automation`.
- Download client: **not in k8s**. `qbittorrent` + `gluetun` (PIA OpenVPN, TCP 443) run via docker-compose in `downloads` LXC 104 (`192.168.1.225`, pve-1) at `/root/docker/docker-compose.yml` (compose v1 binary: `docker-compose`, no plugin). qbit shares gluetun netns; **WebUI is on `:8081` only** (`WEBUI_PORT=8081`; nothing listens on 8080). *arr connect at `http://192.168.1.225:8081`. Compose file contains live PIA credentials — do not commit/print.
- Plex: LXC 100 (`192.168.1.218`, lxc-3, server name `homeflix`), updated via `task update:plex-media-server` (runs `update-plex.yaml`). Public UI: `https://homeflix.theramirez.casa`. Plex owner account: `rafz` (id 365030); server-owner `PlexOnlineToken` lives in LXC 100 `Preferences.xml` — usable for seerr `auth/plex` admin sign-in.
- Media paths: *arr containers mount NFS `192.168.1.216:/home-storage/home-cloud/home-share/Videos` at `/mnt/Videos`; root folders are `/mnt/Videos/Movies` and `/mnt/Videos/TV Shows`. qbit bind-mounts the same path in LXC 104. *arr run `PUID=100000/PGID=101001` (match NAS ownership); qbit `PUID=0/PGID=1001`.
- Seerr: UI/API at `https://request.theramirez.casa`. Admin = Plex owner (sign in via `POST /api/v1/auth/plex` with the PlexOnlineToken). Arr servers configured via `POST /api/v1/settings/radarr|sonarr` (test endpoints return profiles/rootFolders; default quality profile id 6 `HD - 720p/1080p`). Radarr/Sonarr API keys are in each pod's `/config/config.xml`.

2026-08-02 incident (config wipe + recovery):

- Symptom: downloads stalled, seerr looked wiped. Root cause: seerr's radarr/sonarr server entries and `main.mediaServerType` were reset (`mediaServerType: 4` = NOT_CONFIGURED → availability sync logged `An admin is not configured.`). Plex connection, admin user, and request history in `db.sqlite3` were all intact.
- Fixed via API only: Plex-token admin sign-in → recreate radarr/sonarr server entries → `POST /api/v1/settings/main {"mediaServerType": 1}` (1=PLEX) → re-run `radarr-scan`/`sonarr-scan`/`availability-sync` jobs. Verify with logs: no `An admin is not configured` on sync.
- Stuck approved requests (status 2): `POST /api/v1/request/{id}/retry` is a no-op for already-approved requests. Do NOT bulk-recreate them — sonarr/radarr already track the same content as monitored; missing-item search (decluttarr `search_missing` + sonarr RSS) works the backlog.
- `decluttarr` v2 exits 30s after startup if any arr instance is unreachable (by design; k8s restarts it). Causes crashloop-looking restart counts during arr rollouts/node reboots (170 over 44d). Not a bug; restart count freezes once arrs are up.
- `TorrentGalaxyClone` indexer (prowlarr id 5) is flaky; prowlarr auto-disables with ~1d backoff and self-recovers. Only LimeTorrents + TPB + TG configured; consider adding more indexers if miss rate stays high.

2026-08-02 qBittorrent auth + The Challenge recovery:

- qBittorrent restarted without a persisted WebUI password (`WebUI\\Password_*` absent), generated a temporary password, and then banned Sonarr's pod IP after repeated stale-password attempts. Repair: set a persistent WebUI password in qBittorrent, update both Sonarr and Radarr download-client records, restart qBittorrent, and require both Arr client tests to return HTTP 200. Password is intentionally not stored here. Pre-repair config backup: `/docker/qbittorrent/config/qBittorrent/qBittorrent.conf.before-auth-repair-20260802-170943`.
- Prowlarr Full Sync owns Sonarr's `minimumSeeders`; Sonarr-side edits are overwritten. TPB (Prowlarr indexer id 3) now has `torrentBaseSettings.appMinimumSeeders=1`, which syncs to Sonarr. LimeTorrents and TorrentGalaxyClone remain at 5.
- Imported public Google Drive copies for The Challenge directly through downloads LXC 104 into NAS paths. Folder `1ToK_r64GSZYrWkJLbVzHYBMxuwsHGueM` supplied Season 33 E01-E16; folder `1d_apUQJ8aZVriEdPGTkOuyzL7hmFhW6t` supplied Season 34 E01-E16 plus Reunion Parts 1/2 (mapped E17/E18). Final paths: `/home-cloud/home-share/Videos/TV Shows/The Challenge/Season 33|34`; filenames normalized to `The.Challenge.SxxEyy.HDTV.mp4`; owner/group `0:1001` inside LXC (host mapping matches Arr), files `0664`, dirs `0775`.
- Sonarr rescan result: Season 33 `16/18` (only E17/E18 missing), Season 34 `18/18`; nine redundant forced TPB queue entries were removed from Sonarr/qBittorrent after Drive import. All 34 files passed Sonarr-bundled `ffprobe`, are H.264 1280x720, and have plausible nonzero durations; Sonarr maps S34E17/E18 to `Reunion (Part 1)`/`Reunion (Part 2)` correctly. Downloads LXC now has Debian `python3-venv`; gdown ran from a temporary `/tmp/gdown-venv` that was removed after import.

2026-08-17 incident (stalled downloads, gluetun unhealthy):

- Symptom: downloads stalled; qBittorrent stayed up but nothing transferred. Root cause: gluetun became unhealthy after repeated PIA OpenVPN TCP 443 failures (`bad encapsulated packet length` / connection resets) plus DNS timeouts.
- Recovery: `cd /root/docker && docker-compose restart gluetun qbittorrent`; gluetun came back healthy on a fresh endpoint. All 20 unique Sonarr queue downloads returned to `trackedDownloadState=downloading` and moved ~2.36 GiB within the next minute. Four torrents had no ETA and may lack peers.
- Separate unresolved issue: `decluttarr` still crash-loops because its qBittorrent credentials are stale (401). Not fixed by this recovery.

Versions after 2026-08-02 update: radarr 6.3.0, sonarr 4.0.19, prowlarr 2.5.2, seerr v3.4.1 (now pinned, was floating `v3`), decluttarr v2.1.0, recyclarr 8.7.0, qbittorrent 5.2.3-libtorrentv1, gluetun v3.41.3, Plex 1.43.3.10828.

## Kubernetes Management

Rafael is comfortable with Kubernetes and wants homelab patterns that mirror production while staying practical.

Guidance:

- keep cluster architecture boring and recoverable
- prefer a small number of well-sized nodes over too many tiny VMs unless isolation is needed
- separate storage concerns from stateless workloads when practical
- be careful with persistent volumes and backups before app deployment changes
- include ingress, storage, backup, monitoring, and upgrade implications when suggesting services
- Ansible inventory groups include `home_base`, `k8s_master`, `k8s_nodes`; repo manages apt sources/versions manually for K8s updates

## Networking / Ingress / Auth

Known endpoints/domains:

- Home Assistant: `https://homeassistant.theramirez.casa`
- Proxmox prior endpoint: `https://192.168.1.28:8006`
- domains: `theramirez.casa`, `homeapps.io`

Kubernetes ingress/Gateway state observed 2026-06-05:

- Current kube context: `home-base`
- MetalLB runs in `metallb-system` as Helm release/instance `home-lb`; speakers run on all Kubernetes nodes.
- MetalLB `IPAddressPool` `home-network` advertises `192.168.1.80-192.168.1.85`; `L2Advertisement` `home-network` advertises that pool.
- Allocated LoadBalancer VIPs observed: nginx ingress `192.168.1.80`, PostgreSQL `192.168.1.81`, Istio ingress `192.168.1.82`, kgateway `home` `192.168.1.83`.
- `192.168.1.83` is Kubernetes-owned via MetalLB, not a physical host IP. It backs `kgateway-system` Service `home` and Gateway `home` (`GatewayClass=kgateway`, programmed=True).
- Gateway `home` has HTTP `:80` and HTTPS `:443` listeners for `*.theramirez.casa`; TLS uses Secret `theramirez-casa-wildcard-tls` and cert-manager ClusterIssuer `letsencrypt-cloudflare`.
- 2026-06-13: Gateway `home` also has HTTPS listeners for `dipradar.io` and `*.dipradar.io` using cert-manager-created Secrets `dipradar-io-tls` and `dipradar-io-wildcard-tls`; both certs were Ready/valid after applying. `external-dns` release `external-dns` revision 14 filters zones to `theramirez.casa`, `homeapps.io`, and `dipradar.io`. `kgateway` release revision 7 carries the new listeners. Same existing Cloudflare token Secret is used; no new Secret required.
- Repo chart `homebase-infra/charts/ingress` / release `home-endpoints` creates kgateway `Backend` + `HTTPRoute` objects for external LAN services.
- `store.theramirez.casa` currently routes through Gateway VIP `192.168.1.83` to kgateway Backend `home-endpoints/proxmox-store`, static upstream `192.168.1.216:8006` with upstream TLS verify skipped.
- `homeflix.theramirez.casa` routes through Gateway VIP `192.168.1.83` to LXC `100` / `192.168.1.218:32400`.

Guidance:

- separate LAN-only vs internet-exposed services
- call out security implications before exposing Proxmox, NAS, Home Assistant, auth portals, or dashboards
- prefer VPN, Cloudflare Tunnel, Authentik forward auth, MFA, and IP allow lists for sensitive services
- for Kubernetes ingress to external services, use `Service` + `Endpoints`/`EndpointSlice` patterns when appropriate; use `ExternalName` only when appropriate
- always include TLS/certificate considerations
- Rafael wants Authentik for homelab SSO where it fits; distinguish native OIDC apps from forward-auth/proxy-auth apps
- keep break-glass local admin for Proxmox, NAS, and Home Assistant

## Plane CE Pilot (updated 2026-09-01)

- Chart `charts/plane` (untracked, local-only), Helm release/namespace `plane`. Plane CE `v1.3.1` pinned by digest (`makeplane/plane-{backend,frontend,space,admin,live,proxy}@sha256:...`, recorded in `charts/plane/image-lock.yaml`). Dependencies: `postgres:15.7-alpine`, `valkey/valkey:7.2.11-alpine`, `rabbitmq:3.13.6-management-alpine`, `chrislusf/seaweedfs:4.39` (multi-arch index digest) — **SeaweedFS, not MinIO**, deliberately swapped in for S3-compatible object storage.
- Official Plane Caddy proxy (`plane-proxy:v1.3.1`) runs unmodified with its baked-in Caddyfile for internal app routing (`/api`, `/spaces`, `/god-mode`, `/live`, `/*` → `web`). kgateway Gateway `home` HTTPS listener (`*.theramirez.casa` wildcard, `192.168.1.83`) carries two `HTTPRoute`s: `plane.theramirez.casa` (app, → proxy) and `s3.theramirez.casa` (uploads, → SeaweedFS directly, bypassing the proxy's dead MinIO route). All PVCs use StorageClass `ssd`. As of 2026-09-01, live revision 5 has all 12 Deployments Available, `plane-migrator-5` Complete, both HTTPRoutes Accepted/Resolved, and app HTTP 200. Resource sizing: API `384Mi` request/`512Mi` limit; worker `512Mi`/`768Mi`; beat and live each `192Mi`/`256Mi`; total steady-state Deployment requests are `1500m` CPU/`2816Mi` memory, below the 3 CPU/10 GiB pilot ceiling.
- Deploy/update workflow (`Taskfile.yaml` `plane:*` tasks, added alongside the existing `mealie:*` pattern): `task plane:lint` → `task plane:render` → `task plane:dry-run` (all read-only/non-mutating) → `task plane:upgrade` (the only mutating step; interactive `prompt:` confirmation, requires `kubectl config current-context == home-base`, requires a local `charts/plane/secret-values.yaml` present — see `secret-values.example.yaml` for the shape; that file is gitignored and must never be committed).
- **Hard rules — do not violate these**:
  - **LAN/VPN-only.** Never add `plane.theramirez.casa` or `s3.theramirez.casa` to the Cloudflare Tunnel ingress (API or `values.yaml`) — same boundary already enforced for `townhome-admin` and other internal-only hosts.
  - **Disposable pilot — no real data.** This is a throwaway evaluation deployment. Do not create real workspaces, real tasks, or any employer/production data in it.
  - **Local `ssd` reclaim policy is `Delete`.** Deleting the `plane` namespace or any of its PVCs is irreversible — there is no off-cluster backup for this pilot yet.
  - **Off-cluster backup + restore is the next blocking gate before any real data ever goes into this pilot.** Do not relax the disposable-pilot rule until that gate is built and tested.
  - **Known issue:** fixed-name `plane-seaweedfs-bucket` Job hangs because `weed shell` requires unexposed gRPC ports `19333`/`18888`. The revision-5 ephemeral Job hit `activeDeadlineSeconds=300` and was deleted after the API confirmed `Bucket plane-uploads exists`. Until the template changes, the next upgrade recreates this failed Job. Recommended future fix: a validated idempotent Filer HTTP API implementation, rather than exposing gRPC solely for the Job.
- Real presigned upload through `s3.theramirez.casa` and backup/restore remain unverified. The pilot remains disposable and LAN/VPN-only despite the successful revision-5 rollout.

## ntfy Push Notifications (added 2026-07-08)

- Chart `charts/ntfy` (mealie-pattern: namespace/pvc/deployment/service/httproute templates), release `ntfy`, ns `ntfy`, image `binwiederhier/ntfy:v2.25.0`.
- Endpoint: `https://ntfy.theramirez.casa` (kgateway `home`, external-dns auto-created record → 192.168.1.83). Health: `GET /v1/health`.
- HTTPRoute timeouts set to `0s` (disabled) on purpose — ntfy subscribers hold long-lived streams; do NOT "fix" to a finite timeout or phone push breaks.
- Cache PVC 1Gi `ssd` at `/var/cache/ntfy`; messages cached 72h.
- Auth: none (`NTFY_ENABLE_LOGIN=false`), LAN-only DNS. If ever exposed beyond LAN, add auth-file + `NTFY_AUTH_DEFAULT_ACCESS=deny-all` first.
- No wired consumers yet (question-notify opencode plugin built 2026-07-08 then removed same day; Rafael has bigger plans for ntfy).
- Publish test: `curl -H "Title: t" -d "body" https://ntfy.theramirez.casa/<topic>`.

## Monitoring / Alerting

Monitoring preferences:

- free alerting if a Proxmox server goes down
- Prometheus Operator preferred
- simple monitoring first: node exporter, blackbox exporter, Prometheus, Alertmanager, Grafana, uptime checks

High-signal alert categories:

- host down
- disk/pool health
- disk usage
- memory pressure
- CPU saturation
- Kubernetes node not ready
- certificate expiry
- backup failures

Avoid noisy alerts.

## Backup / DR

Backblaze Personal K8s experiment:

- Chart added 2026-06-07: `charts/backblaze-wine`, namespace `backblaze`, release `backblaze-wine`, hostname `backblaze.theramirez.casa` via kgateway Gateway `home`.
- Purpose: run `tessypowder/backblaze-personal-wine:v1.7.2` so NAS backups do not depend on Rafael's Windows gaming PC/Dokan mount staying online.
- Reliability defaults: separate namespace `backblaze` prepared outside Helm by `task backblaze:prepare-namespace`, single replica, `Recreate`, `/config` PVC with Helm keep annotation, NAS mounted read-only at `/drive_d`, `ENABLE_NETWORK_MOUNT_MASKING=true`, `SYS_ADMIN`, unconfined seccomp and AppArmor, `shareProcessNamespace=true`.
- Pod is pinned with `node-role.theramirez.casa/storage-host-adjacent=true`; currently schedules to `home-base5`, the Kubernetes worker on `pve-1` alongside the NAS/storage host. This minimizes NAS path latency but uses the 8 GB node, so keep Backblaze memory limits realistic or move it to a 16 GB `pve-2` node if scans OOM.
- Chart expects VNC Secret `backblaze/backblaze-wine-vnc` key `password` created out of band; never commit Backblaze credentials or VNC password.
- Chart defaults NAS NFS path to `192.168.1.216:/home-storage`, mounted read-only in the pod at `/drive_d`; most important data is `/drive_d/home-cloud/home-share/Videos` and `/drive_d/home-cloud/home-share/Photos`.
- Manage required pve-1 export with `configure-backblaze-nfs-export.yaml` or `task backblaze:configure-nfs-export`; it writes `/etc/exports.d/backblaze-home-storage.exports` with `/home-storage 192.168.1.0/24(ro,sync,no_subtree_check,no_root_squash)` and reloads exports.
- NAS permissions observed 2026-06-07: `/home-storage/home-cloud/home-share`, `Videos`, and `Photos` are owned by `100000:101001`; Backblaze chart uses `USER_ID=100000` and `GROUP_ID=101001` so scans can traverse/read those paths.
- Do not retire the Windows/Dokan backup path until the K8s client has completed initial backup and stayed healthy; Backblaze Personal may treat the container as a new computer unless state inheritance succeeds.
- 2026-06-07 troubleshooting: Backblaze Wine pod was `Running` but client state was incomplete/stuck. `bzinfo.xml`, `bzinstall.xml`, `userPub.pem`, and `bzserv_version.txt` were missing; logs showed `Installer_CallBzTransmit_AtInstallTimeCheckUser FAILED` and `FALSE:http_post_err` from `bztransmit.exe -at_install_time_checkuser` even though Linux `curl` from the pod to `https://ca000.backblaze.com/api/clientversion.xml` and `https://secure.backblaze.com/win32/install_backblaze.exe` succeeded. A pre-repair backup was written inside the PVC at `/config/backblaze-wine-pre-repair-20260607-190007.tgz`. Wine registry was updated so `HKCU\Software\Wine\Drives` has `D:=hd`, but `/drive_d` remained a plain NFS mount and no network-share overlay masking appeared. Old Helm upgrades preserved release values; `helm upgrade --reset-values` was required to move from image `tessypowder/backblaze-personal-wine:v1.7.2` to `:ubuntu24`.
- 2026-06-07 latest-client reset: chart now uses image tag `ubuntu24`, `imagePullPolicy=Always`, `DISABLE_AUTOUPDATE=false`, `FORCE_LATEST_UPDATE=true`; release upgraded to revision 5 with `--reset-values`. Broken prefix was moved aside to `/config/wine.broken-20260607-193126` after backup `/config/backblaze-wine-before-latest-20260607-190828.tgz`. Fresh pod `backblaze-wine-7c7cd9988-xkptv` pulled `ubuntu24`, seeded a new Wine prefix, downloaded latest official installer from `https://www.backblaze.com/win32/install_backblaze.exe`, and started `C:\install_backblaze.exe`; local version file remains missing until GUI install/sign-in completes. New image has masking logic, but masking failed because app user could not create `/drive_d_local` (`mkdir: Permission denied`), so `/drive_d` still falls back to plain NFS and Backblaze may skip it until chart creates/writes `/drive_d_local` or otherwise fixes overlay target permissions. Treat this Backblaze-in-K8s path as experimental until latest install, login, D-drive masking, and initial backup are validated.
- 2026-06-07 Backblaze login/version: user signed in as `rafi89@gmail.com`; latest client installed successfully (`10.0.1.1037`). Backblaze refused to unselect `C:\` because it treats it as main disk; leave `C:\` selected and never select `Z:\` (`Z:` maps to container `/`). The `N5A` machine name appeared in GUI.
- 2026-06-07 D-drive fix: overlay masking made `D:\` show but empty because overlay-on-NFS returned `Object is remote` for `Photos`/`Videos`. User approved RW NFS for Backblaze home-share. `configure-backblaze-nfs-export.yaml` now exports `/home-storage/home-cloud/home-share 192.168.1.0/24(rw,sync,no_subtree_check,no_root_squash)` on `pve-1`; chart direct-mounts that path as `/drive_d` with `nas.readOnly=false`, `ENABLE_NETWORK_MOUNT_MASKING=false`, `USER_ID=0`, `GROUP_ID=0`. Helm release upgraded to revision 11. Validation: `/drive_d` mounted rw from `192.168.1.216:/home-storage/home-cloud/home-share`, `D:` maps to `/drive_d/`, `Photos` and `Videos` are visible, and `.bzvol` write test succeeded. Security tradeoff: Backblaze pod now has write access to the home-share root so it can create `.bzvol`; scope is narrower than whole `/home-storage`, but still treat the pod as write-capable to shared NAS data.
- 2026-06-08 Backblaze Wine status: pod `backblaze-wine-7ff7649c6-9cqgb` on `home-base5` is healthy and Backblaze client `10.0.1.1037` is logged in/running, but VNC may show only a blue Openbox desktop while `bzbui.exe` still runs. Runtime reports `D:\` as a `Network` mount, not fixed/local; Backblaze status showed `D:\` selected but `pervol_sel_for_backup_numfiles=0` while `/drive_d/Photos` and `/drive_d/Videos` are visible from Linux. This means the Kubernetes/NFS approach is not yet a trusted NAS backup path; Backblaze Personal likely ignores the NFS-backed data even when mapped as `D:`. Do not retire Windows/Dokan backup path. If continuing Personal Backup, simplest likely next experiment is running Wine/container on `pve-1` with a local bind mount of `/home-storage/home-cloud/home-share` so Wine sees `D:` as local, not NFS; safer supported alternative is B2 with restic/kopia/rclone for NAS data.
- 2026-06-08 Backblaze migration: user confirmed B2 is too expensive for this data and approved moving Personal Backup out of Kubernetes into LXC. Helm release `backblaze-wine` was uninstalled from namespace `backblaze`; only kept PVC `backblaze-wine-config` remains for rollback/state recovery. Created pve-1 LXC `106` / `backblaze.theramirez.casa` / `192.168.1.79`, privileged, `onboot=1`, 2 cores, 4 GB RAM, 1 GB swap, 40 GB rootfs on `local-lvm`, Docker installed, with local bind mounts `mp0=/home-storage/home-cloud/home-share -> /drive_d` and `mp1=/home-storage/backblaze-wine/config -> /opt/backblaze-wine/config`. Repo playbook/task: `setup-backblaze-lxc.yaml` / `task backblaze:setup-lxc`; playbook validates existing LXC `106` config before continuing so Backblaze cannot run against a wrong/empty `/drive_d`. The Docker systemd unit is installed but intentionally inactive until `/etc/backblaze-wine.env` is created inside LXC with `VNC_PASSWORD`; never store that secret in git or agent memory. URL after start: `http://192.168.1.79:5800`. Existing pve-1 NFS export `/etc/exports.d/backblaze-home-storage.exports` for the retired K8s experiment still exists and exports `/home-storage/home-cloud/home-share` RW to `192.168.1.0/24`; remove only with explicit approval because it changes NFS/storage access. Prepared cleanup playbook/task: `remove-backblaze-nfs-export.yaml` / `task backblaze:remove-retired-nfs-export`.
- 2026-06-08 Backblaze LXC runtime: user ran the NFS cleanup and Backblaze service start steps. Verified `/home-storage/home-cloud/home-share` Backblaze RW NFS export was absent from `exportfs -v`; existing Photos/Videos exports may still remain. Verified LXC `106` running, `/etc/backblaze-wine.env` present, `backblaze-wine.service` enabled/active, Docker container `backblaze-wine` running and publishing `0.0.0.0:5800->5800/tcp`, and noVNC returned HTTP 200 at `http://127.0.0.1:5800` inside the LXC. Installer populated `/opt/backblaze-wine/config/wine/drive_c/Program Files/Backblaze` including `bzbui.exe`. VNC password was provided by user in chat but must not be stored in repo/AGENTS or printed in future outputs. Next validation: log in via `http://192.168.1.79:5800`, complete Backblaze sign-in/inherit state if desired, verify `D:` is not reported as `Network`, selected files for `D:` are non-zero, and perform a small restore test before retiring Windows/Dokan backup.
- 2026-06-08 Backblaze LXC validation: Backblaze in LXC now sees `D:\` as `type="Fixed"` not Network, and `D:` has non-zero selection (`pervol_sel_for_backup_numfiles="33750"`, `pervol_sel_for_backup_numbytes="3661166880410"` observed in `bzhost_testimony_x.xml`). Linux sees `/drive_d/Photos` and `/drive_d/Videos` with 604 files at maxdepth 2. Warning: Backblaze also showed `Z:\` selected (`mountPointNameHex="5a3a5c"`, `pervol_sel_for_backup_numfiles="32071"`); user should unselect `Z:\` because it maps container root, not NAS data. A permission warning on `C:\ProgramData\Backblaze\bzdata\bzreports` was mitigated by backing up Backblaze ProgramData to `/config/backblaze-perms-before-*.tgz` and running `chmod -R a+rwX /config/wine/drive_c/ProgramData/Backblaze`; continue watching for recurrence. Do not retire Windows/Dokan backup until Backblaze completes backup and a restore test succeeds.
- 2026-06-08 Backblaze full home-cloud + monitoring: user clarified desired backup scope is entire `/home-storage/home-cloud`, not only `home-share`/Photos/Videos. LXC `106` now has `mp0=/home-storage/home-cloud -> /drive_d` plus `mp2=/home-storage/home-cloud/home-share -> /drive_d/home-share` so child dataset `home-share` is visible under the full home-cloud root; `setup-backblaze-lxc.yaml` updated accordingly. Backblaze UI now shows duplicate `D:` entries because the drive identity changed after remount: old selected `D:` appears unplugged, current full-home-cloud `D:` appears unchecked; user should uncheck unplugged old `D:`, check current `D:`, and uncheck all `Z:` entries because `Z:` maps container root. Added `install-backblaze-exporter.yaml` / `task backblaze:install-exporter`, exposing `http://192.168.1.79:9810/metrics`; Prometheus scrape job `backblaze-personal` added and applied in `prometheus-stack` revision 20; `PrometheusRule` `backblaze-personal` and Grafana dashboard `Homelab / Backblaze Personal` created. Prometheus `up{job="backblaze-personal",instance="192.168.1.79"}=1` verified. Current exporter shows service/container active, `D:` fixed/selected but selected files `0` until the user selects the current full-home-cloud `D:` in the Backblaze UI; `backblaze_z_drive_selected=1` until user unchecks `Z:`.
- 2026-06-08 Backblaze clean reset: user wanted to start from scratch because duplicate/unplugged drive state was confusing. Stopped `backblaze-wine`, backed up old config to `/home-storage/backblaze-wine/resets/config-before-clean-reset-20260608-084708.tgz`, moved old live config to `/home-storage/backblaze-wine/config.reset-20260608-084708`, recreated empty `/home-storage/backblaze-wine/config`, moved old Backblaze volume markers to `/home-storage/home-cloud/.bzvol.reset-20260608-084909` and `/home-storage/home-cloud/home-share/.bzvol.reset-20260608-084909`, rebooted LXC `106` so bind mounts picked up the clean config, and restarted `backblaze-wine`. `setup-backblaze-lxc.yaml` now removes Wine `Z:` symlink after container start to reduce accidental container-root backup. Clean exporter state after reboot: `backblaze_service_active=1`, `backblaze_container_running=1`, `backblaze_reports_present=1`, `backblaze_z_drive_selected=0`, `backblaze_d_drive_selected=0`, `backblaze_d_drive_selected_files=0` until the user signs in/selects `D:`. Next UI action: log in at `http://192.168.1.79:5800`, select only current `D:` for full `/home-storage/home-cloud` backup, leave `Z:` absent/unchecked, keep `C:` if Backblaze forces it.
- 2026-06-09 Backblaze 65GB-only root cause: Backblaze was backing up only ~82.5GB of `D:` while `/home-storage/home-cloud` holds 2.90T. Cause: `home-share` is a separate ZFS dataset (`st_dev` 47 vs 43), and Backblaze Personal never crosses mount-point/volume boundaries inside a drive — so the 2.78T `home-share` nested under `D:\` was silently skipped; only home-cloud root (70.6G refer) was scanned. Fix: added Wine drive `E:` -> `/drive_d/home-share` (`dosdevices/e:` symlink), so Backblaze enumerates it as a second Fixed 13.7T volume. `setup-backblaze-lxc.yaml` ExecStartPost and `fix-backblaze-ui.yaml` now enforce the `e:` mapping alongside `z:` removal. Added `BackblazeEDriveNotBackupReady` alert (includes `absent()` guard) to `charts/prometheus-stack/rules/backblaze-personal.yaml`, applied to cluster. Rule pattern note: exporter label is literal `E:\`, PromQL in YAML plain scalar must use `mount="E:\\"`. Remaining manual step: select `E:` in Backblaze Settings via `http://192.168.1.79:5800` (keep `D:` and `C:` selected, never `Z:`); selecting `E:` writes `.bzvol` into `home-share` (rw bind, expected). General rule: any new ZFS dataset nested under a Backblaze-backed path needs its own Wine drive letter or Backblaze ignores it.
- 2026-06-10 bzreports permission warning is cosmetic: GUI popup "Permission Issue ... C:\ProgramData\Backblaze\bzdata\bzreports" recurred in LXC `106` while everything was actually healthy. Verified: all of `bzdata`/`bzreports` is `777 root:root`, container runs as root, `bzreports/bzstat_bzserv_perm.txt` = `yes_none` (client self-check passes), `bzdefcon.xml` updated hourly with `success updating file`, uploads actively transmitting (`bzstat_upload_success.xml` showed 2001 successes 2026-06-09 + 1001 on 2026-06-10, 0 CvtTooBusy/CvtNoRoom fails), `bzreports_lastfilestransmitted` updating within 2 min of current time. Root cause: bzbui's Windows ACL probe is unreliable under Wine (no real NTFS security descriptors) and intermittently throws this dialog even when reads/writes succeed. Action: close the dialog and ignore unless exporter shows backup stalled (`backblaze_remaining_bytes` flat or upload fails climbing). Do not chmod-loop or reset the prefix for this warning alone.
- 2026-07-02 Backblaze upload speed tuning: initial backup crawled at ~29 GB/day 7d-avg (~2.7 Mbps) because client defaults in `bzinfo.xml` were `net_auto_throttle="true"`, `net_throttle="50"`, `num_backup_threads="1"`. Final config: `net_auto_throttle="false"`, `net_throttle="100"`, `num_backup_threads="40"`; LXC 106 bumped 2→4 cores via `pct set 106 -cores 4` (cpuset applied live after ~1 min delay, no reboot needed). Real bottleneck after de-throttle: Wine gives each upload connection ~0.5-0.7 Mbps on the 93ms-RTT path to Backblaze pods (WAN measured ~350 Mbps up from pve-1, container netns measured ~343 Mbps to Cloudflare — network path is NOT the limit). Thread count is the scaling lever: 6 threads=4.2 Mbps, 20=13.5 Mbps, 40=20.2 Mbps (~218 GB/day, diminishing per-conn returns above 20; stopped at 40 for memory/stability — container ~2.0/4 GiB, CPU ~14%, zero CvtTooBusy at 40). Client honors >20 threads (thread IDs to 035+ observed). Grafana `Service running: STOPPED` + `Last backup age: No data` during tuning were false alarms from the stop/edit/start maintenance windows; uploads continue via docker container while systemd unit briefly stops. bzreports lives at `bzdata/bzreports` inside the Wine prefix. Procedure: MUST `systemctl stop backblaze-wine` inside LXC before editing `/opt/backblaze-wine/config/wine/drive_c/ProgramData/Backblaze/bzdata/bzinfo.xml` or the client overwrites edits; backup saved at `bzinfo.xml.bak-perf-20260702-203001`. Access note: direct SSH `root@192.168.1.79` is denied from home-mac (publickey) — go through `root@192.168.1.216` + `pct exec 106`. Measure upload rate with Prometheus `delta(backblaze_remaining_bytes[24h])` or Grafana `Homelab / Backblaze Personal`; remaining ceiling after de-throttle is WAN upload bandwidth.
- 2026-06-08 Backblaze UI repair helper: added `fix-backblaze-ui.yaml` / `task backblaze:fix-ui` for noVNC blue/black screen. It stops `backblaze-wine`, moves `/opt/backblaze-wine/config/xdg` aside to `xdg.reset-*`, removes Wine `Z:` mapping, enforces `D:` mapping to `/drive_d`, relaxes Backblaze ProgramData permissions, restarts the service, removes `Z:` again inside the container, and opens `D:` in Wine Explorer to force a visible window. Use when noVNC shows only blue/black desktop despite service/container running.

Always consider backups when changing storage, Kubernetes, media, or auth.

Frame backup recommendations around:

- what data matters
- where it is stored
- what backs it up
- how Rafael would restore it
- whether restore has been tested

Include config backups, ZFS snapshots if applicable, offsite/external backup for irreplaceable data, app-level database backups, Kubernetes manifests/Git repos, and secret recovery plan.

## Safe Troubleshooting Flow

Prefer read-only diagnosis first:

- `kubectl get pods -A -o wide`
- `kubectl describe ...`
- `kubectl logs ...`
- `kubectl get events -A --sort-by=.lastTimestamp`
- `helm status ...`
- `ansible-inventory --graph`
- check Prometheus targets before changing scrape config
- `systemctl status <service>`
- `journalctl -u <service> --no-pager -n 100`
- `lsblk`
- `zpool status`
- `zfs list`
- `ip a`
- `ip route`

Ask before destructive or service-impacting actions:

- node reboot
- Kubernetes upgrade
- Proxmox changes
- deleting pods/PVCs/namespaces
- DNS/Gateway/firewall/routing changes
- storage/ZFS/NFS changes
- applying manifests that affect shared ingress/monitoring/storage

Security guardrails:

- be careful with privileged containers, GPU passthrough permissions, HostPath mounts, LXC privilege boundaries, and backups containing sensitive data
- never expose Proxmox/NAS/Samba/Home Assistant/admin dashboards publicly without explicit security review
- never recommend destructive storage commands without warning and safer verification first

## Config Output Preferences

When generating configs, prefer clear file paths, complete YAML when reasonable, comments only where helpful, small copy-ready scripts, Taskfile examples for multi-step workflows, Go examples for reusable tools, Ansible for OS/node management, and practical names/placeholders.

## Secrets

No `ansible-vault` in this repo. Real group secrets live in gitignored `group_vars/<group>/vault.yaml` files. Never print, commit, or store secrets in agent memory.

## bonsai-lxc opencode worker server (added 2026-08-10)

- `bonsai-lxc` = pve-1 LXC 102, Ubuntu 24.04, `192.168.1.9`, 4 cores / 16 GB (mostly idle), NVIDIA GTX 1080 passthrough (Ollama on :11434). Also hosts the opencode headless worker.
- Service: `opencode-server.service` (systemd, enabled), runs `/root/.opencode/bin/opencode serve --hostname 0.0.0.0 --port 4096`. opencode 1.18.16 installed via official installer to `/root/.opencode/bin` (PATH only in `/root/.bashrc` — service unit sets its own PATH).
- Model pinned in `/root/.config/opencode/opencode.json` to `opencode/deepseek-v4-flash-free` (OpenCode Zen free model, NO auth credential needed — free tier works without `opencode auth login`).
- **Deliberately minimal (2026-08-10, Rafael directive): do NOT replicate the Mac opencode stack here** — no oh-my-openagent.json, no agents/, skills/, scripts/, no `~/.agents`. Plain deepseek worker only. Two additions ARE present per Rafael's explicit asks: (1) `agent-memory` MCP wired into `opencode.json` (`command: ["opencode-agent-memory"]`, v0.2.1 from `github:rramirz/opencode-agent-memory` — registry 0.2.0 has no bin; `MEMORY_TOKEN` lives in `/root/.opencode-server.env` because `$(cat ...)` subshells don't survive opencode 1.18.16 process spawn); (2) the second-brain "soul" — `/root/.config/opencode/context/` (second-brain.md, notes.md, dreams/core.md, dreams/local.md, homespace/, workspace/) plus `plugin/cwd-context.ts`, both copied from the Mac, injected into sessions (verified 2026-08-10). Credentials for agent-memory: `/root/.agent-memory/{config.yaml,token,admin-token}` (chmod 600, `workstation: bonsai`, `default_org: personal`). If a task needs the full agent fleet, run it on the Mac.
- **NO AUTH as of 2026-08-10** (Rafael requested open access). Server accepts connections with no credentials; the env file now holds only `MEMORY_TOKEN` (used by the agent-memory MCP, not by the server auth). Anyone on the LAN can hit `:4096` and get full shell on bonsai — do not expose via tunnel/port-forward.
- Route tasks from any machine: `opencode run --attach http://ai.theramirez.casa:4096 "task"` or `opencode attach http://ai.theramirez.casa:4096` for interactive TUI. Work runs on bonsai, client stays light. `ai.theramirez.casa` resolves to `192.168.1.9` (A record via external-dns, created 2026-08-11 — headless Service `home-endpoints/ai` with `external-dns.alpha.kubernetes.io/hostname` + `target` annotations, added via the `dnsRecords` list in `charts/ingress/values.yaml`; distinct from the `services` list which routes via the Gateway VIP).
- LAN-only; no Cloudflare Tunnel/port-forward. Verified 2026-08-10: session ran on deepseek-v4-flash-free end-to-end.
