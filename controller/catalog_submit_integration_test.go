package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relay"
	grok "github.com/QuantumNous/new-api/relay/channel/task/moliigrok"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real production controller submission, adaptor, expression capture, wallet,
// both insertion paths, reload and terminal consumer. Only the upstream HTTP
// response is a local provider fixture; authentication/routing middleware is
// separately covered and is not claimed by this controller lifecycle test.
func TestCatalogSubmitPersistedSelection(t *testing.T) {
	for _, immediate := range []bool{false, true} {
		t.Run(fmt.Sprintf("immediate=%t", immediate), func(t *testing.T) {
			db := catalogImagePostgres(t, map[string]string{"group_ratio_setting.group_ratio": `{"default":1}`, "quota_setting.trust_quota_usd": "0"})
			require.NoError(t, db.AutoMigrate(&model.TaskBillingJob{}))
			const expression = `u("units") * 0.01`
			const source = `
export const meta={apiVersion:1,key:"catalog-submit",name:"Catalog submit",version:"1.0.0",author:{name:"Test"},models:["submit-model"],fetchMode:"per_task",usageSchema:{units:{type:"number",unit:"count"}}};
export function buildSubmitRequest(ctx){return {url:ctx.baseUrl+"/submit",body:ctx.requestBody};}
export function parseSubmitResponse(ctx,resp){return resp.body.immediate ? {taskId:"provider-task",taskData:resp.body,immediate:{status:"SUCCESS"}} : {taskId:"provider-task",taskData:resp.body};}
export function extractUsage(){return {units:4};}
export function extractUsageOnComplete(ctx,result,body){return body.usage;}
export function buildQueryRequest(){return {}} export function parseTaskResult(){return {status:"SUCCESS"}}
`
			plugin, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
			require.NoError(t, err)
			require.NoError(t, db.Create(&model.TaskPlugin{Key: plugin.Meta.Key, APIVersion: 1, Version: "1.0.0", Source: model.LongText(source), SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(source))), Active: true, Enabled: true}).Error)
			require.NoError(t, db.Create(&model.Model{ModelName: "submit-model", BillingCurrency: "USD"}).Error)
			require.NoError(t, db.Create(&[]model.Option{{Key: "billing_setting.billing_mode", Value: `{"submit-model":"tiered_expr"}`}, {Key: "billing_setting.billing_expr", Value: `{"submit-model":"u(\"units\") * 0.01"}`}}).Error)
			require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
			entered, release := make(chan struct{}), make(chan struct{})
			unblock := sync.OnceFunc(func() { close(release) })
			defer unblock()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				<-release
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"immediate":%t,"usage":{"units":2}}`, immediate)
			}))
			defer func() { unblock(); server.Close() }()
			user := model.User{Username: "catalog-submit", AffCode: "catalog-submit", Quota: 10000000}
			require.NoError(t, db.Create(&user).Error)
			channel := model.Channel{Name: "fixture-provider", Type: constant.ChannelTypeTaskPlugin, Key: "fixture-key", Status: common.ChannelStatusEnabled, Models: "submit-model", Group: "default"}
			require.NoError(t, db.Create(&channel).Error)
			c := taskSubmissionTestContext()
			c.Request = httptest.NewRequest("POST", "/v1/videos", strings.NewReader(`{"model":"submit-model"}`))
			c.Set("group", "default")
			c.Set("username", user.Username)
			c.Set("task_request", map[string]any{"model": "submit-model"})
			c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Plugin: plugin, Generation: jsplugin.DefaultRegistry.Generation()})
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "submit-model")
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
			common.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
			common.SetContextKey(c, constant.ContextKeyChannelType, channel.Type)
			info := taskSubmissionRelayInfo(nil)
			info.UserId, info.UserQuota, info.OriginModelName = user.Id, user.Quota, "submit-model"
			info.UserGroup, info.UsingGroup, info.IsPlayground, info.ForcePreConsume = "default", "default", true, true
			info.UserSetting.BillingPreference = "wallet_only"
			info.PublicTaskID, info.LockedChannel = model.GenerateTaskID(), &channel
			info.ChannelId, info.ChannelType, info.UpstreamModelName = channel.Id, channel.Type, "submit-model"
			info.StartTime = time.Now()
			type result struct {
				outcome *taskSubmissionOutcome
				err     *dto.TaskError
			}
			finished := make(chan result, 1)
			go func() { outcome, err := executeTaskSubmission(c, info); finished <- result{outcome, err} }()
			select {
			case <-entered:
			case done := <-finished:
				t.Fatalf("submission ended before local provider: %+v", done.err)
			case <-time.After(10 * time.Second):
				t.Fatal("submission did not reach local provider")
			}
			err = model.UpdateOptionsBulk(map[string]string{"billing_setting.billing_expr": `{"submit-model":"u(\"units\") * 0.03"}`})
			unblock()
			require.NoError(t, err)
			done := <-finished
			require.Nil(t, done.err)
			require.NotNil(t, done.outcome)
			var stored model.Task
			require.NoError(t, db.Where("task_id = ?", info.PublicTaskID).First(&stored).Error)
			require.NotNil(t, stored.PrivateData.BillingContext)
			snapshot := stored.PrivateData.BillingContext.TieredSnapshot
			require.NotNil(t, snapshot)
			assert.Equal(t, expression, snapshot.ExprString)
			assert.Equal(t, "USD", snapshot.SourceCurrency)
			want := 20000
			if immediate {
				want = 10000
			}
			assert.Equal(t, want, stored.Quota)
			var wallet model.User
			require.NoError(t, db.First(&wallet, user.Id).Error)
			assert.Equal(t, 10000000-want, wallet.Quota)
			if immediate {
				var job model.TaskBillingJob
				require.NoError(t, db.Where("task_id = ?", stored.ID).First(&job).Error)
				assert.Equal(t, model.TaskBillingJobStatusSucceeded, job.Status)
				require.NoError(t, service.ApplyTaskBillingJob(context.Background(), &job))
				require.NoError(t, db.First(&wallet, user.Id).Error)
				assert.Equal(t, 10000000-want, wallet.Quota, "succeeded billing-job replay cannot debit twice")
			} else {
				stored.Status = model.TaskStatusSuccess
				job := service.BuildTerminalTaskBillingJob(context.Background(), relay.GetTaskAdaptor(constant.TaskPlatform(plugin.Meta.Key)), &stored, &relaycommon.TaskInfo{UsageFacts: map[string]any{"units": float64(2)}})
				require.NotNil(t, job.TargetQuota)
				assert.Equal(t, 10000, *job.TargetQuota)
			}
			assert.False(t, c.Writer.Written(), "controller persistence precedes the presenter")
		})
	}
}

func TestCatalogSubmitNativeGrokSnapshot(t *testing.T) {
	db := catalogImagePostgres(t, map[string]string{"group_ratio_setting.group_ratio": `{"default":1}`, "quota_setting.trust_quota_usd": "0", "ModelPrice": `{"grok-imagine-video":1}`, "molii_grok_price.video_720p": "0.07", "molii_grok_price.video_480p": "0.03"})
	require.NoError(t, db.Create(&model.Model{ModelName: grok.LegacyVideoModel, BillingCurrency: "USD"}).Error)
	require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
	published := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		published <- model.UpdateOptionsBulk(map[string]string{"molii_grok_price.video_720p": "0.7"})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"request_id":"native-provider-task"}`)
	}))
	defer server.Close()
	user := model.User{Username: "catalog-grok", AffCode: "catalog-grok", Quota: 10000000}
	require.NoError(t, db.Create(&user).Error)
	channel := model.Channel{Name: "grok-fixture", Type: constant.ChannelTypeMoliiGrokAIGC, Key: "fixture-key", Status: common.ChannelStatusEnabled, Models: grok.LegacyVideoModel, Group: "default", BaseURL: &server.URL}
	require.NoError(t, db.Create(&channel).Error)
	c := taskSubmissionTestContext()
	c.Request = httptest.NewRequest("POST", "/v1/videos", strings.NewReader(`{"model":"grok-imagine-video","prompt":"fixture","duration":5,"resolution":"720p"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("group", "default")
	c.Set("username", user.Username)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, grok.LegacyVideoModel)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
	common.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
	common.SetContextKey(c, constant.ContextKeyChannelType, channel.Type)
	info := taskSubmissionRelayInfo(nil)
	info.UserId, info.UserQuota, info.OriginModelName = user.Id, user.Quota, grok.LegacyVideoModel
	info.UserGroup, info.UsingGroup, info.IsPlayground, info.ForcePreConsume = "default", "default", true, true
	info.UserSetting.BillingPreference = "wallet_only"
	info.PublicTaskID, info.LockedChannel = model.GenerateTaskID(), &channel
	info.ChannelId, info.ChannelType, info.UpstreamModelName = channel.Id, channel.Type, grok.LegacyVideoModel
	info.ChannelBaseUrl, info.ApiKey, info.StartTime = server.URL, "fixture-key", time.Now()
	outcome, taskErr := executeTaskSubmission(c, info)
	require.Nil(t, taskErr)
	require.NoError(t, <-published)
	require.NotNil(t, outcome)
	var stored model.Task
	require.NoError(t, db.Where("task_id = ?", info.PublicTaskID).First(&stored).Error)
	bc := stored.PrivateData.BillingContext
	require.NotNil(t, bc)
	require.NotNil(t, bc.GrokVideoBilling)
	assert.Equal(t, 5, bc.EstimatedSeconds)
	assert.Equal(t, "720p", bc.EstimatedResolution)
	assert.Equal(t, 0.07, bc.EstimatedUnitPrice)
	assert.Equal(t, 0.07, bc.GrokVideoBilling.OutputUnitPrice)
	assert.Equal(t, "USD", bc.GrokVideoBilling.SourceCurrency)
	assert.Equal(t, 175000, stored.Quota)
	info.GrokVideoBilling.OutputUnitPrice = 999
	assert.Equal(t, 0.07, outcome.Task.PrivateData.BillingContext.GrokVideoBilling.OutputUnitPrice, "controller stores a detached Grok clone")
	stored.Status = model.TaskStatusSuccess
	job := service.BuildTerminalTaskBillingJob(context.Background(), &grok.TaskAdaptor{}, &stored, &relaycommon.TaskInfo{ActualDurationSeconds: 5, ActualResolution: "720p"})
	require.NotNil(t, job.TargetQuota)
	assert.Equal(t, 175000, *job.TargetQuota)
}
