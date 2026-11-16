// Copyright 2026 Pomerium Build contributors. Licensed under Apache-2.0.
package analytics

import (
	"net/http"
	"testing"

	"github.com/pomerium/pomerium/config"
	"github.com/pomerium/pomerium/internal/version"
	configpb "github.com/pomerium/pomerium/pkg/grpc/config"
)

type rejectTransport struct{ calls int }

func (rt *rejectTransport) RoundTrip(*http.Request) (*http.Response, error) {
	rt.calls++
	return nil, http.ErrNotSupported
}

// Even enabling the analytics runtime flag must not enable reporting in our
// source build. Test the reporter's send path, without waiting for its timer.
func TestSourceBuildDoesNotReport(t *testing.T) {
	if version.IsOfficialBuild() {
		t.Fatal("This project must compile from source as an unofficial build")
	}
	name, _ := version.License()
	if name != "Apache License 2.0" {
		t.Fatalf("unexpected license: %s", name)
	}
	opts := config.NewDefaultOptions()
	if opts.LicenseType.Value != configpb.LicenseType_LICENSE_TYPE_OPEN_SOURCE {
		t.Fatal("source build must default to the open-source license type")
	}
	opts.RuntimeFlags[config.RuntimeFlagProductAnalytics] = true
	transport := &rejectTransport{}
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })
	r := New().(*reporter)
	if err := r.report(t.Context(), config.New(opts), 10, 20); err != nil {
		t.Fatal(err)
	}
	if transport.calls != 0 {
		t.Fatalf("usage reporter attempted %d outbound requests", transport.calls)
	}
}
