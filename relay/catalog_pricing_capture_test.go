package relay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func catalogPricingPostgres(t *testing.T, options map[string]string) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	require.NotEmpty(t, dsn, "task PostgreSQL is required")
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname())
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	require.NoError(t, err)
	name := fmt.Sprintf("catalog_request_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec(`CREATE DATABASE "`+name+`"`).Error)
	t.Cleanup(func() {
		require.NoError(t, admin.Exec(`DROP DATABASE "`+name+`"`).Error)
		pool, err := admin.DB()
		require.NoError(t, err)
		require.NoError(t, pool.Close())
	})
	u.Path = "/" + name
	db, err := gorm.Open(postgres.Open(u.String()), cfg)
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	gin.SetMode(gin.TestMode)
	previousDB, previousLogDB, previousRegistry := model.DB, model.LOG_DB, jsplugin.DefaultRegistry
	previousType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	previousRedis, previousBatch, previousLogs := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	previousCache := common.MemoryCacheEnabled
	previousConfig := config.GlobalConfig.ExportAllConfigs()
	previousRate, previousName := operation_setting.USDExchangeRate, common.SystemName
	common.OptionMapRWMutex.RLock()
	previousOptions := maps.Clone(common.OptionMap)
	common.OptionMapRWMutex.RUnlock()
	restorePricing := []struct {
		value   string
		restore func(string) error
	}{
		{ratio_setting.ModelPrice2JSONString(), ratio_setting.UpdateModelPriceByJSONString},
		{ratio_setting.ModelRatio2JSONString(), ratio_setting.UpdateModelRatioByJSONString},
		{ratio_setting.CompletionRatio2JSONString(), ratio_setting.UpdateCompletionRatioByJSONString},
		{ratio_setting.CacheRatio2JSONString(), ratio_setting.UpdateCacheRatioByJSONString},
		{ratio_setting.CreateCacheRatio2JSONString(), ratio_setting.UpdateCreateCacheRatioByJSONString},
		{ratio_setting.ImageRatio2JSONString(), ratio_setting.UpdateImageRatioByJSONString},
		{ratio_setting.AudioRatio2JSONString(), ratio_setting.UpdateAudioRatioByJSONString},
		{ratio_setting.AudioCompletionRatio2JSONString(), ratio_setting.UpdateAudioCompletionRatioByJSONString},
	}
	model.DB, jsplugin.DefaultRegistry = db, jsplugin.NewRegistry()
	model.LOG_DB = db
	common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, true
	common.MemoryCacheEnabled = false
	common.SetMainDatabaseType(common.DatabaseTypePostgreSQL)
	common.SetLogDatabaseType(common.DatabaseTypePostgreSQL)
	// Initialize the model package's dialect-aware columns through its public
	// log startup path; a non-master call performs no unrelated migrations.
	t.Setenv("LOG_SQL_DSN", "")
	master := common.IsMasterNode
	common.IsMasterNode = false
	err = model.InitLogDB()
	common.IsMasterNode = master
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(previousConfig))
		for _, pricing := range restorePricing {
			require.NoError(t, pricing.restore(pricing.value))
		}
		operation_setting.USDExchangeRate, common.SystemName = previousRate, previousName
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
		model.DB, model.LOG_DB, jsplugin.DefaultRegistry = previousDB, previousLogDB, previousRegistry
		common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = previousRedis, previousBatch, previousLogs
		common.MemoryCacheEnabled = previousCache
		common.SetMainDatabaseType(previousType)
		common.SetLogDatabaseType(previousLogType)
	})
	require.NoError(t, db.AutoMigrate(&model.Model{}, &model.Vendor{}, &model.Option{}, &model.Channel{}, &model.Ability{}, &model.Task{}, &model.TaskPlugin{}, &model.Midjourney{}, &model.SystemTask{}, &model.User{}, &model.Token{}, &model.Log{}))
	require.NoError(t, model.MigrateCatalogSync(db))
	for key, value := range options {
		require.NoError(t, db.Create(&model.Option{Key: key, Value: value}).Error)
	}
	require.NoError(t, model.InitOptionMapBootstrap(context.Background()))
	require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
	var version string
	require.NoError(t, db.Raw("SELECT version()").Scan(&version).Error)
	t.Log(version)
	return db
}

type catalogPricingBody struct {
	reader  io.Reader
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *catalogPricingBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	return b.reader.Read(p)
}

func TestCatalogRequestExpressionMoneyOverlap(t *testing.T) {
	db := catalogPricingPostgres(t, map[string]string{
		"billing_setting.billing_mode": `{"capture-expression":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"capture-expression":"p * 2"}`,
		"USDExchangeRate":              "7",
	})
	entry := model.Model{ModelName: "capture-expression", BillingCurrency: "USD"}
	require.NoError(t, entry.Insert())
	before, err := model.GetModelPricingSnapshot([]string{entry.ModelName})
	require.NoError(t, err)
	body := &catalogPricingBody{reader: strings.NewReader(`{}`), entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(body.release) }) }
	defer release()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", body)
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{OriginModelName: entry.ModelName, UsingGroup: "default", UserGroup: "default"}
	done := make(chan error, 1)
	go func() { _, err := helper.ModelPriceHelper(c, info, 100, &kittypes.TokenCountMeta{}); done <- err }()
	select {
	case <-body.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach body processing")
	}
	// The actual price+currency save must finish while body work is suspended.
	published := make(chan error, 1)
	go func() {
		published <- model.UpdateModelPricing([]model.ModelPricingChange{{ModelName: entry.ModelName, ExpectedVersion: before.Entries[0].Version, BillingCurrency: "CNY", Pricing: model.PricingValues{"billing_setting.billing_mode": "tiered_expr", "billing_setting.billing_expr": "p * 21"}}})
	}()
	select {
	case err = <-published:
	case <-time.After(5 * time.Second):
		release()
		<-published
		<-done
		t.Fatal("publication waited for request body processing")
	}
	release()
	require.NoError(t, err)
	require.NoError(t, <-done)
	assert.Equal(t, 100, info.PriceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	assert.Equal(t, "p * 2", info.TieredBillingSnapshot.ExprString)
	assert.Equal(t, "USD", info.TieredBillingSnapshot.SourceCurrency)
	assert.Equal(t, float64(7), info.TieredBillingSnapshot.CNYPerUSD)
	newContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	newContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	newInfo := &relaycommon.RelayInfo{OriginModelName: entry.ModelName, UsingGroup: "default", UserGroup: "default"}
	_, err = helper.ModelPriceHelper(newContext, newInfo, 100, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	assert.Equal(t, 150, newInfo.PriceData.QuotaToPreConsume)
	assert.Equal(t, "CNY", newInfo.TieredBillingSnapshot.SourceCurrency)
	var state model.CatalogSyncState
	require.NoError(t, db.First(&state, model.CatalogSyncStateID).Error)
	assert.Equal(t, state.Revision, state.RuntimeRevision)
}

func TestCatalogRequestToolSettlementRemainsSelected(t *testing.T) {
	db := catalogPricingPostgres(t, map[string]string{
		"ModelPrice":                   `{"capture-model":0.1}`,
		"billing_setting.billing_mode": `{}`,
		"tool_price_setting.prices":    `{"late_custom":3}`,
	})
	user := model.User{Username: "capture", Quota: 1000000}
	require.NoError(t, db.Create(&user).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	info := &relaycommon.RelayInfo{OriginModelName: "capture-model", UsingGroup: "default", UserGroup: "default", UserId: user.Id, UserQuota: 1000000, IsPlayground: true, StartTime: time.Now(), ChannelMeta: &relaycommon.ChannelMeta{}}
	_, err := helper.ModelPriceHelper(c, info, 10, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	info.CountBillableToolCall(dto.BuildInCallFunctionCall, "late_custom")
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"tool_price_setting.prices": `{"late_custom":0}`}))
	service.PostTextConsumeQuota(c, info, &dto.Usage{PromptTokens: 10, TotalTokens: 10}, nil)
	var log model.Log
	require.NoError(t, db.Where("user_id = ?", user.Id).Last(&log).Error)
	assert.Equal(t, 51500, log.Quota, "0.1 USD base plus 3 USD/1000 tool fee")
}

func TestCatalogRequestToolPricesRemainSelected(t *testing.T) {
	catalogPricingPostgres(t, map[string]string{
		"ModelPrice":                   `{"capture-model":0.1}`,
		"billing_setting.billing_mode": `{}`,
		"tool_price_setting.prices":    `{"late_custom":3,"free_custom":0}`,
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	info := &relaycommon.RelayInfo{OriginModelName: "capture-model", UsingGroup: "default", UserGroup: "default"}
	_, err := helper.ModelPriceHelper(c, info, 10, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"tool_price_setting.prices": `{"late_custom":0,"free_custom":8,"new_custom":9}`}))
	info.CountBillableToolCall(dto.BuildInCallFunctionCall, "late_custom")
	info.CountBillableToolCall(dto.BuildInCallFunctionCall, "free_custom")
	info.CountBillableToolCall(dto.BuildInCallFunctionCall, "new_custom")
	require.NotNil(t, info.ResponsesUsageInfo)
	assert.Contains(t, info.ResponsesUsageInfo.BuiltInTools, "late_custom", "a priced request-time custom tool must still count after price becomes zero")
	assert.NotContains(t, info.ResponsesUsageInfo.BuiltInTools, "free_custom", "a captured zero must not start counting midstream")
	assert.NotContains(t, info.ResponsesUsageInfo.BuiltInTools, "new_custom", "a missing captured name must not fall back to a newly configured live price")
}

func TestCatalogRequestPendingAndFrozenSelection(t *testing.T) {
	db := catalogPricingPostgres(t, map[string]string{"ModelPrice": `{"capture-pending":0}`, "billing_setting.billing_mode": `{}`})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{OriginModelName: "capture-pending", UsingGroup: "default", UserGroup: "default"}
	_, err := helper.ModelPriceHelper(c, info, 10, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	selected, found := info.PricingSelection.Model("capture-pending")
	require.True(t, found)
	assert.True(t, selected.HasPrice)
	assert.Zero(t, selected.Price)
	assert.False(t, selected.HasExpression)
	assert.Empty(t, selected.Expression)
	failure := errors.New("required pricing projection failed")
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("request-pending", func(tx *gorm.DB) {
		if tx.Statement.Table == "vendors" && tx.Statement.ConnPool == db.Statement.ConnPool {
			tx.AddError(failure)
		}
	}))
	err = model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{"capture-pending":8}`})
	require.NoError(t, db.Callback().Query().Remove("request-pending"))
	require.ErrorIs(t, err, failure)
	newInfo := &relaycommon.RelayInfo{OriginModelName: "capture-pending", UsingGroup: "default", UserGroup: "default"}
	_, err = helper.ModelPriceHelper(c, newInfo, 10, &kittypes.TokenCountMeta{})
	require.ErrorIs(t, err, model.ErrCatalogPublicationPending)
	assert.Nil(t, newInfo.PricingSelection)
	_, err = helper.ModelPriceHelper(c, info, 10, &kittypes.TokenCountMeta{})
	require.NoError(t, err, "already selected request remains usable during pending")
	assert.Zero(t, info.PriceData.ModelPrice)
	require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
	_, err = helper.ModelPriceHelper(c, newInfo, 10, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	assert.Equal(t, float64(8), newInfo.PriceData.ModelPrice)
	assert.Zero(t, info.PriceData.ModelPrice)
}

func TestCatalogRequestToolPrecedenceAndTieredSurcharge(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		mode             int
		want             int
	}{
		{"grok-capture", "", relayconstant.RelayModeResponses, 53500},
		{"capture-search-preview", `tier("request", fixed(0.01))`, relayconstant.RelayModeChatCompletions, 9500},
		{"capture-responses", `tier("request", fixed(0.01))`, relayconstant.RelayModeResponses, 8500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			modes, expressions := map[string]string{}, map[string]string{}
			if tc.expression != "" {
				modes[tc.name] = billing_setting.BillingModeTieredExpr
				expressions[tc.name] = tc.expression
			}
			modeJSON, err := common.Marshal(modes)
			require.NoError(t, err)
			exprJSON, err := common.Marshal(expressions)
			require.NoError(t, err)
			priceJSON, err := common.Marshal(map[string]float64{tc.name: 0.1})
			require.NoError(t, err)
			db := catalogPricingPostgres(t, map[string]string{
				"ModelPrice": string(priceJSON), "billing_setting.billing_mode": string(modeJSON), "billing_setting.billing_expr": string(exprJSON),
				"tool_price_setting.prices":        `{"late":2,"late:grok*":4,"late:grok-capture*":2,"web_search":7,"web_search_preview":7,"x_search":999}`,
				"molii_grok_tool_price.web_search": "0", "molii_grok_tool_price.x_search": "5",
			})
			user := model.User{Username: "tools", Quota: 1000000}
			require.NoError(t, db.Create(&user).Error)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
			info := &relaycommon.RelayInfo{OriginModelName: tc.name, RelayMode: tc.mode, UsingGroup: "default", UserGroup: "default", UserId: user.Id, UserQuota: 1000000, IsPlayground: true, StartTime: time.Now(), ChannelMeta: &relaycommon.ChannelMeta{}}
			_, err = helper.ModelPriceHelper(c, info, 10, &kittypes.TokenCountMeta{})
			require.NoError(t, err)
			require.NoError(t, model.UpdateOptionsBulk(map[string]string{"tool_price_setting.prices": `{"late":0,"web_search":0,"web_search_preview":0,"x_search":0}`, "molii_grok_tool_price.web_search": "9", "molii_grok_tool_price.x_search": "9"}))
			if tc.name == "grok-capture" {
				info.CountBillableToolCall(dto.BuildInCallFunctionCall, "late")
				info.CountBillableToolCall(dto.BuildInCallXSearchCall, "")
				info.CountBillableToolCall(dto.BuildInCallWebSearchCall, "")
				assert.Zero(t, info.ToolPrice("web_search", tc.name), "explicit Grok zero overrides a nonzero generic fee")
			} else if tc.mode == relayconstant.RelayModeResponses {
				info.CountBillableToolCall(dto.BuildInCallWebSearchCall, "")
			} else {
				info.CountBillableToolCall(dto.BuildInCallFunctionCall, "late")
			}
			service.PostTextConsumeQuota(c, info, &dto.Usage{PromptTokens: 10, TotalTokens: 10}, nil)
			var log model.Log
			require.NoError(t, db.Where("user_id = ?", user.Id).Last(&log).Error)
			assert.Equal(t, tc.want, log.Quota)
		})
	}
}

func TestCatalogRequestAudioSettlementRemainsSelected(t *testing.T) {
	db := catalogPricingPostgres(t, map[string]string{
		"ModelRatio": `{"capture-audio":2}`, "CompletionRatio": `{"capture-audio":3}`,
		"AudioRatio": `{"capture-audio":5}`, "AudioCompletionRatio": `{"capture-audio":7}`,
		"billing_setting.billing_mode": `{}`,
	})
	user := model.User{Username: "audio", Quota: 1000000}
	require.NoError(t, db.Create(&user).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/audio", nil)
	info := &relaycommon.RelayInfo{OriginModelName: "capture-audio", UsingGroup: "default", UserGroup: "default", UserId: user.Id, UserQuota: 1000000, IsPlayground: true, StartTime: time.Now(), ChannelMeta: &relaycommon.ChannelMeta{}}
	_, err := helper.ModelPriceHelper(c, info, 10, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelRatio": `{"capture-audio":20}`, "CompletionRatio": `{"capture-audio":30}`, "AudioRatio": `{"capture-audio":50}`, "AudioCompletionRatio": `{"capture-audio":70}`}))
	usage := &dto.Usage{PromptTokens: 40, CompletionTokens: 60, TotalTokens: 100}
	usage.PromptTokensDetails.TextTokens, usage.PromptTokensDetails.AudioTokens = 10, 30
	usage.CompletionTokenDetails.TextTokens, usage.CompletionTokenDetails.AudioTokens = 20, 40
	service.PostAudioConsumeQuota(c, info, usage, "")
	var log model.Log
	require.NoError(t, db.Where("user_id = ?", user.Id).Last(&log).Error)
	assert.Equal(t, 3240, log.Quota, "(10 + 20*3 + 30*5 + 40*5*7)*2 remains frozen")
	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	assert.Equal(t, float64(3), other["completion_ratio"])
	assert.Equal(t, float64(5), other["audio_ratio"])
	assert.Equal(t, float64(7), other["audio_completion_ratio"])
}

func TestCatalogRequestRealtimeRatesAndSaturation(t *testing.T) {
	db := catalogPricingPostgres(t, map[string]string{
		"ModelRatio": `{"capture-audio":2}`, "CompletionRatio": `{"capture-audio":3}`,
		"AudioRatio": `{"capture-audio":5}`, "AudioCompletionRatio": `{"capture-audio":7}`,
		"billing_setting.billing_mode": `{}`,
	})
	user := model.User{Username: "realtime", Quota: 1000000}
	require.NoError(t, db.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: strings.Repeat("k", 48), UnlimitedQuota: true}
	require.NoError(t, db.Create(&token).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{OriginModelName: "capture-audio", UsingGroup: "default", UserGroup: "default", UserId: user.Id, UserQuota: 1000000, TokenId: token.Id, TokenKey: token.Key, IsPlayground: true, StartTime: time.Now(), ChannelMeta: &relaycommon.ChannelMeta{}, RelayFormat: kittypes.RelayFormatOpenAIRealtime}
	_, err := helper.ModelPriceHelper(c, info, 10, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelRatio": `{"capture-audio":20}`, "CompletionRatio": `{"capture-audio":30}`, "AudioRatio": `{"capture-audio":50}`, "AudioCompletionRatio": `{"capture-audio":70}`}))
	usage := &dto.RealtimeUsage{InputTokens: 40, OutputTokens: 60, TotalTokens: 100}
	usage.InputTokenDetails.TextTokens, usage.InputTokenDetails.AudioTokens = 10, 30
	usage.OutputTokenDetails.TextTokens, usage.OutputTokenDetails.AudioTokens = 20, 40
	require.NoError(t, service.PreWssConsumeQuota(c, info, usage))
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 996760, user.Quota)
	service.PostWssConsumeQuota(c, info, "different-upstream-label", usage, "")
	var log model.Log
	require.NoError(t, db.Where("user_id = ?", user.Id).Last(&log).Error)
	assert.Equal(t, 3240, log.Quota)
	assert.Equal(t, "different-upstream-label", log.ModelName, "log identity remains the existing caller's identity")
	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	assert.Equal(t, float64(3), other["completion_ratio"])
	assert.Equal(t, float64(5), other["audio_ratio"])
	assert.Equal(t, float64(7), other["audio_completion_ratio"])
	require.NoError(t, db.First(&user, user.Id).Error)
	before := user.Quota
	huge := *usage
	huge.OutputTokenDetails.AudioTokens = common.MaxQuota
	require.ErrorContains(t, service.PreWssConsumeQuota(c, info, &huge), "user quota is not enough")
	require.NotNil(t, info.QuotaClamp)
	assert.Equal(t, common.MaxQuota, info.QuotaClamp.Clamped)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, before, user.Quota, "saturated quota cannot deduct or wrap")
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelRatio": `{"capture-audio":0}`, "CompletionRatio": `{"capture-audio":0}`, "AudioRatio": `{"capture-audio":0}`, "AudioCompletionRatio": `{"capture-audio":0}`}))
	free := *info
	free.PricingSelection, free.QuotaClamp = nil, nil
	_, err = helper.ModelPriceHelper(c, &free, 10, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelRatio": `{"capture-audio":20}`, "CompletionRatio": `{"capture-audio":30}`, "AudioRatio": `{"capture-audio":50}`, "AudioCompletionRatio": `{"capture-audio":70}`}))
	require.NoError(t, service.PreWssConsumeQuota(c, &free, usage))
	service.PostWssConsumeQuota(c, &free, "capture-audio", usage, "")
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, before, user.Quota, "captured zero prices remain free")
}

func TestCatalogRequestCanonicalAndProviderInputs(t *testing.T) {
	catalogPricingPostgres(t, map[string]string{
		"ModelRatio":                         `{"qwen3-max@effort:high@thinking:on":4,"qwen3-max@thinking:on":3}`,
		"ModelPrice":                         `{"grok-imagine-image":1}`,
		"billing_setting.billing_mode":       `{}`,
		"molii_grok_price.image_standard_1k": "0.02",
		"starai_video_price.standard_720p":   "46",
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	origin := "qwen3-max@thinking:on@effort:high@temperature:0.2"
	info := &relaycommon.RelayInfo{OriginModelName: origin, UsingGroup: "default", UserGroup: "default", ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-image", IsModelMapped: true}}
	_, err := helper.ModelPriceHelper(c, info, 100, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	assert.Equal(t, "qwen3-max@effort:high@thinking:on", info.BillingModelName)
	assert.Equal(t, origin, info.OriginModelName)
	assert.Equal(t, "grok-imagine-image", info.UpstreamModelName)
	assert.Equal(t, float64(4), info.PriceData.ModelRatio)
	anchor, ok := info.PricingSelection.Model(info.UpstreamModelName)
	require.True(t, ok)
	assert.Equal(t, float64(1), anchor.Price)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelRatio": `{"qwen3-max@effort:high@thinking:on":9}`, "ModelPrice": `{"grok-imagine-image":2}`, "molii_grok_price.image_standard_1k": "0.07", "starai_video_price.standard_720p": "49"}))
	oldSelection := info.PricingSelection
	grok := oldSelection.GrokPrices()
	output, _, ok := grok.ImagePricesForQuality("grok-imagine-image", "1k", "")
	require.True(t, ok)
	assert.Equal(t, 0.02, output)
	seedance, ok := oldSelection.SeedancePrices().VideoPrice("doubao-seedance-2-0-260128", "720p", false)
	require.True(t, ok)
	assert.Equal(t, float64(46), seedance)
	anchor.Price = 99
	grok.ImageStandard1K = 99
	same, ok := oldSelection.Model(info.UpstreamModelName)
	require.True(t, ok)
	assert.Equal(t, float64(1), same.Price, "accessors return detached scalar values")
	_, err = helper.ModelPriceHelper(c, info, 100, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	assert.Equal(t, float64(4), info.PriceData.ModelRatio, "same request retains selected legacy prices")
	require.NoError(t, helper.CaptureRequestPricing(c, info), "explicit new attempt may select a later generation")
	_, err = helper.ModelPriceHelper(c, info, 100, &kittypes.TokenCountMeta{})
	require.NoError(t, err)
	assert.Equal(t, float64(9), info.PriceData.ModelRatio)
	newAnchor, ok := info.PricingSelection.Model(info.UpstreamModelName)
	require.True(t, ok)
	assert.Equal(t, float64(2), newAnchor.Price)
	oldModel, ok := oldSelection.Model(info.BillingModelName)
	require.True(t, ok)
	assert.Equal(t, float64(4), oldModel.Ratio)
}
