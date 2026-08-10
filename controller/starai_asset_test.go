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
	"github.com/gin-gonic/gin"
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

func TestCreateStarAIAssetRequiresExactlyOneEnabledChannelBeforeUpstream(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	var upstreamCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		w.WriteHeader(http.StatusOK)
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

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/assets", nil)
	_, ok := createStarAIAssetUpstream(ctx, createStarAIAssetRequest{
		URL:       "https://cdn.example.com/input.png",
		AssetType: "image",
	}, &service.StarAIAssetBinding{})

	require.False(t, ok)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Zero(t, upstreamCalls.Load())
	require.NotContains(t, recorder.Body.String(), "StarAI")
	require.NotContains(t, recorder.Body.String(), "Molii")
}
