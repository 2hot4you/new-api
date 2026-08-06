package controller

import (
	"errors"
	"net/http"
	"testing"

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
