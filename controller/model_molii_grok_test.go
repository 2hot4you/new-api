package controller

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var moliiGrokModels = []string{
	"grok-imagine-image",
	"grok-imagine-image-quality",
	"grok-imagine-video",
	"grok-imagine-video-1.5",
}

func TestDashboardListModelsIncludesAllMoliiGrokModels(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	DashboardListModels(c)
	var response struct {
		Success bool                `json:"success"`
		Data    map[string][]string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.ElementsMatch(t, moliiGrokModels, response.Data[strconv.Itoa(constant.ChannelTypeMoliiGrokAIGC)])
}

func TestChannelListModelsIncludesMoliiGrokGlobalModelsOnce(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ChannelListModels(c)
	var response struct {
		Success bool               `json:"success"`
		Data    []dto.OpenAIModels `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	counts := make(map[string]int)
	for _, item := range response.Data {
		for _, modelName := range moliiGrokModels {
			if item.Id == modelName {
				counts[modelName]++
				assert.Equal(t, "Molii Grok Imagine API", item.OwnedBy)
			}
		}
	}
	for _, modelName := range moliiGrokModels {
		assert.Equal(t, 1, counts[modelName], "model must be globally registered exactly once: %s", modelName)
	}
}
