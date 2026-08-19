package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useGrokImagePreviewRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	common.RDB = client
	t.Cleanup(func() {
		_ = client.Close()
		common.RedisEnabled = previousEnabled
		common.RDB = previousRDB
	})
	return server
}

type previewRedisDeadlineHook struct {
	deadlines []time.Time
}

type stalledTLSRedisServer struct {
	listener    net.Listener
	done        chan struct{}
	acceptDone  chan struct{}
	connections sync.WaitGroup
	active      atomic.Int32
}

func startStalledTLSRedisServer(t *testing.T) *stalledTLSRedisServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &stalledTLSRedisServer{
		listener:   listener,
		done:       make(chan struct{}),
		acceptDone: make(chan struct{}),
	}
	go func() {
		defer close(server.acceptDone)
		for {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			server.connections.Add(1)
			server.active.Add(1)
			go func() {
				defer server.connections.Done()
				defer server.active.Add(-1)
				defer connection.Close()
				buffer := make([]byte, 1024)
				for {
					_, readErr := connection.Read(buffer)
					if readErr != nil {
						return
					}
					select {
					case <-server.done:
						return
					default:
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		close(server.done)
		_ = listener.Close()
		<-server.acceptDone
		server.connections.Wait()
	})
	return server
}

func (h *previewRedisDeadlineHook) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	if deadline, ok := ctx.Deadline(); ok {
		h.deadlines = append(h.deadlines, deadline)
	}
	return ctx, nil
}

func (h *previewRedisDeadlineHook) AfterProcess(context.Context, redis.Cmder) error { return nil }

func (h *previewRedisDeadlineHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *previewRedisDeadlineHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func TestGrokImagePreviewStoresTrustedURLsForTwentyFourHoursWithOpaqueKey(t *testing.T) {
	server := useGrokImagePreviewRedis(t)
	urls := []string{
		"https://imgen.x.ai/a.webp?token=one",
		"https://files-cdn.x.ai/b.webp?token=two",
	}

	require.NoError(t, RegisterGrokImagePreview(42, "req_preview_1", urls))
	actual, err := GetGrokImagePreview(42, "req_preview_1")
	require.NoError(t, err)
	assert.Equal(t, urls, actual)

	key := grokImagePreviewKey(42, "req_preview_1")
	assert.Equal(t, "grok:image:preview:"+common.GenerateHMACWithKey([]byte(common.SessionSecret), "grok-image-preview:v1:42:req_preview_1"), key)
	assert.NotContains(t, key, "req_preview_1")
	ttl := server.TTL(key)
	assert.Equal(t, grokImagePreviewTTL, ttl)

	fourURLs := []string{
		"https://imgen.x.ai/1", "https://imgen.x.ai/2",
		"https://imgen.x.ai/3", "https://imgen.x.ai/4",
	}
	require.NoError(t, RegisterGrokImagePreview(42, "req_preview_four", fourURLs))
	actual, err = GetGrokImagePreview(42, "req_preview_four")
	require.NoError(t, err)
	assert.Equal(t, fourURLs, actual)
}

func TestGrokImagePreviewRedisCommandsReceiveBoundedDeadlines(t *testing.T) {
	useGrokImagePreviewRedis(t)
	hook := &previewRedisDeadlineHook{}
	client := newGrokImagePreviewRedisClient(common.RDB)
	require.NotNil(t, client)
	client.AddHook(hook)
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), grokImagePreviewRedisTimeout)
	defer cancel()
	require.NoError(t, client.Set(ctx, grokImagePreviewKey(42, "req_preview_deadline"), `["https://imgen.x.ai/a"]`, grokImagePreviewTTL).Err())
	_, err := client.Get(ctx, grokImagePreviewKey(42, "req_preview_deadline")).Result()
	require.NoError(t, err)
	require.Len(t, hook.deadlines, 2)
	for _, deadline := range hook.deadlines {
		remaining := time.Until(deadline)
		assert.Positive(t, remaining)
		assert.LessOrEqual(t, remaining, grokImagePreviewRedisTimeout)
	}
}

func TestGrokImagePreviewUsesDedicatedClientWithCappedOperationTimeouts(t *testing.T) {
	useGrokImagePreviewRedis(t)
	previousRDB := common.RDB
	limitedSource := redis.NewClient(&redis.Options{
		Addr:         common.RDB.Options().Addr,
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		PoolTimeout:  time.Second,
	})
	common.RDB = limitedSource
	t.Cleanup(func() {
		_ = limitedSource.Close()
		common.RDB = previousRDB
	})

	client := newGrokImagePreviewRedisClient(common.RDB)
	t.Cleanup(func() { _ = client.Close() })
	options := client.Options()
	assert.LessOrEqual(t, options.DialTimeout, grokImagePreviewRedisTimeout)
	assert.LessOrEqual(t, options.ReadTimeout, grokImagePreviewRedisTimeout)
	assert.LessOrEqual(t, options.WriteTimeout, grokImagePreviewRedisTimeout)
	assert.LessOrEqual(t, options.PoolTimeout, grokImagePreviewRedisTimeout)
	assert.Zero(t, options.MaxRetries, "go-redis normalizes -1 to effective zero retries")
}

func TestGrokImagePreviewRegistrationMakesOneAttemptWhenRedisFailsImmediately(t *testing.T) {
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var attempts atomic.Int32
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			attempts.Add(1)
			_ = connection.Close()
		}
	}()
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: listener.Addr().String()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled, common.RDB = previousEnabled, previousRDB
		_ = listener.Close()
		<-acceptDone
	})

	err = RegisterGrokImagePreview(42, "req_single_attempt", []string{"https://imgen.x.ai/a"})
	require.ErrorIs(t, err, ErrGrokImagePreviewUnavailable)
	require.Eventually(t, func() bool { return attempts.Load() >= 1 }, time.Second, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	assert.EqualValues(t, 1, attempts.Load())
}

func TestGrokImagePreviewTLSStallKeepsSetAndGetBoundedWithoutLeakingConnections(t *testing.T) {
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	server := startStalledTLSRedisServer(t)
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{
		Addr:         server.listener.Addr().String(),
		TLSConfig:    &tls.Config{InsecureSkipVerify: true}, // test listener intentionally never presents a certificate
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		PoolTimeout:  time.Second,
	})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled, common.RDB = previousEnabled, previousRDB
	})

	for index := 0; index < 3; index++ {
		startedAt := time.Now()
		err := RegisterGrokImagePreview(42, fmt.Sprintf("req_tls_stall_set_%d", index), []string{"https://imgen.x.ai/a"})
		require.ErrorIs(t, err, ErrGrokImagePreviewUnavailable)
		assert.Less(t, time.Since(startedAt), 500*time.Millisecond)

		startedAt = time.Now()
		_, err = GetGrokImagePreview(42, fmt.Sprintf("req_tls_stall_get_%d", index))
		require.ErrorIs(t, err, ErrGrokImagePreviewNotFound)
		assert.Less(t, time.Since(startedAt), 500*time.Millisecond)
	}
	require.Eventually(t, func() bool { return server.active.Load() == 0 }, time.Second, 10*time.Millisecond)
}

func TestGrokImagePreviewRejectsUntrustedAndOversizedResults(t *testing.T) {
	useGrokImagePreviewRedis(t)
	for _, urls := range [][]string{
		{"https://images.example/not-xai.png"},
		{
			"https://imgen.x.ai/1", "https://imgen.x.ai/2", "https://imgen.x.ai/3",
			"https://imgen.x.ai/4", "https://imgen.x.ai/5",
		},
	} {
		err := RegisterGrokImagePreview(42, "req_preview_rejected", urls)
		require.ErrorIs(t, err, ErrGrokImagePreviewUnavailable)
	}
}

func TestGrokImagePreviewReturnsNotFoundWhenRedisIsUnavailableOrEntryExpires(t *testing.T) {
	server := useGrokImagePreviewRedis(t)
	previousEnabled := common.RedisEnabled
	common.RedisEnabled = false
	require.ErrorIs(t, RegisterGrokImagePreview(42, "req_no_redis", []string{"https://imgen.x.ai/a"}), ErrGrokImagePreviewUnavailable)
	_, err := GetGrokImagePreview(42, "req_no_redis")
	require.ErrorIs(t, err, ErrGrokImagePreviewNotFound)
	common.RedisEnabled = previousEnabled

	require.NoError(t, RegisterGrokImagePreview(42, "req_expired", []string{"https://imgen.x.ai/a"}))
	server.FastForward(grokImagePreviewTTL + time.Second)
	_, err = GetGrokImagePreview(42, "req_expired")
	require.ErrorIs(t, err, ErrGrokImagePreviewNotFound)
}

func TestGrokImagePreviewDoesNotExposeRedisFailures(t *testing.T) {
	useGrokImagePreviewRedis(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = common.RDB.Close() })

	err := RegisterGrokImagePreview(42, "req_redis_failure", []string{"https://imgen.x.ai/a"})
	require.ErrorIs(t, err, ErrGrokImagePreviewUnavailable)
	assert.NotContains(t, err.Error(), "127.0.0.1")
	assert.NotContains(t, err.Error(), "redis://")

	_, err = GetGrokImagePreview(42, "req_redis_failure")
	require.True(t, errors.Is(err, ErrGrokImagePreviewNotFound))
}

func TestGrokImagePreviewRevalidatesStoredURLs(t *testing.T) {
	useGrokImagePreviewRedis(t)
	for requestID, value := range map[string]string{
		"req_untrusted": `[` + `"https://evil.example/image.png"` + `]`,
		"req_malformed": `{not-json`,
		"req_empty":     `[]`,
		"req_oversized": `["https://imgen.x.ai/1","https://imgen.x.ai/2","https://imgen.x.ai/3","https://imgen.x.ai/4","https://imgen.x.ai/5"]`,
	} {
		require.NoError(t, common.RDB.Set(context.Background(), grokImagePreviewKey(42, requestID), value, grokImagePreviewTTL).Err())
		_, err := GetGrokImagePreview(42, requestID)
		require.ErrorIs(t, err, ErrGrokImagePreviewNotFound)
	}
}
