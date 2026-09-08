package jsplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/service"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRealBatchPluginOversizedStateAccumulatesToCutoff(t *testing.T) {
	// SQLite is a local unit fixture, not PostgreSQL/Redis acceptance evidence.
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "poll.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}, &model.TaskBillingJob{}))
	oldDB, oldMemory, oldRedis, oldCutoff := model.DB, common.MemoryCacheEnabled, common.RedisEnabled, constant.TaskPollMaxFailures
	model.DB, common.MemoryCacheEnabled, common.RedisEnabled, constant.TaskPollMaxFailures = db, false, false, 3
	t.Cleanup(func() {
		model.DB, common.MemoryCacheEnabled, common.RedisEnabled, constant.TaskPollMaxFailures = oldDB, oldMemory, oldRedis, oldCutoff
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("{}")) }))
	defer server.Close()
	baseURL := server.URL
	ch := model.Channel{Type: 1, Status: common.ChannelStatusEnabled, Key: "test", BaseURL: &baseURL}
	require.NoError(t, db.Create(&ch).Error)
	plugin, err := pluginruntime.NewRegistry().Register(`
export const meta={apiVersion:1,key:"oversized-batch",name:"Oversized Batch",version:"1.0.0",author:{name:"Test"},models:["m"],fetchMode:"batch"};
export function buildSubmitRequest(){return {};}
export function parseSubmitResponse(){return {taskId:"unused"};}
export function buildQueryRequest(ctx){return {url:ctx.baseUrl+"/poll"};}
export function parseTaskResult(){return {status:"IN_PROGRESS"};}
export function buildBatchQueryRequest(ctx,tasks){return {url:ctx.baseUrl+"/poll",method:"POST",body:{ids:tasks.map(t=>t.taskId)}};}
export function parseBatchResult(){return [
 {taskId:"up-bad",status:"IN_PROGRESS",progress:"90%",action:"invalid-action",state:{value:"x".repeat(1024*1024+1)}},
 {taskId:"up-good",status:"IN_PROGRESS",progress:"40%",state:{round:"good"}}
];}
`, pluginruntime.Options{})
	require.NoError(t, err)
	adaptor := New(plugin)
	bad := &model.Task{TaskID: "task_bad", Status: model.TaskStatusInProgress, Progress: "10%", Action: "original", ChannelId: ch.Id, Quota: 4000, PrivateData: model.TaskPrivateData{UpstreamTaskID: "up-bad", PluginState: json.RawMessage(`{"keep":true}`)}}
	good := &model.Task{TaskID: "task_good", Status: model.TaskStatusInProgress, Progress: "10%", ChannelId: ch.Id, PrivateData: model.TaskPrivateData{UpstreamTaskID: "up-good", PollFailures: 2}}
	require.NoError(t, db.Create(bad).Error)
	require.NoError(t, db.Create(good).Error)
	for attempt := 1; attempt <= 3; attempt++ {
		require.NoError(t, service.UpdateBatchTasks(context.Background(), adaptor, map[int][]string{ch.Id: {"up-bad", "up-good"}}, map[string]*model.Task{"up-bad": bad, "up-good": good}))
		var gotBad, gotGood model.Task
		require.NoError(t, db.First(&gotBad, bad.ID).Error)
		require.NoError(t, db.First(&gotGood, good.ID).Error)
		require.Equal(t, attempt, gotBad.PrivateData.PollFailures)
		require.Equal(t, "original", gotBad.Action)
		require.JSONEq(t, `{"keep":true}`, string(gotBad.PrivateData.PluginState))
		require.Equal(t, 0, gotGood.PrivateData.PollFailures, "one invalid item must not penalize healthy batch items")
		require.Equal(t, "40%", gotGood.Progress)
		require.JSONEq(t, `{"round":"good"}`, string(gotGood.PrivateData.PluginState))
		if attempt < 3 {
			require.Equal(t, model.TaskStatus(model.TaskStatusInProgress), gotBad.Status)
			require.Equal(t, "10%", gotBad.Progress)
		} else {
			require.Equal(t, model.TaskStatus(model.TaskStatusFailure), gotBad.Status)
			var count int64
			require.NoError(t, db.Model(&model.TaskBillingJob{}).Where("task_id = ?", bad.ID).Count(&count).Error)
			require.EqualValues(t, 1, count)
		}
		bad, good = &gotBad, &gotGood
	}
}
