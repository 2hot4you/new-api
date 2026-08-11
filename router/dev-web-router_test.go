package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetDevWebRouterProxiesFrontendRequestsAndKeepsBackendNamespacesLocal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var upstreamRequests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("dev-web:" + r.URL.RequestURI()))
	}))
	t.Cleanup(upstream.Close)

	engine := gin.New()
	require.NoError(t, SetDevWebRouter(engine, upstream.URL))
	gateway := httptest.NewServer(engine)
	t.Cleanup(gateway.Close)

	for _, path := range []string{"/channels", "/static/js/index.js?hot=1"} {
		status, body := requestDevWebRoute(t, gateway.URL+path)
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, "dev-web:"+path, body)
	}
	assert.Equal(t, int32(2), upstreamRequests.Load())

	for _, path := range []string{
		"/api/not-found",
		"/v1/not-found",
		"/v1beta/not-found",
		"/assets/not-found",
		"/mj/not-found",
		"/pg/not-found",
	} {
		status, body := requestDevWebRoute(t, gateway.URL+path)
		assert.Equal(t, http.StatusNotFound, status, path)
		assert.Contains(t, body, "Invalid URL", path)
	}
	assert.Equal(t, int32(2), upstreamRequests.Load())
}

func TestSetDevWebRouterRejectsInvalidTargetAndReturnsBadGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	require.Error(t, SetDevWebRouter(engine, "not-an-absolute-url"))

	engine = gin.New()
	require.NoError(t, SetDevWebRouter(engine, "http://127.0.0.1:1"))
	gateway := httptest.NewServer(engine)
	t.Cleanup(gateway.Close)
	status, body := requestDevWebRoute(t, gateway.URL+"/channels")
	assert.Equal(t, http.StatusBadGateway, status)
	assert.Contains(t, body, "frontend development server is unavailable")
}

func TestSetRouterUsesFrontendDevServerWhenConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("current-development-frontend"))
	}))
	t.Cleanup(upstream.Close)
	t.Setenv("FRONTEND_DEV_SERVER_URL", upstream.URL)
	t.Setenv("FRONTEND_BASE_URL", "")

	engine := gin.New()
	SetRouter(engine, WebAssets{})
	gateway := httptest.NewServer(engine)
	t.Cleanup(gateway.Close)
	status, body := requestDevWebRoute(t, gateway.URL+"/channels")

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "current-development-frontend", body)
}

func requestDevWebRoute(t *testing.T, requestURL string) (int, string) {
	t.Helper()
	response, err := http.Get(requestURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, string(body)
}
