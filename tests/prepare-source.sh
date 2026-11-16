#!/usr/bin/env bash
# Review gates must reject an upstream license or Envoy runtime change.
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=../config/source.env
source "$ROOT/config/source.env"
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/bin" "$fixture/upstream/internal/analytics"
cat > "$fixture/bin/git" <<'MOCK'
#!/usr/bin/env bash
printf '%040d\n' 1
MOCK
chmod +x "$fixture/bin/git"
export PATH="$fixture/bin:$PATH"
cp "$ROOT/LICENSE" "$fixture/upstream/LICENSE"
# A normal go.mod requirement is indented inside a block.
printf 'module fixture\nrequire (\n\tgithub.com/pomerium/envoy-custom %s\n)\n' "$EXPECTED_ENVOY_MODULE" > "$fixture/upstream/go.mod"
: > "$fixture/upstream/go.sum"
printf -v commit '%040d' 1
"$ROOT/scripts/prepare-source" "$fixture/upstream" "$commit"
grep -Fq "$POMERIUM_ENVOY_MODULE" "$fixture/upstream/go.mod"
test -f "$fixture/upstream/internal/analytics/independent_source_build_test.go"
printf 'restricted license\n' > "$fixture/upstream/LICENSE"
if "$ROOT/scripts/prepare-source" "$fixture/upstream" "$commit" >/dev/null 2>&1; then echo 'License change must fail' >&2; exit 1; fi
cp "$ROOT/LICENSE" "$fixture/upstream/LICENSE"
printf 'module fixture\nrequire (\n\tgithub.com/pomerium/envoy-custom v99.0.0\n)\n' > "$fixture/upstream/go.mod"
if "$ROOT/scripts/prepare-source" "$fixture/upstream" "$commit" >/dev/null 2>&1; then echo 'Runtime change must fail' >&2; exit 1; fi
echo 'Preparation passed: compatible pin accepted; license/runtime changes rejected'
