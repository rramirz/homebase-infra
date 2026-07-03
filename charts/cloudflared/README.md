# cloudflared chart

Cloudflare Tunnel chart for homebase public routes.

## Current tunnel

- Tunnel ID: `251632cb-5abe-4397-8294-1e8e3f6c5b7e`
- Existing Helm release: `cloudflared` in namespace `ocean-sounds`.
- Account ID and tunnel secret already live in `values.yaml`; do not add or commit new secrets.
- Existing media routes stay unchanged.

## Townhome prep

`values.yaml` includes direct service routes:

- `townhome-test.theramirez.casa` -> `http://townhome-frontend.townhome.svc.cluster.local:80`
- `townhome.theramirez.casa` -> `http://townhome-frontend.townhome.svc.cluster.local:80`

The route targets frontend nginx only. nginx proxies `/api` to the backend service.

## Validate before changing cluster

Render only:

```sh
helm template cloudflared /Users/rafaelramirez/homespace/homebase-infra/charts/cloudflared
```

Review rendered ConfigMap ingress order before applying.

Check current release and logs only when intentionally validating cluster state:

```sh
helm status cloudflared -n ocean-sounds
kubectl logs -n ocean-sounds deploy/cloudflared --tail=50
```

## Apply when ready

1. Rotate townhome app admin/JWT secrets before public cutover.
2. In Cloudflare, create a public hostname/DNS route for `townhome-test.theramirez.casa` to tunnel `251632cb-5abe-4397-8294-1e8e3f6c5b7e`.
3. Upgrade chart:

   ```sh
   helm upgrade --install cloudflared /Users/rafaelramirez/homespace/homebase-infra/charts/cloudflared --namespace ocean-sounds
   ```

4. Validate `townhome-test.theramirez.casa` end to end.
5. Only after test passes, create/switch Cloudflare public hostname/DNS for `townhome.theramirez.casa`.

## Rollback order

1. Revert Cloudflare public hostname/DNS for `townhome.theramirez.casa` to the previous path.
2. If needed, revert Cloudflare public hostname/DNS for `townhome-test.theramirez.casa`.
3. Roll back the Helm release or remove the townhome ingress entries from values and run `helm upgrade`.
4. Keep existing HTTPRoute/kgateway path available as rollback until tunnel cutover is proven.
