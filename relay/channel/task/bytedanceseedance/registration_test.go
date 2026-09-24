package bytedanceseedance_test

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestByteDanceSeedanceTaskRouting(t *testing.T) {
	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeByteDanceSeedance))
	adaptor := relay.GetTaskAdaptor(platform)
	require.NotNil(t, adaptor)
	assert.Equal(t, []string{
		"doubao-seedance-2-0-260128",
		"doubao-seedance-2-0-fast-260128",
		"doubao-seedance-2-0-mini-260615",
		"doubao-seedance-2-5-260628",
	}, adaptor.GetModelList())
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, common.GetEndpointTypesByChannelType(constant.ChannelTypeByteDanceSeedance, adaptor.GetModelList()[0]))
	assert.False(t, relay.TaskAdaptorAllowsRetry(platform))
	_, mapped := common.ChannelType2APIType(constant.ChannelTypeByteDanceSeedance)
	assert.False(t, mapped)
}
