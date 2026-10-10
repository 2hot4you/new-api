package model

import (
	"context"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
