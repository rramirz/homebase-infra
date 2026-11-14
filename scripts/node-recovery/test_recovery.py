#!/usr/bin/env python3
"""Policy/trigger tests use explicit fixtures, never real repair commands."""
from email.message import Message
import io
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import urllib.error

spec = importlib.util.spec_from_file_location('recovery', Path(__file__).with_name('agent.py'))
assert spec is not None and spec.loader is not None
agent = importlib.util.module_from_spec(spec)
spec.loader.exec_module(agent)
helper_spec = importlib.util.spec_from_file_location('helper', Path(__file__).with_name('helper.py'))
assert helper_spec is not None and helper_spec.loader is not None
helper = importlib.util.module_from_spec(helper_spec)
helper_spec.loader.exec_module(helper)


def snapshot(ready='True'):
    return {n: {'ready': ready, 'unschedulable': False, 'pressures': []} for n in agent.NODES}


class ProviderTests(unittest.TestCase):
    def test_only_selected_low_cost_model_is_requested(self):
        response = {'model':'jev-1.13','answers':{'action':{'type':'choice','choice':'escalate','confidence':1}}}
        with patch.object(agent.urllib.request, 'urlopen', return_value=io.BytesIO(json.dumps(response).encode())) as request:
            agent.advise({'opencode_api_key':'test-only-not-a-real-key'}, {'test':True}, ['escalate'])
            payload = json.loads(request.call_args.args[0].data)
            self.assertEqual(payload['model'], 'jev-1.13')
            self.assertEqual(payload['questions']['action']['type'], 'choice')
            self.assertEqual(set(payload['questions']['action']['criteria']), {'escalate'})
            self.assertEqual(request.call_count, 1)

    def test_provider_error_never_falls_back(self):
        error = urllib.error.HTTPError('url',429,'rate limit',Message(),None)
        with patch.object(agent.urllib.request, 'urlopen', side_effect=error) as request:
            with self.assertRaises(urllib.error.HTTPError):
                agent.advise({'opencode_api_key':'test-only-not-a-real-key'}, {'test':True}, ['escalate'])
            self.assertEqual(request.call_count, 1)


    def test_low_confidence_escalates(self):
        response = {'model':'jev-1.13','answers':{'action':{'type':'choice','choice':'restart-kubelet','confidence':0.6}}}
        with patch.object(agent.urllib.request, 'urlopen', return_value=io.BytesIO(json.dumps(response).encode())):
            result = agent.advise({'opencode_api_key':'test-only-not-a-real-key'}, {}, ['escalate','restart-kubelet'])
            self.assertEqual(result['action'], 'escalate')

    def test_unapproved_action_and_invalid_confidence_rejected(self):
        for choice, confidence in [('reboot',1), ('escalate',True), ('escalate',float('nan')), ('escalate',2)]:
            response = {'model':'jev-1.13','answers':{'action':{'type':'choice','choice':choice,'confidence':confidence}}}
            with self.subTest(choice=choice,confidence=confidence), \
                 patch.object(agent.urllib.request, 'urlopen', return_value=io.BytesIO(json.dumps(response).encode())):
                with self.assertRaises(ValueError):
                    agent.advise({'opencode_api_key':'test-only-not-a-real-key'}, {}, ['escalate'])


class PolicyTests(unittest.TestCase):
    def setUp(self):
        self.status = snapshot('False')['home-base3']
        self.diag = {'hostname':'home-base3','uptime':'3600','services':{'kubelet':'active','containerd':'active'}}
        self.record = {}

    def allowed(self, node='home-base3', now=10000):
        return agent.allowed_actions(node, self.status, self.diag, self.record, now)

    def test_initial_action_only_kubelet(self):
        self.assertEqual(self.allowed(), ['escalate', 'restart-kubelet'])

    def test_failed_runtime_can_restart(self):
        self.diag['services']['containerd'] = 'failed'
        self.assertEqual(self.allowed(), ['escalate', 'restart-containerd'])

    def test_reboot_only_after_prior_repair_and_cooldown(self):
        self.record.update(last_action='restart-kubelet', last_mutation=9000)
        self.assertIn('reboot', self.allowed())
        self.assertNotIn('reboot', self.allowed(now=9500))

    def test_never_reboot_controlplane(self):
        self.record.update(last_action='restart-kubelet', last_mutation=1)
        self.assertNotIn('reboot', self.allowed(node='home-base1'))

    def test_no_repeated_reboot(self):
        self.record['last_action'] = 'reboot'
        self.assertEqual(self.allowed(), ['escalate'])

    def test_reboot_history_blocks_failed_containerd_bypass(self):
        self.record.update(last_action='restart-containerd', episode_rebooted=True)
        self.diag['services']['containerd'] = 'failed'
        self.assertEqual(self.allowed(), ['escalate'])

    def test_maintenance_pressure_booting_and_ready_escalate(self):
        for changed in ({'ready':'True'}, {'unschedulable':True}, {'pressures':['DiskPressure']}):
            with self.subTest(changed=changed):
                self.status = {**self.status, **changed}
                self.assertEqual(self.allowed(), ['escalate'])
                self.status = snapshot('False')['home-base3']
        self.diag['uptime'] = '60'
        self.assertEqual(self.allowed(), ['escalate'])

    def test_invalid_diagnostics_escalate(self):
        self.diag = {}
        self.assertEqual(self.allowed(), ['escalate'])

    def test_injected_action_or_node_rejected_without_process(self):
        with patch.object(agent.subprocess, 'run') as run:
            for node, action in [('home-base3','reboot; rm -rf /'), ('attacker','diagnose')]:
                with self.assertRaises(ValueError):
                    agent.ssh(node, action)
            run.assert_not_called()


class HelperBoundaryTests(unittest.TestCase):
    def setUp(self):
        self.data = {'spec':{}, 'status':{'conditions':[
            {'type':'Ready','status':'False','lastTransitionTime':'1970-01-01T00:00:00Z'}]}}

    def validate(self, action='reboot', state=None):
        with patch.object(helper.Path, 'read_text', return_value='3600 0'):
            return helper.validate_mutation(action,'home-base3',self.data,state or {},10000)

    def test_direct_reboot_without_repair_denied(self):
        with self.assertRaisesRegex(ValueError,'prior service repair'):
            self.validate()

    def test_reboot_after_repair_and_cooldown_allowed(self):
        self.assertEqual(self.validate(state={'last_action':'restart-kubelet','last_mutation':8000}),[])

    def test_helper_budget_cooldown_and_episode_are_independent(self):
        for state in ({'last_mutation':9900}, {'mutations':[1,2,3]}, {'episode_rebooted':True}):
            with self.subTest(state=state), self.assertRaises(ValueError):
                self.validate('restart-containerd', state)

    def test_helper_rejects_healthy_or_unknown_api(self):
        for data in (None, {'spec':{},'status':{'conditions':[{'type':'Ready','status':'True'}]}}):
            self.data = data
            with self.assertRaises(ValueError):
                self.validate('restart-kubelet')

    def test_disabled_helper_cannot_mutate_even_with_recovery_key(self):
        with tempfile.TemporaryDirectory() as d, \
             patch.object(helper,'STATE_DIR',Path(d)/'state'), \
             patch.object(helper,'POLICY',Path(d)/'policy.json'), \
             patch.object(helper.os,'geteuid',return_value=0), \
             patch.object(helper.sys,'argv',['helper','--root','reboot']), \
             patch.object(helper,'run',return_value=('home-base3',0)), \
             patch.object(helper,'own_node',return_value=None), \
             patch.object(helper.subprocess,'run') as process:
            (Path(d)/'policy.json').write_text('{"enabled":false}')
            with self.assertRaisesRegex(SystemExit,'disabled'):
                helper.main()
            process.assert_not_called()


class AuditTests(unittest.TestCase):
    def test_durable_history_is_private_timestamped_and_skips_healthy_polls(self):
        with tempfile.TemporaryDirectory() as d, patch.object(agent, 'STATE', Path(d)), \
             patch.object(agent, 'AUDIT_ENABLED', True):
            agent.log('repair_requested', node='home-base3', action='restart-kubelet')
            agent.log('monitor', ready=5, total=5, ai_enabled=True)
            path = Path(d) / 'recovery.jsonl'
            rows = [json.loads(line) for line in path.read_text().splitlines()]
            self.assertEqual(len(rows), 1)
            self.assertEqual(rows[0]['event'], 'repair_requested')
            self.assertTrue(rows[0]['timestamp'].endswith('+00:00'))
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)

    def test_attempt_and_later_recovery_are_correlated_without_real_repairs(self):
        bad = snapshot()
        bad['home-base3']['ready'] = 'False'
        diagnostic = {'hostname':'home-base3','uptime':'3600','services':{'kubelet':'active','containerd':'active'}}
        with tempfile.TemporaryDirectory() as d, patch.object(agent, 'STATE', Path(d)), \
             patch.object(agent, 'AUDIT_ENABLED', True), \
             patch.object(agent, 'nodes', side_effect=[bad, bad, bad]), \
             patch.object(agent, 'advise', return_value={'action':'restart-kubelet','reason':'fixture'}), \
             patch.object(agent, 'notify'), patch.object(agent.time, 'sleep'), \
             patch.object(agent.time, 'time', return_value=10000):
            (Path(d) / 'incidents').mkdir()
            history = Path(d) / 'recovery.jsonl'
            def fixture_ssh(node, action):
                if action == 'diagnose':
                    return diagnostic
                self.assertEqual(action, 'restart-kubelet')
                self.assertEqual(json.loads(history.read_text().splitlines()[-1])['event'], 'repair_requested')
                return {'hostname':node, 'action':action, 'returncode':0}
            state: dict[str, dict[str, object]] = {'home-base3':{'first_failure':9000,'fails':4}}
            with patch.object(agent, 'ssh', side_effect=fixture_ssh):
                agent.monitor({'ai_enabled':True}, {}, state)
            incident_name = state['home-base3']['pending_incident']
            assert isinstance(incident_name, str)
            incident_path = Path(incident_name)
            self.assertFalse(json.loads(incident_path.read_text())['verified_ready'])
            agent.observe(snapshot(), state, 10060)
            incident = json.loads(incident_path.read_text())
            self.assertTrue(incident['verified_ready'])
            self.assertEqual(incident['recovery_observed_at'], 10060)
            rows = [json.loads(line) for line in history.read_text().splitlines()]
            self.assertEqual([r['event'] for r in rows], ['decision','repair_requested','repair_verified','node_recovered'])
            self.assertFalse(rows[2]['ready'])
            self.assertEqual(rows[3]['incident'], str(incident_path))
            self.assertNotIn('pending_incident', state['home-base3'])

    def test_audit_failure_prevents_repair_execution(self):
        bad = snapshot()
        bad['home-base3']['ready'] = 'False'
        diagnostic = {'hostname':'home-base3','uptime':'3600','services':{'kubelet':'active','containerd':'active'}}
        def failing_log(event, **fields):
            if event == 'repair_requested':
                raise OSError('fixture audit write failure')
        with tempfile.TemporaryDirectory() as d, patch.object(agent, 'STATE', Path(d)), \
             patch.object(agent, 'nodes', side_effect=[bad,bad]), \
             patch.object(agent, 'ssh', return_value=diagnostic) as ssh, \
             patch.object(agent, 'advise', return_value={'action':'restart-kubelet','reason':'fixture'}), \
             patch.object(agent, 'log', side_effect=failing_log), \
             patch.object(agent.time, 'time', return_value=10000):
            (Path(d) / 'incidents').mkdir()
            with self.assertRaises(OSError):
                agent.monitor({'ai_enabled':True}, {}, {'home-base3':{'first_failure':9000,'fails':4}})
            self.assertTrue(all(call.args[1] == 'diagnose' for call in ssh.call_args_list))


class TriggerTests(unittest.TestCase):
    def test_sustained_failure_and_recovery(self):
        state = {}
        bad = snapshot()
        bad['home-base3']['ready'] = 'Unknown'
        self.assertEqual(agent.observe(bad, state, 10000), [])
        self.assertEqual(agent.observe(bad, state, 10060), [])
        self.assertEqual(agent.observe(bad, state, 10120), [])
        self.assertEqual(agent.observe(bad, state, 10180), ['home-base3'])
        state['home-base3']['last_workflow'] = 10180
        self.assertEqual(agent.observe(bad, state, 10240), [])
        agent.observe(snapshot(), state, 10300)
        self.assertEqual(state['home-base3']['fails'], 0)

    def test_healthy_never_calls_model_or_ssh(self):
        with tempfile.TemporaryDirectory() as d, patch.object(agent, 'STATE', Path(d)), \
             patch.object(agent, 'nodes', return_value=snapshot()), \
             patch.object(agent, 'advise') as model, patch.object(agent, 'ssh') as ssh:
            agent.monitor({'ai_enabled':True}, {}, {})
            model.assert_not_called()
            ssh.assert_not_called()

    def test_disabled_sustained_failure_diagnoses_without_model_or_mutation(self):
        with tempfile.TemporaryDirectory() as d, patch.object(agent, 'STATE', Path(d)), \
             patch.object(agent, 'nodes', return_value=snapshot('False')), \
             patch.object(agent, 'advise') as model, \
             patch.object(agent, 'ssh', return_value={'hostname':'home-base1'}) as ssh, \
             patch.object(agent, 'notify'), patch.object(agent.time, 'time', return_value=10000):
            (Path(d)/'incidents').mkdir()
            state = {n:{'first_failure':9000,'fails':4} for n in agent.NODES}
            agent.monitor({'ai_enabled':False}, {}, state)
            model.assert_not_called()
            self.assertEqual(ssh.call_args.args, ('home-base1','diagnose'))
            incidents = list((Path(d)/'incidents').glob('*.json'))
            self.assertEqual(len(incidents),1)
            self.assertFalse(json.loads(incidents[0].read_text())['ai_enabled'])

    def test_api_outage_never_reboots(self):
        with tempfile.TemporaryDirectory() as d, patch.object(agent, 'STATE', Path(d)), \
             patch.object(agent, 'nodes', side_effect=TimeoutError), \
             patch.object(agent, 'advise') as model, patch.object(agent, 'ssh') as ssh, \
             patch.object(agent, 'notify') as notify:
            agent.monitor({'ai_enabled':True}, {}, {'_api':{'fails':2}})
            model.assert_not_called()
            ssh.assert_not_called()
            notify.assert_called_once()

    def test_provider_failure_never_mutates(self):
        bad = snapshot()
        bad['home-base3']['ready'] = 'False'
        diag = {'hostname':'home-base3','uptime':'3600','services':{'kubelet':'active','containerd':'active'}}
        with tempfile.TemporaryDirectory() as d, patch.object(agent, 'STATE', Path(d)), \
             patch.object(agent, 'nodes', return_value=bad), \
             patch.object(agent, 'advise', side_effect=urllib.error.HTTPError('url',402,'funds',Message(),None)), \
             patch.object(agent, 'ssh', return_value=diag) as ssh, patch.object(agent, 'notify'), \
             patch.object(agent.time, 'time', return_value=10000):
            (Path(d)/'incidents').mkdir()
            state = {'home-base3':{'first_failure':9000,'fails':4}}
            agent.monitor({'ai_enabled':True}, {}, state)
            self.assertEqual(ssh.call_count,1)
            self.assertEqual(ssh.call_args.args,('home-base3','diagnose'))
            self.assertNotIn('last_mutation', state['home-base3'])

    def test_recovered_before_write_cancels_repair(self):
        bad = snapshot()
        bad['home-base3']['ready'] = 'False'
        diag = {'hostname':'home-base3','uptime':'3600','services':{'kubelet':'active','containerd':'active'}}
        with tempfile.TemporaryDirectory() as d, patch.object(agent, 'STATE', Path(d)), \
             patch.object(agent, 'nodes', side_effect=[bad,snapshot()]), \
             patch.object(agent, 'advise', return_value={'action':'restart-kubelet','reason':'fixture'}), \
             patch.object(agent, 'ssh', return_value=diag) as ssh, patch.object(agent, 'notify'), \
             patch.object(agent.time, 'time', return_value=10000):
            (Path(d)/'incidents').mkdir()
            agent.monitor({'ai_enabled':True}, {}, {'home-base3':{'first_failure':9000,'fails':4}})
            self.assertTrue(all(c.args[1]=='diagnose' for c in ssh.call_args_list))

    def test_daily_budget_blocks_actions(self):
        bad = snapshot()
        bad['home-base3']['ready'] = 'False'
        with tempfile.TemporaryDirectory() as d, patch.object(agent, 'STATE', Path(d)), \
             patch.object(agent, 'nodes', return_value=bad), patch.object(agent, 'advise') as model, \
             patch.object(agent, 'ssh', return_value={'hostname':'home-base3'}), \
             patch.object(agent, 'notify'), patch.object(agent.time, 'time', return_value=10000):
            (Path(d)/'incidents').mkdir()
            state = {'home-base3':{'first_failure':9000,'fails':4,'mutations':[100,200,300]}}
            agent.monitor({'ai_enabled':True}, {}, state)
            model.assert_not_called()


if __name__ == '__main__':
    unittest.main()
