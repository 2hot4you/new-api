package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func useStarAIAssetRedis(t *testing.T) {
	t.Helper()
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	previousTTL := constant.StarAIAssetTTLHours
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	common.RDB = client
	constant.StarAIAssetTTLHours = 24
	t.Cleanup(func() {
		_ = client.Close()
		common.RedisEnabled = previousEnabled
		common.RDB = previousRDB
		constant.StarAIAssetTTLHours = previousTTL
	})
}

func TestStarAIAssetBindingOwnershipLifecycle(t *testing.T) {
	useStarAIAssetRedis(t)
	var upstreamStatus atomic.Value
	var requestValid atomic.Bool
	upstreamStatus.Store("PROCESSING")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestValid.Store(r.URL.Path == "/v1/assets/asset-upstream-secret" && r.Header.Get("Authorization") == "Bearer asset-test-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"` + upstreamStatus.Load().(string) + `"}`))
	}))
	t.Cleanup(server.Close)
	verification := StarAIAssetVerificationConfig{BaseURL: server.URL, APIKey: "asset-test-key"}
	binding := &StarAIAssetBinding{
		UpstreamID: "asset-upstream-secret",
		UserID:     42,
		TokenID:    9,
		AssetType:  "video",
		Name:       "opening shot",
		SourceURL:  "https://cdn.example.com/opening.mp4",
		Status:     "PROCESSING",
	}
	require.NoError(t, SaveStarAIAssetBinding(binding))
	require.NotEmpty(t, binding.ID)
	require.NotEqual(t, binding.ID, binding.UpstreamID)

	got, err := GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, "asset-upstream-secret", got.UpstreamID)
	require.Equal(t, "https://cdn.example.com/opening.mp4", got.SourceURL)
	ttlBeforeUpdate, err := common.RDB.TTL(context.Background(), starAIAssetKey(binding.ID)).Result()
	require.NoError(t, err)
	require.NoError(t, UpdateStarAIAssetSourceURL(got, " https://cdn.example.com/recovered.mp4 "))
	require.Equal(t, "https://cdn.example.com/recovered.mp4", got.SourceURL)
	ttlAfterUpdate, err := common.RDB.TTL(context.Background(), starAIAssetKey(binding.ID)).Result()
	require.NoError(t, err)
	require.Equal(t, ttlBeforeUpdate, ttlAfterUpdate)
	_, err = GetStarAIAssetBinding(binding.ID, 7)
	require.ErrorIs(t, err, ErrStarAIAssetForbidden)

	_, err = ResolveStarAIAssetURI(context.Background(), "asset://"+binding.ID, 42, verification)
	require.ErrorIs(t, err, ErrStarAIAssetNotReady)
	upstreamStatus.Store("ACTIVE")
	resolved, err := ResolveStarAIAssetURI(context.Background(), "asset://"+binding.ID, 42, verification)
	require.NoError(t, err)
	require.Equal(t, "asset://asset-upstream-secret", resolved)
	require.True(t, requestValid.Load())
	verified, err := GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, "ACTIVE", verified.Status)
	require.Positive(t, verified.VerifiedAt)

	items, err := ListStarAIAssetBindings(42)
	require.NoError(t, err)
	require.Len(t, items, 1)
	allItems, err := ListAllStarAIAssetBindings()
	require.NoError(t, err)
	require.Len(t, allItems, 1)
	require.Equal(t, 42, allItems[0].UserID)
	require.Equal(t, "https://cdn.example.com/recovered.mp4", allItems[0].SourceURL)
	stats, err := GetStarAIAssetStats()
	require.NoError(t, err)
	require.Equal(t, 1, stats.Total)
	require.Equal(t, 1, stats.Success)
	require.Equal(t, 1, stats.Users)
	require.Equal(t, 1, stats.ByType["video"])
	require.NoError(t, DeleteStarAIAssetBinding(binding.ID, 42))
	_, err = GetStarAIAssetBinding(binding.ID, 42)
	require.ErrorIs(t, err, ErrStarAIAssetNotFound)
}

func TestResolveStarAIAssetURIRejectsRawUpstreamID(t *testing.T) {
	useStarAIAssetRedis(t)
	_, err := ResolveStarAIAssetURI(context.Background(), "asset://asset-upstream-secret", 42, StarAIAssetVerificationConfig{})
	require.True(t, errors.Is(err, ErrStarAIAssetForbidden))
	require.Equal(t, "https://example.com/a.png", mustResolveUnchanged(t, "https://example.com/a.png"))
}

func TestResolveStarAIAssetURIMarksUpstreamNotFoundExpired(t *testing.T) {
	useStarAIAssetRedis(t)
	binding := &StarAIAssetBinding{UpstreamID: "asset-gone", UserID: 42, AssetType: "image", Status: "ACTIVE"}
	require.NoError(t, SaveStarAIAssetBinding(binding))
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)

	_, err := ResolveStarAIAssetURI(context.Background(), "asset://"+binding.ID, 42, StarAIAssetVerificationConfig{BaseURL: server.URL, APIKey: "asset-test-key"})
	require.ErrorIs(t, err, ErrStarAIAssetExpired)
	stored, err := GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, "EXPIRED", stored.Status)
	require.Positive(t, stored.VerifiedAt)
}

func TestResolveStarAIAssetURIUsesSourceURLAcrossDifferentChannelKeys(t *testing.T) {
	useStarAIAssetRedis(t)
	var upstreamRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamRequests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	binding := &StarAIAssetBinding{
		UpstreamID: "asset-created-by-another-key",
		ChannelID:  11,
		UserID:     42,
		AssetType:  "image",
		SourceURL:  "https://cdn.example.com/reference.png",
		SourceKind: "url",
		Status:     "ACTIVE",
	}
	require.NoError(t, SaveStarAIAssetBinding(binding))

	resolved, err := ResolveStarAIAssetURI(context.Background(), "asset://"+binding.ID, 42, StarAIAssetVerificationConfig{
		ChannelID: 22,
		BaseURL:   server.URL,
		APIKey:    "different-key",
	})

	require.NoError(t, err)
	require.Equal(t, "https://cdn.example.com/reference.png", resolved)
	require.Zero(t, upstreamRequests.Load(), "an upstream asset ID must not be queried with a different channel key")
}

func TestResolveStarAIAssetURIUsesSourceURLAfterChannelKeyRotation(t *testing.T) {
	useStarAIAssetRedis(t)
	var upstreamRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamRequests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	binding := &StarAIAssetBinding{
		UpstreamID:            "asset-created-by-old-key",
		ChannelID:             11,
		ChannelKeyFingerprint: StarAIChannelKeyFingerprint("old-key"),
		UserID:                42,
		AssetType:             "image",
		SourceURL:             "https://cdn.example.com/reference.png",
		SourceKind:            "url",
		Status:                "ACTIVE",
	}
	require.NoError(t, SaveStarAIAssetBinding(binding))

	resolved, err := ResolveStarAIAssetURI(context.Background(), "asset://"+binding.ID, 42, StarAIAssetVerificationConfig{
		ChannelID: 11,
		BaseURL:   server.URL,
		APIKey:    "new-key",
	})

	require.NoError(t, err)
	require.Equal(t, "https://cdn.example.com/reference.png", resolved)
	require.Zero(t, upstreamRequests.Load(), "an upstream asset ID must not be queried after its channel key changes")
}

func mustResolveUnchanged(t *testing.T, value string) string {
	t.Helper()
	got, err := ResolveStarAIAssetURI(context.Background(), value, 42, StarAIAssetVerificationConfig{})
	require.NoError(t, err)
	return got
}
