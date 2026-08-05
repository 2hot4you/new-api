package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type privateGrokPollingAdaptor struct {
	statusCode int
	body       []byte
	result     *relaycommon.TaskInfo
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

func (a *privateGrokPollingAdaptor) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return 0
}

func (a *privateGrokPollingAdaptor) IsPrivateTaskPolling() bool { return true }

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
	truncate(t)
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
	assert.Equal(t, currentUserQuota+taskQuota, getUserQuota(t, userID))
	assert.Equal(t, currentTokenQuota+taskQuota, getTokenRemainQuota(t, tokenID))
	assert.Zero(t, getTaskQuota(t, task.ID))
}

func TestMoliiGrokSuccessDoesNotDoubleSettleAcrossStalePolls(t *testing.T) {
	truncate(t)
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
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, reloaded.Status)
	assert.Equal(t, "https://video.example/result.mp4", reloaded.PrivateData.ResultURL)
}
