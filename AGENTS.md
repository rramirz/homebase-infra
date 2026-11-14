# Homebase Infra — Agent Context

Full pre-tidy incident & blow-by-blow history archive: `docs/history/AGENTS-history-20261113.md`. Keep this file under 20KB (`~/.config/opencode/scripts/agents-md-budget.sh`).

## What This Repo Is

`homebase-infra` manages Rafael's home-base infrastructure using Ansible plus in-repo third-party/umbrella Helm charts (`charts/`). First-party homelab apps belong under `apps/<app>/` with adjacent Helm charts.

- **Default inventory:** `hosts` (`rramirez` default SSH user; `proxmox` and `lxc` use `root`).
- **Task harness:** `Taskfile.yaml` (`task check:kubernetes-health`, `task check:proxmox-guests`, `task check:nas-and-sync-storage`, `task check:cluster-upgrades` / `task nova`, `task verify:kubernetes-time-sync`, `task monitoring:upgrade-prometheus-stack`, `task monitoring:apply-grafana-dashboards-and-rules`, `task flannel:*`, `task plane:*`, `task paperclip:*`, `task proxmox:major:*`).
- **Cloudflare Tunnel (`ocean-sounds`, `251632cb-5abe-4397-8294-1e8e3f6c5b7e`):** Remotely-managed (`config_src: "cloudflare"`). Ingress lives in Cloudflare API (`PUT /accounts/{account}/cfd_tunnel/{tunnel}/configurations`); `charts/cloudflared/values.yaml` is ignored at runtime. Keep `values.yaml` synced, push real changes via API.
  - **Townhome tunnel boundary:** `townhome.theramirez.casa` and `townhome-test.theramirez.casa` route to `http://townhome-frontend.townhome.svc.cluster.local:80` (public listing + `POST /api/offers`, `POST /api/showings`, `GET /api/health` only). `townhome-admin.theramirez.casa` is LAN/VPN-only on kgateway `home` (`192.168.1.83`) — **never** add to Cloudflare Tunnel.
  - **external-dns conflict avoidance:** Annotate HTTPRoutes that share a hostname with a Cloudflare Tunnel CNAME using `external-dns.alpha.kubernetes.io/controller: ignore`.
  - **Cloudflare token rotation:** Update all three consumers (`cert-manager/cloudflare-api-token-secret`, `external-dns/cloudflare-credentials`, `~/.cloudflare.env`) and restart `external-dns` + `cert-manager`. Verify scoped tokens via `GET /zones?name=theramirez.casa` (not `/user/tokens/verify`).
- **OPNsense WireGuard (`router.theramirez.casa`, `26.1.11_10`):** Runs `homeVPN` on `wg0`, UDP `51820`, tunnel `10.11.11.0/24` (LANs `192.168.{1,2,10}.0/24`). Current WAN IP `67.224.61.251`; `os-ddclient` installed for planned DNS-only `vpn.theramirez.casa:51820`. Never use `router.theramirez.casa` as endpoint (resolves to private `192.168.1.83`).
- **`apps/family-retirement-planner`:** Static-only Go/PostgreSQL retirement & estate workbench (not deployed). LAN/VPN-only, never Cloudflare Tunnel, no real data until encrypted off-cluster backup + tested restore exist, no credentials in digital metadata.

- **New apps (rule, 2026-11-13):** Deploy to home cluster only via Helm chart. DBs come from the Crossplane `DB` claim (`theramirez.casa/v1alpha1`, composite `XDB`) on shared `home-db`, not per-app CNPG `Cluster` CRDs, unless isolation needed. Go apps: `task app:new NAME=<app> [DB=yes|no]` (cookiecutter `templates/go-app` -> `apps/<app>/`). Third-party apps: `task chart:new NAME=<chart>` (umbrella chart via `~/homespace/cookiecutter-helm-template` into `charts/`). Pattern: `docs/runbooks/app-database.md`. Open: XRD not in repo; `db.host` default `home-db-rw.postgres.svc` unverified.

## Access & Safety Guardrails

- Start read-only (`kubectl get/describe/logs`, `pveversion`, `qm list`, `pct list`, `zpool status`, `systemctl status`).
- Ask before changing VM/LXC configs, starting/stopping/rebooting guests or nodes, K8s/Proxmox upgrades, deleting PVCs/namespaces, DNS/Gateway/firewall/routing changes, or ZFS/NFS storage edits.
- Never print, commit, or store secrets (`group_vars/<group>/vault.yaml`, `*secret-values.yaml`, tokens, passwords).
- Every `home_base` K8s node must keep `chrony` active/synced (`configure-kubernetes-time-sync.yaml`, `task verify:kubernetes-time-sync`).
- Proxmox major upgrades: `docs/runbooks/proxmox-major-upgrade.md` via `task proxmox:major:{preflight,upgrade,resume,reboot,verify}` with exact `PVE_HOST` (`pve-1` or `pve-2`), one host/phase at a time.

## Context Loading

- `docs/memory/current-work.md` (active work), `docs/architecture.md`, `docs/deployment.md`, `docs/database.md`, `docs/memory/decisions.md`, `docs/memory/gotchas.md`, `docs/history/AGENTS-history-20261113.md`.

## Topology

### Kubernetes (`home-base` context, v1.35.9)

| Inventory Host | Node Name | IP | Proxmox Host | VMID | vCPU | RAM | Role / Labels |
|---|---|---|---|---:|---:|---:|---|
| `k8s-master` | `home-base1` | 192.168.1.135 | `pve-2` (`infra`) | 100 | 4 | 16 GB | control-plane + worker (`gpu_nodes`, `cluster-primary=true`) |
| `k8s-node-1` | `home-base2` | 192.168.1.136 | `pve-2` (`infra`) | 101 | 4 | 16 GB | worker (`gpu_nodes`) |
| `k8s-node-2` | `home-base3` | 192.168.1.137 | `pve-2` (`infra`) | 102 | 4 | 16 GB | worker |
| `k8s-node-3` | `home-base4` | 192.168.1.138 | `pve-2` (`infra`) | 103 | 4 | 16 GB | worker |
| `k8s-node-4` | `home-base5` | 192.168.1.139 | `pve-1` (`files`) | 101 | 4 | 8 GB | worker (`storage-host-adjacent=true`) |

- Retired VMs on `pve-1`: `home-base6` (`192.168.1.140`, VM 102) & `home-base7` (`192.168.1.141`, VM 103) — drained, removed, `onboot=0`.
- StorageClasses: default `ssd` (`openebs.io/local`, hostpath `/ssd`, `WaitForFirstConsumer`, reclaim `Delete`), `nvme` (`/nvme`). Do **not** add `nofail` to `/ssd` fstab mounts (prevents silent PV writes to root disk on boot race).
- CNI: `charts/flannel` (`v0.28.9`, namespace `kube-flannel`, pod CIDR `10.244.0.0/16`, VXLAN). Manage via `task flannel:*`.

### Proxmox (`ansible_user=root`, PVE 9 / Debian trixie)

| Host | Hostname | IP | Kernel / Notes |
|---|---|---|---|
| `pve-1` | `files` | 192.168.1.216 | Primary NAS (`home-storage` ZFS) + LXCs + `home-base5`. Pinned to kernel `6.8.12-15-pve` (`BUILD_EXCLUSIVE_KERNEL="^6\.8\."`) for GTX 1080 / NVIDIA `550.163.01` DKMS compatibility (7.0 breaks Pascal). |
| `pve-2` | `infra` | 192.168.1.28 | Beelink SER5 (`SER V2.0`, Ryzen 7 5700U, BIOS `SER5H516`, no IPMI). `pve-manager 9.2.21`, kernel `7.0.14-23-pve` (fallback `6.8.12-43-pve`), microcode `0x08608108`. |

- **pve-2 stability & hardening (2026-10-09):**
  - `/etc/sysctl.d/98-lockup-diagnostics.conf` sets `kernel.nmi_watchdog=1`, `kernel.hardlockup_panic=1`, `kernel.softlockup_panic=1`, `kernel.panic_on_oops=1`, `kernel.panic=30`. Crash dumps archive to `/var/lib/systemd/pstore/<epoch>/`. Do not re-add `tsc=unstable`.
  - Guest keepalive timer on `pve-2` (`install-pve-guest-keepalive.yaml` / `task pve:keepalive`): restarts stopped `onboot=1` VMs/CTs within ~2m. **Create `/etc/pve-guest-keepalive.disable` before intentional VM shutdowns.**
  - Out-of-cluster watchdog on `pve-1` (`install-homelab-watchdog.yaml` / `task install:homelab-watchdog`): checks `192.168.1.28` + `192.168.1.135:6443` every 1m, alerts via private `ntfy.sh` topic in `/etc/homelab-watchdog.env` (mode 0600).
  - Local etcd snapshot CronJob (`configs/etcd-snapshot-cronjob.yaml`, `kube-system/etcd-snapshot`): every 12h into `home-base1:/var/backups/etcd` (7d retention). Manual off-host extraction: `kubectl debug node/home-base1` + `cat /host/var/lib/etcd/<file>` (never `kubectl cp`, which truncates).
  - `prometheus-pve-exporter` pipx venv repair after Python upgrades: `runuser -u pve_exporter -- env PIPX_HOME=/var/lib/prometheus-pve-exporter/pipx PIPX_BIN_DIR=/var/lib/prometheus-pve-exporter/bin pipx reinstall-all --python /usr/bin/python3`.

### LXC Containers (on `pve-1`)

| CTID | Name / Host | IP | Purpose & Constraints |
|---:|---|---|---|
| 100 | `store.theramirez.casa` (`lxc-3`) | 192.168.1.218 | NAS Samba (`[home-cloud]` -> `/home-cloud`) + Plex (`:32400`, GPU passthrough, `task update:plex-media-server`). Keep in LXC. |
| 102 | `bonsai-lxc` | 192.168.1.9 | Ubuntu 24.04, GTX 1080 passthrough (`c 510:* rwm`, pinned `550.163.01-0ubuntu0.24.04.1` user libs, PyTorch `2.7.1+cu126`). Runs `kev.service` (`:8008`) + LAN proxy `kev-proxy.socket` (`http://192.168.1.9:8009/v1/systemone`) and `finbert.service`. Manage via `ssh root@192.168.1.216 pct exec 102`. Never expose publicly. |
| 104 | `downloads` (`lxc-4`) | 192.168.1.225 | Docker Compose (`/root/docker/docker-compose.yml`): `gluetun` (PIA OpenVPN TCP 443) + `qbittorrent` (`:8081` WebUI). Keep in LXC. |
| 105 | `sync.theramirez.casa` | 192.168.1.78 | Syncthing (`syncthing@syncthing.service`, `task update:syncthing-lxc`), 50G ZFS refquota (`subvol-105-disk-0`), node-exporter `:9100`. |
| 106 | `backblaze.theramirez.casa` | 192.168.1.79 | Privileged Docker LXC running `backblaze-wine` (`http://192.168.1.79:5800`, exporter `:9810`). Bind-mounts `/home-storage/home-cloud` -> `/drive_d` (`D:`) and `/home-storage/home-cloud/home-share` -> `/drive_d/home-share` (`E:`, separate ZFS dataset requires its own Wine drive letter; `Z:` removed). Tuned in `bzinfo.xml` (`num_backup_threads="40"`, `net_auto_throttle="false"` — stop `backblaze-wine` before editing). Manage via `root@192.168.1.216` + `pct exec 106` (`task backblaze:{setup-lxc,install-exporter,fix-ui}`). |

## Storage / NAS (`pve-1` + LXC 100)

- ZFS pool `home-storage` on `pve-1`: 4x 8TB Seagate `ST8000VN004` in `raidz2-0` (~29.1T raw), monitored by `zfs_exporter` (`:9134`, `0=ONLINE`).
- Datasets: `/home-storage/home-cloud` (root) and child dataset `/home-storage/home-cloud/home-share` (shared `Videos`, `Photos`, owned `100000:101001`) + `/home-storage/home-cloud/rramirez`.
- NFS server runs on `pve-1`: exports `Videos` (rw) and `Photos` (ro) to `192.168.1.0/24` (`no_root_squash`).

## Networking / Ingress / Auth

- MetalLB (`metallb-system`, pool `192.168.1.80-192.168.1.85`): nginx `192.168.1.80`, Postgres `192.168.1.81`, Istio `192.168.1.82`, **kgateway `home` `192.168.1.83`**.
- Gateway `kgateway-system/home` serves `*.theramirez.casa` (`theramirez-casa-wildcard-tls`) and `dipradar.io` / `*.dipradar.io` via `cert-manager` (`letsencrypt-cloudflare`) and `external-dns` (`theramirez.casa`, `homeapps.io`, `dipradar.io`, `upsert-only`).
- External LAN routes (`charts/ingress`, release `home-endpoints`): `store.theramirez.casa` -> `192.168.1.216:8006`, `homeflix.theramirez.casa` -> `192.168.1.218:32400`.

## Monitoring & Upgrade Visibility (`monitoring` namespace)

- `charts/prometheus-stack` (Grafana `13.1.0` at `https://grafana.theramirez.casa`, Prometheus 30d retention at `https://prometheus.theramirez.casa`, Alertmanager routes `notify="ntfy"` to `http://ntfy.ntfy.svc.cluster.local/rafael-alerts?template=alertmanager`).
  - Scrape targets: `node-external` (`9100`), `pve` (`9221`), `zfs-exporter` (`9134`), `backblaze-personal` (`192.168.1.79:9810`).
  - Grafana image repo must be `grafana/grafana` (never prefix `docker.io/`).
- `charts/version-checker` (`monitoring`): exposes `version_checker_is_latest_version`; alert `VersionCheckerImageLookupFailures` excludes `registry.theramirez.casa/.*`.
- On-demand upgrade check: `task check:cluster-upgrades` (`nova find --helm --show-old` + `--containers`).

## Platform & Application Releases in Kubernetes

- **Hindsight (`charts/hindsight`, ns `hindsight`):** Agent memory service (`1.2.0`), LAN/VPN HTTPRoutes `hindsight.theramirez.casa`, `lb-mem.theramirez.casa`, `arrive-mem.theramirez.casa` (`300s` timeouts). Uses flashrank `ms-marco-TinyBERT-L-2-v2` (100 candidates). Secret `hindsight-llm` out-of-band.
- **Trilium Agent Memory (`ai-memory`, release `agent-memory`):** Repo `~/homespace/agent-memory-gateway`. Hosts `trilium.theramirez.casa` (`triliumnext/trilium:v0.102.2`, `enableServiceLinks: false`) & `agent-memory.theramirez.casa` (LAN/VPN-only, never Cloudflare Tunnel).
- **Home Assistant (`charts/home-assistant`, ns `home-assistant`, release `home-assit`):** `2026.7.1` at `https://homeassistant.theramirez.casa`.
- **Mealie (`charts/mealie`, ns `mealie`, release `mealie`):** `ghcr.io/mealie-recipes/mealie:v3.20.1`, 10Gi `ssd` PVC, `https://mealie.theramirez.casa` (signup disabled).
- **Mattermost (`charts/mattermost`, ns `mattermost`, release `mattermost`):** `mattermost/mattermost-team-edition:12.1.0` + dedicated `postgres:17-alpine` StatefulSet (`mattermost-postgres`), 10Gi `ssd` PVCs for app + DB, auto-generated `mattermost-postgres` Secret (`helm.sh/resource-policy: keep`), `https://mattermost.theramirez.casa` on kgateway `home` HTTPS listener with `0s` timeouts for WebSockets. LAN/VPN-only; never expose via Cloudflare Tunnel without review.
- **ntfy (`charts/ntfy`, ns `ntfy`, release `ntfy`):** `binwiederhier/ntfy:v2.25.0`, 1Gi `ssd` PVC, `https://ntfy.theramirez.casa` (HTTPRoute timeout `0s` required for streaming).
- **Media Automation (`media-automation`):** `radarr`, `sonarr`, `prowlarr`, `seerr` (`https://request.theramirez.casa`, pinned `v3.4.1`), `decluttarr`, `recyclarr`. Mounts `192.168.1.216:/home-storage/home-cloud/home-share/Videos` at `/mnt/Videos` (`PUID=100000/PGID=101001`). Prowlarr Full Sync owns Sonarr `minimumSeeders`.
- **Plane CE Pilot (`charts/plane`, ns/release `plane`):** Disposable evaluation pilot (`v1.3.1` digest-pinned, SeaweedFS `4.39`), `https://plane.theramirez.casa` + `https://s3.theramirez.casa`. LAN/VPN-only, **no real data** until off-cluster backup/restore is built and tested (`task plane:{lint,render,dry-run,upgrade}`).
- **Paperclip Pilot (`charts/paperclip`, ns/release `paperclip-personal`):** Revision 2 post-claim live at `https://paperclip.theramirez.casa` (`personal` enabled; `work` & `logicbroker` dormant). LAN/VPN-only, **no real data** until off-cluster backup/restore is built and tested. See `charts/paperclip/AGENTS.md`.
