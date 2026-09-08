package service

import (
	"context"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestPollFailureThresholdEnqueuesOnePrivateBillingIntent(t *testing.T) {
	truncate(t)
	previous := constant.TaskPollMaxFailures
	constant.TaskPollMaxFailures = 20
	t.Cleanup(func() { constant.TaskPollMaxFailures = previous })
	task := makeTask(812, 812, 4000, 0, BillingSourceWallet, 0)
	task.TaskID = "task_public_failure_threshold"
	task.PrivateData.UpstreamTaskID = "private-provider-id"
	require.NoError(t, model.DB.Create(task).Error)
	originalStatus := task.Status
	for range 19 {
		require.NoError(t, recordPollFailure(context.Background(), nil, task, originalStatus, pollClassTransport, 0, "Bearer secret-token https://private.invalid/private-provider-id"))
	}
	var count int64
	require.NoError(t, model.DB.Model(&model.TaskBillingJob{}).Where("task_id = ?", task.ID).Count(&count).Error)
	require.Zero(t, count)
	require.Equal(t, originalStatus, task.Status)
	require.Equal(t, 19, task.PrivateData.PollFailures)
	require.NoError(t, recordPollFailure(context.Background(), nil, task, originalStatus, pollClassHookError, 502, "Bearer secret-token https://private.invalid/private-provider-id"))
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	require.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	require.Equal(t, 4000, stored.Quota, "settlement belongs to the outbox worker")
	require.NotContains(t, stored.FailReason, "secret-token")
	require.NotContains(t, stored.FailReason, "private-provider-id")
	require.NoError(t, failTaskFromPoll(context.Background(), nil, task, originalStatus, "duplicate stale worker"))
	require.NoError(t, model.DB.Model(&model.TaskBillingJob{}).Where("task_id = ?", task.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestPollOversizedPluginStateDoesNotReplaceDurableState(t *testing.T) {
	truncate(t)
	task := makeTask(813, 813, 4000, 0, BillingSourceWallet, 0)
	task.TaskID = "task_public_oversized_state"
	task.PrivateData.UpstreamTaskID = "private-state-id"
	task.PrivateData.PluginState = []byte(`{"keep":true}`)
	require.NoError(t, model.DB.Create(task).Error)
	adaptor := &scriptedPollingAdaptor{parse: &relaycommon.TaskInfo{
		Status:      model.TaskStatusInProgress,
		PluginState: []byte(strings.Repeat("x", 1024*1024+1)),
	}}
	channel := &model.Channel{Id: 813, Type: constant.ChannelTypeKling}
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, task.GetUpstreamTaskID(), map[string]*model.Task{task.GetUpstreamTaskID(): task}))
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	require.JSONEq(t, `{"keep":true}`, string(stored.PrivateData.PluginState))
	require.Equal(t, 1, stored.PrivateData.PollFailures)
}
