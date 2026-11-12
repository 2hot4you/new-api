#!/usr/bin/python3 -I
"""Fixed, root-owned deploy entrypoint for the existing Tencent Compose service."""

from contextlib import contextmanager
import fcntl
import grp
import json
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import tempfile
import time
from typing import NamedTuple


DIRECTORY = Path('/data/claudeye/production')
DOCKER_CONFIG = Path('/root/.docker')
DOCKER_SOCKET = Path('/run/docker.sock')
PROJECT = 'claudeye-production'
SERVICE = 'new-api'
CONTAINER = 'model-claudeye-tencent-production'
LOCAL_STATUS = 'http://127.0.0.1:3000/api/status'
PUBLIC_STATUS = 'https://model.claudeye.com/api/status'
IMAGE = r'ghcr\.io/2hot4you/new-api@sha256:[0-9a-f]{64}'
SHA = r'[0-9a-f]{40}'
CONTAINER_ID = r'[0-9a-f]{64}'
IMAGE_ID = 'sha256:' + CONTAINER_ID
MAX_CONFIG = 1024 * 1024
MAX_STATUS = 1024 * 1024
HEALTH_TIMEOUT = 120
SWITCH_SIGNALS = (signal.SIGHUP, signal.SIGINT, signal.SIGTERM)
LABELS = ('project', 'service', 'project.working_dir', 'project.config_files', 'oneoff')
IMAGE_FIELDS = '\n'.join((
    '{{json .Id}}', '{{json .RepoDigests}}',
    '{{json (index .Config.Labels "org.opencontainers.image.revision")}}',
    '{{json (index .Config.Labels "org.opencontainers.image.version")}}',
))
CONTAINER_FIELDS = '\n'.join((
    '{{json .Id}}', '{{json .Name}}', '{{json .Config.Image}}', '{{json .Image}}',
    '{{json .State.Running}}', '{{if .State.Health}}{{json .State.Health.Status}}{{else}}null{{end}}',
    *('{{json (index .Config.Labels "com.docker.compose.' + label + '")}}' for label in LABELS),
))
RELEASE = re.compile(rf'release ({IMAGE}) ({IMAGE}) ({SHA})')
SERVICES_LINE = re.compile(rb'^services: *(?:#[^\r\n]*)?\r?$', re.M)
SERVICE_LINE = re.compile(rb'^  new-api: *(?:#[^\r\n]*)?\r?$', re.M)
TOP_LEVEL_LINE = re.compile(rb'^[^ \t#\r\n]', re.M)
SERVICE_LEVEL_LINE = re.compile(rb'^ {1,3}[^ \t#\r\n]', re.M)
PIN_LINE = re.compile(rb'^    image: *(?P<quote>["\']?)(?P<pin>' + IMAGE.encode()
                      + rb')(?P=quote)(?: +#[^\r\n]*)? *\r?$', re.M)
NAME_LINE = re.compile(rb'^    container_name: *(?P<quote>["\']?)' + CONTAINER.encode()
                       + rb'(?P=quote)(?: +#[^\r\n]*)? *\r?$', re.M)


class DeployError(Exception):
    """Only fixed, safe messages cross the SSH boundary."""


class Interrupted(DeployError):
    pass


class Snapshot(NamedTuple):
    contents: bytes
    uid: int
    gid: int
    mode: int
    fingerprint: tuple

    def equivalent(self, other):
        return self[:4] == other[:4]


class ImageIdentity(NamedTuple):
    image_id: str
    revision: str


def app_version(revision):
    return 'production-model-claudeye-' + revision[:12]


def run_fixed(argv, *, timeout, capture):
    return subprocess.run(
        argv, check=True, timeout=timeout, stdin=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        stdout=subprocess.PIPE if capture else subprocess.DEVNULL, cwd='/',
        env={'PATH': '/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin', 'HOME': '/root',
             'LANG': 'C', 'COMPOSE_REMOVE_ORPHANS': 'false', 'COMPOSE_IGNORE_ORPHANS': 'true'},
    ).stdout


def pin_span(contents):
    """Locate the one literal pin under the supported, canonical service layout."""
    services = list(SERVICES_LINE.finditer(contents))
    pins = list(PIN_LINE.finditer(contents))
    names = list(NAME_LINE.finditer(contents))
    if len(services) != 1 or len(pins) != 1 or len(names) != 1:
        raise DeployError('unsupported or ambiguous Compose pin or container name')
    services_start = services[0].end()
    next_top_level = TOP_LEVEL_LINE.search(contents, services_start)
    services_end = next_top_level.start() if next_top_level else len(contents)
    headers = list(SERVICE_LINE.finditer(contents, services_start, services_end))
    if len(headers) != 1:
        raise DeployError('unsupported or ambiguous Compose service declaration')
    service_start = headers[0].end()
    next_service = SERVICE_LEVEL_LINE.search(contents, service_start, services_end)
    service_end = next_service.start() if next_service else services_end
    for key, literal in ((b'image', pins[0]), (b'container_name', names[0])):
        declarations = list(re.compile(rb'^    (?:' + key + rb'|"' + key + rb'"|\'' + key
                                       + rb'\') *:', re.M).finditer(contents, service_start, service_end))
        if len(declarations) != 1 or declarations[0].start() != literal.start():
            raise DeployError('unsupported or ambiguous Compose service identity')
    return pins[0].span('pin')


def fingerprint(info):
    return (info.st_dev, info.st_ino, info.st_uid, info.st_gid, info.st_mode, info.st_nlink,
            info.st_size, info.st_mtime_ns, info.st_ctime_ns)


class Host:
    def __init__(self, *, directory=DIRECTORY, owner=0, boundary=Path('/'), socket_path=DOCKER_SOCKET,
                 docker_config=DOCKER_CONFIG, execute=run_fixed, attempts=30, sleep=time.sleep):
        self.directory = directory
        self.compose_file = directory / 'compose.yaml'
        self.env_file = directory / '.env'
        self.owner = owner
        self.boundary = boundary
        self.socket_path = socket_path
        self.docker_config = docker_config
        self.execute = execute
        self.attempts = attempts
        self.sleep = sleep
        self.health_deadline = None

    def timeout(self, limit):
        if self.health_deadline is None:
            return limit
        remaining = self.health_deadline - time.monotonic()
        if remaining <= 0:
            raise DeployError('application health or version verification timed out')
        return min(limit, remaining)

    def trusted_parents(self, directory):
        if not directory.is_absolute() or '..' in directory.parts:
            raise DeployError('untrusted host directory')
        try:
            relative = directory.relative_to(self.boundary)
        except ValueError:
            raise DeployError('untrusted host directory') from None
        for candidate in (self.boundary, *(self.boundary.joinpath(*relative.parts[:i])
                                           for i in range(1, len(relative.parts) + 1))):
            info = candidate.lstat()
            if not stat.S_ISDIR(info.st_mode) or info.st_uid != self.owner or info.st_mode & 0o022:
                raise DeployError('untrusted host directory')

    def trusted_file(self, info, *, private=False):
        if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_uid != self.owner
                or info.st_mode & (0o7077 if private else 0o7022)):
            raise DeployError('untrusted host file owner, mode or type')

    def check_socket(self):
        self.trusted_parents(self.socket_path.parent)
        info = self.socket_path.lstat()
        if not stat.S_ISSOCK(info.st_mode) or info.st_uid != self.owner or info.st_mode & 0o7007:
            raise DeployError('untrusted local Docker socket')
        if self.socket_path == DOCKER_SOCKET:
            groups = {0}
            try:
                groups.add(grp.getgrnam('docker').gr_gid)
            except KeyError:
                pass
            if info.st_gid not in groups:
                raise DeployError('untrusted local Docker socket group')

    @contextmanager
    def lock(self):
        if os.geteuid() != self.owner:
            raise DeployError('deployment entrypoint requires root')
        self.trusted_parents(self.directory)
        fd = os.open(self.directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            path_info, fd_info = self.directory.lstat(), os.fstat(fd)
            if (path_info.st_dev, path_info.st_ino) != (fd_info.st_dev, fd_info.st_ino):
                raise DeployError('deployment directory changed before locking')
            try:
                fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                raise DeployError('another deployment holds the lock') from None
            self.check_socket()
            self.trusted_parents(self.docker_config)
            self.trusted_file((self.docker_config / 'config.json').lstat(), private=True)
            yield fd
        finally:
            os.close(fd)

    def snapshot(self):
        self.trusted_file(self.compose_file.lstat())
        fd = os.open(self.compose_file, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        with os.fdopen(fd, 'rb') as source:
            info = os.fstat(source.fileno())
            self.trusted_file(info)
            contents = source.read(MAX_CONFIG + 1)
            signature = fingerprint(info)
            if len(contents) > MAX_CONFIG or fingerprint(self.compose_file.lstat()) != signature:
                raise DeployError('Compose file is too large or changed while reading')
        return Snapshot(contents, info.st_uid, info.st_gid, stat.S_IMODE(info.st_mode), signature)

    @contextmanager
    def candidate(self, snapshot):
        fd, name = tempfile.mkstemp(prefix='.model-claudeye-', suffix='.yaml', dir=self.directory)
        path = Path(name)
        try:
            with os.fdopen(fd, 'wb') as target:
                info = os.fstat(target.fileno())
                if (info.st_uid, info.st_gid) != (snapshot.uid, snapshot.gid):
                    os.fchown(target.fileno(), snapshot.uid, snapshot.gid)
                os.fchmod(target.fileno(), snapshot.mode)
                target.write(snapshot.contents)
                target.flush()
                os.fsync(target.fileno())
            yield path
        finally:
            path.unlink(missing_ok=True)

    def docker(self, *args, timeout=15, capture=True, failure, before_execute=None):
        argv = ['/usr/bin/docker', '--host', 'unix:///run/docker.sock', '--config',
                str(self.docker_config), *args]
        timeout = self.timeout(timeout)
        try:
            if before_execute is not None:
                before_execute()
            return self.execute(argv, timeout=timeout, capture=capture)
        except (OSError, subprocess.SubprocessError):
            raise DeployError(failure) from None

    def compose(self, path, *args, **kwargs):
        try:
            info = self.env_file.lstat()
        except FileNotFoundError:
            env_file = '/dev/null'
        else:
            self.trusted_file(info)
            env_file = str(self.env_file)
        return self.docker('compose', '--project-directory', str(self.directory), '--project-name', PROJECT,
                           '--env-file', env_file, '--file', str(path), *args, **kwargs)

    def verify_config(self, path, pin):
        # Resolve required service env files, without receiving rendered values or diagnostics.
        self.compose(path, 'config', '--quiet', capture=False,
                     failure='Compose configuration or a required service env file is invalid')
        # Only request narrow identity projections, not rendered JSON/YAML or environment output.
        for flag, expected in (('--services', SERVICE), ('--images', pin)):
            output = self.compose(path, 'config', '--no-env-resolution', flag,
                                  failure='cannot validate Compose service and image')
            if output.splitlines() != [expected.encode()]:
                raise DeployError('rendered Compose service or image does not match the fixed target')

    def image_identity(self, pin):
        fields = self.docker('image', 'inspect', '--format', IMAGE_FIELDS, pin,
                             failure='cannot inspect the pinned local image')
        image_id, digests, revision, version = parse_fields(fields, 4, 'invalid local image metadata')
        if (not isinstance(image_id, str) or not re.fullmatch(IMAGE_ID, image_id) or not isinstance(digests, list)
                or pin not in digests or not isinstance(revision, str) or not re.fullmatch(SHA, revision)):
            raise DeployError('local image digest, ID or OCI revision does not match')
        if version != app_version(revision):
            raise DeployError('local image OCI version does not match the model site')
        return ImageIdentity(image_id, revision)

    def running_container(self, pin, image):
        fields = self.docker('container', 'inspect', '--format', CONTAINER_FIELDS, CONTAINER,
                             failure='cannot inspect the fixed running container')
        container_id, name, configured_image, image_id, running, health, *labels = parse_fields(
            fields, 6 + len(LABELS), 'invalid running container metadata')
        expected = [PROJECT, SERVICE, str(self.directory), str(self.compose_file), 'False']
        if (not isinstance(container_id, str) or not re.fullmatch(CONTAINER_ID, container_id)
                or name != '/' + CONTAINER or configured_image != pin or image_id != image.image_id
                or running is not True or health != 'healthy' or labels != expected):
            raise DeployError('running container identity, image or health does not match')
        listed = self.compose(self.compose_file, 'ps', '--all', '--quiet', '--no-trunc', SERVICE,
                              failure='cannot resolve the fixed Compose service container')
        if listed.splitlines() != [container_id.encode()]:
            raise DeployError('Compose service does not identify exactly the expected container')
        return container_id

    def check_status(self, url, revision):
        protocol = '=https' if url == PUBLIC_STATUS else '=http'
        argv = ['/usr/bin/curl', '--disable', '--silent', '--fail', '--noproxy', '*', '--proto', protocol,
                '--connect-timeout', '3', '--max-time', '6', '--max-filesize', str(MAX_STATUS),
                '--header', 'Cache-Control: no-cache, no-store', '--header', 'Pragma: no-cache', '--', url]
        try:
            response = self.execute(argv, timeout=self.timeout(8), capture=True)
            if len(response) > MAX_STATUS:
                raise DeployError('application status response is too large')
            value = json.loads(response)
            if value['success'] is True and value['data']['version'] == app_version(revision):
                return
        except (OSError, subprocess.SubprocessError, ValueError, KeyError, TypeError, RecursionError):
            pass
        raise DeployError('local or public application health or version does not match')

    def verify_running(self, pin, image):
        container_id = self.running_container(pin, image)
        self.check_status(LOCAL_STATUS, image.revision)
        self.check_status(PUBLIC_STATUS, image.revision)
        if self.running_container(pin, image) != container_id:
            raise DeployError('running container changed during application verification')
        return container_id

    def wait_for_running(self, pin, image, *, previous_id=None):
        self.health_deadline = time.monotonic() + HEALTH_TIMEOUT
        try:
            for attempt in range(self.attempts):
                try:
                    if self.verify_running(pin, image) == previous_id:
                        raise DeployError('release did not recreate the fixed container')
                    return
                except Interrupted:
                    raise
                except DeployError:
                    remaining = self.health_deadline - time.monotonic()
                    if attempt + 1 == self.attempts or remaining <= 0:
                        raise
                    self.sleep(min(3, remaining))
        finally:
            self.health_deadline = None

    def current(self, snapshot, expected=None):
        start, end = pin_span(snapshot.contents)
        pin = snapshot.contents[start:end].decode('ascii')
        if expected is not None and pin != expected:
            raise DeployError('existing Compose image does not match the preflight pin')
        self.verify_config(self.compose_file, pin)
        image = self.image_identity(pin)
        container = self.verify_running(pin, image)
        if self.snapshot() != snapshot:
            raise DeployError('Compose file changed during verification')
        return pin, image, container

    def up(self, *, before_execute=None):
        self.compose(self.compose_file, 'up', '-d', '--no-deps', '--pull', 'never', '--no-build',
                     '--force-recreate', SERVICE, timeout=120, capture=False,
                     failure='could not recreate the fixed Compose service', before_execute=before_execute)

    def rollback(self, directory_fd, snapshot, replacement, old_pin, old_image, *, unchanged_container_id):
        # Never overwrite unrelated changes. A failed restoration must not run up on the new pin.
        restored = self.snapshot()
        if not restored.equivalent(snapshot):
            if not restored.equivalent(replacement):
                raise DeployError('Compose file was changed outside the deployment')
            with self.candidate(snapshot) as candidate:
                self.verify_config(candidate, old_pin)
                if self.snapshot() != restored:
                    raise DeployError('Compose file changed before rollback')
                os.replace(candidate, self.compose_file)
                os.fsync(directory_fd)
        self.verify_config(self.compose_file, old_pin)
        if not self.snapshot().equivalent(snapshot) or self.image_identity(old_pin) != old_image:
            raise DeployError('rollback file or cached previous image does not match')
        if unchanged_container_id is None:
            self.up()
            self.wait_for_running(old_pin, old_image)
        elif self.verify_running(old_pin, old_image) != unchanged_container_id:
            raise DeployError('original container changed before deployment started')
        if not self.snapshot().equivalent(snapshot):
            raise DeployError('rollback file changed during verification')

    def release(self, directory_fd, snapshot, old_pin, new_pin, revision, report):
        _, old_image, previous_container = self.current(snapshot, old_pin)
        self.docker('image', 'pull', new_pin, timeout=240, capture=False,
                    failure='could not pull the requested immutable image')
        new_image = self.image_identity(new_pin)
        if new_image.revision != revision or new_image.image_id == old_image.image_id:
            raise DeployError('requested image does not match the full source SHA or is the previous image')
        start, end = pin_span(snapshot.contents)
        replacement = snapshot._replace(contents=snapshot.contents[:start] + new_pin.encode() + snapshot.contents[end:])
        with self.candidate(replacement) as candidate:
            self.verify_config(candidate, new_pin)
            _, current_image, current_container = self.current(snapshot, old_pin)
            if (current_image, current_container) != (old_image, previous_container):
                raise DeployError('previous image or container changed during preparation')
            replaced = False
            up_attempted = False

            def mark_up_attempted():
                # Once execution is possible, failure cannot prove Docker did nothing.
                nonlocal up_attempted
                up_attempted = True

            try:
                # Disconnect/termination must not split replace from recording the rollback obligation.
                with blocked_switch_signals():
                    os.replace(candidate, self.compose_file)
                    replaced = True
                os.fsync(directory_fd)
                if not self.snapshot().equivalent(replacement):
                    raise DeployError('installed Compose file does not match the validated candidate')
                self.up(before_execute=mark_up_attempted)
                self.wait_for_running(new_pin, new_image, previous_id=previous_container)
                if not self.snapshot().equivalent(replacement):
                    raise DeployError('installed Compose file changed during verification')
            except Exception:
                if replaced:
                    # A second disconnect/termination cannot interrupt a bounded rollback. SIGKILL can.
                    with signal_handlers(signal.SIG_IGN):
                        report['rollback'] = 'failed'
                        try:
                            self.rollback(directory_fd, snapshot, replacement, old_pin, old_image,
                                          unchanged_container_id=None if up_attempted else previous_container)
                        except Exception as rollback_error:
                            print('rollback verification failed: ' + safe_message(rollback_error), file=sys.stderr)
                        else:
                            report['rollback'] = 'succeeded'
                raise


@contextmanager
def blocked_switch_signals():
    previous = signal.pthread_sigmask(signal.SIG_BLOCK, SWITCH_SIGNALS)
    try:
        yield
    finally:
        signal.pthread_sigmask(signal.SIG_SETMASK, previous)


@contextmanager
def signal_handlers(handler):
    with blocked_switch_signals():
        previous = {sig: signal.signal(sig, handler) for sig in SWITCH_SIGNALS}
    try:
        yield
    finally:
        with blocked_switch_signals():
            for sig, disposition in previous.items():
                signal.signal(sig, disposition)


def parse_fields(payload, length, message):
    try:
        values = [json.loads(line) for line in payload.splitlines()]
        if len(values) == length:
            return values
    except (ValueError, TypeError, RecursionError):
        pass
    raise DeployError(message)


def safe_message(error):
    if isinstance(error, DeployError):
        return str(error)
    if isinstance(error, OSError):
        return 'host file operation failed'
    return 'unexpected host deployment failure'


def dispatch(arguments, host):
    if len(arguments) != 1:
        print('invalid deployment command', file=sys.stderr)
        return 2
    command = arguments[0]
    release = RELEASE.fullmatch(command)
    if command != 'preflight' and (not release or release[1] == release[2]):
        print('invalid deployment command', file=sys.stderr)
        return 2
    report = {'rollback': 'not_attempted'}
    try:
        with host.lock() as directory_fd:
            snapshot = host.snapshot()
            if release:
                host.release(directory_fd, snapshot, *release.groups(), report)
            else:
                pin, _, _ = host.current(snapshot)
                print('MODEL_CLAUDEYE_PREVIOUS_IMAGE=' + pin)
        return 0
    except Exception as error:
        print(safe_message(error), file=sys.stderr)
        return 1
    finally:
        if release:
            print('DEPLOY_ROLLBACK_RESULT=' + report['rollback'])


def stop(_signum, _frame):
    raise Interrupted('deployment interrupted')


if __name__ == '__main__':
    with signal_handlers(stop):
        sys.exit(dispatch(sys.argv[1:], Host()))
