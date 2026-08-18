#!/usr/bin/env bash

set -Eeuo pipefail

readonly ENVIRONMENT=${1:-}
readonly IMAGE_REFERENCE=${2:-}
readonly REQUESTED_HEALTH_URL=${3:-}
readonly DEPLOY_ROOT=${DEPLOY_ROOT:-/opt/molii}
readonly HEALTH_ATTEMPTS=${HEALTH_ATTEMPTS:-36}
readonly HEALTH_INTERVAL_SECONDS=${HEALTH_INTERVAL_SECONDS:-5}

log() {
  printf '[deploy] %s\n' "$*"
}

die() {
  printf '[deploy] error: %s\n' "$*" >&2
  exit 1
}

case "$ENVIRONMENT" in
  production)
    readonly DEPLOY_DIR="$DEPLOY_ROOT/production"
    readonly HOST_PORT=3000
    readonly CONTAINER_NAME=molii-production
    readonly EXPECTED_HEALTH_URL=https://molii.co/api/status
    ;;
  development)
    readonly DEPLOY_DIR="$DEPLOY_ROOT/development"
    readonly HOST_PORT=3010
    readonly CONTAINER_NAME=molii-development
    readonly EXPECTED_HEALTH_URL=https://dev.molii.co/api/status
    ;;
  *)
    die "unsupported environment '$ENVIRONMENT'; expected production or development"
    ;;
esac

[[ -n "$IMAGE_REFERENCE" ]] || die 'image reference is required'
[[ "$IMAGE_REFERENCE" =~ ^[a-zA-Z0-9./:@_-]+$ ]] || die 'image reference contains unsupported characters'
[[ "$REQUESTED_HEALTH_URL" == "$EXPECTED_HEALTH_URL" ]] || die "health URL must be $EXPECTED_HEALTH_URL"
[[ "$HEALTH_ATTEMPTS" =~ ^[1-9][0-9]*$ ]] || die 'HEALTH_ATTEMPTS must be a positive integer'
[[ "$HEALTH_INTERVAL_SECONDS" =~ ^[0-9]+$ ]] || die 'HEALTH_INTERVAL_SECONDS must be a non-negative integer'

for command_name in docker curl flock; do
  command -v "$command_name" >/dev/null 2>&1 || die "required command is unavailable: $command_name"
done

[[ -d "$DEPLOY_DIR" ]] || die "deployment directory does not exist: $DEPLOY_DIR"
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

write_deploy_env() {
  local image=$1
  local temporary_file="$DEPLOY_ENV.tmp"
  {
    printf 'IMAGE=%s\n' "$image"
    printf 'HOST_PORT=%s\n' "$HOST_PORT"
    printf 'CONTAINER_NAME=%s\n' "$CONTAINER_NAME"
    printf 'DEPLOY_ENV=%s\n' "$ENVIRONMENT"
    printf 'COMPOSE_PROJECT_NAME=molii-%s\n' "$ENVIRONMENT"
  } >"$temporary_file"
  chmod 600 "$temporary_file"
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
  log "last container logs for $CONTAINER_NAME"
  docker logs --tail 100 "$CONTAINER_NAME" 2>&1 || true
}

previous_image=$(docker inspect --format '{{.Config.Image}}' "$CONTAINER_NAME" 2>/dev/null || true)

rollback() {
  if [[ -z "$previous_image" || "$previous_image" == "$IMAGE_REFERENCE" ]]; then
    log 'rollback unavailable because no distinct previous image exists'
    return 1
  fi

  log "rolling back $ENVIRONMENT to $previous_image"
  write_deploy_env "$previous_image"
  if ! docker compose --env-file "$DEPLOY_ENV" up -d --remove-orphans; then
    log 'rollback Compose update failed'
    return 1
  fi
  if ! wait_for_container; then
    log 'rollback container did not become healthy'
    return 1
  fi
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

log "deploying $IMAGE_REFERENCE to $ENVIRONMENT"
write_deploy_env "$IMAGE_REFERENCE"

docker compose --env-file "$DEPLOY_ENV" config --quiet || fail_release 'Compose validation failed'
docker compose --env-file "$DEPLOY_ENV" pull || fail_release 'image pull failed'
docker compose --env-file "$DEPLOY_ENV" up -d --remove-orphans || fail_release 'Compose update failed'
wait_for_container || fail_release 'container health check failed'
check_public_health || fail_release "public health check failed: $EXPECTED_HEALTH_URL"

log "deployment succeeded: $ENVIRONMENT is healthy at $EXPECTED_HEALTH_URL"
