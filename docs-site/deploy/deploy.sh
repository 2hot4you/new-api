#!/usr/bin/env bash

set -Eeuo pipefail

environment=${1:-}
release_id=${2:-}
archive_path=${3:-}
archive_sha256=${4:-}
site_origin=${5:-}

public_root=${DOCS_DEPLOY_ROOT:-/opt/1panel/www/sites}
private_root=${DOCS_PRIVATE_ROOT:-/opt/molii}
flock_bin=${FLOCK_BIN:-flock}
staging_dir=''
snapshot_dir=''
next_snapshot_dir=''
headers_file=''
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
  if [[ -n "$staging_dir" && -d "$staging_dir" ]]; then
    rm -rf -- "$staging_dir"
  fi
  if [[ -n "$next_snapshot_dir" && -d "$next_snapshot_dir" ]]; then
    rm -rf -- "$next_snapshot_dir"
  fi
  if [[ -n "$headers_file" && -f "$headers_file" ]]; then
    rm -f -- "$headers_file"
  fi
  if ((archive_owned == 1)) && [[ -f "$archive_path" ]]; then
    rm -f -- "$archive_path"
  fi
}

restore_snapshot() {
  if ((rollback_ready == 0)); then
    return
  fi

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
  production)
    public_dir="$public_root/molii.co/index/docs"
    expected_origin='https://molii.co'
    ;;
  development)
    public_dir="$public_root/dev.molii.co/index/docs"
    expected_origin='https://dev.molii.co'
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
if [[ ! "$archive_sha256" =~ ^[a-fA-F0-9]{64}$ ]]; then
  fail 'artifact checksum must be a SHA-256 value'
fi

environment_root="$private_root/$environment"
expected_archive="$environment_root/molii-docs-$release_id.tar.gz"
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

for command_name in sha256sum tar rsync curl "$flock_bin"; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    fail "required command is unavailable: $command_name"
  fi
done

state_dir="$environment_root/data/docs-deploy"
staging_dir="$state_dir/staging-$release_id"
snapshot_dir="$state_dir/previous"
next_snapshot_dir="$state_dir/snapshot-$release_id"
headers_file="$state_dir/headers-$release_id"
mkdir -p "$state_dir"

exec 9>"$state_dir/deploy.lock"
if ! "$flock_bin" -n 9; then
  fail 'another documentation deployment is in progress'
fi

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

rm -rf -- "$staging_dir" "$next_snapshot_dir"
mkdir -p "$staging_dir" "$next_snapshot_dir"
tar --no-same-owner --no-same-permissions -xzf "$archive_path" -C "$staging_dir"

if [[ ! -f "$staging_dir/quick-start/index.html" ]]; then
  fail 'documentation artifact is missing quick-start/index.html'
fi
if [[ ! -d "$staging_dir/assets" ]]; then
  fail 'documentation artifact is missing assets/'
fi
if ! grep -Fq 'Molii 开发者文档' "$staging_dir/quick-start/index.html"; then
  fail 'quick-start page does not contain the expected site marker'
fi

rsync --archive --delete --delay-updates "$public_dir/" "$next_snapshot_dir/"
rm -rf -- "$snapshot_dir"
mv "$next_snapshot_dir" "$snapshot_dir"
rollback_ready=1

rsync --archive --delete --delay-updates "$staging_dir/" "$public_dir/"

check_redirect() {
  local path=$1
  local status location
  : >"$headers_file"
  status=$(curl --silent --show-error --max-time 15 \
    --output /dev/null --dump-header "$headers_file" --write-out '%{http_code}' \
    "$site_origin$path")
  location=$(awk 'BEGIN { IGNORECASE=1 } /^Location:/ { sub(/^[^:]+:[[:space:]]*/, ""); sub(/\r$/, ""); print; exit }' "$headers_file")
  if [[ "$status" != '308' || "$location" != '/docs/quick-start' ]]; then
    fail "$path did not redirect to /docs/quick-start"
  fi
}

check_redirect '/docs'
check_redirect '/docs/'
if ! curl --fail --silent --show-error --max-time 15 "$site_origin/docs/quick-start" \
  | grep -Fq 'Molii 开发者文档'; then
  fail 'public quick-start health check failed'
fi

rm -f -- "$headers_file"
cleanup
trap - ERR INT TERM
log "release $release_id deployed to $environment"
