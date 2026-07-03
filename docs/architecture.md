# Architecture

## Overview

Homebase Infra manages Rafael's homelab with Ansible playbooks, Taskfile entrypoints, and in-repo Helm charts for shared platform services.

## Main Components

- Ansible inventory/playbooks: host updates, Kubernetes node setup, exporters, Plex/Syncthing/Backblaze LXC tasks.
- Taskfile: preferred command interface for routine checks, updates, monitoring, and Backblaze operations.
- Helm charts: monitoring stack, ingress endpoints, Backblaze experiment history, and shared homelab platform services.
- Kubernetes cluster: home-base cluster backed by Proxmox VMs.
- Proxmox/NAS/LXC layer: storage, media, downloads, sync, and Backblaze Personal runtime.

## Important Flows

- Infra diagnosis: read-only Kubernetes/Proxmox/Prometheus checks before changes.
- Monitoring updates: render/apply Helm chart values, dashboards, and rules under `charts/prometheus-stack`.
- Backblaze operations: manage LXC `106` with local bind mounts and exporter monitoring.

## Files to Know

- `AGENTS.md`: detailed homelab operating context and safety rails.
- `Taskfile.yaml`: routine command entrypoints.
- `hosts`: Ansible inventory.
- `charts/prometheus-stack/`: monitoring chart, dashboards, alerts.
- `setup-backblaze-lxc.yaml`: Backblaze LXC desired setup.
