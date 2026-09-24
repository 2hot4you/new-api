package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestByteDanceSeedanceAdminDiagnosticsHideStarAIID(t *testing.T) {
	task := &model.Task{
		TaskID: "task_local", Platform: constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeByteDanceSeedance)),
		Status:      model.TaskStatusSuccess,
		Data:        json.RawMessage(`{"upstream_id":"cgt-private","data":{"api_key":"secret-canary"},"url":"https://private.invalid"}`),
		PrivateData: model.TaskPrivateData{UpstreamTaskID: "task_molii_public", ResultURL: "https://private.invalid?secret=canary"},
	}
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
		view := tasksToDto([]*model.Task{task}, false, role)[0]
		encoded, err := common.Marshal(view)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "cgt-private")
		assert.NotContains(t, string(encoded), "secret-canary")
		assert.NotContains(t, string(encoded), "private.invalid")
		require.NotNil(t, view.Billing)
		if role == common.RoleRootUser {
			require.NotNil(t, view.RootInfo)
			assert.Equal(t, "task_molii_public", view.RootInfo.UpstreamTaskID)
		} else {
			assert.NotContains(t, string(encoded), "task_molii_public")
		}
	}
	encoded, err := common.Marshal(relay.TaskModel2Dto(task))
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "cgt-private")
	assert.NotContains(t, string(encoded), "private.invalid")
}

func setupSeedancePollingBillingDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.TaskBillingJob{}, &model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}, &model.PerfMetric{}))
	previousDB, previousLog := model.DB, model.LOG_DB
	previousCache, previousRedis, previousBatch, previousConsume := common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	previousFactory := service.GetTaskAdaptorFunc
	previousLimit := constant.TaskQueryLimit
	constant.TaskQueryLimit = 10
	model.DB, model.LOG_DB = db, db
	common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, false, true
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor { return relay.GetTaskAdaptor(platform) }
	service.InitHttpClient()
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLog
		common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = previousCache, previousRedis, previousBatch, previousConsume
		service.GetTaskAdaptorFunc = previousFactory
		constant.TaskQueryLimit = previousLimit
		_ = sqlDB.Close()
	})
	return db
}

func TestByteDanceSeedanceBillingSubmissionDoesNotCountFinalUsage(t *testing.T) {
	db := setupSeedancePollingBillingDB(t)
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "reseller", Quota: 1000}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 1, Name: "reseller"}).Error)
	events := []string{}
	info := taskSubmissionRelayInfo(&taskSubmissionTestBilling{events: &events})
	info.PriceData.Quota, info.PriceData.ModelRatio, info.PriceData.GroupRatioInfo.GroupRatio = 100, 2, 0.5
	outcome, taskErr := executeTaskSubmissionWith(taskSubmissionTestContext(), info, func(*gin.Context, *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *dto.TaskError) {
		return &relay.TaskSubmitResult{Platform: constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeByteDanceSeedance)), UpstreamTaskID: "task_molii_public", Quota: 100, TaskData: []byte(`{"status":"queued"}`)}, nil
	})
	require.Nil(t, taskErr)
	require.NotNil(t, outcome)
	var user model.User
	require.NoError(t, db.First(&user, 1).Error)
	assert.Zero(t, user.UsedQuota)
	assert.Zero(t, user.RequestCount)
	var logs int64
	require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
	assert.Zero(t, logs, "reservation must not be logged as final consumption")
	assert.Equal(t, "task_molii_public", outcome.Task.PrivateData.UpstreamTaskID)
	assert.Equal(t, 2.0, outcome.Task.PrivateData.BillingContext.ModelRatio)
	assert.Nil(t, outcome.Task.PrivateData.Timing)
}

func TestByteDanceSeedanceBillingSubmissionCapturesExplicitFreeGroup(t *testing.T) {
	db := setupSeedancePollingBillingDB(t)
	events := []string{}
	info := taskSubmissionRelayInfo(&taskSubmissionTestBilling{events: &events})
	info.PriceData.ModelRatio = 2
	info.PriceData.GroupRatioInfo.GroupRatio = 0
	outcome, taskErr := executeTaskSubmissionWith(taskSubmissionTestContext(), info, func(*gin.Context, *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *dto.TaskError) {
		return &relay.TaskSubmitResult{Platform: constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeByteDanceSeedance)), UpstreamTaskID: "task_molii_public", TaskData: []byte(`{"status":"queued"}`)}, nil
	})
	require.Nil(t, taskErr)
	var stored model.Task
	require.NoError(t, db.First(&stored, outcome.Task.ID).Error)
	encoded, err := stored.PrivateData.Value()
	require.NoError(t, err)
	assert.Contains(t, encoded, `"group_ratio_captured":true`)
	stored.Status = model.TaskStatusSuccess
	job := service.BuildTerminalTaskBillingJob(context.Background(), relay.GetTaskAdaptor(stored.Platform), &stored, &relaycommon.TaskInfo{TotalTokens: 100})
	require.NotNil(t, job.TargetQuota)
	assert.Zero(t, *job.TargetQuota)
}

func TestByteDanceSeedancePollingBillingRestartRecovery(t *testing.T) {
	previousMaxFailures := constant.TaskPollMaxFailures
	constant.TaskPollMaxFailures = 1
	t.Cleanup(func() { constant.TaskPollMaxFailures = previousMaxFailures })
	for _, tc := range []struct {
		name, body            string
		wantQuota, wantWallet int
		review                bool
		snapshot              string
	}{
		{"success", `{"status":"SUCCESS","data":{"usage":{"total_tokens":40,"completion_tokens":30},"upstream_id":"cgt-private","cost":9999,"url":"https://private.invalid?secret=canary"}}`, 40, 960, false, ""},
		{"failure", `{"status":"FAILURE","fail_reason":"cgt-private secret-canary"}`, 0, 1000, false, ""},
		{"missing_usage", `{"status":"SUCCESS","cost":0,"data":{"upstream_id":"cgt-private"}}`, 100, 900, true, ""},
		{"fractional_usage", `{"status":"SUCCESS","usage":{"total_tokens":1.5}}`, 100, 900, true, ""},
		{"string_usage", `{"status":"SUCCESS","usage":{"total_tokens":"100"}}`, 100, 900, true, ""},
		{"overflow_usage", `{"status":"SUCCESS","usage":{"total_tokens":9223372036854775808}}`, 100, 900, true, ""},
		{"negative_usage", `{"status":"SUCCESS","usage":{"total_tokens":-10}}`, 100, 900, true, ""},
		{"quota_overflow", `{"status":"SUCCESS","usage":{"total_tokens":2147483648}}`, 100, 900, true, ""},
		{"absent_group_ratio", `{"status":"SUCCESS","usage":{"total_tokens":100}}`, 100, 900, true, `{"model_ratio":2}`},
		{"explicit_free_group", `{"status":"SUCCESS","usage":{"total_tokens":100}}`, 0, 1000, false, `{"model_ratio":2,"group_ratio_captured":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupSeedancePollingBillingDB(t)
			var requests atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				assert.Equal(t, "/v1/video/generations/task_molii_public", r.URL.Path)
				assert.Equal(t, "Bearer molii-key", r.Header.Get("Authorization"))
				_, _ = io.WriteString(w, tc.body)
			}))
			defer upstream.Close()
			require.NoError(t, db.Create(&model.User{Id: 1, Username: "reseller", Quota: 900}).Error)
			require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: "reseller-key", RemainQuota: 900, UsedQuota: 100}).Error)
			require.NoError(t, db.Create(&model.Channel{Id: 1, Type: constant.ChannelTypeByteDanceSeedance, BaseURL: &upstream.URL, Key: "molii-key", Status: common.ChannelStatusEnabled}).Error)
			task := &model.Task{
				TaskID: "task_reseller", UserId: 1, ChannelId: 1, Quota: 100, Group: "reseller",
				Platform: constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeByteDanceSeedance)), Status: model.TaskStatusInProgress,
				SubmitTime: time.Now().Unix(),
				PrivateData: model.TaskPrivateData{UpstreamTaskID: "task_molii_public", BillingSource: service.BillingSourceWallet, TokenId: 1,
					BillingContext: &model.TaskBillingContext{ModelRatio: 2, GroupRatio: 0.5, OriginModelName: "seedance-2-0"}},
			}
			if tc.snapshot != "" {
				task.PrivateData.BillingContext = &model.TaskBillingContext{}
				require.NoError(t, common.Unmarshal([]byte(tc.snapshot), task.PrivateData.BillingContext))
			}
			require.NoError(t, db.Create(task).Error)
			_, err := service.RunTaskPollingOnceWithError(context.Background(), nil)
			require.NoError(t, err)
			var job model.TaskBillingJob
			require.NoError(t, db.Where("task_id = ?", task.ID).First(&job).Error)
			if tc.review {
				assert.Nil(t, job.TargetQuota)
				assert.Equal(t, model.TaskBillingOperationSettle, job.Operation)
			}
			// An expired worker claim models a process dying after durable enqueue.
			claimed, err := model.ClaimTaskBillingJobs("crashed-worker", time.Now().Unix(), time.Now().Unix()+60, 1)
			require.NoError(t, err)
			require.Len(t, claimed, 1)
			require.NoError(t, db.Model(&model.TaskBillingJob{}).Where("id = ?", job.ID).Update("locked_until", time.Now().Unix()-1).Error)
			for restart := 0; restart < 2; restart++ {
				_, err = service.RunTaskPollingOnceWithError(context.Background(), nil)
				require.NoError(t, err)
				_, err = service.RunTaskBillingReconciliationOnce(context.Background(), fmt.Sprintf("restart-%d", restart))
				require.NoError(t, err)
			}
			var stored model.Task
			require.NoError(t, db.First(&stored, task.ID).Error)
			assert.Equal(t, tc.wantQuota, stored.Quota)
			if tc.name == "failure" {
				assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
			} else {
				assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), stored.Status)
			}
			assert.Equal(t, "task_molii_public", stored.PrivateData.UpstreamTaskID)
			assert.Nil(t, stored.PrivateData.Timing)
			assert.Nil(t, stored.PrivateData.StoredResult)
			assert.NotContains(t, string(stored.Data), "cgt-private")
			assert.NotContains(t, string(stored.Data), "private.invalid")
			assert.NotContains(t, stored.FailReason, "secret-canary")
			var user model.User
			require.NoError(t, db.First(&user, 1).Error)
			assert.Equal(t, tc.wantWallet, user.Quota)
			var token model.Token
			require.NoError(t, db.First(&token, 1).Error)
			assert.Equal(t, tc.wantWallet, token.RemainQuota)
			var jobs, logs int64
			require.NoError(t, db.Model(&model.TaskBillingJob{}).Count(&jobs).Error)
			require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
			assert.EqualValues(t, 1, jobs)
			assert.EqualValues(t, 1, requests.Load())
			require.NoError(t, db.First(&job, job.ID).Error)
			if tc.review {
				assert.Equal(t, model.TaskBillingJobStatusReviewRequired, job.Status)
				assert.Zero(t, logs)
			} else {
				assert.Equal(t, model.TaskBillingJobStatusSucceeded, job.Status)
				assert.EqualValues(t, 1, logs)
				if tc.name == "success" {
					assert.Equal(t, 40, user.UsedQuota)
					assert.Equal(t, 1, user.RequestCount)
				}
			}
		})
	}
}
