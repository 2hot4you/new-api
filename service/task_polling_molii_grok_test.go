package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type privateGrokPollingAdaptor struct {
	statusCode      int
	body            []byte
	result          *relaycommon.TaskInfo
	adjustQuota     int
	finalizeBilling bool
}

func (a *privateGrokPollingAdaptor) Init(_ *relaycommon.RelayInfo) {}

func (a *privateGrokPollingAdaptor) FetchTask(_ string, _ string, _ map[string]any, _ string) (*http.Response, error) {
	return &http.Response{StatusCode: a.statusCode, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(a.body))}, nil
}

func (a *privateGrokPollingAdaptor) ParseTaskResult(_ []byte) (*relaycommon.TaskInfo, error) {
	if a.result == nil {
		return nil, errors.New("parse failed")
	}
	return a.result, nil
}

func (a *privateGrokPollingAdaptor) AdjustBillingOnComplete(task *model.Task, result *relaycommon.TaskInfo) int {
	if a.finalizeBilling && task != nil && task.PrivateData.BillingContext != nil && task.PrivateData.BillingContext.GrokVideoBilling != nil {
		billing := task.PrivateData.BillingContext.GrokVideoBilling
		billing.ActualDurationSeconds = result.ActualDurationSeconds
		billing.ActualResolution = "480p"
		billing.OutputCost = billing.OutputUnitPrice * result.ActualDurationSeconds
		billing.Subtotal = billing.OutputCost + billing.ImageInputCost + billing.VideoInputCost
	}
	return a.adjustQuota
}

func (a *privateGrokPollingAdaptor) IsPrivateTaskPolling() bool { return true }

func (a *privateGrokPollingAdaptor) AllowPerCallCompletionAdjustment() bool { return true }

func (a *privateGrokPollingAdaptor) IsTaskPollingStatusAccepted(statusCode int) bool {
	return statusCode == http.StatusOK || statusCode == http.StatusAccepted
}

func (a *privateGrokPollingAdaptor) SafePollingData(result *relaycommon.TaskInfo) []byte {
	body, _ := common.Marshal(map[string]any{"status": result.Status, "progress": result.Progress})
	return body
}

func (a *privateGrokPollingAdaptor) SafePollingError(statusCode int) error {
	return errors.New("Molii Grok Imagine API polling failed")
}

func TestPrivateGrokPollingAccepts202AndPersistsOnlySafeData(t *testing.T) {
	truncate(t)
	const (
		publicID   = "task_public_grok_pending"
		upstreamID = "upstream-private-request-id"
	)
	baseURL := "https://api.wxiai.com/xai"
	channel := &model.Channel{Id: 621, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "upstream-secret-key", BaseURL: &baseURL}
	task := &model.Task{
		TaskID:    publicID,
		Platform:  constant.TaskPlatform("62"),
		UserId:    1,
		ChannelId: channel.Id,
		Status:    model.TaskStatusInProgress,
		Progress:  "1%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: upstreamID,
		},
	}
	require.NoError(t, model.DB.Create(task).Error)
	adaptor := &privateGrokPollingAdaptor{
		statusCode: http.StatusAccepted,
		body:       []byte(`{"status":"pending","progress":25,"request_id":"upstream-private-request-id","authorization":"upstream-secret-key","url":"https://api.wxiai.com/xai"}`),
		result:     &relaycommon.TaskInfo{Status: model.TaskStatusInProgress, Progress: "25%"},
	}

	err := updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: task})
	require.NoError(t, err)
	assert.Equal(t, "25%", task.Progress)
	assert.NotContains(t, string(task.Data), upstreamID)
	assert.NotContains(t, string(task.Data), "upstream-secret-key")
	assert.NotContains(t, string(task.Data), "wxiai")
	assert.Contains(t, string(task.Data), "25%")
}

func TestPrivateGrokPollingRejectsUnexpectedHTTPStatusWithoutRawBody(t *testing.T) {
	channel := &model.Channel{Id: 622, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}
	task := &model.Task{TaskID: "task_public", ChannelId: channel.Id, Status: model.TaskStatusInProgress, PrivateData: model.TaskPrivateData{UpstreamTaskID: "upstream-secret"}}
	adaptor := &privateGrokPollingAdaptor{
		statusCode: http.StatusInternalServerError,
		body:       []byte(`{"error":"raw provider body","request_id":"upstream-secret"}`),
		result:     &relaycommon.TaskInfo{Status: model.TaskStatusFailure},
	}

	err := updateVideoSingleTask(context.Background(), adaptor, channel, "upstream-secret", map[string]*model.Task{"upstream-secret": task})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "raw provider body")
	assert.NotContains(t, err.Error(), "upstream-secret")
}

func TestMoliiGrokFailureRefundsExactlyOnceAcrossStalePolls(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	const userID, tokenID, channelID = 631, 632, 633
	const currentUserQuota, currentTokenQuota, taskQuota = 7000, 4000, 2500
	const upstreamID = "grok-upstream-failure-id"
	seedUser(t, userID, currentUserQuota)
	seedToken(t, tokenID, userID, "sk-grok-refund", currentTokenQuota)
	baseURL := "https://internal.invalid/xai"
	require.NoError(t, model.DB.Create(&model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret", BaseURL: &baseURL}).Error)
	task := makeTask(userID, channelID, taskQuota, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task_public_grok_failure"
	task.Platform = constant.TaskPlatform("62")
	task.PrivateData.UpstreamTaskID = upstreamID
	task.PrivateData.BillingContext.PerCallBilling = true
	require.NoError(t, model.DB.Create(task).Error)

	var firstPoll, stalePoll model.Task
	require.NoError(t, model.DB.First(&firstPoll, task.ID).Error)
	require.NoError(t, model.DB.First(&stalePoll, task.ID).Error)
	adaptor := &privateGrokPollingAdaptor{
		statusCode: http.StatusOK,
		body:       []byte(`{"status":"failed","request_id":"private"}`),
		result:     &relaycommon.TaskInfo{Status: model.TaskStatusFailure, Progress: "100%", Reason: "raw upstream failure"},
	}
	channel := &model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret", BaseURL: &baseURL}

	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: &firstPoll}))
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: &stalePoll}))
	job := loadTaskBillingJobByTaskID(t, task.ID)
	assert.Equal(t, model.TaskBillingJobStatusPending, job.Status)
	assert.Equal(t, currentUserQuota, getUserQuota(t, userID))
	assert.Equal(t, currentTokenQuota, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, taskQuota, getTaskQuota(t, task.ID))
	assert.Zero(t, countLogs(t))

	summary, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-refund-worker")
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Succeeded)
	assert.Equal(t, currentUserQuota+taskQuota, getUserQuota(t, userID))
	assert.Equal(t, currentTokenQuota+taskQuota, getTokenRemainQuota(t, tokenID))
	assert.Zero(t, getTaskQuota(t, task.ID))
	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, model.LogTypeRefund, getLastLog(t).Type)
}

func TestMoliiGrokSuccessDoesNotDoubleSettleAcrossStalePolls(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	const userID, tokenID, channelID = 641, 642, 643
	const currentUserQuota, currentTokenQuota, taskQuota = 7000, 4000, 2500
	const upstreamID = "grok-upstream-success-id"
	seedUser(t, userID, currentUserQuota)
	seedToken(t, tokenID, userID, "sk-grok-success", currentTokenQuota)
	baseURL := "https://internal.invalid/xai"
	require.NoError(t, model.DB.Create(&model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret", BaseURL: &baseURL}).Error)
	task := makeTask(userID, channelID, taskQuota, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task_public_grok_success"
	task.Platform = constant.TaskPlatform("62")
	task.PrivateData.UpstreamTaskID = upstreamID
	task.PrivateData.BillingContext.PerCallBilling = true
	require.NoError(t, model.DB.Create(task).Error)

	var firstPoll, stalePoll model.Task
	require.NoError(t, model.DB.First(&firstPoll, task.ID).Error)
	require.NoError(t, model.DB.First(&stalePoll, task.ID).Error)
	adaptor := &privateGrokPollingAdaptor{
		statusCode: http.StatusOK,
		body:       []byte(`{"status":"done","video":{"url":"https://video.example/result.mp4"}}`),
		result:     &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Progress: "100%", Url: "https://video.example/result.mp4"},
	}
	channel := &model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret", BaseURL: &baseURL}

	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: &firstPoll}))
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: &stalePoll}))
	assert.Equal(t, currentUserQuota, getUserQuota(t, userID))
	assert.Equal(t, currentTokenQuota, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, taskQuota, getTaskQuota(t, task.ID))
	job := loadTaskBillingJobByTaskID(t, task.ID)
	require.NotNil(t, job.TargetQuota)
	assert.Equal(t, taskQuota, *job.TargetQuota)
	assert.Zero(t, countLogs(t))

	summary, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-settle-worker")
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Succeeded)
	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, model.LogTypeConsume, getLastLog(t).Type)
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, reloaded.Status)
	assert.Equal(t, "https://video.example/result.mp4", reloaded.PrivateData.ResultURL)
}

func markFinalUsageGrokTask(task *model.Task) {
	task.Platform = constant.TaskPlatform("62")
	task.Properties.OriginModelName = "grok-imagine-video"
	task.PrivateData.BillingContext.OriginModelName = "grok-imagine-video"
	task.PrivateData.BillingContext.FinalUsageLogOnly = true
	task.PrivateData.BillingContext.RequestPath = "/v1/videos/generations"
	task.PrivateData.BillingContext.GrokVideoBilling = &model.GrokVideoBillingSnapshot{
		Version: 1, Model: "grok-imagine-video", Operation: "text_to_video", InputType: "text",
		RequestedDurationSeconds: 5, EstimatedDurationSeconds: 5,
		RequestedResolution: "480p", EstimatedResolution: "480p",
		OutputUnitPrice: 0.05, OutputCost: 0.25, Subtotal: 0.25, GroupRatio: 1,
	}
}

func TestMoliiGrokFinalUsageSuccessLogsExactlyOnceAcrossStalePolls(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	const userID, tokenID, channelID = 651, 652, 653
	const taskQuota = 125000
	const upstreamID = "grok-final-success-upstream"
	seedUser(t, userID, 9000)
	seedToken(t, tokenID, userID, "sk-grok-final-success", 5000)
	seedChannel(t, channelID)
	task := makeTask(userID, channelID, taskQuota, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task_public_grok_final_success"
	task.PrivateData.UpstreamTaskID = upstreamID
	task.PrivateData.BillingContext.PerCallBilling = true
	markFinalUsageGrokTask(task)
	require.NoError(t, model.DB.Create(task).Error)

	var firstPoll, stalePoll model.Task
	require.NoError(t, model.DB.First(&firstPoll, task.ID).Error)
	require.NoError(t, model.DB.First(&stalePoll, task.ID).Error)
	adaptor := &privateGrokPollingAdaptor{
		statusCode:  http.StatusOK,
		body:        []byte(`{"status":"done"}`),
		result:      &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Progress: "100%", ActualDurationSeconds: 5},
		adjustQuota: taskQuota, finalizeBilling: true,
	}
	channel := &model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}

	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: &firstPoll}))
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: &stalePoll}))
	job := loadTaskBillingJobByTaskID(t, task.ID)
	require.NotNil(t, job.TargetQuota)
	assert.Equal(t, taskQuota, *job.TargetQuota)
	assert.Zero(t, countLogs(t), "terminal persistence must not emit a pre-commit final log")

	firstRun, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-final-worker")
	require.NoError(t, err)
	assert.Equal(t, 1, firstRun.Succeeded)
	secondRun, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-final-worker")
	require.NoError(t, err)
	assert.Zero(t, secondRun.Claimed)
	assert.Equal(t, int64(1), countLogs(t))
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeConsume, log.Type)
	assert.Equal(t, taskQuota, log.Quota)
	assert.Contains(t, log.Other, `"grok_video_billing"`)
	assert.Contains(t, log.Other, `"request_path":"/v1/videos/generations"`)
	assert.NotContains(t, log.Other, upstreamID)

	var user model.User
	require.NoError(t, model.DB.First(&user, userID).Error)
	assert.Equal(t, taskQuota, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)
}

func TestMoliiGrokFinalUsageMissingCompletionFinalizationSuppressesSuccessLog(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	const userID, channelID = 655, 656
	const upstreamID = "grok-unfinalized-upstream"
	seedUser(t, userID, 10000)
	seedChannel(t, channelID)
	task := makeTask(userID, channelID, 2500, 0, BillingSourceWallet, 0)
	task.TaskID = "task_public_grok_unfinalized"
	task.PrivateData.UpstreamTaskID = upstreamID
	task.PrivateData.BillingContext.PerCallBilling = true
	markFinalUsageGrokTask(task)
	require.NoError(t, model.DB.Create(task).Error)

	adaptor := &privateGrokPollingAdaptor{statusCode: http.StatusOK, body: []byte(`{"status":"done"}`), result: &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Progress: "100%"}}
	channel := &model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: task}))

	job := loadTaskBillingJobByTaskID(t, task.ID)
	assert.Nil(t, job.TargetQuota)
	summary, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-review-worker")
	require.NoError(t, err)
	assert.Equal(t, 1, summary.ReviewRequired)
	assert.Equal(t, model.TaskBillingJobStatusReviewRequired, loadReconciliationJob(t, job.ID).Status)
	assert.Equal(t, int64(0), countLogs(t))
	assert.Equal(t, 2500, getTaskQuota(t, task.ID))
}

func TestMoliiGrokFinalUsageZeroSettlementRefundsInternallyThenLogsZeroConsume(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	const userID, tokenID, channelID = 661, 662, 663
	const initialQuota, initialTokenQuota, taskQuota = 10000, 4000, 2500
	const upstreamID = "grok-zero-upstream"
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "sk-grok-zero", initialTokenQuota)
	seedChannel(t, channelID)
	task := makeTask(userID, channelID, taskQuota, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task_public_grok_zero"
	task.PrivateData.UpstreamTaskID = upstreamID
	task.PrivateData.BillingContext.PerCallBilling = true
	markFinalUsageGrokTask(task)
	task.PrivateData.BillingContext.GrokVideoBilling.OutputUnitPrice = 0
	require.NoError(t, model.DB.Create(task).Error)

	adaptor := &privateGrokPollingAdaptor{
		statusCode: http.StatusOK, body: []byte(`{"status":"done"}`), adjustQuota: 0, finalizeBilling: true,
		result: &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Progress: "100%", ActualDurationSeconds: 5},
	}
	channel := &model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: task}))

	job := loadTaskBillingJobByTaskID(t, task.ID)
	require.NotNil(t, job.TargetQuota)
	assert.Zero(t, *job.TargetQuota)
	assert.Equal(t, initialQuota, getUserQuota(t, userID))
	assert.Zero(t, countLogs(t))
	summary, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-zero-worker")
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Succeeded)
	assert.Equal(t, initialQuota+taskQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenQuota+taskQuota, getTokenRemainQuota(t, tokenID))
	assert.Zero(t, getTaskQuota(t, task.ID))
	assert.Equal(t, int64(1), countLogs(t))
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeConsume, log.Type)
	assert.Zero(t, log.Quota)
	assert.Contains(t, log.Other, `"pre_consumed_quota":2500`)
}

func TestMoliiGrokFinalUsageFundingFailureSuppressesSuccessLog(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	const userID, channelID = 671, 673
	const upstreamID = "grok-funding-failure-upstream"
	seedUser(t, userID, 10000)
	seedChannel(t, channelID)
	task := makeTask(userID, channelID, 2500, 0, BillingSourceSubscription, 999999)
	task.TaskID = "task_public_grok_funding_failure"
	task.PrivateData.UpstreamTaskID = upstreamID
	task.PrivateData.BillingContext.PerCallBilling = true
	markFinalUsageGrokTask(task)
	require.NoError(t, model.DB.Create(task).Error)

	adaptor := &privateGrokPollingAdaptor{
		statusCode: http.StatusOK, body: []byte(`{"status":"done"}`), adjustQuota: 3000, finalizeBilling: true,
		result: &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Progress: "100%", ActualDurationSeconds: 6},
	}
	channel := &model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: task}))

	job := loadTaskBillingJobByTaskID(t, task.ID)
	require.NotNil(t, job.TargetQuota)
	assert.Equal(t, 3000, *job.TargetQuota)
	summary, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-funding-worker")
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Rescheduled)
	assert.Equal(t, int64(0), countLogs(t))
	assert.Equal(t, 2500, getTaskQuota(t, task.ID))
}

func TestMoliiGrokFinalUsageFailureWritesErrorWithoutRefundLog(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	const userID, tokenID, channelID = 681, 682, 683
	const upstreamID = "grok-final-failure-upstream"
	seedUser(t, userID, 10000)
	seedToken(t, tokenID, userID, "sk-grok-final-failure", 5000)
	seedChannel(t, channelID)
	task := makeTask(userID, channelID, 2500, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task_public_grok_final_failure"
	task.PrivateData.UpstreamTaskID = upstreamID
	markFinalUsageGrokTask(task)
	require.NoError(t, model.DB.Create(task).Error)

	adaptor := &privateGrokPollingAdaptor{statusCode: http.StatusOK, body: []byte(`{"status":"failed"}`), result: &relaycommon.TaskInfo{Status: model.TaskStatusFailure, Progress: "100%", Reason: "private raw failure"}}
	channel := &model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: task}))

	job := loadTaskBillingJobByTaskID(t, task.ID)
	assert.Equal(t, model.TaskBillingJobStatusPending, job.Status)
	assert.Zero(t, countLogs(t))
	summary, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-failure-worker")
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Succeeded)
	assert.Equal(t, int64(1), countLogs(t))
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, 2500, log.Quota)
	assert.NotContains(t, log.Content, "private raw failure")
	assert.NotContains(t, log.Other, upstreamID)
}

func TestMoliiGrokFinalUsageRefundFailureSuppressesTerminalErrorLog(t *testing.T) {
	tests := []struct {
		name          string
		billingSource string
		subscription  int
		failWallet    bool
	}{
		{name: "wallet", billingSource: BillingSourceWallet, failWallet: true},
		{name: "subscription", billingSource: BillingSourceSubscription, subscription: 999999},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTaskBillingReconciliationTest(t)
			userID := 690 + i*10
			channelID := userID + 1
			upstreamID := "grok-refund-failure-" + tt.name
			seedUser(t, userID, 10000)
			seedChannel(t, channelID)
			task := makeTask(userID, channelID, 2500, 0, tt.billingSource, tt.subscription)
			task.TaskID = "task_public_" + tt.name + "_refund_failure"
			task.PrivateData.UpstreamTaskID = upstreamID
			markFinalUsageGrokTask(task)
			require.NoError(t, model.DB.Create(task).Error)

			if tt.failWallet {
				callbackName := "test:fail_grok_wallet_refund"
				require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(db *gorm.DB) {
					if db.Statement.Table == "users" {
						db.AddError(errors.New("forced wallet refund failure"))
					}
				}))
				t.Cleanup(func() { model.DB.Callback().Update().Remove(callbackName) })
			}

			adaptor := &privateGrokPollingAdaptor{statusCode: http.StatusOK, body: []byte(`{"status":"failed"}`), result: &relaycommon.TaskInfo{Status: model.TaskStatusFailure, Progress: "100%", Reason: "private failure"}}
			channel := &model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}
			require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: task}))

			job := loadTaskBillingJobByTaskID(t, task.ID)
			summary, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-refund-failure-worker")
			require.NoError(t, err)
			assert.Equal(t, 1, summary.Rescheduled)
			assert.Equal(t, model.TaskBillingJobStatusPending, loadReconciliationJob(t, job.ID).Status)
			assert.Equal(t, int64(0), countLogs(t))
			assert.Equal(t, 2500, getTaskQuota(t, task.ID))
		})
	}
}

func TestMoliiGrokFinalUsageTimeoutRefundFailureSuppressesTerminalErrorLog(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	const userID, channelID = 715, 716
	seedUser(t, userID, 10000)
	seedChannel(t, channelID)
	task := makeTask(userID, channelID, 2500, 0, BillingSourceSubscription, 999999)
	task.TaskID = "task_public_timeout_refund_failure"
	task.Progress = "50%"
	task.SubmitTime = time.Now().Add(-10 * time.Minute).Unix()
	markFinalUsageGrokTask(task)
	require.NoError(t, model.DB.Create(task).Error)

	previousTimeout := constant.TaskTimeoutMinutes
	constant.TaskTimeoutMinutes = 1
	t.Cleanup(func() { constant.TaskTimeoutMinutes = previousTimeout })
	sweepTimedOutTasks(context.Background())

	job := loadTaskBillingJobByTaskID(t, task.ID)
	summary, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-timeout-worker")
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Rescheduled)
	assert.Equal(t, model.TaskBillingJobStatusPending, loadReconciliationJob(t, job.ID).Status)
	assert.Equal(t, int64(0), countLogs(t))
	assert.Equal(t, 2500, getTaskQuota(t, task.ID))
}

func TestMoliiGrokFinalUsageSuccessfulDeltasEmitOnlyFinalConsumption(t *testing.T) {
	tests := []struct {
		name          string
		preQuota      int
		actualQuota   int
		wantUserQuota int
		wantToken     int
	}{
		{name: "positive charge", preQuota: 2000, actualQuota: 3000, wantUserQuota: 9000, wantToken: 4000},
		{name: "partial refund", preQuota: 5000, actualQuota: 3000, wantUserQuota: 12000, wantToken: 7000},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTaskBillingReconciliationTest(t)
			userID := 720 + i*10
			tokenID := userID + 1
			channelID := userID + 2
			upstreamID := "grok-delta-" + tt.name
			seedUser(t, userID, 10000)
			seedToken(t, tokenID, userID, "sk-grok-delta", 5000)
			seedChannel(t, channelID)
			task := makeTask(userID, channelID, tt.preQuota, tokenID, BillingSourceWallet, 0)
			task.TaskID = "task_public_delta_" + tt.name
			task.PrivateData.UpstreamTaskID = upstreamID
			task.PrivateData.BillingContext.PerCallBilling = true
			markFinalUsageGrokTask(task)
			require.NoError(t, model.DB.Create(task).Error)

			adaptor := &privateGrokPollingAdaptor{
				statusCode: http.StatusOK, body: []byte(`{"status":"done"}`), adjustQuota: tt.actualQuota, finalizeBilling: true,
				result: &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Progress: "100%", ActualDurationSeconds: 5},
			}
			channel := &model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}
			require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, upstreamID, map[string]*model.Task{upstreamID: task}))

			job := loadTaskBillingJobByTaskID(t, task.ID)
			require.NotNil(t, job.TargetQuota)
			assert.Equal(t, tt.actualQuota, *job.TargetQuota)
			summary, err := RunTaskBillingReconciliationOnce(context.Background(), "grok-delta-worker")
			require.NoError(t, err)
			assert.Equal(t, 1, summary.Succeeded)
			assert.Equal(t, tt.wantUserQuota, getUserQuota(t, userID))
			assert.Equal(t, tt.wantToken, getTokenRemainQuota(t, tokenID))
			assert.Equal(t, tt.actualQuota, getTaskQuota(t, task.ID))
			assert.Equal(t, int64(1), countLogs(t))
			log := getLastLog(t)
			require.NotNil(t, log)
			assert.Equal(t, model.LogTypeConsume, log.Type)
			assert.Equal(t, tt.actualQuota, log.Quota)
			var user model.User
			require.NoError(t, model.DB.First(&user, userID).Error)
			assert.Equal(t, tt.actualQuota, user.UsedQuota)
			assert.Equal(t, 1, user.RequestCount)
		})
	}
}
