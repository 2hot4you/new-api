import contextlib
import fcntl
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import stat
import subprocess
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / 'model-claudeye-tencent.py'
OLD = 'ghcr.io/2hot4you/new-api@sha256:' + 'a' * 64
NEW = 'ghcr.io/2hot4you/new-api@sha256:' + 'b' * 64
OLD_SHA, NEW_SHA = 'c' * 40, 'd' * 40
OLD_ID, NEW_ID = 'sha256:' + '1' * 64, 'sha256:' + '2' * 64
CONTAINER = 'model-claudeye-tencent-production'
LOCAL = 'http://127.0.0.1:3000/api/status'
PUBLIC = 'https://model.claudeye.com/api/status'
FIXTURE = (
    b'# The bytes, including these line endings, must survive.\r\n'
    b'name: claudeye-production\r\nservices:\r\n  new-api:\r\n'
    b'    container_name: model-claudeye-tencent-production\r\n'
    b'    image: "' + OLD.encode() + b'" # previous pin\r\n'
    b'    environment:\r\n      FIXTURE_SETTING: synthetic-sensitive-value\r\n'
    b'    volumes: ["./data:/data"]\r\n'
)


class DockerFixture:
    """Fake only Docker/HTTP I/O; the entrypoint edits real temporary files."""

    def __init__(self, directory):
        self.directory = directory
        self.calls = []
        self.image = OLD
        self.revisions = {OLD: OLD_SHA, NEW: NEW_SHA}
        self.oci_version_overrides = {}
        self.old_container_id = '3' * 64
        self.up = []
        self.bad_versions = set()
        self.override_labels = {}
        self.bad_candidate = False
        self.pull_fails = False
        self.container_image_id = None
        self.listed_id = None
        self.healthy = True
        self.failed_ups = set()

    @property
    def container_id(self):
        return self.old_container_id if self.image == OLD else '4' * 64

    def __call__(self, argv, *, timeout, capture):
        self.calls.append((argv, timeout, capture))
        if argv[0] == '/usr/bin/curl':
            url = argv[-1]
            if url not in (LOCAL, PUBLIC):
                raise AssertionError(argv)
            revision = self.revisions[self.image][:12]
            if (self.image, url) in self.bad_versions:
                revision += 'bad'
            return json.dumps({'success': True, 'data': {
                'version': 'production-model-claudeye-' + revision,
            }}).encode()
        if argv[:3] != ['/usr/bin/docker', '--host', 'unix:///run/docker.sock']:
            raise AssertionError(argv)
        if '--config' not in argv:
            raise AssertionError(argv)
        command = argv[5:]
        if command[0] == 'compose':
            self.assert_compose_target(command)
            filename = Path(command[command.index('--file') + 1])
            contents = filename.read_bytes()
            pin = re.search(rb'ghcr.io/2hot4you/new-api@sha256:[a-f0-9]{64}', contents)[0].decode()
            if 'config' in command:
                if '--quiet' in command and '--no-env-resolution' not in command and not capture:
                    return b''
                if '--services' in command:
                    return b'new-api\n' if not self.bad_candidate or pin == OLD else b'new-api\nother\n'
                if '--images' in command:
                    return pin.encode() + b'\n'
            if command[-5:] == ['ps', '--all', '--quiet', '--no-trunc', 'new-api']:
                return (self.listed_id or self.container_id).encode() + b'\n'
            if command[-8:] == ['up', '-d', '--no-deps', '--pull', 'never', '--no-build',
                                '--force-recreate', 'new-api']:
                if filename != self.directory / 'compose.yaml':
                    raise AssertionError('up must use the canonical path')
                self.up.append(pin)
                if pin in self.failed_ups:
                    raise subprocess.CalledProcessError(2, argv, stderr=b'synthetic-sensitive-value')
                self.image = pin
                return b''
        if command[:2] == ['image', 'inspect']:
            image = command[-1]
            image_id = OLD_ID if image == OLD else NEW_ID
            data = [image_id, [image], self.revisions[image]]
            if 'org.opencontainers.image.version' in command[3]:
                data.append(self.oci_version_overrides.get(
                    image, 'production-model-claudeye-' + self.revisions[image][:12]))
            return b'\n'.join(json.dumps(value).encode() for value in data) + b'\n'
        if command[:2] == ['container', 'inspect'] and command[-1] == CONTAINER:
            labels = {
                'com.docker.compose.project': 'claudeye-production',
                'com.docker.compose.service': 'new-api',
                'com.docker.compose.project.working_dir': str(self.directory),
                'com.docker.compose.project.config_files': str(self.directory / 'compose.yaml'),
                'com.docker.compose.oneoff': 'False',
            }
            labels.update(self.override_labels)
            data = [self.container_id, '/' + CONTAINER, self.image, self.container_image_id or (
                OLD_ID if self.image == OLD else NEW_ID), True, 'healthy' if self.healthy else 'unhealthy',
                *labels.values()]
            return b'\n'.join(json.dumps(value).encode() for value in data) + b'\n'
        if command[:2] == ['image', 'pull'] and command[-1] == NEW:
            if self.pull_fails:
                raise subprocess.CalledProcessError(2, argv, stderr=b'synthetic-sensitive-value')
            return b''
        raise AssertionError(argv)

    def assert_compose_target(self, command):
        if command[:5] != ['compose', '--project-directory', str(self.directory),
                            '--project-name', 'claudeye-production']:
            raise AssertionError(command)


class TencentHostTest(unittest.TestCase):
    def setUp(self):
        if not SCRIPT.is_file():
            self.fail('the fixed host deployment entrypoint has not been implemented')
        spec = importlib.util.spec_from_file_location('tencent_host', SCRIPT)
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        work = tempfile.TemporaryDirectory()
        self.addCleanup(work.cleanup)
        self.boundary = Path(work.name).resolve()
        self.directory = self.boundary / 'production'
        self.directory.mkdir(mode=0o750)
        self.compose = self.directory / 'compose.yaml'
        self.compose.write_bytes(FIXTURE)
        self.compose.chmod(0o640)
        # A live Unix socket fixture exercises lstat policy without contacting Docker.
        self.socket_path = self.boundary / 'docker.sock'
        self.socket = socket.socket(socket.AF_UNIX)
        self.addCleanup(self.socket.close)
        self.socket.bind(str(self.socket_path))
        self.socket_path.chmod(0o660)
        self.docker_config = self.boundary / 'docker-config'
        self.docker_config.mkdir(mode=0o700)
        (self.docker_config / 'config.json').write_text('{}\n')
        (self.docker_config / 'config.json').chmod(0o600)
        self.fake = DockerFixture(self.directory)
        self.host = self.module.Host(directory=self.directory, owner=os.getuid(), boundary=self.boundary,
                                     socket_path=self.socket_path, docker_config=self.docker_config,
                                     execute=self.fake, attempts=2,
                                     sleep=lambda seconds: None)

    def invoke(self, command=None, extra=()):
        if command is None:
            command = f'release {OLD} {NEW} {NEW_SHA}'
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = self.module.dispatch([command, *extra], self.host)
        self.assertNotIn('synthetic-sensitive-value', out.getvalue() + err.getvalue())
        return code, out.getvalue(), err.getvalue()

    def assert_no_write(self):
        self.assertEqual(self.compose.read_bytes(), FIXTURE)
        self.assertEqual(list(self.directory.iterdir()), [self.compose])
        self.assertEqual(self.fake.up, [])

    def test_protocol_rejects_arbitrary_shell_and_argv_before_io(self):
        for command, extra in [('preflight\n', ()), (' preflight', ()), ('preflight', ('extra',)),
                               ('preflight; id', ()), (f'release {OLD} {NEW}:tag {NEW_SHA}', ()),
                               (f'release {OLD} {NEW} {NEW_SHA.upper()}', ()),
                               (f'release {OLD}\t{NEW} {NEW_SHA}', ()),
                               (f'release {OLD} {OLD} {NEW_SHA}', ()),
                               (f'release {OLD} {NEW} {NEW_SHA}; id', ())]:
            with self.subTest(command=command):
                self.assertNotEqual(self.invoke(command, extra)[0], 0)
        self.assertEqual(self.fake.calls, [])
        self.assert_no_write()

    def test_preflight_is_read_only_and_returns_exact_previous_pin(self):
        inode = self.compose.stat().st_ino
        self.assertEqual(self.invoke('preflight'), (0, f'MODEL_CLAUDEYE_PREVIOUS_IMAGE={OLD}\n', ''))
        self.assertEqual(self.compose.stat().st_ino, inode)
        self.assert_no_write()

    def test_rejects_unsafe_parent_or_file_permissions_before_docker(self):
        for path in (self.directory, self.compose, self.socket_path):
            with self.subTest(path=path):
                mode = stat.S_IMODE(path.stat().st_mode)
                path.chmod(mode | 0o002)
                try:
                    self.assertNotEqual(self.invoke('preflight')[0], 0)
                finally:
                    path.chmod(mode)
        self.assertEqual(self.fake.calls, [])
        self.assert_no_write()

    def test_rejects_symlink_and_nonregular_compose_without_following(self):
        other = self.boundary / 'foreign.yaml'
        self.compose.rename(other)
        self.compose.symlink_to(other)
        self.assertNotEqual(self.invoke('preflight')[0], 0)
        self.compose.unlink()
        os.mkfifo(self.compose, mode=0o600)
        self.assertNotEqual(self.invoke('preflight')[0], 0)
        self.assertEqual(other.read_bytes(), FIXTURE)
        self.assertEqual(self.fake.calls, [])

    def test_rejects_missing_docker_socket_before_docker(self):
        self.socket_path.unlink()
        self.socket_path.touch()
        self.assertNotEqual(self.invoke('preflight')[0], 0)
        self.assertEqual(self.fake.calls, [])
        self.assert_no_write()

    def test_existing_directory_lock_fails_before_docker(self):
        descriptor = os.open(self.directory, os.O_RDONLY)
        try:
            fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
            self.assertNotEqual(self.invoke('preflight')[0], 0)
            self.assertIn('not_attempted', self.invoke()[1])
        finally:
            os.close(descriptor)
        self.assertEqual(self.fake.calls, [])
        self.assert_no_write()

    def test_refuses_mismatched_compare_and_swap_and_old_container_identity(self):
        self.assertIn('not_attempted', self.invoke(f'release {NEW} {OLD} {NEW_SHA}')[1])
        for name, value in [('container_image_id', NEW_ID), ('listed_id', '5' * 64), ('healthy', False)]:
            with self.subTest(name=name):
                old = getattr(self.fake, name)
                setattr(self.fake, name, value)
                self.assertNotEqual(self.invoke('preflight')[0], 0)
                setattr(self.fake, name, old)
        for label in ('project', 'service', 'project.config_files', 'project.working_dir', 'oneoff'):
            with self.subTest(label=label):
                self.fake.override_labels = {'com.docker.compose.' + label: 'wrong'}
                self.assertNotEqual(self.invoke('preflight')[0], 0)
        self.assert_no_write()

    def test_release_changes_only_pin_and_single_service(self):
        self.assertEqual(self.invoke(), (0, 'DEPLOY_ROLLBACK_RESULT=not_attempted\n', ''))
        self.assertEqual(self.compose.read_bytes(), FIXTURE.replace(OLD.encode(), NEW.encode()))
        self.assertEqual(stat.S_IMODE(self.compose.stat().st_mode), 0o640)
        self.assertEqual(list(self.directory.iterdir()), [self.compose])
        self.assertEqual(self.fake.up, [NEW])

    def test_rejects_multiple_pin_lines_before_docker(self):
        self.compose.write_bytes(FIXTURE + b'    image: ' + NEW.encode() + b'\r\n')
        self.assertNotEqual(self.invoke('preflight')[0], 0)
        self.assertEqual(self.fake.calls, [])

    def test_rejects_candidate_wrong_identity_before_atomic_change(self):
        self.fake.bad_candidate = True
        self.assertNotEqual(self.invoke()[0], 0)
        self.assert_no_write()

    def test_refuses_pull_failure_or_oci_sha_mismatch_without_rebuild(self):
        self.fake.pull_fails = True
        self.assertIn('not_attempted', self.invoke()[1])
        self.fake.pull_fails = False
        self.fake.revisions[NEW] = NEW_SHA[:12] + 'e' * 28
        self.assertIn('not_attempted', self.invoke()[1])
        self.assert_no_write()

    def test_refuses_same_revision_sibling_site_image_before_switch(self):
        self.fake.oci_version_overrides[NEW] = 'production-claudeye-' + NEW_SHA[:12]
        inode = self.compose.stat().st_ino
        code, out, err = self.invoke()
        self.assertNotEqual(code, 0)
        self.assertEqual(out, 'DEPLOY_ROLLBACK_RESULT=not_attempted\n')
        self.assertIn('OCI version', err)
        self.assertEqual(self.compose.stat().st_ino, inode)
        self.assert_no_write()

    def test_preflight_refuses_previous_image_without_model_site_oci_version(self):
        self.fake.oci_version_overrides[OLD] = None
        code, out, err = self.invoke('preflight')
        self.assertNotEqual(code, 0)
        self.assertEqual(out, '')
        self.assertIn('OCI version', err)
        self.assert_no_write()

    def test_public_new_version_mismatch_restores_and_verifies_old(self):
        self.fake.bad_versions.add((NEW, PUBLIC))
        code, stdout, stderr = self.invoke()
        self.assertNotEqual(code, 0)
        self.assertEqual(stdout, 'DEPLOY_ROLLBACK_RESULT=succeeded\n')
        self.assertIn('health or version', stderr)
        self.assertEqual(self.compose.read_bytes(), FIXTURE)
        self.assertEqual(stat.S_IMODE(self.compose.stat().st_mode), 0o640)
        self.assertEqual(self.fake.up, [NEW, OLD])
        self.assertEqual([argv[-1] for argv, _, _ in self.fake.calls if argv[0] == '/usr/bin/curl'][-2:],
                         [LOCAL, PUBLIC])

    def test_rollback_up_failure_does_not_claim_success(self):
        self.fake.bad_versions.add((NEW, LOCAL))
        self.fake.failed_ups.add(OLD)
        code, stdout, stderr = self.invoke()
        self.assertNotEqual(code, 0)
        self.assertEqual(stdout, 'DEPLOY_ROLLBACK_RESULT=failed\n')
        self.assertIn('health or version', stderr)
        self.assertIn('rollback', stderr)
        self.assertEqual(self.compose.read_bytes(), FIXTURE)
        self.assertEqual(self.fake.up, [NEW, OLD])

    def test_rejects_external_pin_drift_after_pull_without_overwriting(self):
        changed = FIXTURE + b'# external operator edit\r\n'

        def change_after_pull(argv, *, timeout, capture):
            result = self.fake(argv, timeout=timeout, capture=capture)
            if argv[5:7] == ['image', 'pull']:
                self.compose.write_bytes(changed)
            return result

        self.host.execute = change_after_pull
        self.assertIn('not_attempted', self.invoke()[1])
        self.assertEqual(self.compose.read_bytes(), changed)
        self.assertEqual(list(self.directory.iterdir()), [self.compose])
        self.assertEqual(self.fake.up, [])

    def test_rollback_does_not_overwrite_external_edit_after_switch(self):
        changed = FIXTURE.replace(OLD.encode(), NEW.encode()) + b'# external operator edit\r\n'
        self.fake.bad_versions.add((NEW, PUBLIC))

        def change_after_up(argv, *, timeout, capture):
            result = self.fake(argv, timeout=timeout, capture=capture)
            if argv[5] == 'compose' and 'up' in argv:
                self.compose.write_bytes(changed)
            return result

        self.host.execute = change_after_up
        self.assertIn('DEPLOY_ROLLBACK_RESULT=failed', self.invoke()[1])
        self.assertEqual(self.compose.read_bytes(), changed)
        self.assertEqual(self.fake.up, [NEW])

    def test_failure_fsyncing_committed_file_restores_previous_config(self):
        fsync = os.fsync
        failed = False

        def fail_once_after_replace(descriptor):
            nonlocal failed
            if stat.S_ISDIR(os.fstat(descriptor).st_mode) and not failed:
                failed = True
                self.assertIn(NEW.encode(), self.compose.read_bytes())
                raise OSError('synthetic-sensitive-value')
            fsync(descriptor)

        with patch.object(self.module.os, 'fsync', fail_once_after_replace):
            self.assertIn('DEPLOY_ROLLBACK_RESULT=succeeded', self.invoke()[1])
        self.assertTrue(failed)
        self.assertEqual(self.compose.read_bytes(), FIXTURE)
        self.assertEqual(self.fake.up, [])
        self.assertEqual([argv[-1] for argv, _, _ in self.fake.calls if argv[0] == '/usr/bin/curl'],
                         [LOCAL, PUBLIC] * 3)

    def test_pre_up_rollback_rejects_unexpected_original_container_id(self):
        fsync = os.fsync

        def change_container_after_replace(descriptor):
            if stat.S_ISDIR(os.fstat(descriptor).st_mode) and NEW.encode() in self.compose.read_bytes():
                self.fake.old_container_id = '5' * 64
                raise OSError('synthetic-sensitive-value')
            fsync(descriptor)

        with patch.object(self.module.os, 'fsync', change_container_after_replace):
            self.assertIn('DEPLOY_ROLLBACK_RESULT=failed', self.invoke()[1])
        self.assertEqual(self.compose.read_bytes(), FIXTURE)
        self.assertEqual(self.fake.up, [])

    def test_disconnect_after_replace_rolls_back_even_with_second_signal(self):
        replace = os.replace

        def disconnect(source, dest):
            replace(source, dest)
            signal.raise_signal(signal.SIGHUP)

        handler = signal.signal(signal.SIGHUP, self.module.stop)
        try:
            with patch.object(self.module.os, 'replace', disconnect):
                code, out, err = self.invoke()
        finally:
            signal.signal(signal.SIGHUP, handler)
        self.assertNotEqual(code, 0)
        self.assertEqual(out, 'DEPLOY_ROLLBACK_RESULT=succeeded\n')
        self.assertIn('deployment interrupted', err)
        self.assertEqual(self.compose.read_bytes(), FIXTURE)
        self.assertEqual(self.fake.up, [])

    def test_failed_candidate_up_is_treated_as_potentially_started(self):
        self.fake.failed_ups.add(NEW)
        code, out, _ = self.invoke()
        self.assertNotEqual(code, 0)
        self.assertEqual(out, 'DEPLOY_ROLLBACK_RESULT=succeeded\n')
        self.assertEqual(self.compose.read_bytes(), FIXTURE)
        self.assertEqual(self.fake.up, [NEW, OLD])

    def use_real_compose_config(self):
        docker = shutil.which('docker')
        if not docker or subprocess.run([docker, 'compose', 'version'], stdout=subprocess.DEVNULL,
                                        stderr=subprocess.DEVNULL, timeout=10).returncode != 0:
            self.skipTest('Docker Compose CLI unavailable')

        def compose_only(argv, *, timeout, capture):
            if argv[5] == 'compose' and 'config' in argv:
                # The absent local endpoint proves that rendering never needs a daemon.
                return subprocess.run([docker, '--host', 'unix:///no-such-compose-test.sock', *argv[5:]],
                                      check=True, timeout=timeout, stderr=subprocess.DEVNULL,
                                      stdout=subprocess.PIPE if capture else subprocess.DEVNULL,
                                      cwd=self.directory).stdout
            return self.fake(argv, timeout=timeout, capture=capture)

        self.host.execute = compose_only

    def test_real_compose_renders_old_new_and_restored_identity_without_daemon(self):
        self.use_real_compose_config()
        self.fake.bad_versions.add((NEW, PUBLIC))
        self.assertIn('succeeded', self.invoke()[1])
        self.assertEqual(self.compose.read_bytes(), FIXTURE)

    def test_missing_required_service_env_file_refuses_release_before_writes(self):
        self.use_real_compose_config()
        contents = FIXTURE + b'    env_file: [./synthetic-sensitive-value.env]\r\n'
        self.compose.write_bytes(contents)
        inode = self.compose.stat().st_ino
        code, out, _ = self.invoke()
        self.assertNotEqual(code, 0)
        self.assertEqual(out, 'DEPLOY_ROLLBACK_RESULT=not_attempted\n')
        self.assertEqual(self.compose.read_bytes(), contents)
        self.assertEqual(self.compose.stat().st_ino, inode)
        self.assertEqual(list(self.directory.iterdir()), [self.compose])
        self.assertEqual(self.fake.up, [])


if __name__ == '__main__':
    unittest.main()
