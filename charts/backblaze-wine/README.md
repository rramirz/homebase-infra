# Backblaze Wine

Status: retired as an active Kubernetes workload on 2026-06-08. Backblaze Personal Backup detected the Kubernetes-mounted `D:` path as a network disk and reported `0` selected files for the NAS volume. Release `backblaze-wine` was uninstalled from namespace `backblaze`; `backblaze-wine-config` PVC was intentionally kept for rollback/state recovery. Current path is pve-1 LXC `106` with local bind mounts, managed by repo playbook `setup-backblaze-lxc.yaml` / task `backblaze:setup-lxc`.

Do not redeploy this chart as the normal NAS backup path. Keep it only for rollback/debug while the LXC migration is validated.

Current LXC path:

```bash
task backblaze:setup-lxc
ssh root@192.168.1.216 'pct exec 106 -- cp /root/backblaze-wine/backblaze-wine.env.example /etc/backblaze-wine.env'
# edit /etc/backblaze-wine.env inside LXC 106 and set VNC_PASSWORD out of band
ssh root@192.168.1.216 'pct exec 106 -- systemctl enable --now backblaze-wine'
```

Before trusting the LXC client, verify Backblaze sees `D:` as a local/fixed disk and reports non-zero selected files for `D:`. Keep the Windows/Dokan backup path active until that validation and a restore test pass.

Historical purpose: ran Backblaze Personal Backup under Wine in Kubernetes so the NAS could back up to Backblaze without keeping a Windows PC online.

## Reliability Defaults

- Single replica with `Recreate`; never run two Backblaze clients against one `/config`.
- `/config` PVC has `helm.sh/resource-policy: keep`; it stores the Wine prefix and Backblaze machine identity.
- NAS mount is read-only at `/drive_d` from `192.168.1.216:/home-storage`; overlayfs masking redirects Backblaze's `.bzvol` writes into `/config`.
- Pod is pinned to `node-role.theramirez.casa/storage-host-adjacent=true`, currently `home-base5`, because that VM is on `pve-1` with the NAS storage layer.
- Namespace is privileged because overlayfs masking needs `SYS_ADMIN` plus unconfined seccomp and AppArmor.
- Backblaze runs as `100000:101001` to match the NAS ownership observed on `/home-storage/home-cloud/home-share`, `Videos`, and `Photos`.

## Prereqs

Prepare the namespace and create the VNC secret out of band:

```bash
task backblaze:prepare-namespace
kubectl -n backblaze create secret generic backblaze-wine-vnc --from-literal=password='<password>'
```

Verify the NAS path is exported by NFS from `pve-1`:

```bash
ssh -o BatchMode=yes -o ConnectTimeout=8 root@192.168.1.216 'exportfs -v'
```

Default chart path is `/home-storage`. Manage the read-only export with:

```bash
task backblaze:configure-nfs-export
```

Videos and Photos live under `/drive_d/home-cloud/home-share/Videos` and `/drive_d/home-cloud/home-share/Photos` inside the container.

## Render And Deploy — Retired Rollback/Debug Only

```bash
helm lint charts/backblaze-wine
helm template backblaze-wine charts/backblaze-wine -n backblaze -f charts/backblaze-wine/values.yaml >/tmp/backblaze-wine-render.yaml
kubectl apply --dry-run=server -f /tmp/backblaze-wine-render.yaml
task backblaze:upgrade
```

Open `https://backblaze.theramirez.casa`, enter VNC password, log into Backblaze, and select the `D:` drive. At minimum, verify `D:\home-cloud\home-share\Videos` and `D:\home-cloud\home-share\Photos` are selected.

## Cutover Guardrails

Do not shut down the Windows/Dokan backup path until the Kubernetes client has completed initial backup and stayed healthy long enough to trust. Backblaze Personal treats this container as a different computer unless backup state inheritance succeeds.

Runtime checks:

```bash
kubectl -n backblaze get pods,pvc,httproute
kubectl -n backblaze logs deploy/backblaze-wine
kubectl -n backblaze exec deploy/backblaze-wine -- mount | grep overlay
```
