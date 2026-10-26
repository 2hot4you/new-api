#!/usr/bin/env bash
# Shared runner/server gate. Approval is transient; never read it from runtime configuration.
quote_remote_shell() {
  local value=$1
  value=${value//\'/\'\\\'\'}
  printf "'%s'" "$value"
}

check_maintenance_window() {
  local target=${1:-} current current_day current_hour
  case "$target" in
    development|production-molii|production-claudeye|production-model-claudeye) return 0 ;;
    production-ixiaozu)
      if [[ ${IXIAOZU_MAINTENANCE_APPROVED:-} != true || ! -f /usr/share/zoneinfo/Asia/Shanghai ]]; then
        printf '[maintenance] iXiaozu requires separate explicit approval and Beijing timezone data\n' >&2
        return 1
      fi
      current=$(LC_ALL=C TZ=Asia/Shanghai date '+%F %H') || return 1
      if [[ ! "$current" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}\ [0-9]{2}$ ]]; then
        printf '[maintenance] cannot establish current Beijing time\n' >&2
        return 1
      fi
      current_day=${current% *}
      current_hour=${current##* }
      if [[ ${IXIAOZU_MAINTENANCE_DATE:-} != "$current_day" || "$current_hour" < 20 || "$current_hour" > 23 ]]; then
        printf '[maintenance] iXiaozu approval must be dated today; allowed Beijing window is 20:00–23:59\n' >&2
        return 1
      fi
      ;;
    *) printf '[maintenance] unsupported environment\n' >&2; return 1 ;;
  esac
}
