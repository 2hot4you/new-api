package controller_test

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/QuantumNous/new-api/common"
	apiController "github.com/QuantumNous/new-api/controller"
	apiRouter "github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/setting/brand_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func performBrandRequest(path string, handler gin.HandlerFunc, headers ...http.Header) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/branding/claudeye/wordmark.svg", handler)
	router.GET("/api/branding/claudeye/favicon.svg", handler)

	request := httptest.NewRequest(http.MethodGet, path, nil)
	if len(headers) > 0 {
		request.Header = headers[0]
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func withClaudeyePaletteOptions(t *testing.T, palette brand_setting.ClaudeyePalette) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{
		brand_setting.ClaudeyeLightMarkColorKey: palette.LightMark,
		brand_setting.ClaudeyeLightTextColorKey: palette.LightText,
		brand_setting.ClaudeyeDarkMarkColorKey:  palette.DarkMark,
		brand_setting.ClaudeyeDarkTextColorKey:  palette.DarkText,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func TestGetClaudeyeWordmarkPreviewOverridesDoNotPersist(t *testing.T) {
	withClaudeyePaletteOptions(t, brand_setting.DefaultClaudeyePalette)

	recorder := performBrandRequest(
		"/api/branding/claudeye/wordmark.svg?surface=light&mark=%23112233&text=%23445566",
		apiController.GetClaudeyeWordmark,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "#112233")
	require.Contains(t, recorder.Body.String(), "#445566")
	require.Equal(t, brand_setting.DefaultClaudeyePalette, brand_setting.GetClaudeyePalette())
}

func TestGetClaudeyeWordmarkDefaultsToLightSurface(t *testing.T) {
	palette := brand_setting.ClaudeyePalette{
		LightMark: "#111111", LightText: "#222222",
		DarkMark: "#EEEEEE", DarkText: "#DDDDDD",
	}
	withClaudeyePaletteOptions(t, palette)

	recorder := performBrandRequest("/api/branding/claudeye/wordmark.svg", apiController.GetClaudeyeWordmark)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "#111111")
	require.Contains(t, recorder.Body.String(), "#222222")
	require.NotContains(t, recorder.Body.String(), "#EEEEEE")
}

func TestGetClaudeyeWordmarkRejectsInvalidSurfaceAndOverrides(t *testing.T) {
	withClaudeyePaletteOptions(t, brand_setting.DefaultClaudeyePalette)
	tests := []string{
		"/api/branding/claudeye/wordmark.svg?surface=sepia",
		"/api/branding/claudeye/wordmark.svg?surface=light&mark=red",
		"/api/branding/claudeye/wordmark.svg?surface=dark&text=%23ABC",
		"/api/branding/claudeye/wordmark.svg?mark=",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			recorder := performBrandRequest(path, apiController.GetClaudeyeWordmark)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestClaudeyeBrandResponsesSetSecurityAndRevalidationHeaders(t *testing.T) {
	withClaudeyePaletteOptions(t, brand_setting.DefaultClaudeyePalette)
	tests := []struct {
		path    string
		handler gin.HandlerFunc
	}{
		{"/api/branding/claudeye/wordmark.svg?surface=dark", apiController.GetClaudeyeWordmark},
		{"/api/branding/claudeye/favicon.svg", apiController.GetClaudeyeFavicon},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			first := performBrandRequest(test.path, test.handler)
			require.Equal(t, http.StatusOK, first.Code)
			require.Equal(t, "image/svg+xml; charset=utf-8", first.Header().Get("Content-Type"))
			require.Equal(t, "nosniff", first.Header().Get("X-Content-Type-Options"))
			require.Equal(t, "no-cache, must-revalidate", first.Header().Get("Cache-Control"))
			etag := first.Header().Get("ETag")
			require.Regexp(t, regexp.MustCompile(`^"[0-9a-f]{64}"$`), etag)

			conditional := performBrandRequest(test.path, test.handler, http.Header{"If-None-Match": []string{etag}})
			require.Equal(t, http.StatusNotModified, conditional.Code)
			require.Empty(t, conditional.Body.Bytes())
			require.Equal(t, etag, conditional.Header().Get("ETag"))
		})
	}
}

func TestGetClaudeyeWordmarkRequiresExactETagMatch(t *testing.T) {
	withClaudeyePaletteOptions(t, brand_setting.DefaultClaudeyePalette)
	first := performBrandRequest("/api/branding/claudeye/wordmark.svg", apiController.GetClaudeyeWordmark)
	etag := first.Header().Get("ETag")

	multiple := performBrandRequest(
		"/api/branding/claudeye/wordmark.svg",
		apiController.GetClaudeyeWordmark,
		http.Header{"If-None-Match": []string{`"other", ` + etag}},
	)
	require.Equal(t, http.StatusOK, multiple.Code)
}

func TestClaudeyeBrandingRoutesArePubliclyRegistered(t *testing.T) {
	withClaudeyePaletteOptions(t, brand_setting.DefaultClaudeyePalette)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	apiRouter.SetApiRouter(engine)

	for _, path := range []string{
		"/api/branding/claudeye/wordmark.svg",
		"/api/branding/claudeye/favicon.svg",
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, recorder.Code, path)
		require.Equal(t, "image/svg+xml; charset=utf-8", recorder.Header().Get("Content-Type"), path)
	}
}

func newClaudeyeBrandingServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	apiRouter.SetApiRouter(engine)
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	return server, &http.Client{Transport: &http.Transport{DisableCompression: true}}
}

func requestBrandResource(
	t *testing.T,
	client *http.Client,
	url string,
	acceptEncoding string,
	etag string,
) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	request.Header.Set("Accept-Encoding", acceptEncoding)
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	response, err := client.Do(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	return response, body
}

func requireBrandAssetResponseContract(t *testing.T, response *http.Response, body []byte) string {
	t.Helper()
	require.Equal(t, http.StatusOK, response.StatusCode)
	digest := sha256.Sum256(body)
	etag := fmt.Sprintf(`"%x"`, digest)
	require.Equal(t, etag, response.Header.Get("ETag"), "ETag must be the SHA-256 of the actual response body")
	require.Empty(t, response.Header.Get("Content-Encoding"), "branding SVG must not be transformed after its strong ETag is calculated")
	require.Equal(t, "image/svg+xml; charset=utf-8", response.Header.Get("Content-Type"))
	require.Equal(t, "nosniff", response.Header.Get("X-Content-Type-Options"))
	require.Equal(t, "no-cache, must-revalidate", response.Header.Get("Cache-Control"))
	return etag
}

func TestClaudeyeBrandingRouteETagMatchesActualBodyAcrossAcceptEncoding(t *testing.T) {
	withClaudeyePaletteOptions(t, brand_setting.DefaultClaudeyePalette)
	server, client := newClaudeyeBrandingServer(t)

	for _, path := range []string{
		"/api/branding/claudeye/wordmark.svg",
		"/api/branding/claudeye/favicon.svg",
	} {
		for _, acceptEncoding := range []string{"identity", "gzip"} {
			t.Run(path+"/"+acceptEncoding, func(t *testing.T) {
				response, body := requestBrandResource(t, client, server.URL+path, acceptEncoding, "")
				requireBrandAssetResponseContract(t, response, body)
			})
		}
	}
}

func TestClaudeyeBrandingRoutesReturnEmpty304OverHTTP(t *testing.T) {
	withClaudeyePaletteOptions(t, brand_setting.DefaultClaudeyePalette)
	server, client := newClaudeyeBrandingServer(t)

	for _, path := range []string{
		"/api/branding/claudeye/wordmark.svg",
		"/api/branding/claudeye/favicon.svg",
	} {
		t.Run(path, func(t *testing.T) {
			first, body := requestBrandResource(t, client, server.URL+path, "gzip", "")
			etag := requireBrandAssetResponseContract(t, first, body)

			conditional, conditionalBody := requestBrandResource(t, client, server.URL+path, "gzip", etag)
			require.Equal(t, http.StatusNotModified, conditional.StatusCode)
			require.Empty(t, conditionalBody)
			require.Equal(t, etag, conditional.Header.Get("ETag"))
			require.Empty(t, conditional.Header.Get("Content-Encoding"))
			require.Equal(t, "nosniff", conditional.Header.Get("X-Content-Type-Options"))
			require.Equal(t, "no-cache, must-revalidate", conditional.Header.Get("Cache-Control"))
		})
	}
}

func TestClaudeyeBrandingRoutesReturn200WhenContentChanges(t *testing.T) {
	withClaudeyePaletteOptions(t, brand_setting.DefaultClaudeyePalette)
	server, client := newClaudeyeBrandingServer(t)

	wordmark, wordmarkBody := requestBrandResource(t, client, server.URL+"/api/branding/claudeye/wordmark.svg", "gzip", "")
	wordmarkETag := requireBrandAssetResponseContract(t, wordmark, wordmarkBody)
	preview, previewBody := requestBrandResource(
		t,
		client,
		server.URL+"/api/branding/claudeye/wordmark.svg?mark=%23010203&text=%23040506",
		"gzip",
		wordmarkETag,
	)
	previewETag := requireBrandAssetResponseContract(t, preview, previewBody)
	require.NotEqual(t, wordmarkETag, previewETag)

	favicon, faviconBody := requestBrandResource(t, client, server.URL+"/api/branding/claudeye/favicon.svg", "gzip", "")
	faviconETag := requireBrandAssetResponseContract(t, favicon, faviconBody)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[brand_setting.ClaudeyeLightMarkColorKey] = "#010203"
	common.OptionMapRWMutex.Unlock()
	changedFavicon, changedFaviconBody := requestBrandResource(
		t,
		client,
		server.URL+"/api/branding/claudeye/favicon.svg",
		"gzip",
		faviconETag,
	)
	changedFaviconETag := requireBrandAssetResponseContract(t, changedFavicon, changedFaviconBody)
	require.NotEqual(t, faviconETag, changedFaviconETag)
}
