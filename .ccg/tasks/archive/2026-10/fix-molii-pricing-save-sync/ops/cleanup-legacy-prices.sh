#!/usr/bin/env bash
# Authorized offline repair of exactly two DEV pricing leaves. Never writes PROD.
set -Eeuo pipefail
set +x
[[ $(hostname) == molii && $(id -un) == molii-deploy ]]
umask 077
ops=$(cd -- "$(dirname -- "$0")" && pwd)
readonly directory=/opt/molii/development
readonly app=molii-development
[[ -f "$ops/cleanup-legacy-prices.sql" ]]
[[ -f "$ops/check-legacy-prices-original.sql" ]]
exec 8>"$directory/.deploy.lock"
flock -n 8 || { printf 'Development deployment active; stopping.\n' >&2; exit 1; }
exec 9>/opt/molii/production/.deploy.lock
flock -n 9 || { printf 'Production deployment active; stopping.\n' >&2; exit 1; }
original_id=$(docker inspect --format '{{.Id}}' "$app")
original_image=$(docker inspect --format '{{.Image}}' "$app")
network=$(docker inspect --format '{{.HostConfig.NetworkMode}}' "$app")
[[ $(stat -c %a "$directory/.env.runtime") == 600 ]]
[[ $(stat -c %U "$directory/.env.runtime") == molii-deploy ]]
live_dsn_hash=$(docker inspect --format '{{json .Config.Env}}' "$app" | jq -er '.[]|select(startswith("SQL_DSN="))' | sha256sum)
file_dsn_hash=$(grep '^SQL_DSN=' "$directory/.env.runtime" | sha256sum)
[[ "$live_dsn_hash" == "$file_dsn_hash" ]]
unset live_dsn_hash file_dsn_hash
backup_dir=$(mktemp -d "$directory/data/catalog-sync/legacy-price-cleanup-XXXXXX")
stopped=false
attempted=false
db() {
  local readonly=$1
  shift
  docker run -i --rm --pull never --network "$network" --user "$(id -u):$(id -g)" \
    --volumes-from "$app:ro" --env-file "$directory/.env.runtime" \
    --mount "type=bind,source=$backup_dir,target=/repair" \
    --mount "type=bind,source=$ops/cleanup-legacy-prices.sql,target=/cleanup.sql,readonly" \
    --mount "type=bind,source=$ops/check-legacy-prices-original.sql,target=/check-original.sql,readonly" \
    --entrypoint sh postgres:15-alpine -c '
      export PGCONNECT_TIMEOUT=10 PGOPTIONS="-c statement_timeout=15000 -c lock_timeout=5000"
      if [ "$1" = true ]; then export PGOPTIONS="$PGOPTIONS -c default_transaction_read_only=on"; fi
      shift
      exec psql "$SQL_DSN" -X -qAt -v ON_ERROR_STOP=1 "$@"
    ' sh "$readonly" "$@"
}
wait_health() {
  local attempt
  for ((attempt=1; attempt<=60; attempt++)); do
    [[ $(docker inspect --format '{{.State.Health.Status}}' "$app") == healthy ]] && return 0
    sleep 2
  done
  return 1
}
assert_same_container() {
  [[ $(docker inspect --format '{{.Id}}' "$app") == "$original_id" && $(docker inspect --format '{{.Image}}' "$app") == "$original_image" ]]
}
recover() {
  local result=$? failed=false
  [[ "$result" == 0 ]] && return
  trap - EXIT HUP INT TERM
  set +e
  if [[ "$stopped" == true ]]; then
    assert_same_container || { printf 'Container identity changed; manual recovery required.\n' >&2; exit "$result"; }
    if [[ "$attempted" == true ]]; then
      if ! docker stop --time 60 "$app" >"$backup_dir/recovery-stop.log" 2>&1 \
        || [[ $(docker inspect --format '{{.State.Running}}' "$app") != false ]]; then
        printf 'DEV stop unverified; no restore SQL issued. Inspect: %s\n' "$backup_dir" >&2
        exit "$result"
      fi
      if ! sha256sum -c "$backup_dir/options.csv.sha256" >"$backup_dir/backup-verification.log" 2>&1 \
        || ! db false -f /check-original.sql >"$backup_dir/original-check" 2>"$backup_dir/original-check-error"; then
        printf 'Backup/original-state verification failed; DEV kept stopped. Inspect: %s\n' "$backup_dir" >&2
        exit "$result"
      fi
      if ! grep -qx t "$backup_dir/original-check"; then
        if ! db false -v restore=true -f /cleanup.sql >"$backup_dir/recovery.log" 2>&1; then
          printf 'Restore CAS refused; DEV kept stopped. Inspect private backup: %s\n' "$backup_dir" >&2
          exit "$result"
        fi
      fi
    fi
    docker start "$app" >"$backup_dir/recovery-start.log" 2>&1 && wait_health || failed=true
  fi
  printf 'Cleanup stopped; recovery failure=%s. Private backup: %s\n' "$failed" "$backup_dir" >&2
  exit "$result"
}
trap recover EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
bash "$ops/preflight.sh" >"$backup_dir/preflight.log" 2>&1
sed 's/^[^ ]* //' "$backup_dir/preflight.log" | jq -se \
  'length==2 and .[0].database_identity!=.[1].database_identity' >/dev/null
[[ $(docker inspect --format '{{.State.Health.Status}}' "$app") == healthy ]]
stopped=true
docker stop --time 60 "$app" >"$backup_dir/stop.log" 2>&1
assert_same_container
[[ $(docker inspect --format '{{.State.Running}}' "$app") == false ]]
db true >"$backup_dir/options.csv" 2>"$backup_dir/backup-error.log" <<'SQL'
BEGIN READ ONLY;
COPY (SELECT key,value FROM options WHERE key IN ('billing_setting.billing_expr','billing_setting.billing_mode') ORDER BY key) TO STDOUT WITH (FORMAT csv,HEADER true);
ROLLBACK;
SQL
[[ -s "$backup_dir/options.csv" ]]
chmod 600 "$backup_dir/options.csv"
sha256sum "$backup_dir/options.csv" >"$backup_dir/options.csv.sha256"
sync --file-system "$backup_dir"
attempted=true
db false -v restore=false -f /cleanup.sql >"$backup_dir/cleanup.log" 2>"$backup_dir/cleanup-error.log"
docker start "$app" >"$backup_dir/start.log" 2>&1
assert_same_container
wait_health
curl --fail --silent --show-error --max-time 20 https://dev.molii.co/api/status | jq -e '.success==true' >/dev/null
trap - EXIT HUP INT TERM
printf 'Only the two approved DEV leaves removed; original container/image healthy.\nPrivate raw backup: %s/options.csv\n' "$backup_dir"
stat -c 'Backup permissions=%a owner=%U size=%s' "$backup_dir/options.csv"
sha256sum "$backup_dir/options.csv"
