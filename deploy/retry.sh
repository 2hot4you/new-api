#!/usr/bin/env bash

set -Eeuo pipefail

retry_with_backoff() {
  local label=${1:-}
  shift || true

  local attempts=${RETRY_ATTEMPTS:-4}
  local delay_seconds=${RETRY_INITIAL_DELAY_SECONDS:-2}
  local max_delay_seconds=${RETRY_MAX_DELAY_SECONDS:-20}
  local attempt status next_delay

  [[ -n "$label" ]] || {
    printf '[retry] error: operation label is required\n' >&2
    return 2
  }
  (( $# > 0 )) || {
    printf '[retry] error: command is required for %s\n' "$label" >&2
    return 2
  }
  [[ "$attempts" =~ ^[1-9][0-9]*$ ]] || {
    printf '[retry] error: RETRY_ATTEMPTS must be a positive integer\n' >&2
    return 2
  }
  [[ "$delay_seconds" =~ ^[0-9]+$ ]] || {
    printf '[retry] error: RETRY_INITIAL_DELAY_SECONDS must be a non-negative integer\n' >&2
    return 2
  }
  [[ "$max_delay_seconds" =~ ^[0-9]+$ ]] || {
    printf '[retry] error: RETRY_MAX_DELAY_SECONDS must be a non-negative integer\n' >&2
    return 2
  }

  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if "$@"; then
      if (( attempt > 1 )); then
        printf '[retry] %s succeeded on attempt %d/%d\n' "$label" "$attempt" "$attempts"
      fi
      return 0
    else
      status=$?
    fi

    if (( attempt == attempts )); then
      printf '[retry] %s exhausted %d attempts (last exit %d)\n' \
        "$label" "$attempts" "$status" >&2
      return "$status"
    fi

    printf '[retry] %s failed (attempt %d/%d, exit %d); retrying in %ss\n' \
      "$label" "$attempt" "$attempts" "$status" "$delay_seconds" >&2
    sleep "$delay_seconds"

    next_delay=$((delay_seconds * 2))
    if (( next_delay > max_delay_seconds )); then
      next_delay=$max_delay_seconds
    fi
    delay_seconds=$next_delay
  done
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  retry_with_backoff "$@"
fi
