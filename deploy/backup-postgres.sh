#!/usr/bin/env sh

set -eu

deploy_dir=${1:?deployment directory is required}
backup_name=${2:?backup name is required}
backup_dir="$deploy_dir/backups"
runtime_env="$deploy_dir/.env.runtime"

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

previous_image=$(docker inspect --format '{{.Config.Image}}' molii-development)
printf '%s\n' "$previous_image" > "$deploy_dir/.rc30-previous-image"

docker run --rm \
  --user "$(id -u):$(id -g)" \
  --env-file "$runtime_env" \
  --volume "$backup_dir:/backup" \
  --env "BACKUP_NAME=$backup_name" \
  postgres:15-alpine \
  sh -ec '
    test -n "$SQL_DSN"
    pg_dump --dbname "$SQL_DSN" --format custom --file "/backup/$BACKUP_NAME"
    test -s "/backup/$BACKUP_NAME"
    pg_restore --list "/backup/$BACKUP_NAME" >/dev/null
  '

chmod 600 "$backup_dir/$backup_name"
sha256sum "$backup_dir/$backup_name"
printf 'PostgreSQL backup verified: %s\n' "$backup_dir/$backup_name"
