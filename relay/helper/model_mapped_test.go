package helper

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelMappedHelperRejectsIncompatibleImagineMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("model_mapping", `{"grok-imagine-image":"grok-imagine-video"}`)
	request := &dto.GeneralOpenAIRequest{Model: "grok-imagine-image"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-image",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-image"},
	}

	err := ModelMappedHelper(c, info, request)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "incompatible_imagine_model_mapping")
	assert.Equal(t, "grok-imagine-image", request.Model)
}

func TestModelMappedHelperPreservesOrdinaryMappings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("model_mapping", `{"gpt-4o":"gpt-4.1"}`)
	request := &dto.GeneralOpenAIRequest{Model: "gpt-4o"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-4o",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"},
	}

	require.NoError(t, ModelMappedHelper(c, info, request))
	assert.True(t, info.IsModelMapped)
	assert.Equal(t, "gpt-4.1", info.UpstreamModelName)
	assert.Equal(t, "gpt-4.1", request.Model)
}
