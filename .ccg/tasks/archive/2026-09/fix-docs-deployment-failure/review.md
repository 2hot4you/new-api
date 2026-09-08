# Review

## Root cause

The public documentation health check piped `curl` directly into `grep -q` while `pipefail` was enabled. Once the generated page became large enough, `grep` exited immediately after matching the title and closed the pipe. `curl` then returned code 23 (`Failure writing output to destination`), so a valid deployment was treated as failed and rolled back.

## Fix

- Download the health-check response completely into a `mktemp` file under the locked deployment state directory.
- Check the HTTP transfer result separately from the expected site marker.
- Remove the temporary response file on both success and rollback paths.
- Preserve all redirect checks and rollback behavior.

## Regression coverage

- Large successful response that models curl code 23 when streamed into an early-closing consumer.
- HTTP request failure and rollback.
- HTTP 200 response without the expected site marker and rollback.
- Success- and failure-path temporary file cleanup.
- Pre-existing predictable symlink is neither followed nor removed.

## Review findings

- Critical: none.
- Warning: none after adding failure-path cleanup coverage.
- Info: `shellcheck` is not installed locally; `bash -n`, behavioral deployment tests, documentation workflow contract tests, and `git diff --check` cover this scoped change.
