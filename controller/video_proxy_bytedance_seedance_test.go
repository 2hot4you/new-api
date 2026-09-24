package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func setupByteDanceSeedanceVideoProxy(t *testing.T, upstreamURL, legacyResult string) *model.Task {
	t.Helper()
	allowPrivateTaskMediaTest(t)
	previousDB, previousCache := model.DB, common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}, &model.TaskBillingJob{}))
	model.DB, common.MemoryCacheEnabled = db, false
	t.Cleanup(func() { model.DB, common.MemoryCacheEnabled = previousDB, previousCache })
	require.NoError(t, db.Create(&model.Channel{Id: 713, Type: constant.ChannelTypeByteDanceSeedance, Name: "seedance", Key: "instance-key", BaseURL: &upstreamURL}).Error)
	task := &model.Task{
		TaskID: "task_reseller_public", UserId: 712, ChannelId: 713,
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeByteDanceSeedance)),
		Status:   model.TaskStatusSuccess, Progress: "100%", Quota: 777,
		PrivateData: model.TaskPrivateData{UpstreamTaskID: "task_molii_public", ResultURL: legacyResult},
	}
	require.NoError(t, db.Create(task).Error)
	require.NoError(t, db.Create(&model.TaskBillingJob{TaskID: task.ID, IdempotencyKey: "seedance-content-test", FromQuota: 777, Operation: model.TaskBillingOperationSettle, Status: model.TaskBillingJobStatusSucceeded}).Error)
	return task
}

func serveByteDanceSeedanceVideoProxy(t *testing.T, task *model.Task, method string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, "/v1/videos/"+task.TaskID+"/content", nil)
	c.Request.Header = headers
	c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
	c.Set("id", task.UserId)
	VideoProxy(c)
	return recorder
}

func TestByteDanceSeedanceVideoProxyUsesChannelCredentialAndRange(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/videos/task_molii_public/content", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer instance-key", r.Header.Get("Authorization"))
		assert.Equal(t, "bytes=100-199", r.Header.Get("Range"))
		assert.Equal(t, `"etag"`, r.Header.Get("If-Range"))
		assert.Empty(t, r.Header.Get("If-None-Match"))
		assert.Empty(t, r.Header.Get("X-Leak"))
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 100-199/300")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("X-Upstream-Secret", "hidden")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, "video-bytes")
	}))
	defer upstream.Close()
	task := setupByteDanceSeedanceVideoProxy(t, upstream.URL, "https://expired.example/signed.mp4")
	recorder := serveByteDanceSeedanceVideoProxy(t, task, http.MethodGet, http.Header{
		"Authorization": []string{"Bearer end-user-key"}, "Range": []string{"bytes=100-199"},
		"If-Range": []string{`"etag"`}, "If-None-Match": []string{`"client-etag"`}, "X-Leak": []string{"secret"},
	})
	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "bytes 100-199/300", recorder.Header().Get("Content-Range"))
	assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	assert.Empty(t, recorder.Header().Get("X-Upstream-Secret"))
	assert.Equal(t, "video-bytes", recorder.Body.String())
}

func TestByteDanceSeedanceVideoProxyHeadUsesMoliiContentEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/videos/task_molii_public/content", r.URL.Path)
		assert.Equal(t, http.MethodHead, r.Method)
		assert.Equal(t, "Bearer instance-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", "300")
	}))
	defer upstream.Close()
	task := setupByteDanceSeedanceVideoProxy(t, upstream.URL, "")
	recorder := serveByteDanceSeedanceVideoProxy(t, task, http.MethodHead, http.Header{"Authorization": []string{"Bearer end-user-key"}})
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "300", recorder.Header().Get("Content-Length"))
	assert.Empty(t, recorder.Body.String())
}

func TestByteDanceSeedanceVideoProxyUpstreamFailureDoesNotMutateTaskOrBilling(t *testing.T) {
	for _, upstreamStatus := range []int{http.StatusBadGateway, http.StatusServiceUnavailable} {
		t.Run(strconv.Itoa(upstreamStatus), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(upstreamStatus)
				_, _ = io.WriteString(w, "private upstream secret")
			}))
			defer upstream.Close()
			task := setupByteDanceSeedanceVideoProxy(t, upstream.URL, "")
			recorder := serveByteDanceSeedanceVideoProxy(t, task, http.MethodGet, nil)
			assert.Equal(t, http.StatusBadGateway, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "artifact_upstream_error")
			assert.NotContains(t, recorder.Body.String(), "private upstream secret")
			var storedTask model.Task
			require.NoError(t, model.DB.First(&storedTask, task.ID).Error)
			assert.EqualValues(t, model.TaskStatusSuccess, storedTask.Status)
			assert.Equal(t, 777, storedTask.Quota)
			var job model.TaskBillingJob
			require.NoError(t, model.DB.Where("task_id = ?", task.ID).First(&job).Error)
			assert.Equal(t, model.TaskBillingJobStatusSucceeded, job.Status)
			assert.Equal(t, 777, job.FromQuota)
		})
	}
}
