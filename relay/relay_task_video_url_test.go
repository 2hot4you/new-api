package relay

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTaskModel2DtoPassesThroughSignedTOSURLForSuccessfulStarAITask(t *testing.T) {
	resultURL := "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/result.mp4?X-Tos-Signature=upstream-signed"
	task := &model.Task{
		TaskID:   "task_public_result",
		UserId:   42,
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI)),
		Status:   model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: resultURL,
		},
	}

	result := TaskModel2Dto(task)
	assert.Equal(t, resultURL, result.ResultURL)
}

func TestTaskModel2DtoDoesNotExposeUnsignedStarAIResultURL(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_unsigned_result",
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI)),
		Status:   model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/result.mp4",
		},
	}

	assert.Empty(t, TaskModel2Dto(task).ResultURL)
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

func TestTaskModel2DtoUsesTrustedMoliiGrokURLAndIgnoresStoredResult(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_grok_expired",
		UserId:   42,
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMoliiGrokAIGC)),
		Status:   model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://files-cdn.x.ai/result.mp4?token=signed",
			StoredResult: &model.TaskStoredResult{
				ObjectKey: "users/grok-results/42/video/result.mp4",
				MIMEType:  "video/mp4",
				ExpiresAt: time.Now().Add(-time.Second).Unix(),
			},
		},
	}

	assert.Equal(t, "https://files-cdn.x.ai/result.mp4?token=signed", TaskModel2Dto(task).ResultURL)
	task.PrivateData.ResultURL = "https://other.example/result.mp4?token=do-not-leak"
	assert.Empty(t, TaskModel2Dto(task).ResultURL)
}

func TestVideoFetchByIDRejectsMoliiGrokPlatformChannelMismatchBeforeOutputOrFetch(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
	})

	var upstreamFetches atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamFetches.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(upstream.Close)
	baseURL := upstream.URL

	tests := []struct {
		name        string
		platform    constant.TaskPlatform
		channelType int
		baseURL     *string
	}{
		{
			name:        "grok platform with non-grok channel",
			platform:    constant.TaskPlatform("62"),
			channelType: constant.ChannelTypeGemini,
			baseURL:     &baseURL,
		},
		{
			name:        "non-grok platform with grok channel",
			platform:    constant.TaskPlatform("1"),
			channelType: constant.ChannelTypeMoliiGrokAIGC,
		},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channelID := 700 + index
			taskID := "task_query_routing_mismatch_" + strconv.Itoa(index)
			require.NoError(t, db.Create(&model.Channel{
				Id: channelID, Type: tt.channelType, Name: tt.name, Key: "secret", BaseURL: tt.baseURL,
			}).Error)
			require.NoError(t, db.Create(&model.Task{
				TaskID: taskID, UserId: 701, ChannelId: channelID,
				Platform: tt.platform, Status: model.TaskStatusSuccess,
				PrivateData: model.TaskPrivateData{
					UpstreamTaskID: "upstream-private-id",
					ResultURL:      "https://vidgen.x.ai/result.mp4?token=do-not-return",
				},
			}).Error)

			beforeFetches := upstreamFetches.Load()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+taskID, nil)
			c.Params = gin.Params{{Key: "task_id", Value: taskID}}
			c.Set("id", 701)

			respBody, taskResp := videoFetchByIDRespBodyBuilder(c)

			require.NotNil(t, taskResp)
			assert.Equal(t, http.StatusBadGateway, taskResp.StatusCode)
			assert.Equal(t, "invalid_task_routing", taskResp.Code)
			assert.Empty(t, respBody)
			assert.Equal(t, beforeFetches, upstreamFetches.Load())
			assert.NotContains(t, taskResp.Message, "vidgen.x.ai")
			assert.NotContains(t, taskResp.Message, "do-not-return")
		})
	}
}
