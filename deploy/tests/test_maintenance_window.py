"""Exercise runner and server guards without contacting any deployment host."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class MaintenanceWindowTest(unittest.TestCase):
    def test_server_clock_rechecked_after_connection_before_login_or_upload(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = Path(directory)
            for name, body in {
                'date': '#!/bin/sh\nprintf "%s\\n" "$FIXTURE_CLOCK"\n',
                'ssh': '#!/usr/bin/env bash\nexport FIXTURE_CLOCK="${SERVER_CLOCK:-2026-10-27 00}"\n/bin/sh -c "${!#}"\n',
                'scp': '#!/bin/sh\ntouch "$MUTATION_MARKER"\n',
            }.items():
                path = fixture / name
                path.write_text(body)
                path.chmod(0o700)
            marker = fixture / 'server-mutation'
            artifact = fixture / 'artifact'
            artifact.write_text('fixture')
            env = os.environ | {
                'PATH': str(fixture) + ':' + os.environ['PATH'],
                'DEPLOY_ENVIRONMENT': 'production-ixiaozu',
                'IXIAOZU_MAINTENANCE_APPROVED': 'true',
                'IXIAOZU_MAINTENANCE_DATE': '2026-10-26',
                'FIXTURE_CLOCK': '2026-10-26 23',
                'MUTATION_MARKER': str(marker),
            }
            for command in [
                ['ssh', '-p', '22', 'fixture-host', 'touch', str(marker)],
                ['scp', '-P', '22', str(artifact), 'fixture-host:' + str(marker)],
            ]:
                result = subprocess.run(['bash', str(ROOT / 'deploy/guard-remote.sh'), *command], env=env, capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0, result.stderr)
                self.assertFalse(marker.exists())
            allowed = env | {'SERVER_CLOCK': '2026-10-26 23'}
            result = subprocess.run(['bash', str(ROOT / 'deploy/guard-remote.sh'), 'scp', '-P', '22', str(artifact), 'fixture-host:' + str(marker)], env=allowed, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(marker.read_text(), 'fixture')
            marker.unlink()
            second = fixture / "quoted ' asset"
            second.write_text('second')
            remote_dir = fixture / 'remote dir'
            remote_dir.mkdir()
            result = subprocess.run(['bash', str(ROOT / 'deploy/guard-remote.sh'), 'scp', '-o', 'BatchMode=yes', str(artifact), str(second), 'fixture-host:' + str(remote_dir) + '/'], env=allowed, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual((remote_dir / artifact.name).read_text(), 'fixture')
            self.assertEqual((remote_dir / second.name).read_text(), 'second')
            self.assertEqual(list(remote_dir.glob('*.pending.*')), [])

    def test_runner_and_server_refuse_before_any_side_effect(self):
        with tempfile.TemporaryDirectory() as directory:
            fixture = Path(directory)
            date = fixture / 'date'
            date.write_text('#!/bin/sh\nprintf "%s\\n" "$FIXTURE_CLOCK"\n')
            date.chmod(0o700)
            marker = fixture / 'contacted'
            env = os.environ | {
                'PATH': str(fixture) + ':' + os.environ['PATH'],
                'DEPLOY_ENVIRONMENT': 'production-ixiaozu',
                'IXIAOZU_MAINTENANCE_APPROVED': 'true',
                'IXIAOZU_MAINTENANCE_DATE': '2026-10-26',
                'MOLII_DEPLOY_ROOT': str(fixture / 'molii'),
                'IXIAOZU_DEPLOY_ROOT': str(fixture / 'ixiaozu'),
                'DOCS_DEPLOY_ROOT': str(fixture / 'public'),
            }
            for clock, approved, day in [('2026-10-26 19', 'true', '2026-10-26'), ('2026-10-26 20', 'false', '2026-10-26'), ('2026-10-27 00', 'true', '2026-10-26'), ('2026-10-26 21', 'true', '')]:
                denied = env | {'FIXTURE_CLOCK': clock, 'IXIAOZU_MAINTENANCE_APPROVED': approved, 'IXIAOZU_MAINTENANCE_DATE': day}
                for command in [
                    ['bash', str(ROOT / 'deploy/guard-remote.sh'), 'touch', str(marker)],
                    ['bash', str(ROOT / 'deploy/deploy.sh'), 'production-ixiaozu', 'fixture', 'https://aigc.ixiaozu.cn/api/status'],
                    ['bash', str(ROOT / 'docs-site/deploy/deploy.sh'), 'production-ixiaozu'],
                ]:
                    with self.subTest(clock=clock, command=command[1]):
                        result = subprocess.run(command, env=denied, text=True, capture_output=True)
                        self.assertNotEqual(result.returncode, 0)
                        self.assertIn('maintenance', result.stderr.lower())
                        self.assertFalse(marker.exists())
                        self.assertFalse((fixture / 'ixiaozu').exists())
                        self.assertFalse((fixture / 'public').exists())
            for clock in ['2026-10-26 20', '2026-10-26 23']:
                result = subprocess.run(['bash', str(ROOT / 'deploy/guard-remote.sh'), 'touch', str(marker)], env=env | {'FIXTURE_CLOCK': clock}, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertTrue(marker.exists())
                marker.unlink()
            for target in ['development', 'production-molii', 'production-claudeye', 'production-model-claudeye']:
                result = subprocess.run(['bash', str(ROOT / 'deploy/guard-remote.sh'), 'touch', str(marker)], env=env | {'DEPLOY_ENVIRONMENT': target, 'FIXTURE_CLOCK': '2026-10-26 09'}, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                marker.unlink()
