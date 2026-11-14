# Charts Directory — Agent Context

`/Users/rafaelramirez/homespace/homebase-infra/charts` is the canonical Helm charts directory for third-party/umbrella homelab platform charts only. First-party app charts stay beside the app source code they deploy (`apps/<app>/` or standalone app repos).

## Conventions

- **Ingress:** Use `gateway.networking.k8s.io/v1` `HTTPRoute` attached to Gateway `home` in `kgateway-system` (`sectionName: https`, VIP `192.168.1.83`). Use `timeouts: { request: 0s, backendRequest: 0s }` for WebSocket/streaming apps (`ntfy`, `mattermost`).
- **Storage:** Default StorageClass is `ssd` (`openebs.io/local`, `WaitForFirstConsumer`, reclaim `Delete`). Keep StatefulSet `volumeClaimTemplates` or PVC `helm.sh/resource-policy: keep` for stateful data.
- **Databases:** Apps use a Helm chart and get their DB via the Crossplane `DB` claim on shared `home-db` (not CNPG `Cluster` CRDs) unless isolation is needed. Pattern: `docs/runbooks/app-database.md`.
- **Secrets:** Never commit real secrets. Either generate/preserve in-cluster secrets with `lookup` + `helm.sh/resource-policy: keep`, create out-of-band secrets documented in the chart's `README.md`, or use gitignored mode-0600 `*.secret-values.yaml` files.
- **Simple single-instance apps (`mealie`, `ntfy`, `mattermost`):** Plain local chart (`Chart.yaml`, `values.yaml`, `templates/{_helpers.tpl,namespace.yaml,pvc.yaml,deployment.yaml,service.yaml,httproute.yaml}`, optional `postgres.yaml`/`secret.yaml`).
- **Multi-profile isolated instances (`charts/paperclip`):** Disabled base `values.yaml` (`enabled: false`) + per-profile overlays (`values-personal.yaml`, `values-work.yaml`, `values-logicbroker.yaml`) + gitignored mode-0600 `*.secret-values.yaml`. Follow `charts/paperclip/AGENTS.md` when adding multi-profile charts.
