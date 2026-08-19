package moliigrok

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
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
		UserId:          42,
		RequestId:       "req_public_image",
		StartTime:       time.Date(2026, time.August, 11, 8, 0, 0, 0, time.UTC),
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		OriginModelName: "grok-imagine-image",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         62,
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

func TestConvertImage20RequestDefaultsQualityAndPreservesExplicitLow(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantQuality string
	}{
		{name: "default medium", body: `{"model":"grok-imagine-image-2.0","prompt":"poster"}`, wantQuality: "medium"},
		{name: "explicit low", body: `{"model":"grok-imagine-image-2.0","prompt":"poster","quality":"low"}`, wantQuality: "low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := imageContext(t, tt.body)
			info := imageInfo()
			info.OriginModelName = "grok-imagine-image-2.0"
			info.ChannelMeta.UpstreamModelName = "grok-imagine-image-2.0"
			converted, err := (&Adaptor{}).ConvertImageRequest(c, info, dto.ImageRequest{Model: "grok-imagine-image-2.0", Prompt: "poster"})
			require.NoError(t, err)
			payload := converted.(imageRequestPayload)
			assert.Equal(t, tt.wantQuality, payload.Quality)
		})
	}
}

func TestConvertImage20RequestRejectsUnsupportedQuality(t *testing.T) {
	c := imageContext(t, `{"model":"grok-imagine-image-2.0","prompt":"poster","quality":"high"}`)
	info := imageInfo()
	info.OriginModelName = "grok-imagine-image-2.0"
	info.ChannelMeta.UpstreamModelName = "grok-imagine-image-2.0"
	_, err := (&Adaptor{}).ConvertImageRequest(c, info, dto.ImageRequest{Model: "grok-imagine-image-2.0", Prompt: "poster"})
	require.ErrorContains(t, err, "quality")
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

func TestConvertImageRequestSupportsOfficialEditPayload(t *testing.T) {
	info := imageInfo()
	info.RelayMode = relayconstant.RelayModeImagesEdits
	info.OriginModelName = "grok-imagine-image-quality"
	info.ChannelMeta.UpstreamModelName = "grok-imagine-image-quality"
	body := `{"model":"grok-imagine-image-quality","prompt":"oil painting","resolution":"2k","image":{"url":"https://images.example/input.png"}}`
	c := imageContext(t, body)
	c.Request.URL.Path = "/v1/images/edits"
	var request dto.ImageRequest
	require.NoError(t, common.Unmarshal([]byte(body), &request))
	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
	require.NoError(t, err)
	encoded, err := common.Marshal(converted)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"grok-imagine-image-quality","prompt":"oil painting","aspect_ratio":"16:9","resolution":"2k","n":1,"image":{"url":"https://images.example/input.png","type":"image_url"}}`, string(encoded))
}

func TestGrokImageResolvesOwnedFileIDBeforeBillingAndSubmit(t *testing.T) {
	body := `{"model":"grok-imagine-image","prompt":"edit","images":[{"url":"https://images.example/a.png"},{"file_id":"file_owned"}]}`
	c := imageContext(t, body)
	c.Request.URL.Path = "/v1/images/edits"
	info := imageInfo()
	info.RelayMode = relayconstant.RelayModeImagesEdits
	var request dto.ImageRequest
	require.NoError(t, common.Unmarshal([]byte(body), &request))
	adaptor := &Adaptor{resolveUserFile: func(_ context.Context, userID int, fileID string, expected model.MoliiFileMediaType) (*model.MoliiFile, string, error) {
		assert.Equal(t, 42, userID)
		assert.Equal(t, "file_owned", fileID)
		assert.Equal(t, model.MoliiFileMediaTypeImage, expected)
		return &model.MoliiFile{FileID: fileID, MediaType: expected}, "https://cos.example/signed.png", nil
	}}

	_, err := adaptor.EstimateImageBilling(c, info, request)
	require.NoError(t, err)
	require.NotNil(t, info.GrokImageBilling)
	assert.Equal(t, 2, info.GrokImageBilling.InputImageCount)
	converted, err := adaptor.ConvertImageRequest(c, info, request)
	require.NoError(t, err)
	encoded, err := common.Marshal(converted)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), "https://cos.example/signed.png")
	assert.NotContains(t, string(encoded), "file_id")
}

func TestGrokImageUnknownFileIDFailsBeforeBilling(t *testing.T) {
	body := `{"model":"grok-imagine-image","prompt":"edit","image":{"file_id":"file_missing"}}`
	c := imageContext(t, body)
	c.Request.URL.Path = "/v1/images/edits"
	info := imageInfo()
	info.RelayMode = relayconstant.RelayModeImagesEdits
	var request dto.ImageRequest
	require.NoError(t, common.Unmarshal([]byte(body), &request))
	adaptor := &Adaptor{resolveUserFile: func(context.Context, int, string, model.MoliiFileMediaType) (*model.MoliiFile, string, error) {
		return nil, "", model.ErrMoliiFileNotFound
	}}

	_, err := adaptor.EstimateImageBilling(c, info, request)
	require.ErrorIs(t, err, model.ErrMoliiFileNotFound)
	assert.Nil(t, info.GrokImageBilling)
	apiErr := adaptor.SanitizeImageRequestError(err)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
	assert.Equal(t, kittypes.ErrorCode("file_not_found"), apiErr.GetErrorCode())
}

func TestGrokImageURLStringAndObjectRemainSupported(t *testing.T) {
	for _, body := range []string{
		`{"model":"grok-imagine-image","prompt":"edit","image":"https://images.example/a.png"}`,
		`{"model":"grok-imagine-image","prompt":"edit","images":[{"url":"https://images.example/a.png"},"data:image/png;base64,AA=="]}`,
	} {
		c := imageContext(t, body)
		c.Request.URL.Path = "/v1/images/edits"
		info := imageInfo()
		info.RelayMode = relayconstant.RelayModeImagesEdits
		var request dto.ImageRequest
		require.NoError(t, common.Unmarshal([]byte(body), &request))

		_, err := (&Adaptor{}).EstimateImageBilling(c, info, request)
		require.NoError(t, err)
		converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
		require.NoError(t, err)
		encoded, err := common.Marshal(converted)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "file_id")
	}
}

func TestEstimateImageBillingSeparatesInputAndOutputUnits(t *testing.T) {
	body := `{"model":"grok-imagine-image-quality","prompt":"edit","resolution":"2k","n":2,"images":[{"url":"https://images.example/a.png"},{"url":"https://images.example/b.png"}]}`
	c := imageContext(t, body)
	c.Request.URL.Path = "/v1/images/edits"
	info := imageInfo()
	info.RelayMode = relayconstant.RelayModeImagesEdits
	info.OriginModelName = "grok-imagine-image-quality"
	info.ChannelMeta.UpstreamModelName = "grok-imagine-image-quality"
	var request dto.ImageRequest
	require.NoError(t, common.Unmarshal([]byte(body), &request))
	ratios, err := (&Adaptor{}).EstimateImageBilling(c, info, request)
	require.NoError(t, err)
	assert.InDelta(t, 0.16, ratios["molii_grok_direct_cost"], 0.000001)
	require.NotNil(t, info.GrokImageBilling)
	assert.Equal(t, 1, info.GrokImageBilling.Version)
	assert.Equal(t, "grok-imagine-image-quality", info.GrokImageBilling.Model)
	assert.Equal(t, "grok-imagine-image-quality", info.GrokImageBilling.RequestedModel)
	assert.Equal(t, "grok-imagine-image-quality", info.GrokImageBilling.BilledModel)
	assert.Equal(t, "edit", info.GrokImageBilling.Operation)
	assert.Equal(t, "2k", info.GrokImageBilling.Resolution)
	assert.Equal(t, "16:9", info.GrokImageBilling.AspectRatio)
	assert.Equal(t, 2, info.GrokImageBilling.RequestedOutputCount)
	assert.Equal(t, 2, info.GrokImageBilling.OutputCount)
	assert.Equal(t, 2, info.GrokImageBilling.InputImageCount)
	assert.InDelta(t, 0.07, info.GrokImageBilling.OutputUnitPrice, 0.000001)
	assert.InDelta(t, 0.01, info.GrokImageBilling.InputUnitPrice, 0.000001)
	assert.InDelta(t, 0.14, info.GrokImageBilling.OutputCost, 0.000001)
	assert.InDelta(t, 0.02, info.GrokImageBilling.InputCost, 0.000001)
	assert.InDelta(t, 0.16, info.GrokImageBilling.Subtotal, 0.000001)
}

func TestEstimateImageBillingSnapshotsGenerationDefaults(t *testing.T) {
	body := `{"model":"grok-imagine-image","prompt":"cat"}`
	c := imageContext(t, body)
	info := imageInfo()

	ratios, err := (&Adaptor{}).EstimateImageBilling(c, info, dto.ImageRequest{Model: "grok-imagine-image", Prompt: "cat"})
	require.NoError(t, err)
	assert.InDelta(t, 0.02, ratios["molii_grok_direct_cost"], 0.000001)
	require.NotNil(t, info.GrokImageBilling)
	assert.Equal(t, "grok-imagine-image", info.GrokImageBilling.Model)
	assert.Equal(t, "grok-imagine-image", info.GrokImageBilling.RequestedModel)
	assert.Equal(t, "grok-imagine-image", info.GrokImageBilling.BilledModel)
	assert.Equal(t, "generation", info.GrokImageBilling.Operation)
	assert.Equal(t, "1k", info.GrokImageBilling.Resolution)
	assert.Equal(t, "16:9", info.GrokImageBilling.AspectRatio)
	assert.Equal(t, 1, info.GrokImageBilling.RequestedOutputCount)
	assert.Equal(t, 1, info.GrokImageBilling.OutputCount)
	assert.Equal(t, 0, info.GrokImageBilling.InputImageCount)
	assert.InDelta(t, 0.02, info.GrokImageBilling.OutputUnitPrice, 0.000001)
	assert.InDelta(t, 0.002, info.GrokImageBilling.InputUnitPrice, 0.000001)
	assert.InDelta(t, 0.02, info.GrokImageBilling.OutputCost, 0.000001)
	assert.InDelta(t, 0.02, info.GrokImageBilling.Subtotal, 0.000001)
}

func TestEstimateImage20BillingUsesQualityTier(t *testing.T) {
	body := `{"model":"grok-imagine-image-2.0","prompt":"poster","quality":"low","resolution":"2k","n":2}`
	c := imageContext(t, body)
	info := imageInfo()
	info.OriginModelName = "grok-imagine-image-2.0"
	info.ChannelMeta.UpstreamModelName = "grok-imagine-image-2.0"

	ratios, err := (&Adaptor{}).EstimateImageBilling(c, info, dto.ImageRequest{Model: "grok-imagine-image-2.0", Prompt: "poster"})
	require.NoError(t, err)
	assert.InDelta(t, 0.12, ratios["molii_grok_direct_cost"], 0.000001)
	require.NotNil(t, info.GrokImageBilling)
	assert.Equal(t, "low", info.GrokImageBilling.Quality)
	assert.Equal(t, "2k", info.GrokImageBilling.Resolution)
	assert.InDelta(t, 0.06, info.GrokImageBilling.OutputUnitPrice, 0.000001)
	assert.InDelta(t, 0.12, info.GrokImageBilling.Subtotal, 0.000001)
}

func TestEstimateImageBillingSnapshotsFinalUpstreamModel(t *testing.T) {
	body := `{"model":"future-grok-image-alias","prompt":"cat","resolution":"2k"}`
	c := imageContext(t, body)
	info := imageInfo()
	info.OriginModelName = "future-grok-image-alias"
	info.ChannelMeta.UpstreamModelName = "grok-imagine-image-quality"

	ratios, err := (&Adaptor{}).EstimateImageBilling(c, info, dto.ImageRequest{Model: "future-grok-image-alias", Prompt: "cat"})

	require.NoError(t, err)
	assert.InDelta(t, 0.07, ratios["molii_grok_direct_cost"], 0.000001)
	require.NotNil(t, info.GrokImageBilling)
	assert.Equal(t, "future-grok-image-alias", info.GrokImageBilling.Model)
	assert.Equal(t, "future-grok-image-alias", info.GrokImageBilling.RequestedModel)
	assert.Equal(t, "grok-imagine-image-quality", info.GrokImageBilling.BilledModel)
	assert.InDelta(t, 0.07, info.GrokImageBilling.OutputUnitPrice, 0.000001)
}

func TestImageResponseExcludesUpstreamCostAndUsesActualCount(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := imageInfo()
	info.PriceData = hosttypes.PriceData{UsePrice: true}
	info.PriceData.AddOtherRatio("molii_grok_direct_cost", 0.08)
	info.GrokImageBilling = &relaycommon.GrokImageBillingSnapshot{
		Version:              1,
		Model:                "grok-imagine-image",
		Operation:            "generation",
		Resolution:           "1k",
		AspectRatio:          "16:9",
		RequestedOutputCount: 4,
		OutputUnitPrice:      0.02,
	}
	c.Set(validatedImageCountContextKey, 4)
	c.Set(imageBillingOutputPriceContextKey, 0.02)
	c.Set(imageBillingInputPriceContextKey, 0.0)
	c.Set(imageBillingInputCountContextKey, 0)
	c.Set(imageBillingBasePriceContextKey, 1.0)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"data":[{"url":"https://imgen.x.ai/a.jpg?token=one","mime_type":"image/jpeg","revised_prompt":"first revision"},{"url":"https://files-cdn.x.ai/b.jpg?token=two","mime_type":"image/jpeg","revised_prompt":"second revision"}],
			"usage":{"cost_in_usd_ticks":200000000}
		}`)),
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.NotContains(t, recorder.Body.String(), "cost_in_usd_ticks")
	assert.JSONEq(t, `{"created":0,"data":[{"url":"https://imgen.x.ai/a.jpg?token=one","b64_json":"","mime_type":"image/jpeg","revised_prompt":"first revision"},{"url":"https://files-cdn.x.ai/b.jpg?token=two","b64_json":"","mime_type":"image/jpeg","revised_prompt":"second revision"}]}`, recorder.Body.String())
	assert.Equal(t, 0.04, info.PriceData.OtherRatios()["molii_grok_direct_cost"])
	assert.Equal(t, 2, info.GrokImageBilling.OutputCount)
	assert.InDelta(t, 0.04, info.GrokImageBilling.OutputCost, 0.000001)
	assert.Zero(t, info.GrokImageBilling.InputCost)
	assert.InDelta(t, 0.04, info.GrokImageBilling.Subtotal, 0.000001)
}

func TestImageResponseKeepsPaidSuccessWhenPreviewRegistrationIsUnavailable(t *testing.T) {
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = previousEnabled
		common.RDB = previousRDB
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(validatedImageCountContextKey, 1)
	info := imageInfo()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"url":"https://imgen.x.ai/paid-result.webp?token=private"}]}`)),
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Contains(t, recorder.Body.String(), "paid-result.webp")
	assert.False(t, info.GrokImagePreviewAvailable)
}

func TestImageResponseRegistersTrustedResultsForOwnerPreview(t *testing.T) {
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	server := miniredis.RunT(t)
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled = previousEnabled
		common.RDB = previousRDB
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(validatedImageCountContextKey, 1)
	info := imageInfo()
	resultURL := "https://imgen.x.ai/preview-result.webp?token=private"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"url":"` + resultURL + `"}]}`)),
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.True(t, info.GrokImagePreviewAvailable)
	urls, err := service.GetGrokImagePreview(info.UserId, info.RequestId)
	require.NoError(t, err)
	assert.Equal(t, []string{resultURL}, urls)
}

func TestImageResponseKeepsPaidSuccessBoundedWhenPreviewRedisStalls(t *testing.T) {
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	stop := make(chan struct{})
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		<-stop
	}()
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{
		Addr:         listener.Addr().String(),
		TLSConfig:    &tls.Config{InsecureSkipVerify: true},
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		PoolTimeout:  time.Second,
	})
	t.Cleanup(func() {
		close(stop)
		_ = listener.Close()
		_ = common.RDB.Close()
		common.RedisEnabled = previousEnabled
		common.RDB = previousRDB
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(validatedImageCountContextKey, 1)
	info := imageInfo()
	resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"url":"https://imgen.x.ai/paid-stalled-preview.webp?token=private"}]}`))}

	startedAt := time.Now()
	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	elapsed := time.Since(startedAt)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Less(t, elapsed, 500*time.Millisecond)
	assert.Contains(t, recorder.Body.String(), "paid-stalled-preview.webp")
	assert.False(t, info.GrokImagePreviewAvailable)
}

func TestImageResponseReturnsTrustedXAIURLForAllSupportedModels(t *testing.T) {
	for _, modelName := range []string{"grok-imagine-image", "grok-imagine-image-quality", "grok-imagine-image-2.0"} {
		t.Run(modelName, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set(validatedImageCountContextKey, 1)
			info := imageInfo()
			info.OriginModelName = modelName
			info.ChannelMeta.UpstreamModelName = modelName
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"data":[{"url":"https://files-cdn.x.ai/result.webp?token=signed","mime_type":"image/webp","revised_prompt":"safe revision"}]
				}`)),
			}

			_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

			require.Nil(t, apiErr)
			assert.JSONEq(t, `{"created":0,"data":[{"url":"https://files-cdn.x.ai/result.webp?token=signed","b64_json":"","mime_type":"image/webp","revised_prompt":"safe revision"}]}`, recorder.Body.String())
		})
	}
}

func TestImageResponsePreservesZeroPricedBillingSnapshot(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := imageInfo()
	info.PriceData = hosttypes.PriceData{UsePrice: true}
	info.PriceData.AddOtherRatio("molii_grok_direct_cost", 0.02)
	info.GrokImageBilling = &relaycommon.GrokImageBillingSnapshot{
		Version:              1,
		Model:                "grok-imagine-image",
		Operation:            "generation",
		Resolution:           "1k",
		AspectRatio:          "16:9",
		RequestedOutputCount: 1,
		OutputCount:          1,
		OutputUnitPrice:      0,
		InputUnitPrice:       0,
	}
	c.Set(validatedImageCountContextKey, 1)
	c.Set(imageBillingOutputPriceContextKey, 0.0)
	c.Set(imageBillingInputPriceContextKey, 0.0)
	c.Set(imageBillingInputCountContextKey, 0)
	c.Set(imageBillingBasePriceContextKey, 1.0)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"url":"https://imgen.x.ai/free.jpg","mime_type":"image/jpeg"}]}`)),
	}

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	assert.Equal(t, 1, info.GrokImageBilling.OutputCount)
	assert.Zero(t, info.GrokImageBilling.OutputUnitPrice)
	assert.Zero(t, info.GrokImageBilling.InputUnitPrice)
	assert.Zero(t, info.GrokImageBilling.OutputCost)
	assert.Zero(t, info.GrokImageBilling.InputCost)
	assert.Zero(t, info.GrokImageBilling.Subtotal)
	encoded, err := common.Marshal(info.GrokImageBilling)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"output_unit_price":0`)
	assert.Contains(t, string(encoded), `"subtotal":0`)
}

func TestImageResponseRejectsUntrustedURLBeforeBillingMutation(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(validatedImageCountContextKey, 2)
	c.Set(imageBillingOutputPriceContextKey, 0.02)
	c.Set(imageBillingInputPriceContextKey, 0.0)
	c.Set(imageBillingInputCountContextKey, 0)
	c.Set(imageBillingBasePriceContextKey, 1.0)
	info := imageInfo()
	info.PriceData = hosttypes.PriceData{UsePrice: true}
	info.PriceData.AddOtherRatio("molii_grok_direct_cost", 0.04)
	info.GrokImageBilling = &relaycommon.GrokImageBillingSnapshot{
		Version:              1,
		RequestedOutputCount: 2,
		OutputCount:          2,
		OutputUnitPrice:      0.02,
		OutputCost:           0.04,
		Subtotal:             0.04,
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{"data":[
			{"url":"https://images.example/secret-a.jpg","mime_type":"image/jpeg"},
			{"url":"https://images.example/secret-b.jpg","mime_type":"image/jpeg"}
		]}`)),
	}
	var logBuffer bytes.Buffer
	oldErrorWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	t.Cleanup(func() { gin.DefaultErrorWriter = oldErrorWriter })
	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Equal(t, "Molii Grok Imagine API request failed", apiErr.Error())
	require.Empty(t, recorder.Body.String())
	require.Equal(t, 0.04, info.PriceData.OtherRatios()["molii_grok_direct_cost"])
	require.Equal(t, 2, info.GrokImageBilling.OutputCount)
	require.Equal(t, 0.04, info.GrokImageBilling.Subtotal)
	require.NotContains(t, apiErr.Error(), "images.example")
	logText := logBuffer.String()
	require.Contains(t, logText, "event=grok_image_result_validation_failed")
	require.NotContains(t, logText, "event=grok_image_result_persistence_failed")
	require.Contains(t, logText, `request_id="req_public_image"`)
	require.Contains(t, logText, "user_id=42")
	require.Contains(t, logText, "channel_id=62")
	require.Contains(t, logText, `stage="validate_source"`)
	require.Contains(t, logText, `error_category="untrusted_result_url"`)
	require.Contains(t, logText, "remote_status=0")
	require.NotContains(t, logText, "images.example")
	require.NotContains(t, logText, "signature=")
	require.NotContains(t, logText, "Bearer-do-not-log")
	require.NotContains(t, logText, "SecretKey")
}

func TestGrokImageValidationFailureLogIncludesOnlyIntegerRemoteStatus(t *testing.T) {
	var logBuffer bytes.Buffer
	oldErrorWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	t.Cleanup(func() { gin.DefaultErrorWriter = oldErrorWriter })

	logGrokImageValidationFailureFieldsWithRemoteStatus(
		context.Background(),
		imageInfo(),
		"remote_fetch",
		"non_success_status",
		"imgen.x.ai",
		http.StatusForbidden,
	)

	logText := logBuffer.String()
	require.Contains(t, logText, "event=grok_image_result_validation_failed")
	require.NotContains(t, logText, "event=grok_image_result_persistence_failed")
	require.Contains(t, logText, `request_id="req_public_image"`)
	require.Contains(t, logText, `stage="remote_fetch"`)
	require.Contains(t, logText, `error_category="non_success_status"`)
	require.Contains(t, logText, "remote_status=403")
	require.Contains(t, logText, `source_host="imgen.x.ai"`)
	require.NotContains(t, logText, "http://")
	require.NotContains(t, logText, "https://")
	require.NotContains(t, logText, "?")
}

func TestImageResponseParseFailureLogsSafeStageWithoutResponseBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	info := imageInfo()
	secretResponse := `{"data":[{"url":"https://images.example/result?signature=do-not-log"}],"authorization":"Bearer-do-not-log"`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(secretResponse)),
	}
	var logBuffer bytes.Buffer
	oldErrorWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	t.Cleanup(func() { gin.DefaultErrorWriter = oldErrorWriter })

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.NotContains(t, apiErr.Error(), "images.example")
	logText := logBuffer.String()
	require.Contains(t, logText, `request_id="req_public_image"`)
	require.Contains(t, logText, `model="grok-imagine-image"`)
	require.Contains(t, logText, "user_id=42")
	require.Contains(t, logText, "channel_id=62")
	require.Contains(t, logText, `stage="parse_upstream_response"`)
	require.Contains(t, logText, `error_category="invalid_response_json"`)
	require.NotContains(t, logText, "images.example")
	require.NotContains(t, logText, "signature=")
	require.NotContains(t, logText, "Bearer-do-not-log")
}

func TestGrokImageValidationLogHandlesMissingChannelMetadata(t *testing.T) {
	var logBuffer bytes.Buffer
	oldErrorWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	t.Cleanup(func() { gin.DefaultErrorWriter = oldErrorWriter })
	info := &relaycommon.RelayInfo{
		UserId:          42,
		RequestId:       "req_without_channel_meta",
		OriginModelName: "grok-imagine-image",
	}

	require.NotPanics(t, func() {
		logGrokImageValidationFailureFields(context.Background(), info, "validate_source", "untrusted_result_url", "")
	})

	logText := logBuffer.String()
	require.Contains(t, logText, "channel_id=0")
	require.Contains(t, logText, `stage="validate_source"`)
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

func TestSanitizeImageErrorMapsContentModeration(t *testing.T) {
	apiErr := (&Adaptor{}).SanitizeImageError(http.StatusBadRequest, []byte(`{
		"error": {
			"code": "imagine:content-moderated",
			"message": "Generated image rejected (Request-ID: upstream-123)",
			"type": "provider_error"
		}
	}`))
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "content_policy_violation", string(apiErr.GetErrorCode()))
	assert.Equal(t, "Image request rejected by content safety policy", apiErr.Error())
	assert.NotContains(t, apiErr.Error(), "upstream-123")
}
