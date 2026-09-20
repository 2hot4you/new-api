#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
readonly SCRIPT_DIR
readonly RETRY_HELPER="$SCRIPT_DIR/retry.sh"
readonly ENVIRONMENT=${1:-}
readonly IMAGE_REFERENCE=${2:-}
readonly REQUESTED_HEALTH_URL=${3:-}
readonly MOLII_DEPLOY_ROOT=${MOLII_DEPLOY_ROOT:-/opt/molii}
readonly IXIAOZU_DEPLOY_ROOT=${IXIAOZU_DEPLOY_ROOT:-/opt/ixiaozu}
readonly CLAUDEYE_DEPLOY_ROOT=${CLAUDEYE_DEPLOY_ROOT:-/opt/claudeye}
readonly HEALTH_ATTEMPTS=${HEALTH_ATTEMPTS:-36}
readonly HEALTH_INTERVAL_SECONDS=${HEALTH_INTERVAL_SECONDS:-5}

[[ -f "$RETRY_HELPER" ]] || {
  printf '[deploy] error: missing retry helper: %s\n' "$RETRY_HELPER" >&2
  exit 1
}
# shellcheck source-path=SCRIPTDIR
# shellcheck source=retry.sh
source "$RETRY_HELPER"

log() {
  printf '[deploy] %s\n' "$*"
}

die() {
  printf '[deploy] error: %s\n' "$*" >&2
  exit 1
}

rollback_result=not_attempted
result_file=
report_result() {
  printf 'DEPLOY_ROLLBACK_RESULT=%s\n' "$rollback_result"
  if [[ -n "$result_file" ]]; then
    printf 'DEPLOY_ROLLBACK_RESULT=%s\n' "$rollback_result" >"$result_file" || true
  fi
}
trap report_result EXIT

case "$ENVIRONMENT" in
  production-molii)
    readonly DEPLOY_DIR="$MOLII_DEPLOY_ROOT/production"
    readonly HOST_PORT=3000
    readonly CONTAINER_NAME=molii-production
    readonly EXPECTED_HEALTH_URL=https://molii.co/api/status
    readonly COMPOSE_PROJECT_NAME=molii-production
    ;;
  production-ixiaozu)
    readonly DEPLOY_DIR="$IXIAOZU_DEPLOY_ROOT/production"
    readonly HOST_PORT=3000
    readonly CONTAINER_NAME=ixiaozu-production
    readonly EXPECTED_HEALTH_URL=https://aigc.ixiaozu.cn/api/status
    readonly COMPOSE_PROJECT_NAME=ixiaozu-production
    ;;
  production-claudeye)
    readonly DEPLOY_DIR="$CLAUDEYE_DEPLOY_ROOT/production"
    readonly HOST_PORT=3000
    readonly CONTAINER_NAME=claudeye-production
    readonly EXPECTED_HEALTH_URL=https://claudeye.com/api/status
    readonly COMPOSE_PROJECT_NAME=claudeye-production
    readonly INFRA_BACKEND_NETWORK=claudeye-production-infra-backend
    readonly APP_MEMORY_LIMIT=1024m
    ;;
  production-model-claudeye)
    readonly DEPLOY_DIR="$CLAUDEYE_DEPLOY_ROOT/production"
    readonly HOST_PORT=3000
    readonly CONTAINER_NAME=model-claudeye-production
    readonly EXPECTED_HEALTH_URL=https://model.claudeye.com/api/status
    readonly COMPOSE_PROJECT_NAME=model-claudeye-production
    readonly INFRA_BACKEND_NETWORK=model-claudeye-production-infra-backend
    readonly APP_MEMORY_LIMIT=768m
    ;;
  development)
    readonly DEPLOY_DIR="$MOLII_DEPLOY_ROOT/development"
    readonly HOST_PORT=3010
    readonly CONTAINER_NAME=molii-development
    readonly EXPECTED_HEALTH_URL=https://dev.molii.co/api/status
    readonly COMPOSE_PROJECT_NAME=molii-development
    ;;
  *)
    die "unsupported environment '$ENVIRONMENT'; expected a supported deployment target"
    ;;
esac

if [[ -n "${INFRA_BACKEND_NETWORK:-}" ]]; then
  [[ "$IMAGE_REFERENCE" =~ ^[a-zA-Z0-9./_-]+@sha256:[a-f0-9]{64}$ ]] || die 'new sites require an immutable sha256 image digest'
fi

[[ -n "$IMAGE_REFERENCE" ]] || die 'image reference is required'
[[ "$IMAGE_REFERENCE" =~ ^[a-zA-Z0-9./:@_-]+$ ]] || die 'image reference contains unsupported characters'
[[ "$REQUESTED_HEALTH_URL" == "$EXPECTED_HEALTH_URL" ]] || die "health URL must be $EXPECTED_HEALTH_URL"
[[ "$HEALTH_ATTEMPTS" =~ ^[1-9][0-9]*$ ]] || die 'HEALTH_ATTEMPTS must be a positive integer'
[[ "$HEALTH_INTERVAL_SECONDS" =~ ^[0-9]+$ ]] || die 'HEALTH_INTERVAL_SECONDS must be a non-negative integer'

for command_name in docker curl flock; do
  command -v "$command_name" >/dev/null 2>&1 || die "required command is unavailable: $command_name"
done

[[ -d "$DEPLOY_DIR" ]] || die "deployment directory does not exist: $DEPLOY_DIR"
mkdir -p "$DEPLOY_DIR/certs"
chmod 0750 "$DEPLOY_DIR/certs"
cd "$DEPLOY_DIR"

readonly RUNTIME_ENV=.env.runtime
readonly DEPLOY_ENV=.deploy.env
readonly COMPOSE_FILE=docker-compose.yml

[[ -f "$RUNTIME_ENV" ]] || die "missing runtime environment file: $DEPLOY_DIR/$RUNTIME_ENV"
[[ -f "$COMPOSE_FILE" ]] || die "missing Compose file: $DEPLOY_DIR/$COMPOSE_FILE"

for required_key in SQL_DSN REDIS_CONN_STRING SESSION_SECRET CRYPTO_SECRET; do
  if ! grep -Eq "^${required_key}=.+" "$RUNTIME_ENV"; then
    die "$RUNTIME_ENV must contain a non-empty $required_key"
  fi
done

umask 077
exec 9>"$DEPLOY_DIR/.deploy.lock"
flock -n 9 || die "another $ENVIRONMENT deployment is already running"
result_file="$DEPLOY_DIR/.deploy-result"
if [[ -n "${INFRA_BACKEND_NETWORK:-}" ]]; then
  network_internal=$(docker network inspect --format '{{.Internal}}' "$INFRA_BACKEND_NETWORK" 2>/dev/null) \
    || die 'required infrastructure backend network is unavailable'
  [[ "$network_internal" == true ]] || die 'infrastructure backend network must be internal'
fi

write_deploy_env() {
  local image=$1
  local temporary_file="$DEPLOY_ENV.tmp"
  {
    printf 'IMAGE=%s\n' "$image"
    printf 'HOST_PORT=%s\n' "$HOST_PORT"
    printf 'CONTAINER_NAME=%s\n' "$CONTAINER_NAME"
    printf 'DEPLOY_ENV=%s\n' "$ENVIRONMENT"
    printf 'COMPOSE_PROJECT_NAME=%s\n' "$COMPOSE_PROJECT_NAME"
    if [[ -n "${INFRA_BACKEND_NETWORK:-}" ]]; then
      printf 'INFRA_BACKEND_NETWORK=%s\n' "$INFRA_BACKEND_NETWORK"
      printf 'APP_MEMORY_LIMIT=%s\n' "$APP_MEMORY_LIMIT"
    fi
  } >"$temporary_file" || return 1
  chmod 600 "$temporary_file" || return 1
  mv "$temporary_file" "$DEPLOY_ENV"
}

container_health() {
  docker inspect \
    --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' \
    "$CONTAINER_NAME" 2>/dev/null || true
}

wait_for_container() {
  local attempt status
  for ((attempt = 1; attempt <= HEALTH_ATTEMPTS; attempt++)); do
    status=$(container_health)
    if [[ "$status" == healthy ]]; then
      return 0
    fi
    if [[ "$status" == exited || "$status" == dead ]]; then
      log "container entered terminal state: $status"
      return 1
    fi
    log "waiting for $CONTAINER_NAME health ($attempt/$HEALTH_ATTEMPTS, status=${status:-missing})"
    sleep "$HEALTH_INTERVAL_SECONDS"
  done
  return 1
}

check_public_health() {
  local response
  response=$(curl --silent --show-error --fail --max-time 15 "$EXPECTED_HEALTH_URL") || return 1
  grep -Eq '"success"[[:space:]]*:[[:space:]]*true' <<<"$response"
}

show_failure_logs() {
  log "Inspect container diagnostics locally on the deployment server; application logs are not published to Actions."
}

retry_public_health() {
  RETRY_ATTEMPTS="${PUBLIC_HEALTH_ATTEMPTS:-6}" \
    RETRY_INITIAL_DELAY_SECONDS="${PUBLIC_HEALTH_INITIAL_DELAY_SECONDS:-2}" \
    RETRY_MAX_DELAY_SECONDS="${PUBLIC_HEALTH_MAX_DELAY_SECONDS:-10}" \
    retry_with_backoff 'public health check' check_public_health
}

previous_image=$(docker inspect --format '{{.Config.Image}}' "$CONTAINER_NAME" 2>/dev/null || true)

rollback() {
  if [[ -z "$previous_image" || "$previous_image" == "$IMAGE_REFERENCE" ]]; then
    log 'rollback unavailable because no distinct previous image exists'
    return 1
  fi

  rollback_result=failed
  log "rolling back $ENVIRONMENT to $previous_image"
  write_deploy_env "$previous_image" || return 1
  if ! docker compose --env-file "$DEPLOY_ENV" up -d; then
    log 'rollback Compose update failed'
    return 1
  fi
  if ! wait_for_container; then
    log 'rollback container did not become healthy'
    return 1
  fi
  if ! retry_public_health; then
    log 'rollback public endpoint did not become healthy'
    return 1
  fi
  rollback_result=succeeded
  log 'rollback succeeded'
  return 0
}

fail_release() {
  local reason=$1
  log "release failed: $reason"
  show_failure_logs
  rollback || true
  exit 1
}

# Preserve deployment metadata when validation or pulling fails before recreation.
previous_deploy_env=
if [[ -f "$DEPLOY_ENV" ]]; then
  previous_deploy_env=$(cat "$DEPLOY_ENV")
fi
preflight_failure() {
  if [[ -n "$previous_deploy_env" ]]; then
    printf '%s\n' "$previous_deploy_env" >"$DEPLOY_ENV"
  else
    rm -f "$DEPLOY_ENV"
  fi
  die "$1; existing container unchanged"
}

log "deploying $IMAGE_REFERENCE to $ENVIRONMENT"
write_deploy_env "$IMAGE_REFERENCE"

docker compose --env-file "$DEPLOY_ENV" config --quiet || preflight_failure 'Compose validation failed'
RETRY_ATTEMPTS="${DEPLOY_PULL_ATTEMPTS:-5}" \
  RETRY_INITIAL_DELAY_SECONDS="${RETRY_INITIAL_DELAY_SECONDS:-3}" \
  RETRY_MAX_DELAY_SECONDS="${RETRY_MAX_DELAY_SECONDS:-30}" \
  retry_with_backoff \
    'image pull' \
    docker compose --env-file "$DEPLOY_ENV" pull \
  || preflight_failure 'image pull failed'
docker compose --env-file "$DEPLOY_ENV" up -d || fail_release 'Compose update failed'
wait_for_container || fail_release 'container health check failed'
retry_public_health || fail_release "public health check failed: $EXPECTED_HEALTH_URL"

log "deployment succeeded: $ENVIRONMENT is healthy at $EXPECTED_HEALTH_URL"
