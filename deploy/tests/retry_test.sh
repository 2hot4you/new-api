#!/usr/bin/env bash

set -Eeuo pipefail

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
RETRY_SCRIPT="$PROJECT_ROOT/deploy/retry.sh"
FIXTURE=$(mktemp -d)
trap 'rm -rf "$FIXTURE"' EXIT

cat >"$FIXTURE/transient-command" <<'MOCK'
#!/usr/bin/env bash
set -Eeuo pipefail
count=0
if [[ -f "$RETRY_TEST_COUNT_FILE" ]]; then
  count=$(<"$RETRY_TEST_COUNT_FILE")
fi
count=$((count + 1))
printf '%s' "$count" >"$RETRY_TEST_COUNT_FILE"
if (( count < 3 )); then
  exit 28
fi
MOCK
chmod +x "$FIXTURE/transient-command"

output=$(
  RETRY_TEST_COUNT_FILE="$FIXTURE/count" \
    RETRY_ATTEMPTS=4 \
    RETRY_INITIAL_DELAY_SECONDS=0 \
    RETRY_MAX_DELAY_SECONDS=0 \
    bash "$RETRY_SCRIPT" 'transient network command' "$FIXTURE/transient-command" 2>&1
)

[[ "$(<"$FIXTURE/count")" == '3' ]]
[[ "$output" == *'transient network command failed (attempt 1/4, exit 28)'* ]]
[[ "$output" == *'transient network command succeeded on attempt 3/4'* ]]

set +e
RETRY_TEST_COUNT_FILE="$FIXTURE/permanent-count" \
  RETRY_ATTEMPTS=2 \
  RETRY_INITIAL_DELAY_SECONDS=0 \
  RETRY_MAX_DELAY_SECONDS=0 \
  bash "$RETRY_SCRIPT" 'permanent network command' "$FIXTURE/transient-command" \
  >"$FIXTURE/permanent-output" 2>&1
status=$?
set -e

[[ "$status" == '28' ]]
[[ "$(<"$FIXTURE/permanent-count")" == '2' ]]
grep -q 'permanent network command exhausted 2 attempts' "$FIXTURE/permanent-output"

printf 'retry helper tests passed\n'
