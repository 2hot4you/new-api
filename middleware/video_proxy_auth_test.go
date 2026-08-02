package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoProxyAuthAcceptsSignedPlaybackURLWithoutAuthorizationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	playbackURL, err := url.Parse(service.BuildSignedVideoProxyURL("task_public", 42))
	require.NoError(t, err)

	router := gin.New()
	router.GET("/v1/videos/:task_id/content", VideoProxyAuth(), func(c *gin.Context) {
		assert.Equal(t, 42, c.GetInt("id"))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, playbackURL.RequestURI(), nil)
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestVideoProxyAuthRejectsTamperedSignedPlaybackURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	playbackURL, err := url.Parse(service.BuildSignedVideoProxyURL("task_public", 42))
	require.NoError(t, err)
	query := playbackURL.Query()
	query.Set("signature", "tampered")
	playbackURL.RawQuery = query.Encode()

	router := gin.New()
	router.GET("/v1/videos/:task_id/content", VideoProxyAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, playbackURL.RequestURI(), nil)
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
