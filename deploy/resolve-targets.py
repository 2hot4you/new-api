#!/usr/bin/env python3
"""Resolve allowed deployment targets and a single immutable source commit."""
import json
import os
import re
import subprocess
import sys

PROFILES = {
    'development': 'molii',
    'production-molii': 'molii',
    'production-ixiaozu': 'ixiaozu',
    'production-claudeye': 'claudeye',
    'production-model-claudeye': 'claudeye',
}
# Expansion deliberately excludes the new sites until production acceptance.
ALL_PRODUCTION = ['production-molii', 'production-ixiaozu']


def resolve(env):
    event = env.get('GITHUB_EVENT_NAME', '')
    branch = env.get('GITHUB_REF', '')
    sha = env.get('GITHUB_SHA', '')
    target = env.get('DEPLOY_TARGET', '')
    source = env.get('SOURCE_REF', '')
    if event not in ('push', 'workflow_dispatch'):
        raise ValueError('Unsupported deployment event')
    if branch not in ('refs/heads/main', 'refs/heads/develop'):
        raise ValueError('Deployment workflow must run on main or develop branch')
    if not re.fullmatch(r'[0-9a-f]{40}', sha):
        raise ValueError('Triggering source must be a full commit SHA')
    production = branch == 'refs/heads/main'
    if event == 'push':
        if target or source:
            raise ValueError('Push cannot override target or source')
        target = 'all-production' if production else 'development'
    if target not in (*PROFILES, 'all-production'):
        raise ValueError('Unsupported deployment target')
    if (target != 'development') != production:
        raise ValueError('Production targets require main; development requires develop')
    if production:
        if source and source != sha:
            raise ValueError('Production source must equal the triggering main commit SHA')
        if any(env.get(name, '').lower() == 'true' for name in ('BACKUP_POSTGRES', 'VERIFY_REPEATED_STARTUP')):
            raise ValueError('Development-only controls cannot target production')
    elif source:
        if source.startswith('-') or re.search(r'[\s\x00-\x1f\x7f]', source):
            raise ValueError('Invalid development source ref')
        candidates = [source]
        if source.startswith('refs/heads/'):
            candidates.insert(0, 'refs/remotes/origin/' + source[len('refs/heads/'):])
        elif not source.startswith('refs/') and not re.fullmatch(r'[0-9a-f]{40}', source):
            candidates.insert(0, 'refs/remotes/origin/' + source)
        for candidate in candidates:
            result = subprocess.run(['git', 'rev-parse', '--verify', '--end-of-options', candidate + '^{commit}'], text=True, capture_output=True, check=False)
            if result.returncode == 0 and re.fullmatch(r'[0-9a-f]{40}\n?', result.stdout):
                sha = result.stdout.strip()
                break
        else:
            raise ValueError('Development source does not resolve to a commit')
    ids = ALL_PRODUCTION if target == 'all-production' else [target]
    targets = [{'id': item, 'environment': item, 'image_tag': item,
                'brand_profile': PROFILES[item],
                'compose_file': 'deploy/docker-compose.local-db.yml' if PROFILES[item] == 'claudeye' else 'deploy/docker-compose.cicd.yml'} for item in ids]
    return {'targets': targets, 'source_sha': sha}


if __name__ == '__main__':
    try:
        resolved = resolve(os.environ)
    except ValueError as error:
        print(f'::error::{error}', file=sys.stderr)
        sys.exit(1)
    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
            output.write('targets=' + json.dumps(resolved['targets'], separators=(',', ':')) + '\n')
            output.write('source_sha=' + resolved['source_sha'] + '\n')
    else:
        print(json.dumps(resolved))
