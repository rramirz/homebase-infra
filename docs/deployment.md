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
- Ask before destructive changes, reboots, PVC deletion, storage edits, or Gateway/DNS/firewall changes.

## CI/CD

- No generic CI/CD pipeline owned by this repo. Routine operations are run locally through Taskfile/Ansible/Helm.
