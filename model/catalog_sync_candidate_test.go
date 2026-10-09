package model

import (
	"maps"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/task_pricing_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogCandidateDefaultsIsolation(t *testing.T) {
	ratio := ratio_setting.GetDefaultModelRatioMap()
	price := ratio_setting.GetDefaultModelPriceMap()
	oldRatio, oldPrice := ratio["gpt-4"], price["suno_music"]
	t.Cleanup(func() { ratio["gpt-4"], price["suno_music"] = oldRatio, oldPrice })
	ratio["gpt-4"], price["suno_music"] = 999, 999
	assert.Equal(t, 15.0, ratio_setting.GetDefaultPricingMaps()["ModelRatio"]["gpt-4"], "a caller must not poison compiled reset defaults")
	assert.Equal(t, 0.1, ratio_setting.GetDefaultPricingMaps()["ModelPrice"]["suno_music"])
}

func captureCatalogCandidateOptions() map[string]string {
	options := make(map[string]string)
	whitelist := catalogmanifest.PriceOptions()
	for key, value := range config.GlobalConfig.ExportAllConfigs() {
		if _, exists := whitelist[key]; exists {
			options[key] = value
		}
	}
	for key, value := range map[string]string{
		"ModelPrice": ratio_setting.ModelPrice2JSONString(), "ModelRatio": ratio_setting.ModelRatio2JSONString(),
		"CompletionRatio": ratio_setting.CompletionRatio2JSONString(), "CacheRatio": ratio_setting.CacheRatio2JSONString(),
		"CreateCacheRatio": ratio_setting.CreateCacheRatio2JSONString(), "ImageRatio": ratio_setting.ImageRatio2JSONString(),
		"AudioRatio": ratio_setting.AudioRatio2JSONString(), "AudioCompletionRatio": ratio_setting.AudioCompletionRatio2JSONString(),
	} {
		options[key] = value
	}
	return options
}

func preserveCatalogCandidate(t *testing.T) {
	t.Helper()
	original, err := stageCommittedCatalogPricing(captureCatalogCandidateOptions(), catalogPricingDependencies{})
	require.NoError(t, err)
	common.OptionMapRWMutex.RLock()
	options := maps.Clone(common.OptionMap)
	common.OptionMapRWMutex.RUnlock()
	t.Cleanup(func() {
		require.NoError(t, WithCatalogWriteBarrier(func() error { original.publishGuarded(); return nil }))
		common.OptionMapRWMutex.Lock()
		common.OptionMap = options
		common.OptionMapRWMutex.Unlock()
	})
}

func TestCatalogCandidateCompleteReplacement(t *testing.T) {
	preserveCatalogCandidate(t)
	pointers := map[string]any{}
	for _, namespace := range []string{"billing_setting", "tool_price_setting", "molii_grok_price", "molii_grok_tool_price", "starai_video_price", "task_pricing_setting"} {
		pointers[namespace] = config.GlobalConfig.Get(namespace)
	}
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap["USDExchangeRate"] = "7.25"
	common.OptionMap["SiteName"] = "local brand"
	common.OptionMapRWMutex.Unlock()
	input := map[string]string{
		"ModelPrice":                             `{"historical-orphan":0,"removed":9}`,
		"ModelRatio":                             `{}`,
		"billing_setting.billing_mode":           `{"custom":"tiered_expr"}`,
		"billing_setting.billing_expr":           `{"custom":"tier(\"raw\", p * 3.17)"}`,
		"tool_price_setting.prices":              `{"web_search":12,"web_search:custom*":7,"web_search:custom-long*":0}`,
		"molii_grok_tool_price.image_generation": "0.17",
		"molii_grok_price.image_standard_1k":     "0",
		"starai_video_price.standard_720p":       "13.25",
		"task_pricing_setting.sora_size_ratio":   `{"1792x1024":3}`,
		"USDExchangeRate":                        "99", // unrelated rows must never enter the candidate
	}
	before := captureCatalogCandidateOptions()
	candidate, err := stageCommittedCatalogPricing(input, catalogPricingDependencies{})
	require.NoError(t, err)
	assert.Equal(t, before, captureCatalogCandidateOptions(), "staging is side-effect free")
	input["ModelPrice"] = `{"historical-orphan":999}`
	require.NoError(t, WithCatalogWriteBarrier(func() error { candidate.publishGuarded(); return nil }))
	price, exists := ratio_setting.GetModelPrice("historical-orphan", false)
	assert.True(t, exists)
	assert.Zero(t, price)
	assert.Empty(t, ratio_setting.GetModelRatioCopy(), "configured empty is not defaults")
	expression, exists := billing_setting.GetBillingExpr("custom")
	assert.True(t, exists)
	assert.Equal(t, `tier("raw", p * 3.17)`, expression)
	assert.Equal(t, 12.0, operation_setting.GetToolPriceForModel("web_search", "other"))
	assert.Equal(t, 7.0, operation_setting.GetToolPriceForModel("web_search", "custom-v1"))
	assert.Zero(t, operation_setting.GetToolPriceForModel("web_search", "custom-long-v1"))
	assert.Equal(t, 170.0, operation_setting.GetToolPriceForModel("image_generation", "grok-3"), "Grok per-image prices convert to per-1000 calls")
	output, _, ok := ratio_setting.GetMoliiGrokImagePrices("grok-imagine-image", "1k")
	assert.True(t, ok)
	assert.Zero(t, output)
	assert.Equal(t, 3.0, task_pricing_setting.SoraSizeRatio("1792x1024"))
	for namespace, pointer := range pointers {
		assert.Same(t, pointer, config.GlobalConfig.Get(namespace), namespace)
	}
	common.OptionMapRWMutex.RLock()
	assert.Equal(t, "7.25", common.OptionMap["USDExchangeRate"])
	assert.Equal(t, "local brand", common.OptionMap["SiteName"])
	common.OptionMapRWMutex.RUnlock()
	// A second complete publication removes leaves and resets absent whole options.
	next, err := stageCommittedCatalogPricing(map[string]string{"ModelPrice": `{"historical-orphan":0}`, "billing_setting.billing_mode": `{}`, "billing_setting.billing_expr": `{}`}, catalogPricingDependencies{})
	require.NoError(t, err)
	require.NoError(t, WithCatalogWriteBarrier(func() error { next.publishGuarded(); return nil }))
	_, exists = ratio_setting.GetModelPrice("removed", false)
	assert.False(t, exists)
	assert.Equal(t, 15.0, ratio_setting.GetModelRatioCopy()["gpt-4"])
	_, exists = billing_setting.GetBillingExpr("custom")
	assert.False(t, exists)
	assert.Equal(t, 10.0, operation_setting.GetToolPriceForModel("web_search", "custom-long-v1"))
	assert.Equal(t, 50.0, operation_setting.GetToolPriceForModel("image_generation", "grok-3"))
	output, _, ok = ratio_setting.GetMoliiGrokImagePrices("grok-imagine-image", "1k")
	assert.True(t, ok)
	assert.Equal(t, 0.02, output)
	assert.Equal(t, 1.666667, task_pricing_setting.SoraSizeRatio("1792x1024"))
	defaults, err := stageCommittedCatalogPricing(nil, catalogPricingDependencies{})
	require.NoError(t, err)
	require.NoError(t, WithCatalogWriteBarrier(func() error { defaults.publishGuarded(); return nil }))
	assert.Equal(t, billing_setting.BillingModeTieredExpr, billing_setting.GetBillingMode("minimax-m3"))
}

func TestCatalogCandidateMalformedDoesNotMutate(t *testing.T) {
	preserveCatalogCandidate(t)
	before := captureCatalogCandidateOptions()
	beforeToolPrice := operation_setting.GetToolPriceForModel("web_search_preview", "gpt-4o-mini")
	common.OptionMapRWMutex.RLock()
	beforeOptions := maps.Clone(common.OptionMap)
	common.OptionMapRWMutex.RUnlock()
	for _, invalid := range []map[string]string{
		{"ModelRatio": `{"ok":4,"bad":null}`}, {"ModelRatio": `{"x":1,"x":2}`},
		{"ModelRatio": `null`}, {"ModelPrice": `[]`}, {"ModelPrice": `{"x":-1}`},
		{"ModelPrice": `{"x":1e999}`}, {"ModelPrice": `{"":1}`},
		{"billing_setting.billing_mode": `{"x":"unknown"}`},
		{"billing_setting.billing_expr": `{"x":"tier(\"x\", -1)"}`},
		{"billing_setting.plugin_billing_expr": `{"bad-key":"tier(\"x\", p)"}`},
		{"tool_price_setting.prices": `{"web_search":"0"}`},
		{"molii_grok_price.image_standard_1k": `null`},
		{"starai_video_price.standard_720p": `-1`},
		{"task_pricing_setting.sora_size_ratio": `{"x":null}`},
		{"billing_setting.unknown": `{}`},
		{"ModelPrice": `{"new-orphan":4}`, "molii_grok_price.image_standard_1k": "0", "tool_price_setting.prices": `{"last-field":null}`},
	} {
		candidate, err := stageCommittedCatalogPricing(invalid, catalogPricingDependencies{})
		require.Error(t, err, "%v", invalid)
		assert.Nil(t, candidate)
		assert.Equal(t, before, captureCatalogCandidateOptions())
		assert.Equal(t, beforeToolPrice, operation_setting.GetToolPriceForModel("web_search_preview", "gpt-4o-mini"))
		common.OptionMapRWMutex.RLock()
		assert.Equal(t, beforeOptions, common.OptionMap)
		common.OptionMapRWMutex.RUnlock()
	}
}

func TestCatalogCandidateWholeScopeReset(t *testing.T) {
	preserveCatalogCandidate(t)
	configured := make(map[string]string)
	for key, spec := range catalogmanifest.PriceOptions() {
		configured[key] = "0"
		if spec.Map {
			configured[key] = "{}"
		}
	}
	empty, err := stageCommittedCatalogPricing(configured, catalogPricingDependencies{})
	require.NoError(t, err)
	require.NoError(t, WithCatalogWriteBarrier(func() error { empty.publishGuarded(); return nil }))
	assert.Equal(t, configured, captureCatalogCandidateOptions(), "every scoped map/scalar publishes explicit empty/zero")
	assert.Equal(t, 1.0, task_pricing_setting.SoraSizeRatio("1792x1024"), "empty task factors use the existing resolver fallback")
	assert.Zero(t, operation_setting.GetToolPriceForModel("web_search", "grok-3"), "Grok explicit zero overrides generic fallback")
	// Changing a returned default must not change the next reset, and current
	// zeroed runtime must not become the baseline for missing persisted options.
	billingDefault := billing_setting.DefaultBillingSetting()
	billingDefault.BillingMode["minimax-m3"] = "ratio"
	billingDefault.BillingExpr["minimax-m3"] = "poison"
	factorsDefault := task_pricing_setting.DefaultTaskPricingSetting()
	factorsDefault.SoraSizeRatio["1792x1024"] = 999
	reset, err := stageCommittedCatalogPricing(nil, catalogPricingDependencies{})
	require.NoError(t, err)
	require.NoError(t, WithCatalogWriteBarrier(func() error { reset.publishGuarded(); return nil }))
	assert.Equal(t, "tiered_expr", billing_setting.GetBillingMode("minimax-m3"))
	expression, _ := billing_setting.GetBillingExpr("minimax-m3")
	assert.Contains(t, expression, "p * 2.1")
	assert.Equal(t, 1.666667, task_pricing_setting.SoraSizeRatio("1792x1024"))
	assert.Equal(t, 5.0, operation_setting.GetToolPriceForModel("web_search", "grok-3"))
	assert.Equal(t, 0.01, ratio_setting.GetMoliiGrokPriceSettingCopy().Video15ImageInput)
	assert.Equal(t, 46.0, ratio_setting.GetStarAIVideoPriceSettingCopy().Seedance251080pVideo)
}

func TestCatalogCandidateOrphanIsNotManagedCompleteness(t *testing.T) {
	_, err := stageCommittedCatalogPricing(map[string]string{"ModelPrice": `{"historical-orphan":0}`}, catalogPricingDependencies{})
	require.NoError(t, err)
	source := catalogSyncTestSource(t)
	key, err := catalogmanifest.EncodePriceKey(catalogmanifest.PriceKey{Option: "ModelPrice", Model: "historical-orphan", Path: "/historical-orphan"})
	require.NoError(t, err)
	price, err := common.Marshal(catalogmanifest.PriceValue{Value: "0", BillingCurrency: "USD", Unit: "request"})
	require.NoError(t, err)
	source.Entries = append(source.Entries, catalogmanifest.Entry{Kind: catalogmanifest.KindModelPrice, Key: key, Value: string(price)})
	source.Coverage[catalogmanifest.KindModelPrice]++
	source.Digest, err = catalogmanifest.SnapshotDigest(source)
	require.NoError(t, err)
	require.ErrorContains(t, catalogmanifest.ValidateSnapshot(source), "requires exact metadata", "even explicitly supplied USD cannot make an orphan a complete managed price")
}

func TestCatalogCandidateProspectiveRetention(t *testing.T) {
	expression := `tier("raw", u("seconds") * 0.371)`
	encoded, err := common.Marshal(map[string]string{"retired::orphan": expression})
	require.NoError(t, err)
	stored := map[string]string{billing_setting.PluginBillingExprOption: string(encoded)}
	_, err = stageCommittedCatalogPricing(stored, catalogPricingDependencies{})
	require.NoError(t, err, "committed stale overrides remain repairable")
	_, err = stageProspectiveCatalogPricing(stored, nil, catalogPricingDependencies{}, nil)
	require.ErrorContains(t, err, "does not declare", "new override cannot impersonate its own prior value")
	_, err = stageProspectiveCatalogPricing(stored, stored, catalogPricingDependencies{}, []catalogPricingLeaf{{billing_setting.PluginBillingExprOption, "retired::orphan"}})
	require.ErrorContains(t, err, "does not declare", "same-value adoption must validate a live binding")
	_, err = stageProspectiveCatalogPricing(map[string]string{billing_setting.PluginBillingExprOption: `{}`}, stored, catalogPricingDependencies{}, nil)
	require.NoError(t, err, "removing a stale override remains possible")
	_, err = stageProspectiveCatalogPricing(stored, stored, catalogPricingDependencies{}, []catalogPricingLeaf{{"ModelPrice", "absent"}})
	require.Error(t, err)
	_, err = stageProspectiveCatalogPricing(nil, nil, catalogPricingDependencies{}, []catalogPricingLeaf{{"ModelPrice", "suno_music"}})
	require.Error(t, err, "a compiled fallback is not an explicitly adopted source leaf")
	// Ambiguous prior JSON must not supply a last-key-wins preservation proof.
	ambiguous := map[string]string{billing_setting.PluginBillingExprOption: `{"retired::orphan":"tier(\"old\", u(\"seconds\"))","retired::orphan":"tier(\"raw\", u(\"seconds\") * 0.371)"}`}
	_, err = stageProspectiveCatalogPricing(stored, ambiguous, catalogPricingDependencies{}, nil)
	require.Error(t, err, "ambiguous previous rows cannot authorize unchanged retention")
}

func TestCatalogCandidateExplicitDependencies(t *testing.T) {
	const expression = `tier("raw", u("seconds") * 0.371)`
	expressions, err := common.Marshal(map[string]string{"declared": expression, "mapped": expression})
	require.NoError(t, err)
	options := map[string]string{
		"billing_setting.billing_mode":     `{"declared":"tiered_expr","mapped":"tiered_expr"}`,
		"billing_setting.billing_expr":     string(expressions),
		"starai_video_price.standard_720p": "13.25",
	}
	plugin := jsplugin.Meta{Key: "provider", Models: []string{"declared"}, UsageSchema: map[string]jsplugin.UsageFieldSchema{"seconds": {Type: "number", Unit: "second"}}}
	deps := catalogPricingDependencies{Plugins: map[string]jsplugin.Meta{"provider": plugin}, Aliases: map[string]TaskAliasTarget{"mapped": {Alias: "mapped", Declared: "declared", PluginKey: "provider"}}}
	candidate, err := stageProspectiveCatalogPricing(options, nil, deps, nil)
	require.NoError(t, err)
	assert.Equal(t, expression, candidate.billing.BillingExpr["declared"], "raw expressions are never generated from the Seedance scalar table")
	delete(deps.Plugins["provider"].UsageSchema, "seconds")
	_, err = stageProspectiveCatalogPricing(options, nil, deps, nil)
	require.ErrorContains(t, err, "not declared", "same-version schema change invalidates staging")
	deps.Plugins["provider"].UsageSchema["seconds"] = jsplugin.UsageFieldSchema{Type: "number", Unit: "second"}
	delete(deps.Aliases, "mapped")
	_, err = stageProspectiveCatalogPricing(options, nil, deps, nil)
	require.ErrorContains(t, err, "no task plugin usage schema")
	assert.Equal(t, expression, candidate.billing.BillingExpr["mapped"], "dependency changes cannot mutate an already owned candidate")
}

func TestCatalogCandidateTypedOwnershipAndConcurrentReads(t *testing.T) {
	preserveCatalogCandidate(t)
	a, err := stageCommittedCatalogPricing(map[string]string{"ModelPrice": `{"concurrent":0}`, "billing_setting.billing_mode": `{"concurrent":"tiered_expr"}`, "billing_setting.billing_expr": `{"concurrent":"tier(\"a\", p)"}`}, catalogPricingDependencies{})
	require.NoError(t, err)
	b, err := stageCommittedCatalogPricing(map[string]string{"ModelPrice": `{"concurrent":2}`, "billing_setting.billing_mode": `{"concurrent":"tiered_expr"}`, "billing_setting.billing_expr": `{"concurrent":"tier(\"b\", p * 2)"}`}, catalogPricingDependencies{})
	require.NoError(t, err)
	require.NoError(t, WithCatalogWriteBarrier(func() error { a.publishGuarded(); return nil }))
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Go(func() {
		<-start
		assert.NoError(t, WithCatalogWriteBarrier(func() error { b.publishGuarded(); a.publishGuarded(); return nil }))
	})
	workers.Go(func() {
		<-start
		price, exists := ratio_setting.GetModelPrice("concurrent", false)
		assert.True(t, exists)
		assert.Contains(t, []float64{0, 2}, price)
		assert.Equal(t, "tiered_expr", billing_setting.GetBillingMode("concurrent"))
		expression, exists := billing_setting.ResolveTaskBillingExpr("", "concurrent", "")
		assert.True(t, exists)
		assert.Contains(t, []string{`tier("a", p)`, `tier("b", p * 2)`}, expression)
		billing_setting.GetBillingExprCopy()["concurrent"] = "poison"
		billing_setting.GetBillingModeCopy()["concurrent"] = "poison"
		billing_setting.GetPluginBillingExprCopy()["test::concurrent"] = "poison"
	})
	workers.Go(func() {
		<-start
		assert.Equal(t, 5.0, operation_setting.GetToolPriceForModel("web_search", "grok-3"))
		assert.Equal(t, 10.0, operation_setting.GetToolPrice("web_search"))
		output, _, ok := ratio_setting.GetMoliiGrokImagePrices("grok-imagine-image", "1k")
		assert.True(t, ok)
		assert.Equal(t, 0.02, output)
		video, ok := ratio_setting.GetStarAIVideoPrice("doubao-seedance-2-0-260128", "720p", false)
		assert.True(t, ok)
		assert.Equal(t, 46.0, video)
		ratio_setting.GetMoliiGrokCatalogPricing("grok-imagine-image")
		ratio_setting.GetStarAIVideoPricing("doubao-seedance-2-0-260128")
		assert.Equal(t, 1.666667, task_pricing_setting.SoraSizeRatio("1792x1024"))
		assert.Equal(t, 1.5, task_pricing_setting.VertexResolutionRatio("veo-3.1", "4k"))
		task_pricing_setting.GetCopy().SoraSizeRatio["1792x1024"] = 999
	})
	close(start)
	workers.Wait()
	expression, _ := billing_setting.GetBillingExpr("concurrent")
	assert.Equal(t, `tier("a", p)`, expression)
	assert.Equal(t, 1.666667, task_pricing_setting.SoraSizeRatio("1792x1024"))
	// The typed setter must copy its input, not expose candidate maps through the registry.
	a.billing.BillingExpr["concurrent"] = "mutated candidate"
	a.ratios["ModelPrice"]["concurrent"] = 999
	a.factors.SoraSizeRatio["1792x1024"] = 999
	expression, _ = billing_setting.GetBillingExpr("concurrent")
	assert.Equal(t, `tier("a", p)`, expression)
	price, _ := ratio_setting.GetModelPrice("concurrent", false)
	assert.Zero(t, price)
	assert.Equal(t, 1.666667, task_pricing_setting.SoraSizeRatio("1792x1024"))
	operatorPrices := map[string]float64{"web_search": 12, "web_search:custom*": 7, "web_search:custom-long*": 0}
	prepared, err := operation_setting.PrepareToolPrices(operatorPrices)
	require.NoError(t, err)
	operatorPrices["web_search:custom-long*"] = 99
	operation_setting.PublishToolPrices(prepared)
	assert.Zero(t, operation_setting.GetToolPriceForModel("web_search", "custom-long-v1"))
	operation_setting.SetToolPriceForTest("web_search:custom-long*", 23)
	operation_setting.PublishToolPrices(prepared)
	assert.Zero(t, operation_setting.GetToolPriceForModel("web_search", "custom-long-v1"), "live raw-map mutations do not alter the prepared index")
	_, err = operation_setting.PrepareToolPrices(map[string]float64{"web_search": -1})
	require.Error(t, err)
	assert.Zero(t, operation_setting.GetToolPriceForModel("web_search", "custom-long-v1"))
}
