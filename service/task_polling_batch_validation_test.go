package service

import (
	"context"
	"encoding/json"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestBatchPollInvalidResultReachesCutoffWithoutPartialMutation(t *testing.T) {
	for _, invalid := range []string{"oversized_state", "oversized_data", "invalid_state", "unmarshalable_data"} {
		t.Run(invalid, func(t *testing.T) {
			truncate(t)
			previous := constant.TaskPollMaxFailures
			constant.TaskPollMaxFailures = 3
			t.Cleanup(func() { constant.TaskPollMaxFailures = previous })
			seedTaskPollingChannel(t, 815, true)
			task := makeTask(815, 815, 4000, 0, BillingSourceWallet, 0)
			task.TaskID = "task_batch_invalid"
			task.PrivateData.UpstreamTaskID = "upstream_batch_invalid"
			task.PrivateData.PluginState = json.RawMessage(`{"keep":true}`)
			task.Action = "original"
			task.FailReason = "original reason"
			task.SubmitTime = 10
			task.StartTime = 20
			task.FinishTime = 0
			task.Progress = "10%"
			task.Data = json.RawMessage(`{"old":true}`)
			require.NoError(t, model.DB.Create(task).Error)
			result := &BatchTaskResult{Action: "invalid action", SubmitTime: 100, StartTime: 200, FinishTime: 300, TaskInfo: relaycommon.TaskInfo{Status: model.TaskStatusInProgress, Reason: "invalid reason", Progress: "90%", PluginState: json.RawMessage(`{"replace":true}`)}, Data: map[string]any{"new": true}}
			switch invalid {
			case "oversized_state":
				result.TaskInfo.PluginState = []byte(strings.Repeat(" ", 1024*1024) + `{}`)
			case "oversized_data":
				result.Data = strings.Repeat("x", 1024*1024)
			case "invalid_state":
				result.TaskInfo.PluginState = []byte(`{"broken"`)
			case "unmarshalable_data":
				result.Data = make(chan int)
			}
			adaptor := &scriptedBatchPollingAdaptor{results: map[string]*BatchTaskResult{task.GetUpstreamTaskID(): result}}
			for attempt := 1; attempt <= 3; attempt++ {
				require.NoError(t, UpdateBatchTasks(context.Background(), adaptor, map[int][]string{815: {task.GetUpstreamTaskID()}}, map[string]*model.Task{task.GetUpstreamTaskID(): task}))
				var stored model.Task
				require.NoError(t, model.DB.First(&stored, task.ID).Error)
				require.Equal(t, attempt, stored.PrivateData.PollFailures)
				require.Equal(t, "original", stored.Action)
				require.EqualValues(t, 10, stored.SubmitTime)
				require.EqualValues(t, 20, stored.StartTime)
				require.JSONEq(t, `{"keep":true}`, string(stored.PrivateData.PluginState))
				require.JSONEq(t, `{"old":true}`, string(stored.Data))
				if attempt < 3 {
					require.Equal(t, model.TaskStatus(model.TaskStatusInProgress), stored.Status)
					require.Equal(t, "original reason", stored.FailReason)
					require.Zero(t, stored.FinishTime)
					require.Equal(t, "10%", stored.Progress)
				} else {
					require.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
					require.NotEqual(t, "invalid reason", stored.FailReason)
					var count int64
					require.NoError(t, model.DB.Model(&model.TaskBillingJob{}).Where("task_id = ?", task.ID).Count(&count).Error)
					require.EqualValues(t, 1, count)
				}
				task = &stored
			}
		})
	}
}
