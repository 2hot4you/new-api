package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestValidatePublicAssetURL(t *testing.T) {
	for _, raw := range []string{
		"http://localhost/a.png",
		"http://127.0.0.1/a.png",
		"http://10.0.0.2/a.png",
		"file:///tmp/a.png",
		"https://user:pass@example.com/a.png",
	} {
		require.Error(t, validatePublicAssetURL(raw), raw)
	}
	require.NoError(t, validatePublicAssetURL("https://cdn.example.com/a.png"))
}

func TestNormalizeStarAIAssetStatus(t *testing.T) {
	require.Equal(t, "SUCCESS", normalizeStarAIAssetStatus("ready"))
	require.Equal(t, "SUCCESS", normalizeStarAIAssetStatus("completed"))
	require.Equal(t, "FAILED", normalizeStarAIAssetStatus("error"))
	require.Equal(t, "PROCESSING", normalizeStarAIAssetStatus("processing"))
	require.Equal(t, "ACTIVE", normalizeStarAIAssetStatus("active"))
	require.Equal(t, "EXPIRED", normalizeStarAIAssetStatus("deleted"))
}

func TestParseStarAIAssetUpstreamFailure(t *testing.T) {
	failure := parseStarAIAssetUpstreamFailure("create", http.StatusBadRequest, []byte(`{
		"error": {
			"code": "invalid_image_size",
			"message": "StarAI image https://cdn.example.com/private.webp must be at least 300px; token=secret-value asset-20260616115348-9df57"
		}
	}`), nil)
	require.Equal(t, "invalid_image_size", failure.Code)
	require.NotContains(t, failure.Reason, "cdn.example.com")
	require.NotContains(t, failure.Reason, "secret-value")
	require.NotContains(t, failure.Reason, "asset-20260616115348-9df57")
	require.NotContains(t, failure.Reason, "StarAI")
	require.Contains(t, failure.Reason, "Molii Volcengine Imagine API")
	require.Contains(t, failure.Reason, "300px")
	require.Equal(t, http.StatusBadRequest, starAIAssetClientStatus(failure.Status))
}

func TestParseStarAIAssetUpstreamFailureFallbacks(t *testing.T) {
	failure := parseStarAIAssetUpstreamFailure("create", http.StatusUnsupportedMediaType, []byte(`not-json`), nil)
	require.Equal(t, "Molii Volcengine Imagine API 不支持该素材格式", failure.Reason)

	failure = parseStarAIAssetUpstreamFailure("create", 0, nil, errors.New("dial failed"))
	require.Equal(t, "无法连接 Molii Volcengine Imagine API 服务", failure.Reason)
	require.Equal(t, http.StatusBadGateway, starAIAssetClientStatus(failure.Status))
}

func TestGetStarAIAssetChannelUsesFirstEnabledChannelWhenSeveralExist(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)

	channels := make([]*model.Channel, 0, 2)
	for _, name := range []string{"first", "second"} {
		channel := &model.Channel{
			Type:   constant.ChannelTypeStarAI,
			Status: common.ChannelStatusEnabled,
			Name:   name,
			Key:    name + "-key",
		}
		require.NoError(t, db.Create(channel).Error)
		channels = append(channels, channel)
	}

	selected, err := getStarAIAssetChannel(nil)
	require.NoError(t, err)
	require.Equal(t, channels[0].Id, selected.Id)
	require.Equal(t, "first-key", selected.Key)

	selected, err = getStarAIAssetChannel(&service.StarAIAssetBinding{ChannelID: channels[1].Id})
	require.NoError(t, err)
	require.Equal(t, channels[1].Id, selected.Id)
	require.Equal(t, "second-key", selected.Key)
}

func TestCreateStarAIAssetRecordsItsChannelAndKeyOwnership(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	previousRedisEnabled, previousRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, redisClient
	t.Cleanup(func() {
		_ = redisClient.Close()
		common.RedisEnabled, common.RDB = previousRedisEnabled, previousRDB
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer model-specific-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"asset-upstream","status":"ACTIVE"}`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	channel := &model.Channel{Type: constant.ChannelTypeStarAI, Status: common.ChannelStatusEnabled, Name: "mini", Key: "model-specific-key", BaseURL: &baseURL}
	require.NoError(t, db.Create(channel).Error)

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/assets", nil)
	ctx.Set("id", 42)
	ctx.Set("token_id", 9)
	binding, ok := createStarAIAssetUpstream(ctx, createStarAIAssetRequest{
		URL:       "https://cdn.example.com/reference.png",
		AssetType: "image",
		Name:      "reference",
	}, &service.StarAIAssetBinding{SourceURL: "https://cdn.example.com/reference.png", SourceKind: "url"})

	require.True(t, ok)
	require.Equal(t, channel.Id, binding.ChannelID)
	require.Equal(t, service.StarAIChannelKeyFingerprint(channel.Key), binding.ChannelKeyFingerprint)
	stored, err := service.GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, binding.ChannelID, stored.ChannelID)
	require.Equal(t, binding.ChannelKeyFingerprint, stored.ChannelKeyFingerprint)
}

func TestRefreshLegacyStarAIAssetDoesNotQueryAnArbitraryChannelKey(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	var upstreamRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ACTIVE"}`))
	}))
	t.Cleanup(server.Close)
	for _, name := range []string{"first", "second"} {
		baseURL := server.URL
		require.NoError(t, db.Create(&model.Channel{
			Type:    constant.ChannelTypeStarAI,
			Status:  common.ChannelStatusEnabled,
			Name:    name,
			Key:     name + "-key",
			BaseURL: &baseURL,
		}).Error)
	}

	binding := &service.StarAIAssetBinding{UpstreamID: "legacy-asset", Status: "ACTIVE"}
	refreshed, err := refreshStarAIAsset(nil, binding)

	require.NoError(t, err)
	require.Same(t, binding, refreshed)
	require.Zero(t, upstreamRequests.Load())
}
