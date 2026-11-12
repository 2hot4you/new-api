"""Exercise the Tencent runner and release gates without any network calls."""
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

from test_deploy_targets import APPROVAL, EVENING, environment, module as targets


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'deploy/model-claudeye-tencent-ci.sh'
MODEL = 'production-model-claudeye'
OLD = 'ghcr.io/2hot4you/new-api@sha256:' + '1' * 64
NEW = 'ghcr.io/2hot4you/new-api@sha256:' + '2' * 64
SHA = 'a' * 40
PREVIOUS = f'MODEL_CLAUDEYE_PREVIOUS_IMAGE={OLD}\n'


class RunnerTest(unittest.TestCase):
    def setUp(self):
        fixture = tempfile.TemporaryDirectory(prefix='model-tencent fixture ')
        self.addCleanup(fixture.cleanup)
        self.path = Path(fixture.name)
        self.output = self.path / 'github-output'
        self.calls = self.path / 'ssh-calls'
        ssh = self.path / 'ssh'
        ssh.write_text('''#!/bin/sh
printf '%s\\n' "$@" >> "$SSH_CALLS"
printf '%s' "$FIXTURE_STDOUT"
printf '%s\\n' 'upstream private diagnostics' >&2
exit "$FIXTURE_STATUS"
''')
        ssh.chmod(0o700)
        self.env = os.environ | {
            'PATH': f'{self.path}:{os.environ["PATH"]}',
            'RUNNER_TEMP': str(self.path),
            'GITHUB_OUTPUT': str(self.output),
            'MODEL_TENCENT_SSH_HOST': 'tencent.example.invalid',
            'MODEL_TENCENT_SSH_PORT': '2222',
            'MODEL_TENCENT_SSH_USER': 'model-deployer',
            'MODEL_TENCENT_SSH_PRIVATE_KEY': 'fixture-private-key',
            'MODEL_TENCENT_SSH_KNOWN_HOSTS': 'fixture-known-host',
            'SSH_CALLS': str(self.calls),
        }

    def run_client(self, *args, stdout='', status=0, overrides=None):
        self.output.unlink(missing_ok=True)
        self.calls.unlink(missing_ok=True)
        result = subprocess.run(
            ['bash', str(SCRIPT), *args], text=True, capture_output=True,
            env=self.env | {'FIXTURE_STDOUT': stdout, 'FIXTURE_STATUS': str(status)} | (overrides or {}),
        )
        self.assertNotIn('upstream private diagnostics', result.stdout + result.stderr)
        self.assertEqual(list(self.path.glob('model-tencent-ssh.*')), [])
        return result, self.output.read_text() if self.output.exists() else ''

    def test_preflight_pins_identity_and_accepts_one_previous_image(self):
        result, outputs = self.run_client('preflight', stdout=PREVIOUS)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(outputs, f'previous_image={OLD}\n')
        self.assertEqual(result.stdout, '')
        argv = self.calls.read_text().splitlines()
        self.assertEqual(argv[-2:], ['model-deployer@tencent.example.invalid', 'preflight'])
        self.assertEqual(argv[argv.index('-F') + 1], '/dev/null')
        self.assertEqual(argv[argv.index('-p') + 1], '2222')
        for option in ('-n', '-T', 'StrictHostKeyChecking=yes', 'GlobalKnownHostsFile=/dev/null',
                       'BatchMode=yes', 'IdentitiesOnly=yes', 'IdentityAgent=none',
                       'ClearAllForwardings=yes', 'ForwardAgent=no', 'ForwardX11=no', 'UpdateHostKeys=no'):
            self.assertIn(option, argv)
        self.assertTrue(any(item.startswith('UserKnownHostsFile=') and item != 'UserKnownHostsFile=/dev/null' for item in argv))

    def test_preflight_rejects_empty_extra_malformed_or_failed_output(self):
        for output, status in [('', 0), (PREVIOUS + '\n', 0), (PREVIOUS * 2, 0),
                               ('::error::untrusted\n' + PREVIOUS, 0), (PREVIOUS.rstrip(), 0),
                               (PREVIOUS.replace('2hot4you', 'other'), 0), (PREVIOUS, 255)]:
            with self.subTest(output=output, status=status):
                result, outputs = self.run_client('preflight', stdout=output, status=status)
                self.assertEqual(result.returncode, status or 1)
                self.assertEqual(outputs, '')
                self.assertNotIn('::error::untrusted', result.stdout + result.stderr)

    def test_release_preserves_remote_status_and_allowlisted_rollback(self):
        for status, rollback in [(0, 'not_attempted'), (21, 'not_attempted'), (42, 'succeeded'), (7, 'failed')]:
            with self.subTest(status=status, rollback=rollback):
                result, outputs = self.run_client('release', OLD, NEW, SHA,
                                                  stdout=f'DEPLOY_ROLLBACK_RESULT={rollback}\n', status=status)
                self.assertEqual(result.returncode, status)
                self.assertEqual(outputs, f'rollback={rollback}\n')
                self.assertEqual(self.calls.read_text().splitlines()[-1], f'release {OLD} {NEW} {SHA}')
                self.assertEqual(result.stdout, '')

    def test_release_cannot_claim_success_or_rollback_from_invalid_response(self):
        for output, status in [('', 0), ('DEPLOY_ROLLBACK_RESULT=succeeded\n', 0),
                               ('DEPLOY_ROLLBACK_RESULT=not_attempted\n\n', 0),
                               ('::warning::untrusted\nDEPLOY_ROLLBACK_RESULT=succeeded\n', 23), ('', 255)]:
            with self.subTest(output=output, status=status):
                result, outputs = self.run_client('release', OLD, NEW, SHA, stdout=output, status=status)
                self.assertEqual(result.returncode, status or 1)
                self.assertEqual(outputs, 'rollback=unavailable\n')
                self.assertNotIn('::warning::untrusted', result.stdout + result.stderr)

    def test_rejects_arguments_or_ssh_settings_outside_the_protocol_before_connecting(self):
        valid = ('release', OLD, NEW, SHA)
        cases = [(('preflight', 'extra'), {}), (('true',), {}), (('release', '', NEW, SHA), {}),
                 (('release', OLD, NEW.replace('2hot4you', 'other'), SHA), {}),
                 (('release', OLD, NEW, SHA + '\n'), {}), (('release', OLD, NEW, SHA.upper()), {}),
                 (valid, {'MODEL_TENCENT_SSH_HOST': '-oProxyCommand=bad'}),
                 (valid, {'MODEL_TENCENT_SSH_PORT': '65536'}),
                 (valid, {'MODEL_TENCENT_SSH_USER': 'other@host'}),
                 (valid, {'MODEL_TENCENT_SSH_PRIVATE_KEY': ''}),
                 (valid, {'MODEL_TENCENT_SSH_KNOWN_HOSTS': ''})]
        for args, overrides in cases:
            with self.subTest(args=args, overrides=overrides):
                result, _ = self.run_client(*args, overrides=overrides)
                self.assertEqual(result.returncode, 1)
                self.assertFalse(self.calls.exists())


def workflow_if(block, *, ids=(), results=None, previous='', target='', event='push', cancelled=False):
    """Evaluate the small condition subset used by these jobs and steps."""
    match = re.search(r'^( +)if: ([^\n]*)\n((?:\1 +[^\n]*\n)*)', block, re.MULTILINE)
    if not match:
        return all(result == 'success' for result in (results or {}).values())
    expression = ' '.join(match[3].split()) if match[2] == '>-' else match[2]
    expression = expression.removeprefix('${{').removesuffix('}}').strip()
    expression = re.sub(r'contains\(fromJSON\(needs\.prepare\.outputs\.targets\)\.\*\.id, (\'[^\']+\')\)', r'\1 in ids', expression)
    expression = re.sub(r'needs\.([a-z_]+)\.result', r'results["\1"]', expression)
    expression = expression.replace('needs.model_tencent_preflight.outputs.previous_image', 'previous')
    expression = expression.replace('matrix.target.id', 'target').replace('github.event_name', 'event')
    expression = expression.replace('inputs.backup_postgres', 'False').replace('inputs.verify_repeated_startup', 'False')
    expression = expression.replace('always()', 'True').replace('cancelled()', 'cancelled')
    expression = expression.replace('&&', ' and ').replace('||', ' or ')
    expression = re.sub(r'!(?!=)', 'not ', expression).strip()
    # GitHub adds success() unless the condition names a status-check function.
    if not re.search(r'\b(?:always|cancelled|failure|success)\(\)', match[0]):
        if any(result != 'success' for result in (results or {}).values()):
            return False
    return bool(eval(expression, {'__builtins__': {}}, dict(ids=ids, results=results, previous=previous,
                                                            target=target, event=event, cancelled=cancelled)))


class WorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        workflow = (ROOT / '.github/workflows/deploy.yml').read_text()
        split = re.split(r'^  ([a-z_]+):\n', workflow.split('\njobs:\n', 1)[1], flags=re.MULTILINE)
        cls.jobs = dict(zip(split[1::2], split[2::2]))

    def test_preflight_follows_verification_and_blocks_entire_selected_matrix(self):
        self.assertIn('model_tencent_preflight', self.jobs.keys())
        preflight = self.jobs['model_tencent_preflight'].split('\n    steps:', 1)[0]
        release = self.jobs['release'].split('\n    steps:', 1)[0]
        self.assertRegex(preflight, r'needs:\n(?:      - [a-z_]+\n)*      - verify\n')
        self.assertRegex(release, r'needs:\n(?:      - [a-z_]+\n)*      - model_tencent_preflight\n')
        self.assertRegex(preflight, r'environment: production-model-claudeye\n')
        for event, branch, selection, wanted in [
            ('push', 'main', '', True), ('push', 'develop', '', False),
            ('workflow_dispatch', 'main', MODEL, True), ('workflow_dispatch', 'main', 'all-production', True),
            ('workflow_dispatch', 'main', 'production-molii', False),
            ('workflow_dispatch', 'main', 'production-claudeye', False),
            ('workflow_dispatch', 'main', 'production-ixiaozu', False),
            ('workflow_dispatch', 'develop', 'development', False),
        ]:
            with self.subTest(event=event, branch=branch, selection=selection):
                approvals = APPROVAL if selection == 'production-ixiaozu' else {}
                resolved = targets.resolve(environment(branch, selection, **approvals) | {'GITHUB_EVENT_NAME': event}, now=EVENING)
                ids = [target['id'] for target in resolved['targets']]
                passed = {'prepare': 'success', 'verify': 'success'}
                self.assertEqual(workflow_if(preflight, ids=ids, results=passed), wanted)
                self.assertFalse(workflow_if(preflight, ids=ids, results=passed | {'verify': 'failure'}))
                for status, output in [('success', OLD), ('success', ''), ('failure', OLD), ('cancelled', ''), ('skipped', '')]:
                    results = passed | {'model_tencent_preflight': status}
                    expected = (status == 'success' and output == OLD) if wanted else status == 'skipped'
                    self.assertEqual(workflow_if(release, ids=ids, results=results, previous=output), expected)
                    self.assertFalse(workflow_if(release, ids=ids, results=results, previous=output, cancelled=True))
                    self.assertFalse(workflow_if(release, ids=ids, results=results | {'verify': 'failure'}, previous=output))

    def test_model_build_and_reporting_stay_shared_but_legacy_remote_steps_are_unreachable(self):
        header, step_block = self.jobs['release'].split('\n    steps:', 1)
        split = re.split(r'^      - name: (.+)\n', step_block, flags=re.MULTILINE)
        steps = dict(zip(split[1::2], split[2::2]))
        self.assertNotRegex(header, r'secrets\.DEPLOY_SSH_|MODEL_TENCENT_SSH_|GHCR_TOKEN')
        for name in ('Build and push target image', 'Record target result', 'Upload target result', 'Notify Telegram'):
            self.assertTrue(workflow_if(steps[name], target=MODEL), name)
        for name in ('Configure SSH', 'Verify SSH connectivity', 'Authenticate server to GHCR',
                     'Upload development backup helper', 'Back up development PostgreSQL', 'Upload deployment assets',
                     'Deploy immutable image', 'Verify repeated development startup'):
            self.assertFalse(workflow_if(steps[name], target=MODEL), name)
        for target in ('development', 'production-molii', 'production-claudeye', 'production-ixiaozu'):
            self.assertTrue(workflow_if(steps['Deploy immutable image'], target=target))
            self.assertFalse(workflow_if(steps['Deploy Tencent application'], target=target))
        dedicated = steps['Deploy Tencent application']
        self.assertTrue(workflow_if(dedicated, target=MODEL))
        self.assertNotRegex(dedicated, r'github\.token|DEPLOY_SSH_|docker login|\bscp\b')
        for name in ('Record target result', 'Notify Telegram'):
            self.assertIn('steps.deploy_tencent.outputs.rollback', steps[name])


if __name__ == '__main__':
    unittest.main()
