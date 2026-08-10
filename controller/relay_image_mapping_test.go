package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func imageBillingMappingContext(t *testing.T, modelName, mapping string, channelType int) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	body := fmt.Sprintf(`{"model":%q,"prompt":"cat","resolution":"1k"}`, modelName)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyRequestBody, []byte(body))
	c.Set("model_mapping", mapping)
	common.SetContextKey(c, constant.ContextKeyChannelType, channelType)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, modelName)
	t.Cleanup(func() { common.CleanupBodyStorage(c) })
	return c
}

func TestPrepareImageRequestBillingRejectsMappingBeforeEstimateOrPreconsume(t *testing.T) {
	c := imageBillingMappingContext(t, "grok-imagine-image", `{"grok-imagine-image":"grok-imagine-video"}`, constant.ChannelTypeMoliiGrokAIGC)
	request := &dto.ImageRequest{Model: "grok-imagine-image", Prompt: "cat"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-image",
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		Request:         request,
	}

	_, apiErr := prepareImageRequestBilling(c, info, request)

	require.NotNil(t, apiErr)
	assert.Contains(t, apiErr.Error(), "incompatible_imagine_model_mapping")
	assert.Nil(t, info.GrokImageBilling, "estimator must not run after mapping validation fails")
	assert.Nil(t, info.Billing, "pre-consumption must not start after mapping validation fails")
}

func TestPrepareImageRequestBillingAllowsGrokImageIdentity(t *testing.T) {
	for _, modelName := range []string{"grok-imagine-image", "grok-imagine-image-quality"} {
		t.Run(modelName, func(t *testing.T) {
			mapping := fmt.Sprintf(`{%q:%q}`, modelName, modelName)
			c := imageBillingMappingContext(t, modelName, mapping, constant.ChannelTypeMoliiGrokAIGC)
			request := &dto.ImageRequest{Model: modelName, Prompt: "cat"}
			info := &relaycommon.RelayInfo{
				OriginModelName: modelName,
				RelayMode:       relayconstant.RelayModeImagesGenerations,
				Request:         request,
			}

			ratios, apiErr := prepareImageRequestBilling(c, info, request)

			require.Nil(t, apiErr)
			require.Contains(t, ratios, "molii_grok_direct_cost")
			require.NotNil(t, info.GrokImageBilling)
			assert.Equal(t, modelName, info.GrokImageBilling.RequestedModel)
			assert.Equal(t, modelName, info.GrokImageBilling.BilledModel)
			assert.True(t, info.ImageModelMappingPrepared)
		})
	}
}

func TestPrepareImageRequestBillingPreservesOrdinaryImageMapping(t *testing.T) {
	c := imageBillingMappingContext(t, "dall-e-3", `{"dall-e-3":"gpt-image-1"}`, constant.ChannelTypeOpenAI)
	request := &dto.ImageRequest{Model: "dall-e-3", Prompt: "cat"}
	info := &relaycommon.RelayInfo{OriginModelName: "dall-e-3", Request: request}

	ratios, apiErr := prepareImageRequestBilling(c, info, request)

	require.Nil(t, apiErr)
	assert.Nil(t, ratios)
	assert.Equal(t, "gpt-image-1", request.Model)
	assert.Equal(t, "gpt-image-1", info.UpstreamModelName)
	assert.True(t, info.ImageModelMappingPrepared)
}
