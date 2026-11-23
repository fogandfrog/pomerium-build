#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/bin" "$fixture/upstream/internal/analytics" "$fixture/upstream/pkg/envoy/files"
cat > "$fixture/bin/git" <<'MOCK'
#!/usr/bin/env bash
printf '%040d\n' 1
MOCK
chmod +x "$fixture/bin/git"
export PATH="$fixture/bin:$PATH"
cp "$ROOT/LICENSE" "$fixture/upstream/LICENSE"
touch "$fixture/upstream/pkg/envoy/files/files_linux_amd64.go"
printf 'module fixture\n' > "$fixture/upstream/go.mod"
printf -v commit '%040d' 1
# Reconstruct the unpatched hunk contexts for the packaging fixture. The
# behavioral test runs against the complete upstream functions in the build.
awk -v root="$fixture/upstream/" '
  /^--- a\// {file = root substr($0, 7); next}
  /^diff |^index |^\+\+\+ |^@@/ {next}
  /^[ -]/ {print substr($0, 2) > file}
' "$ROOT/patches/envoy-admin-close-idle.patch"
"$ROOT/scripts/prepare-source" "$fixture/upstream" "$commit"
test -f "$fixture/upstream/internal/analytics/independent_source_build_test.go"
test -f "$fixture/upstream/independent-build/embed-envoy/main.go"
test -f "$fixture/upstream/pkg/envoy/independent_admin_connections_test.go"
test "$(grep -c 'defer client.CloseIdleConnections()' "$fixture/upstream/pkg/envoy/envoy.go")" = 2
test "$(grep -c 'defer client.CloseIdleConnections()' "$fixture/upstream/pkg/envoy/dynamic_extensions.go")" = 1
test "$(cat "$fixture/upstream/go.mod")" = 'module fixture'
if "$ROOT/scripts/prepare-source" "$fixture/upstream" "$commit" >/dev/null 2>&1; then echo 'Repeated backport must fail' >&2; exit 1; fi
printf 'restricted license\n' > "$fixture/upstream/LICENSE"
if "$ROOT/scripts/prepare-source" "$fixture/upstream" "$commit" >/dev/null 2>&1; then echo 'License change must fail' >&2; exit 1; fi
cp "$ROOT/LICENSE" "$fixture/upstream/LICENSE"
if "$ROOT/scripts/prepare-source" "$fixture/upstream" bad-commit >/dev/null 2>&1; then echo 'Checkout mismatch must fail' >&2; exit 1; fi
echo 'Preparation passed: module untouched; license and checkout mismatches rejected'
