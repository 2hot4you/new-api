package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDashboardCORSTestEngine(t *testing.T, allowedOrigins []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	originalOrigins := common.DashboardCORSAllowedOrigins
	common.DashboardCORSAllowedOrigins = allowedOrigins
	t.Cleanup(func() {
		common.DashboardCORSAllowedOrigins = originalOrigins
	})

	engine := gin.New()
	engine.Use(DashboardCORS())
	engine.Use(CORS())
	engine.GET("/api/status", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	engine.GET("/v1/models", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return engine
}

func performDashboardCORSRequest(
	t *testing.T,
	engine http.Handler,
	method string,
	path string,
	origin string,
	requestHeaders string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	if requestHeaders != "" {
		request.Header.Set("Access-Control-Request-Method", http.MethodPost)
		request.Header.Set("Access-Control-Request-Headers", requestHeaders)
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func TestDashboardCORSAllowsConfiguredOrigin(t *testing.T) {
	engine := newDashboardCORSTestEngine(t, []string{"https://dashboard.example.com"})

	response := performDashboardCORSRequest(
		t,
		engine,
		http.MethodGet,
		"/api/status",
		"https://dashboard.example.com",
		"",
	)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "https://dashboard.example.com", response.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", response.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, response.Header().Get("Access-Control-Expose-Headers"), "Retry-After")
	assert.Contains(t, response.Header().Get("Access-Control-Expose-Headers"), "Auth-Version")
	assert.Contains(t, response.Header().Get("Access-Control-Expose-Headers"), common.RequestIdKey)
}

func TestDashboardCORSAllowsConfiguredOriginPreflight(t *testing.T) {
	engine := newDashboardCORSTestEngine(t, []string{"https://dashboard.example.com"})

	response := performDashboardCORSRequest(
		t,
		engine,
		http.MethodOptions,
		"/api/status",
		"https://dashboard.example.com",
		"Authorization, Cache-Control, Content-Type, X-Auth-Session, X-Security-Proof, Accept-Language",
	)

	require.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "https://dashboard.example.com", response.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", response.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, response.Header().Get("Access-Control-Allow-Methods"), "PATCH")
	assert.NotContains(t, response.Header().Get("Access-Control-Allow-Methods"), "*")
	assert.Contains(t, response.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Contains(t, response.Header().Get("Access-Control-Allow-Headers"), "Cache-Control")
	assert.Contains(t, response.Header().Get("Access-Control-Allow-Headers"), "X-Auth-Session")
	assert.NotContains(t, response.Header().Get("Access-Control-Allow-Headers"), "*")
}

func TestDashboardCORSRejectsUnconfiguredOrigin(t *testing.T) {
	engine := newDashboardCORSTestEngine(t, []string{"https://dashboard.example.com"})

	actualResponse := performDashboardCORSRequest(
		t,
		engine,
		http.MethodGet,
		"/api/status",
		"https://evil.example.com",
		"",
	)
	preflightResponse := performDashboardCORSRequest(
		t,
		engine,
		http.MethodOptions,
		"/api/status",
		"https://evil.example.com",
		"Authorization",
	)

	assert.Equal(t, http.StatusForbidden, actualResponse.Code)
	assert.Empty(t, actualResponse.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, http.StatusForbidden, preflightResponse.Code)
	assert.Empty(t, preflightResponse.Header().Get("Access-Control-Allow-Origin"))
}

func TestDashboardCORSBlankConfigurationAddsNoPermissions(t *testing.T) {
	engine := newDashboardCORSTestEngine(t, nil)

	actualResponse := performDashboardCORSRequest(
		t,
		engine,
		http.MethodGet,
		"/api/status",
		"https://dashboard.example.com",
		"",
	)
	preflightResponse := performDashboardCORSRequest(
		t,
		engine,
		http.MethodOptions,
		"/api/status",
		"https://dashboard.example.com",
		"Authorization",
	)

	assert.Equal(t, http.StatusOK, actualResponse.Code)
	assert.Empty(t, actualResponse.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, http.StatusNotFound, preflightResponse.Code)
	assert.Empty(t, preflightResponse.Header().Get("Access-Control-Allow-Origin"))
}

func TestDashboardCORSDoesNotApplyOutsideAPI(t *testing.T) {
	engine := newDashboardCORSTestEngine(t, []string{"https://dashboard.example.com"})

	response := performDashboardCORSRequest(
		t,
		engine,
		http.MethodGet,
		"/v1/models",
		"https://dashboard.example.com",
		"",
	)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "*", response.Header().Get("Access-Control-Allow-Origin"))
}
