package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A local catalog outage must not be reported as a missing administrator quote.
func TestCatalogPricingUnavailableChannelTest(t *testing.T) {
	require.NoError(t, i18n.Init())
	db := catalogControllerPostgres(t, map[string]string{"ModelPrice": `{"catalog-priced":0.1}`, "GroupRatio": `{"default":1}`})
	user := model.User{Username: "catalog-error-test", AffCode: "catalog-error-test", Group: "default", Quota: 2000000}
	require.NoError(t, db.Create(&user).Error)
	base := "http://127.0.0.1:1"
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Key: "fixture-only", Name: "catalog-error", Status: common.ChannelStatusEnabled, Models: "catalog-priced", Group: "default", BaseURL: &base, ResponseTime: 731, AutoBan: common.GetPointer(1)}
	require.NoError(t, db.Create(&channel).Error)
	request := func(modelName string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/channel/test/%d?model=%s", channel.Id, modelName), nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(channel.Id)}}
		c.Set("id", user.Id)
		TestChannel(c)
		return recorder
	}
	for _, state := range []string{"busy", "pending"} {
		t.Run(state, func(t *testing.T) {
			var response *httptest.ResponseRecorder
			if state == "busy" {
				require.NoError(t, model.WithCatalogWriteBarrier(func() error { response = request("catalog-priced"); return nil }))
			} else {
				require.NoError(t, db.Model(&model.CatalogSyncState{}).Where("id = ?", model.CatalogSyncStateID).Update("publication_state", "committed_pending_publish").Error)
				response = request("catalog-priced")
			}
			assert.Equal(t, http.StatusOK, response.Code, "channel tests retain their outer HTTP contract")
			var result struct {
				Success   bool    `json:"success"`
				ErrorCode string  `json:"error_code"`
				Time      float64 `json:"time"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			assert.False(t, result.Success)
			assert.Equal(t, "catalog_pricing_unavailable", result.ErrorCode)
			assert.Zero(t, result.Time)
		})
	}
	previousDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = previousDisable })
	require.True(t, channel.GetAutoBan())
	summary := testChannelForHealthCheck(context.Background(), &channel, user.Id, true, -1)
	assert.Equal(t, channelTestSummary{Tested: 1, Failed: 1}, summary, "local catalog failure must not request channel disable even above the latency threshold")
	var after model.Channel
	require.NoError(t, db.First(&after, channel.Id).Error)
	assert.Equal(t, 731, after.ResponseTime, "local catalog failure must not replace upstream latency")
	assert.Equal(t, common.ChannelStatusEnabled, after.Status)
	assert.Zero(t, after.TestTime)
	require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
	missing := request("catalog-unpriced")
	assert.Equal(t, http.StatusOK, missing.Code)
	var missingResult struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"error_code"`
	}
	require.NoError(t, common.Unmarshal(missing.Body.Bytes(), &missingResult))
	assert.False(t, missingResult.Success)
	assert.Equal(t, "model_price_error", missingResult.ErrorCode)
}

func TestCatalogPricingErrorClassification(t *testing.T) {
	previousDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = previousDisable })
	for _, tc := range []struct {
		name        string
		err         error
		wantCode    string
		wantStatus  int
		unavailable bool
	}{
		{"wrapped writer", fmt.Errorf("selection: %w", model.ErrCatalogWriterBusy), "catalog_pricing_unavailable", 503, true},
		{"joined pending", errors.Join(errors.New("safe diagnostic"), model.ErrCatalogPublicationPending), "catalog_pricing_unavailable", 503, true},
		{"previously misclassified", types.NewError(fmt.Errorf("selection: %w", model.ErrCatalogPublicationPending), types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(400)), "catalog_pricing_unavailable", 503, true},
		{"same text is not sentinel", errors.New(model.ErrCatalogWriterBusy.Error()), "model_price_error", 400, false},
		{"missing quote", errors.New("model has no configured quote"), "model_price_error", 400, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			apiErr := helper.ModelPricingError(tc.err)
			assert.Equal(t, tc.wantStatus, apiErr.StatusCode)
			assert.EqualValues(t, tc.wantCode, apiErr.GetErrorCode())
			assert.EqualValues(t, tc.wantCode, apiErr.ToOpenAIError().Code)
			assert.Equal(t, tc.unavailable, helper.IsCatalogPricingUnavailable(apiErr))
			assert.Equal(t, tc.unavailable, types.IsSkipRetryError(apiErr))
			if tc.unavailable {
				assert.Equal(t, "catalog_pricing_unavailable", apiErr.ToClaudeError().Type)
				assert.True(t, helper.IsCatalogPricingUnavailable(apiErr), "format conversion must preserve the sentinel")
				assert.True(t, types.IsSkipRetryError(apiErr), "format conversion must preserve skip-retry")
				assert.False(t, service.ShouldDisableChannel(apiErr))
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				assert.False(t, service.ShouldRetryRelayError(c, apiErr, 3))
			} else {
				assert.Equal(t, "new_api_error", apiErr.ToClaudeError().Type, "other Claude error types must remain unchanged")
			}
		})
	}
}

func TestCatalogPricingUnavailableImageSelection(t *testing.T) {
	db := catalogControllerPostgres(t, map[string]string{"ModelPrice": `{"catalog-image":0.1}`, "GroupRatio": `{"default":1}`})
	c, info, request := catalogImageRequest(t, db, `{"model":"catalog-image","prompt":"cat"}`, "catalog-image", "catalog-image", constant.ChannelTypeOpenAI, false)
	require.NoError(t, model.WithCatalogWriteBarrier(func() error {
		apiErr := prepareSelectedImageBilling(c, info, request, request.GetTokenCountMeta(), 1)
		require.NotNil(t, apiErr)
		assert.ErrorIs(t, apiErr, model.ErrCatalogWriterBusy)
		assert.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
		assert.EqualValues(t, "catalog_pricing_unavailable", apiErr.GetErrorCode())
		assert.False(t, shouldRetry(c, apiErr, 3))
		assert.Nil(t, info.Billing, "unavailable selection must stop before reservation")
		return nil
	}))
}

func TestCatalogPricingUnavailableTransportBilling(t *testing.T) {
	db := catalogControllerPostgres(t, map[string]string{"ModelPrice": `{"catalog-transport":0.1}`, "GroupRatio": `{"default":1}`})
	c, info, _ := catalogImageRequest(t, db, `{"model":"catalog-transport","prompt":"cat"}`, "catalog-transport", "catalog-transport", constant.ChannelTypeOpenAI, false)
	require.NoError(t, model.WithCatalogWriteBarrier(func() error {
		apiErr := relay.PrepareRequestBilling(c, info)
		require.NotNil(t, apiErr)
		assert.ErrorIs(t, apiErr, model.ErrCatalogWriterBusy)
		assert.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode, "shared HTTP/WebSocket billing must retain the availability status")
		assert.EqualValues(t, "catalog_pricing_unavailable", apiErr.GetErrorCode())
		assert.Nil(t, info.Billing)
		return nil
	}))
}

func TestCatalogPricingUnavailableRelayResponse(t *testing.T) {
	db := catalogControllerPostgres(t, map[string]string{"ModelPrice": `{"catalog-relay":0.1}`, "GroupRatio": `{"default":1}`})
	user := model.User{Username: "catalog-relay-test", AffCode: "catalog-relay-test", Group: "default", Quota: 2000000}
	require.NoError(t, db.Create(&user).Error)
	cache, err := model.GetUserCache(user.Id)
	require.NoError(t, err)
	previousCount := constant.CountToken
	constant.CountToken = false
	t.Cleanup(func() { constant.CountToken = previousCount })
	for _, format := range []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude} {
		t.Run(string(format), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			path := "/v1/chat/completions"
			if format == types.RelayFormatClaude {
				path = "/v1/messages"
			}
			c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"catalog-relay","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
			c.Request.Header.Set("Content-Type", "application/json")
			cache.WriteContext(c)
			c.Set("id", user.Id)
			c.Set(common.RequestIdKey, "catalog-fixture-request")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "catalog-relay")
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
			t.Cleanup(func() { common.CleanupBodyStorage(c) })
			require.NoError(t, model.WithCatalogWriteBarrier(func() error {
				Relay(c, format)
				return nil
			}))
			assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			var response struct {
				Error struct {
					Code    string `json:"code"`
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			if format == types.RelayFormatClaude {
				assert.Equal(t, "catalog_pricing_unavailable", response.Error.Type)
			} else {
				assert.Equal(t, "catalog_pricing_unavailable", response.Error.Code)
			}
			assert.Contains(t, response.Error.Message, "catalog-fixture-request")
		})
	}
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, 2000000, after.Quota)
	var count int64
	require.NoError(t, db.Model(&model.Log{}).Where("type = ?", model.LogTypeConsume).Count(&count).Error)
	assert.Zero(t, count, "unavailable pricing cannot produce a consumption log")
}

func TestCatalogPricingUnavailableTaskSelection(t *testing.T) {
	db := catalogControllerPostgres(t, map[string]string{"ModelPrice": `{"catalog-task":0.1}`, "GroupRatio": `{"default":1}`})
	const source = `
export const meta={apiVersion:1,key:"catalog-errors",name:"Catalog errors",version:"1.0.0",author:{name:"Test"},models:["catalog-task"],fetchMode:"per_task",routes:[{method:"POST",path:"/catalog-errors/jobs",type:"submit",decode:"decode",render:"created"}]};
export function buildSubmitRequest(){return {url:"https://unused.example/jobs",body:{}};}
export function parseSubmitResponse(){return {taskId:"unused"};}
export function buildQueryRequest(){return {url:"https://unused.example/jobs"};}
export function parseTaskResult(){return {status:"SUCCESS"};}
export const native={decode:function(){return {kind:"submit",model:"catalog-task"};},created:function(ctx,task){return task;},%s};
`
	plugin, err := jsplugin.CompilePlugin(fmt.Sprintf(source, `error:function(ctx,error){return error;}`), jsplugin.Options{})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.CatalogSyncState{}).Where("id = ?", model.CatalogSyncStateID).Update("publication_state", "committed_pending_publish").Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://unused.example")
	c.Set("task_request", relaycommon.TaskSubmitReq{Model: "catalog-task", Prompt: "cat"})
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Plugin: plugin})
	info := &relaycommon.RelayInfo{OriginModelName: "catalog-task", UsingGroup: "default", UserGroup: "default", ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "catalog-task"}}
	_, taskErr := relay.EstimateTaskSubmit(c, info)
	require.NotNil(t, taskErr)
	assert.ErrorIs(t, taskErr.Error, model.ErrCatalogPublicationPending)
	assert.Equal(t, http.StatusServiceUnavailable, taskErr.StatusCode)
	assert.Equal(t, "catalog_pricing_unavailable", taskErr.Code)
	assert.True(t, taskErr.LocalError)
	assert.True(t, taskErr.NoRetry)
	assert.Equal(t, "stop", decideTaskRetry(c, taskErr, 3).Action)
	apiErr := taskSubmissionAPIError(taskErr)
	assert.EqualValues(t, "catalog_pricing_unavailable", apiErr.GetErrorCode())
	assert.Nil(t, info.Billing)
	var count int64
	require.NoError(t, db.Model(&model.Task{}).Count(&count).Error)
	assert.Zero(t, count)
	for _, tc := range []struct {
		name, renderer string
		native         bool
	}{
		{"native renderer", `error:function(ctx,error){return error;}`, true},
		{"missing renderer", "", false},
		{"throwing renderer", `error:function(){throw new Error("renderer-private-detail");}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			routePlugin, compileErr := jsplugin.CompilePlugin(fmt.Sprintf(source, tc.renderer), jsplugin.Options{})
			require.NoError(t, compileErr)
			router := gin.New()
			router.POST("/catalog-errors/jobs", func(c *gin.Context) {
				c.Set(common.RequestIdKey, "catalog-native-request")
				c.Set(jsplugin.ContextKeyPinnedRoute, jsplugin.PinnedRoute{Plugin: routePlugin, Route: routePlugin.Meta.Routes[0]})
				c.Set(jsplugin.ContextKeyRouteRequest, jsplugin.RouteRequestContext{Path: "/catalog-errors/jobs", Method: http.MethodPost})
				respondTaskSubmissionError(c, taskErr)
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/catalog-errors/jobs", nil))
			assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			var body map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
			assert.Equal(t, "catalog_pricing_unavailable", body["code"])
			if tc.native {
				assert.Equal(t, false, body["retryable"])
				assert.EqualValues(t, 503, body["httpStatus"])
				assert.Equal(t, "catalog-native-request", body["requestId"])
			} else {
				assert.Contains(t, body["message"], "catalog-native-request")
			}
			assert.Contains(t, body["message"], "Pricing service is temporarily unavailable")
			assert.NotContains(t, recorder.Body.String(), "catalog publication pending recovery")
			assert.NotContains(t, recorder.Body.String(), "renderer-private-detail")
		})
	}
}
