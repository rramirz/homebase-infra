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

## Plane CE Pilot (added 2026-07-14)

Local Helm chart at `charts/plane` deploys Plane CE `v1.3.1` (project-management app) as a disposable homelab pilot, release/namespace `plane`. Components: Plane app tier (backend/frontend/space/admin/live) + official Plane Caddy proxy (unmodified baked-in Caddyfile, owns internal path routing) + stateful dependencies Postgres 15.7, Valkey 7.2.11, RabbitMQ 3.13.6, and SeaweedFS 4.39 (S3-compatible object storage, **not MinIO**). All images pinned by digest in `charts/plane/image-lock.yaml`. Exposed through the existing kgateway `home` Gateway's HTTPS wildcard listener via two `HTTPRoute`s (`plane.theramirez.casa` for the app, `s3.theramirez.casa` for uploads direct to SeaweedFS) — no new Gateway, cert, or tunnel was added. Storage uses the default `ssd` StorageClass. As of 2026-09-01, live Helm revision 5 has all 12 Deployments Available, `plane-migrator-5` Complete, both HTTPRoutes Accepted/Resolved, and app HTTP 200; steady-state Deployment requests are 1500m CPU/2816Mi memory. The pilot remains disposable and LAN/VPN-only with no real data until backup and restore are tested; presigned upload and backup remain unverified. See `AGENTS.md` for details and deploy workflow (`task plane:lint`/`plane:render`/`plane:dry-run`/`plane:upgrade`).

## Family Retirement Planner (added 2026-08-13)

First-party Go/HTMX-style server-rendered app at `apps/family-retirement-planner`, with adjacent Helm chart and detailed `ARCHITECTURE.md`. It uses PostgreSQL and a pure deterministic decimal retirement engine. Chart is static-only and not deployed. Intended hostname is `family-retirement-planner.theramirez.casa`, LAN/VPN-only. Never add to Cloudflare Tunnel; do not store real financial data until encrypted off-cluster backup and restore are tested.

The app also contains a separate `pkg/estate` deterministic estate-inventory module. Estate records use dedicated household-scoped tables and typed repositories for beneficiaries, trusts, ownership, primary/contingent designations, insurance, document/contact/digital metadata, residuary shares, death scenarios, readiness, and immutable printable results. It deliberately does not calculate law, probate entitlement, intestacy, document validity, or taxes.
