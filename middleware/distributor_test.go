package middleware

import (
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
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelMatchesExpectedTaskPluginUsesGenericChannelSetting(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeTaskPlugin}
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "generic-alpha"})

	assert.True(t, channelMatchesExpectedTaskPlugin(nil, channel, "generic-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, "generic-beta"))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, ""))
}

func TestChannelMatchesExpectedTaskPluginUsesPinnedLegacyIndex(t *testing.T) {
	registry := jsplugin.NewRegistry()
	alpha, err := registry.Register(distributorTaskPluginSource("legacy-alpha", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)
	pinnedGeneration := registry.Generation()

	require.NoError(t, registry.Unregister("legacy-alpha"))
	_, err = registry.Register(distributorTaskPluginSource("legacy-beta", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{
		Generation: pinnedGeneration,
		Plugin:     alpha,
	})
	channel := &model.Channel{Type: constant.ChannelTypeKling}

	assert.True(t, channelMatchesExpectedTaskPlugin(c, channel, "legacy-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, channel, "legacy-beta"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "legacy-alpha"))
}

func TestChannelMatchesExpectedTaskPluginRejectsUnindexedLegacyChannel(t *testing.T) {
	registry := jsplugin.NewRegistry()
	plugin, err := registry.Register(distributorTaskPluginSource("legacy-alpha", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{
		Generation: registry.Generation(),
		Plugin:     plugin,
	})

	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "legacy-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: 0}, "legacy-alpha"))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, ""))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, &model.Channel{Type: constant.ChannelTypeKling}, "legacy-alpha"))

	c.Set("expected_task_plugin_key", "legacy-alpha")
	setupErr := SetupContextForSelectedChannel(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "task-model")
	require.NotNil(t, setupErr)
	assert.Contains(t, setupErr.Error(), "does not match")
}

func TestSharedEndpointRebindsToSelectedLegacyProvider(t *testing.T) {
	registry := jsplugin.NewRegistry()
	_, err := registry.Register(distributorEndpointPluginSource("gemini-shared", constant.ChannelTypeGemini), jsplugin.Options{})
	require.NoError(t, err)
	_, err = registry.Register(distributorEndpointPluginSource("vertex-shared", constant.ChannelTypeVertexAi), jsplugin.Options{})
	require.NoError(t, err)
	candidates := registry.Generation().LookupEndpointCandidates("POST", "/v1/responses", "task-model")
	require.Len(t, candidates, 2)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: registry.Generation(), Plugin: candidates[0].Plugin})
	c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{
		Generation: registry.Generation(),
		Plugin:     candidates[0].Plugin,
		Protocol:   candidates[0].Protocol,
		Operation:  candidates[0].Operation,
		Model:      "task-model",
		Candidates: candidates,
	})
	c.Set("expected_task_plugin_key", candidates[0].Plugin.Meta.Key)
	c.Set(jsplugin.ContextKeyProtocolRequest, jsplugin.ProtocolRequestContext{
		Protocol:  "stale-protocol",
		Operation: "stale-operation",
		Model:     "task-model",
	})
	c.Set("task_request", map[string]any{"candidate": "gemini-shared"})
	c.Set("task_action", "gemini-action")

	geminiChannel := &model.Channel{Id: 1, Type: constant.ChannelTypeGemini}
	vertexChannel := &model.Channel{Id: 2, Type: constant.ChannelTypeVertexAi}
	assert.True(t, channelMatchesExpectedTaskPlugin(c, geminiChannel, candidates[0].Plugin.Meta.Key))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, vertexChannel, candidates[0].Plugin.Meta.Key))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeKling}, candidates[0].Plugin.Meta.Key))

	require.Nil(t, SetupContextForSelectedChannel(c, vertexChannel, "task-model"))
	pinnedValue, exists := c.Get(jsplugin.ContextKeyPinnedEndpoint)
	require.True(t, exists)
	pinned, ok := pinnedValue.(jsplugin.PinnedEndpoint)
	require.True(t, ok)
	assert.Equal(t, "vertex-shared", pinned.Plugin.Meta.Key)
	assert.Equal(t, "vertex-shared", c.GetString("expected_task_plugin_key"))
	assert.Equal(t, "vertex-shared", c.GetString("task_plugin_key"))
	protocolRequest := c.MustGet(jsplugin.ContextKeyProtocolRequest).(jsplugin.ProtocolRequestContext)
	assert.Equal(t, candidates[1].Protocol, protocolRequest.Protocol)
	assert.Equal(t, candidates[1].Operation.Name, protocolRequest.Operation)
	_, hasTaskRequest := c.Get("task_request")
	assert.False(t, hasTaskRequest, "a rebound candidate must not inherit the first candidate request")
	_, hasTaskAction := c.Get("task_action")
	assert.False(t, hasTaskAction, "a rebound candidate must not inherit the first candidate action")
	assert.True(t, channelMatchesExpectedTaskPlugin(c, geminiChannel, "vertex-shared"), "a retry may select another declared provider")
	require.Nil(t, SetupContextForSelectedChannel(c, geminiChannel, "task-model"))
	assert.Equal(t, "gemini-shared", c.GetString("expected_task_plugin_key"))
	assert.Equal(t, "gemini-shared", c.GetString("task_plugin_key"))
	assert.Equal(t, "gemini-shared", c.GetString("platform"))
}

func TestDistributeSwitchesSelectedCandidateToItsValidatedOriginChannel(t *testing.T) {
	require.NoError(t, i18n.Init())
	originalMemoryCache := common.MemoryCacheEnabled
	originalDB := model.DB
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCache
		if originalMemoryCache && originalDB != nil {
			model.InitChannelCache()
		}
	})
	setupOriginTaskDB(t)
	common.MemoryCacheEnabled = true
	require.NoError(t, model.DB.AutoMigrate(&model.Ability{}))

	const (
		alphaKey = "origin-pair-alpha"
		betaKey  = "origin-pair-beta"
		modelKey = "origin-pair-model"
	)
	for _, spec := range []struct {
		key    string
		decode string
	}{
		{key: alphaKey, decode: `return {kind:"submit",model:ctx.model,requestBody:ctx.body.value}`},
		{key: betaKey, decode: `return {kind:"submit",model:ctx.model,originTaskIds:["paired-origin"],requestBody:ctx.body.value}`},
	} {
		source := taskResponsesPluginSource(spec.key, 0, `["`+modelKey+`"]`, `["sync"]`, `renderFinal:function(){return {};}`, spec.decode)
		_, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
		require.NoError(t, err)
		key := spec.key
		t.Cleanup(func() { require.NoError(t, jsplugin.DefaultRegistry.Unregister(key)) })
	}

	selected := &model.Channel{Name: "random-beta", Key: "sk-random", Status: common.ChannelStatusEnabled, Type: constant.ChannelTypeTaskPlugin, Group: "default", Models: modelKey}
	selected.SetSetting(dto.ChannelSettings{TaskPluginKey: betaKey})
	require.NoError(t, selected.Insert())
	origin := &model.Channel{Name: "origin-beta", Key: "sk-origin", Status: common.ChannelStatusEnabled, Type: constant.ChannelTypeTaskPlugin, Group: "origin-only", Models: modelKey}
	origin.SetSetting(dto.ChannelSettings{TaskPluginKey: betaKey})
	require.NoError(t, origin.Insert())
	insertOriginOwnedTask(t, "paired-origin", 7, origin.Id, constant.TaskPlatform(betaKey))
	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	router := gin.New()
	router.POST("/v1/responses", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, 7)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
	}, PinTaskPluginEndpoint(), PrepareTaskPluginEndpoint(), Distribute(), func(c *gin.Context) {
		assert.Equal(t, origin.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
		assert.Equal(t, betaKey, c.GetString("task_plugin_key"))
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"`+modelKey+`","prompt":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
}

func TestSetupContextForSelectedNativeChannelClearsStalePluginIdentity(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Set("expected_task_plugin_key", "")
	c.Set("task_plugin_key", "stale-plugin")
	c.Set("platform", "stale-plugin")
	channel := &model.Channel{Id: 62, Type: constant.ChannelTypeMoliiGrokAIGC}

	require.Nil(t, SetupContextForSelectedChannel(c, channel, "grok-imagine-video"))
	assert.Empty(t, c.GetString("task_plugin_key"))
	assert.Empty(t, c.GetString("platform"))
}

func TestChannelMatchesExpectedTaskPluginAllowsUnifiedStarAIEndpoint(t *testing.T) {
	registry := jsplugin.NewRegistry()
	plugin, err := registry.Register(distributorOpenAIVideoPluginSource(), jsplugin.Options{})
	require.NoError(t, err)
	generation := registry.Generation()
	const modelName = "doubao-seedance-2-0-fast-260128"

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: generation, Plugin: plugin})
	c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{
		Generation: generation,
		Plugin:     plugin,
		Protocol:   "openai_video",
		Model:      modelName,
	})
	c.Set("expected_task_plugin_key", "doubao")

	channel := &model.Channel{
		Id:     61,
		Type:   constant.ChannelTypeStarAI,
		Name:   "starai-fast",
		Key:    "sk-test",
		Models: modelName,
	}
	assert.True(t, channelMatchesExpectedTaskPlugin(c, channel, "doubao"))
	require.Nil(t, SetupContextForSelectedChannel(c, channel, modelName))
	assert.Equal(t, constant.ChannelTypeStarAI, c.GetInt(string(constant.ContextKeyChannelType)))
	assert.Equal(t, "https://openapi.starcube.art", c.GetString(string(constant.ContextKeyChannelBaseUrl)))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeMoliiGrokAIGC}, "doubao"))
}

func TestUnifiedStarAIEndpointRebindsToAcceptedDoubaoCandidate(t *testing.T) {
	registry := jsplugin.NewRegistry()
	_, err := registry.Register(distributorSharedVideoPluginSource("alpha-video", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)
	_, err = registry.Register(distributorSharedVideoPluginSource("doubao", constant.ChannelTypeDoubaoVideo), jsplugin.Options{})
	require.NoError(t, err)
	candidates := registry.Generation().LookupEndpointCandidates("POST", "/v1/videos", "doubao-seedance-2-0-fast-260128")
	require.Len(t, candidates, 2)
	require.Equal(t, "alpha-video", candidates[0].Plugin.Meta.Key)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: registry.Generation(), Plugin: candidates[0].Plugin})
	c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{
		Generation: registry.Generation(),
		Plugin:     candidates[0].Plugin,
		Protocol:   candidates[0].Protocol,
		Operation:  candidates[0].Operation,
		Model:      "doubao-seedance-2-0-fast-260128",
		Candidates: candidates,
	})
	c.Set("expected_task_plugin_key", candidates[0].Plugin.Meta.Key)
	channel := &model.Channel{Id: 61, Type: constant.ChannelTypeStarAI, Key: "sk-test"}

	selected, matched := pinnedEndpointCandidateForChannel(c, channel, candidates[0].Plugin.Meta.Key)
	require.True(t, matched)
	require.NotNil(t, selected.Plugin)
	assert.Equal(t, "doubao", selected.Plugin.Meta.Key)
	require.Nil(t, SetupContextForSelectedChannel(c, channel, "doubao-seedance-2-0-fast-260128"))
	assert.Equal(t, "doubao", c.GetString("expected_task_plugin_key"))
	assert.Equal(t, "doubao", c.GetString("task_plugin_key"))
}

func distributorTaskPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["task-model"],
  fetchMode: "per_task",
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
`, key, key, channelType)
}

func distributorEndpointPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["task-model"],
  fetchMode: "per_task",
  protocols: [{name: "openai_responses", supports: ["stream", "sync", "background"]}],
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
export const protocols = {openai_responses: {
  decodeRequest: function(ctx) { return {kind: "submit", model: "task-model", requestBody: ctx.body.value}; },
  renderEvents: function() { return {events: [], state: null, done: false}; },
  renderFinal: function() { return {output: []}; },
}};
`, key, key, channelType)
}

func distributorOpenAIVideoPluginSource() string {
	return `
export const meta = {
  apiVersion: 1,
  key: "doubao",
  name: "Doubao Test",
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [54, 45],
  models: ["doubao-seedance-2-0-fast-260128"],
  fetchMode: "per_task",
  protocols: ["openai_video"],
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
export function listArtifacts() { return []; }
export function buildContentRequest() { return {url: "https://example.com"}; }
export const protocols = {openai_video: {
  decodeRequest: function(ctx) { return {kind: "submit", model: ctx.model, requestBody: ctx.body.value}; },
  render: function(_ctx, task) { return task; },
}};
`
}

func distributorSharedVideoPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["doubao-seedance-2-0-fast-260128"],
  fetchMode: "per_task",
  protocols: ["openai_video"],
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
export function listArtifacts() { return []; }
export function buildContentRequest() { return {url: "https://example.com"}; }
export const protocols = {openai_video: {
  decodeRequest: function(ctx) { return {kind: "submit", model: ctx.model, requestBody: ctx.body.value}; },
  render: function(_ctx, task) { return task; },
}};
`, key, key, channelType)
}

func TestTokenModelLimitAllowsLegacyAliasAndModifierVariant(t *testing.T) {
	aliasOnly := map[string]bool{"claude-3-7-sonnet-thinking": true}
	assert.True(t, tokenModelLimitAllows(aliasOnly, "claude-3-7-sonnet-thinking"))
	assert.False(t, tokenModelLimitAllows(aliasOnly, "claude-3-7-sonnet"))

	baseOnly := map[string]bool{"claude-3-7-sonnet": true}
	assert.True(t, tokenModelLimitAllows(baseOnly, "claude-3-7-sonnet@thinking:on"))
	assert.True(t, tokenModelLimitAllows(baseOnly, "claude-3-7-sonnet-thinking"))

	wildcard := map[string]bool{"gemini-2.5-flash-thinking-*": true}
	assert.True(t, tokenModelLimitAllows(wildcard, "gemini-2.5-flash-thinking-8192"))
}

func TestTokenModelLimitAllowsExemptAtNameByFullName(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	original := append([]string(nil), settings.ThinkingModelBlacklist...)
	t.Cleanup(func() { settings.ThinkingModelBlacklist = original })
	settings.ThinkingModelBlacklist = append(original, "re:.*@sha256:.*")

	fullOnly := map[string]bool{"opaque@sha256:deadbeef": true}
	assert.True(t, tokenModelLimitAllows(fullOnly, "opaque@sha256:deadbeef"))

	baseOnly := map[string]bool{"opaque": true}
	assert.False(t, tokenModelLimitAllows(baseOnly, "opaque@sha256:deadbeef"))
}

func TestNoAvailableChannelMessageNamesClaimingTaskPlugin(t *testing.T) {
	require.NoError(t, i18n.Init())
	registry := jsplugin.NewRegistry()
	plugin, err := registry.Register(distributorTaskPluginSource("claimer", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	pinned, _ := gin.CreateTestContext(nil)
	pinned.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	pinned.Request.Header.Set("Accept-Language", "en")
	pinned.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: registry.Generation(), Plugin: plugin})
	message := noAvailableChannelMessage(pinned, "default", "kling-v1")
	assert.Contains(t, message, `"claimer"`)
	assert.Contains(t, message, "disable or override")
	assert.Contains(t, message, "kling-v1")

	plain, _ := gin.CreateTestContext(nil)
	plain.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	plain.Request.Header.Set("Accept-Language", "en")
	generic := noAvailableChannelMessage(plain, "default", "gpt-4o")
	assert.NotContains(t, generic, "task plugin")
	assert.Contains(t, generic, "gpt-4o")
}

func TestSharedEndpointRebindsToSelectedType61Plugin(t *testing.T) {
	registry := jsplugin.NewRegistry()
	for _, key := range []string{"alpha", "beta"} {
		source := strings.Replace(distributorEndpointPluginSource(key, 0), "channelTypes: [0],", "", 1)
		_, err := registry.Register(source, jsplugin.Options{})
		require.NoError(t, err)
	}
	generation := registry.Generation()
	candidates := generation.LookupEndpointCandidates("POST", "/v1/responses", "task-model")
	require.Len(t, candidates, 2)
	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: generation, Plugin: candidates[0].Plugin})
	c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{Generation: generation, Plugin: candidates[0].Plugin, Protocol: candidates[0].Protocol, Operation: candidates[0].Operation, Model: "task-model", Candidates: candidates})
	c.Set("expected_task_plugin_key", "alpha")
	channel := &model.Channel{Id: 2, Type: constant.ChannelTypeTaskPlugin}
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "unrelated"})
	assert.False(t, channelMatchesExpectedTaskPlugin(c, channel, "alpha"))
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "beta"})
	require.Nil(t, SetupContextForSelectedChannel(c, channel, "task-model"))
	assert.Equal(t, "beta", c.GetString("task_plugin_key"))
	assert.Equal(t, "beta", c.GetString("expected_task_plugin_key"))
	assert.Equal(t, "beta", c.MustGet(jsplugin.ContextKeyPinnedEndpoint).(jsplugin.PinnedEndpoint).Plugin.Meta.Key)
	require.NoError(t, i18n.Init())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	assert.Contains(t, noAvailableChannelMessage(c, "default", "task-model"), "alpha, beta")
}
