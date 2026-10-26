#!/usr/bin/env bash

set -Eeuo pipefail

environment=${1:-}
release_id=${2:-}
archive_path=${3:-}
archive_sha256=${4:-}
site_origin=${5:-}
site_marker=${6:-}

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# Repository tests use the shared helper; servers receive it beside this script.
maintenance_helper="$script_dir/maintenance-window.sh"
if [[ ! -f "$maintenance_helper" && -f "$script_dir/../../deploy/maintenance-window.sh" ]]; then
  maintenance_helper="$script_dir/../../deploy/maintenance-window.sh"
fi
source "$maintenance_helper"
check_maintenance_window "$environment"

guarded() {
  check_maintenance_window "$environment" || return 1
  "$@"
}

public_root=${DOCS_DEPLOY_ROOT:-/opt/1panel/www/sites}
molii_private_root=${MOLII_PRIVATE_ROOT:-/opt/molii}
ixiaozu_private_root=${IXIAOZU_PRIVATE_ROOT:-/opt/ixiaozu}
claudeye_private_root=${CLAUDEYE_PRIVATE_ROOT:-/opt/claudeye}
flock_bin=${FLOCK_BIN:-flock}
staging_dir=''
snapshot_dir=''
next_snapshot_dir=''
headers_file=''
health_body_file=''
public_dir=''
rollback_ready=0
archive_owned=0

log() {
  printf '[docs-deploy] %s\n' "$1"
}

fail() {
  printf '[docs-deploy] %s\n' "$1" >&2
  return 1
}

cleanup() {
  check_maintenance_window "$environment" || return 1
  if [[ -n "$staging_dir" && -d "$staging_dir" ]]; then
    guarded rm -rf -- "$staging_dir" || return 1
  fi
  if [[ -n "$next_snapshot_dir" && -d "$next_snapshot_dir" ]]; then
    guarded rm -rf -- "$next_snapshot_dir" || return 1
  fi
  if [[ -n "$headers_file" && -f "$headers_file" ]]; then
    guarded rm -f -- "$headers_file" || return 1
  fi
  if [[ -n "$health_body_file" && -f "$health_body_file" ]]; then
    guarded rm -f -- "$health_body_file" || return 1
  fi
  if ((archive_owned == 1)) && [[ -f "$archive_path" ]]; then
    guarded rm -f -- "$archive_path" || return 1
  fi
}

restore_snapshot() {
  if ((rollback_ready == 0)); then
    return
  fi
  check_maintenance_window "$environment" || return 1

  log 'deployment failed; restoring previous documentation snapshot'
  if ! rsync --archive --delete --delay-updates "$snapshot_dir/" "$public_dir/"; then
    printf '[docs-deploy] rollback failed; manual recovery is required\n' >&2
  fi
}

on_error() {
  local status=$?
  trap - ERR INT TERM
  restore_snapshot
  cleanup
  exit "$status"
}

on_signal() {
  local status=$1
  trap - ERR INT TERM
  restore_snapshot
  cleanup
  exit "$status"
}

trap on_error ERR
trap 'on_signal 130' INT
trap 'on_signal 143' TERM

case "$environment" in
  production-molii)
    public_dir="$public_root/molii.co/index/docs"
    environment_root="$molii_private_root/production"
    expected_origin='https://molii.co'
    ;;
  development)
    public_dir="$public_root/dev.molii.co/index/docs"
    environment_root="$molii_private_root/development"
    expected_origin='https://dev.molii.co'
    ;;
  production-ixiaozu)
    public_dir="$public_root/aigc.ixiaozu.cn/index/docs"
    environment_root="$ixiaozu_private_root/production"
    expected_origin='https://aigc.ixiaozu.cn'
    ;;
  production-claudeye)
    public_dir="$public_root/claudeye.com/index/docs"
    environment_root="$claudeye_private_root/production"
    expected_origin='https://claudeye.com'
    ;;
  production-model-claudeye)
    public_dir="$public_root/model.claudeye.com/index/docs"
    environment_root="$claudeye_private_root/production"
    expected_origin='https://model.claudeye.com'
    ;;
  *)
    fail "unsupported environment: $environment"
    ;;
esac

if [[ ! "$release_id" =~ ^[A-Za-z0-9._-]+$ ]]; then
  fail 'release ID contains unsupported characters'
fi
if [[ "$site_origin" != "$expected_origin" ]]; then
  fail "site origin does not match $environment"
fi
if [[ -z "$site_marker" || ${#site_marker} -gt 160 ]]; then
  fail 'site marker must contain between 1 and 160 characters'
fi
if [[ ! "$archive_sha256" =~ ^[a-fA-F0-9]{64}$ ]]; then
  fail 'artifact checksum must be a SHA-256 value'
fi

expected_archive="$environment_root/docs-$environment-$release_id.tar.gz"
if [[ "$archive_path" != "$expected_archive" ]]; then
  fail 'artifact path does not match the release destination'
fi
if [[ ! -f "$archive_path" ]]; then
  fail 'documentation artifact does not exist'
fi
archive_owned=1
if [[ ! -d "$public_dir" || ! -w "$public_dir" ]]; then
  fail 'public documentation directory is missing or not writable'
fi

for command_name in sha256sum tar rsync curl mktemp "$flock_bin"; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    fail "required command is unavailable: $command_name"
  fi
done

state_dir="$environment_root/data/docs-deploy"
check_maintenance_window "$environment"
staging_dir="$state_dir/staging-$release_id"
snapshot_dir="$state_dir/previous"
next_snapshot_dir="$state_dir/snapshot-$release_id"
headers_file="$state_dir/headers-$release_id"
mkdir -p "$state_dir"

exec 9>"$state_dir/deploy.lock"
if ! "$flock_bin" -n 9; then
  fail 'another documentation deployment is in progress'
fi
health_body_file=$(guarded mktemp "$state_dir/health-body-$release_id.XXXXXX")

actual_sha256=$(sha256sum "$archive_path" | awk '{print $1}' | tr '[:upper:]' '[:lower:]')
expected_sha256=$(printf '%s' "$archive_sha256" | tr '[:upper:]' '[:lower:]')
if [[ "$actual_sha256" != "$expected_sha256" ]]; then
  fail 'documentation artifact checksum mismatch'
fi

archive_names=$(tar -tzf "$archive_path")
while IFS= read -r entry; do
  normalized_entry=${entry#./}
  if [[ -z "$normalized_entry" ]]; then
    continue
  fi
  if [[ "$normalized_entry" == /* ]]; then
    fail 'unsupported archive entry: absolute path'
  fi
  IFS='/' read -r -a path_parts <<<"$normalized_entry"
  for path_part in "${path_parts[@]}"; do
    if [[ "$path_part" == '..' ]]; then
      fail 'unsupported archive entry: parent traversal'
    fi
  done
done <<<"$archive_names"

archive_details=$(tar -tvzf "$archive_path")
while IFS= read -r entry_detail; do
  entry_type=${entry_detail:0:1}
  if [[ "$entry_type" != '-' && "$entry_type" != 'd' ]]; then
    fail 'unsupported archive entry: links and special files are forbidden'
  fi
done <<<"$archive_details"

guarded rm -rf -- "$staging_dir" "$next_snapshot_dir"
guarded mkdir -p "$staging_dir" "$next_snapshot_dir"
guarded tar --no-same-owner --no-same-permissions -xzf "$archive_path" -C "$staging_dir"

if [[ ! -f "$staging_dir/quick-start/index.html" ]]; then
  fail 'documentation artifact is missing quick-start/index.html'
fi
if [[ ! -d "$staging_dir/assets" ]]; then
  fail 'documentation artifact is missing assets/'
fi
if ! grep -Fq -- "$site_marker" "$staging_dir/quick-start/index.html"; then
  fail 'quick-start page does not contain the expected site marker'
fi

guarded rsync --archive --delete --delay-updates "$public_dir/" "$next_snapshot_dir/"
guarded rm -rf -- "$snapshot_dir"
guarded mv "$next_snapshot_dir" "$snapshot_dir"
rollback_ready=1

check_maintenance_window "$environment"
guarded rsync --archive --delete --delay-updates "$staging_dir/" "$public_dir/"

check_redirect() {
  local path=$1
  local status location expected_path expected_absolute
  expected_path='/docs/quick-start'
  expected_absolute="$site_origin$expected_path"
  check_maintenance_window "$environment" || return 1
  : >"$headers_file"
  status=$(guarded curl --silent --show-error --max-time 15 \
    --output /dev/null --dump-header "$headers_file" --write-out '%{http_code}' \
    "$site_origin$path")
  location=$(awk 'tolower($0) ~ /^location:/ { sub(/^[^:]+:[[:space:]]*/, ""); sub(/\r$/, ""); print; exit }' "$headers_file")
  if [[ "$status" != '308' ]] ||
    [[ "$location" != "$expected_path" && "$location" != "$expected_absolute" ]]; then
    fail "$path did not redirect to /docs/quick-start"
  fi
}

check_redirect '/docs'
check_redirect '/docs/'
if ! guarded curl --fail --silent --show-error --max-time 15 \
  --output "$health_body_file" "$site_origin/docs/quick-start/"; then
  fail 'public quick-start request failed'
fi
if ! grep -Fq -- "$site_marker" "$health_body_file"; then
  fail 'public quick-start health check failed'
fi

guarded rm -f -- "$headers_file"
cleanup
trap - ERR INT TERM
log "release $release_id deployed to $environment"
