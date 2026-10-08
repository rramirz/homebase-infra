# Gotchas

## Kubernetes node clock skew and missing chrony

### Symptom

A Kubernetes node is reachable/alive but reports an implausibly old system
clock, while the RTC is current. Kubelet TLS/authentication and lease
freshness may fail even though the VM and OS are running.

### Cause

No time-synchronization daemon is installed or enabled, or the node clock has
drifted beyond the tolerance of Kubernetes certificates and API authentication.
Do not infer recovery from VM reachability alone.

### Fix

Use `configure-kubernetes-time-sync.yaml` through the serial, prompt-gated
Taskfile workflow. It installs/enables `chrony` and validates active, enabled,
tracking, and normal leap status. Do not hard-code an NTP pool in this repo.
If the RTC is first proven correct, `hwclock --hctosys --utc` is an allowed one-off repair; it is not a replacement for chrony.

### Checks

Run `task check:kubernetes-time-sync`, then
`task configure:kubernetes-time-sync-dry-run` (check mode previews apt and
skips service/runtime checks when chrony is absent); after an approved rollout
run `task verify:kubernetes-time-sync` and Kubernetes health checks. Abort on
failed chrony validation, unexpected package changes, or node readiness
regression.

### Incident reference

On 2026-09-10, the operator proved home-base5's RTC correct and ran
`hwclock --hctosys --utc` interactively. No kubelet restart was needed; the node
became Ready naturally and its lease renewed. Delayed local-PV workloads started
after Flannel recreated `/run/flannel/subnet.env`; all five nodes became Ready.
The persistent rollout completed on all five nodes. Passwordless SSH verification
showed chrony enabled/active, valid NTP Reference IDs, `Leap status: Normal`, and
system offsets under 5 ms. Remaining non-Running pods were pre-existing and not
caused by this incident: ai-memory `qualtrim-sync` missing secret/
`CreateContainerConfigError`, media-automation `decluttarr` `ContainerCreating`,
and qualtrim-sync `bedrock-trader-paper` `Error`.

## Ansible `-b`/`sysctl` blockers for quick host reads on home_base nodes

### Symptom

`ansible home_base -b -m shell -a '...'` fails with `Missing sudo password`, even in an interactive terminal; adding `-e ansible_become=false` to skip it then fails with `sysctl: not found`.

### Cause

This inventory has no passwordless sudo configured for `home_base`/`k8s_nodes`, so any `-b`/`become` Ansible run blocks on an interactive BECOME password prompt. Separately, `sysctl` is not on the non-root `PATH` for the `rramirez` user on these Debian nodes (it's normally `/usr/sbin` or `/sbin`, root-only PATH by default).

### Fix

For read-only kernel/network flag checks that don't need root, skip Ansible/sudo entirely and read `/proc/sys/...` directly over a plain keyed SSH loop instead, e.g. `cat /proc/sys/net/bridge/bridge-nf-call-iptables` and `cat /proc/sys/net/ipv4/ip_forward` instead of `sysctl -n net.bridge...`. For image pre-pull without sudo/crictl, apply a short-lived Kubernetes DaemonSet with the target image(s) as init containers (any command is fine — the pull happens before the command runs) instead of `ansible ... -m command -a 'crictl pull ...'`.

### Related Files

- `hosts` (inventory)
- `ansible.cfg`

## OpenCode Model IDs

### Symptom

Background agents retry or fail with model-not-found errors.

### Cause

Stale or unavailable model IDs in OpenCode routing, especially old `openai/gpt-5.2`, old `anthropic/claude-opus-4-7`, or unavailable active `github-copilot/*` routes.

### Fix

Use verified active IDs: `openai/gpt-5.5`, `anthropic/claude-opus-4-8`, `anthropic/claude-sonnet-4-6`, `anthropic/claude-haiku-4-5`, `google/gemini-3.1-pro-preview`, and `google/gemini-3-flash-preview`.

### Related Files

- `/Users/rafaelramirez/.config/opencode/oh-my-openagent.json`

## Plane SeaweedFS bucket Job hangs

### Symptom

The fixed-name `plane-seaweedfs-bucket` Job reaches `activeDeadlineSeconds=300` and fails during the Plane revision-5 rollout, although the API reports `Bucket plane-uploads exists`.

### Cause

The Job uses `weed shell`, which requires SeaweedFS gRPC ports `19333`/`18888`; those ports are not exposed. The failed ephemeral Job was deleted after API confirmation, and the next Helm upgrade will recreate it until the template changes.

### Fix

Not implemented yet. Before the next upgrade, replace the shell-based Job with a validated idempotent Filer HTTP API call and verify HTTP status handling and repeated execution. Do not expose gRPC solely to support this Job. Resource right-sizing does not fix the earlier Redis `ECONNREFUSED` dependency outage.

### Related Files

- `charts/plane/templates/seaweedfs-bucket-job.yaml`
- `charts/plane/values.yaml`
- `docs/deployment.md`
