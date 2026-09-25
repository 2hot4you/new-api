package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	publicdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// This acceptance test deliberately has no SQLite/miniredis fallback. Use the
// existing loopback PostgreSQL database harness and a disposable Redis server:
// TEST_POSTGRES_DSN=... TEST_SEEDANCE_REDIS_URL=redis://127.0.0.1:PORT/0 go test ./controller -run TestSeedanceResellerFullChain -count=1 -v
func setupSeedanceResellerIntegration(t *testing.T) *gorm.DB {
	t.Helper()
	dsn, redisURL := os.Getenv("TEST_POSTGRES_DSN"), os.Getenv("TEST_SEEDANCE_REDIS_URL")
	if dsn == "" || redisURL == "" {
		t.Skip("requires TEST_POSTGRES_DSN and TEST_SEEDANCE_REDIS_URL (isolated PostgreSQL and disposable Redis; no SQLite acceptance)")
	}
	options, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	host, _, err := net.SplitHostPort(options.Addr)
	require.NoError(t, err)
	require.True(t, net.ParseIP(host).IsLoopback(), "Redis acceptance tests only permit loopback instances")
	client := redis.NewClient(options)
	require.NoError(t, client.Ping(context.Background()).Err())
	t.Cleanup(func() { _ = client.Close() })
	db, _ := newAuditTestDatabase(t, "postgres", dsn)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Task{}, &model.TaskBillingJob{}, &model.Log{}, &model.PerfMetric{}))
	previousDB, previousLog := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousRDB := common.RedisEnabled, common.RDB
	previousCache, previousBatch, previousConsume := common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	previousFactory, previousLimit := service.GetTaskAdaptorFunc, constant.TaskQueryLimit
	previousRatios, previousGroups := ratio_setting.ModelRatio2JSONString(), ratio_setting.GroupRatio2JSONString()
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypePostgreSQL)
	// Initialize dialect-specific quoted columns just as application startup
	// does. Merely replacing model.DB leaves token lookup columns uninitialized.
	t.Setenv("LOG_SQL_DSN", "")
	previousMaster := common.IsMasterNode
	common.IsMasterNode = false
	require.NoError(t, model.InitLogDB())
	common.IsMasterNode = previousMaster
	common.RedisEnabled, common.RDB = true, client
	common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, true
	constant.TaskQueryLimit = 10
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor { return relay.GetTaskAdaptor(platform) }
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"doubao-seedance-2-0-260128":2}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":0.5}`))
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)
	allowPrivateTaskMediaTest(t) // Only the nested loopback HTTP fixtures need this override.
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLog
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.IsMasterNode = false
		require.NoError(t, model.InitLogDB())
		common.SetDatabaseTypes(previousMainType, previousLogType)
		model.LOG_DB = previousLog
		common.IsMasterNode = previousMaster
		common.RedisEnabled, common.RDB = previousRedis, previousRDB
		common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = previousCache, previousBatch, previousConsume
		service.GetTaskAdaptorFunc, constant.TaskQueryLimit = previousFactory, previousLimit
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousRatios))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroups))
	})
	return db
}

func TestSeedanceResellerAuthFailureRestartRetainsReservation(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			db := setupSeedanceResellerIntegration(t)
			oldMax, oldTimeout := constant.TaskPollMaxFailures, constant.TaskTimeoutMinutes
			constant.TaskPollMaxFailures, constant.TaskTimeoutMinutes = 1, 1
			t.Cleanup(func() { constant.TaskPollMaxFailures, constant.TaskTimeoutMinutes = oldMax, oldTimeout })
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"error":{"message":"private-key cgt-private"}}`)
			}))
			defer upstream.Close()
			ch := seedanceResellerChannel(upstream.URL, "fixture-reseller-key")
			require.NoError(t, db.Create(ch).Error)
			task := &model.Task{TaskID: "task_auth_restart", ChannelId: ch.Id, UserId: 1, Platform: "64", Status: model.TaskStatusInProgress, Progress: "50%", Quota: 777, SubmitTime: time.Now().Unix(), PrivateData: model.TaskPrivateData{UpstreamTaskID: "task_molii_auth", Key: "fixture-reseller-key"}}
			require.NoError(t, db.Create(task).Error)
			for attempt := 0; attempt < 3; attempt++ {
				_, err := service.RunTaskPollingOnceWithError(context.Background(), nil)
				require.NoError(t, err)
				var stored model.Task
				require.NoError(t, model.DB.First(&stored, task.ID).Error)
				require.EqualValues(t, model.TaskStatusInProgress, stored.Status)
				require.Equal(t, 777, stored.Quota)
				require.Equal(t, attempt+1, stored.PrivateData.PollFailures)
				encoded, encodeErr := json.Marshal(stored.PrivateData)
				require.NoError(t, encodeErr)
				require.Contains(t, string(encoded), `"poll_failure_class":"auth"`)
				require.Empty(t, stored.FailReason)
				require.Zero(t, stored.FinishTime)
				require.NotContains(t, string(stored.Data), "private")
				var jobs int64
				require.NoError(t, model.DB.Model(&model.TaskBillingJob{}).Count(&jobs).Error)
				require.Zero(t, jobs, "auth uncertainty must not enqueue a refund")
				// Restart with new DB connections and fresh adaptor instances, then run
				// beyond the ordinary timeout cutoff. Only persisted facts survive.
				require.NoError(t, model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Update("submit_time", time.Now().Add(-2*time.Minute).Unix()).Error)
				reopened, err := gorm.Open(db.Dialector, &gorm.Config{})
				require.NoError(t, err)
				model.DB, model.LOG_DB = reopened, reopened
				t.Cleanup(func() { sqlDB, _ := reopened.DB(); _ = sqlDB.Close() })
			}
		})
	}
}

// Catches removal of local asset ownership checks, changed credential/task-ID
// routing, forwarding raw provider diagnostics, or duplicate terminal charging.
// Only the two remote HTTP services are fixtures; local auth, assets, submit,
// polling, durable billing and content proxy all use production implementations.
func TestSeedanceResellerFullChain(t *testing.T) {
	db := setupSeedanceResellerIntegration(t)
	const (
		moliiKey   = "fixture-molii-instance-private-key"
		starAIKey  = "fixture-starai-private-key"
		starAITask = "cgt-fixture-private-task"
		moliiTask  = "task_molii_public_fixture"
		localTask  = "task_reseller_public_fixture"
		modelName  = "doubao-seedance-2-0-260128"
	)
	assetID := fmt.Sprintf("asset-fixture-%d", time.Now().UnixNano())
	expiresAt := time.Now().Add(time.Hour).Unix()
	var assetQueries, submits, polls, downloads atomic.Int32
	starAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, "Bearer "+starAIKey, r.Header.Get("Authorization")) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/assets":
			var input map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&input))
			assert.Equal(t, "https://media.example/reference.png", input["url"])
			assert.Equal(t, "image", input["asset_type"])
			_ = json.NewEncoder(w).Encode(gin.H{"id": assetID, "status": "PROCESSING"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/assets/"+assetID:
			assetQueries.Add(1)
			_ = json.NewEncoder(w).Encode(gin.H{"id": assetID, "status": "ACTIVE", "asset_type": "image", "expires_at": expiresAt, "api_key": starAIKey})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/video/generations":
			submits.Add(1)
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			var payload struct {
				Model   string `json:"model"`
				Content []struct {
					Type     string `json:"type"`
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			}
			assert.NoError(t, json.Unmarshal(body, &payload))
			assert.Equal(t, modelName, payload.Model)
			var imageURI string
			for _, item := range payload.Content {
				if item.Type == "image_url" {
					imageURI = item.ImageURL.URL
				}
			}
			assert.Equal(t, "asset://"+assetID, imageURI)
			assert.NotContains(t, string(body), moliiKey)
			_ = json.NewEncoder(w).Encode(gin.H{"id": starAITask, "status": "queued"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/video/generations/"+starAITask:
			polls.Add(1)
			_ = json.NewEncoder(w).Encode(gin.H{"code": "success", "data": gin.H{"status": "SUCCESS", "progress": "100%", "usage": gin.H{"total_tokens": 40, "completion_tokens": 30}, "upstream_id": starAITask, "api_key": starAIKey, "url": "https://private.invalid/result?token=" + starAIKey}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/"+starAITask+"/content":
			downloads.Add(1)
			assert.Equal(t, "bytes=2-5", r.Header.Get("Range"))
			assert.Equal(t, `"fixture-etag"`, r.Header.Get("If-Range"))
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Content-Range", "bytes 2-5/10")
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("X-Private-Key", starAIKey)
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, "2345")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(starAI.Close)
	// Molii maps its public task ID to the provider ID and owns the StarAI key.
	// Extra private fields intentionally model an untrusted upstream response.
	molii := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, "Bearer "+moliiKey, r.Header.Get("Authorization")) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		path := strings.ReplaceAll(r.URL.Path, moliiTask, starAITask)
		request, err := http.NewRequestWithContext(r.Context(), r.Method, starAI.URL+path, r.Body)
		if !assert.NoError(t, err) {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		request.Header.Set("Authorization", "Bearer "+starAIKey)
		request.Header.Set("Range", r.Header.Get("Range"))
		request.Header.Set("If-Range", r.Header.Get("If-Range"))
		response, err := starAI.Client().Do(request)
		if !assert.NoError(t, err) {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		if r.URL.Path == "/v1/video/generations" && r.Method == http.MethodPost {
			var accepted map[string]any
			assert.NoError(t, json.NewDecoder(response.Body).Decode(&accepted))
			assert.Equal(t, starAITask, accepted["id"])
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(gin.H{"id": moliiTask, "status": "queued", "upstream_id": starAITask, "api_key": moliiKey})
			return
		}
		if r.URL.Path == "/v1/video/generations/"+moliiTask && r.Method == http.MethodGet {
			// Exercise the actual legacy public projection between reseller hops,
			// not just a fixture that hands the downstream trustworthy usage.
			body, readErr := io.ReadAll(response.Body)
			if !assert.NoError(t, readErr) {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			platform := constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeStarAI))
			adaptor := relay.GetTaskAdaptor(platform)
			facts, parseErr := adaptor.ParseTaskResult(nil, response, body)
			if !assert.NoError(t, parseErr) {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			safe := service.SanitizeStarAIResponseBody(body, moliiTask)
			publicTask := relay.TaskModel2Dto(&model.Task{
				TaskID: moliiTask, Platform: platform, Status: model.TaskStatus(facts.Status),
				SubmitTime: 1700000000, StartTime: 1700000002, FinishTime: 1700000008,
				Data: safe, FailReason: "fixture-private-provider-diagnostic",
				PrivateData: model.TaskPrivateData{Key: starAIKey, UpstreamTaskID: starAITask, ResultURL: "https://private.invalid/result"},
			})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(publicdto.TaskResponse[any]{Code: publicdto.TaskSuccessCode, Data: publicTask})
			return
		}
		for name, values := range response.Header {
			w.Header()[name] = values
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	t.Cleanup(molii.Close)

	user := &model.User{Username: "seedance-reseller-owner", AffCode: "owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", Quota: 2_000_000}
	otherUser := &model.User{Username: "seedance-reseller-other", AffCode: "other", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", Quota: 2_000_000}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(otherUser).Error)
	token := &model.Token{UserId: user.Id, Key: fmt.Sprintf("fixtureowner%d", time.Now().UnixNano()), Status: common.TokenStatusEnabled, RemainQuota: 2_000_000, ExpiredTime: -1, Group: "default"}
	otherToken := &model.Token{UserId: otherUser.Id, Key: fmt.Sprintf("fixtureother%d", time.Now().UnixNano()), Status: common.TokenStatusEnabled, RemainQuota: 2_000_000, ExpiredTime: -1, Group: "default"}
	require.NoError(t, db.Create(token).Error)
	require.NoError(t, db.Create(otherToken).Error)
	channel := &model.Channel{Type: constant.ChannelTypeByteDanceSeedance, Name: "reseller-full-chain", Status: common.ChannelStatusEnabled, Key: moliiKey, BaseURL: &molii.URL, Models: modelName, Group: "default"}
	require.NoError(t, db.Create(channel).Error)

	router := gin.New()
	router.POST("/v1/assets", middleware.TokenAuth(), CreateStarAIAsset)
	router.GET("/v1/assets/:id", middleware.TokenAuthReadOnly(), GetStarAIAsset)
	router.DELETE("/v1/assets/:id", middleware.TokenAuth(), DeleteStarAIAsset)
	router.POST("/v1/video/generations", middleware.TokenAuth(), func(c *gin.Context) {
		if setupErr := middleware.SetupContextForSelectedChannel(c, channel, modelName); setupErr != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		info := &relaycommon.RelayInfo{
			UserId: c.GetInt("id"), TokenId: c.GetInt("token_id"), TokenKey: c.GetString("token_key"),
			UserGroup: "default", UsingGroup: "default", TokenGroup: "default", UserQuota: 2_000_000,
			OriginModelName: modelName, StartTime: time.Now(),
			UserSetting:   dto.UserSetting{BillingPreference: "wallet_only"},
			TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: localTask, LockedChannel: channel},
		}
		outcome, taskErr := executeTaskSubmissionWith(c, info, relay.RelayTaskSubmit)
		if taskErr != nil {
			respondTaskSubmissionError(c, taskErr)
			return
		}
		presentTaskSubmission(c, outcome)
	})
	router.GET("/v1/video/generations/:task_id", middleware.TokenAuthReadOnly(), func(c *gin.Context) {
		if taskErr := relay.RelayTaskFetch(c, relayconstant.RelayModeVideoFetchByID); taskErr != nil {
			respondTaskError(c, taskErr)
		}
	})
	router.GET("/v1/videos/:task_id", middleware.TokenAuthReadOnly(), func(c *gin.Context) {
		if taskErr := relay.RelayTaskFetch(c, relayconstant.RelayModeVideoFetchByID); taskErr != nil {
			respondTaskError(c, taskErr)
		}
	})
	router.GET("/v1/videos/:task_id/content", middleware.VideoProxyAuth(), VideoProxy)
	request := func(method, path, body, key string, headers http.Header) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		for name, values := range headers {
			req.Header[name] = values
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		for _, private := range []string{moliiKey, starAIKey, starAITask, moliiTask, token.Key, otherToken.Key, "private.invalid", "fixture-private-provider-diagnostic"} {
			assert.NotContains(t, recorder.Body.String(), private, "%s %s body", method, path)
			assert.NotContains(t, fmt.Sprint(recorder.Header()), private, "%s %s headers", method, path)
		}
		return recorder
	}
	created := request(http.MethodPost, "/v1/assets", `{"url":"https://media.example/reference.png","asset_type":"image","name":"reference"}`, token.Key, nil)
	require.Equal(t, http.StatusOK, created.Code, created.Body.String())
	assert.JSONEq(t, fmt.Sprintf(`{"id":%q}`, assetID), created.Body.String())
	t.Cleanup(func() { _ = service.DeleteStarAIAssetBinding(assetID, user.Id) })
	queried := request(http.MethodGet, "/v1/assets/"+assetID, "", token.Key, nil)
	require.Equal(t, http.StatusOK, queried.Code, queried.Body.String())
	assert.Contains(t, queried.Body.String(), `"status":"ACTIVE"`)
	binding, err := service.GetStarAIAssetBinding(assetID, user.Id)
	require.NoError(t, err)
	assert.Equal(t, assetID, binding.UpstreamID)
	assert.Equal(t, expiresAt, binding.ExpiresAt)
	assert.Equal(t, constant.ChannelTypeByteDanceSeedance, binding.ChannelType)
	beforeDeniedQuery := assetQueries.Load()
	deniedAsset := request(http.MethodGet, "/v1/assets/"+assetID, "", otherToken.Key, nil)
	require.Equal(t, http.StatusNotFound, deniedAsset.Code)
	assert.Equal(t, beforeDeniedQuery, assetQueries.Load(), "ownership rejection must happen before upstream access")

	submitted := request(http.MethodPost, "/v1/video/generations", fmt.Sprintf(`{"model":%q,"content":[{"type":"text","text":"a lighthouse"},{"type":"image_url","image_url":{"url":"asset://%s"},"role":"reference_image"}],"duration":5}`, modelName, assetID), token.Key, nil)
	require.Equal(t, http.StatusOK, submitted.Code, submitted.Body.String())
	assert.Contains(t, submitted.Body.String(), localTask)
	assert.EqualValues(t, 1, submits.Load())
	var stored model.Task
	require.NoError(t, db.Where("task_id = ?", localTask).First(&stored).Error)
	require.Greater(t, stored.Quota, 40)
	assert.Equal(t, moliiTask, stored.PrivateData.UpstreamTaskID)
	assert.Equal(t, service.BillingSourceWallet, stored.PrivateData.BillingSource)
	assert.Equal(t, 2.0, stored.PrivateData.BillingContext.ModelRatio)
	assert.Equal(t, 0.5, stored.PrivateData.BillingContext.GroupRatio)
	var wallet model.User
	require.NoError(t, db.First(&wallet, user.Id).Error)
	assert.Equal(t, 2_000_000-stored.Quota, wallet.Quota)
	assert.Zero(t, wallet.UsedQuota, "reservation is not final consumption")

	_, err = service.RunTaskPollingOnceWithError(context.Background(), nil)
	require.NoError(t, err)
	var job model.TaskBillingJob
	require.NoError(t, db.Where("task_id = ?", stored.ID).First(&job).Error)
	require.NotNil(t, job.TargetQuota)
	assert.Equal(t, 40, *job.TargetQuota, "40 tokens × local model ratio 2 × local group ratio 0.5")
	// A new worker must recover a durable job after an expired process lease.
	claimed, err := model.ClaimTaskBillingJobs("crashed-full-chain", time.Now().Unix(), time.Now().Unix()+60, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, db.Model(&model.TaskBillingJob{}).Where("id = ?", job.ID).Update("locked_until", time.Now().Unix()-1).Error)
	for _, worker := range []string{"restarted-full-chain", "repeated-full-chain"} {
		_, err = service.RunTaskPollingOnceWithError(context.Background(), nil)
		require.NoError(t, err)
		_, err = service.RunTaskBillingReconciliationOnce(context.Background(), worker)
		require.NoError(t, err)
	}
	require.NoError(t, db.First(&stored, stored.ID).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, stored.Status)
	assert.Equal(t, 40, stored.Quota)
	assert.NotNil(t, stored.PrivateData.Timing)
	assert.EqualValues(t, 1700000000, stored.PrivateData.Timing.UpstreamSubmittedAt)
	assert.EqualValues(t, 1700000002, stored.PrivateData.Timing.UpstreamStartedAt)
	assert.EqualValues(t, 1700000008, stored.PrivateData.Timing.UpstreamFinishedAt)
	assert.Nil(t, stored.PrivateData.StoredResult, "reseller must not persist another video copy")
	for _, private := range []string{moliiKey, starAIKey, starAITask, "private.invalid"} {
		assert.NotContains(t, string(stored.Data), private)
	}
	require.NoError(t, db.First(&wallet, user.Id).Error)
	assert.Equal(t, 1_999_960, wallet.Quota)
	assert.Equal(t, 40, wallet.UsedQuota)
	assert.Equal(t, 1, wallet.RequestCount)
	var settledToken model.Token
	require.NoError(t, db.First(&settledToken, token.Id).Error)
	assert.Equal(t, 1_999_960, settledToken.RemainQuota)
	assert.Equal(t, 40, settledToken.UsedQuota)
	require.NoError(t, db.First(&job, job.ID).Error)
	assert.Equal(t, model.TaskBillingJobStatusSucceeded, job.Status)
	var jobs, logs int64
	require.NoError(t, db.Model(&model.TaskBillingJob{}).Count(&jobs).Error)
	require.NoError(t, db.Model(&model.Log{}).Where("type = ?", model.LogTypeConsume).Count(&logs).Error)
	assert.EqualValues(t, 1, jobs)
	assert.EqualValues(t, 1, logs)
	assert.EqualValues(t, 1, polls.Load())
	for _, path := range []string{"/v1/video/generations/" + localTask, "/v1/videos/" + localTask} {
		result := request(http.MethodGet, path, "", token.Key, nil)
		require.Equal(t, http.StatusOK, result.Code, result.Body.String())
		assert.Contains(t, result.Body.String(), localTask)
		assert.Contains(t, result.Body.String(), `"total_tokens":40`)
		if strings.HasPrefix(path, "/v1/video/generations/") {
			facts, parseErr := relay.GetTaskAdaptor(stored.Platform).ParseTaskResult(nil, nil, result.Body.Bytes())
			require.NoError(t, parseErr)
			assert.Equal(t, model.TaskStatusSuccess, facts.Status)
			assert.Equal(t, 40, facts.TotalTokens)
			assert.Equal(t, 30, facts.CompletionTokens)
		}
	}
	contentPath := "/v1/videos/" + localTask + "/content"
	unauthenticated := request(http.MethodGet, contentPath, "", "", nil)
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	otherContent := request(http.MethodGet, contentPath, "", otherToken.Key, nil)
	require.Equal(t, http.StatusNotFound, otherContent.Code)
	assert.Zero(t, downloads.Load())
	content := request(http.MethodGet, contentPath, "", token.Key, http.Header{"Range": {"bytes=2-5"}, "If-Range": {`"fixture-etag"`}})
	require.Equal(t, http.StatusPartialContent, content.Code, content.Body.String())
	assert.Equal(t, "2345", content.Body.String())
	assert.Equal(t, "bytes 2-5/10", content.Header().Get("Content-Range"))
	assert.Equal(t, "video/mp4", content.Header().Get("Content-Type"))
	assert.EqualValues(t, 1, downloads.Load())
	deleted := request(http.MethodDelete, "/v1/assets/"+assetID, "", token.Key, nil)
	require.Equal(t, http.StatusOK, deleted.Code, deleted.Body.String())
	_, err = service.GetStarAIAssetBinding(assetID, user.Id)
	require.ErrorIs(t, err, service.ErrStarAIAssetNotFound)
}
