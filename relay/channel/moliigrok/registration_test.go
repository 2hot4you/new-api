package moliigrok_test

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoliiGrokChannelRegistration(t *testing.T) {
	assert.Equal(t, 61, constant.ChannelTypeStarAI)
	assert.Equal(t, 62, constant.ChannelTypeMoliiGrokAIGC)
	assert.Equal(t, 63, constant.ChannelTypeDummy)
	require.Len(t, constant.ChannelBaseURLs, constant.ChannelTypeDummy)
	assert.Equal(t, "https://api.wxiai.com/xai", constant.ChannelBaseURLs[constant.ChannelTypeMoliiGrokAIGC])
	assert.Equal(t, "Molii Grok Imagine API", constant.GetChannelTypeName(constant.ChannelTypeMoliiGrokAIGC))

	apiType, ok := common.ChannelType2APIType(constant.ChannelTypeMoliiGrokAIGC)
	require.True(t, ok)
	assert.Equal(t, constant.APITypeMoliiGrokAIGC, apiType)
	imageAdaptor := relay.GetAdaptor(apiType)
	require.NotNil(t, imageAdaptor)
	assert.Equal(t, "Molii Grok Imagine API", imageAdaptor.GetChannelName())
	assert.ElementsMatch(t, []string{"grok-imagine-image", "grok-imagine-image-quality", "grok-imagine-image-2.0"}, imageAdaptor.GetModelList())

	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMoliiGrokAIGC))
	videoAdaptor := relay.GetTaskAdaptor(platform)
	require.NotNil(t, videoAdaptor)
	assert.Equal(t, "Molii Grok Imagine API", videoAdaptor.GetChannelName())
	assert.ElementsMatch(t, []string{"grok-imagine-video", "grok-imagine-video-1.5"}, videoAdaptor.GetModelList())
	assert.False(t, relay.TaskAdaptorAllowsRetry(platform))

	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeImageGeneration}, common.GetEndpointTypesByChannelType(constant.ChannelTypeMoliiGrokAIGC, "grok-imagine-image"))
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeImageGeneration}, common.GetEndpointTypesByChannelType(constant.ChannelTypeMoliiGrokAIGC, "grok-imagine-image-2.0"))
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, common.GetEndpointTypesByChannelType(constant.ChannelTypeMoliiGrokAIGC, "grok-imagine-video-1.5"))
	assert.Empty(t, common.GetEndpointTypesByChannelType(constant.ChannelTypeMoliiGrokAIGC, "grok-imagine-video-1.5-preview"))
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, common.GetEndpointTypesByChannelType(constant.ChannelTypeMoliiGrokAIGC, "grok-imagine-video"))
}
