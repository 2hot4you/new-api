#!/bin/sh
set -eu

deploy_dir=${1:?deployment directory required}
container=${2:?application container required}
backup_name=${3:?backup name required}
runtime_env="$deploy_dir/.env.runtime"

case "$deploy_dir" in
  /opt/molii/production|/opt/ixiaozu/production|/opt/claudeye/production) ;;
  *) exit 1 ;;
esac
case "$container" in
  molii-production|ixiaozu-production|claudeye-production|model-claudeye-production) ;;
  *) exit 1 ;;
esac
case "$backup_name" in
  *[!a-zA-Z0-9._-]*|'') exit 1 ;;
esac

test -r "$runtime_env"
test "$(docker inspect --format '{{.State.Running}}' "$container")" = true
umask 077
mkdir -p "$deploy_dir/backups"
backup_dir=$(mktemp -d "$deploy_dir/backups/$backup_name.XXXXXX")
docker inspect --format '{{.Config.Image}}' "$container" > "$backup_dir/previous-image.txt"

docker run --rm \
  --network "container:$container" \
  --user "$(id -u):$(id -g)" \
  --env-file "$runtime_env" \
  --volume "$backup_dir:/backup" \
  --volume "$deploy_dir/certs:/app/certs:ro" \
  postgres:15-alpine \
  sh -ec '
    umask 077
    test -n "$SQL_DSN"
    pg_dump --dbname "$SQL_DSN" --format custom --file /backup/main.dump
    test -s /backup/main.dump
    pg_restore --list /backup/main.dump >/dev/null
    if test -n "${LOG_SQL_DSN:-}" && test "$LOG_SQL_DSN" != "$SQL_DSN"; then
      pg_dump --dbname "$LOG_SQL_DSN" --format custom --file /backup/log.dump
      test -s /backup/log.dump
      pg_restore --list /backup/log.dump >/dev/null
    fi
  '

chmod 600 "$backup_dir/main.dump" "$backup_dir/previous-image.txt"
sha256sum "$backup_dir/main.dump"
if test -f "$backup_dir/log.dump"; then
  chmod 600 "$backup_dir/log.dump"
  sha256sum "$backup_dir/log.dump"
fi
printf 'Verified production backup: %s\n' "$backup_dir"
