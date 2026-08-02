package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestVideoProxyRejectsUnsignedStarAIPrivateTOSURL(t *testing.T) {
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

	const (
		userID       = 712
		channelID    = 713
		publicTaskID = "task_public_starai_unsigned_tos"
	)
	require.NoError(t, db.Create(&model.Channel{
		Id:   channelID,
		Type: constant.ChannelTypeStarAI,
		Name: "starai_test",
		Key:  "must-not-be-forwarded",
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    publicTaskID,
		UserId:    userID,
		ChannelId: channelID,
		Quota:     777,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "private-upstream-task-id",
			ResultURL:      "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/private/result.mp4",
		},
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+publicTaskID+"/content", nil)
	ctx.Params = gin.Params{{Key: "task_id", Value: publicTaskID}}
	ctx.Set("id", userID)

	VideoProxy(ctx)

	assert.Equal(t, http.StatusBadGateway, recorder.Code)
	var response struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "upstream_invalid_result_url", response.Error.Code)
	assert.Equal(t, "StarAI returned an unsigned private TOS result URL", response.Error.Message)
	assert.Equal(t, "server_error", response.Error.Type)

	var unchanged model.Task
	require.NoError(t, db.Where("task_id = ?", publicTaskID).First(&unchanged).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, unchanged.Status)
	assert.Equal(t, 777, unchanged.Quota)
}

func TestVideoProxyAllowsSignedStarAIPrivateTOSURLWithoutBearer(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	fetchSetting := system_setting.GetFetchSetting()
	previousSSRFProtection := fetchSetting.EnableSSRFProtection
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	fetchSetting.EnableSSRFProtection = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		fetchSetting.EnableSSRFProtection = previousSSRFProtection
	})

	var reachedProxy atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedProxy.Store(true)
		assert.Equal(t, "ark-acg-cn-beijing.tos-cn-beijing.volces.com", r.URL.Hostname())
		assert.Empty(t, r.Header.Get("Authorization"))
		assert.Equal(t, "bytes=0-4", r.Header.Get("Range"))
		assert.Equal(t, "signed-secret", r.URL.Query().Get("X-Tos-Signature"))
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Disposition", "attachment")
		w.Header().Set("Content-Range", "bytes 0-4/12")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, "stara")
	}))
	t.Cleanup(proxy.Close)

	const (
		userID       = 722
		channelID    = 723
		publicTaskID = "task_public_starai_signed_tos"
	)
	channel := &model.Channel{
		Id:   channelID,
		Type: constant.ChannelTypeStarAI,
		Name: "starai_signed_test",
		Key:  "must-not-be-forwarded",
	}
	channel.SetSetting(dto.ChannelSettings{Proxy: proxy.URL})
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    publicTaskID,
		UserId:    userID,
		ChannelId: channelID,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "private-upstream-task-id",
			ResultURL:      "http://ark-acg-cn-beijing.tos-cn-beijing.volces.com/private/result.mp4?X-Tos-Signature=signed-secret",
		},
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+publicTaskID+"/content", nil)
	ctx.Request.Header.Set("Range", "bytes=0-4")
	ctx.Params = gin.Params{{Key: "task_id", Value: publicTaskID}}
	ctx.Set("id", userID)

	VideoProxy(ctx)

	assert.True(t, reachedProxy.Load())
	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "bytes 0-4/12", recorder.Header().Get("Content-Range"))
	assert.Equal(t, "inline", recorder.Header().Get("Content-Disposition"))
	assert.Equal(t, "stara", recorder.Body.String())
}
