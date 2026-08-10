package controller

import (
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	task := &model.Task{
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
	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI))
	settled := &model.Task{
		TaskID: "task_seedance_settled", UserId: 42, Platform: platform,
		Status: model.TaskStatusSuccess, Quota: 277500,
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
			OriginModelName: "doubao-seedance-2-0-260128", GroupRatio: 1.5,
			ActualTokens: 10000, EstimatedResolution: "720p", EstimatedRatio: "16:9",
			EstimatedSeconds: 5, EstimatedUnitPrice: 37,
		}},
	}
	refunded := &model.Task{
		TaskID: "task_seedance_refunded", UserId: 42, Platform: platform,
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
