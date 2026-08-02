package relay

import (
	"encoding/json"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskModel2DtoUsesSignedPlaybackURLForSuccessfulStarAITask(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_public_result",
		UserId:   42,
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI)),
		Status:   model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://private.example/video.mp4?signature=upstream-secret",
		},
		Data: json.RawMessage(`{"data":{"result_url":"https://private.example/result.mp4","content":{"video_url":"https://private.example/content.mp4"}}}`),
	}

	result := TaskModel2Dto(task)
	parsed, err := url.Parse(result.ResultURL)
	require.NoError(t, err)
	assert.Equal(t, "/v1/videos/task_public_result/content", parsed.Path)
	assert.NotContains(t, result.ResultURL, "upstream-secret")
	assert.NotContains(t, string(result.Data), "private.example")
	assert.Contains(t, string(result.Data), "task_public_result")
	userID, err := service.VerifyVideoPlaybackSignature(task.TaskID, parsed.Query().Get("user_id"), parsed.Query().Get("expires"), parsed.Query().Get("signature"), time.Now())
	require.NoError(t, err)
	assert.Equal(t, task.UserId, userID)
}

func TestTaskModel2DtoPreservesNonStarAIResultURL(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_other",
		UserId:   42,
		Platform: constant.TaskPlatform("other"),
		Status:   model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://example.com/video.mp4",
		},
	}
	assert.Equal(t, task.PrivateData.ResultURL, TaskModel2Dto(task).ResultURL)
}
