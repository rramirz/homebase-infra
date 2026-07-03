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
- Cloudflare Tunnel routes: **the `ocean-sounds` tunnel (`251632cb-5abe-4397-8294-1e8e3f6c5b7e`) is a remotely-managed tunnel (`config_src: "cloudflare"`)**. Its ingress rules live in Cloudflare's control plane via the Tunnel Configuration API (`PUT /accounts/{account}/cfd_tunnel/{tunnel}/configurations`), NOT in `charts/cloudflared/values.yaml`/the local `config.yaml` ConfigMap — that local file is mounted into the pod but is **completely ignored** by a remotely-managed tunnel. Keep `values.yaml` in sync for documentation/rollback reference, but any real ingress change must be pushed via the API too, or it silently does nothing (discovered 2026-07-01 after a values.yaml-only change produced 404s at the edge with zero cloudflared logs).
- **2026-07-01: townhome tunnel cutover is LIVE.** Both `townhome-test.theramirez.casa` and `townhome.theramirez.casa` are proxied CNAMEs (`<tunnel-id>.cfargotunnel.com`) routing to `http://townhome-frontend.townhome.svc.cluster.local:80` via the tunnel's remote config. **Townhome tunnel boundary**: listing frontend + offer/showing submission only (`POST /api/offers`, `POST /api/showings`, `GET /api/health`) — nginx in the app itself blocks `/admin*` and other `/api/*` paths regardless. Seller offer-review/admin uses separate host `townhome-admin.theramirez.casa` via the app repo's `k8s/admin-httproute.yaml` (now applied to the cluster, previously was not) on the internal kgateway `home` Gateway (LAN IP `192.168.1.83`) — reachable only from the home network/VPN, deliberately NOT in the tunnel config. Do not add `townhome-admin.theramirez.casa` to the tunnel's ingress (API or values.yaml); public listing hosts must not route `/admin*` or `/api/admin/*` at all.
- **external-dns ownership conflict pattern**: `external-dns` (source `gateway-httproute`, domain-filter `theramirez.casa`/`homeapps.io`/`dipradar.io`, policy `upsert-only`) auto-manages DNS for any HTTPRoute hostname it can see, and will silently revert manual DNS changes to that hostname on its ~60s reconcile loop as long as the HTTPRoute exists. To let a Cloudflare Tunnel CNAME "win" for a hostname that also has a live HTTPRoute (kept as an internal/rollback path), annotate that HTTPRoute with `external-dns.alpha.kubernetes.io/controller: <anything-other-than-dns-controller>` (e.g. `ignore`) — official external-dns behavior, causes the source to skip that resource entirely. Used on the `townhome` HTTPRoute in namespace `townhome` 2026-07-01 to free `townhome.theramirez.casa` for the tunnel while keeping the HTTPRoute/kgateway path intact as rollback. `townhome-admin` HTTPRoute was deliberately left unannotated (still external-dns managed, LAN-only, correct).
- **2026-07-01 incident: Cloudflare API token had been dead for ~17 days**, breaking `external-dns` (`CrashLoopBackOff`, `403 Invalid access token`, code 9109, all DNS automation stalled) and putting the `theramirez-casa-wildcard-tls` cert renewal (due ~2026-08-02) at risk via the same broken `cloudflare-api-token-secret` used by cert-manager's `letsencrypt-cloudflare` ClusterIssuer. Root cause: token expired/revoked at Cloudflare's end; local `~/.cloudflare.env` had also gone stale. Fixed by rotating to a fresh token and updating all three consumers: `cert-manager/cloudflare-api-token-secret` (key `api-token`), `external-dns/cloudflare-credentials` (key `api-token`), and `~/.cloudflare.env`. **Gotcha**: `https://api.cloudflare.com/client/v4/user/tokens/verify` rejected the new, working, narrowly-scoped token with `401 Invalid API Token` even though it worked fine for real calls (`/zones`, DNS edits) — that endpoint is unreliable for scoped tokens; verify with an actual scoped call (e.g. `GET /zones?name=<zone>`) instead. Token scope needed: `Zone:DNS:Edit` for the three zones (cert-manager + external-dns) plus `Account:Cloudflare Tunnel:Edit` (for pushing tunnel ingress config via API). Restart both `external-dns` and `cert-manager` deployments after rotating the secret to force them to pick it up and clear any crash loop.

Default inventory: `hosts`. Default SSH user from `ansible.cfg`: `rramirez`, except groups with explicit `ansible_user=root`.

Task harness: `Taskfile.yaml` is the preferred entrypoint for routine operations. Prefer descriptive task names: `task check:kubernetes-health`, `task check:proxmox-guests`, `task check:nas-and-sync-storage`, `task install:all-node-exporters`, `task install:node-exporters-proxmox-lxcs`, `task update:active-kubernetes-node-os`, `task update:ssh-managed-server-os-no-reboot`, `task update:kubernetes-version`, `task update:plex-media-server`, `task update:syncthing-lxc`, `task monitoring:render-prometheus-stack`, `task monitoring:upgrade-prometheus-stack`, `task monitoring:apply-grafana-dashboards-and-rules`, `task monitoring:show-syncthing-storage`. Short aliases like `task k8s:status`, `task exporters:all`, and `task syncthing:update` still work.

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
