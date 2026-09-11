#!/usr/bin/env bash

set -Eeuo pipefail

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
DEPLOY_SCRIPT="$PROJECT_ROOT/docs-site/deploy/deploy.sh"
TEST_COUNT=0
FAILED_COUNT=0
ARTIFACT_PATH=''
ARTIFACT_SHA=''

pass() {
  TEST_COUNT=$((TEST_COUNT + 1))
  printf 'ok %d - %s\n' "$TEST_COUNT" "$1"
}

fail() {
  TEST_COUNT=$((TEST_COUNT + 1))
  FAILED_COUNT=$((FAILED_COUNT + 1))
  printf 'not ok %d - %s\n' "$TEST_COUNT" "$1" >&2
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

new_fixture() {
  local fixture
  fixture=$(mktemp -d)
  mkdir -p \
    "$fixture/public/molii.co/index/docs" \
    "$fixture/public/dev.molii.co/index/docs" \
    "$fixture/public/aigc.ixiaozu.cn/index/docs" \
    "$fixture/private/molii/production/data" \
    "$fixture/private/molii/development/data" \
    "$fixture/private/ixiaozu/production/data" \
    "$fixture/bin"

  cat >"$fixture/bin/flock" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK

  cat >"$fixture/bin/curl" <<'MOCK'
#!/usr/bin/env bash
set -Eeuo pipefail

headers=''
output=''
url=''
while (($#)); do
  case "$1" in
    --dump-header|-D)
      headers=$2
      shift 2
      ;;
    --output|-o)
      output=$2
      shift 2
      ;;
    --write-out|-w|--max-time|--connect-timeout)
      shift 2
      ;;
    --silent|--show-error|--fail|--head|-I)
      shift
      ;;
    *)
      url=$1
      shift
      ;;
  esac
done

case "$url" in
  */docs|*/docs/)
    if [[ -n "$headers" ]]; then
      redirect_base=${url%/}
      printf 'HTTP/2 308\r\nLocation: %s/quick-start\r\n\r\n' "$redirect_base" >"$headers"
    fi
    printf '308'
    ;;
  */docs/quick-start)
    if [[ "${MOCK_HEALTH:-success}" == failed ]]; then
      exit 22
    elif [[ "${MOCK_HEALTH:-success}" == large ]]; then
      body="<title data-rh=\"true\">${MOCK_SITE_MARKER:-Molii 开发者文档}</title>"
      if [[ -n "$output" ]]; then
        printf '%s' "$body" >"$output"
      else
        printf '%s' "$body"
        exit 23
      fi
    elif [[ "${MOCK_HEALTH:-success}" == missing-marker ]]; then
      body='<title data-rh="true">Unexpected documentation</title>'
      if [[ -n "$output" ]]; then
        printf '%s' "$body" >"$output"
      else
        printf '%s' "$body"
      fi
    else
      body="<title data-rh=\"true\">${MOCK_SITE_MARKER:-Molii 开发者文档}</title><script src=\"/docs/assets/main.js\"></script>"
      if [[ -n "$output" ]]; then
        printf '%s' "$body" >"$output"
      else
        printf '%s' "$body"
      fi
    fi
    ;;
  *)
    exit 22
    ;;
esac
MOCK

  chmod +x "$fixture/bin/flock" "$fixture/bin/curl"
  printf '%s\n' "$fixture"
}

make_artifact() {
  local fixture=$1
  local target=$2
  local release_id=$3
  local marker=${4:-Molii 开发者文档}
  local private_target
  local source="$fixture/source-$release_id"

  case "$target" in
    development) private_target="$fixture/private/molii/development" ;;
    production-molii) private_target="$fixture/private/molii/production" ;;
    production-ixiaozu) private_target="$fixture/private/ixiaozu/production" ;;
    *) private_target="$fixture/private/unknown" ;;
  esac

  mkdir -p "$source/quick-start" "$source/assets"
  printf '<title data-rh="true">%s</title><script src="/docs/assets/main.js"></script>\n' "$marker" >"$source/quick-start/index.html"
  printf 'new-%s\n' "$release_id" >"$source/assets/main.js"
  printf '<title data-rh="true">%s</title>\n' "$marker" >"$source/index.html"

  mkdir -p "$private_target"
  ARTIFACT_PATH="$private_target/docs-$target-$release_id.tar.gz"
  tar -C "$source" -czf "$ARTIFACT_PATH" .
  ARTIFACT_SHA=$(sha256sum "$ARTIFACT_PATH" | awk '{print $1}')
}

run_deploy() {
  local fixture=$1
  shift
  PATH="$fixture/bin:$PATH" \
    DOCS_DEPLOY_ROOT="$fixture/public" \
    MOLII_PRIVATE_ROOT="$fixture/private/molii" \
    IXIAOZU_PRIVATE_ROOT="$fixture/private/ixiaozu" \
    FLOCK_BIN="$fixture/bin/flock" \
    "$DEPLOY_SCRIPT" "$@"
}

test_requires_deploy_script() {
  if [[ -f "$DEPLOY_SCRIPT" ]]; then
    pass 'deployment script exists'
  else
    fail 'deployment script exists'
  fi
}

test_rejects_unknown_environment() {
  local fixture output status
  fixture=$(new_fixture)
  set +e
  output=$(run_deploy "$fixture" qa release "$fixture/archive.tar.gz" deadbeef https://qa.example 'QA Docs' 2>&1)
  status=$?
  set -e
  if ((status != 0)); then
    pass 'unknown environment returns nonzero'
  else
    fail 'unknown environment returns nonzero'
  fi
  assert_contains "$output" 'unsupported environment' 'unknown environment explains the failure'
}

test_validation_failure_does_not_delete_unowned_files() {
  local fixture protected_file status
  fixture=$(new_fixture)
  protected_file="$fixture/protected.txt"
  printf 'keep\n' >"$protected_file"

  set +e
  run_deploy "$fixture" qa release "$protected_file" 0000000000000000000000000000000000000000000000000000000000000000 https://qa.example 'QA Docs' >/dev/null 2>&1
  status=$?
  set -e
  if ((status != 0)); then
    pass 'validation failure returns nonzero before ownership is established'
  else
    fail 'validation failure returns nonzero before ownership is established'
  fi
  if [[ -f "$protected_file" ]]; then
    pass 'validation failure preserves unowned files'
  else
    fail 'validation failure preserves unowned files'
  fi
}

test_rejects_checksum_mismatch() {
  local fixture output status
  fixture=$(new_fixture)
  make_artifact "$fixture" development release-a
  set +e
  output=$(run_deploy "$fixture" development release-a "$ARTIFACT_PATH" 0000000000000000000000000000000000000000000000000000000000000000 https://dev.molii.co 'Molii 开发者文档' 2>&1)
  status=$?
  set -e
  if ((status != 0)); then
    pass 'checksum mismatch returns nonzero'
  else
    fail 'checksum mismatch returns nonzero'
  fi
  assert_contains "$output" 'checksum mismatch' 'checksum mismatch is identified'
}

test_rejects_archives_with_links() {
  local fixture source output status
  fixture=$(new_fixture)
  source="$fixture/unsafe-source"
  mkdir -p "$source/quick-start" "$source/assets"
  printf '<title data-rh="true">Molii 开发者文档</title>\n' >"$source/quick-start/index.html"
  printf 'asset\n' >"$source/assets/main.js"
  ln -s /etc/passwd "$source/unsafe-link"
  ARTIFACT_PATH="$fixture/private/molii/development/docs-development-unsafe.tar.gz"
  tar -C "$source" -czf "$ARTIFACT_PATH" .
  ARTIFACT_SHA=$(sha256sum "$ARTIFACT_PATH" | awk '{print $1}')

  set +e
  output=$(run_deploy "$fixture" development unsafe "$ARTIFACT_PATH" "$ARTIFACT_SHA" https://dev.molii.co 'Molii 开发者文档' 2>&1)
  status=$?
  set -e
  if ((status != 0)); then
    pass 'archive links return nonzero'
  else
    fail 'archive links return nonzero'
  fi
  assert_contains "$output" 'unsupported archive entry' 'archive links are identified'
}

test_development_publish_replaces_stale_content() {
  local fixture public_dir content
  fixture=$(new_fixture)
  public_dir="$fixture/public/dev.molii.co/index/docs"
  mkdir -p "$public_dir/quick-start"
  printf 'old\n' >"$public_dir/quick-start/index.html"
  printf 'stale\n' >"$public_dir/stale.txt"
  make_artifact "$fixture" development release-b

  run_deploy "$fixture" development release-b "$ARTIFACT_PATH" "$ARTIFACT_SHA" https://dev.molii.co 'Molii 开发者文档'

  content=$(<"$public_dir/assets/main.js")
  assert_equals "$content" 'new-release-b' 'development publishes the requested artifact'
  if [[ ! -e "$public_dir/stale.txt" ]]; then
    pass 'successful publish deletes stale files'
  else
    fail 'successful publish deletes stale files'
  fi
}

test_ixiaozu_publish_uses_independent_roots_and_marker() {
  local fixture public_dir content
  fixture=$(new_fixture)
  public_dir="$fixture/public/aigc.ixiaozu.cn/index/docs"
  mkdir -p "$public_dir/quick-start"
  printf 'old\n' >"$public_dir/quick-start/index.html"
  make_artifact "$fixture" production-ixiaozu release-ixiaozu 'iXiaozu 开发者文档'

  MOCK_SITE_MARKER='iXiaozu 开发者文档' run_deploy \
    "$fixture" production-ixiaozu release-ixiaozu "$ARTIFACT_PATH" "$ARTIFACT_SHA" \
    https://aigc.ixiaozu.cn 'iXiaozu 开发者文档'

  content=$(<"$public_dir/assets/main.js")
  assert_equals "$content" 'new-release-ixiaozu' 'iXiaozu publishes into its independent public root'
  if [[ -d "$fixture/private/ixiaozu/production/data/docs-deploy" ]]; then
    pass 'iXiaozu keeps deployment state in its independent private root'
  else
    fail 'iXiaozu keeps deployment state in its independent private root'
  fi
}

test_rejects_cross_site_origin() {
  local fixture output status
  fixture=$(new_fixture)
  make_artifact "$fixture" production-ixiaozu release-origin 'iXiaozu 开发者文档'

  set +e
  output=$(run_deploy "$fixture" production-ixiaozu release-origin "$ARTIFACT_PATH" "$ARTIFACT_SHA" https://molii.co 'iXiaozu 开发者文档' 2>&1)
  status=$?
  set -e
  if ((status != 0)); then
    pass 'cross-site origin returns nonzero'
  else
    fail 'cross-site origin returns nonzero'
  fi
  assert_contains "$output" 'site origin does not match' 'cross-site origin explains the failure'
}

test_failed_health_check_restores_previous_content() {
  local fixture public_dir state_dir content leftovers status
  fixture=$(new_fixture)
  public_dir="$fixture/public/dev.molii.co/index/docs"
  state_dir="$fixture/private/molii/development/data/docs-deploy"
  mkdir -p "$public_dir/quick-start"
  printf 'previous\n' >"$public_dir/quick-start/index.html"
  printf 'keep\n' >"$public_dir/previous.txt"
  make_artifact "$fixture" development release-c

  set +e
  MOCK_HEALTH=failed run_deploy "$fixture" development release-c "$ARTIFACT_PATH" "$ARTIFACT_SHA" https://dev.molii.co 'Molii 开发者文档' >/dev/null 2>&1
  status=$?
  set -e
  if ((status != 0)); then
    pass 'failed health check returns nonzero'
  else
    fail 'failed health check returns nonzero'
  fi
  content=$(<"$public_dir/quick-start/index.html")
  assert_equals "$content" 'previous' 'failed health check restores the prior quick-start page'
  if [[ -f "$public_dir/previous.txt" && ! -e "$public_dir/assets/main.js" ]]; then
    pass 'rollback restores the complete prior tree'
  else
    fail 'rollback restores the complete prior tree'
  fi
  leftovers=$(find "$state_dir" -maxdepth 1 -type f -name 'health-body-*' -print)
  assert_equals "$leftovers" '' 'failed health check removes its temporary response file'
}

test_large_health_response_does_not_fail_after_marker_match() {
  local fixture public_dir content status
  fixture=$(new_fixture)
  public_dir="$fixture/public/dev.molii.co/index/docs"
  mkdir -p "$public_dir/quick-start"
  printf 'previous\n' >"$public_dir/quick-start/index.html"
  make_artifact "$fixture" development release-large

  set +e
  MOCK_HEALTH=large run_deploy "$fixture" development release-large "$ARTIFACT_PATH" "$ARTIFACT_SHA" https://dev.molii.co 'Molii 开发者文档' >/dev/null 2>&1
  status=$?
  set -e
  assert_equals "$status" '0' 'large public page remains a successful health check'
  if [[ -f "$public_dir/assets/main.js" ]]; then
    content=$(<"$public_dir/assets/main.js")
  else
    content='missing'
  fi
  assert_equals "$content" 'new-release-large' 'large public page keeps the new documentation release'
}

test_missing_marker_restores_previous_content() {
  local fixture public_dir content status
  fixture=$(new_fixture)
  public_dir="$fixture/public/dev.molii.co/index/docs"
  mkdir -p "$public_dir/quick-start"
  printf 'previous\n' >"$public_dir/quick-start/index.html"
  make_artifact "$fixture" development release-missing-marker

  set +e
  MOCK_HEALTH=missing-marker run_deploy "$fixture" development release-missing-marker "$ARTIFACT_PATH" "$ARTIFACT_SHA" https://dev.molii.co 'Molii 开发者文档' >/dev/null 2>&1
  status=$?
  set -e
  if ((status != 0)); then
    pass 'missing site marker returns nonzero'
  else
    fail 'missing site marker returns nonzero'
  fi
  content=$(<"$public_dir/quick-start/index.html")
  assert_equals "$content" 'previous' 'missing site marker restores the prior quick-start page'
}

test_health_body_does_not_follow_predictable_symlink() {
  local fixture state_dir protected_file content leftovers
  fixture=$(new_fixture)
  state_dir="$fixture/private/molii/development/data/docs-deploy"
  protected_file="$fixture/protected-health-body.txt"
  mkdir -p "$state_dir"
  printf 'keep\n' >"$protected_file"
  ln -s "$protected_file" "$state_dir/health-body-release-symlink"
  make_artifact "$fixture" development release-symlink

  run_deploy "$fixture" development release-symlink "$ARTIFACT_PATH" "$ARTIFACT_SHA" https://dev.molii.co 'Molii 开发者文档' >/dev/null

  content=$(<"$protected_file")
  assert_equals "$content" 'keep' 'health check does not overwrite a predictable symlink target'
  if [[ -L "$state_dir/health-body-release-symlink" ]]; then
    pass 'health check preserves an unrelated pre-existing symlink'
  else
    fail 'health check preserves an unrelated pre-existing symlink'
  fi
  leftovers=$(find "$state_dir" -maxdepth 1 -type f -name 'health-body-release-symlink.*' -print)
  assert_equals "$leftovers" '' 'health check removes its temporary response file'
}

test_requires_deploy_script
test_rejects_unknown_environment
test_validation_failure_does_not_delete_unowned_files
test_rejects_checksum_mismatch
test_rejects_archives_with_links
test_development_publish_replaces_stale_content
test_ixiaozu_publish_uses_independent_roots_and_marker
test_rejects_cross_site_origin
test_failed_health_check_restores_previous_content
test_large_health_response_does_not_fail_after_marker_match
test_missing_marker_restores_previous_content
test_health_body_does_not_follow_predictable_symlink

printf '1..%d\n' "$TEST_COUNT"
if ((FAILED_COUNT > 0)); then
  exit 1
fi
