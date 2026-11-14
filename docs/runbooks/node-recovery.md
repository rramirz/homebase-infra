# Out-of-cluster node recovery

## Ownership and operating model

Runs on `pve-1` (`192.168.1.216`, hostname `files`), outside the Kubernetes failure domain on `pve-2`. Source: `scripts/node-recovery/`.

A systemd timer checks all five inventoried Kubernetes nodes every minute using a dedicated read-only Kubernetes identity. Healthy checks do not invoke a language model. Ready=False or Ready=Unknown must persist for at least 180 seconds and three failed samples before a recovery workflow triggers. First-boot/recently rebooted guests receive a 10-minute boot grace period for mutations.

AI recovery is **enabled as of November 10, 2026**, using OpenCode's paid `jev-1.13` structured decision model after Rafael authorized the cheapest usable model. Provider connectivity and read-only decision fixtures passed. All five nodes were Ready at activation; no real incident repair or reboot has been exercised. Earlier disabled/free-model attempts are retained in the verification history below.

While disabled, sustained failures still collect restricted SSH diagnostics, save an incident on pve-1, and notify the existing out-of-cluster ntfy channel. No service restarts or reboots execute.

## Allowed recovery behavior after explicit enablement

The provider endpoint is `https://opencode.ai/inference/systemone/v1/systemone`, with model `jev-1.13`. OpenCode's official pricing page listed $0.042 per million input tokens and no output charge on November 10, 2026, the cheapest listed paid option. After free API requests remained quota-blocked, Rafael authorized the cheapest paid option. There is no automatic model fallback. Provider errors fail closed.

Jev is an adviser, not a shell agent: one typed `choice` question selects from a deterministic action allowlist. It receives concise status facts, failure duration, previous repair, and allowed actions; disk/memory command output is not sent to the model. The response must identify the configured model, a permitted choice, and a finite numeric confidence in [0, 1]. Confidence below 0.8 escalates; this is a conservative decision gate, not a calibrated guarantee. Neither model output nor diagnostics can introduce commands, targets, or arguments.

- One isolated NotReady node only. Multiple unhealthy nodes escalate as a possible shared infrastructure failure.
- Collect fixed diagnostics: identity, uptime, kubelet/containerd status, memory, and root filesystem usage. No application logs, environment variables, kubeconfigs, or secrets are sent to the model.
- First attempt: restart an inactive/failed containerd, or restart kubelet.
- If a worker is still NotReady after a prior service repair and a 15-minute cooldown: the adviser may choose a graceful guest reboot (`systemctl reboot`, scheduled 10 seconds later).
- A healthy recheck, cordoned node, pressure condition, boot grace period, or uncertain service state cancels/escalates rather than rebooting.
- Maximum three mutation attempts per node per rolling day; a reboot cannot repeat during the same unhealthy episode.
- Persist action intent before sending it. SSH ambiguity still consumes a budget slot.
- Recheck actual Kubernetes readiness after 45 seconds; this is not a guarantee that every reboot has completed. Subsequent monitor checks report recovery.
- Never reboot `home-base1` automatically. Never reboot/reset a Proxmox host, force-reset a VM, drain/delete a node, delete pods/data, change storage, upgrade packages, or execute model-generated shell.
- An unavailable Kubernetes API escalates without repairs. This workflow cannot recover a frozen Proxmox host. Existing `homelab-watchdog` remains independent and unchanged.

This is bounded recovery, not a fix for recurring pve-2 host instability.

## Credentials and privileges

On pve-1:

- `/etc/homelab-node-recovery/` is root-only; credentials and private key have mode 0600.
- `credentials.json` contains the OpenCode API key and a dedicated Kubernetes service-account token. No credential is placed in this repository.
- Kubernetes SA `kube-system/homelab-node-recovery`, ClusterRole/Binding `homelab-node-recovery`, Secret `kube-system/homelab-node-recovery-token` allow only get/list nodes. The long-lived token is used because the monitor lives outside Kubernetes; revoke by deleting its Secret/SA. It does not carry cluster-admin permissions.
- Dedicated SSH key, separate known_hosts, strict host-key checking, and `IdentitiesOnly`.

On each guest:

- `/usr/local/sbin/homebase-node-recovery-helper` is root-owned and only accepts exact action identifiers.
- Its authorized key is source-restricted to pve-1, forced-command restricted, and disallows forwarding/PTY/user rc.
- `/etc/sudoers.d/homebase-node-recovery` grants `rramirez` only the helper's five exact root invocations (diagnose, disable, and three repair actions). It does not grant arbitrary systemctl or a root shell.
- Helper independently enforces an enabled gate, fresh NotReady state using the node's own kubelet identity, a prior service repair before reboot, a 15-minute cooldown, a three-attempt daily budget, and at most one reboot per unhealthy episode. It rejects control-plane reboots independently of the adviser.
- `/etc/homebase-node-recovery-policy.json` is root-owned and initially disabled. The SSH key may disarm it but cannot arm it. Arming requires another explicitly authorized bootstrap deployment.

The approved temporary privileged installer mounts the node filesystem only during bootstrap. It adds the helper, exact sudo rules, and restricted key, preserving unrelated login keys. Its DaemonSet, ConfigMap, and namespace are deleted afterward. The deployed agent has no permission to create privileged workloads.

## Commands

Deploy/redeploy from the workstation, after reviewing changes and obtaining bootstrap authorization:

    python3 scripts/node-recovery/deploy.py

The deployment deliberately refuses to overwrite an enabled AI configuration or a pre-existing bootstrap namespace. It preserves incident state. It verifies exact remote file hashes, five completed installers, five real SSH diagnostic calls, Kubernetes node reads, and systemd timer/service behavior.

Tests, with scratch paths outside the repository:

    TMPDIR="$HOME/.hermes/cache/scratch" python3 -m unittest discover -s scripts/node-recovery -p 'test_*.py' -v

Inspect from the workstation:

    ssh root@192.168.1.216 'systemctl status homelab-node-recovery.timer --no-pager'
    ssh root@192.168.1.216 'journalctl -u homelab-node-recovery.service -n 40 --no-pager'
    ssh root@192.168.1.216 '/usr/local/sbin/homelab-node-recovery --diagnose-all'

Verify the current provider/model with a real non-mutating call (consumes a small amount of API credit):

    ssh root@192.168.1.216 '/usr/local/sbin/homelab-node-recovery --probe-model'

Enable AI only after a successful probe, review of this policy, and explicit authorization to rerun the temporary privileged installer. From the workstation/repository root:

    python3 scripts/node-recovery/deploy.py --enable-ai

This first probes the provider while the agent stays disabled, arms the guest helpers through the installer, then probes again before enabling the agent. A failure leaves the agent disabled. The host-local `--enable-ai` command cannot arm disabled guest helpers; it requires all five helpers already armed. No model fallback or infinite retry.

Disable AI while leaving monitoring on:

    ssh root@192.168.1.216 '/usr/local/sbin/homelab-node-recovery --disable-ai'

This disables the local agent first, then disarms the guest helpers. Inspect its output if a guest is unreachable; the local monitor still cannot issue repairs while disabled.

For maintenance, stop the timer and wait for/inspect any in-flight service:

    ssh root@192.168.1.216 'systemctl stop homelab-node-recovery.timer; systemctl status homelab-node-recovery.service --no-pager'

Do not stop an in-flight workflow assuming a scheduled guest reboot is cancelled. Check `homebase-recovery-reboot.timer` on that guest. Only after confirming that state should maintenance proceed.

Incidents and rate-limit state: `/var/lib/homelab-node-recovery/` on pve-1, root-only. Do not delete state to bypass cooldowns. Incidents contain minimal host diagnostics and the model's reason; review before sharing.

## Durable recovery history

The persistent event history is `/var/lib/homelab-node-recovery/recovery.jsonl` on pve-1. Each entry has a UTC timestamp, event, node/action when applicable, and an incident-file link. It records failure detection, repair decisions, durable repair intent, acknowledged/uncertain execution, readiness verification, eventual recovery, and escalation/errors. Routine healthy polls remain in the systemd journal and are not appended to this history.

Read it from the workstation:

    ssh root@192.168.1.216 'tail -n 30 /var/lib/homelab-node-recovery/recovery.jsonl'

Follow service activity:

    ssh root@192.168.1.216 'journalctl -fu homelab-node-recovery.service'

The history is mode 0600. `/etc/logrotate.d/homelab-node-recovery` rotates weekly or when over 1 MiB at the next logrotate run, keeps 12 archives, and compresses older archives. Writes are append-only through the application, serialized, flushed, and fsynced. Failure to persist the pre-action history aborts repair execution.

`repair_requested` is intent, not proof that a command succeeded. `repair_verified` distinguishes acknowledged execution from failed/unknown execution and includes the immediate Ready result. If readiness returns later, `node_recovered` links the most recent attempted repair, and its incident file gains `recovery_observed_at` and `verified_ready=true`. This records temporal association, not proof that automation caused recovery.

## Verification boundaries

Unit tests use explicitly mocked fixtures; they are not evidence of real remediation. Live installation tests are read-only. No node is intentionally made NotReady and no service is restarted/rebooted as an installation test. The first authorized real repair must be separately verified in the incident record and Kubernetes readiness.

### Installation verification — November 9, 2026

- Installed on pve-1 and verified `homelab-node-recovery.timer` enabled/active. Real systemd service runs reported five Ready nodes with `ai_enabled=false`, Result=success, ExecMainStatus=0.
- All five restricted SSH helpers returned their expected node identity, Ready=True, active kubelet/containerd, and `enabled=false`. Diagnostics also succeeded under systemd filesystem/home protection.
- A real `reboot` request through the recovery key was rejected with `helper recovery is disabled`. A real `id` shell request was rejected with `exact recovery action required`. No guest service restart or reboot was executed.
- A real model probe from pve-1 returned HTTP 402; agent configuration remained disabled. Successful DeepSeek inference remains unverified.
- Temporary installer completed on exactly the five expected nodes. Its namespace was deleted and deletion was awaited before finishing.
- All 21 unit tests passed, including privileged-boundary restrictions and reboot-history regression coverage. Tests were also run with ResourceWarning treated as an error. `git diff --check` passed.
- Credential directory mode was verified as 0700; API/token credential file and SSH private key were 0600. Existing unrelated repository changes and the original watchdog were not modified.

### Free-only provider update — November 10, 2026

Rafael requested free OpenCode models instead of consuming the newly added credits. Set the deployed adviser to `deepseek-v4.1-flash-free`, with no paid fallback. Probes from pve-1 for DeepSeek Free, GLM-5.3 Flash Free, and MiniMax M3.1 Flash Free all returned HTTP 429 `FreeUsageLimitError` / `Free usage limit reached`. The provider message says free models work in OpenCode; this workflow currently uses its direct API, not the OpenCode client. No alternative client route was installed or verified.

Verified the deployed file by read-back hash and model constant, ran the installed `--probe-model` (HTTP 429), and confirmed `ai_enabled=false`, a successful monitor service, and an active timer reporting all five nodes Ready. All 23 unit tests passed, including free-only request selection and refusal to fall back after a free-quota error. Helpers remained disabled; no bootstrap rerun, repair, or reboot occurred.

### Cheapest paid model activation — November 10, 2026

- Rafael authorized the cheapest usable model after the free API quota rejection. Inspected OpenCode's current official model, endpoint, typed-question examples, and pricing tables; selected paid `jev-1.13` rather than a text-generation model.
- A real probe from pve-1 succeeded. Its reported charge was $0.00001407 for that one probe (not the total activation cost).
- Initial nested-JSON failed-kubelet evidence fell below the 0.8 threshold and safely escalated. Recovery was disarmed while investigating. Explicit concise status facts, duration, and prior-repair context resolved the ambiguity without lowering the threshold.
- Real API calls on four explicitly labeled, read-only fixtures passed: healthy -> escalate (1.000), failed kubelet -> restart-kubelet (0.960), failed containerd -> restart-containerd (0.960), failed prior service repair -> reboot (0.900). These were adviser decisions only; no fixture action was executed.
- All 25 unit tests passed, including Jev schema/choice validation, low-confidence escalation, invalid-confidence rejection, no fallback, and existing helper/cooldown/reboot safeguards.
- Reinstalled the approved temporary bootstrap with helpers enabled, verified all five helpers enabled and Ready, and enabled the pve-1 agent after a successful provider probe. Read back `model=jev-1.13`, `ai_enabled=true`, an enabled/active timer, and a successful actual service run reporting five Ready nodes.
- Awaited temporary installer namespace deletion. No node was deliberately failed, restarted, or rebooted; actual incident remediation remains unverified.

### Persistent history verification — November 10, 2026

Rafael accepted the bounded Jev workflow and prioritized low-cost automatic repair plus a durable record. Added timestamped history and later-readiness correlation without changing the repair allowlist or model. All 28 unit tests passed, including history permissions, intent-before-execution ordering, later recovery/incident updates, and refusal to repair when history persistence fails. Logrotate's debug check passed; rotation was not forced. A real monitor service run created a genuine `audit_initialized` entry, retained `ai_enabled=true`, and reported all five nodes Ready. No actual repair/reboot was performed, and no synthetic recovery was written into production history.
