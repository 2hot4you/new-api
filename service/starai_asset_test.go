package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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

func TestSaveStarAIAssetBindingUsesSevenDayDefaultTTL(t *testing.T) {
	useStarAIAssetRedis(t)
	constant.StarAIAssetTTLHours = 0
	startedAt := time.Now()
	binding := &StarAIAssetBinding{
		UpstreamID: "asset-seven-day-default",
		UserID:     42,
		AssetType:  "image",
		Status:     "ACTIVE",
		COSKey:     "users/42/starai-assets/image/reference.png",
	}

	require.NoError(t, SaveStarAIAssetBinding(binding))

	require.WithinDuration(t, startedAt.Add(168*time.Hour), time.Unix(binding.ExpiresAt, 0), time.Second)
	ttl, err := common.RDB.TTL(context.Background(), starAIAssetBindingKey(binding)).Result()
	require.NoError(t, err)
	require.InDelta(t, (168 * time.Hour).Seconds(), ttl.Seconds(), 1)
	cleanupAt, err := common.RDB.ZScore(context.Background(), starAICOSCleanupIndexKey, binding.COSKey).Result()
	require.NoError(t, err)
	require.Equal(t, float64(binding.ExpiresAt), cleanupAt)
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
	require.Equal(t, binding.UpstreamID, binding.ID)

	got, err := GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, "asset-upstream-secret", got.UpstreamID)
	require.Equal(t, "https://cdn.example.com/opening.mp4", got.SourceURL)
	ttlBeforeUpdate, err := common.RDB.TTL(context.Background(), starAIAssetBindingKey(binding)).Result()
	require.NoError(t, err)
	require.NoError(t, UpdateStarAIAssetSourceURL(got, " https://cdn.example.com/recovered.mp4 "))
	require.Equal(t, "https://cdn.example.com/recovered.mp4", got.SourceURL)
	ttlAfterUpdate, err := common.RDB.TTL(context.Background(), starAIAssetBindingKey(binding)).Result()
	require.NoError(t, err)
	require.Equal(t, ttlBeforeUpdate, ttlAfterUpdate)
	_, err = GetStarAIAssetBinding(binding.ID, 7)
	require.ErrorIs(t, err, ErrStarAIAssetNotFound)

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

func TestStarAIAssetBindingsAreScopedByUserForTheSameUpstreamID(t *testing.T) {
	useStarAIAssetRedis(t)
	first := &StarAIAssetBinding{
		UpstreamID: "asset-shared-upstream-id",
		UserID:     42,
		TokenID:    7,
		AssetType:  "image",
		Name:       "first user asset",
		Status:     "ACTIVE",
	}
	second := &StarAIAssetBinding{
		UpstreamID: "asset-shared-upstream-id",
		UserID:     84,
		TokenID:    9,
		AssetType:  "image",
		Name:       "second user asset",
		Status:     "ACTIVE",
	}
	require.NoError(t, SaveStarAIAssetBinding(first))
	require.NoError(t, SaveStarAIAssetBinding(second))

	firstStored, err := GetStarAIAssetBinding(first.UpstreamID, first.UserID)
	require.NoError(t, err)
	require.Equal(t, first.Name, firstStored.Name)
	require.Equal(t, first.TokenID, firstStored.TokenID)

	secondStored, err := GetStarAIAssetBinding(second.UpstreamID, second.UserID)
	require.NoError(t, err)
	require.Equal(t, second.Name, secondStored.Name)
	require.Equal(t, second.TokenID, secondStored.TokenID)

	all, err := ListAllStarAIAssetBindings()
	require.NoError(t, err)
	require.Len(t, all, 2)

	require.NoError(t, DeleteStarAIAssetBinding(first.UpstreamID, first.UserID))
	remaining, err := GetStarAIAssetBinding(second.UpstreamID, second.UserID)
	require.NoError(t, err)
	require.Equal(t, second.Name, remaining.Name)
}

func TestStarAIAssetBindingRejectsAnotherUsersUpstreamID(t *testing.T) {
	useStarAIAssetRedis(t)
	binding := &StarAIAssetBinding{
		UpstreamID: "asset-owned-by-user-84",
		UserID:     84,
		TokenID:    9,
		AssetType:  "image",
		Status:     "ACTIVE",
	}
	require.NoError(t, SaveStarAIAssetBinding(binding))

	_, err := GetStarAIAssetBinding(binding.UpstreamID, 42)
	require.ErrorIs(t, err, ErrStarAIAssetNotFound)
}

func TestStarAIAssetBindingAllowsSameUserAcrossTokens(t *testing.T) {
	useStarAIAssetRedis(t)
	binding := &StarAIAssetBinding{
		UpstreamID: "asset-shared-across-user-tokens",
		UserID:     42,
		TokenID:    7,
		AssetType:  "image",
		Status:     "ACTIVE",
	}
	require.NoError(t, SaveStarAIAssetBinding(binding))

	stored, err := GetStarAIAssetBinding(binding.UpstreamID, 42)
	require.NoError(t, err)
	require.Equal(t, 7, stored.TokenID)
}

func TestGetStarAIAssetBindingReadsLegacyMoliiID(t *testing.T) {
	useStarAIAssetRedis(t)
	binding := &StarAIAssetBinding{
		ID:         "asset-molii-legacy12345678",
		UpstreamID: "asset-upstream-legacy12345678",
		UserID:     42,
		AssetType:  "image",
		Status:     "ACTIVE",
		CreatedAt:  time.Now().Unix(),
		ExpiresAt:  time.Now().Add(time.Hour).Unix(),
	}
	body, err := common.Marshal(binding)
	require.NoError(t, err)
	require.NoError(t, common.RDB.Set(
		context.Background(),
		"starai:asset:"+binding.ID,
		body,
		time.Hour,
	).Err())

	stored, err := GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, binding.UpstreamID, stored.UpstreamID)
}

func TestResolveStarAIAssetURIRejectsUnboundUpstreamID(t *testing.T) {
	useStarAIAssetRedis(t)
	_, err := ResolveStarAIAssetURI(context.Background(), "asset://asset-upstream-secret", 42, StarAIAssetVerificationConfig{})
	require.True(t, errors.Is(err, ErrStarAIAssetNotFound))
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

func TestResolveStarAIAssetURIReturnsAndPersistsUpstreamFailureReason(t *testing.T) {
	useStarAIAssetRedis(t)
	binding := &StarAIAssetBinding{
		UpstreamID: "asset-sensitive",
		UserID:     42,
		AssetType:  "image",
		Status:     "PROCESSING",
	}
	require.NoError(t, SaveStarAIAssetBinding(binding))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"asset_type":"image",
			"status":"FAILED",
			"error":{
				"code":"InputImageSensitiveContentDetected",
				"message":"The request failed because the input image may contain sensitive information."
			}
		}`))
	}))
	t.Cleanup(server.Close)

	_, err := ResolveStarAIAssetURI(context.Background(), "asset://"+binding.ID, 42, StarAIAssetVerificationConfig{
		BaseURL: server.URL,
		APIKey:  "asset-test-key",
	})
	require.ErrorIs(t, err, ErrStarAIAssetVerify)
	require.Contains(t, err.Error(), "sensitive information")

	stored, err := GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, "FAILED", stored.Status)
	require.Equal(t, "InputImageSensitiveContentDetected", stored.ErrorCode)
	require.Contains(t, stored.ErrorMessage, "sensitive information")
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
