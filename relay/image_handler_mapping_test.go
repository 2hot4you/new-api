package relay

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareImageModelMappingConsumesControllerPreparationOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("model_mapping", `{"grok-imagine-image":"grok-imagine-video"}`)
	request := &dto.ImageRequest{Model: "grok-imagine-image", Prompt: "cat"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-image",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-imagine-image",
		},
		ImageModelMappingPrepared: true,
	}

	err := prepareImageModelMapping(c, info, request)

	require.NoError(t, err)
	assert.Equal(t, "grok-imagine-image", request.Model)
	assert.False(t, info.ImageModelMappingPrepared)
}
