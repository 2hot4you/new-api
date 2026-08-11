package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestVideoProxyStreamsOwnedGrokCOSResultWithRangeWithoutChannelLookup(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	fetched := false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
	})

	const userID = 801
	const publicTaskID = "task_public_grok_stored"
	require.NoError(t, db.Create(&model.Task{
		TaskID: publicTaskID,
		UserId: userID,
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{StoredResult: &model.TaskStoredResult{
			ObjectKey: "users/grok-results/801/video/2026/08/result.mp4",
			MIMEType:  "video/mp4",
			Size:      12,
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}},
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+publicTaskID+"/content", nil)
	c.Request.Header.Set("Range", "bytes=0-3")
	c.Request.Header.Set("If-Range", `"video-etag"`)
	c.Params = gin.Params{{Key: "task_id", Value: publicTaskID}}
	c.Set("id", userID)
	videoProxyWithStoredFetcher(c, func(_ context.Context, key, rangeHeader, ifRangeHeader string) (*http.Response, error) {
		fetched = true
		assert.Equal(t, "users/grok-results/801/video/2026/08/result.mp4", key)
		assert.Equal(t, "bytes=0-3", rangeHeader)
		assert.Equal(t, `"video-etag"`, ifRangeHeader)
		return &http.Response{
			StatusCode: http.StatusPartialContent,
			Header: http.Header{
				"Content-Type":     []string{"video/mp4"},
				"Content-Range":    []string{"bytes 0-3/12"},
				"Accept-Ranges":    []string{"bytes"},
				"ETag":             []string{`"video-etag"`},
				"X-Cos-Request-Id": []string{"must-not-leak"},
			},
			Body: io.NopCloser(strings.NewReader("vide")),
		}, nil
	})

	assert.True(t, fetched)
	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "vide", recorder.Body.String())
	assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "inline", recorder.Header().Get("Content-Disposition"))
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "bytes 0-3/12", recorder.Header().Get("Content-Range"))
	assert.Empty(t, recorder.Header().Get("X-Cos-Request-Id"))
}

func TestVideoProxyReturnsGoneForExpiredGrokStoredResultWithoutFetchingCOS(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	fetched := false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
	})

	const userID = 811
	const publicTaskID = "task_public_grok_expired"
	require.NoError(t, db.Create(&model.Task{
		TaskID: publicTaskID,
		UserId: userID,
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{StoredResult: &model.TaskStoredResult{
			ObjectKey: "users/grok-results/811/video/2026/08/result.mp4",
			MIMEType:  "video/mp4",
			ExpiresAt: time.Now().Add(-time.Second).Unix(),
		}},
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+publicTaskID+"/content", nil)
	c.Params = gin.Params{{Key: "task_id", Value: publicTaskID}}
	c.Set("id", userID)
	videoProxyWithStoredFetcher(c, func(context.Context, string, string, string) (*http.Response, error) {
		fetched = true
		return nil, nil
	})

	assert.False(t, fetched)
	assert.Equal(t, http.StatusGone, recorder.Code)
	assert.True(t, strings.Contains(recorder.Body.String(), "result_expired"))
}

func TestStoredGrokVideoRangeNotSatisfiableDoesNotExposeCOSBody(t *testing.T) {
	task := &model.Task{
		TaskID: "task_public_grok_range",
		UserId: 821,
		PrivateData: model.TaskPrivateData{StoredResult: &model.TaskStoredResult{
			ObjectKey: "users/grok-results/821/video/2026/08/result.mp4",
			MIMEType:  "video/mp4",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}},
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+task.TaskID+"/content", nil)

	serveStoredGrokVideo(c, task, func(context.Context, string, string, string) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusRequestedRangeNotSatisfiable,
			Header:     http.Header{"Content-Range": []string{"bytes */12"}},
			Body:       io.NopCloser(strings.NewReader("<Error>private COS details</Error>")),
		}, nil
	})

	assert.Equal(t, http.StatusRequestedRangeNotSatisfiable, recorder.Code)
	assert.Equal(t, "bytes */12", recorder.Header().Get("Content-Range"))
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	assert.Empty(t, recorder.Body.String())
}

func TestStoredGrokVideoRejectsCrossUserObjectKeyBeforeFetch(t *testing.T) {
	task := &model.Task{
		TaskID: "task_public_grok_wrong_owner",
		UserId: 831,
		PrivateData: model.TaskPrivateData{StoredResult: &model.TaskStoredResult{
			ObjectKey: "users/grok-results/999/video/2026/08/result.mp4",
			MIMEType:  "video/mp4",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}},
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+task.TaskID+"/content", nil)
	fetched := false

	serveStoredGrokVideo(c, task, func(context.Context, string, string, string) (*http.Response, error) {
		fetched = true
		return nil, nil
	})

	assert.False(t, fetched)
	assert.Equal(t, http.StatusBadGateway, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "999")
	assert.Contains(t, recorder.Body.String(), "stored_result_invalid")
}

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
