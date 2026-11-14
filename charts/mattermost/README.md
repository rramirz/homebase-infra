# mattermost

Mattermost Team Edition (`mattermost/mattermost-team-edition:12.1.0`) with dedicated PostgreSQL (`postgres:17-alpine`).
Namespace `mattermost`, Helm release `mattermost`.
Host (LAN/VPN only, kgateway `home` HTTPS listener): `https://mattermost.theramirez.casa`.

DB password secret `mattermost-postgres` is auto-generated on first install and preserved across upgrades/uninstalls via `lookup` + `helm.sh/resource-policy: keep`.

## Deploy / Upgrade

```sh
helm lint charts/mattermost
helm upgrade --install mattermost charts/mattermost -n mattermost -f charts/mattermost/values.yaml --wait
```
