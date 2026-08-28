package starai_test

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskAdaptorAndEndpointRegistration(t *testing.T) {
	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI))
	adaptor := relay.GetTaskAdaptor(platform)
	require.NotNil(t, adaptor)
	assert.Equal(t, "molii-aigc", adaptor.GetChannelName())
	assert.Equal(t, []string{
		"doubao-seedance-2-0-260128",
		"doubao-seedance-2-0-fast-260128",
		"doubao-seedance-2-0-mini-260615",
		"doubao-seedance-2-5-260628",
	}, adaptor.GetModelList())
	assert.False(t, relay.TaskAdaptorAllowsRetry(platform))
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, common.GetEndpointTypesByChannelType(constant.ChannelTypeStarAI, adaptor.GetModelList()[0]))
	assert.Equal(t, constant.ChannelTypeStarAI+2, constant.ChannelTypeDummy)
	_, chatMapped := common.ChannelType2APIType(constant.ChannelTypeStarAI)
	assert.False(t, chatMapped, "video provider must remain task-only and must not use a chat adaptor")
}
