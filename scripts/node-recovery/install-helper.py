#!/usr/bin/python3
"""One-time privileged bootstrap; run only by the approved temporary installer."""
import os
import json
from pathlib import Path
import pwd
import subprocess
import tempfile
import secrets


def atomic(path, content, mode, uid=0, gid=0):
    fd, name = tempfile.mkstemp(prefix='.' + path.name + '.', dir=path.parent)
    temp = Path(name)
    os.fchmod(fd, mode)
    os.fchown(fd, uid, gid)
    with os.fdopen(fd, 'w') as f:
        f.write(content)
    temp.replace(path)


def install_login_key(user, key):
    # No root writes in user-owned directories. Drop all privileges before opening .ssh.
    pid = os.fork()
    if pid == 0:
        try:
            os.setgroups([])
            os.setgid(user.pw_gid)
            os.setuid(user.pw_uid)
            sshdir = Path(user.pw_dir) / '.ssh'
            sshdir.mkdir(mode=0o700, exist_ok=True)
            directory = os.open(sshdir, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            try:
                fd = os.open('authorized_keys', os.O_RDONLY | os.O_NOFOLLOW, dir_fd=directory)
            except FileNotFoundError:
                original = ''
            else:
                with os.fdopen(fd) as f:
                    original = f.read()
            lines = [line for line in original.splitlines() if not line.endswith(' homelab-node-recovery')]
            lines.append(key)
            temp = '.node-recovery-' + secrets.token_hex(12)
            fd = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=directory)
            with os.fdopen(fd, 'w') as f:
                f.write('\n'.join(lines) + '\n')
                f.flush()
                os.fsync(f.fileno())
            os.rename(temp, 'authorized_keys', src_dir_fd=directory, dst_dir_fd=directory)
            os.close(directory)
            os._exit(0)
        except Exception:
            os._exit(1)
    _, status = os.waitpid(pid, 0)
    if status != 0:
        raise RuntimeError('Unprivileged SSH-key installation failed; inspect .ssh ownership/symlinks')


def main():
    if os.geteuid() != 0:
        raise SystemExit('Installer requires root')
    user = pwd.getpwnam('rramirez')
    helper = Path('/usr/local/sbin/homebase-node-recovery-helper')
    atomic(helper, Path('/run/node-recovery-helper.py').read_text(), 0o755)
    policy = json.loads(Path('/run/node-recovery-policy.json').read_text())
    if type(policy.get('enabled')) is not bool:
        raise ValueError('Invalid helper policy')
    atomic(Path('/etc/homebase-node-recovery-policy.json'), json.dumps(policy), 0o600)
    actions = ('diagnose', 'disable', 'restart-kubelet', 'restart-containerd', 'reboot')
    sudoers = ', '.join(str(helper) + ' --root ' + a for a in actions)
    sudo_content = '# Managed by homebase-infra node recovery\nrramirez ALL=(root) NOPASSWD: ' + sudoers + '\n'
    candidate = Path('/etc/sudoers.d/.homebase-node-recovery-check')
    atomic(candidate, sudo_content, 0o440)
    subprocess.run(['/usr/sbin/visudo', '-cf', str(candidate)], check=True)
    candidate.replace('/etc/sudoers.d/homebase-node-recovery')
    key = Path('/run/node-recovery-authorized-key').read_text().strip()
    if not key.startswith('from="192.168.1.216",restrict,command=') or '\n' in key:
        raise SystemExit('Invalid restricted key')
    install_login_key(user, key)
    print('INSTALLED restricted helper, sudo rules, and source-restricted SSH key')


if __name__ == '__main__':
    main()
