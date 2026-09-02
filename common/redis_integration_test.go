package common

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestConfiguredRedisRoundTripPreservesTTL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_REDIS_DSN"))
	if dsn == "" {
		t.Skip("TEST_REDIS_DSN is not configured")
	}

	options, err := redis.ParseURL(dsn)
	require.NoError(t, err)
	client := redis.NewClient(options)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.Ping(ctx).Err())

	key := fmt.Sprintf("molii:ci:redis:%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })
	require.NoError(t, client.Set(ctx, key, "40", 30*time.Second).Err())

	previousClient, previousEnabled := RDB, RedisEnabled
	RDB, RedisEnabled = client, true
	t.Cleanup(func() {
		RDB, RedisEnabled = previousClient, previousEnabled
	})

	value, err := RedisGet(key)
	require.NoError(t, err)
	require.Equal(t, "40", value)
	require.NoError(t, RedisIncr(key, 2))

	value, err = client.Get(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, "42", value)
	ttl, err := client.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, 30*time.Second)

	require.NoError(t, RedisDel(key))
	exists, err := client.Exists(ctx, key).Result()
	require.NoError(t, err)
	require.Zero(t, exists)
}
