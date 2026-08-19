package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const (
	grokImagePreviewTTL          = 24 * time.Hour
	grokImagePreviewRedisTimeout = 200 * time.Millisecond
)

var (
	ErrGrokImagePreviewUnavailable = errors.New("Grok image preview is unavailable")
	ErrGrokImagePreviewNotFound    = errors.New("Grok image preview was not found")
)

// grokImagePreviewKey intentionally does not expose user or request identifiers
// in Redis keyspace. SessionSecret is an installation-specific HMAC key.
func grokImagePreviewKey(userID int, requestID string) string {
	payload := fmt.Sprintf("grok-image-preview:v1:%d:%s", userID, strings.TrimSpace(requestID))
	return "grok:image:preview:" + common.GenerateHMACWithKey([]byte(common.SessionSecret), payload)
}

// newGrokImagePreviewRedisClient derives an operation-local client from the
// current global configuration without changing global Redis behavior. Its
// transport limits cover dial, TLS handshake, command I/O, and pool waiting.
// Closing it after one operation prevents retry/dial state from leaking across
// paid image responses and lets tests replace common.RDB safely.
func newGrokImagePreviewRedisClient(source *redis.Client) *redis.Client {
	if source == nil {
		return nil
	}
	options := *source.Options()
	if options.TLSConfig != nil {
		options.TLSConfig = options.TLSConfig.Clone()
	}
	options.DialTimeout = grokImagePreviewRedisTimeout
	options.ReadTimeout = grokImagePreviewRedisTimeout
	options.WriteTimeout = grokImagePreviewRedisTimeout
	options.PoolTimeout = grokImagePreviewRedisTimeout
	// go-redis v8 treats zero as its default three retries; -1 is normalized
	// to an effective zero-retry client during initialization.
	options.MaxRetries = -1
	options.PoolSize = 2
	options.MinIdleConns = 0
	options.Dialer = func(ctx context.Context, network, address string) (net.Conn, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		dialContext, cancel := context.WithTimeout(ctx, grokImagePreviewRedisTimeout)
		defer cancel()
		connection, err := (&net.Dialer{}).DialContext(dialContext, network, address)
		if err != nil || options.TLSConfig == nil {
			return connection, err
		}
		if deadline, ok := dialContext.Deadline(); ok {
			_ = connection.SetDeadline(deadline)
		}
		tlsConnection := tls.Client(connection, options.TLSConfig.Clone())
		if err = tlsConnection.HandshakeContext(dialContext); err != nil {
			_ = connection.Close()
			return nil, err
		}
		_ = connection.SetDeadline(time.Time{})
		return tlsConnection, nil
	}
	return redis.NewClient(&options)
}

// RegisterGrokImagePreview records only validated upstream result URLs. Callers
// must treat errors as best-effort failures so a paid generation response is not
// affected when Redis is unavailable.
func RegisterGrokImagePreview(userID int, requestID string, urls []string) error {
	if userID <= 0 || strings.TrimSpace(requestID) == "" || len(urls) < 1 || len(urls) > 4 {
		return ErrGrokImagePreviewUnavailable
	}
	if !common.RedisEnabled || common.RDB == nil {
		return ErrGrokImagePreviewUnavailable
	}
	for _, rawURL := range urls {
		if !IsTrustedMoliiGrokImageURL(rawURL) {
			return ErrGrokImagePreviewUnavailable
		}
	}
	body, err := common.Marshal(urls)
	if err != nil {
		return ErrGrokImagePreviewUnavailable
	}
	client := newGrokImagePreviewRedisClient(common.RDB)
	if client == nil {
		return ErrGrokImagePreviewUnavailable
	}
	defer func() { _ = client.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), grokImagePreviewRedisTimeout)
	defer cancel()
	if err = client.Set(ctx, grokImagePreviewKey(userID, requestID), body, grokImagePreviewTTL).Err(); err != nil {
		return ErrGrokImagePreviewUnavailable
	}
	return nil
}

// GetGrokImagePreview returns a request owner's short-lived result URLs. Redis
// errors deliberately collapse to not-found, so no connection details reach an
// API response or ordinary server log.
func GetGrokImagePreview(userID int, requestID string) ([]string, error) {
	if userID <= 0 || strings.TrimSpace(requestID) == "" || !common.RedisEnabled || common.RDB == nil {
		return nil, ErrGrokImagePreviewNotFound
	}
	client := newGrokImagePreviewRedisClient(common.RDB)
	if client == nil {
		return nil, ErrGrokImagePreviewNotFound
	}
	defer func() { _ = client.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), grokImagePreviewRedisTimeout)
	defer cancel()
	body, err := client.Get(ctx, grokImagePreviewKey(userID, requestID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrGrokImagePreviewNotFound
		}
		return nil, ErrGrokImagePreviewNotFound
	}
	var urls []string
	if err := common.Unmarshal(body, &urls); err != nil || len(urls) < 1 || len(urls) > 4 {
		return nil, ErrGrokImagePreviewNotFound
	}
	for _, rawURL := range urls {
		if !IsTrustedMoliiGrokImageURL(rawURL) {
			return nil, ErrGrokImagePreviewNotFound
		}
	}
	return urls, nil
}
