#!/usr/bin/env bash
# Run inside the offline Go container: compile the actual upstream functions
# with only their address/server scaffolding, avoiding unrelated dependencies.
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
upstream=${1:?Usage: admin-connections.sh UPSTREAM_DIRECTORY}
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
printf 'module regression\n\ngo 1.27\n' > "$fixture/go.mod"
cat > "$fixture/envoy.go" <<'GO'
package envoy
import("context";"fmt";"net";"net/http";"net/netip";"net/url";"sync")
type address struct { endpoint string }
func (a address) Dial(ctx context.Context) (net.Conn,error) { return (&net.Dialer{}).DialContext(ctx,"tcp",a.endpoint) }
func NewInternalAddressForTCP(endpoint netip.AddrPort) *address { return &address{endpoint.String()} }
type ServerOptions struct { adminAddress address }
type Server struct { ServerOptions; mu sync.Mutex }
GO
awk '
  /^func envoyAdminClient\(/ || /^func \(srv \*Server\) envoyReady\(/ {keep=1}
  keep {print}
  keep && /^}/ {keep=0}
' "$upstream/pkg/envoy/envoy.go" | sed 's/netutil.InternalAddress/address/g' >> "$fixture/envoy.go"
sed -e '/"github.com\/pomerium\/pomerium\/pkg\/netutil"/d' \
    -e 's/netutil.NewInternalAddressForTCP/NewInternalAddressForTCP/g' \
    "$ROOT/tests/admin-connections_test.go" > "$fixture/admin_test.go"
cd "$fixture"
go test -v -count=1
