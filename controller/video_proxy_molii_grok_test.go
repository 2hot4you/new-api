package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

func TestVideoProxyRedirectsOwnedSuccessfulGrokResultWithoutServerFetch(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	fetched := false
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
	const publicTaskID = "task_public_grok_stored"
	require.NoError(t, db.Create(&model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID: publicTaskID, UserId: userID, ChannelId: channelID,
		Platform: constant.TaskPlatform("62"), Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://vidgen.x.ai/result.mp4?token=signed",
			StoredResult: &model.TaskStoredResult{
				ObjectKey: "users/grok-results/801/video/2026/08/legacy.mp4",
				MIMEType:  "video/mp4", ExpiresAt: time.Now().Add(time.Hour).Unix(),
			},
		},
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+publicTaskID+"/content", nil)
	c.Request.Header.Set("Range", "bytes=0-3")
	c.Request.Header.Set("If-Range", `"video-etag"`)
	c.Params = gin.Params{{Key: "task_id", Value: publicTaskID}}
	c.Set("id", userID)
	videoProxyWithStoredFetcher(c, func(context.Context, string, string, string) (*http.Response, error) {
		fetched = true
		return nil, errors.New("server fetch must not be called")
	})

	assert.False(t, fetched)
	assert.Equal(t, http.StatusTemporaryRedirect, recorder.Code)
	assert.Equal(t, "https://vidgen.x.ai/result.mp4?token=signed", recorder.Header().Get("Location"))
	assert.Empty(t, recorder.Body.String())
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "no-referrer", recorder.Header().Get("Referrer-Policy"))
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
}

func TestVideoProxyRejectsDifferentAuthenticatedUserBeforeAnyGrokResultFetch(t *testing.T) {
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

	var remoteFetches atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		remoteFetches.Add(1)
	}))
	t.Cleanup(remote.Close)

	const ownerUserID, differentUserID, channelID = 841, 842, 843
	const publicTaskID = "task_public_grok_owned_by_another_user"
	resultURL := remote.URL + "/result.mp4?signature=private-result-query"
	require.NoError(t, db.Create(&model.Channel{
		Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret",
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID: publicTaskID, UserId: ownerUserID, ChannelId: channelID,
		Platform: constant.TaskPlatform("62"), Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: resultURL,
			StoredResult: &model.TaskStoredResult{
				ObjectKey: "users/grok-results/841/video/2026/08/legacy.mp4",
				MIMEType:  "video/mp4",
				ExpiresAt: time.Now().Add(time.Hour).Unix(),
			},
		},
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+publicTaskID+"/content", nil)
	c.Params = gin.Params{{Key: "task_id", Value: publicTaskID}}
	c.Set("id", differentUserID)
	var storedFetches atomic.Int32
	videoProxyWithStoredFetcher(c, func(context.Context, string, string, string) (*http.Response, error) {
		storedFetches.Add(1)
		return nil, errors.New("stored fetch must not be called")
	})

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Empty(t, recorder.Header().Get("Location"))
	assert.Zero(t, storedFetches.Load())
	assert.Zero(t, remoteFetches.Load())
	assert.NotContains(t, recorder.Body.String(), remote.URL)
	assert.NotContains(t, recorder.Body.String(), "private-result-query")
}

func TestVideoProxyIgnoresLegacyGrokStoredResultWithoutDirectURL(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	fetched := false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
	})

	const userID, channelID = 811, 812
	const publicTaskID = "task_public_grok_expired"
	require.NoError(t, db.Create(&model.Channel{Id: channelID, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "secret"}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID: publicTaskID, UserId: userID, ChannelId: channelID,
		Platform: constant.TaskPlatform("62"), Status: model.TaskStatusSuccess,
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
	assert.Equal(t, http.StatusBadGateway, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "result_expired")
}

func TestVideoProxyKeepsNonGrokStoredResultBeforeChannelLookup(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
	})

	const userID = 813
	const publicTaskID = "task_public_non_grok_stored"
	require.NoError(t, db.Create(&model.Task{
		TaskID: publicTaskID, UserId: userID, ChannelId: 99999,
		Platform: constant.TaskPlatform("other"), Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{StoredResult: &model.TaskStoredResult{
			ObjectKey: "users/grok-results/813/video/2026/08/result.mp4",
			MIMEType:  "video/mp4", ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}},
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+publicTaskID+"/content", nil)
	c.Params = gin.Params{{Key: "task_id", Value: publicTaskID}}
	c.Set("id", userID)
	videoProxyWithStoredFetcher(c, func(_ context.Context, key, _, _ string) (*http.Response, error) {
		assert.Equal(t, "users/grok-results/813/video/2026/08/result.mp4", key)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"video/mp4"}},
			Body:       io.NopCloser(strings.NewReader("video")),
		}, nil
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "video", recorder.Body.String())
}

func TestVideoProxyRejectsGrokPlatformChannelMismatchWithoutFetch(t *testing.T) {
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

	tests := []struct {
		name        string
		platform    constant.TaskPlatform
		channelType int
		resultURL   string
	}{
		{
			name: "grok platform with non-grok channel", platform: constant.TaskPlatform("62"),
			channelType: constant.ChannelTypeOpenAI, resultURL: "data:video/mp4;base64,dmlkZW8=",
		},
		{
			name: "non-grok platform with grok channel", platform: constant.TaskPlatform("other"),
			channelType: constant.ChannelTypeMoliiGrokAIGC, resultURL: "https://vidgen.x.ai/result.mp4?token=signed",
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channelID := 820 + i
			publicTaskID := "task_platform_mismatch_" + strconv.Itoa(i)
			require.NoError(t, db.Create(&model.Channel{Id: channelID, Type: tt.channelType, Name: tt.name, Key: "secret"}).Error)
			require.NoError(t, db.Create(&model.Task{
				TaskID: publicTaskID, UserId: 814, ChannelId: channelID,
				Platform: tt.platform, Status: model.TaskStatusSuccess,
				PrivateData: model.TaskPrivateData{
					ResultURL: tt.resultURL,
					StoredResult: &model.TaskStoredResult{
						ObjectKey: "users/grok-results/814/video/2026/08/legacy.mp4",
						MIMEType:  "video/mp4",
						ExpiresAt: time.Now().Add(time.Hour).Unix(),
					},
				},
			}).Error)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+publicTaskID+"/content", nil)
			c.Params = gin.Params{{Key: "task_id", Value: publicTaskID}}
			c.Set("id", 814)
			fetched := false
			videoProxyWithStoredFetcher(c, func(context.Context, string, string, string) (*http.Response, error) {
				fetched = true
				return nil, errors.New("fetch must not be called")
			})

			assert.False(t, fetched)
			assert.Equal(t, http.StatusBadGateway, recorder.Code)
			assert.Empty(t, recorder.Header().Get("Location"))
			assert.NotContains(t, recorder.Body.String(), tt.resultURL)
		})
	}
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
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
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
		Platform:  constant.TaskPlatform("62"),
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
