package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relay/channel/moliigrok"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Controller TestMain opens no DB. Every persistent check here requires a
// loopback PostgreSQL server and owns a freshly created, exactly dropped DB.
func catalogImagePostgres(t *testing.T, options map[string]string) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	require.NotEmpty(t, dsn, "task PostgreSQL is required")
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname())
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	require.NoError(t, err)
	name := fmt.Sprintf("catalog_image_%d", time.Now().UnixNano())
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
	previousType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousBatch, previousLogs, previousCache := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.MemoryCacheEnabled
	model.DB, model.LOG_DB, jsplugin.DefaultRegistry = db, db, jsplugin.NewRegistry()
	common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.MemoryCacheEnabled = false, false, true, false
	common.SetMainDatabaseType(common.DatabaseTypePostgreSQL)
	common.SetLogDatabaseType(common.DatabaseTypePostgreSQL)
	t.Setenv("LOG_SQL_DSN", "")
	master := common.IsMasterNode
	common.IsMasterNode = false
	err = model.InitLogDB()
	common.IsMasterNode = master
	require.NoError(t, err)
	t.Cleanup(func() {
		model.DB, model.LOG_DB, jsplugin.DefaultRegistry = previousDB, previousLogDB, previousRegistry
		common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.MemoryCacheEnabled = previousRedis, previousBatch, previousLogs, previousCache
		common.SetMainDatabaseType(previousType)
		common.SetLogDatabaseType(previousLogType)
	})
	require.NoError(t, db.AutoMigrate(&model.Model{}, &model.Vendor{}, &model.Option{}, &model.Channel{}, &model.Ability{}, &model.Task{}, &model.TaskPlugin{}, &model.Midjourney{}, &model.SystemTask{}, &model.User{}, &model.Token{}, &model.Log{}, &model.MoliiFile{}))
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

// A second image attempt must capture its newly mapped channel, and must not
// retain an expression snapshot when the new channel uses legacy pricing.
func TestCatalogImageRetrySelectsNewMappedPricing(t *testing.T) {
	db := catalogImagePostgres(t, map[string]string{
		"ModelPrice": `{"upstream-c":0.2}`, "GroupRatio": `{"default":1}`, "quota_setting.trust_quota_usd": "0",
		"billing_setting.billing_mode": `{"upstream-b":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"upstream-b":"tier(\"request\", fixed(0.1)) * image_count"}`,
	})
	priorityB, priorityC, weight := int64(10), int64(0), uint(100)
	baseB, baseC := "https://channel-b.example", "https://channel-c.example"
	mappingB, mappingC := `{"image-routing-test":"upstream-b"}`, `{"image-routing-test":"upstream-c"}`
	channelB := model.Channel{Type: constant.ChannelTypeOpenAI, Key: "key-b", Status: common.ChannelStatusEnabled, Name: "channel-b", Weight: &weight, Models: "image-routing-test", Group: "default", Priority: &priorityB, BaseURL: &baseB, ModelMapping: &mappingB}
	channelC := model.Channel{Type: constant.ChannelTypeOpenAI, Key: "key-c", Status: common.ChannelStatusEnabled, Name: "channel-c", Weight: &weight, Models: "image-routing-test", Group: "default", Priority: &priorityC, BaseURL: &baseC, ModelMapping: &mappingC}
	for _, channel := range []*model.Channel{&channelB, &channelC} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "image-routing-test", ChannelId: channel.Id, Enabled: true, Priority: channel.Priority, Weight: weight}).Error)
	}
	c, info, request := catalogImageRequest(t, db, `{"model":"image-routing-test","prompt":"cat"}`, "image-routing-test", "upstream-b", constant.ChannelTypeOpenAI, false)
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, &channelB, info.OriginModelName))
	retry := 0
	retryParam := &service.RetryParam{Ctx: c, TokenGroup: "default", ModelName: info.OriginModelName, RequestPath: c.Request.URL.Path, Retry: &retry}
	selected, apiErr := getChannel(c, info, retryParam)
	require.Nil(t, apiErr)
	require.Equal(t, channelB.Id, selected.Id)
	require.Nil(t, prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1))
	require.NotNil(t, info.TieredBillingSnapshot)
	oldSelection, oldSnapshot := info.PricingSelection, info.TieredBillingSnapshot
	assert.Equal(t, 50000, info.Billing.GetPreConsumedQuota())
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{"upstream-c":0.4}`, "billing_setting.billing_expr": `{"upstream-b":"tier(\"request\", fixed(0.3)) * image_count"}`}))
	retryParam.IncreaseRetry()
	selected, apiErr = getChannel(c, info, retryParam)
	require.Nil(t, apiErr)
	require.Equal(t, channelC.Id, selected.Id)
	require.Nil(t, prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1))
	assert.Equal(t, "image-routing-test", info.OriginModelName)
	assert.Equal(t, "upstream-c", info.BillingModelName)
	assert.Equal(t, "upstream-c", request.Model)
	assert.Equal(t, "key-c", info.ApiKey)
	assert.Equal(t, baseC, info.ChannelBaseUrl)
	assert.Equal(t, channelC.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
	assert.Nil(t, info.TieredBillingSnapshot, "legacy retry must not settle the previous expression")
	assert.NotSame(t, oldSelection, info.PricingSelection)
	assert.Equal(t, `tier("request", fixed(0.1)) * image_count`, oldSnapshot.ExprString)
	assert.Equal(t, 50000, oldSnapshot.EstimatedQuotaAfterGroup)
	assert.Equal(t, 200000, info.Billing.GetPreConsumedQuota())
	require.Nil(t, service.PrepareImageBillingForRequest(c, info, 2))
	assert.Equal(t, 400000, info.Billing.GetPreConsumedQuota(), "new legacy price must also govern outbound quantity reservation")
	info.UpdateImageCount(1)
	service.PostTextConsumeQuota(c, info, &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil)
	var log model.Log
	require.NoError(t, db.Where("user_id = ?", info.UserId).Last(&log).Error)
	assert.Equal(t, 200000, log.Quota)
	assert.Equal(t, "upstream-c", log.ModelName, "preserve existing billed-model log identity")
	var user model.User
	require.NoError(t, db.First(&user, info.UserId).Error)
	assert.Equal(t, 1800000, user.Quota)
}

func TestCatalogImagePendingRejectsBeforeMedia(t *testing.T) {
	db := catalogImagePostgres(t, map[string]string{"ModelPrice": `{"grok-imagine-image":1}`, "billing_setting.billing_mode": `{}`, "GroupRatio": `{"default":1}`})
	c, info, request := catalogImageRequest(t, db, `{"model":"grok-imagine-image","prompt":"cat"}`, "grok-imagine-image", "grok-imagine-image", constant.ChannelTypeMoliiGrokAIGC, false)
	require.Nil(t, prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1))
	savedSelection := info.PricingSelection
	failure := errors.New("required image pricing projection failed")
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("image-pending", func(tx *gorm.DB) {
		if tx.Statement.Table == "vendors" && tx.Statement.ConnPool == db.Statement.ConnPool {
			tx.AddError(failure)
		}
	}))
	err := model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{"grok-imagine-image":2}`, "molii_grok_price.image_standard_1k": "0.2"})
	require.NoError(t, db.Callback().Query().Remove("image-pending"))
	require.ErrorIs(t, err, failure)
	c2, blocked, request2 := catalogImageRequest(t, db, `{"model":"grok-imagine-image","prompt":"edit","image":{"file_id":"file_missing"}}`, "grok-imagine-image", "grok-imagine-image", constant.ChannelTypeMoliiGrokAIGC, true)
	// Even a stale previous attempt selection cannot bypass the pending gate.
	blocked.PricingSelection = savedSelection
	apiErr := prepareSelectedImageBilling(c2, blocked, request2, request2.GetTokenCountMeta(), 1)
	require.NotNil(t, apiErr)
	require.ErrorIs(t, apiErr.Err, model.ErrCatalogPublicationPending)
	assert.Nil(t, blocked.PricingSelection)
	assert.Nil(t, blocked.GrokImageBilling)
	assert.Nil(t, blocked.Billing)
	catalogImageResponse(t, c, info, request)
	assert.Same(t, savedSelection, info.PricingSelection)
	assert.InDelta(t, 0.02, info.GrokImageBilling.OutputUnitPrice, 1e-12)
	require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
	apiErr = prepareSelectedImageBilling(c2, blocked, request2, request2.GetTokenCountMeta(), 1)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCode("file_not_found"), apiErr.GetErrorCode(), "recovered capture now reaches actual media processing")
	assert.Nil(t, blocked.GrokImageBilling)
	assert.Nil(t, blocked.Billing)
}

func TestCatalogImageZeroAndAnchorStaySelected(t *testing.T) {
	db := catalogImagePostgres(t, map[string]string{
		"ModelPrice": `{"grok-imagine-image":1}`, "billing_setting.billing_mode": `{}`, "GroupRatio": `{"default":1}`,
		"molii_grok_price.image_standard_1k": "0", "molii_grok_price.image_standard_input": "0",
	})
	body := `{"model":"grok-imagine-image","prompt":"cat"}`
	c, info, request := catalogImageRequest(t, db, body, "grok-imagine-image", "grok-imagine-image", constant.ChannelTypeMoliiGrokAIGC, false)
	require.Nil(t, prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1))
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"molii_grok_price.image_standard_1k": "0.2", "molii_grok_price.image_standard_input": "0.02"}))
	ratios, err := (&moliigrok.Adaptor{}).EstimateImageBilling(c, info, *request)
	require.NoError(t, err)
	assert.Zero(t, ratios["molii_grok_direct_cost"])
	assert.Zero(t, info.GrokImageBilling.OutputUnitPrice)
	assert.Zero(t, info.GrokImageBilling.InputUnitPrice)
	catalogImageResponse(t, c, info, request)
	var log model.Log
	require.NoError(t, db.Where("user_id = ?", info.UserId).Last(&log).Error)
	assert.Zero(t, log.Quota, "captured zero direct rates remain free after publication")
	var user model.User
	require.NoError(t, db.First(&user, info.UserId).Error)
	assert.Equal(t, 2000000, user.Quota)

	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{"grok-imagine-image":0}`}))
	c2, zero, request2 := catalogImageRequest(t, db, body, "grok-imagine-image", "grok-imagine-image", constant.ChannelTypeMoliiGrokAIGC, false)
	apiErr := prepareSelectedImageBilling(c2, zero, request2, request2.GetTokenCountMeta(), 1)
	require.NotNil(t, apiErr)
	assert.Contains(t, apiErr.Error(), "anchor is invalid")
	assert.Nil(t, zero.Billing)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{"grok-imagine-image":3}`}))
	_, err = (&moliigrok.Adaptor{}).EstimateImageBilling(c2, zero, *request2)
	require.ErrorContains(t, err, "anchor is invalid", "captured explicit zero cannot become configured or default price")

	// The supported Grok names have built-in anchors. Removing a configured
	// anchor must select that same-generation default, without reading later 3.
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{}`}))
	zero.PricingSelection = nil
	require.NoError(t, helper.CaptureRequestPricing(c2, zero))
	selected, ok := zero.PricingSelection.Model("grok-imagine-image")
	require.True(t, ok)
	assert.False(t, selected.HasPrice)
	assert.True(t, selected.HasDefaultPrice)
	assert.False(t, selected.HasExpression)
	assert.Empty(t, selected.Expression)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{"grok-imagine-image":3}`}))
	ratios, err = (&moliigrok.Adaptor{}).EstimateImageBilling(c2, zero, *request2)
	require.NoError(t, err)
	assert.InDelta(t, zero.GrokImageBilling.CostUSD, ratios["molii_grok_direct_cost"], 1e-12, "selected default anchor is 1")

	// Defensive selected-state cases cannot arise from today's Grok built-ins,
	// but must fail closed instead of filling holes with current live prices.
	for _, tc := range []struct {
		name   string
		values map[string]relaycommon.ModelPricing
		prices ratio_setting.MoliiGrokPriceSetting
		want   string
	}{
		{"missing-model", nil, ratio_setting.DefaultMoliiGrokPriceSetting(), "outside the request pricing selection"},
		{"missing-anchor", map[string]relaycommon.ModelPricing{"grok-imagine-image": {}}, ratio_setting.DefaultMoliiGrokPriceSetting(), "anchor is invalid"},
		{"empty-money", map[string]relaycommon.ModelPricing{"grok-imagine-image": {Price: 1, HasPrice: true}}, ratio_setting.DefaultMoliiGrokPriceSetting(), "cost is invalid"},
		{"invalid-direct", map[string]relaycommon.ModelPricing{"grok-imagine-image": selected}, ratio_setting.MoliiGrokPriceSetting{ImageStandard1K: -1}, "pricing is not configured"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			zero.PricingSelection = relaycommon.NewRequestPricingSelection(tc.values, operation_setting.ToolPrices{}, tc.prices, ratio_setting.StarAIVideoPriceSetting{})
			_, err := (&moliigrok.Adaptor{}).EstimateImageBilling(c2, zero, *request2)
			require.ErrorContains(t, err, tc.want)
		})
	}
	zero.PricingSelection = nil
	ratios, err = (&moliigrok.Adaptor{}).EstimateImageBilling(c2, zero, *request2)
	require.NoError(t, err, "historical direct callers retain the explicit no-selection compatibility path")
	assert.InDelta(t, 0.2/3, ratios["molii_grok_direct_cost"], 1e-12)
	assert.Nil(t, zero.PricingSelection, "compatibility does not introduce a separately captured half-attempt")
}

func TestCatalogImageQualityCountAndQuotaBounds(t *testing.T) {
	db := catalogImagePostgres(t, map[string]string{"ModelPrice": `{"grok-imagine-image-2.0":1}`, "billing_setting.billing_mode": `{}`, "GroupRatio": `{"default":1}`, "molii_grok_price.image_20_low_2k": "0.11"})
	for _, tc := range []struct {
		name, extra, wantError string
		wantPrice              float64
	}{
		{"medium-default", ``, "", 0.06},
		{"low-2k", `,"quality":"low","resolution":"2k"`, "", 0.11},
		{"invalid-quality", `,"quality":"high"`, "quality must be", 0},
		{"zero-count", `,"n":0`, "n must be", 0},
		{"excess-count", `,"n":5`, "n must be", 0},
		{"unsupported-resolution", `,"resolution":"8k"`, "pricing is not configured", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"grok-imagine-image-2.0","prompt":"cat"` + tc.extra + `}`
			c, info, request := catalogImageRequest(t, db, body, "grok-imagine-image-2.0", "grok-imagine-image-2.0", constant.ChannelTypeMoliiGrokAIGC, false)
			apiErr := prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1)
			if tc.wantError != "" {
				require.NotNil(t, apiErr)
				assert.Contains(t, apiErr.Error(), tc.wantError)
				assert.Nil(t, info.Billing)
				return
			}
			require.Nil(t, apiErr)
			assert.Equal(t, tc.wantPrice, info.GrokImageBilling.OutputUnitPrice)
			catalogImageResponse(t, c, info, request)
		})
	}
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"molii_grok_price.image_20_medium_1k": "1000000000000"}))
	c, info, request := catalogImageRequest(t, db, `{"model":"grok-imagine-image-2.0","prompt":"cat"}`, "grok-imagine-image-2.0", "grok-imagine-image-2.0", constant.ChannelTypeMoliiGrokAIGC, false)
	apiErr := prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1)
	require.NotNil(t, apiErr)
	assert.Nil(t, info.Billing, "oversized quota cannot be reserved or wrap into credit")
	var user model.User
	require.NoError(t, db.First(&user, info.UserId).Error)
	assert.Equal(t, 2000000, user.Quota)
}

func catalogImageRequest(t *testing.T, db *gorm.DB, body, origin, upstream string, channelType int, edit bool) (*gin.Context, *relaycommon.RelayInfo, *dto.ImageRequest) {
	t.Helper()
	user := model.User{Username: fmt.Sprintf("image-%d", time.Now().UnixNano()), AffCode: fmt.Sprintf("%x", time.Now().UnixNano()), Quota: 2000000}
	require.NoError(t, db.Create(&user).Error)
	path, mode := "/v1/images/generations", relayconstant.RelayModeImagesGenerations
	if edit {
		path, mode = "/v1/images/edits", relayconstant.RelayModeImagesEdits
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyRequestBody, []byte(body))
	c.Set("model_mapping", fmt.Sprintf(`{%q:%q}`, origin, upstream))
	common.SetContextKey(c, constant.ContextKeyChannelType, channelType)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, origin)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	t.Cleanup(func() { common.CleanupBodyStorage(c) })
	request := &dto.ImageRequest{}
	require.NoError(t, common.UnmarshalJsonStr(body, request))
	info := &relaycommon.RelayInfo{OriginModelName: origin, RelayMode: mode, Request: request, RequestURLPath: path,
		UsingGroup: "default", UserGroup: "default", TokenGroup: "default", UserId: user.Id, UserQuota: user.Quota,
		IsPlayground: true, StartTime: time.Now(), UserSetting: dto.UserSetting{BillingPreference: "wallet_only"}}
	return c, info, request
}

func catalogImageResponse(t *testing.T, c *gin.Context, info *relaycommon.RelayInfo, request *dto.ImageRequest) {
	t.Helper()
	a := &moliigrok.Adaptor{}
	a.Init(info)
	_, err := a.ConvertImageRequest(c, info, *request)
	require.NoError(t, err)
	response := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"url":"https://imgen.x.ai/test.png"}]}`))}
	usage, apiErr := a.DoResponse(c, response, info)
	require.Nil(t, apiErr)
	actualUsage := usage.(*dto.Usage)
	actualUsage.PromptTokens, actualUsage.TotalTokens = 1, 1
	service.PostTextConsumeQuota(c, info, actualUsage, nil)
}

// Catches rates selected before media combined with anchor/currency/base read
// after publication. All production consumers and the wallet are real.
func TestCatalogImageMediaPublicationOverlap(t *testing.T) {
	db := catalogImagePostgres(t, map[string]string{
		"ModelPrice": `{"grok-imagine-image-quality":1}`, "billing_setting.billing_mode": `{}`,
		"molii_grok_price.image_quality_1k": "0.14", "molii_grok_price.image_quality_input": "0.07",
		"USDExchangeRate": "7", "quota_setting.trust_quota_usd": "0", "GroupRatio": `{"default":1}`,
		"COSBucket": "catalog-test", "COSRegion": "test", "COSSecretID": "test-only", "COSSecretKey": "test-only",
	})
	entry := model.Model{ModelName: "grok-imagine-image-quality", BillingCurrency: "USD"}
	require.NoError(t, entry.Insert())
	before, err := model.GetModelPricingSnapshot([]string{entry.ModelName})
	require.NoError(t, err)
	body := `{"model":"grok-imagine-image-quality","prompt":"edit","n":2,"image":{"file_id":"file_catalog"}}`
	c, info, request := catalogImageRequest(t, db, body, entry.ModelName, entry.ModelName, constant.ChannelTypeMoliiGrokAIGC, true)
	require.NoError(t, db.Create(&model.MoliiFile{FileID: "file_catalog", UserID: info.UserId, ObjectKey: "test/image.png", MediaType: model.MoliiFileMediaTypeImage, ExpiresAt: time.Now().Add(time.Hour).Unix()}).Error)
	entered, release := make(chan struct{}), make(chan struct{})
	var once, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("catalog-image-media", func(tx *gorm.DB) {
		if tx.Statement.Table == "molii_files" {
			once.Do(func() { close(entered); <-release })
		}
	}))
	defer db.Callback().Query().Remove("catalog-image-media")
	done := make(chan *types.NewAPIError, 1)
	go func() { done <- prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1) }()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("did not reach media processing: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("did not reach media processing")
	}
	published := make(chan error, 1)
	go func() {
		err := model.UpdateModelPricing([]model.ModelPricingChange{{ModelName: entry.ModelName, ExpectedVersion: before.Entries[0].Version, BillingCurrency: "CNY", Pricing: model.PricingValues{"ModelPrice": float64(2)}}})
		if err == nil {
			err = model.UpdateOptionsBulk(map[string]string{"molii_grok_price.image_quality_1k": "0.7", "molii_grok_price.image_quality_input": "0.14"})
		}
		published <- err
	}()
	select {
	case err = <-published:
	case <-time.After(5 * time.Second):
		unblock()
		<-published
		<-done
		t.Fatal("catalog writer waited for media processing")
	}
	unblock()
	require.NoError(t, err)
	require.Nil(t, <-done)
	require.NotNil(t, info.GrokImageBilling)
	assert.Equal(t, "USD", info.GrokImageBilling.SourceCurrency)
	assert.InDelta(t, 0.35, info.GrokImageBilling.CostUSD, 1e-12)
	assert.Equal(t, float64(1), info.PriceData.ModelPrice)
	// Preserve the pre-existing n multiplier policy: the direct subtotal already
	// includes two outputs and the generic image path also multiplies by n=2.
	assert.Equal(t, 350000, info.PriceData.QuotaToPreConsume)
	assert.Equal(t, 350000, info.Billing.GetPreConsumedQuota())
	assert.Equal(t, entry.ModelName, info.OriginModelName)
	assert.Equal(t, entry.ModelName, info.BillingModelName)
	assert.Equal(t, entry.ModelName, request.Model)
	saved := *info.GrokImageBilling
	savedSelection := info.PricingSelection
	catalogImageResponse(t, c, info, request)
	assert.Equal(t, 1, info.GrokImageBilling.OutputCount)
	assert.InDelta(t, 0.21, info.GrokImageBilling.CostUSD, 1e-12)
	assert.Equal(t, 2, saved.OutputCount)
	var log model.Log
	require.NoError(t, db.Where("user_id = ?", info.UserId).Last(&log).Error)
	assert.Equal(t, 210000, log.Quota, "old direct prices and currency survive count adjustment")
	assert.Equal(t, entry.ModelName, log.ModelName)
	var user model.User
	require.NoError(t, db.First(&user, info.UserId).Error)
	assert.Equal(t, 1790000, user.Quota)
	newBody := `{"model":"grok-imagine-image-quality","prompt":"edit","n":2,"image":"https://example.test/input.png"}`
	c2, info2, request2 := catalogImageRequest(t, db, newBody, entry.ModelName, entry.ModelName, constant.ChannelTypeMoliiGrokAIGC, true)
	require.Nil(t, prepareSelectedImageBilling(c2, info2, request2, request2.GetTokenCountMeta(), 1))
	assert.Equal(t, "CNY", info2.GrokImageBilling.SourceCurrency)
	assert.InDelta(t, 0.22, info2.GrokImageBilling.CostUSD, 1e-12)
	assert.Equal(t, float64(2), info2.PriceData.ModelPrice)
	assert.Equal(t, 220000, info2.PriceData.QuotaToPreConsume)
	catalogImageResponse(t, c2, info2, request2)
	var newLog model.Log
	require.NoError(t, db.Where("user_id = ?", info2.UserId).Last(&newLog).Error)
	assert.Equal(t, 120000, newLog.Quota)
	oldPrice, ok := savedSelection.Model(entry.ModelName)
	require.True(t, ok)
	assert.Equal(t, float64(1), oldPrice.Price)
}
