#!/usr/bin/env bash
set -Eeuo pipefail

# No default database, implicit legacy TestMain, or successful optional skip.
: "${TEST_POSTGRES_DSN:?TEST_POSTGRES_DSN is required for the catalog PostgreSQL gate}"
task_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
exec python3 "$task_root/deploy/tests/test_catalog_postgres_gate.py" --run "${1:-normal}"
