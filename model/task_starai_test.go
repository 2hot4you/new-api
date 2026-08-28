package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestInitTaskPersistsStarAIKeyForOriginalChannelPolling(t *testing.T) {
	task := InitTask(constant.TaskPlatform("61"), &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeStarAI,
			ChannelId:   27,
			ApiKey:      "dedicated-model-key",
		},
	})

	require.Equal(t, 27, task.ChannelId)
	require.Equal(t, "dedicated-model-key", task.PrivateData.Key)
}
