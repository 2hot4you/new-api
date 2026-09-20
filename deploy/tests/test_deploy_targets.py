import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'resolve-targets.py'
spec = importlib.util.spec_from_file_location('targets', SCRIPT)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
SHA = 'a' * 40


def environment(branch='main', target='production-molii', **values):
    return dict(GITHUB_EVENT_NAME='workflow_dispatch', GITHUB_REF='refs/heads/' + branch,
                GITHUB_SHA=SHA, DEPLOY_TARGET=target, GITHUB_OUTPUT='', **values)


class TargetsTest(unittest.TestCase):
    def test_every_manual_target_and_branch_boundary(self):
        for target in (*module.PROFILES, 'all-production'):
            branch = 'develop' if target == 'development' else 'main'
            with self.subTest(target=target):
                result = module.resolve(environment(branch, target))
                self.assertEqual(result['source_sha'], SHA)
                ids = ['production-molii', 'production-ixiaozu', 'production-claudeye', 'production-model-claudeye'] if target == 'all-production' else [target]
                self.assertEqual([entry['id'] for entry in result['targets']], ids)
                for entry in result['targets']:
                    self.assertEqual(entry['brand_profile'], 'claudeye' if 'claudeye' in entry['id'] else ('ixiaozu' if 'ixiaozu' in entry['id'] else 'molii'))
                    self.assertEqual('local-db' in entry['compose_file'], 'claudeye' in entry['id'])
                with self.assertRaises(ValueError):
                    module.resolve(environment('main' if branch == 'develop' else 'develop', target))

    def test_push_targets_all_production_sites_and_isolates_development(self):
        for branch, expected in [('main', ['production-molii', 'production-ixiaozu', 'production-claudeye', 'production-model-claudeye']), ('develop', ['development'])]:
            env = environment(branch, '')
            env['GITHUB_EVENT_NAME'] = 'push'
            self.assertEqual([item['id'] for item in module.resolve(env)['targets']], expected)
            env['DEPLOY_TARGET'] = 'production-claudeye'
            with self.assertRaises(ValueError):
                module.resolve(env)

    def test_invalid_inputs(self):
        cases = [{'GITHUB_EVENT_NAME': 'pull_request'}, {'GITHUB_REF': 'refs/tags/main'},
                 {'GITHUB_REF': 'refs/heads/feature'}, {'DEPLOY_TARGET': ''},
                 {'DEPLOY_TARGET': 'production'}, {'GITHUB_SHA': 'abc'},
                 {'SOURCE_REF': 'main'}, {'SOURCE_REF': 'b' * 40},
                 {'BACKUP_POSTGRES': 'true'}, {'VERIFY_REPEATED_STARTUP': 'true'}]
        for overrides in cases:
            with self.subTest(overrides=overrides), self.assertRaises(ValueError):
                module.resolve(environment() | overrides)
        self.assertEqual(module.resolve(environment(SOURCE_REF=SHA))['source_sha'], SHA)

    def test_cli_outputs_nothing_for_invalid_production_source(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'output'
            env = os.environ | environment(SOURCE_REF='develop') | {'GITHUB_OUTPUT': str(output)}
            result = subprocess.run(['python3', str(SCRIPT)], env=env, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(output.exists())
            env['SOURCE_REF'] = SHA
            result = subprocess.run(['python3', str(SCRIPT)], env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            values = dict(line.split('=', 1) for line in output.read_text().splitlines())
            self.assertEqual(values['source_sha'], SHA)
            self.assertEqual([entry['id'] for entry in json.loads(values['targets'])], ['production-molii'])

    def test_development_source_resolves_real_git_commit_once(self):
        with tempfile.TemporaryDirectory() as directory:
            def git(*args):
                return subprocess.check_output(['git', '-C', directory, *args], text=True).strip()
            git('init', '-q')
            git('-c', 'user.name=Test', '-c', 'user.email=test@example.com', 'commit', '--allow-empty', '-qm', 'first')
            first = git('rev-parse', 'HEAD')
            git('tag', 'candidate')
            git('update-ref', 'refs/remotes/origin/candidate-branch', first)
            git('-c', 'user.name=Test', '-c', 'user.email=test@example.com', 'commit', '--allow-empty', '-qm', 'second')
            second = git('rev-parse', 'HEAD')
            for ref in ['candidate', 'candidate-branch', 'refs/heads/candidate-branch', first]:
                result = subprocess.run(['python3', str(SCRIPT)], cwd=directory, env=os.environ | environment('develop', 'development', SOURCE_REF=ref), capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(json.loads(result.stdout)['source_sha'], first)
                self.assertNotEqual(first, second)
            output = Path(directory) / 'output'
            env = os.environ | environment('develop', 'development', SOURCE_REF='candidate')
            env['GITHUB_OUTPUT'] = str(output)
            result = subprocess.run(['python3', str(SCRIPT)], cwd=directory, env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, '')
            values = dict(line.split('=', 1) for line in output.read_text().splitlines())
            self.assertEqual(values['source_sha'], first)
            self.assertEqual([entry['id'] for entry in json.loads(values['targets'])], ['development'])
            for ref in ['--help', '-q', 'missing', 'HEAD\nother', 'HEAD:missing']:
                result = subprocess.run(['python3', str(SCRIPT)], cwd=directory, env=os.environ | environment('develop', 'development', SOURCE_REF=ref), capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0, ref)


if __name__ == '__main__':
    unittest.main()
