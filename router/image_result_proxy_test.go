package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageResultProxyRejectsInvalidLinkWithoutAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })
	t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "true")
	engine := gin.New()
	SetRelayRouter(engine)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/images/proxy/invalid", nil))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.JSONEq(t, `{"error":{"code":"image_link_invalid","message":"Image link is invalid or unavailable"}}`, recorder.Body.String())
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	assert.Empty(t, recorder.Header().Get("Location"))
}
