# homebase-infra

Ansible playbooks for managing the homebase Kubernetes cluster, LXC containers, and related services.

## Prerequisites

- Ansible installed on your local machine
- SSH keys configured for all target hosts
- Inventory and config are set in `ansible.cfg` (defaults to `hosts` inventory, user `rramirez`)

## Playbooks

| Playbook | Targets | Description |
|---|---|---|
| `add-k8s-node.yaml` | `bootstrap` | Bootstrap a new Kubernetes node — disables swap, installs containerd, K8s packages, and the swapoff systemd service. Add the new host to the `[bootstrap]` group in `hosts` before running. |
| `update-k8s.yaml` | `home_base` | Upgrade Kubernetes to a selected version. Prompts for the version, upgrades master first, then rolls through workers one at a time. |
| `update-homebase.yaml` | `home_base` | Dist-upgrade all OS packages on K8s nodes and reboot if needed. |
| `update-servers.yaml` | `servers` | Dist-upgrade SSH-managed servers; defaults to no reboot unless `-e reboot_if_required=true`. |
| `update-proxmox.yaml` | `proxmox` | Serial Proxmox package maintenance; defaults to no reboot unless `-e reboot_if_required=true`. |
| `proxmox-major-upgrade.yaml` | one limited `proxmox` host | Guarded PVE 8/bookworm → PVE 9/trixie phase workflow. See [`docs/runbooks/proxmox-major-upgrade.md`](./docs/runbooks/proxmox-major-upgrade.md). |
| `update-plex.yaml` | `plex` | Download and install the latest Plex Media Server `.deb` package. |
| `update-syncthing-lxc.yaml` | `proxmox_managed_lxc` | Back up and update Syncthing inside the Proxmox-managed sync LXC via `pct exec`. |
| `disable-swap.yaml` | `bootstrap` | Deploy the swapoff systemd service to persistently disable swap. Already included in `add-k8s-node.yaml`. |
| `setup-iscsi.yaml` | `home_base` | Install and configure iSCSI initiators for shared storage. |
| `setup-gpu.yaml` | `gpu_nodes` | Install AMD GPU drivers and VAAPI video acceleration packages. |
| `node-exporters.yaml` | `servers` | Install and configure Prometheus Node Exporter on all servers. |
| `node-exporters-proxmox-lxc.yaml` | `proxmox_managed_lxc` | Install and configure Node Exporter inside LXCs managed via `pct exec` when direct SSH is not available. |
| `install-pve-exporter.yaml` | `proxmox` | Install `prometheus-pve-exporter` on Proxmox hypervisors, create the `monitoring@pve` API user + token, and run a systemd service on `:9221` for the central Prometheus (in k8s) to scrape. See [`MONITORING.md`](./MONITORING.md). |

## Usage

Preferred harness is [`Taskfile.yaml`](./Taskfile.yaml):

```bash
# See available operations
task --list

# Read-only health checks
task check:kubernetes-health
task check:proxmox-guests
task check:nas-and-sync-storage

# Node exporters
task install:node-exporters-ssh-servers
task install:node-exporters-proxmox-lxcs
task install:all-node-exporters

# OS / Kubernetes / Plex
task update:active-kubernetes-node-os
task update:ssh-managed-server-os-no-reboot
task update:proxmox-packages-no-reboot
task update:kubernetes-version
task update:plex-media-server
task update:syncthing-lxc

# Monitoring chart and dashboards
task monitoring:render-prometheus-stack
task monitoring:upgrade-prometheus-stack
task monitoring:apply-grafana-dashboards-and-rules
task monitoring:show-syncthing-storage
```

Major Proxmox upgrades are one-host, explicit-acknowledgement operations:

```bash
task proxmox:major:preflight PVE_HOST=pve-1 BACKUP_VERIFIED=true OUTAGE_ACKNOWLEDGED=true
task proxmox:major:upgrade PVE_HOST=pve-1 BACKUP_VERIFIED=true OUTAGE_ACKNOWLEDGED=true
task proxmox:major:resume PVE_HOST=pve-1 BACKUP_VERIFIED=true OUTAGE_ACKNOWLEDGED=true RESUME_CONFIRMED=true
task proxmox:major:reboot PVE_HOST=pve-1 OUTAGE_ACKNOWLEDGED=true REBOOT_CONFIRMED=true
task proxmox:major:verify PVE_HOST=pve-1
```

Read the [major-upgrade runbook](./docs/runbooks/proxmox-major-upgrade.md)
before running any phase. It supports PVE 8/bookworm → PVE 9/trixie only and
does not provide an in-place downgrade path.

Short aliases still work (`task k8s:status`, `task exporters:all`, etc.), but
the longer names are preferred because they describe scope and impact.

Raw Ansible commands still work:

```bash
# Update all OS packages on K8s nodes
ansible-playbook update-homebase.yaml

# Upgrade Kubernetes (interactive version prompt)
ansible-playbook update-k8s.yaml

# Update Plex
ansible-playbook update-plex.yaml

# Update Syncthing in the Proxmox-managed sync LXC
ansible-playbook update-syncthing-lxc.yaml

# Bootstrap a new K8s node (add host to [bootstrap] group first)
ansible-playbook add-k8s-node.yaml

# Install node exporters on all servers
ansible-playbook node-exporters.yaml

# Install node exporters in Proxmox-managed LXCs such as sync
ansible-playbook node-exporters-proxmox-lxc.yaml

# Install Proxmox exporter (see MONITORING.md for bootstrap flow)
ansible-playbook install-pve-exporter.yaml
```

For the full home-lab monitoring flow (Grafana + Prometheus in k8s scraping
Proxmox, LXCs, and K8s nodes) see [`MONITORING.md`](./MONITORING.md).

## Inventory Groups

| Group | Description |
|---|---|
| `home_base` | All K8s nodes (master + workers) |
| `k8s_master` | K8s control plane node |
| `k8s_nodes` | K8s worker nodes |
| `lxc` | LXC containers |
| `proxmox_managed_lxc` | LXCs managed through Proxmox `pct exec` instead of direct SSH |
| `proxmox` | Proxmox VE hypervisors (ansible_user=root) |
| `gpu_nodes` | Nodes with AMD GPUs |
| `plex` | Plex Media Server host |
| `bootstrap` | Temporary group for new K8s nodes being set up |
| `servers` | All servers (`home_base` + `lxc` + `proxmox`) — used for monitoring |

## Secrets

This repo does **not** use `ansible-vault`. Group-level secrets live in
`group_vars/<group>/vault.yaml` and are gitignored. Each group with secrets
ships a committed `vault.example.yaml` template — copy it to `vault.yaml` and
fill in real values locally.

Currently used by: `group_vars/proxmox/vault.yaml` (PVE API token).
