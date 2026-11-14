#!/usr/bin/python3
"""Forced-SSH entry point. No caller-supplied shell or command arguments."""
import json
from datetime import datetime
import os
from pathlib import Path
import subprocess
import sys
import time

ACTIONS = ('diagnose', 'disable', 'restart-kubelet', 'restart-containerd', 'reboot')
POLICY = Path('/etc/homebase-node-recovery-policy.json')
STATE_DIR = Path('/var/lib/homebase-node-recovery-helper')


def run(argv):
    result = subprocess.run(argv, capture_output=True, text=True, timeout=20)
    return result.stdout.strip()[:2000], result.returncode


def save(path, data):
    temp = path.with_suffix('.tmp')
    fd = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, 'w') as f:
        json.dump(data, f)
        f.flush()
        os.fsync(f.fileno())
    temp.replace(path)


def own_node(hostname):
    result = subprocess.run(['/usr/bin/kubectl', '--kubeconfig=/etc/kubernetes/kubelet.conf',
                             '--request-timeout=8s', 'get', 'node', hostname, '-o', 'json'],
                            capture_output=True, text=True, timeout=12)
    if result.returncode:
        return None
    data = json.loads(result.stdout)
    if data['metadata']['name'] != hostname:
        raise ValueError('Node identity mismatch')
    return data


def validate_mutation(action, hostname, data, state, now):
    if action == 'reboot' and hostname == 'home-base1':
        raise ValueError('Control-plane reboots require a human')
    if data is None or data.get('spec', {}).get('unschedulable', False):
        raise ValueError('API unavailable or node cordoned')
    conditions = data.get('status', {}).get('conditions', [])
    ready = next((c for c in conditions if c['type'] == 'Ready'), None)
    if ready is None or ready['status'] == 'True':
        raise ValueError('Node is not confirmed NotReady')
    changed = datetime.fromisoformat(ready['lastTransitionTime'].replace('Z', '+00:00')).timestamp()
    if now - changed < 180 or float(Path('/proc/uptime').read_text().split()[0]) < 600:
        raise ValueError('Failure/boot grace period')
    if any(c['type'].endswith('Pressure') and c['status'] == 'True' for c in conditions):
        raise ValueError('Pressure condition requires human review')
    if state.get('episode_rebooted'):
        raise ValueError('Already rebooted during this unhealthy episode')
    if now - state.get('last_mutation', 0) < 900:
        raise ValueError('Mutation cooldown (15 minutes)')
    attempts = [t for t in state.get('mutations', []) if now - t < 86400]
    if len(attempts) >= 3:
        raise ValueError('Daily mutation budget exhausted')
    if action == 'reboot' and state.get('last_action') not in ('restart-kubelet', 'restart-containerd'):
        raise ValueError('Reboot requires a prior service repair')
    return attempts


def main():
    if os.geteuid() != 0:
        action = os.environ.get('SSH_ORIGINAL_COMMAND', '')
        if action not in ACTIONS or len(sys.argv) != 1:
            raise SystemExit('Denied: exact recovery action required')
        os.execv('/usr/bin/sudo', ['sudo', '-n', __file__, '--root', action])
    if len(sys.argv) != 3 or sys.argv[1] != '--root' or sys.argv[2] not in ACTIONS:
        raise SystemExit('Denied: exact recovery action required')
    action = sys.argv[2]
    hostname = run(['/bin/hostname'])[0]
    if hostname not in ('home-base1', 'home-base2', 'home-base3', 'home-base4', 'home-base5'):
        raise SystemExit('Denied: unexpected node')
    os.umask(0o077)
    STATE_DIR.mkdir(mode=0o700, exist_ok=True)
    # SSH invocations can overlap; serialize policy decisions at the privileged boundary.
    import fcntl
    with (STATE_DIR / 'lock').open('w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        return locked_main(action, hostname)


def locked_main(action, hostname):
    policy = json.loads(POLICY.read_text())
    if action == 'disable':
        save(POLICY, {'enabled': False})
        print(json.dumps({'hostname': hostname, 'enabled': False}))
        return
    state_file = STATE_DIR / 'state.json'
    state = json.loads(state_file.read_text()) if state_file.exists() else {}
    data = own_node(hostname)
    ready = (next((c['status'] for c in data.get('status', {}).get('conditions', [])
                   if c['type'] == 'Ready'), 'Unknown') if data is not None else None)
    if ready == 'True':
        state.update(episode_rebooted=False, last_action=None)
        save(state_file, state)
    if action == 'diagnose':
        units = {unit: run(['/usr/bin/systemctl', 'is-active', unit])[0]
                 for unit in ('kubelet', 'containerd')}
        print(json.dumps({'hostname': hostname, 'services': units, 'enabled': policy['enabled'],
                          'ready': ready,
                          'uptime': Path('/proc/uptime').read_text().split()[0],
                          'memory': run(['/usr/bin/free', '-m'])[0],
                          'disk': run(['/bin/df', '-P', '/'])[0]}))
        return
    if policy.get('enabled') is not True:
        raise SystemExit('Denied: helper recovery is disabled')
    now = time.time()
    attempts = validate_mutation(action, hostname, data, state, now)
    state.update(last_action=action, last_mutation=now, mutations=attempts + [now])
    if action == 'reboot':
        state['episode_rebooted'] = True
    save(state_file, state)
    if action == 'reboot':
        argv = ['/usr/bin/systemd-run', '--unit=homebase-recovery-reboot',
                '--on-active=10s', '/usr/bin/systemctl', 'reboot']
    else:
        argv = ['/usr/bin/systemctl', 'restart', action.removeprefix('restart-')]
    output, rc = run(argv)
    print(json.dumps({'hostname': hostname, 'action': action, 'returncode': rc,
                      'output': output}))
    raise SystemExit(rc)


if __name__ == '__main__':
    main()
