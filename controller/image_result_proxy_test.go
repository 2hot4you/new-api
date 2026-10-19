package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestImageResultProxyServesBytesWithoutUpstreamMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "true")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/proxy/opaque", nil)
	c.Request.Header.Set("Authorization", "Bearer client-secret")
	c.Params = gin.Params{{Key: "token", Value: "opaque"}}
	image := []byte("validated raster bytes")
	serveImageResultProxy(c,
		func(token string) (string, error) {
			assert.Equal(t, "opaque", token)
			return "https://private-provider.example/result?secret=source-secret", nil
		},
		func(ctx context.Context, source string) ([]byte, string, error) {
			assert.Equal(t, c.Request.Context(), ctx)
			assert.Equal(t, "https://private-provider.example/result?secret=source-secret", source)
			return image, "image/png", nil
		},
	)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, image, recorder.Body.Bytes())
	assert.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Empty(t, recorder.Header().Get("Location"))
	assert.Empty(t, recorder.Header().Get("Set-Cookie"))
	assert.NotContains(t, recorder.Header(), "Authorization")
}

func TestImageResultProxySanitizesFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name       string
		enabled    string
		resolveErr error
		fetchErr   error
		status     int
		code       string
	}{
		{"disabled", "false", nil, nil, http.StatusNotFound, "image_link_invalid"},
		{"invalid", "true", service.ErrImageResultProxyTokenInvalid, nil, http.StatusNotFound, "image_link_invalid"},
		{"expired", "true", service.ErrImageResultProxyExpired, nil, http.StatusGone, "image_link_expired"},
		{"fetch failed", "true", nil, errors.New("Get https://private-provider.example/secret: failed"), http.StatusBadGateway, "image_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("IMAGE_RESULT_PROXY_ENABLED", tc.enabled)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/proxy/token", nil)
			resolveCalled, fetchCalled := false, false
			serveImageResultProxy(c, func(string) (string, error) {
				resolveCalled = true
				return "https://private-provider.example/secret", tc.resolveErr
			}, func(context.Context, string) ([]byte, string, error) {
				fetchCalled = true
				return nil, "", tc.fetchErr
			})
			assert.Equal(t, tc.status, recorder.Code)
			assert.Contains(t, recorder.Body.String(), tc.code)
			assert.NotContains(t, recorder.Body.String(), "private-provider")
			assert.NotContains(t, recorder.Body.String(), "secret")
			assert.Empty(t, recorder.Header().Get("Location"))
			assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			assert.Equal(t, tc.enabled == "true", resolveCalled)
			assert.Equal(t, tc.enabled == "true" && tc.resolveErr == nil, fetchCalled)
		})
	}
}
