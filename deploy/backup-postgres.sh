#!/usr/bin/env sh

set -eu

deploy_dir=${1:?deployment directory is required}
backup_name=${2:?backup name is required}
backup_dir="$deploy_dir/backups"
runtime_env="$deploy_dir/.env.runtime"
main_dump="$backup_dir/$backup_name"
log_dump="$backup_dir/$backup_name.log.dump"
private_log="$backup_dir/$backup_name.verify.log"
lock_dir="$backup_dir/.$backup_name.lock"
verify_container_id=''

case "$backup_name" in
  *[!a-zA-Z0-9._-]*|'')
    printf 'invalid backup name\n' >&2
    exit 1
    ;;
esac

test -r "$runtime_env"
grep -Eq '^SQL_DSN=.+' "$runtime_env"
umask 077
install -d -m 700 "$backup_dir"

# Refuse existing outputs before Docker or metadata writes. The exclusive lock
# serializes this helper; noclobber inside the dump container also protects
# against a file appearing after this check.
for output in "$main_dump" "$log_dump" "$private_log"; do
  if test -e "$output" || test -L "$output"; then
    printf 'backup output already exists: %s\n' "$output" >&2
    exit 1
  fi
done
mkdir "$lock_dir"

cleanup() {
  backup_status=$?
  trap - 0 HUP INT TERM
  if test -n "$verify_container_id"; then
    if ! docker rm --force --volumes "$verify_container_id" >>"$private_log" 2>&1; then
      backup_status=1
    fi
  fi
  if ! rmdir "$lock_dir"; then
    backup_status=1
  fi
  if test "$backup_status" -ne 0; then
    printf 'PostgreSQL backup verification failed; dumps retained, private diagnostics: %s\n' "$private_log" >&2
  fi
  exit "$backup_status"
}
trap cleanup 0
trap 'exit 1' HUP INT TERM
(set -C; : >"$private_log")

previous_image=$(docker inspect --format '{{.Config.Image}}' molii-development 2>>"$private_log")
printf '%s\n' "$previous_image" > "$deploy_dir/.rc30-previous-image"
chmod 600 "$deploy_dir/.rc30-previous-image"

docker run --rm \
  --user "$(id -u):$(id -g)" \
  --env-file "$runtime_env" \
  --volume "$backup_dir:/backup" \
  --env "BACKUP_NAME=$backup_name" \
  postgres:15-alpine \
  sh -ec '
    set -euC
    umask 077
    test -n "$SQL_DSN"
    pg_dump --dbname "$SQL_DSN" --format custom > "/backup/$BACKUP_NAME"
    test -s "/backup/$BACKUP_NAME"
    if test -n "${LOG_SQL_DSN:-}" && test "$LOG_SQL_DSN" != "$SQL_DSN"; then
      pg_dump --dbname "$LOG_SQL_DSN" --format custom > "/backup/$BACKUP_NAME.log.dump"
      test -s "/backup/$BACKUP_NAME.log.dump"
    fi
  ' >>"$private_log" 2>&1

chmod 600 "$main_dump"
if test -e "$log_dump"; then
  chmod 600 "$log_dump"
fi

# Restore only through the new container's local Unix socket. It has no
# network, published ports, host data directory or source database credentials.
# Ownership/ACLs refer to production roles, so validate schema/data separately.
# Main and log restores share a bounded 768 MiB PGDATA (including WAL/indexes);
# archives exceeding that capacity fail closed, keeping both dumps intact.
created_container_id=$(docker create \
  --network none \
  --memory 1g \
  --memory-swap 1g \
  --cpus 1 \
  --tmpfs /var/lib/postgresql/data:rw,nosuid,nodev,size=768m \
  --volume "$backup_dir:/backup:ro" \
  --env POSTGRES_HOST_AUTH_METHOD=trust \
  --env POSTGRES_USER=backup_verify \
  --env POSTGRES_DB=backup_verify \
  postgres:15-alpine 2>>"$private_log")
case "$created_container_id" in
  *[!a-f0-9]*|'')
    printf 'invalid verification container identity\n' >>"$private_log"
    exit 1
    ;;
esac
if test "${#created_container_id}" -ne 64; then
  printf 'invalid verification container identity length\n' >>"$private_log"
  exit 1
fi
verify_container_id=$created_container_id
verify_limits=$(docker inspect --format '{{.HostConfig.Memory}} {{.HostConfig.MemorySwap}} {{.HostConfig.NanoCpus}}' "$verify_container_id" 2>>"$private_log")
verify_tmpfs=$(docker inspect --format '{{index .HostConfig.Tmpfs "/var/lib/postgresql/data"}}' "$verify_container_id" 2>>"$private_log")
if test "$verify_limits" != '1073741824 1073741824 1000000000' || test "$verify_tmpfs" != 'rw,nosuid,nodev,size=768m'; then
  printf 'verification resource limits unavailable or discarded\n' >>"$private_log"
  exit 1
fi
printf 'Verification resources: %s; pgdata=%s\n' "$verify_limits" "$verify_tmpfs" >>"$private_log"
docker start "$verify_container_id" >>"$private_log" 2>&1
ready_attempt=0
# The image starts a temporary socket-only server during initialization. Wait
# until the entrypoint has exec'd the final PostgreSQL process as PID1 too.
until docker exec "$verify_container_id" sh -ec 'test "$(cat /proc/1/comm)" = postgres && exec pg_isready --username backup_verify --dbname backup_verify' >>"$private_log" 2>&1; do
  ready_attempt=$((ready_attempt + 1))
  if test "$ready_attempt" -ge 30; then
    printf 'verification PostgreSQL did not become ready\n' >>"$private_log"
    exit 1
  fi
  sleep 1
done

docker exec "$verify_container_id" createdb --username backup_verify restore_main >>"$private_log" 2>&1
docker exec "$verify_container_id" pg_restore --username backup_verify --dbname restore_main \
  --exit-on-error --no-owner --no-acl --verbose "/backup/$backup_name" >>"$private_log" 2>&1
if test -s "$log_dump"; then
  docker exec "$verify_container_id" createdb --username backup_verify restore_log >>"$private_log" 2>&1
  docker exec "$verify_container_id" pg_restore --username backup_verify --dbname restore_log \
    --exit-on-error --no-owner --no-acl --verbose "/backup/$backup_name.log.dump" >>"$private_log" 2>&1
fi
docker rm --force --volumes "$verify_container_id" >>"$private_log" 2>&1
verify_container_id=''

sha256sum "$main_dump"
if test -s "$log_dump"; then
  sha256sum "$log_dump"
fi
printf 'PostgreSQL backups restored and verified offline: %s\n' "$main_dump"
