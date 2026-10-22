package relay

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
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
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func catalogPricingPostgres(t *testing.T, options map[string]string) *gorm.DB {
	t.Helper()
	require.NoError(t, i18n.Init())
	u, err := catalogPricingPostgresDSN()
	require.NoError(t, err)
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(u.String()), cfg)
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

func catalogPricingPostgresDSN() (*url.URL, error) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	u, err := url.Parse(dsn)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || !net.ParseIP(u.Hostname()).IsLoopback() || u.User == nil || u.User.Username() == "" || u.Path == "" || u.Path == "/" || strings.Contains(u.Path[1:], "/") || u.Fragment != "" || u.Query().Get("sslmode") != "disable" {
		return nil, fmt.Errorf("catalog PostgreSQL tests require a loopback TEST_POSTGRES_DSN URL with sslmode=disable")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query) != 1 || len(query["sslmode"]) != 1 || query["sslmode"][0] != "disable" {
		return nil, fmt.Errorf("catalog PostgreSQL tests reject ambiguous TEST_POSTGRES_DSN overrides")
	}
	parsed, err := pgx.ParseConfig(dsn)
	if err != nil || parsed.Host != u.Hostname() || parsed.Database != u.Path[1:] || parsed.TLSConfig != nil {
		return nil, fmt.Errorf("catalog PostgreSQL tests reject ambiguous TEST_POSTGRES_DSN overrides")
	}
	for _, fallback := range parsed.Fallbacks {
		if fallback.Host != parsed.Host || fallback.Port != parsed.Port {
			return nil, fmt.Errorf("catalog PostgreSQL tests reject TEST_POSTGRES_DSN host fallbacks")
		}
	}
	return u, nil
}

func TestCatalogPricingPostgresDSNGuard(t *testing.T) {
	const base = "postgresql://fixture@127.0.0.1:5432/postgres?sslmode=disable"
	t.Setenv("TEST_POSTGRES_DSN", base)
	u, err := catalogPricingPostgresDSN()
	require.NoError(t, err)
	u.Path = "/catalog_request_guard"
	parsed, err := pgx.ParseConfig(u.String())
	require.NoError(t, err)
	require.Equal(t, "catalog_request_guard", parsed.Database)
	for _, query := range []string{
		"&dbname=postgres", "&database=postgres", "&port=5432",
		"&service=fixture", "&host=127.0.0.1", "&sslmode=disable",
	} {
		t.Run(query, func(t *testing.T) {
			t.Setenv("TEST_POSTGRES_DSN", base+query)
			_, err := catalogPricingPostgresDSN()
			require.Error(t, err, "query overrides must not survive the owned database path rewrite")
		})
	}
	for _, dsn := range []string{
		"", "mysql://fixture@127.0.0.1/postgres?sslmode=disable",
		"postgresql://fixture@localhost/postgres?sslmode=disable",
		"postgresql://127.0.0.1/postgres?sslmode=disable",
		"postgresql://fixture@127.0.0.1/?sslmode=disable",
		"postgresql://fixture@127.0.0.1/nested/postgres?sslmode=disable",
		base + "#fragment", base + "&malformed=%",
	} {
		t.Run(dsn, func(t *testing.T) {
			t.Setenv("TEST_POSTGRES_DSN", dsn)
			_, err := catalogPricingPostgresDSN()
			require.Error(t, err)
		})
	}
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
	service.PostAudioConsumeQuota(c, info, usage, nil)
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
	service.PostWssConsumeQuota(c, info, "different-upstream-label", usage, nil)
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
	service.PostWssConsumeQuota(c, &free, "capture-audio", usage, nil)
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

func TestCatalogBuiltinFallbackCurrencyBoundary(t *testing.T) {
	const name = "deepseek-v4-flash"
	builtin, ok := billing_setting.GetBuiltinBillingExpr(name)
	require.True(t, ok)
	for _, tc := range []struct {
		name, currency, expression, pluginExpression string
		configured, blocked                          bool
	}{
		{name: "CNY_unconfigured_fails_closed", currency: "CNY", blocked: true},
		{name: "USD_unconfigured_uses_builtin", currency: "USD"},
		{name: "CNY_explicit_custom_quote", currency: "CNY", expression: "p * 2", configured: true},
		{name: "CNY_explicit_quote_identical_to_builtin", currency: "CNY", expression: builtin, configured: true},
		{name: "CNY_provider_override_bypasses_unused_builtin", currency: "CNY", pluginExpression: `u("seconds") * 3`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := map[string]string{
				"ModelPrice": "{}", "ModelRatio": "{}", "USDExchangeRate": "7",
				"billing_setting.billing_mode": "{}", "billing_setting.billing_expr": "{}",
				billing_setting.PluginBillingExprOption: "{}",
			}
			if tc.configured {
				raw, err := common.Marshal(map[string]string{name: tc.expression})
				require.NoError(t, err)
				options["billing_setting.billing_expr"] = string(raw)
			}
			if tc.pluginExpression != "" {
				raw, err := common.Marshal(map[string]string{"currency-probe::" + name: tc.pluginExpression})
				require.NoError(t, err)
				options[billing_setting.PluginBillingExprOption] = string(raw)
			}
			db := catalogPricingPostgres(t, options)
			var provider *jsplugin.LoadedPlugin
			if tc.pluginExpression != "" {
				for _, key := range []string{"currency-probe", "currency-probe-inherited"} {
					source := strings.NewReplacer("bill-fallback", key, "declared-model", name).Replace(billingFallbackPlugin)
					source = strings.Replace(source, `fetchMode:"per_task"`, `fetchMode:"per_task",usageSchema:{seconds:{type:"number",unit:"second"}}`, 1) + `export function extractUsage(){return {seconds:2};}`
					plugin, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
					require.NoError(t, err)
					if key == "currency-probe" {
						provider = plugin
					}
					require.NoError(t, db.Create(&model.TaskPlugin{Key: key, Version: plugin.Meta.Version, APIVersion: 1,
						Source: model.LongText(source), SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(source))), Active: true, Enabled: true}).Error)
				}
			}
			vendor := model.Vendor{Name: "currency-probe", Status: 1}
			require.NoError(t, db.Create(&vendor).Error)
			entry := model.Model{ModelName: name, BillingCurrency: tc.currency, Status: 1, MarketplaceEnabled: true,
				VendorID: vendor.Id, DisplayName: "Currency probe", Description: "Complete fixture", ReleaseDate: "2026-10-17",
				InputModalities: []string{"text"}, OutputModalities: []string{"text"}, Capabilities: []string{"chat"},
				SupportedParameters: []string{"temperature"}, ContextLength: 1000000, MaxOutputTokens: 1000}
			require.True(t, entry.EvaluateMarketplaceReadiness().Complete)
			require.NoError(t, db.Create(&entry).Error)
			channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "fixture", Models: name}
			if provider != nil {
				channel.Type = constant.ChannelTypeTaskPlugin
			}
			require.NoError(t, db.Create(&channel).Error)
			require.NoError(t, db.Create(&model.Ability{Group: "default", Model: name, ChannelId: channel.Id, Enabled: true}).Error)
			require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{}`))
			info := &relaycommon.RelayInfo{OriginModelName: name, UsingGroup: "default", UserGroup: "default"}
			if tc.pluginExpression != "" {
				c, info = catalogTaskContext(name)
				c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Plugin: provider, Generation: jsplugin.DefaultRegistry.Generation()})
				_, taskErr := EstimateTaskSubmit(c, info)
				require.Nil(t, taskErr)
				selected, ok := info.PricingSelection.Task()
				require.True(t, ok)
				assert.Equal(t, tc.pluginExpression, selected.Expression)
				assert.True(t, selected.UseExpression)
				require.NotNil(t, info.TieredBillingSnapshot)
				assert.Equal(t, "CNY", info.TieredBillingSnapshot.SourceCurrency)
				assert.Equal(t, float64(6), info.TieredBillingSnapshot.EstimatedSourceCost)
				assert.InDelta(t, 6.0/7, info.TieredBillingSnapshot.EstimatedCostUSD, 1e-12)
			} else {
				_, err := helper.ModelPriceHelper(c, info, 1000000, &kittypes.TokenCountMeta{})
				if tc.blocked {
					require.ErrorContains(t, err, "explicit")
					assert.Nil(t, info.PricingSelection, "reject before freezing a false CNY quote")
					assert.Nil(t, info.TieredBillingSnapshot)
					assert.Zero(t, info.FinalPreConsumedQuota)
				} else {
					require.NoError(t, err)
					require.NotNil(t, info.TieredBillingSnapshot)
					assert.Equal(t, tc.currency, info.TieredBillingSnapshot.SourceCurrency)
					expected := builtin
					if tc.configured {
						expected = tc.expression
					}
					assert.Equal(t, expected, info.TieredBillingSnapshot.ExprString)
					money := info.TieredBillingSnapshot
					if tc.currency == "CNY" {
						assert.InDelta(t, money.EstimatedSourceCost/7, money.EstimatedCostUSD, 1e-12)
					} else {
						assert.Equal(t, money.EstimatedSourceCost, money.EstimatedCostUSD)
					}
				}
			}
			snapshot, err := model.GetModelPricingSnapshot([]string{name})
			require.NoError(t, err)
			require.Len(t, snapshot.Entries, 1)
			assert.Equal(t, tc.currency, snapshot.Entries[0].BillingCurrency)
			if tc.currency == "CNY" && !tc.configured {
				assert.NotContains(t, snapshot.Entries[0].Effective, "billing_setting.billing_expr")
				var expressions map[string]string
				require.NoError(t, common.Unmarshal([]byte(snapshot.Options["billing_setting.billing_expr"]), &expressions))
				assert.NotContains(t, expressions, name, "compatibility editor must not inject a USD quote as CNY")
				filtered, err := model.GetModelPricingSnapshot([]string{"unrelated-model"})
				require.NoError(t, err)
				require.NoError(t, common.Unmarshal([]byte(filtered.Options["billing_setting.billing_expr"]), &expressions))
				assert.NotContains(t, expressions, name, "full Options must honor currency even outside the requested entries")
				if provider == nil {
					assert.Empty(t, model.GetPricing(), "complete metadata alone must not publish an unconfigured CNY quote")
					preview, err := model.PreviewModelPricing(name, model.PricingValues{})
					require.NoError(t, err)
					assert.NotContains(t, preview, "billing_setting.billing_expr")
					all, err := model.GetModelPricingSnapshot(nil)
					require.NoError(t, err)
					editable := false
					for _, entry := range all.Entries {
						editable = editable || entry.ModelName == name && entry.HasMetadata
					}
					assert.True(t, editable, "unconfigured CNY metadata remains editable")
				} else {
					published := model.GetPricing()
					require.Len(t, published, 1)
					assert.Empty(t, published[0].BillingExpr)
					require.Len(t, published[0].BillingPluginVariants, 2)
					quotes := map[string]string{}
					for _, variant := range published[0].BillingPluginVariants {
						quotes[variant.PluginKey] = variant.BillingExpr
					}
					assert.Equal(t, tc.pluginExpression, quotes["currency-probe"])
					assert.Empty(t, quotes["currency-probe-inherited"], "an unused provider must not publish the USD fallback as CNY")
				}
			} else {
				assert.Contains(t, snapshot.Entries[0].Effective, "billing_setting.billing_expr")
				published := model.GetPricing()
				require.Len(t, published, 1)
				assert.Equal(t, tc.currency, published[0].BillingCurrency)
				assert.NotEmpty(t, published[0].BillingExpr)
			}
		})
	}
	t.Run("unused_CNY_builtin_candidate_does_not_block_selected_legacy_price", func(t *testing.T) {
		db := catalogPricingPostgres(t, map[string]string{
			"ModelRatio": `{"selected-legacy":2}`, "ModelPrice": "{}",
			"billing_setting.billing_mode": "{}", "billing_setting.billing_expr": "{}",
		})
		require.NoError(t, db.Create(&model.Model{ModelName: name, BillingCurrency: "CNY"}).Error)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
		info := &relaycommon.RelayInfo{OriginModelName: "selected-legacy", UsingGroup: "default", UserGroup: "default",
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: name, IsModelMapped: true}}
		_, err := helper.ModelPriceHelper(c, info, 100, &kittypes.TokenCountMeta{})
		require.NoError(t, err)
		assert.Equal(t, float64(2), info.PriceData.ModelRatio)
		assert.Nil(t, info.TieredBillingSnapshot)
	})
	for _, tc := range []struct {
		name, aliasCurrency, targetCurrency, overrideModel string
		blocked                                            bool
	}{
		{name: "default_alias_to_CNY_builtin", targetCurrency: "CNY", blocked: true},
		{name: "CNY_alias_to_USD_builtin", aliasCurrency: "CNY", targetCurrency: "USD"},
		{name: "CNY_alias_to_implicit_USD_builtin", aliasCurrency: "CNY"},
		{name: "alias_override_keeps_alias_currency", aliasCurrency: "CNY", targetCurrency: "USD", overrideModel: "currency-alias"},
		{name: "mapped_override_keeps_alias_currency", aliasCurrency: "CNY", targetCurrency: "USD", overrideModel: name},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const alias = "currency-alias"
			const quote = `u("seconds") * 3`
			options := map[string]string{"ModelRatio": "{}", "ModelPrice": "{}", "billing_setting.billing_mode": "{}", "billing_setting.billing_expr": "{}"}
			if tc.overrideModel != "" {
				raw, err := common.Marshal(map[string]string{"currency-alias-probe::" + tc.overrideModel: quote})
				require.NoError(t, err)
				options[billing_setting.PluginBillingExprOption] = string(raw)
			}
			db := catalogPricingPostgres(t, options)
			source := strings.NewReplacer("bill-fallback", "currency-alias-probe", "declared-model", name).Replace(billingFallbackPlugin)
			source = strings.Replace(source, `fetchMode:"per_task"`, `fetchMode:"per_task",usageSchema:{seconds:{type:"number",unit:"second"}}`, 1)
			plugin, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
			require.NoError(t, err)
			require.NoError(t, db.Create(&model.TaskPlugin{Key: plugin.Meta.Key, Version: plugin.Meta.Version, APIVersion: 1,
				Source: model.LongText(source), SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(source))), Active: true, Enabled: true}).Error)
			if tc.aliasCurrency != "" {
				require.NoError(t, db.Create(&model.Model{ModelName: alias, BillingCurrency: tc.aliasCurrency}).Error)
			}
			if tc.targetCurrency != "" {
				require.NoError(t, db.Create(&model.Model{ModelName: name, BillingCurrency: tc.targetCurrency}).Error)
			}
			mapping := `{"currency-alias":"deepseek-v4-flash"}`
			channel := model.Channel{Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Key: "fixture", Models: alias, ModelMapping: &mapping}
			require.NoError(t, db.Create(&channel).Error)
			require.NoError(t, db.Create(&model.Ability{Group: "default", Model: alias, ChannelId: channel.Id, Enabled: true}).Error)
			require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/videos", nil)
			info := &relaycommon.RelayInfo{OriginModelName: alias, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: name, IsModelMapped: true}}
			err = helper.CaptureTaskPricing(c, info, plugin.Meta.Key)
			published := model.GetPricing()
			require.Len(t, published, 1, "task aliases retain their baseline unconfigured visibility")
			if tc.blocked {
				require.ErrorContains(t, err, "explicit")
				assert.Empty(t, published[0].BillingExpr, "a CNY target's USD fallback cannot be advertised under alias metadata")
			} else {
				require.NoError(t, err)
				selected, ok := info.PricingSelection.Task()
				require.True(t, ok)
				money, ok := info.PricingSelection.Model(selected.BillingModelName)
				require.True(t, ok)
				assert.Equal(t, string(money.Money.SourceCurrency), published[0].BillingCurrency)
				if tc.overrideModel != "" {
					assert.Equal(t, alias, selected.BillingModelName)
					assert.Equal(t, quote, published[0].BillingExpr)
				} else {
					assert.Equal(t, name, selected.BillingModelName)
					assert.Equal(t, builtin, published[0].BillingExpr)
				}
			}
		})
	}
	t.Run("fallback_provenance_respects_presence_mode_and_legacy_precedence", func(t *testing.T) {
		catalogPricingPostgres(t, map[string]string{"ModelRatio": "{}", "ModelPrice": "{}"})
		for _, tc := range []struct {
			name, mode, expression, legacyKey string
			hasMode, hasExpression, fallback  bool
		}{
			{name: "no_config", fallback: true},
			{name: "explicit_tiered_mode", hasMode: true, mode: "tiered_expr", fallback: true},
			{name: "explicit_ratio_mode", hasMode: true, mode: "ratio"},
			{name: "explicit_blank_mode", hasMode: true},
			{name: "explicit_blank_expression", hasExpression: true},
			{name: "explicit_invalid_expression", hasExpression: true, expression: "invalid("},
			{name: "explicit_identical_expression", hasExpression: true, expression: builtin},
			{name: "configured_free_ratio", legacyKey: "ratio"},
			{name: "configured_free_price", legacyKey: "price"},
			{name: "explicit_tiered_overrides_legacy", hasMode: true, mode: "tiered_expr", legacyKey: "price", fallback: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				settings := billing_setting.BillingSetting{BillingMode: map[string]string{}, BillingExpr: map[string]string{}}
				if tc.hasMode {
					settings.BillingMode[name] = tc.mode
				}
				if tc.hasExpression {
					settings.BillingExpr[name] = tc.expression
				}
				billing_setting.PublishBillingSetting(settings)
				ratios, prices := "{}", "{}"
				if tc.legacyKey == "ratio" {
					ratios = `{"deepseek-v4-flash":0}`
				}
				if tc.legacyKey == "price" {
					prices = `{"deepseek-v4-flash":0}`
				}
				require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(ratios))
				require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(prices))
				assert.Equal(t, tc.fallback, billing_setting.IsBuiltinBillingExprFallback(name))
				if tc.hasExpression {
					expression, ok := billing_setting.GetBillingExpr(name)
					require.True(t, ok)
					assert.Equal(t, tc.expression, expression, "blank/invalid configured values must never fall back")
				}
			})
		}
	})
}
