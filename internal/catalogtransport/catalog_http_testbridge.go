//go:build catalogsynctest

package catalogtransport

import (
	"context"
	"crypto/x509"
	"net"
	"net/netip"

	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
)

// FetchControlledForHTTPTest is compiled only by explicitly tagged integration
// tests. Production Fetch and its fixed destination/TLS policy are unchanged.
// This exposes the existing low-level test dependencies, never a parsed result.
func FetchControlledForHTTPTest(ctx context.Context, token, sourceID string, lookup func(context.Context, string) ([]netip.Addr, error), dial func(context.Context, string, string) (net.Conn, error), roots *x509.CertPool) (catalogmanifest.Snapshot, error) {
	client := newClient(lookup, dial, roots)
	defer client.CloseIdleConnections()
	return fetch(ctx, client, token, sourceID)
}
