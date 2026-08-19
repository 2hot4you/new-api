package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type recordingImageBilling struct {
	preConsumed int
	reserved    []int
}

func (*recordingImageBilling) Settle(int) error           { return nil }
func (*recordingImageBilling) Refund(*gin.Context)        {}
func (*recordingImageBilling) NeedsRefund() bool          { return false }
func (b *recordingImageBilling) GetPreConsumedQuota() int { return b.preConsumed }
func (b *recordingImageBilling) Reserve(target int) error {
	b.reserved = append(b.reserved, target)
	b.preConsumed = max(b.preConsumed, target)
	return nil
}

func imageBillingMappingContext(t *testing.T, modelName, mapping string, channelType int) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	body := fmt.Sprintf(`{"model":%q,"prompt":"cat","resolution":"1k"}`, modelName)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyRequestBody, []byte(body))
	c.Set("model_mapping", mapping)
	common.SetContextKey(c, constant.ContextKeyChannelType, channelType)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, modelName)
	t.Cleanup(func() { common.CleanupBodyStorage(c) })
	return c
}

func TestPrepareImageRequestBillingRejectsMappingBeforeEstimateOrPreconsume(t *testing.T) {
	c := imageBillingMappingContext(t, "grok-imagine-image", `{"grok-imagine-image":"grok-imagine-video"}`, constant.ChannelTypeMoliiGrokAIGC)
	request := &dto.ImageRequest{Model: "grok-imagine-image", Prompt: "cat"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-image",
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		Request:         request,
	}

	apiErr := prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1)

	require.NotNil(t, apiErr)
	assert.Contains(t, apiErr.Error(), "incompatible_imagine_model_mapping")
	assert.Nil(t, info.GrokImageBilling, "estimator must not run after mapping validation fails")
	assert.Nil(t, info.Billing, "pre-consumption must not start after mapping validation fails")
}

func TestPrepareImageRequestBillingAllowsGrokImageIdentity(t *testing.T) {
	for _, modelName := range []string{"grok-imagine-image", "grok-imagine-image-quality", "grok-imagine-image-2.0"} {
		t.Run(modelName, func(t *testing.T) {
			mapping := fmt.Sprintf(`{%q:%q}`, modelName, modelName)
			c := imageBillingMappingContext(t, modelName, mapping, constant.ChannelTypeMoliiGrokAIGC)
			request := &dto.ImageRequest{Model: modelName, Prompt: "cat"}
			info := &relaycommon.RelayInfo{
				OriginModelName: modelName,
				RelayMode:       relayconstant.RelayModeImagesGenerations,
				Request:         request,
				Billing:         &recordingImageBilling{},
				UserSetting:     dto.UserSetting{AcceptUnsetRatioModel: true},
			}

			meta := request.GetTokenCountMeta()
			apiErr := prepareSelectedImageBilling(c, info, request, meta, 1)

			if apiErr != nil {
				t.Fatalf("prepare billing: %v", apiErr)
			}
			require.Contains(t, meta.BillingRatios, "molii_grok_direct_cost")
			require.NotNil(t, info.GrokImageBilling)
			assert.Equal(t, modelName, info.GrokImageBilling.RequestedModel)
			assert.Equal(t, modelName, info.GrokImageBilling.BilledModel)
		})
	}
}

func TestPrepareGrokImageUnknownFileIDFailsBeforeSnapshotOrPreconsume(t *testing.T) {
	setupFilesControllerDB(t)
	body := `{"model":"grok-imagine-image","prompt":"edit","images":[{"url":"https://images.example/a.png"},{"file_id":"file_abc"}]}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyRequestBody, []byte(body))
	c.Set("model_mapping", `{"grok-imagine-image":"grok-imagine-image"}`)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeMoliiGrokAIGC)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "grok-imagine-image")
	t.Cleanup(func() { common.CleanupBodyStorage(c) })
	billing := &recordingImageBilling{}
	request := &dto.ImageRequest{Model: "grok-imagine-image", Prompt: "edit"}
	info := &relaycommon.RelayInfo{
		UserId:          42,
		OriginModelName: "grok-imagine-image",
		RelayMode:       relayconstant.RelayModeImagesEdits,
		Request:         request,
		Billing:         billing,
		UserSetting:     dto.UserSetting{AcceptUnsetRatioModel: true},
	}

	apiErr := prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1)

	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
	assert.Equal(t, types.ErrorCode("file_not_found"), apiErr.GetErrorCode())
	assert.Contains(t, apiErr.Error(), "file not found")
	assert.Nil(t, info.GrokImageBilling)
	assert.Empty(t, billing.reserved)
}

func TestPrepareImageRequestBillingPreservesOrdinaryImageMapping(t *testing.T) {
	c := imageBillingMappingContext(t, "dall-e-3", `{"dall-e-3":"gpt-image-1"}`, constant.ChannelTypeOpenAI)
	request := &dto.ImageRequest{Model: "dall-e-3", Prompt: "cat"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "dall-e-3",
		Request:         request,
		Billing:         &recordingImageBilling{},
		UserSetting:     dto.UserSetting{AcceptUnsetRatioModel: true},
	}

	meta := &types.TokenCountMeta{}
	apiErr := prepareSelectedImageBilling(c, info, request, meta, 1)

	require.Nil(t, apiErr)
	assert.Nil(t, meta.BillingRatios)
	assert.Equal(t, "gpt-image-1", request.Model)
	assert.Equal(t, "gpt-image-1", info.UpstreamModelName)
}

func TestOrdinaryImageAttemptPreparationTracksSelectedChannelAndRetry(t *testing.T) {
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalModelPrices := ratio_setting.ModelPrice2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"upstream-b":0.1,"upstream-c":0.2}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		if originalMemoryCacheEnabled && originalDB != nil && originalDB.Migrator().HasTable(&model.Channel{}) {
			model.InitChannelCache()
		}
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	priorityB, priorityC := int64(10), int64(0)
	weight := uint(100)
	baseB, baseC := "https://channel-b.example", "https://channel-c.example"
	mappingB, mappingC := `{"image-routing-test":"upstream-b"}`, `{"image-routing-test":"upstream-c"}`
	channelB := model.Channel{Id: 6201, Type: constant.ChannelTypeOpenAI, Key: "key-b", Status: common.ChannelStatusEnabled, Name: "channel-b", Weight: &weight, Models: "image-routing-test", Group: "default", Priority: &priorityB, BaseURL: &baseB, ModelMapping: &mappingB}
	channelC := model.Channel{Id: 6202, Type: constant.ChannelTypeOpenAI, Key: "key-c", Status: common.ChannelStatusEnabled, Name: "channel-c", Weight: &weight, Models: "image-routing-test", Group: "default", Priority: &priorityC, BaseURL: &baseC, ModelMapping: &mappingC}
	for _, channel := range []*model.Channel{&channelB, &channelC} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "image-routing-test", ChannelId: channel.Id, Enabled: true, Priority: channel.Priority, Weight: weight}).Error)
	}
	model.InitChannelCache()

	c := imageBillingMappingContext(t, "image-routing-test", mappingB, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, &channelB, "image-routing-test"))
	request := &dto.ImageRequest{Model: "image-routing-test", Prompt: "cat"}
	billing := &recordingImageBilling{}
	info := &relaycommon.RelayInfo{
		OriginModelName: "image-routing-test",
		TokenGroup:      "default",
		UsingGroup:      "default",
		UserGroup:       "default",
		Request:         request,
		Billing:         billing,
	}
	meta := request.GetTokenCountMeta()
	retry := 0
	retryParam := &service.RetryParam{Ctx: c, TokenGroup: "default", ModelName: "image-routing-test", RequestPath: "/v1/images/generations", Retry: &retry}

	selected, channelErr := getChannel(c, info, retryParam)
	require.Nil(t, channelErr)
	require.Equal(t, channelB.Id, selected.Id, "first attempt must preserve the middleware-selected channel")
	require.Nil(t, prepareSelectedImageBilling(c, info, request, meta, 1))
	assert.Equal(t, channelB.Id, info.ChannelId)
	assert.Equal(t, "upstream-b", info.UpstreamModelName)
	assert.Equal(t, "key-b", info.ApiKey)
	assert.Equal(t, baseB, info.ChannelBaseUrl)

	retryParam.IncreaseRetry()
	selected, channelErr = getChannel(c, info, retryParam)
	require.Nil(t, channelErr)
	require.Equal(t, channelC.Id, selected.Id)
	require.Nil(t, prepareSelectedImageBilling(c, info, request, meta, 1))
	assert.Equal(t, channelC.Id, info.ChannelId)
	assert.Equal(t, "upstream-c", info.UpstreamModelName, "retry must remap from the requested model, not remap upstream-b")
	assert.Equal(t, "key-c", info.ApiKey)
	assert.Equal(t, baseC, info.ChannelBaseUrl)
	assert.Equal(t, channelC.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId), "logging and disable context must own the retry channel")
	require.Len(t, billing.reserved, 2, "each selected attempt must recompute and reserve its price")
	assert.Greater(t, billing.reserved[1], billing.reserved[0], "retry must reserve the higher price of its newly mapped upstream model")
}

func TestMoliiImagineSyncRequestsNeverRetryOrSwitchChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiErr := types.NewOpenAIError(fmt.Errorf("upstream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)

	for _, channelType := range []int{constant.ChannelTypeStarAI, constant.ChannelTypeMoliiGrokAIGC} {
		t.Run(fmt.Sprintf("channel_type_%d", channelType), func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			common.SetContextKey(c, constant.ContextKeyChannelType, channelType)
			assert.False(t, shouldRetry(c, apiErr, 3), "paid Imagine creation must not be replayed on another channel")
		})
	}
}

func TestOrdinarySyncChannelRetainsRetryBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	apiErr := types.NewOpenAIError(fmt.Errorf("upstream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)

	assert.True(t, shouldRetry(c, apiErr, 3))
}

func TestMoliiGrokFailureCallsSelectedUpstreamOnceWithEligibleFallback(t *testing.T) {
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalModelPrices := ratio_setting.ModelPrice2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"grok-imagine-image":0.02}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		if originalMemoryCacheEnabled && originalDB != nil && originalDB.Migrator().HasTable(&model.Channel{}) {
			model.InitChannelCache()
		}
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	var selectedCalls atomic.Int32
	var fallbackCalls atomic.Int32
	selectedUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") == "Bearer fallback-key" {
			fallbackCalls.Add(1)
			_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/image.png"}]}`))
			return
		}
		selectedCalls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"code":"upstream_failed"}}`))
	}))
	defer selectedUpstream.Close()
	originalBaseURL := constant.ChannelBaseURLs[constant.ChannelTypeMoliiGrokAIGC]
	constant.ChannelBaseURLs[constant.ChannelTypeMoliiGrokAIGC] = selectedUpstream.URL
	t.Cleanup(func() { constant.ChannelBaseURLs[constant.ChannelTypeMoliiGrokAIGC] = originalBaseURL })

	prioritySelected, priorityFallback := int64(10), int64(0)
	weight := uint(100)
	identityMapping := `{"grok-imagine-image":"grok-imagine-image"}`
	selectedChannel := model.Channel{Id: 6301, Type: constant.ChannelTypeMoliiGrokAIGC, Key: "selected-key", Status: common.ChannelStatusEnabled, Name: "selected", Weight: &weight, Models: "grok-imagine-image", Group: "default", Priority: &prioritySelected, BaseURL: &selectedUpstream.URL, ModelMapping: &identityMapping}
	fallbackChannel := model.Channel{Id: 6302, Type: constant.ChannelTypeMoliiGrokAIGC, Key: "fallback-key", Status: common.ChannelStatusEnabled, Name: "fallback", Weight: &weight, Models: "grok-imagine-image", Group: "default", Priority: &priorityFallback, BaseURL: &selectedUpstream.URL, ModelMapping: &identityMapping}
	for _, channel := range []*model.Channel{&selectedChannel, &fallbackChannel} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "grok-imagine-image", ChannelId: channel.Id, Enabled: true, Priority: channel.Priority, Weight: weight}).Error)
	}
	model.InitChannelCache()
	eligibleFallback, err := model.GetRandomSatisfiedChannel("default", "grok-imagine-image", 1, "/v1/images/generations")
	require.NoError(t, err)
	require.Equal(t, fallbackChannel.Id, eligibleFallback.Id)

	c := imageBillingMappingContext(t, "grok-imagine-image", identityMapping, constant.ChannelTypeMoliiGrokAIGC)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, &selectedChannel, "grok-imagine-image"))
	request := &dto.ImageRequest{Model: "grok-imagine-image", Prompt: "cat"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-image",
		TokenGroup:      "default",
		UsingGroup:      "default",
		UserGroup:       "default",
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		RequestURLPath:  "/v1/images/generations",
		Request:         request,
		Billing:         &recordingImageBilling{},
	}
	selected, channelErr := getChannel(c, info, &service.RetryParam{Ctx: c, TokenGroup: "default", ModelName: "grok-imagine-image", RequestPath: c.Request.URL.Path, Retry: common.GetPointer(0)})
	require.Nil(t, channelErr)
	require.Equal(t, selectedChannel.Id, selected.Id)
	addUsedChannel(c, selected.Id)
	require.Nil(t, prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1))

	apiErr := relay.ImageHelper(c, info)
	require.NotNil(t, apiErr)
	assert.False(t, shouldRetry(c, apiErr, 3))
	assert.Equal(t, int32(1), selectedCalls.Load())
	assert.Zero(t, fallbackCalls.Load())
	assert.Equal(t, []string{fmt.Sprintf("%d", selectedChannel.Id)}, c.GetStringSlice("use_channel"))
	assert.Equal(t, selectedChannel.Id, info.ChannelId)
	assert.Equal(t, "selected-key", info.ApiKey)
	assert.Equal(t, selectedUpstream.URL, info.ChannelBaseUrl)
}
