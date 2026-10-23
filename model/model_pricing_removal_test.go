package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise the public JSON intent even before the Remove field exists, so RED
// demonstrates the missing behavior rather than a missing Go struct member.
func pricingRemovalRequest(t *testing.T, name, version string) ModelPricingChange {
	t.Helper()
	var change ModelPricingChange
	require.NoError(t, common.UnmarshalJsonStr(fmt.Sprintf(
		`{"model_name":%q,"expected_version":%q,"pricing":{},"remove":true}`, name, version), &change))
	return change
}

func TestModelPricingRemovalDatabaseMatrix(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			catalogBusinessActor(t, db)
			// Explicit real enrollment is required on every engine, not only the
			// audited legacy PostgreSQL fixtures selected by the gate.
			require.NoError(t, db.AutoMigrate(&SystemInstance{}))
			identity, master, started := common.GetNodeIdentity(), common.IsMasterNode, common.StartTime
			t.Cleanup(func() {
				common.NodeName, common.NodeNameSource, common.NodeNameManuallyConfigured = identity.Name, identity.Source, identity.ManuallyConfigured
				common.IsMasterNode, common.StartTime = master, started
			})
			common.NodeName, common.NodeNameSource, common.NodeNameManuallyConfigured = "pricing-removal-test", common.NodeNameSourceManual, true
			common.IsMasterNode, common.StartTime = true, time.Now().Unix()-10
			t.Setenv("CATALOG_SYNC_SINGLE_INSTANCE", "true")
			catalogStartupHeartbeat(t)

			const orphan = "qwen3.7-max"
			const providerOnly = "qwen3.8-max"
			const cny = "removal-cny-model"
			require.NoError(t, db.Create(&Model{ModelName: cny, NameRule: NameRuleExact, BillingCurrency: "CNY", Description: "preserve exact metadata"}).Error)
			rows := []Option{
				{Key: "ModelPrice", Value: ` {"qwen3.7-max":0.3,"removal-cny-model":0.5,"untouched":0} `},
				{Key: "CacheRatio", Value: ` { "untouched" : 0.25 } `},
				{Key: "billing_setting.billing_mode", Value: `{"qwen3.7-max":"tiered_expr"}`},
				{Key: "billing_setting.billing_expr", Value: `{"qwen3.7-max":"tier(\"base\", p * 3)","gpt-image-2":"tier(\"override\", p * 99)","untouched":"tier(\"base\", p * 7)"}`},
				{Key: billing_setting.PluginBillingExprOption, Value: `{"gone-a::qwen3.7-max":"u(\"seconds\") * 0.4","gone-b::qwen3.7-max":"u(\"seconds\") * 0.5","gone-a::qwen3.8-max":"u(\"seconds\") * 0.6","gone-a::untouched":"u(\"seconds\") * 0.7"}`},
				{Key: "UnrelatedRemovalFixture", Value: " raw unrelated value "},
			}
			require.NoError(t, db.Create(&rows).Error)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))

			entry := func(name string) ModelPricingEntry {
				t.Helper()
				snapshot, err := GetModelPricingSnapshot([]string{name})
				require.NoError(t, err)
				require.Len(t, snapshot.Entries, 1)
				return snapshot.Entries[0]
			}
			state := func() CatalogSyncState {
				t.Helper()
				var value CatalogSyncState
				require.NoError(t, db.First(&value, CatalogSyncStateID).Error)
				return value
			}
			options := func() []Option {
				t.Helper()
				var value []Option
				require.NoError(t, db.Order(commonKeyCol).Find(&value).Error)
				return value
			}
			metadata := func() []Model {
				t.Helper()
				var value []Model
				require.NoError(t, db.Unscoped().Order("id").Find(&value).Error)
				return value
			}
			beforeOrphan, beforeProvider := entry(orphan), entry(providerOnly)
			require.False(t, beforeOrphan.HasMetadata)
			require.Equal(t, "USD", beforeOrphan.BillingCurrency, "read fallback is only a version input, not write authority")
			require.Contains(t, beforeOrphan.Configured, "ModelPrice")
			require.Len(t, beforeProvider.Configured, 1)
			beforeMetadata := metadata()
			assertUnchanged := func(t *testing.T, changes []ModelPricingChange, message string, conflict bool) {
				t.Helper()
				beforeRows, beforeState, beforeCache := options(), state(), captureCatalogCandidateOptions()
				err := UpdateModelPricing(changes)
				if conflict {
					require.ErrorIs(t, err, ErrModelPricingConflict)
				} else {
					require.ErrorContains(t, err, message)
				}
				assert.Equal(t, beforeRows, options(), "rejection must not persist any batch member")
				assert.Equal(t, beforeState, state(), "rejection must not change revision or runtime acknowledgement")
				assert.Equal(t, beforeCache, captureCatalogCandidateOptions())
				assert.Equal(t, beforeMetadata, metadata())
				assert.Equal(t, beforeOrphan.Version, entry(orphan).Version)
			}

			t.Run("ordinary-orphan-writes-stay-strict", func(t *testing.T) {
				for _, change := range []ModelPricingChange{
					{ModelName: orphan, ExpectedVersion: beforeOrphan.Version, Pricing: PricingValues{}},
					{ModelName: orphan, ExpectedVersion: beforeOrphan.Version, Pricing: PricingValues{"ModelPrice": float64(2)}},
					{ModelName: orphan, ExpectedVersion: beforeOrphan.Version, Pricing: beforeOrphan.Configured, BillingCurrency: "CNY"},
					{ModelName: orphan, ExpectedVersion: beforeOrphan.Version, Reset: true},
					{ModelName: "new-orphan", ExpectedVersion: entry("new-orphan").Version, Pricing: PricingValues{"ModelPrice": float64(2)}},
				} {
					assertUnchanged(t, []ModelPricingChange{change}, "metadata", false)
				}
			})
			t.Run("reject-conflicting-remove-intent", func(t *testing.T) {
				for _, mutate := range []func(*ModelPricingChange){
					func(c *ModelPricingChange) { c.Reset = true },
					func(c *ModelPricingChange) { c.Pricing = PricingValues{"ModelPrice": float64(2)} },
					func(c *ModelPricingChange) { c.BillingCurrency = "USD" },
					func(c *ModelPricingChange) { c.BillingCurrency = "CNY" },
				} {
					change := pricingRemovalRequest(t, orphan, beforeOrphan.Version)
					mutate(&change)
					assertUnchanged(t, []ModelPricingChange{change}, "Invalid parameters", false)
				}
				assertUnchanged(t, []ModelPricingChange{pricingRemovalRequest(t, "  ", beforeOrphan.Version)}, "Model name cannot be empty", false)
				managed := pricingRemovalRequest(t, ratio_setting.StarAIVideoPricingModels()[0], beforeOrphan.Version)
				managed.Reset = true
				assertUnchanged(t, []ModelPricingChange{managed}, "managed in the Seedance", false)
			})
			t.Run("stale-and-mixed-batch-are-atomic", func(t *testing.T) {
				remove := pricingRemovalRequest(t, orphan, beforeOrphan.Version)
				assertUnchanged(t, []ModelPricingChange{pricingRemovalRequest(t, orphan, "stale-version")}, "", true)
				assertUnchanged(t, []ModelPricingChange{remove, pricingRemovalRequest(t, providerOnly, "stale-version")}, "", true)
				assertUnchanged(t, []ModelPricingChange{remove, {ModelName: cny, ExpectedVersion: entry(cny).Version, Pricing: PricingValues{"ModelPrice": float64(-1)}}}, "non-negative", false)
				assertUnchanged(t, []ModelPricingChange{remove, {ModelName: cny, ExpectedVersion: entry(cny).Version, Pricing: PricingValues{billing_setting.PluginBillingExprOption: map[string]any{"gone-a": `u("seconds") * 2`}}}}, "does not declare", false)
				assertUnchanged(t, []ModelPricingChange{remove, {ModelName: providerOnly, ExpectedVersion: beforeProvider.Version, Pricing: PricingValues{}}}, "metadata", false)
			})

			t.Run("remove-persisted-model-and-provider-leaves", func(t *testing.T) {
				beforeState := state()
				remove := pricingRemovalRequest(t, orphan, beforeOrphan.Version)
				remove.BillingCurrency = " \t " // no currency intent
				require.NoError(t, UpdateModelPricing([]ModelPricingChange{remove, pricingRemovalRequest(t, providerOnly, beforeProvider.Version)}))
				afterState := state()
				assert.Equal(t, beforeState.Revision+1, afterState.Revision)
				assert.Equal(t, afterState.Revision, afterState.RuntimeRevision)
				assert.Equal(t, "ready", afterState.PublicationState)
				assert.Empty(t, afterState.PendingOperationID)
				assert.Equal(t, beforeState.BaselineGeneration, afterState.BaselineGeneration)
				assert.Equal(t, beforeState.SourceID, afterState.SourceID)
				assert.Equal(t, beforeState.IncarnationKey, afterState.IncarnationKey)
				assert.NotEqual(t, beforeState.CurrentDigest, afterState.CurrentDigest)
				for _, name := range []string{orphan, providerOnly} {
					after := entry(name)
					assert.Empty(t, after.Configured)
					assert.False(t, after.HasMetadata)
					assert.Equal(t, ModelPricingVersion(PricingValues{}), after.Version)
				}
				assert.NotEqual(t, beforeOrphan.Version, entry(orphan).Version)
				assert.NotEqual(t, beforeProvider.Version, entry(providerOnly).Version)
				assert.Equal(t, beforeMetadata, metadata(), "removal must never create or change metadata/currency")
				stored := map[string]string{}
				for _, row := range options() {
					stored[row.Key] = row.Value
				}
				assert.JSONEq(t, `{"removal-cny-model":0.5,"untouched":0}`, stored["ModelPrice"])
				assert.JSONEq(t, `{}`, stored["billing_setting.billing_mode"])
				assert.JSONEq(t, `{"gpt-image-2":"tier(\"override\", p * 99)","untouched":"tier(\"base\", p * 7)"}`, stored["billing_setting.billing_expr"])
				assert.JSONEq(t, `{"gone-a::untouched":"u(\"seconds\") * 0.7"}`, stored[billing_setting.PluginBillingExprOption])
				assert.Equal(t, ` { "untouched" : 0.25 } `, stored["CacheRatio"], "untouched raw pricing row must be byte-identical")
				assert.Equal(t, " raw unrelated value ", stored["UnrelatedRemovalFixture"])
				assert.NotContains(t, stored, "ModelRatio")
				require.NoError(t, WithCatalogPricingRead(context.Background(), orphan, func() error {
					_, priced := ratio_setting.GetModelPrice(orphan, false)
					assert.False(t, priced)
					_, configured := billing_setting.GetBillingExpr(orphan)
					assert.False(t, configured)
					assert.NotContains(t, billing_setting.GetPluginBillingExprCopy(), "gone-a::"+providerOnly)
					assert.Equal(t, `u("seconds") * 0.7`, billing_setting.GetPluginBillingExprCopy()["gone-a::untouched"])
					return nil
				}))
			})

			t.Run("remove-preserves-cny-metadata-and-builtin-fallback", func(t *testing.T) {
				builtinBefore := entry("gpt-image-2")
				require.NoError(t, UpdateModelPricing([]ModelPricingChange{
					pricingRemovalRequest(t, cny, entry(cny).Version),
					pricingRemovalRequest(t, "gpt-image-2", builtinBefore.Version),
				}))
				assert.Equal(t, beforeMetadata, metadata())
				assert.Equal(t, "CNY", entry(cny).BillingCurrency)
				assert.True(t, entry(cny).HasMetadata)
				assert.Empty(t, entry(cny).Configured)
				builtinAfter := entry("gpt-image-2")
				assert.Empty(t, builtinAfter.Configured)
				assert.NotEqual(t, builtinBefore.Version, builtinAfter.Version)
				assert.Equal(t, `tier("standard", p * 5 + cr * 1.25 + img * 8 + img_cr * 2 + c * 30)`, builtinAfter.Effective["billing_setting.billing_expr"])
				assert.Equal(t, billing_setting.BillingModeTieredExpr, billing_setting.GetBillingMode("gpt-image-2"))
			})
			t.Run("absent-maps-and-default-only-removal-are-storage-noops", func(t *testing.T) {
				beforeRows, beforeState, beforeCache := options(), state(), captureCatalogCandidateOptions()
				legacy, builtin := entry("gpt-4"), entry("gpt-6-astra")
				require.Equal(t, float64(15), legacy.Configured["ModelRatio"])
				require.NotEmpty(t, builtin.Effective["billing_setting.billing_expr"])
				require.NoError(t, UpdateModelPricing([]ModelPricingChange{
					pricingRemovalRequest(t, "gpt-4", legacy.Version),
					pricingRemovalRequest(t, "gpt-6-astra", builtin.Version),
					pricingRemovalRequest(t, "never-configured", entry("never-configured").Version),
				}))
				assert.Equal(t, beforeRows, options(), "inherited defaults must not become explicit empty/modified maps")
				assert.Equal(t, beforeState, state())
				assert.Equal(t, beforeCache, captureCatalogCandidateOptions())
				assert.Equal(t, legacy, entry("gpt-4"))
				assert.Equal(t, builtin, entry("gpt-6-astra"))
				ratio, found, _ := ratio_setting.GetModelRatio("gpt-4")
				assert.True(t, found)
				assert.Equal(t, float64(15), ratio)
				assert.Equal(t, beforeMetadata, metadata())
			})
		})
	}
}
