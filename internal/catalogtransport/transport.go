// Package catalogtransport owns the single fixed catalog source connection.
package catalogtransport

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"reflect"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
)

var ErrUnavailable = errors.New("catalog source unavailable")

type fixedTransport struct {
	lookup func(context.Context, string) ([]netip.Addr, error)
	dial   func(context.Context, string, string) (net.Conn, error)
	roots  *x509.CertPool
}

func newClient(lookup func(context.Context, string) ([]netip.Addr, error), dial func(context.Context, string, string) (net.Conn, error), roots *x509.CertPool) *http.Client {
	return &http.Client{Transport: &fixedTransport{lookup: lookup, dial: dial, roots: roots}, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrUnavailable }}
}

func (fixed *fixedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport := &http.Transport{
		Proxy: nil, DisableCompression: true, DisableKeepAlives: true,
		TLSClientConfig:     &tls.Config{ServerName: "dev.molii.co", MinVersion: tls.VersionTLS12, RootCAs: fixed.roots},
		TLSHandshakeTimeout: 15 * time.Second, ResponseHeaderTimeout: 15 * time.Second, MaxResponseHeaderBytes: 64 * 1024,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			// net/http detaches the dial context from the request. Join the actual
			// request lifetime back in so DNS cannot outlive a canceled request.
			dialCtx, cancel := context.WithCancel(request.Context())
			stop := context.AfterFunc(ctx, cancel)
			defer stop()
			defer cancel()
			ctx = dialCtx
			if network != "tcp" || address != "dev.molii.co:443" {
				return nil, ErrUnavailable
			}
			addresses, err := fixed.lookup(ctx, "dev.molii.co")
			if err != nil || len(addresses) == 0 || len(addresses) > 64 {
				return nil, ErrUnavailable
			}
			// Validate the whole answer before any dial; mixed public/private DNS
			// answers fail closed. Dial only these IP literals, never re-resolve.
			for _, ip := range addresses {
				if !publicAddress(ip) {
					return nil, ErrUnavailable
				}
			}
			for _, ip := range addresses {
				conn, err := fixed.dial(ctx, "tcp", net.JoinHostPort(ip.String(), "443"))
				if err == nil {
					return conn, nil
				}
				if ctx.Err() != nil {
					break
				}
			}
			return nil, ErrUnavailable
		},
	}
	defer transport.CloseIdleConnections()
	return transport.RoundTrip(request)
}

// Conservative IANA special-purpose exclusions, including IPv4 transition and
// IPv6 documentation/transition ranges. Only native IPv6 2000::/3 is eligible.
var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("224.0.0.0/4"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("3fff::/20"),
}

func publicAddress(ip netip.Addr) bool {
	if !ip.IsValid() || ip.Zone() != "" || ip.Is4In6() || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func fetch(ctx context.Context, client *http.Client, token, sourceID string) (catalogmanifest.Snapshot, error) {
	if ctx == nil || token == "" || sourceID == "" {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://dev.molii.co/api/catalog_sync/export", nil)
	if err != nil {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	response, err := client.Do(req)
	if err != nil {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	if len(response.Header.Values("Content-Type")) != 1 || len(response.Header.Values("Content-Encoding")) > 1 {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	const limit = 10 * 1024 * 1024
	// Bound encoded input too (including gzip headers), then separately cap the
	// expanded bytes. Content-Length is never the authority for either limit.
	encoded := &io.LimitedReader{R: response.Body, N: limit + 1}
	var reader io.Reader = encoded
	switch response.Header.Get("Content-Encoding") {
	case "", "identity":
	case "gzip":
		gz, err := gzip.NewReader(encoded)
		if err != nil {
			return catalogmanifest.Snapshot{}, ErrUnavailable
		}
		defer gz.Close()
		reader = gz
	default:
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil || len(body) > limit || encoded.N <= 0 || ctx.Err() != nil {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	if common.ValidateJsonNoDuplicateKeys(body) != nil {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	var envelope map[string]common.RawMessage
	if common.Unmarshal(body, &envelope) != nil || len(envelope) != 3 {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	var success bool
	var message string
	if string(envelope["success"]) != "true" || common.Unmarshal(envelope["success"], &success) != nil || !success || string(envelope["message"]) != "\"\"" || common.Unmarshal(envelope["message"], &message) != nil {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	var snapshot catalogmanifest.Snapshot
	if !exactWireFields(envelope["data"], reflect.TypeOf(snapshot)) || common.Unmarshal(envelope["data"], &snapshot) != nil || snapshot.SourceID != sourceID || len(snapshot.ObjectVersions) > 0 {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	if catalogmanifest.ValidateSnapshot(snapshot) != nil || ctx.Err() != nil {
		return catalogmanifest.Snapshot{}, ErrUnavailable
	}
	return snapshot, nil
}

// exactWireFields derives the allowed/required keys from the authoritative
// manifest types instead of creating another manifest serializer or schema.
func exactWireFields(raw common.RawMessage, t reflect.Type) bool {
	if t.Kind() == reflect.Map {
		var values map[string]common.RawMessage
		if common.Unmarshal(raw, &values) != nil || values == nil {
			return false
		}
		for _, value := range values {
			if !exactWireFields(value, t.Elem()) {
				return false
			}
		}
		return true
	}
	if t.Kind() == reflect.Slice {
		var values []common.RawMessage
		if common.Unmarshal(raw, &values) != nil || string(raw) == "null" {
			return false
		}
		for _, value := range values {
			if !exactWireFields(value, t.Elem()) {
				return false
			}
		}
		return true
	}
	if t.Kind() != reflect.Struct {
		return string(raw) != "null"
	}
	var fields map[string]common.RawMessage
	if common.Unmarshal(raw, &fields) != nil || fields == nil {
		return false
	}
	for i := range t.NumField() {
		field := t.Field(i)
		name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
		value, exists := fields[name]
		if !exists {
			if options != "omitempty" {
				return false
			}
			continue
		}
		if !exactWireFields(value, field.Type) {
			return false
		}
		delete(fields, name)
	}
	return len(fields) == 0
}

// Fetch has no source URL, client, resolver or TLS options. The unexported
// constructor supports same-package controlled network tests only.
func Fetch(ctx context.Context, token, sourceID string) (catalogmanifest.Snapshot, error) {
	client := newClient(func(ctx context.Context, host string) ([]netip.Addr, error) {
		return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	}, (&net.Dialer{Timeout: 15 * time.Second}).DialContext, nil)
	defer client.CloseIdleConnections()
	return fetch(ctx, client, token, sourceID)
}
