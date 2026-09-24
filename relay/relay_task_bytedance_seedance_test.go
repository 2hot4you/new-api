package relay

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedanceLegacyQueryBody(t *testing.T, task *model.Task) []byte {
	t.Helper()
	body, err := common.Marshal(dto.TaskResponse[any]{Code: dto.TaskSuccessCode, Data: TaskModel2Dto(task)})
	require.NoError(t, err)
	return body
}

func TestByteDanceSeedanceLegacyQueryReusesSafePollingFacts(t *testing.T) {
	platform := constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeByteDanceSeedance))
	adaptor := GetTaskAdaptor(platform)
	polling := adaptor.(interface {
		SafePollingData(*relaycommon.TaskInfo) []byte
	})
	task := &model.Task{
		TaskID: "task_local", Platform: platform, Status: model.TaskStatusSuccess,
		Data: polling.SafePollingData(&relaycommon.TaskInfo{Status: model.TaskStatusSuccess, TotalTokens: 40, CompletionTokens: 30}),
	}
	body := seedanceLegacyQueryBody(t, task)
	result, err := adaptor.ParseTaskResult(nil, nil, body)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, 40, result.TotalTokens, "the next reseller hop must receive settlement facts")
	assert.Equal(t, 30, result.CompletionTokens)
	assert.Contains(t, string(body), `"task_id":"task_local"`)
}

func TestByteDanceSeedanceLegacyQueryRejectsPrivateAndUntrustworthyData(t *testing.T) {
	platform := constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeByteDanceSeedance))
	for _, test := range []struct {
		name, data        string
		completion, total int
	}{
		{"contaminated safe snapshot", `{"status":"fixture-diagnostic","total_tokens":40,"completion_tokens":30,"api_key":"fixture-private-key","upstream_id":"cgt-private","task_id":"task_molii_private","url":"https://private.invalid?signature=fixture-signed","data":{"usage":{"total_tokens":999},"upstream_id":"cgt-private"}}`, 30, 40},
		{"completion fallback", `{"completion_tokens":30}`, 30, 0},
		{"missing usage", `{}`, 0, 0},
		{"nested historical usage is not trusted", `{"data":{"usage":{"total_tokens":999}}}`, 0, 0},
		{"negative total", `{"total_tokens":-1,"completion_tokens":30}`, 0, 0},
		{"negative completion", `{"total_tokens":40,"completion_tokens":-1}`, 0, 0},
		{"null total", `{"total_tokens":null,"completion_tokens":30}`, 0, 0},
		{"fractional total", `{"total_tokens":1.5,"completion_tokens":30}`, 0, 0},
		{"string total", `{"total_tokens":"40","completion_tokens":30}`, 0, 0},
		{"overflow total", `{"total_tokens":999999999999999999999,"completion_tokens":30}`, 0, 0},
		{"invalid JSON", `not-json-fixture-private-key`, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			task := &model.Task{
				TaskID: "task_local", Platform: platform, Status: model.TaskStatusSuccess,
				Data: json.RawMessage(test.data), FailReason: "fixture-diagnostic",
				PrivateData: model.TaskPrivateData{Key: "fixture-private-key", UpstreamTaskID: "task_molii_private", ResultURL: "https://private.invalid?signature=fixture-signed"},
			}
			body := seedanceLegacyQueryBody(t, task)
			for _, private := range []string{"fixture-private-key", "task_molii_private", "cgt-private", "private.invalid", "fixture-signed", "fixture-diagnostic"} {
				assert.NotContains(t, string(body), private)
			}
			result, err := GetTaskAdaptor(platform).ParseTaskResult(nil, nil, body)
			require.NoError(t, err)
			assert.Equal(t, model.TaskStatusSuccess, result.Status, "database status is authoritative")
			assert.Equal(t, test.completion, result.CompletionTokens)
			assert.Equal(t, test.total, result.TotalTokens)
		})
	}
}

func TestByteDanceSeedanceLegacyQueryFailureUsesGenericReason(t *testing.T) {
	task := &model.Task{TaskID: "task_local", Platform: constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeByteDanceSeedance)), Status: model.TaskStatusFailure, FailReason: "fixture-private-provider-diagnostic"}
	body := seedanceLegacyQueryBody(t, task)
	assert.NotContains(t, string(body), "fixture-private-provider-diagnostic")
	assert.Contains(t, string(body), "Molii video task failed")
	result, err := GetTaskAdaptor(task.Platform).ParseTaskResult(nil, nil, body)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, result.Status)
}
