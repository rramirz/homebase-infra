# Current Work

## Active Goal

Improve OpenCode context efficiency and model routing.

## Current Status

- OpenCode global routing moved toward cheap orchestrator + specialized agents.
- Fable policy changed to conscious opt-in, not default.
- Repo memory scaffold added for on-demand context docs.
- 2026-06-13: added and applied `dipradar.io` support for Gateway/cert-manager/external-dns. Same Cloudflare API token is reused; no new secret required. Applied `ClusterIssuer` `letsencrypt-cloudflare`; upgraded Helm releases `kgateway` to revision 7 and `external-dns` to revision 14. Gateway `home` now has HTTPS listeners for `dipradar.io` and `*.dipradar.io` with cert refs `dipradar-io-tls` and `dipradar-io-wildcard-tls`. Both certs are Ready/valid. ExternalDNS domain filters now include `theramirez.casa`, `homeapps.io`, and `dipradar.io`. Cert-manager ClusterIssuer solver is scoped to those DNS zones.
- 2026-06-29: upgraded Helm release `my-n8n` in namespace `8n8` to latest stable n8n version `2.27.5`. Local chart [Chart.yaml](file:///Users/rafaelramirez/homespace/homebase-infra/charts/n8n/Chart.yaml) appVersion updated to `2.27.5` and chart version bumped to `1.2.0`. Image tag in [values.yaml](file:///Users/rafaelramirez/homespace/homebase-infra/charts/n8n/values.yaml) was updated to `2.27.5`. Rollout was successful, and SQLite database migration completed cleanly on startup.

## Next Steps

- Keep `AGENTS.md` stable; move future volatile notes into memory/topic docs.
- Use `scripts/update-agent-memory.sh` after meaningful repo context changes.

## Open Questions

- Whether to later compact the existing large `AGENTS.md` into topic docs.

## Last Updated

2026-06-29
