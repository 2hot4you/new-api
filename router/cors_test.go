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

func TestSetRouterInstallsDashboardCORSBeforeAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalOrigins := common.DashboardCORSAllowedOrigins
	common.DashboardCORSAllowedOrigins = []string{"https://dashboard.example.com"}
	t.Cleanup(func() {
		common.DashboardCORSAllowedOrigins = originalOrigins
	})

	engine := gin.New()
	SetRouter(engine, WebAssets{})

	allowedRequest := httptest.NewRequest(http.MethodOptions, "/api/status", nil)
	allowedRequest.Header.Set("Origin", "https://dashboard.example.com")
	allowedRequest.Header.Set("Access-Control-Request-Method", http.MethodGet)
	allowedRequest.Header.Set(
		"Access-Control-Request-Headers",
		"Cache-Control, Authorization, Content-Type, X-Auth-Session",
	)
	allowedResponse := httptest.NewRecorder()
	engine.ServeHTTP(allowedResponse, allowedRequest)

	require.Equal(t, http.StatusNoContent, allowedResponse.Code)
	assert.Equal(
		t,
		"https://dashboard.example.com",
		allowedResponse.Header().Get("Access-Control-Allow-Origin"),
	)
	assert.Contains(t, allowedResponse.Header().Get("Access-Control-Allow-Headers"), "Cache-Control")

	disallowedRequest := httptest.NewRequest(http.MethodOptions, "/api/status", nil)
	disallowedRequest.Header.Set("Origin", "https://evil.example.com")
	disallowedRequest.Header.Set("Access-Control-Request-Method", http.MethodGet)
	disallowedResponse := httptest.NewRecorder()
	engine.ServeHTTP(disallowedResponse, disallowedRequest)

	assert.Equal(t, http.StatusForbidden, disallowedResponse.Code)
	assert.Empty(t, disallowedResponse.Header().Get("Access-Control-Allow-Origin"))
}
