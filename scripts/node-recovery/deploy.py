#!/usr/bin/env python3
"""Deploy from Rafael's workstation; credentials travel only over stdin/TLS/SSH."""
import base64
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import time

HERE = Path(__file__).resolve().parent
HOST = 'root@192.168.1.216'
CONTEXT = 'home-base'
HOME = '/etc/homelab-node-recovery'
BOOTSTRAP_NS = 'homebase-node-recovery-bootstrap'


def command(argv, payload=None):
    result = subprocess.run(argv, input=payload, text=True, capture_output=True, timeout=120)
    if result.returncode:
        # Commands never include credentials; provider/credential responses are not printed.
        raise RuntimeError(f'Command failed: {argv[:4]}: {result.stderr[:1000]}')
    return result.stdout


def kube(*args, payload=None):
    return command(['kubectl', '--context', CONTEXT, '--request-timeout=30s', *args], payload)


def remote(script, payload=None):
    return command(['ssh', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', HOST, script], payload)


def put_files(files):
    # Read-back hashes verify exact external targets without printing credential contents.
    script = """import sys,json,os,base64,hashlib
from pathlib import Path
files=json.load(sys.stdin)
for item in files:
 p=Path(item['path']); p.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
 data=base64.b64decode(item['data']); temp=p.with_suffix('.deploy-tmp')
 fd=os.open(temp,os.O_WRONLY|os.O_CREAT|os.O_TRUNC,item['mode'])
 with os.fdopen(fd,'wb') as f: f.write(data)
 os.chmod(temp,item['mode']); temp.replace(p)
 assert hashlib.sha256(p.read_bytes()).hexdigest()==item['sha256']
print('verified',len(files),'remote files')
"""
    import shlex
    data = [{'path': path, 'data': base64.b64encode(content.encode()).decode(), 'mode': mode,
             'sha256': hashlib.sha256(content.encode()).hexdigest()} for path, content, mode in files]
    print(remote('python3 -c ' + shlex.quote(script), json.dumps(data)).strip())


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--enable-ai', action='store_true', help='Explicitly arm helpers and AI after successful provider probe')
    args = parser.parse_args()
    text = (Path.home() / '.opencode.env').read_text()
    match = re.search(r'(?m)^(?:export\s+)?api\s*=\s*(.*)$', text)
    if not match:
        raise SystemExit('Expected api variable in ~/.opencode.env')
    api_key = match.group(1).strip().strip('\"\'')
    if not api_key or any(c.isspace() for c in api_key):
        raise SystemExit('Invalid API key format')
    # Never overwrite an enabled deployment silently.
    existing = remote(f'test ! -e {HOME}/config.json || python3 -c "import json; print(json.load(open(\'{HOME}/config.json\'))[\'ai_enabled\'])"').strip()
    if existing == 'True':
        raise SystemExit('AI recovery is already enabled; disable/review before redeploying')
    remote(f'install -d -m 0700 {HOME}; test -f {HOME}/id_ed25519 || ssh-keygen -q -t ed25519 -N "" -C homelab-node-recovery -f {HOME}/id_ed25519')
    public_key = remote(f'python3 -c "from pathlib import Path; print(Path(\'{HOME}/id_ed25519.pub\').read_text().strip())"').strip()
    if not public_key.startswith('ssh-ed25519 ') or not public_key.endswith(' homelab-node-recovery'):
        raise SystemExit('Unexpected recovery public key')
    restricted = 'from="192.168.1.216",restrict,command="/usr/local/sbin/homebase-node-recovery-helper" ' + public_key
    known = []
    for i in range(1, 6):
        ip = '192.168.1.' + str(134+i)
        key = command(['ssh', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', 'rramirez@'+ip,
                       "python3 -c \"from pathlib import Path; print(Path('/etc/ssh/ssh_host_ed25519_key.pub').read_text().strip())\""]).strip()
        known.append(ip + ' ' + ' '.join(key.split()[:2]))
    resources = [
        {'apiVersion': 'v1', 'kind': 'ServiceAccount', 'metadata': {'name': 'homelab-node-recovery', 'namespace': 'kube-system'}, 'automountServiceAccountToken': False},
        {'apiVersion': 'rbac.authorization.k8s.io/v1', 'kind': 'ClusterRole', 'metadata': {'name': 'homelab-node-recovery'},
         'rules': [{'apiGroups': [''], 'resources': ['nodes'], 'verbs': ['get', 'list']}]},
        {'apiVersion': 'rbac.authorization.k8s.io/v1', 'kind': 'ClusterRoleBinding', 'metadata': {'name': 'homelab-node-recovery'},
         'roleRef': {'apiGroup': 'rbac.authorization.k8s.io', 'kind': 'ClusterRole', 'name': 'homelab-node-recovery'},
         'subjects': [{'kind': 'ServiceAccount', 'name': 'homelab-node-recovery', 'namespace': 'kube-system'}]},
        {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': 'homelab-node-recovery-token', 'namespace': 'kube-system',
         'annotations': {'kubernetes.io/service-account.name': 'homelab-node-recovery'}}, 'type': 'kubernetes.io/service-account-token'},
    ]
    print(kube('apply', '-f', '-', payload=json.dumps({'apiVersion':'v1','kind':'List','items':resources})).strip())
    for _ in range(15):
        secret = json.loads(kube('-n', 'kube-system', 'get', 'secret', 'homelab-node-recovery-token', '-o', 'json'))
        if 'token' in secret.get('data', {}):
            break
        time.sleep(1)
    else:
        raise SystemExit('Service-account token not issued')
    role = json.loads(kube('get', 'clusterrole', 'homelab-node-recovery', '-o', 'json'))
    assert role['rules'] == resources[1]['rules'], 'Unexpected recovery permissions'
    credentials = {'opencode_api_key': api_key, 'kube_token': base64.b64decode(secret['data']['token']).decode()}
    files = [
        (HOME+'/credentials.json', json.dumps(credentials), 0o600),
        (HOME+'/config.json', json.dumps({'ai_enabled':False}), 0o600),
        (HOME+'/ca.crt', base64.b64decode(secret['data']['ca.crt']).decode(), 0o644),
        (HOME+'/known_hosts', '\n'.join(known)+'\n', 0o600),
        ('/usr/local/sbin/homelab-node-recovery', (HERE/'agent.py').read_text(), 0o755),
    ]
    put_files(files)
    if args.enable_ai:
        print(remote('/usr/local/sbin/homelab-node-recovery --probe-model').strip())
    namespace = {'apiVersion':'v1','kind':'Namespace','metadata':{'name':BOOTSTRAP_NS,
                  'labels':{'pod-security.kubernetes.io/enforce':'privileged','app.kubernetes.io/managed-by':'homebase-node-recovery'}}}
    existing_ns = json.loads(kube('get', 'namespaces', '-o', 'json'))
    if any(n['metadata']['name'] == BOOTSTRAP_NS for n in existing_ns['items']):
        raise SystemExit('Bootstrap namespace exists: inspect it before rerunning')
    kube('create', '-f', '-', payload=json.dumps(namespace))
    try:
        cm = {'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'installer','namespace':BOOTSTRAP_NS},
              'data':{'helper.py':(HERE/'helper.py').read_text(), 'install.py':(HERE/'install-helper.py').read_text(),
                      'authorized-key':restricted+'\n', 'policy.json':json.dumps({'enabled':args.enable_ai})}}
        kube('create','-f','-',payload=json.dumps(cm))
        script = ('set -eu; cp /payload/helper.py /host/run/node-recovery-helper.py; '
                  'cp /payload/install.py /host/run/node-recovery-install.py; '
                  'cp /payload/authorized-key /host/run/node-recovery-authorized-key; '
                  'cp /payload/policy.json /host/run/node-recovery-policy.json; '
                  'chroot /host /usr/bin/python3 /run/node-recovery-install.py; '
                  'rm /host/run/node-recovery-helper.py /host/run/node-recovery-install.py /host/run/node-recovery-authorized-key /host/run/node-recovery-policy.json; '
                  'touch /tmp/installed; sleep 3600')
        ds = {'apiVersion':'apps/v1','kind':'DaemonSet','metadata':{'name':'installer','namespace':BOOTSTRAP_NS},'spec':{
            'selector':{'matchLabels':{'app':'node-recovery-installer'}},
            'template':{'metadata':{'labels':{'app':'node-recovery-installer'}},'spec':{
                'automountServiceAccountToken':False,
                'affinity':{'nodeAffinity':{'requiredDuringSchedulingIgnoredDuringExecution':{'nodeSelectorTerms':[
                    {'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':[f'home-base{i}' for i in range(1,6)]}]}]}}},
                'tolerations':[{'operator':'Exists'}],
                'containers':[{'name':'installer','image':'docker.io/library/busybox:1.37.0',
                    'securityContext':{'privileged':True,'runAsUser':0},'command':['sh','-c',script],
                    'resources':{'requests':{'cpu':'10m','memory':'16Mi'},'limits':{'memory':'64Mi'}},
                    'readinessProbe':{'exec':{'command':['test','-f','/tmp/installed']},'periodSeconds':2},
                    'volumeMounts':[{'name':'root','mountPath':'/host'},{'name':'payload','mountPath':'/payload','readOnly':True}]}],
                'volumes':[{'name':'root','hostPath':{'path':'/','type':'Directory'}},{'name':'payload','configMap':{'name':'installer'}}]}}}}
        kube('create','-f','-',payload=json.dumps(ds))
        # Individual calls remain bounded; installation progress is checked against five exact targets.
        for _ in range(60):
            status = json.loads(kube('-n',BOOTSTRAP_NS,'get','daemonset','installer','-o','json'))['status']
            if status.get('numberReady') == 5 and status.get('desiredNumberScheduled') == 5:
                break
            time.sleep(3)
        else:
            print(kube('-n',BOOTSTRAP_NS,'get','pods','-o','wide'))
            raise RuntimeError('Installer did not become ready on all five nodes')
        pods = json.loads(kube('-n',BOOTSTRAP_NS,'get','pods','-o','json'))['items']
        assert {p['spec']['nodeName'] for p in pods} == {f'home-base{i}' for i in range(1,6)}
        for p in pods:
            output = kube('-n',BOOTSTRAP_NS,'logs',p['metadata']['name'])
            assert 'INSTALLED restricted helper' in output
        print('Verified installer completed on all five nodes')
    finally:
        kube('delete','namespace',BOOTSTRAP_NS,'--wait=false')
    # Read-only, real SSH/forced-command and Kubernetes API verification before enabling the timer.
    print(remote('/usr/local/sbin/homelab-node-recovery --diagnose-all').strip())
    print(remote('/usr/local/sbin/homelab-node-recovery').strip())
    service = '''[Unit]
Description=Out-of-cluster Kubernetes monitor and bounded Jev recovery
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/homelab-node-recovery
TimeoutStartSec=240
UMask=0077
StateDirectory=homelab-node-recovery
StateDirectoryMode=0700
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/var/lib/homelab-node-recovery
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
'''
    timer = '''[Unit]
Description=Monitor Kubernetes nodes from pve-1 every minute

[Timer]
OnBootSec=2min
OnUnitActiveSec=1min
AccuracySec=5s

[Install]
WantedBy=timers.target
'''
    rotation = (HERE / 'logrotate.conf').read_text()
    put_files([('/etc/systemd/system/homelab-node-recovery.service',service,0o644),
               ('/etc/systemd/system/homelab-node-recovery.timer',timer,0o644),
               ('/etc/logrotate.d/homelab-node-recovery',rotation,0o644)])
    print(remote('systemd-analyze verify /etc/systemd/system/homelab-node-recovery.service /etc/systemd/system/homelab-node-recovery.timer; systemctl daemon-reload && systemctl enable --now homelab-node-recovery.timer && systemctl start homelab-node-recovery.service && systemctl is-enabled homelab-node-recovery.timer && systemctl is-active homelab-node-recovery.timer && journalctl -u homelab-node-recovery.service -n 8 --no-pager').strip())
    if args.enable_ai:
        print(remote('/usr/local/sbin/homelab-node-recovery --enable-ai').strip())
        print('Deployment verified. AI recovery explicitly ENABLED.')
    else:
        print('Deployment verified. AI recovery remains DISABLED at agent and helpers.')


if __name__ == '__main__':
    main()
