#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/oci-retry.sh"
FIXTURES=$(mktemp -d)
trap 'rm -rf "$FIXTURES"' EXIT
MODE=
sleep() { echo "$1" >> "$FIXTURES/sleeps"; }
command_fixture() {
  local count
  count=$(cat "$FIXTURES/count")
  echo "$((count + 1))" > "$FIXTURES/count"
  if [[ $MODE == success || ( $MODE != exhausted && $count -gt 0 ) ]]; then return 0; fi
  case "$MODE" in
    throttled|exhausted) echo 'OCI download error: 429 Too Many Requests' ;;
    compact) echo 'TOOMANYREQUESTS: slow down' ;;
    header) printf '429 Too Many Requests\nRetry-After: 301\n' ;;
    budget) printf '429 Too Many Requests\nRetry-After: 901\n' ;;
    quota) echo 'You have reached your unauthenticated pull rate limit.' ;;
    denied) echo 'unauthorized: authentication required' ;;
    findings) echo 'CRITICAL vulnerabilities found' ;;
    report-number) echo 'Total: 429 (CRITICAL: 1)' ;;
  esac
  return 17
}
check() {
  MODE=$1
  local expected=$2 calls=$3 waits=$4 status=0
  echo 0 > "$FIXTURES/count"; : > "$FIXTURES/sleeps"
  oci_retry command_fixture > "$FIXTURES/output" 2>&1 || status=$?
  [[ $status == "$expected" && $(cat "$FIXTURES/count") == "$calls" && $(paste -sd, "$FIXTURES/sleeps") == "$waits" ]] \
    || { echo "Failed OCI retry fixture: $MODE"; cat "$FIXTURES/output"; exit 1; }
}
check success 0 1 ''
check throttled 0 2 '60'
check compact 0 2 '60'
check header 0 2 '301'
check budget 17 1 ''
check quota 17 1 ''
check denied 17 1 ''
check findings 17 1 ''
check report-number 17 1 ''
check exhausted 17 4 '60,120,240'
echo 'OCI retry fixtures passed (10 scenarios)'
# Exercise the actual launcher: original binary, argument boundaries, exit code.
cat > "$FIXTURES/mock-trivy" <<'MOCK'
#!/usr/bin/env bash
printf '%s\0' "$@" > "$OCI_MOCK_ARGS"
echo 'CRITICAL vulnerabilities found'
exit 23
MOCK
chmod +x "$FIXTURES/mock-trivy"
status=0
OCI_TRIVY_BINARY="$FIXTURES/mock-trivy" OCI_MOCK_ARGS="$FIXTURES/args" \
  bash "$(dirname "${BASH_SOURCE[0]}")/trivy-with-retry.sh" image 'path with spaces' \
  > "$FIXTURES/launcher.log" 2>&1 || status=$?
[[ $status == 23 ]]
mapfile -d '' -t received < "$FIXTURES/args"
[[ ${#received[@]} == 2 && ${received[0]} == image && ${received[1]} == 'path with spaces' ]]
echo 'Trivy launcher preserves arguments and security-finding exit status'
