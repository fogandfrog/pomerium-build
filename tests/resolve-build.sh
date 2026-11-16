#!/usr/bin/env bash
# Test the actual resolver without network or registry credentials.
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=../config/source.env
source "$ROOT/config/source.env"
export RESOLVER_EXISTING_TAG="$PINNED_POMERIUM_VERSION-build.$BUILD_REVISION"
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
cat > "$fixture/gh" <<'MOCK'
#!/usr/bin/env bash
case "$*" in
  *releases/latest*) printf '%s\n' '{"tag_name":"v0.35.0","draft":false,"prerelease":false}' ;;
  *commits/*) printf '%040d\n' 1 ;;
  *versions*)
    case "$SCENARIO" in
      absent) echo 'gh: Not Found (HTTP 404)' >&2; exit 1 ;;
      denied) echo 'gh: Forbidden (HTTP 403)' >&2; exit 1 ;;
      exists) echo "$RESOLVER_EXISTING_TAG" ;;
      new) echo 'v0.33.6-build.1' ;;
    esac ;;
  *) exit 2 ;;
esac
MOCK
chmod +x "$fixture/gh"
export PATH="$fixture:$PATH"
export SCENARIO=absent
[[ $("$ROOT/scripts/resolve-build" pinned | jq -r .build) == true ]]
SCENARIO=exists
[[ $("$ROOT/scripts/resolve-build" pinned | jq -r .build) == false ]]
SCENARIO=new
[[ $("$ROOT/scripts/resolve-build" latest | jq -r .tag) == "v0.35.0-build.$BUILD_REVISION" ]]
SCENARIO=denied
if "$ROOT/scripts/resolve-build" pinned >/dev/null 2>&1; then echo 'Denied registry access must fail closed' >&2; exit 1; fi
if "$ROOT/scripts/resolve-build" invalid >/dev/null 2>&1; then exit 1; fi
echo 'Release resolver passed: first publication, idempotence, new release, denied registry and invalid mode'
