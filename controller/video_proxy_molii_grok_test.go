package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestVideoProxyRejectsNonHTTPSMoliiGrokResult(t *testing.T) {
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

	const userID, channelID = 801, 802
	const publicTaskID = "task_public_grok_http"
	require.NoError(t, db.Create(&model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    publicTaskID,
		UserId:    userID,
		ChannelId: channelID,
		Status:    model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "upstream-private-id",
			ResultURL:      "http://videos.example/result.mp4",
		},
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+publicTaskID+"/content", nil)
	c.Params = gin.Params{{Key: "task_id", Value: publicTaskID}}
	c.Set("id", userID)
	VideoProxy(c)

	assert.Equal(t, http.StatusBadGateway, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "videos.example")
	assert.NotContains(t, recorder.Body.String(), "upstream-private-id")
	assert.Contains(t, recorder.Body.String(), "HTTPS")
}

func TestMoliiGrokVideoResponseHeadersForceMP4(t *testing.T) {
	header := make(http.Header)
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Disposition", "attachment")
	applyMoliiGrokVideoResponseHeaders(header)
	assert.Equal(t, "video/mp4", header.Get("Content-Type"))
	assert.Equal(t, "inline", header.Get("Content-Disposition"))
}

func TestMoliiGrokVideoResponseHeaderAllowlist(t *testing.T) {
	for _, header := range []string{"Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		assert.True(t, isMoliiGrokVideoResponseHeaderAllowed(header), header)
	}
	for _, header := range []string{"Server", "Via", "X-Powered-By", "Request-ID", "X-Request-ID"} {
		assert.False(t, isMoliiGrokVideoResponseHeaderAllowed(header), header)
	}
}
