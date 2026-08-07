#!/bin/zsh
set -euo pipefail

source_dir="${MOLII_DEMO_SOURCE_DIR:-/Users/naf/Documents/Codex/Projects/molii.co/new-api/tools/molii-aigc-demo}"
runtime_dir="${MOLII_DEMO_RUNTIME_DIR:-/Users/naf/Library/Application Support/Molii/aigc-demo}"
mirror_dir="$runtime_dir/source"
state_file="$runtime_dir/repository.state"

mkdir -p "$mirror_dir"
chmod 700 "$runtime_dir" "$mirror_dir"

source_state() {
  /usr/bin/find "$source_dir" -type f \( -name '*.go' -o -name '*.html' -o -name '*.css' -o -name '*.js' -o -name 'go.mod' -o -name 'go.sum' \) -exec /usr/bin/stat -f '%m %z %N' {} \; | /usr/bin/sort | /usr/bin/shasum -a 256 | /usr/bin/awk '{print $1}'
}

sync_once() {
  local next_state
  next_state="$(source_state)"
  local saved_state=""
  [[ -f "$state_file" ]] && saved_state="$(<"$state_file")"
  if [[ "$next_state" == "$saved_state" && -f "$mirror_dir/go.mod" ]]; then
    return
  fi
  echo "[$(/bin/date '+%Y-%m-%d %H:%M:%S')] syncing repository source to protected runtime mirror"
  /usr/bin/rsync -a --delete \
    --exclude '.env' \
    --exclude 'var/' \
    --exclude 'scripts/' \
    "$source_dir/" "$mirror_dir/"
  print -r -- "$next_state" >| "$state_file"
}

sync_once
while true; do
  /bin/sleep 2
  sync_once
done
