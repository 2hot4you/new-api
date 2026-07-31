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

func TestDashboardListModelsIncludesTaskOnlyStarAIModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	DashboardListModels(ctx)

	var response struct {
		Success bool                `json:"success"`
		Data    map[string][]string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, []string{
		"doubao-seedance-2-0-260128",
		"doubao-seedance-2-0-fast-260128",
	}, response.Data[strconv.Itoa(constant.ChannelTypeStarAI)])
}

func TestChannelListModelsIncludesStarAIGlobalModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	ChannelListModels(ctx)

	var response struct {
		Success bool               `json:"success"`
		Data    []dto.OpenAIModels `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)

	wanted := map[string]bool{
		"doubao-seedance-2-0-260128":      false,
		"doubao-seedance-2-0-fast-260128": false,
	}
	for _, item := range response.Data {
		if _, ok := wanted[item.Id]; !ok {
			continue
		}
		assert.False(t, wanted[item.Id], "global model must not be duplicated: %s", item.Id)
		wanted[item.Id] = true
		assert.Equal(t, "model", item.Object)
		assert.Equal(t, "starai", item.OwnedBy)
	}
	for modelName, found := range wanted {
		assert.True(t, found, "global model is missing: %s", modelName)
	}
}
