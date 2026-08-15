package controller

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelMetaCatalogControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "model-meta-catalog.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Model{}, &model.Vendor{}, &model.Channel{}, &model.Ability{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		model.InvalidatePricingCache()
	})
	return db
}

func modelMetaRequestContext(method string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, "/api/models/", bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")
	return context, recorder
}

func TestCreateModelMetaPersistsNormalizedCatalogFields(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	context, recorder := modelMetaRequestContext("POST", `{
      "model_name":"catalog-controller-model","status":1,"sync_official":1,
      "context_length":1000000,"max_output_tokens":65536,
      "release_date":"2026-02-16","metadata_verified_at":"2026-08-13",
      "input_modalities":[" text ","image","text"],"output_modalities":["text"],
      "capabilities":[" tools ","streaming","tools"],"metadata_source":" models.dev "
    }`)

	CreateModelMeta(context)

	assert.Equal(t, 200, recorder.Code)
	var entry model.Model
	require.NoError(t, db.Where("model_name = ?", "catalog-controller-model").First(&entry).Error)
	assert.Equal(t, []string{"text", "image"}, entry.InputModalities)
	assert.Equal(t, []string{"tools", "streaming"}, entry.Capabilities)
	assert.Equal(t, "models.dev", entry.MetadataSource)
}

func TestCreateModelMetaRejectsInvalidCatalogFields(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	context, recorder := modelMetaRequestContext("POST", `{
      "model_name":"invalid-catalog-model","status":1,"context_length":-1,
      "release_date":"2026/02/16"
    }`)

	CreateModelMeta(context)

	var count int64
	require.NoError(t, db.Model(&model.Model{}).Count(&count).Error)
	assert.Zero(t, count)
	assert.True(t, strings.Contains(recorder.Body.String(), "context_length") || strings.Contains(recorder.Body.String(), "release_date"))
}

func TestUpdateModelMetaRejectsInvalidCatalogFieldsWithoutChangingRow(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := model.Model{ModelName: "existing-catalog-model", Description: "original", Status: 1, SyncOfficial: 1, ContextLength: 128_000}
	require.NoError(t, entry.Insert())
	context, recorder := modelMetaRequestContext("PUT", `{
      "id":1,"model_name":"existing-catalog-model","description":"changed","status":1,
      "context_length":-1,"release_date":"invalid"
    }`)

	UpdateModelMeta(context)

	var stored model.Model
	require.NoError(t, db.First(&stored, entry.Id).Error)
	assert.Equal(t, "original", stored.Description)
	assert.Equal(t, 128_000, stored.ContextLength)
	assert.Contains(t, recorder.Body.String(), "context_length")
}

type modelMetaResponse struct {
	Success       bool        `json:"success"`
	Code          string      `json:"code"`
	Message       string      `json:"message"`
	MissingFields []string    `json:"missing_fields"`
	Data          model.Model `json:"data"`
}

func completeModelMetaFixture(t *testing.T, db *gorm.DB, name string, published bool) model.Model {
	t.Helper()
	vendor := model.Vendor{Name: "Vendor " + name, Status: 1}
	require.NoError(t, vendor.Insert())
	entry := model.Model{
		ModelName:           name,
		DisplayName:         "Complete model",
		Description:         "Complete description",
		VendorID:            vendor.Id,
		Status:              1,
		SyncOfficial:        1,
		ReleaseDate:         "2026-08-15",
		InputModalities:     []string{"text"},
		OutputModalities:    []string{"text"},
		Capabilities:        []string{"streaming"},
		SupportedParameters: []string{"stream"},
		ContextLength:       128_000,
		MaxOutputTokens:     8_000,
		MarketplaceEnabled:  published,
	}
	require.NoError(t, db.Create(&entry).Error)
	return entry
}

func completeModelMetaPayload(entry model.Model) map[string]any {
	return map[string]any{
		"id":                   entry.Id,
		"model_name":           entry.ModelName,
		"display_name":         entry.DisplayName,
		"description":          entry.Description,
		"vendor_id":            entry.VendorID,
		"status":               entry.Status,
		"sync_official":        entry.SyncOfficial,
		"release_date":         entry.ReleaseDate,
		"input_modalities":     entry.InputModalities,
		"output_modalities":    entry.OutputModalities,
		"capabilities":         entry.Capabilities,
		"supported_parameters": entry.SupportedParameters,
		"context_length":       entry.ContextLength,
		"max_output_tokens":    entry.MaxOutputTokens,
		"marketplace_enabled":  entry.MarketplaceEnabled,
	}
}

func invokeModelMetaJSON(t *testing.T, method string, payload map[string]any, handler func(*gin.Context)) modelMetaResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	context, recorder := modelMetaRequestContext(method, string(body))
	handler(context)
	require.Equal(t, 200, recorder.Code)
	var response modelMetaResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response), recorder.Body.String())
	return response
}

func TestUpdateModelMetaRejectsChangedModelName(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := completeModelMetaFixture(t, db, "immutable-model", false)
	payload := completeModelMetaPayload(entry)
	payload["model_name"] = "renamed-model"

	response := invokeModelMetaJSON(t, "PUT", payload, UpdateModelMeta)

	assert.False(t, response.Success)
	assert.Equal(t, "model_name_immutable", response.Code)
	var stored model.Model
	require.NoError(t, db.First(&stored, entry.Id).Error)
	assert.Equal(t, "immutable-model", stored.ModelName)
}

func TestCreateModelMetaRejectsIncompleteMarketplacePublicationWithAllMissingFields(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	payload := map[string]any{
		"model_name":          "incomplete-publication",
		"status":              1,
		"marketplace_enabled": true,
		"input_modalities":    []string{"text"},
		"output_modalities":   []string{"text"},
	}

	response := invokeModelMetaJSON(t, "POST", payload, CreateModelMeta)

	assert.False(t, response.Success)
	assert.Equal(t, "marketplace_metadata_incomplete", response.Code)
	assert.Equal(t, []string{
		"capabilities", "context_length", "description", "display_name", "max_output_tokens", "release_date", "supported_parameters", "vendor_id",
	}, response.MissingFields)
	var count int64
	require.NoError(t, db.Model(&model.Model{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestCreateModelMetaAllowsCompleteMarketplacePublication(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := completeModelMetaFixture(t, db, "complete-template", false)
	require.NoError(t, db.Delete(&entry).Error)
	payload := completeModelMetaPayload(entry)
	payload["id"] = 0
	payload["model_name"] = "complete-publication"
	payload["marketplace_enabled"] = true

	response := invokeModelMetaJSON(t, "POST", payload, CreateModelMeta)

	require.True(t, response.Success, response.Message)
	assert.True(t, response.Data.MarketplaceEnabled)
	assert.True(t, response.Data.MarketplaceComplete)
	assert.Equal(t, "llm", response.Data.MarketplaceCategory)
	var stored model.Model
	require.NoError(t, db.Where("model_name = ?", "complete-publication").First(&stored).Error)
	assert.True(t, stored.MarketplaceEnabled)
}

func TestCreateModelMetaAllowsIncompleteDraft(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	payload := map[string]any{
		"model_name":          "incomplete-draft",
		"status":              1,
		"marketplace_enabled": false,
	}

	response := invokeModelMetaJSON(t, "POST", payload, CreateModelMeta)

	require.True(t, response.Success, response.Message)
	assert.False(t, response.Data.MarketplaceEnabled)
	assert.False(t, response.Data.MarketplaceComplete)
	assert.Contains(t, response.Data.MarketplaceMissingFields, "description")
	var stored model.Model
	require.NoError(t, db.Where("model_name = ?", "incomplete-draft").First(&stored).Error)
	assert.False(t, stored.MarketplaceEnabled)
}

func TestUpdateModelMetaRejectsIncompleteNewPublicationWithoutChangingDraft(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := completeModelMetaFixture(t, db, "draft-publication", false)
	payload := completeModelMetaPayload(entry)
	payload["description"] = ""
	payload["marketplace_enabled"] = true

	response := invokeModelMetaJSON(t, "PUT", payload, UpdateModelMeta)

	assert.False(t, response.Success)
	assert.Equal(t, "marketplace_metadata_incomplete", response.Code)
	assert.Equal(t, []string{"description"}, response.MissingFields)
	var stored model.Model
	require.NoError(t, db.First(&stored, entry.Id).Error)
	assert.Equal(t, "Complete description", stored.Description)
	assert.False(t, stored.MarketplaceEnabled)
}

func TestUpdateModelMetaAllowsCompleteNewPublication(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := completeModelMetaFixture(t, db, "complete-draft-publication", false)
	payload := completeModelMetaPayload(entry)
	payload["marketplace_enabled"] = true

	response := invokeModelMetaJSON(t, "PUT", payload, UpdateModelMeta)

	require.True(t, response.Success, response.Message)
	assert.True(t, response.Data.MarketplaceEnabled)
	assert.True(t, response.Data.MarketplaceComplete)
	var stored model.Model
	require.NoError(t, db.First(&stored, entry.Id).Error)
	assert.True(t, stored.MarketplaceEnabled)
}

func TestUpdateModelMetaWithdrawsIncompletePublishedModel(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := completeModelMetaFixture(t, db, "published-model", true)
	payload := completeModelMetaPayload(entry)
	payload["description"] = ""

	response := invokeModelMetaJSON(t, "PUT", payload, UpdateModelMeta)

	require.True(t, response.Success, response.Message)
	assert.True(t, response.Data.MarketplaceWithdrawn)
	assert.False(t, response.Data.MarketplaceEnabled)
	assert.Contains(t, response.Data.MarketplaceMissingFields, "description")
	var stored model.Model
	require.NoError(t, db.First(&stored, entry.Id).Error)
	assert.Empty(t, stored.Description)
	assert.False(t, stored.MarketplaceEnabled)
}

func TestGetModelMetaDynamicBlockersDoNotChangePublicationIntent(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := completeModelMetaFixture(t, db, "gpt-4", true)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: strconv.Itoa(entry.Id)}}
	context.Request = httptest.NewRequest("GET", "/api/models/"+strconv.Itoa(entry.Id), nil)

	GetModelMeta(context)

	var response modelMetaResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response), recorder.Body.String())
	require.True(t, response.Success, response.Message)
	assert.True(t, response.Data.MarketplaceEnabled)
	assert.True(t, response.Data.MarketplaceComplete)
	assert.False(t, response.Data.MarketplaceVisible)
	assert.Equal(t, []string{"pricing_missing", "group_unavailable", "endpoint_unavailable"}, response.Data.MarketplaceBlockers)
	var stored model.Model
	require.NoError(t, db.First(&stored, entry.Id).Error)
	assert.True(t, stored.MarketplaceEnabled)
}

func TestModelMarketplaceBlockersAreStableAndComplete(t *testing.T) {
	assert.Equal(t,
		[]string{"vendor_disabled", "pricing_missing", "group_unavailable", "endpoint_unavailable"},
		model.EvaluateMarketplaceBlockers(false, false, 0, 0),
	)
	assert.Empty(t, model.EvaluateMarketplaceBlockers(true, true, 1, 1))
}

func insertRuleMarketplaceRuntime(t *testing.T, db *gorm.DB, ruleName string, endpoints string, channelType int, settings dto.ChannelOtherSettings) model.Model {
	t.Helper()
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
	vendor := model.Vendor{Name: "Vendor " + ruleName, Status: 1}
	require.NoError(t, vendor.Insert())
	entry := model.Model{
		ModelName:           ruleName,
		NameRule:            model.NameRulePrefix,
		DisplayName:         "Rule model",
		Description:         "Rule description",
		VendorID:            vendor.Id,
		Endpoints:           endpoints,
		Status:              1,
		SyncOfficial:        1,
		ReleaseDate:         "2026-08-15",
		InputModalities:     []string{"text"},
		OutputModalities:    []string{"text"},
		Capabilities:        []string{"streaming"},
		SupportedParameters: []string{"stream"},
		ContextLength:       128_000,
		MaxOutputTokens:     8_000,
		MarketplaceEnabled:  true,
	}
	require.NoError(t, db.Create(&entry).Error)

	channel := model.Channel{
		Name:   "Channel " + ruleName,
		Key:    "not-a-real-key",
		Type:   channelType,
		Status: common.ChannelStatusEnabled,
	}
	if settings.AdvancedCustom != nil {
		channel.SetOtherSettings(settings)
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     ruleName + "child",
		ChannelId: channel.Id,
		Enabled:   true,
	}).Error)
	model.InvalidatePricingCache()
	return entry
}

func advancedCustomRouteForOtherModel() dto.ChannelOtherSettings {
	return dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{
			Routes: []dto.AdvancedCustomRoute{{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Models:       []string{"some-other-model"},
			}},
		},
	}
}

func TestModelMarketplaceRuleCustomObjectEndpointUsesRuntimeEndpointSet(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := insertRuleMarketplaceRuntime(t, db, "custom-object-rule-", `{"custom_chat":"/v1/custom/chat"}`, constant.ChannelTypeAdvancedCustom, advancedCustomRouteForOtherModel())

	enrichModels([]*model.Model{&entry})

	assert.NotContains(t, entry.MarketplaceBlockers, "endpoint_unavailable")
	assert.Equal(t, []string{"custom-object-rule-child"}, entry.MatchedModels)
}

func TestModelMarketplaceRuleMatchedRuntimeEndpointsUseUnion(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := insertRuleMarketplaceRuntime(t, db, "runtime-union-rule-", "", constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})

	enrichModels([]*model.Model{&entry})

	assert.NotContains(t, entry.MarketplaceBlockers, "endpoint_unavailable")
	assert.Contains(t, entry.Endpoints, string(constant.EndpointTypeOpenAI))
}

func TestModelMarketplaceRuleStaleEndpointArrayDoesNotCountAsRuntimeEndpoint(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := insertRuleMarketplaceRuntime(t, db, "stale-array-rule-", `["stale_endpoint"]`, constant.ChannelTypeAdvancedCustom, advancedCustomRouteForOtherModel())

	enrichModels([]*model.Model{&entry})

	assert.Contains(t, entry.MarketplaceBlockers, "endpoint_unavailable")
}
