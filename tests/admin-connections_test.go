package envoy

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pomerium/pomerium/pkg/netutil"
)

// Exercise the actual readiness check with Envoy's chunked /ready response.
// Go 1.27 drains an early-closed response into its transport's idle pool;
// a new transport per check must explicitly close that pool.
func TestIndependentAdminConnectionsReleased(t *testing.T) {
	var open atomic.Int64
	httpServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.(http.Flusher).Flush()
		_, _ = io.WriteString(w, "LIVE\n")
	}))
	httpServer.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			open.Add(1)
		case http.StateClosed, http.StateHijacked:
			open.Add(-1)
		}
	}
	httpServer.Start()
	defer httpServer.Close()
	address := netutil.NewInternalAddressForTCP(netip.MustParseAddrPort(httpServer.Listener.Addr().String()))
	srv := &Server{ServerOptions: ServerOptions{adminAddress: *address}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for range 20 {
		if err := srv.envoyReady(ctx); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for open.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if remaining := open.Load(); remaining != 0 {
		t.Fatalf("readiness checks retained %d admin connections", remaining)
	}
}
