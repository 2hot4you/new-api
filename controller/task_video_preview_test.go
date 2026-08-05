package controller

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTasksToDtoExposesOnlySignedStarAIPlaybackURL(t *testing.T) {
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
	assert.Contains(t, items[0].ResultURL, "/v1/videos/task_preview_success/content?")
	assert.Contains(t, items[0].ResultURL, "signature=")
	assert.NotContains(t, items[0].ResultURL, "private.example")
	assert.Empty(t, items[1].ResultURL)
}

func TestTasksToDtoExposesOnlySignedMoliiGrokPlaybackURL(t *testing.T) {
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
	assert.Contains(t, items[0].ResultURL, "/v1/videos/task_grok_preview_success/content?")
	assert.NotContains(t, items[0].ResultURL, "private.example")
	assert.Nil(t, items[0].Data)
	require.NotNil(t, items[0].VideoParams)
	assert.Equal(t, "480p", items[0].VideoParams.Resolution)
	assert.Equal(t, "16:9", items[0].VideoParams.Ratio)
	assert.Equal(t, 5, items[0].VideoParams.Seconds)
}
