package controller

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAsyncTaskPollHandlerReportsUnfinishedQueryFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.TaskBillingJob{}, &model.SystemTask{}, &model.SystemTaskLock{}))
	previousDB := model.DB
	previousFactory := service.GetTaskAdaptorFunc
	model.DB = db
	service.GetTaskAdaptorFunc = func(constant.TaskPlatform) service.TaskPollingAdaptor { return nil }
	t.Cleanup(func() {
		model.DB = previousDB
		service.GetTaskAdaptorFunc = previousFactory
	})

	const runnerID = "poll-handler-runner"
	activeKey := model.SystemTaskTypeAsyncTaskPoll
	task := &model.SystemTask{
		TaskID: "poll-handler-query-failure", Type: model.SystemTaskTypeAsyncTaskPoll,
		Status: model.SystemTaskStatusRunning, ActiveKey: &activeKey, LockedBy: runnerID,
	}
	require.NoError(t, db.Create(task).Error)
	require.NoError(t, db.Create(&model.SystemTaskLock{
		Type: model.SystemTaskTypeAsyncTaskPoll, TaskID: task.TaskID,
		LockedBy: runnerID, LockedUntil: time.Now().Add(time.Minute).Unix(),
	}).Error)

	callbackName := "test:fail_handler_unfinished_query"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Task" {
			tx.AddError(errors.New("handler unfinished query failed"))
		}
	}))
	t.Cleanup(func() { db.Callback().Query().Remove(callbackName) })

	(asyncTaskPollHandler{}).Run(context.Background(), task, runnerID)
	var reloaded model.SystemTask
	require.NoError(t, db.Where("task_id = ?", task.TaskID).First(&reloaded).Error)
	assert.Equal(t, model.SystemTaskStatusFailed, reloaded.Status)
	assert.Contains(t, reloaded.Error, "handler unfinished query failed")
}

func TestAsyncTaskPollHandlerEnabledOnUnfinishedQueryFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	previousDB := model.DB
	previousUpdateTask := constant.UpdateTask
	model.DB = db
	constant.UpdateTask = true
	t.Cleanup(func() {
		model.DB = previousDB
		constant.UpdateTask = previousUpdateTask
	})

	callbackName := "test:fail_handler_enabled_query"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Task" {
			tx.AddError(errors.New("handler enabled query failed"))
		}
	}))
	t.Cleanup(func() { db.Callback().Query().Remove(callbackName) })

	assert.True(t, (asyncTaskPollHandler{}).Enabled(), "query failure must schedule a run that records failed")
}

func setupTaskDTOBillingDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TaskBillingJob{}))
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
	return db
}

func TestTasksToDtoBillingStateUnavailableWhenJobLookupFails(t *testing.T) {
	db := setupTaskDTOBillingDB(t)

	callbackName := "test:fail_billing_job_lookup"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "TaskBillingJob" {
			tx.AddError(errors.New("injected billing job lookup failure"))
		}
	}))
	t.Cleanup(func() { db.Callback().Query().Remove(callbackName) })

	task := &model.Task{
		ID: 991, TaskID: "modern_terminal_without_lookup", UserId: 42,
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI)),
		Status:   model.TaskStatusSuccess, SubmitTime: model.TaskRefundLegacyCutoff + 1,
	}
	items := tasksToDto([]*model.Task{task}, false)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].Billing)
	assert.Equal(t, "unavailable", items[0].Billing.State)
}

func TestTaskBillingStateUnavailableWithoutJobForModernAndLegacyTerminalTasks(t *testing.T) {
	modern := &model.Task{Status: model.TaskStatusFailure, Quota: 2500, SubmitTime: model.TaskRefundLegacyCutoff + 1}
	legacy := &model.Task{Status: model.TaskStatusSuccess, Quota: 2500, SubmitTime: model.TaskRefundLegacyCutoff - 1}
	assert.Equal(t, "unavailable", service.TaskBillingPublicState(modern, nil))
	assert.Equal(t, "unavailable", service.TaskBillingPublicState(legacy, nil))
}

func TestTasksToDtoExposesOnlySignedStarAIPlaybackURL(t *testing.T) {
	previousAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://configured.example"
	t.Cleanup(func() { system_setting.ServerAddress = previousAddress })

	success := &model.Task{
		TaskID:   "task_preview_success",
		UserId:   42,
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI)),
		Status:   model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://private.example/video.mp4?signature=upstream-secret",
		},
	}
	pending := &model.Task{
		TaskID:   "task_preview_pending",
		UserId:   42,
		Platform: success.Platform,
		Status:   model.TaskStatusInProgress,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://private.example/pending.mp4",
		},
	}

	items := tasksToDto([]*model.Task{success, pending}, false)
	require.Len(t, items, 2)
	assert.True(t, strings.HasPrefix(items[0].ResultURL, "/v1/videos/"), items[0].ResultURL)
	assert.Contains(t, items[0].ResultURL, "/v1/videos/task_preview_success/content?")
	assert.Contains(t, items[0].ResultURL, "signature=")
	assert.NotContains(t, items[0].ResultURL, "configured.example")
	assert.NotContains(t, items[0].ResultURL, "private.example")
	assert.Empty(t, items[1].ResultURL)
}

func TestTasksToDtoExposesOnlySignedMoliiGrokPlaybackURL(t *testing.T) {
	previousAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://configured.example"
	t.Cleanup(func() { system_setting.ServerAddress = previousAddress })

	task := &model.Task{
		TaskID:   "task_grok_preview_success",
		UserId:   42,
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMoliiGrokAIGC)),
		Status:   model.TaskStatusSuccess,
		Data:     []byte(`{"status":"done","url":"https://private.example/result.mp4"}`),
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "upstream-private-id",
			ResultURL:      "https://private.example/video.mp4?signature=upstream-secret",
			BillingContext: &model.TaskBillingContext{
				EstimatedResolution: "480p",
				EstimatedRatio:      "16:9",
				EstimatedSeconds:    5,
			},
		},
	}

	items := tasksToDto([]*model.Task{task}, false)
	require.Len(t, items, 1)
	assert.True(t, strings.HasPrefix(items[0].ResultURL, "/v1/videos/"), items[0].ResultURL)
	assert.Contains(t, items[0].ResultURL, "/v1/videos/task_grok_preview_success/content?")
	assert.NotContains(t, items[0].ResultURL, "configured.example")
	assert.NotContains(t, items[0].ResultURL, "private.example")
	assert.Nil(t, items[0].Data)
	require.NotNil(t, items[0].VideoParams)
	assert.Equal(t, "480p", items[0].VideoParams.Resolution)
	assert.Equal(t, "16:9", items[0].VideoParams.Ratio)
	assert.Equal(t, 5, items[0].VideoParams.Seconds)
}

func TestTasksToDtoExposesSafeSettledGrokBillingSummary(t *testing.T) {
	db := setupTaskDTOBillingDB(t)
	task := &model.Task{
		ID:       1001,
		TaskID:   "task_grok_billing",
		UserId:   42,
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMoliiGrokAIGC)),
		Status:   model.TaskStatusSuccess,
		Quota:    180000,
		PrivateData: model.TaskPrivateData{
			Key:            "must-not-leak",
			UpstreamTaskID: "upstream-must-not-leak",
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "grok-imagine-video",
				GroupRatio:      1,
				GrokVideoBilling: &model.GrokVideoBillingSnapshot{
					Version: 1, Model: "grok-imagine-video", Operation: "video_edit", InputType: "video",
					ActualDurationSeconds: 6, ActualResolution: "480p",
					VideoInputBilledSeconds: 6, OutputUnitPrice: 0.05, VideoInputUnitPrice: 0.01,
					OutputCost: 0.3, VideoInputCost: 0.06, Subtotal: 0.36, GroupRatio: 1, FinalCost: 0.36,
				},
			},
		},
	}
	require.NoError(t, db.Create(&model.TaskBillingJob{
		TaskID: task.ID, IdempotencyKey: "dto-grok-settled", Operation: model.TaskBillingOperationSettle,
		Status: model.TaskBillingJobStatusSucceeded,
	}).Error)

	items := tasksToDto([]*model.Task{task}, false)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].Billing)
	assert.Equal(t, "settled", items[0].Billing.State)
	assert.Equal(t, "grok_video", items[0].Billing.Mode)
	assert.Equal(t, "grok-imagine-video", items[0].Billing.Model)
	assert.InDelta(t, 0.36, items[0].Billing.FinalCost, 1e-9)
	assert.True(t, items[0].Billing.DetailAvailable)
	require.NotNil(t, items[0].Billing.GrokVideo)
	assert.Equal(t, 6.0, items[0].Billing.GrokVideo.ActualDurationSeconds)
	assert.Equal(t, "480p", items[0].Billing.GrokVideo.ActualResolution)

	encoded, err := common.Marshal(items[0])
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "must-not-leak")
}

func TestTasksToDtoExposesSeedanceSettlementAndRefundStates(t *testing.T) {
	db := setupTaskDTOBillingDB(t)
	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI))
	settled := &model.Task{
		ID: 1002, TaskID: "task_seedance_settled", UserId: 42, Platform: platform,
		Status: model.TaskStatusSuccess, Quota: 277500,
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
			OriginModelName: "doubao-seedance-2-0-260128", GroupRatio: 1.5,
			ActualTokens: 10000, EstimatedResolution: "720p", EstimatedRatio: "16:9",
			EstimatedSeconds: 5, EstimatedUnitPrice: 37,
		}},
	}
	refunded := &model.Task{
		ID: 1003, TaskID: "task_seedance_refunded", UserId: 42, Platform: platform,
		Status: model.TaskStatusFailure, Quota: 0,
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
			OriginModelName: "doubao-seedance-2-0-fast-260128",
		}},
	}
	pending := &model.Task{
		TaskID: "task_seedance_pending", UserId: 42, Platform: platform,
		Status: model.TaskStatusInProgress, Quota: 500000,
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
			OriginModelName: "doubao-seedance-2-0-260128", EstimatedTokens: 10000,
		}},
	}
	require.NoError(t, db.Create(&model.TaskBillingJob{
		TaskID: settled.ID, IdempotencyKey: "dto-seedance-settled", Operation: model.TaskBillingOperationSettle,
		Status: model.TaskBillingJobStatusSucceeded,
	}).Error)
	require.NoError(t, db.Create(&model.TaskBillingJob{
		TaskID: refunded.ID, IdempotencyKey: "dto-seedance-refunded", Operation: model.TaskBillingOperationRefund,
		Status: model.TaskBillingJobStatusSucceeded,
	}).Error)

	items := tasksToDto([]*model.Task{settled, refunded, pending}, false)
	require.Len(t, items, 3)
	require.NotNil(t, items[0].Billing)
	assert.Equal(t, "settled", items[0].Billing.State)
	assert.Equal(t, "seedance", items[0].Billing.Mode)
	assert.InDelta(t, 0.555, items[0].Billing.FinalCost, 1e-9)
	assert.True(t, items[0].Billing.DetailAvailable)
	require.NotNil(t, items[0].Billing.Seedance)
	assert.Equal(t, 10000, items[0].Billing.Seedance.ActualTokens)
	assert.Equal(t, "refunded", items[1].Billing.State)
	assert.Zero(t, items[1].Billing.FinalCost)
	assert.Equal(t, "pending", items[2].Billing.State)
	assert.Zero(t, items[2].Billing.FinalCost, "precharge must not be exposed as a final charge")
	assert.False(t, items[2].Billing.DetailAvailable)
	assert.Nil(t, items[2].Billing.Seedance)
}
