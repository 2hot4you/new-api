package moliigrok

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func imageContext(t *testing.T, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyRequestBody, []byte(body))
	t.Cleanup(func() { common.CleanupBodyStorage(c) })
	return c
}

func imageInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		OriginModelName: "grok-imagine-image",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://internal.invalid/xai",
			ApiKey:            "secret",
			UpstreamModelName: "grok-imagine-image",
		},
	}
}

func TestConvertImageRequestPreservesAllSupportedFields(t *testing.T) {
	body := `{"model":"grok-imagine-image-quality","prompt":"  orange cat  ","aspect_ratio":"9:16","resolution":"2k","n":4}`
	c := imageContext(t, body)
	info := imageInfo()
	info.OriginModelName = "grok-imagine-image-quality"
	info.ChannelMeta.UpstreamModelName = "grok-imagine-image-quality"

	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, dto.ImageRequest{Model: info.OriginModelName, Prompt: "  orange cat  ", N: common.GetPointer(uint(4))})
	require.NoError(t, err)
	encoded, err := common.Marshal(converted)
	require.NoError(t, err)

	assert.JSONEq(t, `{"model":"grok-imagine-image-quality","prompt":"orange cat","aspect_ratio":"9:16","resolution":"2k","n":4}`, string(encoded))
}

func TestConvertImageRequestAppliesDefaults(t *testing.T) {
	body := `{"model":"grok-imagine-image","prompt":"orange cat"}`
	c := imageContext(t, body)

	converted, err := (&Adaptor{}).ConvertImageRequest(c, imageInfo(), dto.ImageRequest{Model: "grok-imagine-image", Prompt: "orange cat"})
	require.NoError(t, err)
	encoded, err := common.Marshal(converted)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"grok-imagine-image","prompt":"orange cat","aspect_ratio":"16:9","resolution":"1k","n":1}`, string(encoded))
}

func TestConvertImageRequestRejectsInvalidInputs(t *testing.T) {
	longPrompt := strings.Repeat("猫", 10001)
	tests := []struct {
		name string
		body string
	}{
		{name: "explicit zero", body: `{"model":"grok-imagine-image","prompt":"cat","n":0}`},
		{name: "too many", body: `{"model":"grok-imagine-image","prompt":"cat","n":5}`},
		{name: "long prompt", body: `{"model":"grok-imagine-image","prompt":"` + longPrompt + `"}`},
		{name: "resolution", body: `{"model":"grok-imagine-image","prompt":"cat","resolution":"4k"}`},
		{name: "aspect ratio", body: `{"model":"grok-imagine-image","prompt":"cat","aspect_ratio":"7:5"}`},
		{name: "model", body: `{"model":"grok-imagine-video-1.5","prompt":"cat"}`},
		{name: "blank prompt", body: `{"model":"grok-imagine-image","prompt":"   "}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := imageContext(t, tt.body)
			var request dto.ImageRequest
			require.NoError(t, common.Unmarshal([]byte(tt.body), &request))
			_, err := (&Adaptor{}).ConvertImageRequest(c, imageInfo(), request)
			require.Error(t, err)
			apiErr := (&Adaptor{}).SanitizeImageRequestError(err)
			assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		})
	}
}

func TestConvertImageRequestRejectsEdits(t *testing.T) {
	info := imageInfo()
	info.RelayMode = relayconstant.RelayModeImagesEdits
	_, err := (&Adaptor{}).ConvertImageRequest(imageContext(t, `{}`), info, dto.ImageRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
	assert.Equal(t, http.StatusBadRequest, (&Adaptor{}).SanitizeImageRequestError(err).StatusCode)
}

func TestImageResponseExcludesUpstreamCostAndUsesActualCount(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := imageInfo()
	info.PriceData = hosttypes.PriceData{UsePrice: true}
	info.PriceData.AddOtherRatio("n", 4)
	c.Set(validatedImageCountContextKey, 4)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"data":[{"url":"https://images.example/a.jpg","mime_type":"image/jpeg"},{"url":"https://images.example/b.jpg","mime_type":"image/jpeg"}],
			"usage":{"cost_in_usd_ticks":200000000}
		}`)),
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.NotContains(t, recorder.Body.String(), "cost_in_usd_ticks")
	assert.NotContains(t, recorder.Body.String(), "mime_type")
	assert.JSONEq(t, `{"created":0,"data":[{"url":"https://images.example/a.jpg","b64_json":"","revised_prompt":""},{"url":"https://images.example/b.jpg","b64_json":"","revised_prompt":""}]}`, recorder.Body.String())
	assert.Equal(t, 2.0, info.PriceData.OtherRatios()["n"])
}

func TestSanitizeImageErrorNeverReturnsRawProviderDetails(t *testing.T) {
	apiErr := (&Adaptor{}).SanitizeImageError(http.StatusBadRequest, []byte(`{"error":{"message":"bad (Request-ID: upstream-123)","code":"invalid"},"url":"https://api.wxiai.com/xai"}`))
	require.NotNil(t, apiErr)
	text := apiErr.Error()
	assert.Contains(t, text, "Molii Grok Imagine API request failed")
	assert.NotContains(t, text, "upstream-123")
	assert.NotContains(t, text, "wxiai")
	transportErr := (&Adaptor{}).SanitizeImageTransportError(assert.AnError)
	assert.Equal(t, http.StatusBadGateway, transportErr.StatusCode)
	assert.Equal(t, "Molii Grok Imagine API request failed", transportErr.Error())
	assert.False(t, (&Adaptor{}).AllowImageRequestBodyLog())
}
