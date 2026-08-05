package helper

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoliiGrokModelsHaveNoGuessedBuiltInPrices(t *testing.T) {
	models := []string{
		"grok-imagine-image",
		"grok-imagine-image-quality",
		"grok-imagine-video-1.5",
	}
	for _, modelName := range models {
		_, hasDefaultPrice := ratio_setting.GetDefaultModelPriceMap()[modelName]
		assert.False(t, hasDefaultPrice, "price must be configured by an administrator: %s", modelName)
		_, hasDefaultRatio := ratio_setting.GetDefaultModelRatioMap()[modelName]
		assert.False(t, hasDefaultRatio, "a token ratio must not silently replace fixed pricing: %s", modelName)
	}
}

func TestMoliiGrokMissingFixedPriceReturnsExplicitError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{OriginModelName: "grok-imagine-video-1.5", UsingGroup: "default"}

	_, err := ModelPriceHelperPerCall(c, info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "grok-imagine-video-1.5")
	assert.Contains(t, err.Error(), "价格")

	imageInfo := &relaycommon.RelayInfo{OriginModelName: "grok-imagine-image-quality", UsingGroup: "default"}
	_, err = ModelPriceHelper(c, imageInfo, 0, &types.TokenCountMeta{BillingRatios: map[string]float64{"n": 1}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "grok-imagine-image-quality")
}
