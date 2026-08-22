#!/bin/sh

set -eu

docs_base_url=${DOCS_BASE_URL:-/}
case "$docs_base_url" in
  *://*|*\?*|*\#*)
    echo 'DOCS_BASE_URL must be a path.' >&2
    exit 1
    ;;
esac

normalized_base_url=$(printf '%s' "$docs_base_url" | sed 's#^/*##; s#/*$##')
if [ -n "$normalized_base_url" ]; then
  docs_base_url="/$normalized_base_url/"
else
  docs_base_url='/'
fi
site_url="http://127.0.0.1:3100${docs_base_url}"
log_file="$(mktemp "${TMPDIR:-/tmp}/molii-docs-link-check.XXXXXX.log")"
docs_pid=''

cleanup() {
  if [ -n "$docs_pid" ] && kill -0 "$docs_pid" 2>/dev/null; then
    kill "$docs_pid" 2>/dev/null || true
    wait "$docs_pid" 2>/dev/null || true
  fi
  rm -f "$log_file"
}

trap cleanup EXIT HUP INT TERM

./node_modules/.bin/docusaurus serve --host 127.0.0.1 --port 3100 >"$log_file" 2>&1 &
docs_pid=$!

attempt=0
while :; do
  attempt=$((attempt + 1))
  if ! kill -0 "$docs_pid" 2>/dev/null; then
    echo 'The documentation preview server exited before it became ready.' >&2
    cat "$log_file" >&2
    exit 1
  fi
  if curl --fail --silent "$site_url" | grep -Fq '<title data-rh="true">Molii 开发者文档</title>'; then
    # A different process may already own port 3100. Confirm that the preview
    # process which we started is still alive before accepting the response.
    sleep 1
    if kill -0 "$docs_pid" 2>/dev/null; then
      break
    fi
    echo 'Port 3100 responded, but the documentation preview server exited.' >&2
    cat "$log_file" >&2
    exit 1
  fi
  if [ "$attempt" -ge 30 ]; then
    echo 'Timed out waiting 30 seconds for the documentation preview server.' >&2
    cat "$log_file" >&2
    exit 1
  fi
  sleep 1
done

if [ "${1:-}" = '--external' ]; then
  ./node_modules/.bin/linkinator "$site_url" --recurse --check-fragments
  exit $?
fi

./node_modules/.bin/linkinator "$site_url" --recurse --check-fragments \
  --skip '^https?://(?!127[.]0[.]0[.]1:3100)'
