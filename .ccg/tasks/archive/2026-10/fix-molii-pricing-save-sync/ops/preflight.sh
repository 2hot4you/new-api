#!/usr/bin/env bash
# Read-only inventory, executed as the existing deployment user on molii.
set -Eeuo pipefail
[[ $(hostname) == molii && $(id -un) == molii-deploy ]]
umask 077
probe_dir=$(mktemp -d /tmp/molii-catalog-preflight.XXXXXX)
trap 'rm -f -- "$probe_dir/db-error" "$probe_dir/db-state"; rmdir -- "$probe_dir"' EXIT
for environment in development production; do
  container="molii-$environment"
  directory="/opt/molii/$environment"
  [[ -f "$directory/.env.runtime" && -f "$directory/.deploy.env" ]]
  [[ $(docker inspect --format '{{.State.Health.Status}}' "$container") == healthy ]]
  network=$(docker inspect --format '{{.HostConfig.NetworkMode}}' "$container")
  docker inspect "$container" --format '{{json .Config.Env}}' | jq -e '
    any(.[]; startswith("NODE_NAME=") and . != "NODE_NAME=") and
    (any(.[]; . == "NODE_TYPE=slave") | not) and
    (any(.[]; startswith("CATALOG_SYNC_")) | not)' >/dev/null
  if ! docker run -i --rm --pull never --network "$network" \
    --volumes-from "$container:ro" --env-file "$directory/.env.runtime" \
    --entrypoint sh postgres:15-alpine -c '
      export PGCONNECT_TIMEOUT=10 PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=15000"
      exec psql "$SQL_DSN" -X -qAt -v ON_ERROR_STOP=1
    ' >"$probe_dir/db-state" 2>"$probe_dir/db-error" <<'SQL'
BEGIN READ ONLY;
SELECT json_build_object(
 'database_identity', md5(current_database() || current_schema() || coalesce(inet_server_addr()::text,'') || inet_server_port()::text),
 'active_tasks', (SELECT count(*) FROM tasks WHERE status IN ('NOT_START','SUBMITTED','QUEUED','IN_PROGRESS','UNKNOWN')),
 'active_instances', (SELECT count(*) FROM system_instances WHERE last_seen_at >= extract(epoch from now())::bigint - 90),
 'manual_nodes', (SELECT coalesce(bool_and(info::jsonb #>> '{node,source}' = 'manual'), false) FROM system_instances WHERE last_seen_at >= extract(epoch from now())::bigint - 90)
);
ROLLBACK;
SQL
  then
    printf 'Read-only database preflight failed for %s (details withheld).\n' "$container" >&2
    exit 1
  fi
  state=$(<"$probe_dir/db-state")
  printf '%s %s\n' "$container" "$state"
  jq -e '.active_tasks == 0 and .active_instances == 1 and .manual_nodes == true' <<<"$state" >/dev/null
done
