#!/usr/bin/env bash
# Source this file. Wrap only idempotent pulls or read-only scanner commands.
oci_retry() (
  set -o pipefail
  local attempt=1 delay=60 waited=0 requested status log
  log=$(mktemp)
  trap 'rm -f "$log"' EXIT
  while :; do
    if "$@" 2>&1 | tee "$log"; then return 0; else status=$?; fi
    if grep -Eiq 'you have reached your (unauthenticated )?pull rate limit|pull rate limit exceeded' "$log"; then
      echo '::error::Docker Hub pull quota exhausted; wait for its reset or use suitable authentication.' >&2
      return "$status"
    fi
    if ! grep -Eiq 'too[[:space:]]*many[[:space:]]*requests|toomanyrequests|(error|fatal|failed|download|http|unexpected status).*[^0-9]429([^0-9]|$)' "$log" \
      || (( attempt >= 4 )); then return "$status"; fi
    requested=$(sed -nE 's/.*[Rr]etry-[Aa]fter:[[:space:]]*([0-9]{1,6})[[:space:]]*$/\1/p' "$log" | tail -n 1)
    if [[ -n $requested ]] && (( 10#$requested > delay )); then delay=$((10#$requested)); fi
    if (( delay > 900 - waited )); then
      echo '::error::Registry backoff would exceed the 900-second total wait budget.' >&2
      return "$status"
    fi
    printf 'Temporary registry throttling; retry %s/4 in %s seconds.\n' "$((attempt + 1))" "$delay" >&2
    sleep "$delay"
    waited=$((waited + delay))
    delay=$((delay * 2))
    attempt=$((attempt + 1))
  done
)
