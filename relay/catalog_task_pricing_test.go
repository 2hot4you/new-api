package relay

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	grok "github.com/QuantumNous/new-api/relay/channel/task/moliigrok"
	"github.com/QuantumNous/new-api/relay/channel/task/starai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func catalogTaskContext(name string) (*gin.Context, *relaycommon.RelayInfo) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/videos", nil)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, name)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://provider.example")
	catalogTaskRequest(c, relaycommon.TaskSubmitReq{Model: name, Prompt: "p", Duration: 5, Resolution: "720p"})
	return c, &relaycommon.RelayInfo{OriginModelName: name, UsingGroup: "default", UserGroup: "default", TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: name}}
}

func catalogTaskPostgres(t *testing.T, options map[string]string) *gorm.DB {
	t.Helper()
	saveBillingConfig(t)
	if _, exists := options["group_ratio_setting.group_ratio"]; !exists {
		options["group_ratio_setting.group_ratio"] = `{"default":1}`
	}
	return catalogPricingPostgres(t, options)
}

func catalogTaskRequest(c *gin.Context, req relaycommon.TaskSubmitReq) {
	body, err := common.Marshal(req)
	if err != nil {
		panic(err)
	}
	c.Request = httptest.NewRequest("POST", c.Request.URL.Path, strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("task_request", req)
}

func TestCatalogTaskExpressionPrecedence(t *testing.T) {
	db := catalogTaskPostgres(t, map[string]string{"ModelPrice": `{}`, "ModelRatio": `{}`, "billing_setting.billing_mode": `{}`})
	source := strings.Replace(billingFallbackPlugin, `fetchMode:"per_task"`, `fetchMode:"per_task",usageSchema:{seconds:{type:"number",unit:"second"}}`, 1) + `export function extractUsage(){return {seconds:2};}`
	registry := jsplugin.DefaultRegistry
	plugin, err := registry.Register(source, jsplugin.Options{})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.TaskPlugin{Key: plugin.Meta.Key, Version: plugin.Meta.Version, APIVersion: 1, Source: model.LongText(source), SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(source))), Active: true, Enabled: true}).Error)
	require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
	for _, tc := range []struct {
		name, explicit, overrides, modes, expressions, wantModel, wantExpr string
		wantError                                                          bool
	}{
		{"origin override", "", `{"bill-fallback::alias-model":"u(\"seconds\") * 2","bill-fallback::declared-model":"u(\"seconds\") * 3"}`, `{}`, `{}`, "alias-model", `u("seconds") * 2`, false},
		{"mapped override", "", `{"bill-fallback::declared-model":"u(\"seconds\") * 3"}`, `{"alias-model":"tiered_expr"}`, `{"alias-model":"u(\"seconds\") * 4"}`, "alias-model", `u("seconds") * 3`, false},
		{"origin expression", "", `{}`, `{"alias-model":"tiered_expr","declared-model":"tiered_expr"}`, `{"alias-model":"u(\"seconds\") * 4","declared-model":"u(\"seconds\") * 5"}`, "alias-model", `u("seconds") * 4`, false},
		{"mapped expression", "", `{}`, `{"declared-model":"tiered_expr"}`, `{"declared-model":"u(\"seconds\") * 5"}`, "declared-model", `u("seconds") * 5`, false},
		{"explicit identity", "explicit-model", `{"bill-fallback::declared-model":"u(\"seconds\") * 3"}`, `{"explicit-model":"tiered_expr"}`, `{"explicit-model":"u(\"seconds\") * 6"}`, "explicit-model", `u("seconds") * 6`, false},
		{"empty override is terminal", "", `{"bill-fallback::alias-model":"","bill-fallback::declared-model":"u(\"seconds\") * 3"}`, `{"alias-model":"tiered_expr"}`, `{"alias-model":"u(\"seconds\") * 4"}`, "alias-model", "", true},
		{"missing active expression", "", `{}`, `{"alias-model":"tiered_expr","declared-model":"tiered_expr"}`, `{"declared-model":"u(\"seconds\") * 5"}`, "alias-model", "", true},
		{"explicit cannot use mapped fallback", "explicit-model", `{"bill-fallback::declared-model":"u(\"seconds\") * 3"}`, `{}`, `{}`, "explicit-model", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// These are persisted administrator inputs, including legacy empty
			// values which the current write API deliberately refuses to create.
			for key, value := range map[string]string{billing_setting.PluginBillingExprOption: tc.overrides, "billing_setting.billing_mode": tc.modes, "billing_setting.billing_expr": tc.expressions} {
				require.NoError(t, db.Where("key = ?", key).Delete(&model.Option{}).Error)
				require.NoError(t, db.Create(&model.Option{Key: key, Value: value}).Error)
			}
			err := model.RecoverCatalogSyncRuntime(context.Background())
			if tc.name == "empty override is terminal" || tc.name == "missing active expression" {
				require.Error(t, err, "invalid persisted expressions cannot become a selected generation")
				return
			}
			require.NoError(t, err)
			c, info := catalogTaskContext("alias-model")
			c.Set("model_mapping", `{"alias-model":"declared-model"}`)
			c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Plugin: plugin, Generation: registry.Generation()})
			info.BillingModelName = tc.explicit
			_, taskErr := EstimateTaskSubmit(c, info)
			if tc.wantError {
				require.NotNil(t, taskErr)
				assert.Equal(t, "model_price_error", taskErr.Code)
				assert.Nil(t, info.TieredBillingSnapshot)
			} else {
				require.Nil(t, taskErr)
				require.NotNil(t, info.TieredBillingSnapshot)
				assert.Equal(t, tc.wantExpr, info.TieredBillingSnapshot.ExprString)
				assert.Equal(t, tc.wantModel, info.TieredBillingSnapshot.ModelName)
			}
			assert.Equal(t, "alias-model", info.OriginModelName)
			assert.Equal(t, "declared-model", info.UpstreamModelName)
		})
	}
}

func TestCatalogTaskPendingRejectsBeforeUsageHook(t *testing.T) {
	db := catalogTaskPostgres(t, map[string]string{"ModelPrice": `{"declared-model":1}`, "billing_setting.billing_mode": `{}`})
	called := 0
	source := billingFallbackPlugin + `export function extractUsage(){console.log("usage");return {seconds:2};}`
	plugin, err := jsplugin.NewRegistry().Register(source, jsplugin.Options{Log: func(string) { called++ }})
	require.NoError(t, err)
	c, info := catalogTaskContext("declared-model")
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Plugin: plugin})
	_, taskErr := EstimateTaskSubmit(c, info)
	require.Nil(t, taskErr)
	old := info.PricingSelection
	called = 0
	failure := errors.New("task pricing projection failed")
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("task-pending", func(tx *gorm.DB) {
		if tx.Statement.Table == "vendors" && tx.Statement.ConnPool == db.Statement.ConnPool {
			tx.AddError(failure)
		}
	}))
	err = model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{"declared-model":3}`})
	require.NoError(t, db.Callback().Query().Remove("task-pending"))
	require.ErrorIs(t, err, failure)
	_, taskErr = EstimateTaskSubmit(c, info)
	require.NotNil(t, taskErr)
	assert.ErrorIs(t, taskErr.Error, model.ErrCatalogPublicationPending)
	assert.Nil(t, info.PricingSelection)
	assert.Nil(t, info.TieredBillingSnapshot)
	assert.Nil(t, info.GrokVideoBilling)
	assert.Zero(t, called)
	previous, _ := old.Model("declared-model")
	assert.Equal(t, 1.0, previous.Price)
	require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
	_, taskErr = EstimateTaskSubmit(c, info)
	require.Nil(t, taskErr)
	assert.Positive(t, called)
	assert.Equal(t, 3.0, info.PriceData.ModelPrice)
}

func TestCatalogTaskRetryClearsEstimateInputs(t *testing.T) {
	catalogTaskPostgres(t, map[string]string{"ModelRatio": `{"doubao-seedance-2-0-260128":2}`, "billing_setting.billing_mode": `{}`})
	c, info := catalogTaskContext(starai.ModelSeedance20)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeStarAI)
	_, taskErr := EstimateTaskSubmit(c, info)
	require.Nil(t, taskErr)
	require.Positive(t, info.EstimatedVideoTokens)
	plugin, err := jsplugin.NewRegistry().Register(strings.ReplaceAll(billingFallbackPlugin, "declared-model", starai.ModelSeedance20), jsplugin.Options{})
	require.NoError(t, err)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeTaskPlugin)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Plugin: plugin})
	_, taskErr = EstimateTaskSubmit(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, 500000, info.PriceData.Quota, "new generic attempt must not reuse Seedance's old token quantity")
	assert.Zero(t, info.EstimatedVideoTokens)
	assert.Zero(t, info.EstimatedVideoSeconds)
	assert.Empty(t, info.EstimatedVideoResolution)
}

func TestCatalogTaskNativePrepareAndDurableCompletion(t *testing.T) {
	db := catalogTaskPostgres(t, map[string]string{
		"ModelRatio": `{"doubao-seedance-2-0-260128":2}`, "ModelPrice": `{"grok-imagine-video":1}`,
		"billing_setting.billing_mode": `{}`, "starai_video_price.standard_720p": "46",
		"molii_grok_price.video_720p": "0.07", "molii_grok_price.video_480p": "0.03", "molii_grok_price.video_video_input": "0.02",
		"group_ratio_setting.group_ratio": `{"default":0.5}`, "USDExchangeRate": "7",
	})
	entry := model.Model{ModelName: grok.LegacyVideoModel, BillingCurrency: "USD"}
	require.NoError(t, entry.Insert())
	c, info := catalogTaskContext(starai.ModelSeedance20)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeStarAI)
	_, taskErr := EstimateTaskSubmit(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, 108900, info.EstimatedVideoTokens)
	assert.Equal(t, 1252350, info.PriceData.Quota)
	user := model.User{Username: "task-durable", AffCode: "task-durable", Quota: 10000000}
	require.NoError(t, db.Create(&user).Error)
	seed := model.Task{TaskID: model.GenerateTaskID(), UserId: user.Id, Group: "default", Platform: constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeStarAI)), Quota: info.PriceData.Quota,
		Properties:  model.Properties{OriginModelName: info.OriginModelName},
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{OriginModelName: info.OriginModelName, ModelRatio: info.PriceData.ModelRatio, GroupRatio: 0.5, GroupRatioCaptured: true, OtherRatios: info.PriceData.OtherRatios()}}}
	require.NoError(t, db.Create(&seed).Error)
	gc, gi := catalogTaskContext(grok.LegacyVideoModel)
	common.SetContextKey(gc, constant.ContextKeyChannelType, constant.ChannelTypeMoliiGrokAIGC)
	gc.Request = httptest.NewRequest("POST", "/v1/videos/edits", nil)
	catalogTaskRequest(gc, relaycommon.TaskSubmitReq{Model: grok.LegacyVideoModel, Prompt: "edit", Video: "https://example.com/video.mp4"})
	gi.InputVideoDurationSeconds, gi.InputVideoResolutionTier, gi.InputVideoResolutionSource = 4, "720p", relaycommon.GrokVideoResolutionSourceInputProbeV1
	_, taskErr = EstimateTaskSubmit(gc, gi)
	require.Nil(t, taskErr)
	assert.Equal(t, 90000, gi.PriceData.Quota)
	assert.Equal(t, map[string]float64{"480p": 0.03, "720p": 0.07}, gi.EstimatedVideoOutputUnitPrices)
	bc := &model.TaskBillingContext{OriginModelName: gi.OriginModelName, GroupRatio: 0.5, ModelPrice: gi.PriceData.ModelPrice, OtherRatios: gi.PriceData.OtherRatios(), PerCallBilling: true}
	require.True(t, service.ConfigureGrokVideoFinalUsage(bc, gi.GrokVideoBilling, gc.Request.URL.Path))
	video := model.Task{TaskID: model.GenerateTaskID(), UserId: user.Id, Group: "default", Platform: constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeMoliiGrokAIGC)), Quota: gi.PriceData.Quota, PrivateData: model.TaskPrivateData{BillingContext: bc}}
	require.NoError(t, db.Create(&video).Error)
	ec, ei := catalogTaskContext(grok.LegacyVideoModel)
	common.SetContextKey(ec, constant.ContextKeyChannelType, constant.ChannelTypeMoliiGrokAIGC)
	ec.Request = httptest.NewRequest("POST", "/v1/videos/extensions", nil)
	catalogTaskRequest(ec, relaycommon.TaskSubmitReq{Model: grok.LegacyVideoModel, Prompt: "extend", Video: "https://example.com/video.mp4", Duration: 6})
	ei.InputVideoDurationSeconds, ei.InputVideoResolutionTier, ei.InputVideoResolutionSource = 4, "720p", relaycommon.GrokVideoResolutionSourceInputProbeV1
	_, taskErr = EstimateTaskSubmit(ec, ei)
	require.Nil(t, taskErr)
	assert.Equal(t, 125000, ei.PriceData.Quota)
	assert.Equal(t, map[string]float64{"480p": 0.03, "720p": 0.07}, ei.EstimatedVideoOutputUnitPrices)
	extension := model.Task{TaskID: model.GenerateTaskID(), Platform: video.Platform, Status: model.TaskStatusSuccess, Quota: ei.PriceData.Quota, PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{GroupRatio: 0.5, PerCallBilling: true}}}
	require.True(t, service.ConfigureGrokVideoFinalUsage(extension.PrivateData.BillingContext, ei.GrokVideoBilling, ec.Request.URL.Path))
	require.NoError(t, db.Create(&extension).Error)
	before, err := model.GetModelPricingSnapshot([]string{entry.ModelName})
	require.NoError(t, err)
	require.NoError(t, model.UpdateModelPricing([]model.ModelPricingChange{{ModelName: entry.ModelName, ExpectedVersion: before.Entries[0].Version, BillingCurrency: "CNY", Pricing: model.PricingValues{"ModelPrice": float64(4)}}}))
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelRatio": `{"doubao-seedance-2-0-260128":20}`, "starai_video_price.standard_720p": "80", "molii_grok_price.video_720p": "0.7", "molii_grok_price.video_video_input": "0.2"}))
	var savedSeed, savedVideo model.Task
	require.NoError(t, db.First(&savedSeed, seed.ID).Error)
	require.NoError(t, db.First(&savedVideo, video.ID).Error)
	require.True(t, service.RecalculateTaskQuotaByTokens(context.Background(), &savedSeed, 1000))
	assert.Equal(t, 11500, savedSeed.Quota)
	var wallet model.User
	require.NoError(t, db.First(&wallet, user.Id).Error)
	assert.Equal(t, 11240850, wallet.Quota)
	savedVideo.Status = model.TaskStatusSuccess
	job := service.BuildTerminalTaskBillingJob(context.Background(), &grok.TaskAdaptor{}, &savedVideo, &relaycommon.TaskInfo{ActualDurationSeconds: 9, ActualResolution: "480p"})
	require.NotNil(t, job.TargetQuota)
	assert.Equal(t, 90000, *job.TargetQuota, "edit completion retains probed input duration and selected V2 prices/currency")
	assert.Equal(t, "USD", savedVideo.PrivateData.BillingContext.GrokVideoBilling.SourceCurrency)
	assert.Equal(t, 4.0, savedVideo.PrivateData.BillingContext.GrokVideoBilling.ActualDurationSeconds)
	require.True(t, service.RecalculateTaskQuota(context.Background(), &savedVideo, *job.TargetQuota, "selected V2"))
	var savedExtension model.Task
	require.NoError(t, db.First(&savedExtension, extension.ID).Error)
	extensionJob := service.BuildTerminalTaskBillingJob(context.Background(), &grok.TaskAdaptor{}, &savedExtension, &relaycommon.TaskInfo{ActualDurationSeconds: 9})
	require.NotNil(t, extensionJob.TargetQuota)
	assert.Equal(t, 125000, *extensionJob.TargetQuota)
	_, taskErr = EstimateTaskSubmit(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, 80.0, info.EstimatedVideoUnitPrice)
	assert.Equal(t, 2178000, info.PriceData.Quota)
	assert.Equal(t, 2.0, savedSeed.PrivateData.BillingContext.ModelRatio)
}

func TestCatalogTaskSelectedZeroMissingAndBounds(t *testing.T) {
	db := catalogTaskPostgres(t, map[string]string{"ModelPrice": `{"grok-imagine-video":1}`, "ModelRatio": `{"doubao-seedance-2-0-260128":2}`, "starai_video_price.standard_720p": "0", "billing_setting.billing_mode": `{}`, "molii_grok_price.video_720p": "0", "molii_grok_price.video_image_input": "0", "molii_grok_price.video_video_input": "0"})
	c, info := catalogTaskContext(grok.LegacyVideoModel)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeMoliiGrokAIGC)
	_, taskErr := EstimateTaskSubmit(c, info)
	require.Nil(t, taskErr)
	assert.Zero(t, info.PriceData.Quota)
	sc, si := catalogTaskContext(starai.ModelSeedance20)
	common.SetContextKey(sc, constant.ContextKeyChannelType, constant.ChannelTypeStarAI)
	_, taskErr = EstimateTaskSubmit(sc, si)
	require.Nil(t, taskErr)
	assert.Zero(t, si.EstimatedVideoUnitPrice)
	// Existing positive-only multiplier policy drops zero: this is a known
	// separate billing defect, not a claim that zero-price Seedance is free.
	assert.Equal(t, 217800, si.PriceData.Quota)
	task := model.Task{TaskID: model.GenerateTaskID(), Platform: constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeStarAI)), Status: model.TaskStatusSuccess, Group: "default", Quota: si.PriceData.Quota, PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{OriginModelName: si.OriginModelName, ModelRatio: si.PriceData.ModelRatio, GroupRatio: 1, OtherRatios: si.PriceData.OtherRatios()}}}
	require.NoError(t, db.Create(&task).Error)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{"grok-imagine-video":4}`, "molii_grok_price.video_720p": "0.7", "starai_video_price.standard_720p": "80"}))
	assert.Zero(t, (&starai.TaskAdaptor{}).EstimateBilling(sc, si)["seedance-720p-no-video-input"])
	var saved model.Task
	require.NoError(t, db.First(&saved, task.ID).Error)
	job := service.BuildTerminalTaskBillingJob(context.Background(), &starai.TaskAdaptor{}, &saved, &relaycommon.TaskInfo{TotalTokens: 1000})
	require.NotNil(t, job.TargetQuota)
	assert.Equal(t, 2000, *job.TargetQuota, "selected zero must not reread new80; existing base-charge limitation remains")
	assert.Zero(t, (&grok.TaskAdaptor{}).EstimateBilling(c, info)["molii_grok_direct_cost"])
	snapshot := service.BuildGrokVideoBillingSnapshot(c, info, 0)
	require.NotNil(t, snapshot)
	assert.Zero(t, snapshot.Subtotal)
	for _, tc := range []struct {
		name   string
		models map[string]relaycommon.ModelPricing
		rates  ratio_setting.MoliiGrokPriceSetting
	}{
		{"missing identity", nil, info.PricingSelection.GrokPrices()},
		{"explicit zero anchor", map[string]relaycommon.ModelPricing{grok.LegacyVideoModel: {HasPrice: true, Price: 0, HasDefaultPrice: true, DefaultPrice: 1}}, info.PricingSelection.GrokPrices()},
		{"missing anchor", map[string]relaycommon.ModelPricing{grok.LegacyVideoModel: {}}, info.PricingSelection.GrokPrices()},
		{"invalid selected direct rate", map[string]relaycommon.ModelPricing{grok.LegacyVideoModel: {HasPrice: true, Price: 1}}, ratio_setting.MoliiGrokPriceSetting{Video720p: math.NaN()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info.PricingSelection = relaycommon.NewRequestPricingSelection(tc.models, operation_setting.ToolPrices{}, tc.rates, ratio_setting.StarAIVideoPriceSetting{})
			_, err := (&grok.TaskAdaptor{}).EstimateBillingValidated(c, info)
			require.Error(t, err)
			assert.Nil(t, service.BuildGrokVideoBillingSnapshot(c, info, 0), "incomplete selected money/rates cannot use live fallback")
		})
	}
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{"grok-imagine-video":0}`}))
	c, info = catalogTaskContext(grok.LegacyVideoModel)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeMoliiGrokAIGC)
	_, taskErr = EstimateTaskSubmit(c, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, "plugin_usage_invalid", taskErr.Code)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelPrice": `{"grok-imagine-video":4}`}))
	// Actual preparation retains request bounds and checked quota saturation.
	c, info = catalogTaskContext(grok.LegacyVideoModel)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeMoliiGrokAIGC)
	catalogTaskRequest(c, relaycommon.TaskSubmitReq{Model: grok.LegacyVideoModel, Prompt: "p", Duration: 16, Resolution: "720p"})
	_, taskErr = EstimateTaskSubmit(c, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_duration", taskErr.Code)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"molii_grok_price.video_720p": "1000000000000"}))
	c, info = catalogTaskContext(grok.LegacyVideoModel)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeMoliiGrokAIGC)
	catalogTaskRequest(c, relaycommon.TaskSubmitReq{Model: grok.LegacyVideoModel, Prompt: "p", Duration: 5, Resolution: "720p"})
	_, taskErr = EstimateTaskSubmit(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, common.MaxQuota, info.PriceData.Quota)
	assert.NotNil(t, info.QuotaClamp)
	assert.Nil(t, info.Billing)
	user := model.User{Username: "task-bounds", AffCode: "task-bounds", Quota: 1000}
	require.NoError(t, db.Create(&user).Error)
	info.UserId, info.UserQuota, info.IsPlayground, info.ForcePreConsume = user.Id, 1000, true, true
	require.NotNil(t, service.PreConsumeBilling(c, info.PriceData.Quota, info))
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 1000, user.Quota)
}

func TestCatalogTaskPluginHookPublicationOverlap(t *testing.T) {
	const expr = `u("seconds") * 0.2`
	db := catalogTaskPostgres(t, map[string]string{
		billing_setting.PluginBillingExprOption: `{"bill-fallback::declared-model":"u(\"seconds\") * 0.2"}`,
		"USDExchangeRate":                       "7",
	})
	entry := model.Model{ModelName: "declared-model", BillingCurrency: "USD"}
	require.NoError(t, entry.Insert())
	before, err := model.GetModelPricingSnapshot([]string{entry.ModelName})
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	var once, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	source := strings.Replace(billingFallbackPlugin, `fetchMode:"per_task"`, `fetchMode:"per_task",usageSchema:{seconds:{type:"number",unit:"second"}}`, 1) + `
export function extractUsage(){console.log("usage-boundary");return {seconds:2};}`
	registry := jsplugin.DefaultRegistry
	plugin, err := registry.Register(source, jsplugin.Options{Timeout: 20 * time.Second, Log: func(message string) {
		if strings.Contains(message, "usage-boundary") {
			once.Do(func() { close(entered); <-release })
		}
	}})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.TaskPlugin{Key: plugin.Meta.Key, Version: plugin.Meta.Version, APIVersion: 1, Source: model.LongText(source), SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(source))), Active: true, Enabled: true}).Error)
	otherSource := strings.ReplaceAll(strings.ReplaceAll(source, "bill-fallback", "unrelated-plugin"), "seconds", "credits")
	other, err := registry.Register(otherSource, jsplugin.Options{})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.TaskPlugin{Key: other.Meta.Key, Version: other.Meta.Version, APIVersion: 1, Source: model.LongText(otherSource), SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(otherSource))), Active: true, Enabled: true}).Error)
	require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
	c, info := catalogTaskContext(entry.ModelName)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Plugin: plugin, Generation: registry.Generation()})
	c.Set("task_plugin_key", "unrelated-plugin")
	done := make(chan *TaskBillingEstimate, 1)
	failure := make(chan any, 1)
	go func() { estimate, taskErr := EstimateTaskSubmit(c, info); failure <- taskErr; done <- estimate }()
	select {
	case <-entered:
	case value := <-failure:
		t.Fatalf("submission failed before hook: %+v", value)
	case <-time.After(5 * time.Second):
		t.Fatal("actual extractUsage hook was not reached")
	}
	published := make(chan error, 1)
	go func() {
		published <- model.UpdateModelPricing([]model.ModelPricingChange{{ModelName: entry.ModelName, ExpectedVersion: before.Entries[0].Version, BillingCurrency: "CNY", Pricing: model.PricingValues{"ModelPrice": float64(3)}}})
	}()
	select {
	case err = <-published:
	case <-time.After(5 * time.Second):
		unblock()
		<-published
		<-done
		t.Fatal("catalog writer held across plugin hook")
	}
	unblock()
	require.NoError(t, err)
	require.Nil(t, <-failure)
	require.NotNil(t, <-done)
	require.NotNil(t, info.TieredBillingSnapshot)
	assert.Equal(t, expr, info.TieredBillingSnapshot.ExprString)
	assert.Equal(t, "USD", info.TieredBillingSnapshot.SourceCurrency)
	assert.Equal(t, 200000, info.PriceData.Quota)
	// Round-trip the actual submission contract before later publication and
	// evaluate the public completion consumer with measured usage.
	task := model.Task{TaskID: model.GenerateTaskID(), PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{TieredSnapshot: info.TieredBillingSnapshot}}}
	require.NoError(t, db.Create(&task).Error)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{billing_setting.PluginBillingExprOption: `{"bill-fallback::declared-model":"u(\"seconds\") * 8"}`}))
	var saved model.Task
	require.NoError(t, db.First(&saved, task.ID).Error)
	result, _, err := service.EvaluateTaskCompletionUsage(saved.PrivateData.BillingContext.TieredSnapshot, map[string]any{"seconds": float64(4)})
	require.NoError(t, err)
	assert.Equal(t, 400000, result.ActualQuotaAfterGroup)
	saved.Status = model.TaskStatusSuccess
	job := service.BuildTerminalTaskBillingJob(context.Background(), GetTaskAdaptor(constant.TaskPlatform(plugin.Meta.Key)), &saved, &relaycommon.TaskInfo{UsageFacts: map[string]any{"seconds": float64(4)}})
	require.NotNil(t, job.TargetQuota)
	assert.Equal(t, 400000, *job.TargetQuota)
	_, taskErr := EstimateTaskSubmit(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, "CNY", info.TieredBillingSnapshot.SourceCurrency)
	assert.Equal(t, 1142857, info.PriceData.Quota)
	assert.Equal(t, expr, saved.PrivateData.BillingContext.TieredSnapshot.ExprString)
}

func TestCatalogTaskDirectConsumersRemainSelected(t *testing.T) {
	db := catalogTaskPostgres(t, map[string]string{
		"ModelRatio":                       `{"doubao-seedance-2-0-260128":2}`,
		"ModelPrice":                       `{"grok-imagine-video":1}`,
		"billing_setting.billing_mode":     `{}`,
		"starai_video_price.standard_720p": "46",
		"molii_grok_price.video_720p":      "0.07",
		"USDExchangeRate":                  "7",
	})
	require.NoError(t, db.Create(&model.Model{ModelName: grok.LegacyVideoModel, BillingCurrency: "USD"}).Error)
	c, info := catalogTaskContext(starai.ModelSeedance20)
	require.NoError(t, helper.CaptureRequestPricing(c, info))
	gc, gi := catalogTaskContext(grok.LegacyVideoModel)
	gi.EstimatedVideoSeconds, gi.EstimatedVideoResolution = 5, "720p"
	gi.PriceData.GroupRatioInfo.GroupRatio = 1
	require.NoError(t, helper.CaptureRequestPricing(gc, gi))
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		"ModelRatio":                       `{"doubao-seedance-2-0-260128":20}`,
		"ModelPrice":                       `{"grok-imagine-video":4}`,
		"starai_video_price.standard_720p": "80",
		"molii_grok_price.video_720p":      "0.7",
	}))
	current, err := model.GetModelPricingSnapshot([]string{grok.LegacyVideoModel})
	require.NoError(t, err)
	require.NoError(t, model.UpdateModelPricing([]model.ModelPricingChange{{ModelName: grok.LegacyVideoModel, ExpectedVersion: current.Entries[0].Version, BillingCurrency: "CNY", Pricing: model.PricingValues{"ModelPrice": float64(4)}}}))
	ratios := (&starai.TaskAdaptor{}).EstimateBilling(c, info)
	assert.InDelta(t, 11.5, ratios["seedance-720p-no-video-input"], 1e-9)
	assert.Equal(t, 46.0, info.EstimatedVideoUnitPrice)
	gr := (&grok.TaskAdaptor{}).EstimateBilling(gc, gi)
	assert.InDelta(t, 0.35, gr["molii_grok_direct_cost"], 1e-9)
	assert.InDelta(t, 0.07, gi.EstimatedVideoUnitPrice, 1e-9)
	snapshot := service.BuildGrokVideoBillingSnapshot(gc, gi, 0)
	require.NotNil(t, snapshot)
	assert.InDelta(t, 0.07, snapshot.OutputUnitPrice, 1e-9)
	assert.InDelta(t, 0.35, snapshot.Subtotal, 1e-9)
	assert.Equal(t, "USD", snapshot.SourceCurrency)
	assert.Equal(t, 175000, gi.PriceData.Quota)
	// Nil-selection compatibility remains explicitly bounded to direct callers.
	info.PricingSelection = nil
	assert.Equal(t, 2.0, (&starai.TaskAdaptor{}).EstimateBilling(c, info)["seedance-720p-no-video-input"])
	assert.Nil(t, info.PricingSelection)
	// Capture the original default anchors too; later overrides must not become
	// denominators for the frozen direct tables.
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelRatio": `{}`, "ModelPrice": `{}`, "starai_video_price.standard_720p": "46", "molii_grok_price.video_720p": "0.07"}))
	require.NoError(t, helper.CaptureRequestPricing(c, info))
	require.NoError(t, helper.CaptureRequestPricing(gc, gi))
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{"ModelRatio": `{"doubao-seedance-2-0-260128":20}`, "ModelPrice": `{"grok-imagine-video":4}`}))
	assert.Equal(t, 1.0, (&starai.TaskAdaptor{}).EstimateBilling(c, info)["seedance-720p-no-video-input"])
	assert.InDelta(t, 0.35, (&grok.TaskAdaptor{}).EstimateBilling(gc, gi)["molii_grok_direct_cost"], 1e-9)
}
