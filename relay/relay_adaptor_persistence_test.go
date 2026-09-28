package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/moliigrok"
	taskbytedanceseedance "github.com/QuantumNous/new-api/relay/channel/task/bytedanceseedance"
	taskmoliigrok "github.com/QuantumNous/new-api/relay/channel/task/moliigrok"
	taskstarai "github.com/QuantumNous/new-api/relay/channel/task/starai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistedChannelMoliiTaskPlatformsSelectNativeAdaptors(t *testing.T) {
	tests := []struct {
		name        string
		platform    constant.TaskPlatform
		wantAdaptor any
		wantName    string
	}{
		{
			name:        "persisted type 61 selects StarAI",
			platform:    constant.TaskPlatform("61"),
			wantAdaptor: &taskstarai.TaskAdaptor{},
			wantName:    "molii-aigc",
		},
		{
			name:        "persisted type 62 selects Molii Grok AIGC",
			platform:    constant.TaskPlatform("62"),
			wantAdaptor: &taskmoliigrok.TaskAdaptor{},
			wantName:    "Molii Grok Imagine API",
		},
		{
			name:        "persisted type 64 selects ByteDance Seedance",
			platform:    constant.TaskPlatform("64"),
			wantAdaptor: &taskbytedanceseedance.TaskAdaptor{},
			wantName:    "bytedance-seedance",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adaptor := GetTaskAdaptor(test.platform)
			require.NotNil(t, adaptor)
			assert.IsType(t, test.wantAdaptor, adaptor)
			assert.Equal(t, test.wantName, adaptor.GetChannelName())
		})
	}
}

func TestPersistedChannelMoliiGrokSelectsOrdinaryAdaptor(t *testing.T) {
	apiType, mapped := common.ChannelType2APIType(62)
	require.True(t, mapped)

	adaptor := GetAdaptor(apiType)
	require.NotNil(t, adaptor)
	assert.IsType(t, &moliigrok.Adaptor{}, adaptor)
	assert.Equal(t, "Molii Grok Imagine API", adaptor.GetChannelName())
}

func TestPersistedChannelTaskPluginStaysOnTaskPlatform(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("channel_type", 63)

	assert.Equal(t, constant.TaskPlatform("63"), GetTaskPlatform(ctx))
	apiType, mapped := common.ChannelType2APIType(63)
	assert.Equal(t, -1, apiType)
	assert.False(t, mapped)
	assert.Nil(t, GetAdaptor(apiType))
}
