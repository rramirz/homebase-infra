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
| `update-plex.yaml` | `plex` | Download and install the latest Plex Media Server `.deb` package. |
| `disable-swap.yaml` | `bootstrap` | Deploy the swapoff systemd service to persistently disable swap. Already included in `add-k8s-node.yaml`. |
| `setup-iscsi.yaml` | `home_base` | Install and configure iSCSI initiators for shared storage. |
| `setup-gpu.yaml` | `gpu_nodes` | Install AMD GPU drivers and VAAPI video acceleration packages. |
| `node-exporters.yaml` | `servers` | Install and configure Prometheus Node Exporter on all servers. |

## Usage

```bash
# Update all OS packages on K8s nodes
ansible-playbook update-homebase.yaml

# Upgrade Kubernetes (interactive version prompt)
ansible-playbook update-k8s.yaml

# Update Plex
ansible-playbook update-plex.yaml

# Bootstrap a new K8s node (add host to [bootstrap] group first)
ansible-playbook add-k8s-node.yaml

# Install node exporters on all servers
ansible-playbook node-exporters.yaml
```

## Inventory Groups

| Group | Description |
|---|---|
| `home_base` | All K8s nodes (master + workers) |
| `k8s_master` | K8s control plane node |
| `k8s_nodes` | K8s worker nodes |
| `lxc` | LXC containers |
| `gpu_nodes` | Nodes with AMD GPUs |
| `plex` | Plex Media Server host |
| `bootstrap` | Temporary group for new K8s nodes being set up |
| `servers` | All servers (home_base + lxc) — used for monitoring |
