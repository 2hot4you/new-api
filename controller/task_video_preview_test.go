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
