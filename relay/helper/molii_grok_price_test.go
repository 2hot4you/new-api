package helper

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoliiGrokModelsUseDirectCostAnchors(t *testing.T) {
	models := []string{
		"grok-imagine-image",
		"grok-imagine-image-quality",
		"grok-imagine-video",
		"grok-imagine-video-1.5",
	}
	for _, modelName := range models {
		price, hasDefaultPrice := ratio_setting.GetDefaultModelPriceMap()[modelName]
		assert.True(t, hasDefaultPrice, "direct-cost anchor missing: %s", modelName)
		assert.Equal(t, 1.0, price)
		_, hasDefaultRatio := ratio_setting.GetDefaultModelRatioMap()[modelName]
		assert.False(t, hasDefaultRatio, "a token ratio must not silently replace fixed pricing: %s", modelName)
	}
	_, hasPreviewPrice := ratio_setting.GetDefaultModelPriceMap()["grok-imagine-video-1.5-preview"]
	assert.False(t, hasPreviewPrice)
}

func TestMoliiGrokFixedPriceAnchorsAreAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{OriginModelName: "grok-imagine-video-1.5", UsingGroup: "default"}

	price, err := ModelPriceHelperPerCall(c, info)
	require.NoError(t, err)
	assert.Equal(t, 1.0, price.ModelPrice)

}
