#!/bin/zsh
set -euo pipefail

source_dir="${MOLII_DEMO_SOURCE_DIR:-/Users/naf/Documents/Codex/Projects/molii.co/new-api/tools/molii-aigc-demo}"
runtime_dir="${MOLII_DEMO_RUNTIME_DIR:-/Users/naf/Library/Application Support/Molii/aigc-demo}"
keychain_service="${MOLII_DEMO_KEYCHAIN_SERVICE:-com.molii.aigc-demo.master-key}"
binary_path="$runtime_dir/molii-aigc-demo"
state_path="$runtime_dir/source.state"
child_pid=""

mkdir -p "$runtime_dir"
chmod 700 "$runtime_dir"

export MOLII_DEMO_MASTER_KEY="$(/usr/bin/security find-generic-password -a "$USER" -s "$keychain_service" -w)"
export MOLII_DEMO_ADDR="${MOLII_DEMO_ADDR:-127.0.0.1:8788}"
export MOLII_DEMO_DB="${MOLII_DEMO_DB:-$runtime_dir/molii-aigc-demo.db}"

source_state() {
  /usr/bin/find "$source_dir" -type f \( -name '*.go' -o -name '*.html' -o -name '*.css' -o -name '*.js' -o -name 'go.mod' -o -name 'go.sum' \) -exec /usr/bin/stat -f '%m %z %N' {} \; | /usr/bin/sort | /usr/bin/shasum -a 256 | /usr/bin/awk '{print $1}'
}

build_binary() {
  local temporary="$runtime_dir/molii-aigc-demo.new"
  echo "[$(/bin/date '+%Y-%m-%d %H:%M:%S')] source changed; rebuilding"
  /bin/rm -f "$temporary"
  if ! (
    cd "$source_dir"
    GOWORK=off /opt/homebrew/bin/go build -trimpath -o "$temporary" ./cmd/server
  ); then
    /bin/rm -f "$temporary"
    return 1
  fi
  /bin/chmod 700 "$temporary"
  /bin/mv -f "$temporary" "$binary_path"
}

stop_child() {
  if [[ -n "$child_pid" ]] && /bin/kill -0 "$child_pid" 2>/dev/null; then
    /bin/kill -TERM "$child_pid" 2>/dev/null || true
    for _ in {1..50}; do
      /bin/kill -0 "$child_pid" 2>/dev/null || break
      /bin/sleep 0.1
    done
    /bin/kill -KILL "$child_pid" 2>/dev/null || true
    wait "$child_pid" 2>/dev/null || true
  fi
  child_pid=""
}

start_child() {
  echo "[$(/bin/date '+%Y-%m-%d %H:%M:%S')] starting Demo at $MOLII_DEMO_ADDR"
  "$binary_path" -addr "$MOLII_DEMO_ADDR" -db "$MOLII_DEMO_DB" &
  child_pid=$!
}

cleanup() {
  stop_child
}
trap cleanup EXIT INT TERM HUP

current_state="$(source_state)"
saved_state=""
[[ -f "$state_path" ]] && saved_state="$(<"$state_path")"
if [[ ! -x "$binary_path" || "$current_state" != "$saved_state" ]]; then
  build_binary
  print -r -- "$current_state" >| "$state_path"
fi
start_child

while true; do
  /bin/sleep 2
  next_state="$(source_state)"
  if [[ "$next_state" != "$current_state" ]]; then
    current_state="$next_state"
    if build_binary; then
      print -r -- "$current_state" >| "$state_path"
      stop_child
      start_child
    else
      echo "[$(/bin/date '+%Y-%m-%d %H:%M:%S')] build failed; keeping the previous process"
    fi
  elif ! /bin/kill -0 "$child_pid" 2>/dev/null; then
    wait "$child_pid" 2>/dev/null || true
    start_child
  fi
done
