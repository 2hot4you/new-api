#!/usr/bin/env bash
set -Eeuo pipefail
helper="$(dirname "${BASH_SOURCE[0]}")/maintenance-window.sh"
source "$helper"
target=${DEPLOY_ENVIRONMENT:-${TARGET_ID:-}}
check_maintenance_window "$target"
if [[ "$target" != production-ixiaozu || ( ${1:-} != ssh && ${1:-} != scp ) ]]; then
  exec "$@"
fi

# Bootstrap the trusted gate over the connection itself: login and the first
# asset upload must not depend on an older helper already on the server.
transport=$1
shift
ssh_options=()
while (($#)) && [[ $1 == -* ]]; do
  case "$1" in
    -p|-P) ssh_options+=(-p "$2"); shift 2 ;;
    -o) ssh_options+=(-o "$2"); shift 2 ;;
    *) printf '[maintenance] unsupported remote transport option\n' >&2; exit 1 ;;
  esac
done

remote_run() {
  local body=$1 program remote_command
  check_maintenance_window "$target" || return 1
  printf -v program '%s\nexport IXIAOZU_MAINTENANCE_APPROVED=%q IXIAOZU_MAINTENANCE_DATE=%q\ncheck_maintenance_window %q || exit 1\n%s' \
    "$(cat "$helper")" "$IXIAOZU_MAINTENANCE_APPROVED" "$IXIAOZU_MAINTENANCE_DATE" "$target" "$body"
  remote_command="bash -c $(quote_remote_shell "$program")"
  ssh "${ssh_options[@]}" "$host" "$remote_command"
}

if [[ "$transport" == ssh ]]; then
  host=${1:?missing SSH host}
  shift
  [[ -n "$*" ]] || { printf '[maintenance] remote command required\n' >&2; exit 1; }
  remote_run "$*"
else
  (($# >= 2)) || { printf '[maintenance] upload source and destination required\n' >&2; exit 1; }
  for destination in "$@"; do :; done
  [[ "$destination" == *:/* ]] || { printf '[maintenance] absolute remote upload destination required\n' >&2; exit 1; }
  host=${destination%%:*}
  remote_path=${destination#*:}
  while (($# > 1)); do
    file=$1
    shift
    [[ -f "$file" ]] || { printf '[maintenance] upload requires a regular file\n' >&2; exit 1; }
    output_path=$remote_path
    [[ "$output_path" != */ ]] || output_path+=$(basename "$file")
    printf -v body 'set -Eeuo pipefail\numask 077\noutput=%q\ntemporary=$(mktemp "$output.pending.XXXXXX")\ntrap '\''check_maintenance_window production-ixiaozu && rm -f -- "$temporary"'\'' EXIT\ncat > "$temporary"\ncheck_maintenance_window production-ixiaozu\nmv -- "$temporary" "$output"' "$output_path"
    remote_run "$body" < "$file"
  done
fi
