package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func configureImageResultProxyTest(t *testing.T) {
	t.Helper()
	originalSecret, originalAddress := common.CryptoSecret, system_setting.ServerAddress
	common.CryptoSecret = "test-image-proxy-secret"
	system_setting.ServerAddress = "https://site.example/"
	t.Cleanup(func() { common.CryptoSecret = originalSecret; system_setting.ServerAddress = originalAddress })
	t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "true")
	t.Setenv("IMAGE_RESULT_PROXY_BASE_URL", "")
}

func TestImageResultProxyModelOptIn(t *testing.T) {
	configureImageResultProxyTest(t)
	for _, model := range []string{"gpt-image-2", "gpt-image-2.5", "gpt-image-2-high", "gpt-image-2.5-2026"} {
		require.True(t, ImageResultProxyEnabledForModel(model), model)
	}
	for _, model := range []string{"gpt-image-20", "gpt-image-2.50", "gpt-image-1", "", "other-gpt-image-2"} {
		require.False(t, ImageResultProxyEnabledForModel(model), model)
	}
	t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "false")
	require.False(t, ImageResultProxyEnabledForModel("gpt-image-2"))
}

func TestImageResultProxyTokenRoundTripAndTampering(t *testing.T) {
	configureImageResultProxyTest(t)
	source := "https://images.example/result.png?secret=private-source-token"
	result, err := BuildImageResultProxyURL(source)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(result, "https://site.example/v1/images/proxy/"))
	require.NotContains(t, result, "images.example")
	token := result[strings.LastIndex(result, "/")+1:]
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	require.NotContains(t, string(decoded), source)
	resolved, err := ResolveImageResultProxyToken(token)
	require.NoError(t, err)
	require.Equal(t, source, resolved)
	second, err := BuildImageResultProxyURL(source)
	require.NoError(t, err)
	require.NotEqual(t, result, second)
	decoded[len(decoded)-1] ^= 1
	_, err = ResolveImageResultProxyToken(base64.RawURLEncoding.EncodeToString(decoded))
	require.ErrorIs(t, err, ErrImageResultProxyTokenInvalid)
	common.CryptoSecret = "rotated-secret"
	_, err = ResolveImageResultProxyToken(token)
	require.ErrorIs(t, err, ErrImageResultProxyTokenInvalid)
	for _, malformed := range []string{"", "%%%%", "YWJj", strings.Repeat("a", 16385)} {
		_, err = ResolveImageResultProxyToken(malformed)
		require.ErrorIs(t, err, ErrImageResultProxyTokenInvalid)
	}
}

// Construct an authenticated fixture independently to exercise expiry and payload validation.
func imageResultProxyFixture(t *testing.T, source string, expires int64) string {
	t.Helper()
	payload, err := common.Marshal(map[string]any{"url": source, "expires_at": expires})
	require.NoError(t, err)
	mac := hmac.New(sha256.New, []byte(common.CryptoSecret))
	_, err = mac.Write([]byte("image-result-proxy:v1"))
	require.NoError(t, err)
	block, err := aes.NewCipher(mac.Sum(nil))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := make([]byte, aead.NonceSize())
	_, err = rand.Read(nonce)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, payload, []byte("image-result-proxy:v1")))
}

func TestImageResultProxyExpiryAndInvalidPayload(t *testing.T) {
	configureImageResultProxyTest(t)
	_, err := ResolveImageResultProxyToken(imageResultProxyFixture(t, "https://images.example/a", time.Now().Add(-time.Second).Unix()))
	require.ErrorIs(t, err, ErrImageResultProxyExpired)
	for _, source := range []string{"http://images.example/a", "https://127.0.0.1/a", "https://images.example/a#secret"} {
		_, err = ResolveImageResultProxyToken(imageResultProxyFixture(t, source, time.Now().Add(time.Hour).Unix()))
		require.ErrorIs(t, err, ErrImageResultProxyTokenInvalid)
	}
}

func TestImageResultProxyRejectsBadSourcesAndConfiguration(t *testing.T) {
	configureImageResultProxyTest(t)
	for _, source := range []string{"", "http://images.example/a", "https://u:p@images.example/a", "https://images.example/a#fragment", "https://127.0.0.1/a", "https://10.0.0.1/a", "https://[::1]/a", "https://169.254.169.254/a", "https://images.example:0/a", "https://images.example/" + strings.Repeat("a", 8192)} {
		_, err := BuildImageResultProxyURL(source)
		require.Error(t, err, source)
		require.ErrorIs(t, err, errImageResultProxyUnavailable)
	}
	for _, base := range []string{"http://site.example", "https://u:p@site.example", "https://site.example?key=x", "https://site.example#x", "/relative"} {
		t.Setenv("IMAGE_RESULT_PROXY_BASE_URL", base)
		_, err := BuildImageResultProxyURL("https://images.example/a")
		require.Error(t, err, base)
	}
	t.Setenv("IMAGE_RESULT_PROXY_BASE_URL", "https://custom.example/prefix/")
	result, err := BuildImageResultProxyURL("https://images.example/a")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(result, "https://custom.example/prefix/v1/images/proxy/"))
	common.CryptoSecret = ""
	_, err = BuildImageResultProxyURL("https://images.example/a")
	require.Error(t, err)
}

func imageResultProxyTLSClient(t *testing.T, handler http.HandlerFunc) (*http.Client, string) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	hostname := server.Certificate().DNSNames[0]
	client := newImageResultProxyHTTPClient(staticSSRFResolver{hostname: {{IP: net.ParseIP("8.8.8.8")}}}, func(ctx context.Context, network, address string) (net.Conn, error) {
		require.Equal(t, "8.8.8.8:443", address)
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	client.Transport.(*http.Transport).TLSClientConfig.RootCAs = roots
	t.Cleanup(client.CloseIdleConnections)
	return client, "https://" + hostname + "/image"
}

func TestImageResultProxyFetchSniffsRasterAndRejectsOtherContent(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	for _, tc := range []struct{ body, mime string }{
		{"\x89PNG\r\n\x1a\ncontent", "image/png"}, {"\xff\xd8\xffcontent", "image/jpeg"},
		{"GIF89acontent", "image/gif"}, {"RIFF\x00\x00\x00\x00WEBPVP8 content", "image/webp"},
		{"<html>provider secret</html>", ""}, {"<svg xmlns='http://www.w3.org/2000/svg'/>", ""}, {"", ""},
	} {
		t.Run(tc.mime+tc.body[:min(len(tc.body), 4)], func(t *testing.T) {
			client, target := imageResultProxyTLSClient(t, func(w http.ResponseWriter, r *http.Request) {
				require.Empty(t, r.Header.Get("Authorization"))
				require.Empty(t, r.Header.Get("Cookie"))
				w.Header().Set("Content-Type", "image/png")
				_, _ = io.WriteString(w, tc.body)
			})
			body, mime, err := fetchImageResultProxy(context.Background(), target, client)
			if tc.mime == "" {
				require.Error(t, err)
				require.Nil(t, body)
				require.Empty(t, mime)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.mime, mime)
				require.Equal(t, tc.body, string(body))
			}
		})
	}
}

func TestImageResultProxyFetchLimitsAndFailures(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		client, target := imageResultProxyTLSClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, "secret upstream body")
		})
		body, mime, err := fetchImageResultProxy(context.Background(), target, client)
		require.Error(t, err)
		require.Nil(t, body)
		require.Empty(t, mime)
		require.NotContains(t, err.Error(), target)
		require.NotContains(t, err.Error(), "secret")
	}
	for _, declared := range []bool{false, true} {
		client, target := imageResultProxyTLSClient(t, func(w http.ResponseWriter, r *http.Request) {
			if declared {
				w.Header().Set("Content-Length", "31457281")
			} else {
				w.(http.Flusher).Flush()
			}
			_, _ = w.Write(append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("x"), 30*1024*1024)...))
		})
		body, _, err := fetchImageResultProxy(context.Background(), target, client)
		require.Error(t, err)
		require.Nil(t, body)
	}
	client, target := imageResultProxyTLSClient(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, _, err := fetchImageResultProxy(ctx, target, client)
	require.Error(t, err)
}

func TestImageResultProxyStrictSSRFAndTLS(t *testing.T) {
	settings := system_setting.GetFetchSetting()
	original := *settings
	originalTLS := common.TLSInsecureSkipVerify
	t.Cleanup(func() { *settings = original; common.TLSInsecureSkipVerify = originalTLS })
	settings.EnableSSRFProtection = false
	settings.AllowPrivateIp = true
	settings.ApplyIPFilterForDomain = false
	common.TLSInsecureSkipVerify = true
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	for _, ips := range [][]net.IPAddr{{{IP: net.ParseIP("127.0.0.1")}}, {{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("10.0.0.1")}}} {
		client := newImageResultProxyHTTPClient(staticSSRFResolver{"images.example": ips}, func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("must reject before dialing")
			return nil, nil
		})
		_, _, err := fetchImageResultProxy(context.Background(), "https://images.example/a", client)
		require.Error(t, err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("untrusted TLS must not reach handler") }))
	defer server.Close()
	hostname := server.Certificate().DNSNames[0]
	client := newImageResultProxyHTTPClient(staticSSRFResolver{hostname: {{IP: net.ParseIP("8.8.8.8")}}}, func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	_, _, err := fetchImageResultProxy(context.Background(), "https://"+hostname+"/a", client)
	require.Error(t, err)
	require.False(t, client.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify)
	_, _, err = FetchImageResultProxy(context.Background(), "https://127.0.0.1/private")
	require.Error(t, err)
}

func TestImageResultProxyRejectsUnsafeRedirects(t *testing.T) {
	for _, redirect := range []string{"https://127.0.0.1/private", "http://images.example/a", "https://u:p@images.example/a", "https://images.example/a#secret"} {
		calls := 0
		client, target := imageResultProxyTLSClient(t, func(w http.ResponseWriter, r *http.Request) { calls++; http.Redirect(w, r, redirect, http.StatusFound) })
		_, _, err := fetchImageResultProxy(context.Background(), target, client)
		require.Error(t, err)
		require.Equal(t, 1, calls)
		require.NotContains(t, err.Error(), redirect)
	}
	// Redirect loop is bounded even when every URL is otherwise safe.
	calls := 0
	client, target := imageResultProxyTLSClient(t, func(w http.ResponseWriter, r *http.Request) { calls++; http.Redirect(w, r, "/again", http.StatusFound) })
	_, _, err := fetchImageResultProxy(context.Background(), target, client)
	require.Error(t, err)
	require.LessOrEqual(t, calls, 10)
}
