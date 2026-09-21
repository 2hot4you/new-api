package controller_test

import (
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
