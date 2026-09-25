package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resellerSeedanceTask() *model.Task {
	task := makeTask(701, 701, 718937, 701, BillingSourceWallet, 0)
	task.Platform = constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeByteDanceSeedance))
	task.TaskID = "task_reseller_local"
	task.PrivateData.UpstreamTaskID = "task_molii_public"
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		OriginModelName: "seedance-2-0", ModelRatio: 2, GroupRatio: 0.5,
		OtherRatios: map[string]float64{"local_discount": 0.25},
	}
	return task
}

func TestByteDanceSeedanceBillingUsesOnlyReliableUsageAndLocalSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result relaycommon.TaskInfo
		change func(*model.Task)
		want   *int
	}{
		{name: "missing usage requires review"},
		{name: "finite quota overflow requires review", result: relaycommon.TaskInfo{TotalTokens: common.MaxQuota}, change: func(task *model.Task) { task.PrivateData.BillingContext.ModelRatio = 100 }},
		{name: "infinite calculation requires review", result: relaycommon.TaskInfo{TotalTokens: 100}, change: func(task *model.Task) { task.PrivateData.BillingContext.ModelRatio = math.MaxFloat64 }},
		{name: "infinite snapshot requires review", result: relaycommon.TaskInfo{TotalTokens: 100}, change: func(task *model.Task) { task.PrivateData.BillingContext.GroupRatio = math.Inf(1) }},
		{name: "total tokens", result: relaycommon.TaskInfo{TotalTokens: 100, CompletionTokens: 80}, want: intPointerSeedance(25)},
		{name: "completion fallback", result: relaycommon.TaskInfo{CompletionTokens: 80}, want: intPointerSeedance(20)},
		{name: "negative usage requires review", result: relaycommon.TaskInfo{TotalTokens: -1, CompletionTokens: 80}},
		{name: "missing snapshot requires review", result: relaycommon.TaskInfo{TotalTokens: 100}, change: func(task *model.Task) { task.PrivateData.BillingContext = nil }},
		{name: "missing local price requires review", result: relaycommon.TaskInfo{TotalTokens: 100}, change: func(task *model.Task) { task.PrivateData.BillingContext.ModelRatio = 0 }},
		{name: "invalid local price requires review", result: relaycommon.TaskInfo{TotalTokens: 100}, change: func(task *model.Task) { task.PrivateData.BillingContext.ModelRatio = math.NaN() }},
		{name: "invalid multiplier requires review", result: relaycommon.TaskInfo{TotalTokens: 100}, change: func(task *model.Task) { task.PrivateData.BillingContext.OtherRatios["local_discount"] = -1 }},
		{name: "no current group lookup", result: relaycommon.TaskInfo{TotalTokens: 100}, change: func(task *model.Task) { task.Group = "" }, want: intPointerSeedance(25)},
		{name: "explicit free group", result: relaycommon.TaskInfo{TotalTokens: 100}, change: func(task *model.Task) {
			require.NoError(t, common.Unmarshal([]byte(`{"group_ratio":0,"group_ratio_captured":true}`), task.PrivateData.BillingContext))
		}, want: intPointerSeedance(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := resellerSeedanceTask()
			task.Status = model.TaskStatusSuccess
			if tc.change != nil {
				tc.change(task)
			}
			job := BuildTerminalTaskBillingJob(context.Background(), &mockAdaptor{adjustReturn: 99999}, task, &tc.result)
			require.NotNil(t, job)
			assert.Equal(t, model.TaskBillingOperationSettle, job.Operation)
			assert.Equal(t, 718937, job.FromQuota)
			assert.Equal(t, tc.want, job.TargetQuota, "never settle to an upstream cost or a guessed reservation")
			if tc.want != nil {
				assert.Positive(t, task.PrivateData.BillingContext.ActualTokens)
			}
		})
	}
}

func intPointerSeedance(value int) *int { return &value }

func TestByteDanceSeedanceBillingGroupRatioProvenanceSurvivesJSON(t *testing.T) {
	for _, tc := range []struct {
		name, snapshot string
		want           *int
	}{
		{"absent group ratio", `{"model_ratio":2}`, nil},
		{"legacy zero is ambiguous", `{"model_ratio":2,"group_ratio":0}`, nil},
		{"explicit captured zero", `{"model_ratio":2,"group_ratio_captured":true}`, intPointerSeedance(0)},
		{"legacy positive", `{"model_ratio":2,"group_ratio":0.5}`, intPointerSeedance(100)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := resellerSeedanceTask()
			task.Status = model.TaskStatusSuccess
			var private model.TaskPrivateData
			require.NoError(t, private.Scan(`{"billing_context":`+tc.snapshot+`}`))
			encoded, err := private.Value()
			require.NoError(t, err)
			task.PrivateData = model.TaskPrivateData{}
			require.NoError(t, task.PrivateData.Scan(encoded))
			job := BuildTerminalTaskBillingJob(context.Background(), &mockAdaptor{}, task, &relaycommon.TaskInfo{TotalTokens: 100})
			require.NotNil(t, job)
			assert.Equal(t, tc.want, job.TargetQuota)
		})
	}
}

func TestByteDanceSeedancePollingRetryableErrorsRetainState(t *testing.T) {
	previous := constant.TaskPollMaxFailures
	constant.TaskPollMaxFailures = 1
	t.Cleanup(func() { constant.TaskPollMaxFailures = previous })
	for _, code := range []int{0, 401, 403, 429, 502, 503, 504} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			truncate(t)
			task := resellerSeedanceTask()
			task.Progress = "50%"
			require.NoError(t, model.DB.Create(task).Error)
			adaptor := &scriptedPollingAdaptor{statusCode: code}
			if code == 0 {
				adaptor.fetchErr = errors.New("transport cgt-private https://private.invalid?secret=canary")
			}
			ch := &model.Channel{Id: task.ChannelId, Type: constant.ChannelTypeByteDanceSeedance}
			for attempt := 0; attempt < 2; attempt++ {
				reloaded := loadReconciliationTask(t, task.ID)
				require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, ch, task.GetUpstreamTaskID(), map[string]*model.Task{task.GetUpstreamTaskID(): &reloaded}))
				require.Equal(t, model.TaskStatus(model.TaskStatusInProgress), loadReconciliationTask(t, task.ID).Status)
			}
			persisted := loadReconciliationTask(t, task.ID)
			assert.Equal(t, model.TaskStatus(model.TaskStatusInProgress), persisted.Status)
			assert.Equal(t, "50%", persisted.Progress)
			assert.Equal(t, 718937, persisted.Quota)
			assert.Empty(t, persisted.FailReason)
			assert.Zero(t, persisted.FinishTime)
			var jobs int64
			require.NoError(t, model.DB.Model(&model.TaskBillingJob{}).Count(&jobs).Error)
			assert.Zero(t, jobs)
		})
	}
}
