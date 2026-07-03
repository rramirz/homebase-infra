# Home-lab monitoring

End-to-end Grafana/Prometheus setup for the home-lab. Everything below is
managed from this repo (`homebase-infra`) with the in-repo Helm chart at
`charts/prometheus-stack` (and dashboard ConfigMaps at
`charts/prometheus-stack/dashboards`).

## Topology

```
 Proxmox hypervisors  (pve-1 .216, pve-2 .28)
   ├─ prometheus-node-exporter      :9100   (OS/host metrics)
   └─ prometheus-pve-exporter       :9221   (VMs/LXCs/ZFS/cluster metrics)

  LXCs                (lxc-3 .218, lxc-4 .225, sync .78)
   └─ prometheus-node-exporter      :9100

 K8s nodes           (k8s-master .135, k8s-node-{1..4} .136-.139)
   └─ prometheus-node-exporter      :9100   (baremetal OS, not the k8s DaemonSet)

 Kubernetes (monitoring ns)          ← central stack
   ├─ kube-prometheus-stack  (Prometheus + Alertmanager + Grafana)
   ├─ node-exporter DaemonSet  (in-cluster view)
   ├─ kube-state-metrics
   └─ Grafana sidecar auto-loads ConfigMaps labelled grafana_dashboard=1

 Grafana  →  https://grafana.theramirez.casa   (nginx ingress, 192.168.1.80)
```

Prometheus inside the cluster scrapes the three host-level jobs via
`additionalScrapeConfigs` in
`charts/prometheus-stack/values.yaml`.

## One-time bootstrap

1. **Add hosts to inventory** — already done; see `hosts`. The
   `[servers:children]` group now includes `proxmox`, so
   `node-exporters.yaml` will cover PVE hosts too.

2. **SSH access to Proxmox** — the `[proxmox]` group uses `ansible_user=root`.
   Make sure your SSH key is in `/root/.ssh/authorized_keys` on each PVE node.

3. **Install node-exporter on every server**:
   ```bash
   ansible-playbook node-exporters.yaml
   ```

4. **Install pve-exporter on PVE hosts** (two-pass on first run):
   ```bash
   # First pass — creates monitoring@pve user + token, prints the secret.
   ansible-playbook install-pve-exporter.yaml
   ```
   Copy the printed token value into
   `group_vars/proxmox/vault.yaml` (gitignored) as
   `pve_api_token_secret`, then re-run:
   ```bash
   ansible-playbook install-pve-exporter.yaml
   ```
   The second pass writes `/etc/prometheus/pve.yml`, installs the systemd
   unit, and starts the exporter on `:9221`. Subsequent runs are no-ops.

5. **Sanity-check exporters** from your workstation:
   ```bash
   for ip in 192.168.1.216 192.168.1.28; do
     curl -s "http://$ip:9100/metrics"        | head -1   # node-exporter
     curl -s "http://$ip:9221/pve?target=$ip" | head -1   # pve-exporter
   done
   ```

6. **Roll out the Prometheus scrape config, dashboards, and rules**:
   ```bash
   cd charts/prometheus-stack
   helm upgrade --install prometheus-stack . \
       -n monitoring --create-namespace \
       -f values.yaml
   kubectl apply --server-side=true -n monitoring -f dashboards/
   kubectl apply               -n monitoring -f rules/
   ```
   (Server-side apply is required for `dashboards/`: the Node Exporter Full
   dashboard JSON is large enough that client-side apply blows past the
   256 KiB limit on the `kubectl.kubernetes.io/last-applied-configuration`
   annotation. `rules/` contains small manifests and works fine with
   client-side apply.)
   The Grafana sidecar picks up the dashboard ConfigMaps automatically
   (label `grafana_dashboard=1`).

7. **Verify targets** in Prometheus →
   `https://prometheus.theramirez.casa/targets` — you should see:
    - `node-external` (10 targets, grouped by `group=k8s|lxc|proxmox`)
   - `pve` (2 targets)
   - `zfs-exporter` (1 target, if ZFS exporter is running on pve-1)

8. **Open Grafana** → `https://grafana.theramirez.casa`.
   Important dashboards land in the `Homelab` folder:
   - *Homelab / Essentials* — low-noise health and capacity overview
   - *Homelab / Storage Overview* — NAS/ZFS/storage detail
   - *Home Kubernetes / Cluster Overview* — Kubernetes health
   - *Proxmox via Prometheus* — cluster/guest metrics from pve-exporter

## NAS / storage monitoring

The host `pve-1` (`192.168.1.216`, hostname `files`) is simultaneously the
primary Proxmox node and the NAS. It backs a 29 TiB ZFS pool
(`home-storage`) and NFS-exports `Videos` to `192.168.1.0/24`. No dedicated
NAS appliance exists — the "NAS" is Proxmox ZFS.

Storage coverage comes from three overlapping exporters, all running on
`pve-1`:

| Exporter | Port | Key metrics | Source of truth for |
|---|---|---|---|
| `prometheus-node-exporter` | 9100 | `node_filesystem_*` | `df`-style free/used per mount |
| `prometheus-pve-exporter` | 9221 | `pve_disk_size_bytes`, `pve_disk_usage_bytes`, `pve_storage_info` | Proxmox-visible storage backends |
| `pdf/zfs_exporter` | 9134 | `zfs_pool_health`, `zfs_pool_{free,allocated}_bytes`, `zfs_pool_fragmentation_ratio`, `zfs_dataset_used_bytes`, `zfs_dataset_available_bytes` | ZFS internals (pool state, per-dataset usage) |

> The ZFS exporter unit was installed outside this ansible repo. If `pve-1`
> is rebuilt, re-install `zfs_exporter` ([pdf/zfs_exporter releases](https://github.com/pdf/zfs_exporter/releases))
> and bind it to `0.0.0.0:9134`. TODO: codify as `install-zfs-exporter.yaml`
> once this is stable.

### Dashboards

- **Homelab / Storage Overview** — pool health, used %, free space,
  fragmentation, per-dataset bargauge, per-filesystem free %, and PVE
  storage view. Source:
  `charts/prometheus-stack/dashboards/homelab-storage.yaml`.
- **Homelab / Essentials** — low-noise home overview: K8s node readiness,
  NAS ZFS health/capacity, Syncthing LXC storage pressure, and filesystems
  below 20% free. Source:
  `charts/prometheus-stack/dashboards/homelab-essentials.yaml`.
- **Node Exporter Full** — generic per-host OS metrics; retained in repo but dashboard auto-import is disabled because it is noisy for normal home ops.
- **Proxmox via Prometheus** — cluster/guest metrics from pve-exporter.

### Alerts

Defined in `charts/prometheus-stack/rules/homelab-storage.yaml`.
Three groups, each landing in Alertmanager with `component=storage` or
`component=nas` labels:

| Group | Alert | Condition | For |
|---|---|---|---|
| `.zfs` | `ZpoolNotOnline` (critical) | `zfs_pool_health > 0` (anything other than ONLINE) | 10 m |
| `.zfs` | `ZpoolReadOnly` (warning) | `zfs_pool_readonly > 0` | 5 m |
| `.zfs` | `ZpoolLowFreeSpace` (warning) | pool <10% free | 30 m |
| `.zfs` | `ZpoolCriticallyLowFreeSpace` (critical) | pool <5% free | 15 m |
| `.zfs` | `ZpoolHighFragmentation` (info) | frag >80% | 6 h |
| `.zfs` | `ZfsExporterDown` (warning) | `up{job="zfs-exporter"}==0` | 10 m |
| `.filesystems` | `FilesystemAlmostFull` (warning) | any non-tmpfs mount on `node-external` below 10% | 30 m |
| `.filesystems` | `FilesystemCritical` (critical) | same, <3% | 10 m |
| `.nas` | `NasHostDown` (critical) | `up{job="node-external",instance="192.168.1.216"}==0` | 5 m |
| `.nas` | `NasPveExporterDown` (warning) | `up{job="pve",instance="192.168.1.216"}==0` | 10 m |
| `.sync` | `SyncStorageAlmostFull` (warning) | sync LXC rootfs <10% free | 30 m |
| `.sync` | `SyncStorageCritical` (critical) | sync LXC rootfs <5% free | 10 m |

`zfs_pool_health` value mapping used by the dashboard and by the
`ZpoolNotOnline` description: `0=ONLINE, 1=DEGRADED, 2=FAULTED,
3=OFFLINE, 4=REMOVED, 5=UNAVAIL, 6=SUSPENDED`.

### Useful ad-hoc queries

```promql
# Free TiB on the NAS ZFS pool
zfs_pool_free_bytes{pool="home-storage"} / 1024^4

# Used % per ZFS pool
100 * zfs_pool_allocated_bytes
    / (zfs_pool_allocated_bytes + zfs_pool_free_bytes)

# Used % per PVE storage backend (infra + files)
100 * pve_disk_usage_bytes{id=~"storage/.+"}
    / (pve_disk_size_bytes{id=~"storage/.+"} > 0)

# Filesystem free % on the NAS host
100 * node_filesystem_avail_bytes{instance="192.168.1.216"}
    / node_filesystem_size_bytes{instance="192.168.1.216"}
```

### Silencing and known-noise notes

- The LXC at `192.168.1.218` imports the same `home-storage` pool, so
  `zfs_pool_health` appears on two `instance` labels. The
  `ZpoolNotOnline` query uses `max by (instance, pool)` so both surface;
  silence either via Alertmanager if they're too chatty.
- When rebuilding the NAS host, silence `NasHostDown`, `NasPveExporterDown`,
  and `Zfs*` in Alertmanager for the maintenance window rather than muting
  the rule files.

## Day-2 operations

- **Add a new host** (LXC/K8s/PVE): put it in the right group in `hosts`,
  re-run `node-exporters.yaml`, then add its `<ip>:9100` to the
  `node-external` static config in
  `charts/prometheus-stack/values.yaml` and `helm upgrade`.
- **Re-enable retired K8s workers** (`k8s-node-5/6`): move them from
  `[retired_k8s_nodes]` back to `[home_base]` and `[k8s_nodes]`, start/rejoin
  the VMs, then add `192.168.1.140:9100` / `192.168.1.141:9100` back to
  `node-external` only after node-exporter is reachable.
- **Rotate PVE token**: on any PVE node, run
  `pveum user token remove monitoring@pve prom && pveum user token add monitoring@pve prom --privsep 0`,
  update `group_vars/proxmox/vault.yaml`, re-run
  `install-pve-exporter.yaml`.
- **Add a dashboard**: drop a `*.yaml` `ConfigMap` (label
  `grafana_dashboard: "1"`, namespace `monitoring`) under
  `charts/prometheus-stack/dashboards/` and
  `kubectl apply -f` it. Reference the Prometheus datasource via the
  `${datasource}` template variable
  (`templating.list: [{name: datasource, type: datasource, query: prometheus, current: {text: Prometheus, value: Prometheus}}]`),
  not a hardcoded UID or Grafana's `__inputs`/`DS_PROMETHEUS` export convention
  — that's what the sidecar actually resolves here.
- **Townhome tunnel dashboard**: `charts/prometheus-stack/dashboards/townhome-traffic.yaml`
  intentionally combines two sources: blackbox `job="blackbox-townhome"` for
  public Cloudflare Tunnel edge availability/TLS/latency, and
  `townhome_http_*` nginx-log-exporter metrics for origin-side per-host request
  rate/status/latency/bytes. cloudflared itself is not scraped here and would
  only provide shared tunnel-level metrics, not per-host traffic.
- **Retention**: Prometheus keeps `120h` (5 days) at
  `prometheus.prometheusSpec.retention` in `values.yaml`. Bump if needed.
- **Adding a ServiceMonitor/PodMonitor for your own app** (2026-07-02,
  discovered wiring up `townhome-listing` traffic metrics): the live
  Prometheus CR's `serviceMonitorSelector`/`podMonitorSelector` are
  `{matchLabels: {release: prometheus-stack}}` — **not** the empty `{}`
  shown in `values.yaml`'s `prometheus.prometheusSpec.serviceMonitorSelector`
  (that key is a stray/unused default in this 182K-line values file; the
  real selector lives elsewhere and was only confirmed by reading the live
  `Prometheus` CR with `kubectl get prometheus -n monitoring -o
  jsonpath='{.items[0].spec.serviceMonitorSelector}'`). A ServiceMonitor
  missing the `release: prometheus-stack` label is silently never scraped —
  it exists as a resource, the operator even renders it into the generated
  scrape config, but Prometheus's own service-discovery relabeling drops it
  before it becomes a target. `namespaceSelector` IS empty (`{}`), so the
  ServiceMonitor/PodMonitor can live in the app's own namespace — no need to
  centralize it here, matching this repo's "first-party app config stays
  with the app" convention (see `marketlens-traders` PodMonitor for a
  namespace-based pattern, though notably even that one lives in
  `monitoring` rather than the trader namespaces — either works as long as
  `namespaceSelector` stays empty).
  **Second gotcha**: the ServiceMonitor's `spec.selector.matchLabels` filters
  on the target **Service's own `metadata.labels`**, not the Service's
  `spec.selector` (which only picks which Pods the Service load-balances to)
  and not the Pod's labels either. A Service with a `selector` but no
  `labels` of its own will never match any ServiceMonitor selector — add
  `metadata.labels` to the Service explicitly, even if it looks redundant
  next to `spec.selector`.
  **Verify a new target actually works** (don't trust the resource
  existing — `immich-server`'s ServiceMonitor in the `immich` namespace has
  been silently unscraped this whole time for the same reason):
  ```bash
  kubectl port-forward -n monitoring svc/prometheus-stack-kube-prom-prometheus 19090:9090
  curl -s http://localhost:19090/api/v1/targets | jq '.data.activeTargets[] | select(.scrapePool | contains("<name>"))'
  ```

## File layout (this feature)

```
homebase-infra/
├── hosts                              # [proxmox] group added, stale lxc-1/2 removed
├── install-pve-exporter.yaml          # new playbook
├── node-exporters.yaml                # now also hits [proxmox] via servers:children
├── group_vars/
│   └── proxmox/
│       ├── main.yaml                  # non-secret (user/role/port names)
│       ├── vault.example.yaml         # committed template
│       └── vault.yaml                 # gitignored, holds pve_api_token_secret
├── .gitignore                         # excludes vault.yaml
├── README.md
└── MONITORING.md                      # this file

charts/prometheus-stack/
├── values.yaml                        # additionalScrapeConfigs: node-external + pve jobs
├── dashboards/
│   ├── homelab-essentials.yaml        # low-noise home overview
│   ├── node-exporter-full.yaml        # grafana.com #1860, disabled/noisy by label
│   ├── proxmox-via-prometheus.yaml    # grafana.com #10347
│   └── homelab-storage.yaml           # custom ZFS/NAS overview
└── rules/
    └── homelab-storage.yaml           # PrometheusRule (ZFS + filesystem + NAS alerts)
```
