package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	imageResultProxyPurpose         = "image-result-proxy:v1"
	imageResultProxyLifetime        = time.Hour
	imageResultProxyMaxSourceLength = 8192
	imageResultProxyMaxTokenLength  = 16 * 1024
	imageResultProxyMaxBytes        = 30 * 1024 * 1024
	imageResultProxyTimeout         = 60 * time.Second
)

var (
	ErrImageResultProxyTokenInvalid = errors.New("invalid image result link")
	ErrImageResultProxyExpired      = errors.New("image result link expired")
	errImageResultProxyUnavailable  = errors.New("image result proxy unavailable")
	errImageResultProxyFetch        = errors.New("image result could not be fetched")
)

type imageResultProxyPayload struct {
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expires_at"`
}

// ImageResultProxyEnabledForModel restricts opt-in rewriting to these model families.
func ImageResultProxyEnabledForModel(model string) bool {
	if !common.GetEnvOrDefaultBool("IMAGE_RESULT_PROXY_ENABLED", false) {
		return false
	}
	for _, family := range []string{"gpt-image-2", "gpt-image-2.5"} {
		if model == family || strings.HasPrefix(model, family+"-") {
			return true
		}
	}
	return false
}

func imageResultProxyAEAD() (cipher.AEAD, error) {
	if strings.TrimSpace(common.CryptoSecret) == "" {
		return nil, errImageResultProxyUnavailable
	}
	mac := hmac.New(sha256.New, []byte(common.CryptoSecret))
	_, _ = mac.Write([]byte(imageResultProxyPurpose))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, errImageResultProxyUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errImageResultProxyUnavailable
	}
	return aead, nil
}

// BuildImageResultProxyURL creates an opaque, authenticated one-hour capability.
func BuildImageResultProxyURL(sourceURL string) (string, error) {
	if err := validateImageResultProxySource(sourceURL); err != nil {
		return "", errImageResultProxyUnavailable
	}
	base := strings.TrimSpace(os.Getenv("IMAGE_RESULT_PROXY_BASE_URL"))
	if base == "" {
		base = strings.TrimSpace(system_setting.ServerAddress)
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(base, "#") || parsed.Opaque != "" {
		return "", errImageResultProxyUnavailable
	}
	payload, err := common.Marshal(imageResultProxyPayload{URL: sourceURL, ExpiresAt: time.Now().Add(imageResultProxyLifetime).Unix()})
	if err != nil {
		return "", errImageResultProxyUnavailable
	}
	aead, err := imageResultProxyAEAD()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", errImageResultProxyUnavailable
	}
	encrypted := aead.Seal(nonce, nonce, payload, []byte(imageResultProxyPurpose))
	token := base64.RawURLEncoding.EncodeToString(encrypted)
	if len(token) > imageResultProxyMaxTokenLength {
		return "", errImageResultProxyUnavailable
	}
	return strings.TrimRight(parsed.String(), "/") + "/v1/images/proxy/" + token, nil
}

func ResolveImageResultProxyToken(token string) (string, error) {
	if len(token) == 0 || len(token) > imageResultProxyMaxTokenLength {
		return "", ErrImageResultProxyTokenInvalid
	}
	encrypted, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil {
		return "", ErrImageResultProxyTokenInvalid
	}
	aead, err := imageResultProxyAEAD()
	if err != nil || len(encrypted) < aead.NonceSize()+aead.Overhead() {
		return "", ErrImageResultProxyTokenInvalid
	}
	plaintext, err := aead.Open(nil, encrypted[:aead.NonceSize()], encrypted[aead.NonceSize():], []byte(imageResultProxyPurpose))
	if err != nil {
		return "", ErrImageResultProxyTokenInvalid
	}
	var payload imageResultProxyPayload
	if common.Unmarshal(plaintext, &payload) != nil || payload.ExpiresAt <= 0 || validateImageResultProxySource(payload.URL) != nil {
		return "", ErrImageResultProxyTokenInvalid
	}
	if time.Now().Unix() >= payload.ExpiresAt {
		return "", ErrImageResultProxyExpired
	}
	return payload.URL, nil
}

// Retain operator filters, but never permit disabling private-IP checks or DNS pinning.
func imageResultProxyProtection() (*common.SSRFProtection, bool, error) {
	protection, enabled, err := currentFetchProtection()
	if err != nil {
		return nil, true, err
	}
	if !enabled {
		protection = &common.SSRFProtection{}
	}
	strict := *protection
	strict.AllowPrivateIp = false
	strict.ApplyIPFilterForDomain = true
	return &strict, true, nil
}

func validateImageResultProxySource(source string) error {
	if len(source) == 0 || len(source) > imageResultProxyMaxSourceLength || strings.Contains(source, "#") {
		return errImageResultProxyFetch
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" || strings.Contains(parsed.Hostname(), "%") {
		return errImageResultProxyFetch
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errImageResultProxyFetch
	}
	if validateProtectedFetchURLWithoutDNS(parsed, imageResultProxyProtection) != nil {
		return errImageResultProxyFetch
	}
	return nil
}

func newImageResultProxyHTTPClient(resolver ssrfResolver, dialContext func(context.Context, string, string) (net.Conn, error)) *http.Client {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if dialContext == nil {
		dialContext = (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	}
	dialer := &protectedFetchDialer{resolver: resolver, dialContext: dialContext, getProtection: imageResultProxyProtection}
	// Use the existing pinned dialer with a dedicated transport: the shared client
	// honors an insecure TLS flag and environment proxies, unsuitable for public links.
	transport := &http.Transport{
		DialContext:            dialer.DialContext,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  imageResultProxyTimeout,
		MaxResponseHeaderBytes: 1 << 20,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           10,
		MaxIdleConnsPerHost:    2,
		IdleConnTimeout:        30 * time.Second,
	}
	return &http.Client{Transport: transport, Timeout: imageResultProxyTimeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || req == nil || req.URL == nil {
			return errImageResultProxyFetch
		}
		return validateImageResultProxySource(req.URL.String())
	}}
}

// FetchImageResultProxy buffers a bounded raster response; errors never contain provider details.
func FetchImageResultProxy(ctx context.Context, sourceURL string) ([]byte, string, error) {
	client := newImageResultProxyHTTPClient(nil, nil)
	defer client.CloseIdleConnections()
	return fetchImageResultProxy(ctx, sourceURL, client)
}

func fetchImageResultProxy(ctx context.Context, sourceURL string, client *http.Client) ([]byte, string, error) {
	if validateImageResultProxySource(sourceURL) != nil {
		return nil, "", errImageResultProxyFetch
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, imageResultProxyTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, "", errImageResultProxyFetch
	}
	req.Header.Set("Accept", "image/png, image/jpeg, image/webp, image/gif")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", errImageResultProxyFetch
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength > imageResultProxyMaxBytes {
		return nil, "", errImageResultProxyFetch
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, imageResultProxyMaxBytes+1))
	if err != nil || len(body) == 0 || len(body) > imageResultProxyMaxBytes {
		return nil, "", errImageResultProxyFetch
	}
	mime := http.DetectContentType(body)
	switch mime {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return body, mime, nil
	default:
		return nil, "", errImageResultProxyFetch
	}
}
