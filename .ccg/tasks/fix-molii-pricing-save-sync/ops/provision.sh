#!/usr/bin/env bash
# Approved dev -> molii provisioning only. Never executes catalog apply/preview.
set -Eeuo pipefail
set +x
[[ $(hostname) == molii && $(id -un) == molii-deploy ]]
umask 077
readonly dev_dir=/opt/molii/development
readonly prod_dir=/opt/molii/production
readonly probe_binary=/home/molii-deploy/molii-catalog-transport-probe
[[ -x "$probe_binary" ]]
exec 8>"$dev_dir/.deploy.lock"
flock -n 8 || { printf 'Development deployment is active; stopping.\n' >&2; exit 1; }
exec 9>"$prod_dir/.deploy.lock"
flock -n 9 || { printf 'Production deployment is active; stopping.\n' >&2; exit 1; }
stage=$(mktemp -d /home/molii-deploy/molii-catalog-setup.XXXXXX)
stamp=$(date -u +%Y%m%dT%H%M%SZ)
dev_backup="$dev_dir/.env.runtime.before-catalog-sync-$stamp"
prod_backup="$prod_dir/.env.runtime.before-catalog-sync-$stamp"
dev_changed=false
prod_changed=false

compose() {
  local dir=$1 app=$2
  shift 2
  docker compose --project-directory "$dir" --env-file "$dir/.deploy.env" \
    -f "$dir/docker-compose.yml" -p "$app" "$@"
}
wait_health() {
  local app=$1 attempt health
  for ((attempt=1; attempt<=60; attempt++)); do
    health=$(docker inspect --format '{{.State.Health.Status}}' "$app" 2>/dev/null || true)
    [[ "$health" == healthy ]] && return 0
    sleep 2
  done
  return 1
}
fingerprint() {
  docker inspect "$1" | jq -S '.[0] | {
    image:.Image,command:.Config.Cmd,entrypoint:.Config.Entrypoint,
    environment:(.Config.Env|map(select(startswith("CATALOG_SYNC_")|not))|sort),
    mounts:(.Mounts|map({Type,Source,Destination,RW})|sort_by(.Destination)),
    ports:.HostConfig.PortBindings,restart:.HostConfig.RestartPolicy,
    networks:(.NetworkSettings.Networks|keys)
  }' | sha256sum | awk '{print $1}'
}
recreate() {
  compose "$1" "$2" up -d --no-deps --no-build --pull never --force-recreate new-api >"$stage/compose-log" 2>&1 || return $?
  wait_health "$2"
}
assert_original() {
  [[ $(fingerprint "$1") == "$2" ]] || return 1
  docker inspect "$1" --format '{{json .Config.Env}}' | jq -e 'any(.[]; startswith("CATALOG_SYNC_")) | not' >/dev/null || return 1
}
assert_no_compose_drift() {
  local directory=$1 app=$2 image
  compose "$directory" "$app" config --format json >"$stage/rendered.json"
  image=$(jq -r '.services["new-api"].image' "$stage/rendered.json")
  docker image inspect "$image" >"$stage/image.json"
  docker inspect "$app" >"$stage/live.json"
  jq -en --slurpfile rendered "$stage/rendered.json" --slurpfile image "$stage/image.json" --slurpfile live "$stage/live.json" '
    $rendered[0] as $config | $config.services["new-api"] as $service |
    $image[0][0] as $image | $live[0][0] as $live |
    ($image.Id == $live.Image) and
    (($service.command // $image.Config.Cmd) == $live.Config.Cmd) and
    (($service.entrypoint // $image.Config.Entrypoint) == $live.Config.Entrypoint) and
    (($service.restart // "no") == $live.HostConfig.RestartPolicy.Name) and
    (([$service.volumes[] | {Type:.type,Source:.source,Destination:.target,RW:((.read_only // false)|not)}]|sort_by(.Destination)) == ($live.Mounts|map({Type,Source,Destination,RW})|sort_by(.Destination))) and
    (([$service.networks|keys[] as $key|$config.networks[$key].name]|sort) == ($live.NetworkSettings.Networks|keys|sort)) and
    ((reduce $service.ports[] as $port ({}; .[($port.target|tostring)+"/"+$port.protocol] += [{HostIp:$port.host_ip,HostPort:($port.published|tostring)}])) == $live.HostConfig.PortBindings) and
    (((reduce $image.Config.Env[] as $env ({}; ($env|index("=")) as $i | . + {($env[0:$i]):$env[$i+1:]})) + $service.environment) == (reduce $live.Config.Env[] as $env ({}; ($env|index("=")) as $i | . + {($env[0:$i]):$env[$i+1:]})))
  ' >/dev/null || { printf 'Compose differs from live configuration; refusing recreation.\n' >&2; return 1; }
}
rollback() {
  local original=$? failed=false
  [[ "$original" == 0 ]] && return
  trap - EXIT HUP INT TERM
  set +e
  printf 'Provisioning failed; restoring the two owned runtime configurations.\n' >&2
  if [[ "$prod_changed" == true ]]; then
    cp -p -- "$prod_backup" "$prod_dir/.env.runtime" && recreate "$prod_dir" molii-production && assert_original molii-production "$prod_fingerprint" || failed=true
  fi
  if [[ "$dev_changed" == true ]]; then
    cp -p -- "$dev_backup" "$dev_dir/.env.runtime" && recreate "$dev_dir" molii-development && assert_original molii-development "$dev_fingerprint" || failed=true
  fi
  printf 'Rollback failure=%s; private stage retained: %s\n' "$failed" "$stage" >&2
  exit "$original"
}
trap rollback EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

dev_fingerprint=$(fingerprint molii-development)
prod_fingerprint=$(fingerprint molii-production)
dev_identity=
for environment in development production; do
  directory="/opt/molii/$environment"
  app="molii-$environment"
  [[ $(docker inspect --format '{{.State.Health.Status}}' "$app") == healthy ]]
  [[ $(stat -c %a "$directory/.env.runtime") == 600 ]]
  [[ $(stat -c %U "$directory/.env.runtime") == molii-deploy ]]
  if grep -Eq '^[[:space:]]*(export[[:space:]]+)?CATALOG_SYNC_' "$directory/.env.runtime"; then
    printf 'Existing catalog configuration found; refusing overwrite.\n' >&2
    exit 1
  else
    grep_status=$?
    [[ "$grep_status" == 1 ]] || exit 1
  fi
  docker inspect "$app" --format '{{json .Config.Env}}' | jq -e '
    any(.[]; startswith("NODE_NAME=") and . != "NODE_NAME=") and
    (any(.[]; . == "NODE_TYPE=slave") | not) and
    (any(.[]; startswith("CATALOG_SYNC_")) | not)' >/dev/null
  network=$(docker inspect --format '{{.HostConfig.NetworkMode}}' "$app")
  docker run -i --rm --pull never --network "$network" --volumes-from "$app:ro" \
    --env-file "$directory/.env.runtime" --entrypoint sh postgres:15-alpine -c '
    export PGCONNECT_TIMEOUT=10 PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=15000"
    exec psql "$SQL_DSN" -X -qAt -v ON_ERROR_STOP=1
  ' >"$stage/db-state" 2>"$stage/db-error" <<'SQL'
BEGIN READ ONLY;
SELECT json_build_object(
 'identity', md5(current_database() || current_schema() || coalesce(inet_server_addr()::text,'') || inet_server_port()::text),
 'active_tasks', (SELECT count(*) FROM tasks WHERE status IN ('NOT_START','SUBMITTED','QUEUED','IN_PROGRESS','UNKNOWN')),
 'instances', (SELECT count(*) FROM system_instances WHERE last_seen_at >= extract(epoch from now())::bigint - 90),
 'manual', (SELECT coalesce(bool_and(info::jsonb #>> '{node,source}' = 'manual'),false) FROM system_instances WHERE last_seen_at >= extract(epoch from now())::bigint - 90)
);
ROLLBACK;
SQL
  jq -e '.active_tasks == 0 and .instances == 1 and .manual == true' "$stage/db-state" >/dev/null
  identity=$(jq -r .identity "$stage/db-state")
  if [[ "$environment" == development ]]; then dev_identity=$identity; else [[ "$identity" != "$dev_identity" ]]; fi
  compose "$directory" "$app" config --quiet >"$stage/config-log" 2>&1
  assert_no_compose_drift "$directory" "$app"
  printf '%s preflight passed (no active tasks, single manual instance).\n' "$app"
done

if [[ ${1:-} == --check ]]; then
  trap - EXIT HUP INT TERM
  rm -f -- "$stage/db-state" "$stage/db-error" "$stage/config-log" "$stage/rendered.json" "$stage/image.json" "$stage/live.json"
  rmdir -- "$stage"
  printf 'Read-only configuration/topology preflight complete; no runtime edits or recreation.\n'
  exit 0
fi

reader="molii-prod-$(openssl rand -hex 8)"
secret=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n')
[[ ${#secret} == 43 ]]
token="$reader.$secret"
verifier=$(printf %s "$token" | sha256sum | awk '{print $1}')
cp -p -- "$dev_dir/.env.runtime" "$dev_backup"
cp -p -- "$prod_dir/.env.runtime" "$prod_backup"
cp -- "$dev_dir/.env.runtime" "$stage/source.env"
cp -- "$prod_dir/.env.runtime" "$stage/target.env"
printf '\nCATALOG_SYNC_ROLE=source\nCATALOG_SYNC_SOURCE_ID=dev\nCATALOG_SYNC_EXTERNAL_ORIGIN=https://dev.molii.co\nCATALOG_SYNC_SINGLE_INSTANCE=true\nCATALOG_SYNC_READERS_JSON={"%s":{"verifier":"sha256:%s","status":"active"}}\n' "$reader" "$verifier" >>"$stage/source.env"
printf '\nCATALOG_SYNC_ROLE=target\nCATALOG_SYNC_SOURCE_ID=dev\nCATALOG_SYNC_TARGET_ID=molii\nCATALOG_SYNC_EXTERNAL_ORIGIN=https://molii.co\nCATALOG_SYNC_SINGLE_INSTANCE=true\nCATALOG_SYNC_TOKEN=%s\n' "$token" >>"$stage/target.env"
unset token secret verifier
chmod 600 "$stage/source.env" "$stage/target.env"

printf 'Recreating development source with the existing immutable image.\n'
dev_changed=true
install -m 600 "$stage/source.env" "$dev_dir/.env.runtime"
recreate "$dev_dir" molii-development
[[ $(fingerprint molii-development) == "$dev_fingerprint" ]]
network=$(docker inspect --format '{{.HostConfig.NetworkMode}}' molii-production)
docker run --rm --pull never --network "$network" --volumes-from molii-production:ro \
  --env-file "$stage/target.env" \
  --mount "type=bind,source=$probe_binary,target=/catalog-transport-probe,readonly" \
  --entrypoint /catalog-transport-probe postgres:15-alpine
printf 'Development source healthy; recreating production target with its existing immutable image.\n'
prod_changed=true
install -m 600 "$stage/target.env" "$prod_dir/.env.runtime"
recreate "$prod_dir" molii-production
[[ $(fingerprint molii-production) == "$prod_fingerprint" ]]

network=$(docker inspect --format '{{.HostConfig.NetworkMode}}' molii-production)
docker run --rm --pull never --network "$network" --volumes-from molii-production:ro \
  --env-file "$prod_dir/.env.runtime" \
  --mount "type=bind,source=$probe_binary,target=/catalog-transport-probe,readonly" \
  --entrypoint /catalog-transport-probe postgres:15-alpine
for host in dev.molii.co molii.co; do
  curl --fail --silent --show-error --max-time 20 "https://$host/api/status" | jq -e '.success == true' >/dev/null
  printf '%s health verified.\n' "$host"
done
printf 'Awaiting authenticated target status verification: %s\n' "$stage"
verified=false
for ((attempt=1; attempt<=150; attempt++)); do
  if [[ -f "$stage/target-status-verified" ]]; then verified=true; break; fi
  sleep 2
done
[[ "$verified" == true ]] || { printf 'Target browser status was not verified; rolling back.\n' >&2; exit 1; }
trap - EXIT HUP INT TERM
printf 'Runtime configuration complete; no catalog changes applied.\nSource backup: %s\nTarget backup: %s\n' "$dev_backup" "$prod_backup"
rm -f -- "$stage/source.env" "$stage/target.env" "$stage/db-state" "$stage/db-error" "$stage/compose-log" "$stage/config-log" "$stage/rendered.json" "$stage/image.json" "$stage/live.json" "$stage/target-status-verified"
rmdir -- "$stage"
