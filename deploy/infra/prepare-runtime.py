#!/usr/bin/env python3
"""Prepare a fresh application directory, preserving existing infrastructure secrets."""
import argparse
import os
from pathlib import Path
import pwd
import re
import socket
import stat

SITES = {
    'production-claudeye': ('claudeye', 'claudeye.com', 'claudeye-production'),
    'production-model-claudeye': ('bwg-us-claudeye', 'model.claudeye.com', 'model-claudeye-production'),
}


def runtime_text(body, site):
    values = {}
    for line in body.splitlines():
        key, separator, value = line.partition('=')
        if not separator or key in values:
            raise ValueError('malformed or duplicate setting')
        values[key] = value
    if set(values) != {'SQL_DSN', 'REDIS_CONN_STRING', 'SESSION_SECRET', 'CRYPTO_SECRET'}:
        raise ValueError('unexpected settings')
    patterns = {
        'SQL_DSN': r'postgresql://new_api:[A-Za-z0-9_-]+@postgres:5432/new_api\?sslmode=disable',
        'REDIS_CONN_STRING': r'redis://:[A-Za-z0-9_-]+@redis:6379/0',
        'SESSION_SECRET': r'[a-f0-9]{64}',
        'CRYPTO_SECRET': r'[a-f0-9]{64}',
    }
    if any(not re.fullmatch(pattern, values[key]) for key, pattern in patterns.items()):
        raise ValueError('unexpected infrastructure credential format')
    _, domain, node = SITES[site]
    return body.rstrip('\n') + '\n\n' + '\n'.join([
        'TZ=Asia/Shanghai', 'ERROR_LOG_ENABLED=true', 'BATCH_UPDATE_ENABLED=true',
        'NODE_NAME=' + node, 'SESSION_COOKIE_SECURE=true',
        'SESSION_COOKIE_TRUSTED_URL=https://' + domain, '',
    ])


def write_tree(target, text, uid, gid):
    # Exclusive directory creation also rejects existing symlinks and partial runs.
    target.mkdir(mode=0o700)
    with (target / '.env.runtime').open('x') as output:
        os.fchmod(output.fileno(), 0o600)
        output.write(text)
        output.flush()
        os.fsync(output.fileno())
        os.fchown(output.fileno(), uid, gid)
    for name in ('data', 'logs', 'certs'):
        directory = target / name
        directory.mkdir(mode=0o750)
        os.chown(directory, uid, gid)
    os.chown(target, uid, gid)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--site', choices=SITES, required=True)
    args = parser.parse_args()
    if os.geteuid() != 0 or socket.gethostname() != SITES[args.site][0]:
        raise ValueError('wrong user or host')
    owner = pwd.getpwnam('claudeye-deploy')
    if owner.pw_uid == 0:
        raise ValueError('deployment account must not be root')
    parent = Path('/opt/claudeye')
    source = parent / 'infra/app.env'
    if source.resolve(strict=True) != source:
        raise ValueError('symlink source refused')
    for directory, private in ((parent, False), (source.parent, True)):
        info = directory.stat()
        if info.st_uid != 0 or info.st_mode & 0o022 or (private and stat.S_IMODE(info.st_mode) != 0o700):
            raise ValueError('unsafe parent permissions')
    os.umask(0o077)
    with os.fdopen(os.open(source, os.O_RDONLY | os.O_NOFOLLOW), 'r') as source_file:
        info = os.fstat(source_file.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or stat.S_IMODE(info.st_mode) != 0o600 or info.st_size > 16384:
            raise ValueError('unsafe source file')
        text = runtime_text(source_file.read(16385), args.site)
    write_tree(parent / 'production', text, owner.pw_uid, owner.pw_gid)
    print('PASS: fresh /opt/claudeye/production/.env.runtime prepared; mode600; owner=claudeye-deploy')
    print('Existing infrastructure secrets preserved. No containers started or permissions granted to Docker.')


if __name__ == '__main__':
    try:
        main()
    except Exception:
        raise SystemExit('STOP: runtime preparation incomplete or existing path refused; secrets suppressed. Preserve files and inspect locally; do not blindly rerun.')
