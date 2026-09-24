package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func useControllerStarAIAssetRedis(t *testing.T) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, client
	t.Cleanup(func() {
		_ = client.Close()
		common.RedisEnabled, common.RDB = previousEnabled, previousRDB
	})
}

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
	require.Equal(t, "asset-upstream", binding.ID)
	require.Equal(t, "asset-upstream", safeStarAIAsset(binding).ID)
	require.Equal(t, channel.Id, binding.ChannelID)
	require.Equal(t, service.StarAIChannelKeyFingerprint(channel.Key), binding.ChannelKeyFingerprint)
	stored, err := service.GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, binding.ChannelID, stored.ChannelID)
	require.Equal(t, binding.ChannelKeyFingerprint, stored.ChannelKeyFingerprint)
}

func TestTemporaryAssetResellerCreateRefreshesUpstreamExpiry(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	useControllerStarAIAssetRedis(t)
	var calls atomic.Int32
	expiresAt := time.Now().Add(2 * time.Hour).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "Bearer instance-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			require.Equal(t, "/v1/assets", r.URL.Path)
			_, _ = w.Write([]byte(`{"id":"asset-reseller","status":"PROCESSING"}`))
		case http.MethodGet:
			require.Equal(t, "/v1/assets/asset-reseller", r.URL.Path)
			_, _ = fmt.Fprintf(w, `{"status":"ACTIVE","expires_at":%d}`, expiresAt)
		default:
			t.Errorf("unexpected upstream method %s", r.Method)
		}
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	channel := &model.Channel{Type: constant.ChannelTypeByteDanceSeedance, Status: common.ChannelStatusEnabled, Name: "reseller", Key: "instance-key", BaseURL: &baseURL}
	require.NoError(t, db.Create(channel).Error)
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/assets", nil)
	ctx.Set("id", 42)
	binding, ok := createStarAIAssetUpstream(ctx, createStarAIAssetRequest{URL: "https://cdn.example.com/reference.png", AssetType: "image", Name: "reference"}, &service.StarAIAssetBinding{})
	require.True(t, ok)
	require.Equal(t, "asset-reseller", binding.ID)
	require.Equal(t, constant.ChannelTypeByteDanceSeedance, binding.ChannelType)
	require.Equal(t, "ACTIVE", binding.Status)
	require.Equal(t, expiresAt, binding.ExpiresAt)
	require.Equal(t, int32(2), calls.Load())
	stored, err := service.GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, expiresAt, stored.ExpiresAt)
}

func TestRefreshLegacyStarAIAssetUsesAnEnabledChannel(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	useControllerStarAIAssetRedis(t)
	var upstreamRequests atomic.Int32
	var authorization atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests.Add(1)
		authorization.Store(r.Header.Get("Authorization"))
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

	binding := &service.StarAIAssetBinding{UpstreamID: "legacy-asset", UserID: 42, Status: "PROCESSING"}
	require.NoError(t, service.SaveStarAIAssetBinding(binding))
	refreshed, err := refreshStarAIAsset(nil, binding)

	require.NoError(t, err)
	require.Same(t, binding, refreshed)
	require.Equal(t, int32(1), upstreamRequests.Load())
	require.Equal(t, "Bearer first-key", authorization.Load())
}

func TestRefreshStarAIAssetFallsBackWhenOriginalChannelIsDisabled(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	useControllerStarAIAssetRedis(t)
	var authorization atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ACTIVE"}`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	disabled := &model.Channel{
		Type:    constant.ChannelTypeStarAI,
		Status:  common.ChannelStatusManuallyDisabled,
		Name:    "disabled original",
		Key:     "disabled-key",
		BaseURL: &baseURL,
	}
	require.NoError(t, db.Create(disabled).Error)
	require.NoError(t, db.Create(&model.Channel{
		Type:    constant.ChannelTypeStarAI,
		Status:  common.ChannelStatusEnabled,
		Name:    "enabled fallback",
		Key:     "fallback-key",
		BaseURL: &baseURL,
	}).Error)

	binding := &service.StarAIAssetBinding{
		UpstreamID: "asset-created-on-disabled-channel",
		ChannelID:  disabled.Id,
		UserID:     42,
		Status:     "PROCESSING",
	}
	require.NoError(t, service.SaveStarAIAssetBinding(binding))
	refreshed, err := refreshStarAIAsset(nil, binding)

	require.NoError(t, err)
	require.Equal(t, "ACTIVE", refreshed.Status)
	require.Equal(t, "Bearer fallback-key", authorization.Load())
}

func TestRefreshStarAIAssetPersistsAndReturnsBusinessFailure(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	previousRedisEnabled, previousRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, redisClient
	t.Cleanup(func() {
		_ = redisClient.Close()
		common.RedisEnabled, common.RDB = previousRedisEnabled, previousRDB
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"asset_type":"image",
			"status":"FAILED",
			"error":{
				"code":"InputImageSensitiveContentDetected",
				"message":"The request failed because https://cdn.example.com/input.png may contain sensitive information. Request ID: request-1234567890"
			}
		}`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	channel := &model.Channel{
		Type:    constant.ChannelTypeStarAI,
		Status:  common.ChannelStatusEnabled,
		Name:    "asset failure",
		Key:     "asset-failure-key",
		BaseURL: &baseURL,
	}
	require.NoError(t, db.Create(channel).Error)
	binding := &service.StarAIAssetBinding{
		UpstreamID: "asset-upstream-failed",
		ChannelID:  channel.Id,
		UserID:     42,
		AssetType:  "image",
		Status:     "PROCESSING",
	}
	require.NoError(t, service.SaveStarAIAssetBinding(binding))

	refreshed, err := refreshStarAIAsset(nil, binding)
	require.NoError(t, err)
	require.Equal(t, "FAILED", refreshed.Status)
	require.Equal(t, "InputImageSensitiveContentDetected", refreshed.ErrorCode)
	require.NotContains(t, refreshed.ErrorMessage, "cdn.example.com")
	require.Contains(t, refreshed.ErrorMessage, "sensitive information")

	stored, err := service.GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, refreshed.ErrorCode, stored.ErrorCode)
	require.Equal(t, refreshed.ErrorMessage, stored.ErrorMessage)

	response := safeStarAIAsset(refreshed)
	require.NotNil(t, response.Error)
	require.Equal(t, "InputImageSensitiveContentDetected", response.Error.Code)
	require.Equal(t, refreshed.ErrorMessage, response.Error.Message)
}
