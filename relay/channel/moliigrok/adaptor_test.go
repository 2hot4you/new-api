package moliigrok

import (
	"context"
	"errors"
	"io"
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
		UserId:          42,
		RequestId:       "req_public_image",
		StartTime:       time.Date(2026, time.August, 11, 8, 0, 0, 0, time.UTC),
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
			"data":[{"url":"https://images.example/a.jpg","mime_type":"image/jpeg","revised_prompt":"first revision"},{"url":"https://images.example/b.jpg","mime_type":"image/jpeg","revised_prompt":"second revision"}],
			"usage":{"cost_in_usd_ticks":200000000}
		}`)),
	}
	var persistenceRequest service.GrokImagePersistenceRequest
	adaptor := &Adaptor{persistImageResults: func(_ context.Context, request service.GrokImagePersistenceRequest) ([]service.GrokImagePersistedResult, error) {
		persistenceRequest = request
		return []service.GrokImagePersistedResult{
			{URL: "https://cos.example/signed-a", MIMEType: "image/jpeg", RevisedPrompt: "first revision"},
			{URL: "https://cos.example/signed-b", MIMEType: "image/jpeg", RevisedPrompt: "second revision"},
		}, nil
	}}

	usage, apiErr := adaptor.DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, info.UserId, persistenceRequest.UserID)
	require.Equal(t, info.RequestId, persistenceRequest.RequestID)
	require.Equal(t, info.StartTime, persistenceRequest.CreatedAt)
	require.Equal(t, []service.GrokImageSource{
		{URL: "https://images.example/a.jpg", MIMEType: "image/jpeg", RevisedPrompt: "first revision"},
		{URL: "https://images.example/b.jpg", MIMEType: "image/jpeg", RevisedPrompt: "second revision"},
	}, persistenceRequest.Images)
	assert.NotContains(t, recorder.Body.String(), "cost_in_usd_ticks")
	assert.NotContains(t, recorder.Body.String(), "images.example")
	assert.JSONEq(t, `{"created":0,"data":[{"url":"https://cos.example/signed-a","b64_json":"","mime_type":"image/jpeg","revised_prompt":"first revision"},{"url":"https://cos.example/signed-b","b64_json":"","mime_type":"image/jpeg","revised_prompt":"second revision"}]}`, recorder.Body.String())
	assert.Equal(t, 0.04, info.PriceData.OtherRatios()["molii_grok_direct_cost"])
	assert.Equal(t, 2, info.GrokImageBilling.OutputCount)
	assert.InDelta(t, 0.04, info.GrokImageBilling.OutputCost, 0.000001)
	assert.Zero(t, info.GrokImageBilling.InputCost)
	assert.InDelta(t, 0.04, info.GrokImageBilling.Subtotal, 0.000001)
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
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"url":"https://images.example/free.jpg","mime_type":"image/jpeg"}]}`)),
	}
	adaptor := &Adaptor{persistImageResults: func(_ context.Context, _ service.GrokImagePersistenceRequest) ([]service.GrokImagePersistedResult, error) {
		return []service.GrokImagePersistedResult{{URL: "https://cos.example/free", MIMEType: "image/jpeg"}}, nil
	}}

	_, apiErr := adaptor.DoResponse(c, resp, info)
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

func TestImageResponsePersistenceFailureReturnsSanitizedErrorBeforeBillingMutation(t *testing.T) {
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
	adaptor := &Adaptor{persistImageResults: func(context.Context, service.GrokImagePersistenceRequest) ([]service.GrokImagePersistedResult, error) {
		return nil, errors.New("copy https://images.example/secret-a.jpg failed")
	}}

	usage, apiErr := adaptor.DoResponse(c, resp, info)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Equal(t, "Molii Grok Imagine API request failed", apiErr.Error())
	require.Empty(t, recorder.Body.String())
	require.Equal(t, 0.04, info.PriceData.OtherRatios()["molii_grok_direct_cost"])
	require.Equal(t, 2, info.GrokImageBilling.OutputCount)
	require.Equal(t, 0.04, info.GrokImageBilling.Subtotal)
	require.NotContains(t, apiErr.Error(), "images.example")
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
