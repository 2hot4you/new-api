package jsplugin

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestTaskAdaptorSingleOversizedStateIsInvalidPoll(t *testing.T) {
	for _, status := range []string{"IN_PROGRESS", "SUCCESS"} {
		t.Run(status, func(t *testing.T) {
			source := strings.Replace(mockPlugin,
				`return {taskId: body.id, status: "SUCCESS", progress: "100%", url: body.url};`,
				`return {taskId: body.id, status: "`+status+`", progress: "100%", state: {value: "x".repeat(1024*1024+1)}};`, 1)
			plugin, err := pluginruntime.NewRegistry().Register(source, pluginruntime.Options{})
			require.NoError(t, err)
			adaptor := New(plugin)
			adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
			result, err := adaptor.ParseTaskResult(&model.Task{}, &http.Response{StatusCode: http.StatusOK}, []byte(`{"id":"upstream"}`))
			require.NoError(t, err)
			require.Equal(t, model.TaskStatusUnknown, result.Status)
			require.Empty(t, result.PluginState)
			require.Empty(t, result.Progress)
		})
	}
}
