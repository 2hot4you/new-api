#!/usr/bin/env bash

set -Eeuo pipefail

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
DEPLOY_SCRIPT="$PROJECT_ROOT/deploy/deploy.sh"
TEST_COUNT=0
FAILED_COUNT=0

pass() {
  TEST_COUNT=$((TEST_COUNT + 1))
  printf 'ok %d - %s\n' "$TEST_COUNT" "$1"
}

fail() {
  TEST_COUNT=$((TEST_COUNT + 1))
  FAILED_COUNT=$((FAILED_COUNT + 1))
  printf 'not ok %d - %s\n' "$TEST_COUNT" "$1" >&2
}

assert_contains() {
  local haystack=$1
  local needle=$2
  local label=$3
  if [[ "$haystack" == *"$needle"* ]]; then
    pass "$label"
  else
    fail "$label (missing: $needle)"
  fi
}

assert_equals() {
  local actual=$1
  local expected=$2
  local label=$3
  if [[ "$actual" == "$expected" ]]; then
    pass "$label"
  else
    fail "$label (expected: $expected, actual: $actual)"
  fi
}

assert_not_contains() {
  local haystack=$1
  local needle=$2
  local label=$3
  if [[ "$haystack" != *"$needle"* ]]; then
    pass "$label"
  else
    fail "$label (unexpected: $needle)"
  fi
}

create_mocks() {
  local fixture=$1
  mkdir -p "$fixture/bin"

  cat >"$fixture/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -Eeuo pipefail
printf '%s\n' "$*" >>"$MOCK_LOG"

if [[ "${1:-}" == "inspect" && "$*" == *".Config.Image"* ]]; then
  printf '%s\n' "${MOCK_PREVIOUS_IMAGE:-}"
  exit 0
fi

if [[ "${1:-}" == "inspect" && "$*" == *".State.Health"* ]]; then
  count=0
  if [[ -f "$MOCK_HEALTH_COUNT_FILE" ]]; then
    count=$(<"$MOCK_HEALTH_COUNT_FILE")
  fi
  count=$((count + 1))
  printf '%s' "$count" >"$MOCK_HEALTH_COUNT_FILE"
  if (( count <= ${MOCK_UNHEALTHY_ATTEMPTS:-0} )); then
    printf 'unhealthy\n'
  else
    printf 'healthy\n'
  fi
  exit 0
fi

exit 0
MOCK

  cat >"$fixture/bin/curl" <<'MOCK'
#!/usr/bin/env bash
set -Eeuo pipefail
printf '%s\n' "$*" >>"$MOCK_LOG"
if [[ "${MOCK_PUBLIC_HEALTH:-success}" == "success" ]]; then
  printf '{"success":true}\n'
  exit 0
fi
printf '{"success":false}\n'
exit 0
MOCK

  cat >"$fixture/bin/flock" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK

  cat >"$fixture/bin/sleep" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK

  chmod +x "$fixture/bin/docker" "$fixture/bin/curl" "$fixture/bin/flock" "$fixture/bin/sleep"
}

write_runtime_env() {
  local target=$1
  cat >"$target" <<'ENV'
SQL_DSN=postgresql://app:secret@postgres.example:5432/molii?sslmode=require
REDIS_CONN_STRING=redis://default:secret@redis.example:6379/0
SESSION_SECRET=session-secret-with-at-least-32-random-characters
CRYPTO_SECRET=crypto-secret-with-at-least-32-random-characters
TZ=Asia/Shanghai
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://example.com
NODE_NAME=molii-test
ENV
  chmod 600 "$target"
}

new_fixture() {
  local fixture
  fixture=$(mktemp -d)
  mkdir -p "$fixture/root/production" "$fixture/root/development"
  : >"$fixture/root/production/docker-compose.yml"
  : >"$fixture/root/development/docker-compose.yml"
  write_runtime_env "$fixture/root/production/.env.runtime"
  write_runtime_env "$fixture/root/development/.env.runtime"
  create_mocks "$fixture"
  printf '%s\n' "$fixture"
}

run_deploy() {
  local fixture=$1
  shift
  PATH="$fixture/bin:$PATH" \
    DEPLOY_ROOT="$fixture/root" \
    HEALTH_ATTEMPTS=1 \
    HEALTH_INTERVAL_SECONDS=0 \
    MOCK_LOG="$fixture/mock.log" \
    MOCK_HEALTH_COUNT_FILE="$fixture/health-count" \
    "$DEPLOY_SCRIPT" "$@"
}

test_rejects_unknown_environment() {
  local fixture output status
  fixture=$(new_fixture)
  set +e
  output=$(run_deploy "$fixture" qa ghcr.io/2hot4you/new-api@sha256:abc https://qa.example/api/status 2>&1)
  status=$?
  set -e
  if (( status != 0 )); then
    pass 'unknown environment returns nonzero'
  else
    fail 'unknown environment returns nonzero'
  fi
  assert_contains "$output" 'unsupported environment' 'unknown environment explains the failure'
  rm -rf "$fixture"
}

test_requires_runtime_secrets() {
  local fixture output status
  fixture=$(new_fixture)
  rm "$fixture/root/production/.env.runtime"
  set +e
  output=$(run_deploy "$fixture" production ghcr.io/2hot4you/new-api@sha256:abc https://molii.co/api/status 2>&1)
  status=$?
  set -e
  if (( status != 0 )); then
    pass 'missing runtime file returns nonzero'
  else
    fail 'missing runtime file returns nonzero'
  fi
  assert_contains "$output" '.env.runtime' 'missing runtime file is identified'

  write_runtime_env "$fixture/root/production/.env.runtime"
  sed -i.bak 's/^SESSION_SECRET=.*/SESSION_SECRET=/' "$fixture/root/production/.env.runtime"
  rm -f "$fixture/root/production/.env.runtime.bak"
  set +e
  output=$(run_deploy "$fixture" production ghcr.io/2hot4you/new-api@sha256:abc https://molii.co/api/status 2>&1)
  status=$?
  set -e
  if (( status != 0 )); then
    pass 'empty required secret returns nonzero'
  else
    fail 'empty required secret returns nonzero'
  fi
  assert_contains "$output" 'SESSION_SECRET' 'empty required secret is identified'
  rm -rf "$fixture"
}

test_successful_deploy_keeps_requested_image() {
  local fixture image saved_image log
  fixture=$(new_fixture)
  image='ghcr.io/2hot4you/new-api@sha256:new'
  MOCK_PREVIOUS_IMAGE='ghcr.io/2hot4you/new-api@sha256:old' \
    MOCK_UNHEALTHY_ATTEMPTS=0 \
    MOCK_PUBLIC_HEALTH=success \
    run_deploy "$fixture" production "$image" https://molii.co/api/status

  saved_image=$(sed -n 's/^IMAGE=//p' "$fixture/root/production/.deploy.env")
  assert_equals "$saved_image" "$image" 'successful deploy records the requested image'
  log=$(<"$fixture/mock.log")
  assert_contains "$log" 'compose --env-file .deploy.env pull' 'successful deploy pulls the image'
  assert_contains "$log" 'compose --env-file .deploy.env up -d --remove-orphans' 'successful deploy starts Compose'
  assert_contains "$log" 'https://molii.co/api/status' 'successful deploy checks the public endpoint'
  rm -rf "$fixture"
}

test_failed_deploy_rolls_back_previous_image() {
  local fixture output status saved_image up_count
  fixture=$(new_fixture)
  set +e
  output=$(MOCK_PREVIOUS_IMAGE='ghcr.io/2hot4you/new-api@sha256:old' \
    MOCK_UNHEALTHY_ATTEMPTS=1 \
    MOCK_PUBLIC_HEALTH=success \
    run_deploy "$fixture" production ghcr.io/2hot4you/new-api@sha256:new https://molii.co/api/status 2>&1)
  status=$?
  set -e

  if (( status != 0 )); then
    pass 'failed release remains failed after rollback'
  else
    fail 'failed release remains failed after rollback'
  fi
  assert_contains "$output" 'rollback succeeded' 'failed release reports successful rollback'
  saved_image=$(sed -n 's/^IMAGE=//p' "$fixture/root/production/.deploy.env")
  assert_equals "$saved_image" 'ghcr.io/2hot4you/new-api@sha256:old' 'rollback restores the previous image'
  up_count=$(grep -c 'compose --env-file .deploy.env up -d --remove-orphans' "$fixture/mock.log")
  assert_equals "$up_count" '2' 'rollback starts Compose a second time'
  rm -rf "$fixture"
}

test_compose_isolates_both_environments() {
  local compose_file fixture environment port container rendered summary
  compose_file="$PROJECT_ROOT/deploy/docker-compose.cicd.yml"
  if [[ ! -f "$compose_file" ]]; then
    fail 'Compose deployment file exists'
    return
  fi

  for environment in production development; do
    if [[ "$environment" == production ]]; then
      port=3000
      container=molii-production
    else
      port=3010
      container=molii-development
    fi

    fixture=$(mktemp -d)
    cp "$compose_file" "$fixture/docker-compose.yml"
    write_runtime_env "$fixture/.env.runtime"
    cat >"$fixture/.deploy.env" <<ENV
IMAGE=ghcr.io/2hot4you/new-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
HOST_PORT=$port
CONTAINER_NAME=$container
DEPLOY_ENV=$environment
COMPOSE_PROJECT_NAME=molii-$environment
ENV

    rendered=$(cd "$fixture" && docker compose --env-file .deploy.env config --format json)
    summary=$(python3 -c '
import json, sys
config = json.load(sys.stdin)
services = config["services"]
app = services["new-api"]
port = app["ports"][0]
print("services=" + ",".join(sorted(services)))
print("container=" + app["container_name"])
print("host_ip=" + port["host_ip"])
print("published=" + str(port["published"]))
print("target=" + str(port["target"]))
' <<<"$rendered")

    assert_contains "$summary" 'services=new-api' "$environment Compose contains only the application service"
    assert_contains "$summary" "container=$container" "$environment uses an isolated container name"
    assert_contains "$summary" 'host_ip=127.0.0.1' "$environment binds only to loopback"
    assert_contains "$summary" "published=$port" "$environment publishes the expected host port"
    assert_contains "$summary" 'target=3000' "$environment targets the application port"
    rm -rf "$fixture"
  done
}

test_workflow_delivery_contract() {
  local workflow content
  workflow="$PROJECT_ROOT/.github/workflows/deploy.yml"
  if [[ ! -f "$workflow" ]]; then
    fail 'deployment workflow exists'
    return
  fi

  content=$(<"$workflow")
  assert_contains "$content" '- main' 'workflow deploys main pushes'
  assert_contains "$content" '- develop' 'workflow deploys develop pushes'
  assert_contains "$content" 'packages: write' 'workflow can publish GHCR images'
  assert_contains "$content" "ghcr.io/\${{ github.repository }}" 'workflow publishes to the repository GHCR package'
  assert_contains "$content" "@\${{ needs.build.outputs.digest }}" 'workflow deploys an immutable image digest'
  assert_contains "$content" 'DEPLOY_SSH_KNOWN_HOSTS' 'workflow uses a pinned SSH host key secret'
  assert_not_contains "$content" 'ssh-keyscan' 'workflow does not trust a network-discovered SSH host key'
  assert_contains "$content" "group: deploy-\${{ github.ref_name }}" 'workflow serializes deployment per branch environment'
  assert_contains "$content" "if: \${{ always() }}" 'Telegram notification runs for every deployment outcome'
  assert_contains "$content" 'TELEGRAM_BOT_TOKEN' 'workflow supports Telegram bot delivery'
  assert_contains "$content" 'bash deploy/app-version.sh' 'workflow derives a schema-safe application version'
  assert_contains "$content" 'source_ref:' 'workflow accepts an explicit candidate source ref'
  assert_contains "$content" 'ref: ${{ inputs.source_ref || github.sha }}' 'candidate source ref controls verification and image checkout'
  assert_contains "$content" 'backup_postgres:' 'workflow exposes an explicit PostgreSQL backup gate'
  assert_contains "$content" 'verify_repeated_startup:' 'workflow exposes an explicit repeated-startup gate'
  assert_not_contains "$content" 'SQL_DSN' 'workflow does not receive the PostgreSQL secret'
  assert_not_contains "$content" 'REDIS_CONN_STRING' 'workflow does not receive the Redis secret'
}

test_app_version_fits_setup_schema() {
  local script sha output status
  script="$PROJECT_ROOT/deploy/app-version.sh"
  sha='6b9673b4ac7f4360fa0aab3674154a4833ff462f'

  set +e
  output=$(bash "$script" development "$sha" 2>&1)
  status=$?
  set -e

  if (( status != 0 )); then
    fail 'application version generator runs successfully'
    return
  fi

  assert_equals "$output" 'development-6b9673b4ac7f' 'application version uses a 12-character commit SHA'
  if (( ${#output} <= 50 )); then
    pass 'application version fits setups.version varchar(50)'
  else
    fail "application version fits setups.version varchar(50) (length: ${#output})"
  fi
}

printf 'TAP version 13\n'
test_rejects_unknown_environment
test_requires_runtime_secrets
test_successful_deploy_keeps_requested_image
test_failed_deploy_rolls_back_previous_image
test_compose_isolates_both_environments
test_workflow_delivery_contract
test_app_version_fits_setup_schema

if (( FAILED_COUNT > 0 )); then
  printf '# %d of %d assertions failed\n' "$FAILED_COUNT" "$TEST_COUNT" >&2
  exit 1
fi
printf '# all %d assertions passed\n' "$TEST_COUNT"
