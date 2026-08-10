#!/bin/zsh
set -euo pipefail

script_dir="${0:A:h}"
docs_dir="${script_dir:h}"

export DOCS_ENV="${DOCS_ENV:-development}"
export DOCS_SITE_URL="${DOCS_SITE_URL:-http://127.0.0.1:3100}"
export DOCS_BASE_URL="${DOCS_BASE_URL:-/}"
export DOCS_API_BASE_URL="${DOCS_API_BASE_URL:-http://127.0.0.1:3000}"

cd "$docs_dir"
exec /opt/homebrew/bin/bun run dev
