package catalogtransport

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wireSnapshot(t *testing.T) catalogmanifest.Snapshot {
	t.Helper()
	s := catalogmanifest.Snapshot{SchemaVersion: catalogmanifest.SchemaVersion, SourceID: "dev", Complete: true, ExportedAt: 1, Capabilities: catalogmanifest.RequiredCapabilities(), Coverage: map[string]int{}, Entries: []catalogmanifest.Entry{}}
	for _, kind := range catalogmanifest.Kinds() {
		s.Coverage[kind] = 0
	}
	var err error
	s.Digest, err = catalogmanifest.SnapshotDigest(s)
	require.NoError(t, err)
	require.NoError(t, catalogmanifest.ValidateSnapshot(s))
	return s
}
func wireBody(t *testing.T, s catalogmanifest.Snapshot) []byte {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.ApiSuccess(ctx, s)
	return recorder.Body.Bytes()
}

// The transport still validates public DNS answers and uses fixed host/SNI,
// normal certificate validation and its real HTTP stack. Only the low-level
// TCP dial maps that already-validated public address to the local TLS server.
func controlledTransport(t *testing.T, handler http.HandlerFunc, names ...string) (*http.Client, *atomic.Int64) {
	t.Helper()
	if len(names) == 0 {
		names = []string{"dev.molii.co"}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: names[0]}, DNSNames: names, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	pk, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	pair, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}))
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
	server := httptest.NewUnstartedServer(handler)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	resolutions := &atomic.Int64{}
	lookup := func(ctx context.Context, host string) ([]netip.Addr, error) {
		assert.Equal(t, "dev.molii.co", host)
		resolutions.Add(1)
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		assert.Equal(t, "tcp", network)
		assert.Equal(t, "93.184.216.34:443", address)
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}
	client := newClient(lookup, dial, roots)
	t.Cleanup(client.CloseIdleConnections)
	return client, resolutions
}

func TestCatalogTransportFixedTLSAndEnvelope(t *testing.T) {
	expected := wireSnapshot(t)
	var calls atomic.Int64
	client, resolutions := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "dev.molii.co", r.Host)
		assert.Equal(t, "dev.molii.co", r.TLS.ServerName)
		assert.Equal(t, "/api/catalog_sync/export", r.URL.RequestURI())
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "Bearer reader.fake-test-only", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(wireBody(t, expected))
	})
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	got, err := fetch(context.Background(), client, "reader.fake-test-only", "dev")
	require.NoError(t, err)
	assert.Equal(t, expected, got)
	assert.Equal(t, int64(1), calls.Load())
	assert.Equal(t, int64(1), resolutions.Load())
	badClient, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) { t.Error("wrong certificate reached HTTP handler") }, "wrong.example")
	_, err = fetch(context.Background(), badClient, "secret", "dev")
	assert.ErrorIs(t, err, ErrUnavailable)
	client.Transport.(*fixedTransport).roots = x509.NewCertPool()
	_, err = fetch(context.Background(), client, "secret", "dev")
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestCatalogTransportRejectsUnsafeDestinationsAndRebinding(t *testing.T) {
	for _, raw := range []string{"0.0.0.0", "0.1.2.3", "10.0.0.1", "100.64.0.1", "127.0.0.1", "169.254.169.254", "172.16.0.1", "192.0.0.9", "192.0.2.1", "192.88.99.1", "192.168.0.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "224.0.0.1", "240.0.0.1", "255.255.255.255", "::", "::1", "::ffff:93.184.216.34", "64:ff9b::808:808", "100::1", "2001::1", "2001:db8::1", "2002:0808:0808::1", "3fff::1", "fc00::1", "fe80::1", "ff02::1", "fe80::1%en0"} {
		t.Run(raw, func(t *testing.T) {
			var dials int
			client := newClient(func(context.Context, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr(raw)}, nil
			}, func(context.Context, string, string) (net.Conn, error) {
				dials++
				return nil, fmt.Errorf("must not dial")
			}, nil)
			_, err := fetch(context.Background(), client, "secret", "dev")
			assert.ErrorIs(t, err, ErrUnavailable)
			assert.Zero(t, dials)
		})
	}
	for _, raw := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		assert.True(t, publicAddress(netip.MustParseAddr(raw)))
	}
	var lookups, dials int
	client := newClient(func(context.Context, string) ([]netip.Addr, error) {
		lookups++
		if lookups > 1 {
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}, func(_ context.Context, _, address string) (net.Conn, error) {
		dials++
		assert.Equal(t, "93.184.216.34:443", address)
		return nil, fmt.Errorf("unavailable secret")
	}, nil)
	_, err := fetch(context.Background(), client, "secret", "dev")
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.Equal(t, 1, lookups)
	assert.Equal(t, 1, dials)
	assert.NotContains(t, err.Error(), "secret")
}

func TestCatalogTransportRejectsWireFailures(t *testing.T) {
	valid := wireBody(t, wireSnapshot(t))
	for _, tc := range []struct {
		name     string
		status   int
		body     []byte
		encoding string
	}{
		{"redirect", 302, valid, ""}, {"unavailable", 503, []byte("private-upstream-body"), ""},
		{"raw-snapshot", 200, func() []byte { b, _ := common.Marshal(wireSnapshot(t)); return b }(), ""},
		{"false-success", 200, bytes.Replace(valid, []byte(`"success":true`), []byte(`"success":false`), 1), ""},
		{"missing-data", 200, []byte(`{"success":true,"message":""}`), ""},
		{"null-data", 200, []byte(`{"success":true,"message":"","data":null}`), ""},
		{"message", 200, bytes.Replace(valid, []byte(`"message":""`), []byte(`"message":"private"`), 1), ""},
		{"unknown", 200, append([]byte(`{"extra":1,`), valid[1:]...), ""},
		{"duplicate", 200, append([]byte(`{"success":true,`), valid[1:]...), ""},
		{"trailing", 200, append(append([]byte{}, valid...), []byte(` {}`)...), ""},
		{"truncated", 200, valid[:len(valid)-10], ""},
		{"unsupported-encoding", 200, valid, "br"},
		{"invalid-gzip", 200, valid, "gzip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			client, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "https://dev.molii.co/secondary")
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", tc.encoding)
				w.WriteHeader(tc.status)
				_, _ = w.Write(tc.body)
			})
			s, err := fetch(context.Background(), client, "private-token", "dev")
			require.ErrorIs(t, err, ErrUnavailable)
			assert.False(t, s.Complete)
			assert.NotContains(t, err.Error(), "private")
			assert.Equal(t, int64(1), calls.Load())
		})
	}
	for _, change := range []string{"source", "schema", "coverage", "digest", "capabilities", "partial", "unknown-field"} {
		t.Run(change, func(t *testing.T) {
			s := wireSnapshot(t)
			switch change {
			case "source":
				s.SourceID = "other"
			case "schema":
				s.SchemaVersion++
			case "coverage":
				s.Coverage[catalogmanifest.KindModel]++
			case "digest":
				s.Digest = "wrong"
			case "capabilities":
				s.Capabilities["unexpected"] = "yes"
			case "partial":
				s.Complete = false
			}
			body := wireBody(t, s)
			if change == "unknown-field" {
				body = bytes.Replace(body, []byte(`"source_id"`), []byte(`"unknown":true,"source_id"`), 1)
			}
			client, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
			})
			got, err := fetch(context.Background(), client, "secret", "dev")
			require.ErrorIs(t, err, ErrUnavailable)
			assert.False(t, got.Complete)
		})
	}
}

func TestCatalogTransportDecompressionLimitAndTruncation(t *testing.T) {
	valid := wireBody(t, wireSnapshot(t))
	for _, tc := range []struct {
		name       string
		extra      int
		compressed bool
		length     string
	}{
		{"exact-limit", 10*1024*1024 - len(valid), false, ""},
		{"oversize", 10*1024*1024 + 1 - len(valid), false, ""},
		{"gzip-exact", 10*1024*1024 - len(valid), true, ""},
		{"gzip-bomb", 10*1024*1024 + 1 - len(valid), true, ""},
		{"truncated-length", 0, false, "999999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(append([]byte{}, valid...), []byte(strings.Repeat(" ", tc.extra))...)
			if tc.compressed {
				var out bytes.Buffer
				gz := gzip.NewWriter(&out)
				_, err := gz.Write(body)
				require.NoError(t, err)
				require.NoError(t, gz.Close())
				body = out.Bytes()
			}
			client, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if tc.compressed {
					w.Header().Set("Content-Encoding", "gzip")
				}
				if tc.length != "" {
					w.Header().Set("Content-Length", tc.length)
				}
				_, _ = w.Write(body)
			})
			got, err := fetch(context.Background(), client, "secret", "dev")
			if strings.Contains(tc.name, "exact") {
				require.NoError(t, err)
				assert.True(t, got.Complete)
			} else {
				require.ErrorIs(t, err, ErrUnavailable)
				assert.False(t, got.Complete)
			}
		})
	}
}

func TestCatalogTransportDeadlineAndCancellation(t *testing.T) {
	for _, delay := range []time.Duration{40 * time.Millisecond, 0} {
		client, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
		ctx, cancel := context.WithCancel(context.Background())
		if delay > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), delay)
		} else {
			cancel()
		}
		start := time.Now()
		_, err := fetch(ctx, client, "secret", "dev")
		cancel()
		assert.ErrorIs(t, err, ErrUnavailable)
		assert.Less(t, time.Since(start), 2*time.Second)
	}
	var observed time.Duration
	client := newClient(func(ctx context.Context, _ string) ([]netip.Addr, error) {
		deadline, ok := ctx.Deadline()
		assert.True(t, ok)
		observed = time.Until(deadline)
		return nil, fmt.Errorf("stop before any actual DNS")
	}, nil, nil)
	_, err := fetch(context.Background(), client, "secret", "dev")
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.Greater(t, observed, 14*time.Second)
	assert.LessOrEqual(t, observed, 15*time.Second)
}

func TestCatalogTransportConcurrentRequestCancellation(t *testing.T) {
	client, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(wireBody(t, wireSnapshot(t)))
	})
	fixed := client.Transport.(*fixedTransport)
	original := fixed.lookup
	type requestKey struct{}
	firstEntered, secondEntered, releaseSecond, firstStopped := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	fixed.lookup = func(ctx context.Context, host string) ([]netip.Addr, error) {
		if ctx.Value(requestKey{}) == "first" {
			close(firstEntered)
			<-ctx.Done()
			close(firstStopped)
			return nil, ctx.Err()
		}
		close(secondEntered)
		select {
		case <-releaseSecond:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return original(ctx, host)
	}
	firstCtx, cancel := context.WithCancel(context.WithValue(context.Background(), requestKey{}, "first"))
	defer cancel()
	first, second := make(chan error, 1), make(chan error, 1)
	go func() { _, err := fetch(firstCtx, client, "secret", "dev"); first <- err }()
	<-firstEntered
	go func() { _, err := fetch(context.Background(), client, "secret", "dev"); second <- err }()
	<-secondEntered
	cancel()
	select {
	case <-firstStopped:
	case <-time.After(time.Second):
		t.Fatal("canceled request left resolver running")
	}
	require.ErrorIs(t, <-first, ErrUnavailable)
	close(releaseSecond)
	require.NoError(t, <-second, "concurrent request owns an independent context")
}

func TestCatalogTransportActualFifteenSecondTimeout(t *testing.T) {
	client, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	start := time.Now()
	_, err := fetch(context.Background(), client, "secret", "dev")
	require.ErrorIs(t, err, ErrUnavailable)
	assert.GreaterOrEqual(t, time.Since(start), 14*time.Second)
	assert.Less(t, time.Since(start), 18*time.Second)
}

func TestCatalogTransportDuplicateHeaders(t *testing.T) {
	for _, header := range []string{"Content-Encoding", "Content-Type"} {
		t.Run(header, func(t *testing.T) {
			client, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if header == "Content-Encoding" {
					w.Header().Add(header, "identity")
					w.Header().Add(header, "br")
				} else {
					w.Header().Add(header, "text/plain")
				}
				_, _ = w.Write(wireBody(t, wireSnapshot(t)))
			})
			_, err := fetch(context.Background(), client, "secret", "dev")
			require.ErrorIs(t, err, ErrUnavailable)
		})
	}
}

func TestCatalogTransportPreservesPricesAndRejectsInvalidExpression(t *testing.T) {
	s := wireSnapshot(t)
	modelJSON, err := common.Marshal(catalogmanifest.ModelValue{ModelName: "raw-model", BillingCurrency: "CNY", Status: 1})
	require.NoError(t, err)
	s.Entries = append(s.Entries, catalogmanifest.Entry{Kind: catalogmanifest.KindModel, Key: "raw-model", Value: string(modelJSON)})
	s.Coverage[catalogmanifest.KindModel] = 1
	key, err := catalogmanifest.EncodePriceKey(catalogmanifest.PriceKey{Option: "billing_setting.billing_expr", Model: "raw-model", Path: "/raw-model"})
	require.NoError(t, err)
	expression := `"v1:p * 2.5 + c * 15"`
	price := catalogmanifest.PriceValue{Value: expression, BillingCurrency: "CNY", Unit: catalogmanifest.PriceOptions()["billing_setting.billing_expr"].Unit}
	priceJSON, err := common.Marshal(price)
	require.NoError(t, err)
	s.Entries = append(s.Entries, catalogmanifest.Entry{Kind: catalogmanifest.KindModelPrice, Key: key, Value: string(priceJSON)})
	s.Coverage[catalogmanifest.KindModelPrice] = 1
	s.Digest, err = catalogmanifest.SnapshotDigest(s)
	require.NoError(t, err)
	client, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(wireBody(t, s))
	})
	got, err := fetch(context.Background(), client, "secret", "dev")
	require.NoError(t, err)
	assert.Equal(t, s, got, "no currency conversion, expression rewrite or default generation")
	price.Value = `"not an expression"`
	priceJSON, err = common.Marshal(price)
	require.NoError(t, err)
	s.Entries[1].Value = string(priceJSON)
	s.Digest, err = catalogmanifest.SnapshotDigest(s)
	require.NoError(t, err)
	got, err = fetch(context.Background(), client, "secret", "dev")
	require.ErrorIs(t, err, ErrUnavailable)
	assert.False(t, got.Complete)
}

func TestCatalogTransportBodyCancellationAndCorruptGzip(t *testing.T) {
	t.Run("body-cancellation", func(t *testing.T) {
		started := make(chan struct{})
		client, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"success":`)
			w.(http.Flusher).Flush()
			close(started)
			<-r.Context().Done()
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		finished := make(chan error, 1)
		go func() { _, err := fetch(ctx, client, "secret", "dev"); finished <- err }()
		<-started
		cancel()
		select {
		case err := <-finished:
			require.ErrorIs(t, err, ErrUnavailable)
		case <-time.After(time.Second):
			t.Fatal("body read ignored cancellation")
		}
	})
	for _, corruption := range []string{"checksum", "truncated", "trailing-member"} {
		t.Run(corruption, func(t *testing.T) {
			var out bytes.Buffer
			gz := gzip.NewWriter(&out)
			_, err := gz.Write(wireBody(t, wireSnapshot(t)))
			require.NoError(t, err)
			require.NoError(t, gz.Close())
			body := out.Bytes()
			switch corruption {
			case "checksum":
				body[len(body)-8] ^= 0xff
			case "truncated":
				body = body[:len(body)-3]
			case "trailing-member":
				body = append(body, []byte("bad-member")...)
			}
			client, _ := controlledTransport(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", "gzip")
				_, _ = w.Write(body)
			})
			_, err = fetch(context.Background(), client, "secret", "dev")
			require.ErrorIs(t, err, ErrUnavailable)
		})
	}
}
