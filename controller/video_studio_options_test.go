package controller

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoStudioMaskedKeyUsesClientFacingSKFormat(t *testing.T) {
	assert.Equal(t, "sk-abcdxxxxwxyz", videoStudioMaskedKey("abcdefghijklmnopwxyz"))
	assert.Equal(t, "sk-xxxx", videoStudioMaskedKey("short"))
}

func TestCacheVideoStudioEstimateAssetsAvoidsUpstreamVerification(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	service.SetVideoStudioRequestSnapshot(c, &model.VideoStudioRequestSnapshot{
		Media: []model.VideoStudioMediaReference{{Source: "asset", AssetID: "asset-local"}},
	})
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeStarAI)
	info := &relaycommon.RelayInfo{UserId: 42}
	lookups := 0

	taskErr := cacheVideoStudioEstimateAssets(c, info, func(id string, userID int) (*service.StarAIAssetBinding, error) {
		lookups++
		assert.Equal(t, "asset-local", id)
		assert.Equal(t, 42, userID)
		return &service.StarAIAssetBinding{
			ID:          id,
			UpstreamID:  "upstream-local",
			ChannelType: constant.ChannelTypeStarAI,
			Status:      "SUCCESS",
			ExpiresAt:   time.Now().Add(time.Hour).Unix(),
		}, nil
	})

	require.Nil(t, taskErr)
	assert.Equal(t, 1, lookups)
	cached, exists := c.Get(videoStudioResolvedAssetCacheKey)
	require.True(t, exists)
	assert.Equal(t, map[string]string{
		"asset://asset-local": "asset://upstream-local",
	}, cached)
}

func TestCacheVideoStudioEstimateAssetsRejectsUnavailableLocalBinding(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	service.SetVideoStudioRequestSnapshot(c, &model.VideoStudioRequestSnapshot{
		Media: []model.VideoStudioMediaReference{{Source: "asset", AssetID: "asset-pending"}},
	})
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeStarAI)
	info := &relaycommon.RelayInfo{UserId: 42}

	taskErr := cacheVideoStudioEstimateAssets(c, info, func(string, int) (*service.StarAIAssetBinding, error) {
		return &service.StarAIAssetBinding{
			ID:          "asset-pending",
			UpstreamID:  "upstream-pending",
			ChannelType: constant.ChannelTypeStarAI,
			Status:      "PROCESSING",
			ExpiresAt:   time.Now().Add(time.Hour).Unix(),
		}, nil
	})

	require.NotNil(t, taskErr)
	_, exists := c.Get(videoStudioResolvedAssetCacheKey)
	assert.False(t, exists)
}
