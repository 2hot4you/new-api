#!/usr/bin/env bash
# GitHub runner client for the Tencent host's forced-command protocol.
set -euo pipefail
export LC_ALL=C

fail() {
  printf '::error::%s\n' "$1" >&2
  exit "${2:-1}"
}

image_pattern='ghcr\.io/2hot4you/new-api@sha256:[0-9a-f]{64}'
case ${1:-} in
  preflight)
    [[ $# == 1 ]] || fail 'Invalid Tencent preflight arguments.'
    remote_command=preflight
    ;;
  release)
    [[ $# == 4 && $2 =~ ^${image_pattern}$ && $3 =~ ^${image_pattern}$ && $4 =~ ^[0-9a-f]{40}$ ]] ||
      fail 'Invalid Tencent release image or source.'
    remote_command="release $2 $3 $4"
    ;;
  *) fail 'Invalid Tencent deployment operation.' ;;
esac

host=${MODEL_TENCENT_SSH_HOST:-}
port=${MODEL_TENCENT_SSH_PORT:-}
user=${MODEL_TENCENT_SSH_USER:-}
[[ $host =~ ^[a-zA-Z0-9][a-zA-Z0-9.-]*$ && $user =~ ^[a-zA-Z0-9_][a-zA-Z0-9_-]*$ ]] ||
  fail 'Invalid Tencent SSH host or user.'
[[ $port =~ ^[0-9]{1,5}$ ]] && (( 10#$port > 0 && 10#$port <= 65535 )) ||
  fail 'Invalid Tencent SSH port.'
[[ -n ${MODEL_TENCENT_SSH_PRIVATE_KEY:-} && -n ${MODEL_TENCENT_SSH_KNOWN_HOSTS:-} ]] ||
  fail 'Tencent SSH identity or pinned host keys are missing.'
[[ -n ${GITHUB_OUTPUT:-} && -n ${RUNNER_TEMP:-} ]] || fail 'Tencent runner paths are missing.'

umask 077
work=$(mktemp -d "$RUNNER_TEMP/model-tencent-ssh.XXXXXXXX")
trap 'rm -f -- "$work/identity" "$work/known_hosts" "$work/stdout"; rmdir -- "$work"' EXIT
printf '%s\n' "$MODEL_TENCENT_SSH_PRIVATE_KEY" > "$work/identity"
printf '%s\n' "$MODEL_TENCENT_SSH_KNOWN_HOSTS" > "$work/known_hosts"
unset MODEL_TENCENT_SSH_PRIVATE_KEY MODEL_TENCENT_SSH_KNOWN_HOSTS

status=0
ssh -F /dev/null -n -T -i "$work/identity" -p "$((10#$port))" \
  -o BatchMode=yes -o IdentitiesOnly=yes -o IdentityAgent=none -o PreferredAuthentications=publickey \
  -o StrictHostKeyChecking=yes -o GlobalKnownHostsFile=/dev/null \
  -o "UserKnownHostsFile=\"$work/known_hosts\"" -o UpdateHostKeys=no \
  -o ClearAllForwardings=yes -o ForwardAgent=no -o ForwardX11=no \
  -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=3 \
  "$user@$host" "$remote_command" > "$work/stdout" 2>/dev/null || status=$?

# Never print the raw remote response. Check byte equality as read alone can
# ignore NULs, trailing data or an absent newline.
line=
if ! IFS= read -r line < "$work/stdout" || ! printf '%s\n' "$line" | cmp -s - "$work/stdout"; then
  line=
fi

if [[ $1 == preflight ]]; then
  (( status == 0 )) || fail 'Tencent read-only preflight failed; inspect private server diagnostics.' "$status"
  [[ $line =~ ^MODEL_CLAUDEYE_PREVIOUS_IMAGE=(${image_pattern})$ ]] || fail 'Invalid Tencent preflight response.'
  printf 'previous_image=%s\n' "${BASH_REMATCH[1]}" >> "$GITHUB_OUTPUT"
else
  rollback=unavailable
  if [[ $line =~ ^DEPLOY_ROLLBACK_RESULT=(not_attempted|succeeded|failed)$ ]]; then
    if (( status != 0 )) || [[ ${BASH_REMATCH[1]} == not_attempted ]]; then
      rollback=${BASH_REMATCH[1]}
    fi
  fi
  printf 'rollback=%s\n' "$rollback" >> "$GITHUB_OUTPUT"
  (( status == 0 )) || fail 'Remote deployment failed; inspect private server diagnostics.' "$status"
  [[ $rollback != unavailable ]] || fail 'Invalid Tencent release response.'
fi
