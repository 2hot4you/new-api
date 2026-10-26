package model

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelPricingRejectsUnchangedUnsafeTaskExpressions(t *testing.T) {
	const name = "pricing-usage-model"
	const taskExpr = `tier("base", u("seconds") * 0.4)`
	const tokenExpr = `tier("tokens", p * 2 + c * 10)`
	const fixedExpr = `tier("request", fixed(0.1))`
	for _, tc := range []struct {
		name, expression, override, wantError string
		removeOverride, ratio, alias          bool
		addOverride, reverse                  bool
		resetMapped, removeMapped             bool
	}{
		{name: "unchanged inherited token quote", expression: tokenExpr, wantError: "token variable"},
		{name: "unchanged inherited fixed quote", expression: fixedExpr, wantError: "fixed pricing is not supported"},
		{name: "unchanged provider token quote", expression: taskExpr, override: tokenExpr, wantError: "token variable"},
		{name: "unchanged provider fixed quote", expression: taskExpr, override: fixedExpr, wantError: "fixed pricing is not supported"},
		{name: "removing override exposes unsafe token quote", expression: tokenExpr, override: taskExpr, removeOverride: true, wantError: "token variable"},
		{name: "provider override protects ordinary token quote", expression: tokenExpr, override: taskExpr},
		{name: "dormant ratio model quote", expression: tokenExpr, ratio: true},
		{name: "mapped provider override protects ordinary alias quote", expression: tokenExpr, override: taskExpr, alias: true},
		{name: "removing mapped override exposes unsafe alias quote", expression: tokenExpr, override: taskExpr, alias: true, removeOverride: true, wantError: "token variable"},
		{name: "mapped override removal before alias", expression: tokenExpr, override: taskExpr, alias: true, removeOverride: true, reverse: true, wantError: "token variable"},
		{name: "reset mapped pricing exposes unsafe alias quote", expression: tokenExpr, override: taskExpr, alias: true, resetMapped: true, wantError: "token variable"},
		{name: "remove mapped pricing exposes unsafe alias quote", expression: tokenExpr, override: taskExpr, alias: true, removeMapped: true, wantError: "token variable"},
		{name: "alias before new mapped override", expression: tokenExpr, override: taskExpr, alias: true, addOverride: true},
		{name: "new mapped override before alias", expression: tokenExpr, override: taskExpr, alias: true, addOverride: true, reverse: true},
		{name: "constant task dollars", expression: `tier("base", 0.1)`},
		{name: "request selectors remain available", expression: `param("service_tier") == "fast" && header("x-mode") == "fast" ? 0.2 : 0.1`},
		{name: "declared token usage fact", expression: `tier("tokens", u("tokens") * 2 / 1000000)`},
		{name: "unchanged narrowed usage profile", expression: `tier("legacy", u("old_units") * 0.1)`},
		{name: "unchanged narrowed provider profile", expression: taskExpr, override: `tier("legacy", u("old_units") * 0.1)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupModelPricingCurrencyTest(t)
			modelName := name
			if tc.alias {
				modelName = "pricing-usage-alias"
				mapping := `{"pricing-usage-alias":"pricing-usage-model"}`
				channel := Channel{Name: "pricing-alias", Type: constant.ChannelTypeTaskPlugin, Key: "test", Status: 1, Models: modelName, ModelMapping: &mapping}
				require.NoError(t, DB.Create(&channel).Error)
				require.NoError(t, DB.Create(&Ability{Group: "default", Model: modelName, ChannelId: channel.Id, Enabled: true}).Error)
			}
			source := `
export const meta = {
 apiVersion:1,key:"pricing-usage-probe",name:"Pricing Usage Probe",version:"1.0.0",author:{name:"Test"},
 models:["pricing-usage-model"],fetchMode:"per_task",
 usageSchema:{seconds:{type:"number",unit:"second"},tokens:{type:"number",unit:"token"}},
 usageExamples:[{label:"default",facts:{seconds:1,tokens:1000000}}]
};
export function buildSubmitRequest(){return {};}
export function parseSubmitResponse(){return {};}
export function buildQueryRequest(){return {};}
export function parseTaskResult(){return {};}
`
			plugin, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
			require.NoError(t, err)
			require.NoError(t, DB.Create(&TaskPlugin{
				Key: plugin.Meta.Key, APIVersion: plugin.Meta.APIVersion, Version: plugin.Meta.Version,
				Source: LongText(source), SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(source))), Active: true, Enabled: true,
			}).Error)
			require.NoError(t, DB.Create(&Model{ModelName: modelName, NameRule: NameRuleExact, BillingCurrency: "USD"}).Error)
			if tc.alias {
				require.NoError(t, DB.Create(&Model{ModelName: name, NameRule: NameRuleExact, BillingCurrency: "USD"}).Error)
			}
			// Recover a valid catalog before installing historical request inputs.
			// Startup itself now also rejects these unsafe stored expressions.
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			pricing := PricingValues{"billing_setting.billing_mode": "tiered_expr", "billing_setting.billing_expr": tc.expression}
			if tc.ratio {
				pricing["billing_setting.billing_mode"] = "ratio"
			}
			if tc.override != "" && !tc.alias {
				pricing[billing_setting.PluginBillingExprOption] = map[string]any{plugin.Meta.Key: tc.override}
			}
			maps := defaultPricingMaps()
			replaceModelPricing(maps, modelName, pricing)
			if tc.alias && !tc.addOverride {
				maps[billing_setting.PluginBillingExprOption][billing_setting.PluginBillingExprKey(plugin.Meta.Key, name)] = tc.override
			}
			storedOptions := make(map[string]string)
			for _, key := range []string{"billing_setting.billing_mode", "billing_setting.billing_expr", billing_setting.PluginBillingExprOption} {
				encoded, err := common.Marshal(maps[key])
				require.NoError(t, err)
				require.NoError(t, DB.Save(&Option{Key: key, Value: string(encoded)}).Error)
				storedOptions[key] = string(encoded)
			}
			before, err := GetModelPricingSnapshot([]string{modelName})
			require.NoError(t, err)
			require.Len(t, before.Entries, 1)
			require.Equal(t, pricing, before.Entries[0].Configured, "fixture must load the actual persisted previous quote")
			if tc.removeOverride && !tc.alias {
				delete(pricing, billing_setting.PluginBillingExprOption)
			}
			if tc.alias && !tc.addOverride {
				require.NoError(t, config.GlobalConfig.LoadFromDB(storedOptions))
				require.NoError(t, ValidateModelPricing(modelName, pricing), "the runtime preview wrapper must honor the same mapped override")
			}
			changes := []ModelPricingChange{{ModelName: modelName, ExpectedVersion: before.Entries[0].Version, Pricing: pricing}}
			if tc.alias && (tc.removeOverride || tc.addOverride || tc.resetMapped || tc.removeMapped) {
				mapped, err := GetModelPricingSnapshot([]string{name})
				require.NoError(t, err)
				mappedPricing := PricingValues{}
				if tc.addOverride {
					mappedPricing[billing_setting.PluginBillingExprOption] = map[string]any{plugin.Meta.Key: tc.override}
				}
				mappedChange := ModelPricingChange{ModelName: name, ExpectedVersion: mapped.Entries[0].Version, Pricing: mappedPricing, Reset: tc.resetMapped, Remove: tc.removeMapped}
				if tc.removeMapped {
					mappedChange.Pricing = nil
				}
				changes = append(changes, mappedChange)
				if tc.reverse {
					changes[0], changes[1] = changes[1], changes[0]
				}
			}
			err = UpdateModelPricing(changes)
			if tc.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantError)
			}
			after, err := GetModelPricingSnapshot([]string{modelName})
			require.NoError(t, err)
			require.Equal(t, before.Entries[0].Configured, after.Entries[0].Configured, "a rejected or unchanged draft must not change stored prices")
			require.Equal(t, before.Entries[0].Version, after.Entries[0].Version)
			if tc.alias {
				var stored Option
				require.NoError(t, DB.Where(map[string]any{"key": billing_setting.PluginBillingExprOption}).First(&stored).Error)
				expectedOverrides := storedOptions[billing_setting.PluginBillingExprOption]
				if tc.addOverride {
					encoded, err := common.Marshal(map[string]string{"pricing-usage-probe::pricing-usage-model": taskExpr})
					require.NoError(t, err)
					expectedOverrides = string(encoded)
				}
				require.JSONEq(t, expectedOverrides, stored.Value, "mapped provider override must reflect the whole accepted or rejected batch")
			}
		})
	}
}

func setupModelPricingCurrencyTest(t *testing.T) {
	t.Helper()
	engine := "sqlite"
	if os.Getenv("TEST_POSTGRES_DSN") != "" {
		engine = "postgres"
	}
	db := catalogFenceTestDB(t, engine)
	initializeOrdinaryCatalogTest(t, db)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
}

func TestModelPricingSnapshotAndSaveIncludeBillingCurrency(t *testing.T) {
	setupModelPricingCurrencyTest(t)
	const name = "pricing-currency-model"
	require.NoError(t, DB.Create(&Model{
		ModelName: name, NameRule: NameRuleExact, BillingCurrency: "CNY",
	}).Error)

	snapshot, err := GetModelPricingSnapshot([]string{name, "missing-pricing-metadata"})
	require.NoError(t, err)
	require.Len(t, snapshot.Entries, 2)
	entries := map[string]ModelPricingEntry{}
	for _, entry := range snapshot.Entries {
		entries[entry.ModelName] = entry
	}
	assert.Equal(t, "USD", entries["missing-pricing-metadata"].BillingCurrency)
	assert.False(t, entries["missing-pricing-metadata"].HasMetadata)
	assert.Equal(t, "CNY", entries[name].BillingCurrency)
	assert.True(t, entries[name].HasMetadata)

	pricing := PricingValues{
		"billing_setting.billing_mode": billing_setting.BillingModeTieredExpr,
		"billing_setting.billing_expr": `tier("base", p * 5)`,
	}
	beforeVersion := entries[name].Version
	require.NoError(t, UpdateModelPricing([]ModelPricingChange{{
		ModelName: name, ExpectedVersion: beforeVersion, Pricing: pricing, BillingCurrency: "USD",
	}}))

	updated, err := GetModelPricingSnapshot([]string{name})
	require.NoError(t, err)
	require.Len(t, updated.Entries, 1)
	assert.Equal(t, "USD", updated.Entries[0].BillingCurrency)
	assert.NotEqual(t, beforeVersion, updated.Entries[0].Version)
	var stored Model
	require.NoError(t, DB.Where("model_name = ?", name).First(&stored).Error)
	assert.Equal(t, "USD", stored.BillingCurrency)
	require.NoError(t, UpdateModelPricing([]ModelPricingChange{{
		ModelName: name, ExpectedVersion: updated.Entries[0].Version, Pricing: pricing, BillingCurrency: "CNY",
	}}))
	currencyOnly, err := GetModelPricingSnapshot([]string{name})
	require.NoError(t, err)
	assert.Equal(t, pricing, currencyOnly.Entries[0].Configured)
	assert.Equal(t, "CNY", currencyOnly.Entries[0].BillingCurrency)
	assert.NotEqual(t, updated.Entries[0].Version, currencyOnly.Entries[0].Version)

	err = UpdateModelPricing([]ModelPricingChange{{
		ModelName: name, ExpectedVersion: currencyOnly.Entries[0].Version, Pricing: pricing, BillingCurrency: "EUR",
	}})
	require.ErrorContains(t, err, "USD or CNY")

	require.NoError(t, UpdateModelPricing([]ModelPricingChange{{
		ModelName: name, ExpectedVersion: currencyOnly.Entries[0].Version, Reset: true, BillingCurrency: "USD",
	}}))
	reset, err := GetModelPricingSnapshot([]string{name})
	require.NoError(t, err)
	assert.Equal(t, "USD", reset.Entries[0].BillingCurrency)

	err = UpdateModelPricing([]ModelPricingChange{{
		ModelName: "missing-pricing-metadata", ExpectedVersion: entries["missing-pricing-metadata"].Version,
		Pricing: pricing, BillingCurrency: "CNY",
	}})
	require.ErrorContains(t, err, "metadata")
}

func TestModelPricingSnapshotAllIncludesUnconfiguredExactMetadata(t *testing.T) {
	setupModelPricingCurrencyTest(t)
	const name = "glm-5.3"
	require.NoError(t, DB.Create(&[]Model{
		{ModelName: name, NameRule: NameRuleExact, BillingCurrency: "CNY"},
		{ModelName: "glm-prefix", NameRule: NameRulePrefix, BillingCurrency: "CNY"},
	}).Error)

	snapshot, err := GetModelPricingSnapshot(nil)
	require.NoError(t, err)
	entries := make(map[string]ModelPricingEntry, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		entries[entry.ModelName] = entry
	}
	entry, exists := entries[name]
	require.True(t, exists, "unconfigured exact metadata must be editable from the full pricing list")
	assert.Equal(t, "CNY", entry.BillingCurrency)
	assert.True(t, entry.HasMetadata)
	assert.NotEqual(t, snapshot.EmptyVersion, entry.Version)
	assert.NotContains(t, entries, "glm-prefix")

	require.NoError(t, UpdateModelPricing([]ModelPricingChange{{
		ModelName: name, ExpectedVersion: entry.Version,
		Pricing: PricingValues{"ModelPrice": float64(2)}, BillingCurrency: "CNY",
	}}))
}

func TestUpdateModelPricingRollsBackCurrencyWithPricingFailure(t *testing.T) {
	setupModelPricingCurrencyTest(t)
	const name = "pricing-currency-rollback"
	require.NoError(t, DB.Create(&Model{
		ModelName: name, NameRule: NameRuleExact, BillingCurrency: "USD",
	}).Error)
	before, err := GetModelPricingSnapshot([]string{name})
	require.NoError(t, err)

	err = UpdateModelPricing([]ModelPricingChange{
		{ModelName: name, ExpectedVersion: before.Entries[0].Version, Pricing: PricingValues{"ModelPrice": float64(2)}, BillingCurrency: "CNY"},
		{ModelName: "missing-metadata", ExpectedVersion: ModelPricingVersion(PricingValues{}), Pricing: PricingValues{"ModelPrice": float64(3)}, BillingCurrency: "CNY"},
	})
	require.Error(t, err)

	after, err := GetModelPricingSnapshot([]string{name})
	require.NoError(t, err)
	assert.Equal(t, before.Entries[0].Version, after.Entries[0].Version)
	assert.Equal(t, "USD", after.Entries[0].BillingCurrency)
}

func TestUpdateStarAIVideoPricingSetsExactMetadataToCNY(t *testing.T) {
	setupModelPricingCurrencyTest(t)
	const pluginKey = "seedance-currency-test"
	_, err := jsplugin.DefaultRegistry.RegisterFactory(`
export const meta = {
  apiVersion:1,key:"seedance-currency-test",name:"Seedance Currency Test",version:"1.0.0",author:{name:"Test"},
  models:["doubao-seedance-2-0-260128","doubao-seedance-2-0-fast-260128","doubao-seedance-2-0-mini-260615","doubao-seedance-2-5-260628"],fetchMode:"per_task",
  usageSchema:{tokens:{type:"number",unit:"token"},resolution:{enum:["720p","1080p","4k"]},video_input:{enum:["none","video"]}},
  usageExamples:[{label:"default",facts:{tokens:1000000,resolution:"720p",video_input:"none"}}]
};
export function buildSubmitRequest(){return {};}
export function parseSubmitResponse(){return {};}
export function buildQueryRequest(){return {};}
export function parseTaskResult(){return {};}
`, jsplugin.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { jsplugin.DefaultRegistry.Unregister(pluginKey) })
	for _, name := range ratio_setting.StarAIVideoPricingModels() {
		require.NoError(t, DB.Create(&Model{ModelName: name, NameRule: NameRuleExact, BillingCurrency: "USD"}).Error)
	}

	require.NoError(t, UpdateStarAIVideoPricing(ratio_setting.StarAIVideoPriceSetting{
		Standard720p: 46, Standard720pVideo: 28,
		Standard1080p: 51, Standard1080pVideo: 31,
		Standard4K: 26, Standard4KVideo: 16,
		Fast720p: 37, Fast720pVideo: 22,
		Mini720p: 23, Mini720pVideo: 14,
		Seedance25720p: 70, Seedance25720pVideo: 42,
		Seedance251080p: 77, Seedance251080pVideo: 46,
	}))

	var rows []Model
	require.NoError(t, DB.Where("model_name IN ?", ratio_setting.StarAIVideoPricingModels()).Find(&rows).Error)
	require.Len(t, rows, 4)
	for _, row := range rows {
		assert.Equal(t, "CNY", row.BillingCurrency)
	}
}
