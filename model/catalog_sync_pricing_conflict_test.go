package model

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogSyncGrokFixedPricingConflict(t *testing.T) {
	// Read the real factory source without importing its package-level registry
	// initialization into every model test.
	source, err := os.ReadFile("../plugins/tasks/xai/plugin.js")
	require.NoError(t, err)
	plugin, err := jsplugin.NewRegistry().RegisterFactory(string(source), jsplugin.Options{Key: "xai"})
	require.NoError(t, err)
	const name = "grok-imagine-video"
	const expression = `tier("request", fixed(0.123456789))`
	draft := catalogmanifest.Snapshot{Coverage: map[string]int{}}
	require.NoError(t, appendManagedEntry(&draft, catalogmanifest.KindModel, name, catalogmanifest.ModelValue{ModelName: name, BillingCurrency: "CNY"}))
	var strict []string
	for option, value := range map[string]string{"billing_setting.billing_mode": `"tiered_expr"`, "billing_setting.billing_expr": fmt.Sprintf("%q", expression)} {
		key, err := catalogmanifest.EncodePriceKey(catalogmanifest.PriceKey{Model: name, Option: option, Path: "/" + catalogmanifest.PointerSegment(name)})
		require.NoError(t, err)
		require.NoError(t, appendManagedEntry(&draft, catalogmanifest.KindModelPrice, key, catalogmanifest.PriceValue{Value: value, BillingCurrency: "CNY", Unit: catalogmanifest.PriceOptions()[option].Unit}))
		strict = append(strict, catalogmanifest.EntryID(draft.Entries[len(draft.Entries)-1]))
	}
	data := catalogValidationData{Draft: draft, Effective: map[string]catalogValidationPlugin{"xai": {Key: "xai", Models: plugin.Meta.Models, UsageSchema: plugin.Meta.UsageSchema, UsageProfiles: plugin.Meta.UsageProfiles}}}
	stage := func(data catalogValidationData) error {
		encoded, err := common.Marshal(data)
		require.NoError(t, err)
		attestation, err := stageCatalogValidation(catalogValidationInput{canonical: string(encoded)})
		if err != nil {
			assert.Empty(t, attestation, "rejected pricing must never gain an attestation")
		}
		return err
	}
	for _, adopt := range []bool{false, true} {
		t.Run(fmt.Sprintf("adopt=%t", adopt), func(t *testing.T) {
			candidate := data
			if adopt {
				candidate.Before, candidate.StrictPrices = draft, strict
			}
			err := stage(candidate)
			require.EqualError(t, err, "model grok-imagine-video: plugin xai: fixed pricing is not supported for task usage expressions")
			var conflict *ModelPricingValidationConflict
			require.ErrorAs(t, err, &conflict)
			assert.Equal(t, name, conflict.Model)
			assert.Equal(t, "xai", conflict.Provider)
			assert.Equal(t, "task_fixed_pricing_unsupported", conflict.Reason)
			require.NotNil(t, errors.Unwrap(conflict))
			assert.Equal(t, err.Error(), errors.Unwrap(conflict).Error())
		})
	}
	data.Before = draft
	require.NoError(t, stage(data), "reconstructing unchanged stored data must not block unrelated managed changes")

	dependencies := modelPricingValidationDependencies{plugins: map[string]jsplugin.Meta{"xai": plugin.Meta}, resolveAlias: func(string) (TaskAliasTarget, bool) { return TaskAliasTarget{PluginKey: "xai", Declared: name}, true }}
	for _, name := range []string{name, "grok-alias"} {
		err := validateModelPricingWithDependencies(name, PricingValues{"billing_setting.billing_expr": expression}, nil, dependencies)
		var conflict *ModelPricingValidationConflict
		require.ErrorAs(t, err, &conflict)
		assert.Equal(t, name, conflict.Model)
		assert.Equal(t, "xai", conflict.Provider)
		if name == "grok-alias" {
			assert.EqualError(t, err, "model grok-alias: plugin xai: fixed pricing is not supported for task usage expressions")
		}
	}
	for _, expression := range []string{`tier("request", fixed(1)) [`, `u("not_declared_secret")`, `1 / u("seconds")`} {
		err := validateModelPricingWithDependencies(name, PricingValues{billing_setting.PluginBillingExprOption: map[string]any{"xai": expression}}, nil, dependencies)
		require.Error(t, err)
		var conflict *ModelPricingValidationConflict
		assert.False(t, errors.As(err, &conflict), "syntax, schema and runtime errors remain unclassified")
	}
	err = validateModelPricingWithDependencies(name, PricingValues{billing_setting.PluginBillingExprOption: map[string]any{"xai": expression}}, nil, dependencies)
	var conflict *ModelPricingValidationConflict
	require.ErrorAs(t, err, &conflict, "explicit provider errors retain their identity")
	require.NoError(t, validateModelPricingWithDependencies(name, PricingValues{"billing_setting.billing_expr": expression, billing_setting.PluginBillingExprOption: map[string]any{"xai": `u("seconds") * 0.4`}}, nil, dependencies), "valid explicit usage pricing still shadows the model expression")
}
