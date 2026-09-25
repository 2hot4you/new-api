package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func resetVideoStudioIdempotencyTestState(t *testing.T) {
	t.Helper()
	previousRedis := common.RedisEnabled
	common.RedisEnabled = false
	videoStudioLocalRequests.Lock()
	videoStudioLocalRequests.expiresAt = make(map[string]time.Time)
	videoStudioLocalRequests.Unlock()
	t.Cleanup(func() {
		common.RedisEnabled = previousRedis
	})
}

func setupVideoStudioDatabaseTest(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedis
	})
}

func TestVideoStudioIdempotencyRejectsDuplicateSuccessfulRequest(t *testing.T) {
	resetVideoStudioIdempotencyTestState(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/tasks", func(c *gin.Context) { c.Set("id", 17); c.Next() }, VideoStudioIdempotency(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	first := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/tasks", nil)
	firstRequest.Header.Set(VideoStudioRequestIDHeader, "request-id-1234567890")
	router.ServeHTTP(first, firstRequest)
	assert.Equal(t, http.StatusOK, first.Code)

	second := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/tasks", nil)
	secondRequest.Header.Set(VideoStudioRequestIDHeader, "request-id-1234567890")
	router.ServeHTTP(second, secondRequest)
	assert.Equal(t, http.StatusConflict, second.Code)
}

func TestVideoStudioIdempotencyReleasesFailedRequest(t *testing.T) {
	resetVideoStudioIdempotencyTestState(t)
	gin.SetMode(gin.TestMode)
	attempts := 0
	router := gin.New()
	router.POST("/tasks", func(c *gin.Context) { c.Set("id", 23); c.Next() }, VideoStudioIdempotency(), func(c *gin.Context) {
		attempts++
		c.JSON(http.StatusBadGateway, gin.H{"success": false})
	})

	for range 2 {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/tasks", nil)
		request.Header.Set(VideoStudioRequestIDHeader, "request-id-abcdefghij")
		router.ServeHTTP(response, request)
		assert.Equal(t, http.StatusBadGateway, response.Code)
	}
	assert.Equal(t, 2, attempts)
}

func TestVideoStudioIdempotencyRejectsMissingRequestID(t *testing.T) {
	resetVideoStudioIdempotencyTestState(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/tasks", VideoStudioIdempotency(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/tasks", nil))
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestVideoStudioTokenAuthRejectsAnotherUsersToken(t *testing.T) {
	setupVideoStudioDatabaseTest(t)
	require.NoError(t, model.DB.Create(&model.Token{UserId: 42, Name: "private", Key: "private-key"}).Error)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/tasks", func(c *gin.Context) { c.Set("id", 7); c.Next() }, VideoStudioTokenAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/tasks", nil)
	request.Header.Set(VideoStudioTokenIDHeader, "1")
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestVideoStudioTokenAuthUsesOwnedTokenWithoutExposingSecret(t *testing.T) {
	setupDirectTokenGroupSelectionTest(t)
	gin.SetMode(gin.TestMode)

	user := &model.User{
		Username: "video-studio-user", Password: "password", Group: "default",
		Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Quota: 1000,
	}
	require.NoError(t, model.DB.Create(user).Error)
	token := &model.Token{
		UserId: user.Id, Key: "videostudiosecret", Name: "studio key",
		Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(token).Error)

	handled := 0
	router := gin.New()
	router.POST("/tasks", func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Next()
	}, VideoStudioTokenAuth(), func(c *gin.Context) {
		handled++
		assert.Equal(t, token.Id, c.GetInt("token_id"))
		assert.Equal(t, "Bearer sk-videostudiosecret", c.GetHeader("Authorization"))
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/tasks", nil)
	request.Header.Set(VideoStudioTokenIDHeader, strconv.Itoa(token.Id))
	request.Header.Set("Authorization", "Bearer dashboard-session")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, 1, handled)
	assert.Equal(t, "Bearer dashboard-session", request.Header.Get("Authorization"))
}

func TestPrepareVideoStudioRequestCapturesPublicMediaWithoutSecrets(t *testing.T) {
	previousRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedis })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/tasks", func(c *gin.Context) { c.Set("id", 7); c.Next() }, PrepareVideoStudioRequest(), func(c *gin.Context) {
		snapshot := service.VideoStudioRequestSnapshotFromContext(c)
		require.NotNil(t, snapshot)
		c.JSON(http.StatusOK, snapshot)
	})
	body := []byte(`{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"test prompt"},{"type":"image_url","role":"reference_image","image_url":{"url":"https://cdn.example/reference.png"}}],"resolution":"720p","ratio":"16:9","duration":6,"generate_audio":true,"watermark":false}`)
	request := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(VideoStudioRequestIDHeader, "request-id-snapshot-123")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"prompt":"test prompt"`)
	assert.Contains(t, response.Body.String(), `"source":"url"`)
	assert.NotContains(t, response.Body.String(), "Authorization")
}

func TestPrepareVideoStudioRequestRejectsNonSeedanceModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/tasks", PrepareVideoStudioRequest(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	body := []byte(`{"model":"grok-imagine-video","content":[{"type":"text","text":"test prompt"}],"resolution":"720p","ratio":"16:9","duration":6}`)
	request := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "Seedance")
}

func TestCaptureVideoStudioRequestSnapshotRecordsPublicSeedanceRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/videos", func(c *gin.Context) {
		c.Set("id", 7)
		c.Next()
	}, CaptureVideoStudioRequestSnapshot(), func(c *gin.Context) {
		snapshot := service.VideoStudioRequestSnapshotFromContext(c)
		require.NotNil(t, snapshot)
		assert.Equal(t, "doubao-seedance-2-5-260628", snapshot.Model)
		assert.Equal(t, "public API prompt", snapshot.Prompt)
		assert.Equal(t, "720p", snapshot.Resolution)
		assert.Equal(t, "9:16", snapshot.Ratio)
		assert.Equal(t, 15, snapshot.Duration)
		assert.True(t, snapshot.GenerateAudio)
		assert.False(t, snapshot.Watermark)
		c.Status(http.StatusNoContent)
	})

	body := `{"model":"doubao-seedance-2-5-260628","content":[{"type":"text","text":"public API prompt"}],"resolution":"720p","ratio":"9:16","duration":15,"generate_audio":true,"watermark":false}`
	request := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestCaptureVideoStudioRequestSnapshotDoesNotChangeNonSeedanceRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/videos", CaptureVideoStudioRequestSnapshot(), func(c *gin.Context) {
		assert.Nil(t, service.VideoStudioRequestSnapshotFromContext(c))
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"other-video-model"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}
