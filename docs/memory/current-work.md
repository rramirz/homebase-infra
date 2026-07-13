# Current Work

## Active Goal

Improve OpenCode context efficiency and model routing.

## Current Status

- OpenCode global routing moved toward cheap orchestrator + specialized agents.
- Fable policy changed to conscious opt-in, not default.
- Repo memory scaffold added for on-demand context docs.
- 2026-06-13: added and applied `dipradar.io` support for Gateway/cert-manager/external-dns. Same Cloudflare API token is reused; no new secret required. Applied `ClusterIssuer` `letsencrypt-cloudflare`; upgraded Helm releases `kgateway` to revision 7 and `external-dns` to revision 14. Gateway `home` now has HTTPS listeners for `dipradar.io` and `*.dipradar.io` with cert refs `dipradar-io-tls` and `dipradar-io-wildcard-tls`. Both certs are Ready/valid. ExternalDNS domain filters now include `theramirez.casa`, `homeapps.io`, and `dipradar.io`. Cert-manager ClusterIssuer solver is scoped to those DNS zones.
- 2026-06-29: upgraded Helm release `my-n8n` in namespace `8n8` to latest stable n8n version `2.27.5`. Local chart [Chart.yaml](file:///Users/rafaelramirez/homespace/homebase-infra/charts/n8n/Chart.yaml) appVersion updated to `2.27.5` and chart version bumped to `1.2.0`. Image tag in [values.yaml](file:///Users/rafaelramirez/homespace/homebase-infra/charts/n8n/values.yaml) was updated to `2.27.5`. Rollout was successful, and SQLite database migration completed cleanly on startup.
- 2026-07-05: confirmed Home Assistant latest upstream release is `2026.7.1` and applied local chart [charts/home-assistant](file:///Users/rafaelramirez/homespace/homebase-infra/charts/home-assistant) as Helm release `home-assit` revision 20 in namespace `home-assistant`. Chart version is `0.2.2`; appVersion and image tag are `2026.7.1`. Rollout succeeded and `https://homeassistant.theramirez.casa` returned HTTP 200.
- 2026-07-05: installed Mealie recipe manager with local chart [charts/mealie](file:///Users/rafaelramirez/homespace/homebase-infra/charts/mealie) as Helm release `mealie` revision 1 in namespace `mealie`. Image is `ghcr.io/mealie-recipes/mealie:v3.20.1`; storage is SQLite on PVC `mealie-data` (`ssd`, 10Gi, mounted at `/app/data`); signup disabled; hostname is `https://mealie.theramirez.casa` through kgateway HTTPRoute `mealie`. Rollout succeeded, route Accepted/ResolvedRefs=True, and `/api/app/about` plus `/` returned HTTP 200. First login uses upstream default `changeme@example.com` / `MyPassword`; change password after login.

## Next Steps

- Keep `AGENTS.md` stable; move future volatile notes into memory/topic docs.
- Use `scripts/update-agent-memory.sh` after meaningful repo context changes.

## Open Questions

- Whether to later compact the existing large `AGENTS.md` into topic docs.

## Last Updated

2026-07-05
