package model

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PricingValues is one model's configuration, keyed by the existing option
// names. A missing key inherits the engine's default; an explicit zero is free.
type PricingValues map[string]any

type ModelPricingChange struct {
	ModelName       string        `json:"model_name"`
	ExpectedVersion string        `json:"expected_version"`
	Pricing         PricingValues `json:"pricing"`
	BillingCurrency string        `json:"billing_currency"`
	Reset           bool          `json:"reset,omitempty"`
}

type ModelPricingEntry struct {
	ModelPricingDescription
	PluginVariants  []ModelPricingPluginVariant          `json:"plugin_variants,omitempty"`
	ModelName       string                               `json:"model_name"`
	Version         string                               `json:"version"`
	Configured      PricingValues                        `json:"configured"`
	BillingCurrency string                               `json:"billing_currency"`
	HasMetadata     bool                                 `json:"has_metadata"`
	UsageSchema     map[string]jsplugin.UsageFieldSchema `json:"usage_schema,omitempty"`
}

type ModelPricingPluginVariant struct {
	PluginKey     string                               `json:"plugin_key"`
	PluginName    string                               `json:"plugin_name"`
	Icon          string                               `json:"icon,omitempty"`
	UsageSchema   map[string]jsplugin.UsageFieldSchema `json:"usage_schema"`
	UsageExamples []jsplugin.UsageExample              `json:"usage_examples,omitempty"`
	Configured    string                               `json:"configured"`
	Effective     string                               `json:"effective"`
	Compatible    bool                                 `json:"compatible"`
	Stale         bool                                 `json:"stale,omitempty"`
}

type ModelPricingSnapshot struct {
	Entries      []ModelPricingEntry `json:"entries"`
	Options      map[string]string   `json:"options"`
	EmptyVersion string              `json:"empty_version"`
}

var ErrModelPricingConflict = errors.New("model pricing changed; reload before saving")

// Stable option order is shared by snapshots and complete model drafts. Saves
// serialize through the catalog's actual root transaction and final writer.
var modelPricingOptionKeys = []string{
	"AudioCompletionRatio", "AudioRatio", "CacheRatio", "CompletionRatio",
	"CreateCacheRatio", "ImageRatio", "ModelPrice", "ModelRatio",
	"billing_setting.billing_expr", "billing_setting.billing_mode", billing_setting.PluginBillingExprOption,
}

func IsModelPricingOption(key string) bool {
	return slices.Contains(modelPricingOptionKeys, key)
}

func ModelPricingVersion(values PricingValues) string {
	return modelPricingVersion(values, "USD")
}

func modelPricingVersion(values PricingValues, billingCurrency string) string {
	encoded, _ := common.Marshal(struct {
		Pricing         PricingValues `json:"pricing"`
		BillingCurrency string        `json:"billing_currency"`
	}{Pricing: values, BillingCurrency: billingCurrency})
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}

func defaultPricingMaps() map[string]map[string]any {
	result := make(map[string]map[string]any, len(modelPricingOptionKeys))
	for _, key := range modelPricingOptionKeys {
		result[key] = make(map[string]any)
	}
	for key, values := range ratio_setting.GetDefaultPricingMaps() {
		for name, value := range values {
			result[key][name] = value
		}
	}
	return result
}

func readModelPricingMaps(db *gorm.DB) (map[string]map[string]any, map[string]bool, []string, error) {
	var rows []Option
	if err := db.Where(commonKeyCol+" IN ?", modelPricingOptionKeys).Find(&rows).Error; err != nil {
		return nil, nil, nil, err
	}
	values := defaultPricingMaps()
	existing := make(map[string]bool)
	counts := make(map[string]int)
	for _, row := range rows {
		var entries map[string]any
		if err := common.UnmarshalJsonStr(row.Value, &entries); err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", row.Key, err)
		}
		if entries == nil {
			return nil, nil, nil, fmt.Errorf("%s must be a JSON object", row.Key)
		}
		values[row.Key] = entries
		existing[row.Key] = true
		counts[row.Key]++
	}
	var duplicated []string
	for _, key := range modelPricingOptionKeys {
		if counts[key] > 1 {
			duplicated = append(duplicated, key)
		}
	}
	return values, existing, duplicated, nil
}

func modelPricingValues(values map[string]map[string]any, name string) PricingValues {
	result := make(PricingValues)
	for _, key := range modelPricingOptionKeys {
		if key == billing_setting.PluginBillingExprOption {
			variants := make(map[string]any)
			for variant, expression := range values[key] {
				if plugin, model, ok := billing_setting.SplitPluginBillingExprKey(variant); ok && model == name {
					variants[plugin] = expression
				}
			}
			if len(variants) > 0 {
				result[key] = variants
			}
			continue
		}
		if value, exists := values[key][name]; exists {
			result[key] = value
		}
	}
	return result
}

// replaceModelPricing writes one complete model draft into the option maps.
// Plugin expressions are grouped in the draft and flattened only in storage.
func replaceModelPricing(values map[string]map[string]any, name string, draft PricingValues) {
	for _, key := range modelPricingOptionKeys {
		if key == billing_setting.PluginBillingExprOption {
			for variant := range values[key] {
				if _, model, ok := billing_setting.SplitPluginBillingExprKey(variant); ok && model == name {
					delete(values[key], variant)
				}
			}
			variants, _ := draft[key].(map[string]any)
			for plugin, expr := range variants {
				values[key][billing_setting.PluginBillingExprKey(plugin, name)] = expr
			}
			continue
		}
		delete(values[key], name)
		if value, exists := draft[key]; exists {
			values[key][name] = value
		}
	}
}

func effectiveModelPricing(values map[string]map[string]any, name string) PricingValues {
	result := modelPricingValues(values, name)
	// Legacy wildcard aliases are resolved by the same normalization as relay.
	alias := ratio_setting.FormatMatchingModelName(name)
	for _, key := range []string{"ModelPrice", "ModelRatio", "CompletionRatio", "AudioRatio", "AudioCompletionRatio"} {
		delete(result, key)
		if value, exists := values[key][alias]; exists {
			result[key] = value
		}
	}
	mode, _ := result["billing_setting.billing_mode"].(string)
	if mode == "" {
		_, hasPrice := result["ModelPrice"]
		_, hasRatio := result["ModelRatio"]
		if _, builtin := billing_setting.GetBuiltinBillingExpr(name); builtin && !hasPrice && !hasRatio {
			mode = "tiered_expr"
		}
	}
	if mode == "tiered_expr" {
		result["billing_setting.billing_mode"] = mode
		if _, exists := result["billing_setting.billing_expr"]; !exists {
			if expression, ok := billing_setting.GetBuiltinBillingExpr(name); ok {
				result["billing_setting.billing_expr"] = expression
			}
		}
		return result
	}
	if _, exists := result["ModelPrice"]; exists {
		return result
	}
	if _, exists := result["ModelRatio"]; !exists && operation_setting.SelfUseModeEnabled {
		result["ModelRatio"] = float64(37.5)
	}
	// Completion ratios include engine-enforced model defaults. Expose their
	// effective value without persisting them into the editable configuration.
	var configuredCompletion *float64
	if ratio, exists := result["CompletionRatio"].(float64); exists {
		configuredCompletion = &ratio
	}
	result["CompletionRatio"] = ratio_setting.ResolveCompletionRatio(name, configuredCompletion).Ratio
	for key, fallback := range map[string]float64{
		"CacheRatio":       ratio_setting.DefaultCacheRatio,
		"CreateCacheRatio": ratio_setting.DefaultCreateCacheRatio,
		"ImageRatio":       ratio_setting.DefaultImageRatio,
	} {
		if _, exists := result[key]; !exists {
			result[key] = fallback
		}
	}
	return result
}

// PreviewModelPricing resolves a complete editable draft using the same defaults
// as the saved-price display and conversion. It has no write side effects.
func PreviewModelPricing(name string, draft PricingValues) (PricingValues, error) {
	if draft == nil {
		return nil, errors.New("pricing draft is required")
	}
	values, _, _, err := readModelPricingMaps(DB)
	if err != nil {
		return nil, err
	}
	if err := validateModelPricing(name, draft, modelPricingValues(values, name)); err != nil {
		return nil, err
	}
	replaceModelPricing(values, name, draft)
	return effectiveModelPricing(values, name), nil
}

func GetModelPricingSnapshot(names []string) (*ModelPricingSnapshot, error) {
	values, _, _, err := readModelPricingMaps(DB)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		nameSet := make(map[string]bool)
		for key, entries := range values {
			for name := range entries {
				if key == billing_setting.PluginBillingExprOption {
					_, model, ok := billing_setting.SplitPluginBillingExprKey(name)
					if !ok {
						continue
					}
					name = model
				}
				nameSet[name] = true
			}
		}
		for name := range billing_setting.GetBuiltinBillingExprCopy() {
			nameSet[name] = true
		}
		if DB.Migrator().HasTable(&Model{}) {
			var metadataNames []string
			if err := DB.Model(&Model{}).
				Where("name_rule = ?", NameRuleExact).
				Distinct("model_name").
				Pluck("model_name", &metadataNames).Error; err != nil {
				return nil, err
			}
			for _, name := range metadataNames {
				nameSet[name] = true
			}
		}
		for name := range nameSet {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	currencies, err := LoadModelBillingCurrencies(DB, names, false)
	if err != nil {
		return nil, err
	}
	result := &ModelPricingSnapshot{Entries: make([]ModelPricingEntry, 0, len(names)), Options: make(map[string]string), EmptyVersion: ModelPricingVersion(PricingValues{})}
	generation := jsplugin.DefaultRegistry.Generation()
	for _, name := range names {
		configured := modelPricingValues(values, name)
		currency := currencies[name]
		entry := ModelPricingEntry{ModelName: name, Version: modelPricingVersion(configured, string(currency.BillingCurrency)), Configured: configured,
			BillingCurrency: string(currency.BillingCurrency), HasMetadata: currency.HasMetadata,
			ModelPricingDescription: ModelPricingDescription{Effective: effectiveModelPricing(values, name)}}
		entry.CacheWriteMode = ResolveCacheWriteMode(name, configured)
		entry.BillingDetails = ResolveLegacyBillingDetails(name, entry.Effective, configured)
		if plugin, ok := generation.GetByModel(name); ok {
			entry.UsageSchema, _ = plugin.Meta.UsageForModel(name)
		} else if target, ok := ResolveTaskModelAlias(generation, name); ok {
			if plugin, ok := generation.Get(target.PluginKey); ok {
				entry.UsageSchema, _ = plugin.Meta.UsageForModel(target.Declared)
			}
		}
		plugins := generation.PluginsByModel(name)
		configuredVariants, _ := configured[billing_setting.PluginBillingExprOption].(map[string]any)
		if len(plugins) >= 2 || len(configuredVariants) > 0 {
			keys := make(map[string]bool, len(plugins)+len(configuredVariants))
			for _, plugin := range plugins {
				keys[plugin.Meta.Key] = true
			}
			for key := range configuredVariants {
				keys[key] = true
			}
			for _, key := range slices.Sorted(maps.Keys(keys)) {
				configuredValue, overridden := configuredVariants[key]
				configuredExpr, _ := configuredValue.(string)
				plugin, exists := generation.Get(key)
				if !exists || !slices.Contains(plugin.Meta.Models, name) {
					variant := ModelPricingPluginVariant{
						PluginKey: key, PluginName: key, Configured: configuredExpr,
						UsageSchema: map[string]jsplugin.UsageFieldSchema{}, Stale: true,
					}
					if exists {
						variant.PluginName, variant.Icon = plugin.Meta.Name, plugin.Meta.Icon
					}
					entry.PluginVariants = append(entry.PluginVariants, variant)
					continue
				}
				schema, examples := plugin.Meta.UsageForModel(name)
				if schema == nil {
					schema = map[string]jsplugin.UsageFieldSchema{}
				}
				expression := configuredExpr
				if !overridden && entry.Effective["billing_setting.billing_mode"] == billing_setting.BillingModeTieredExpr {
					expression, _ = entry.Effective["billing_setting.billing_expr"].(string)
				}
				entry.PluginVariants = append(entry.PluginVariants, ModelPricingPluginVariant{
					PluginKey: plugin.Meta.Key, PluginName: plugin.Meta.Name, Icon: plugin.Meta.Icon,
					UsageSchema: schema, UsageExamples: examples, Configured: configuredExpr, Effective: expression,
					Compatible: billing_setting.TaskExprCompatible(expression, schema),
				})
			}
		}
		result.Entries = append(result.Entries, entry)
	}
	// Preserve the existing settings editor's full-map interface. Built-in
	// expressions are display defaults only; per-model writes do not persist them.
	for name, expression := range billing_setting.GetBuiltinBillingExprCopy() {
		effective := effectiveModelPricing(values, name)
		if effective["billing_setting.billing_mode"] != "tiered_expr" {
			continue
		}
		if _, ok := values["billing_setting.billing_mode"][name]; !ok {
			values["billing_setting.billing_mode"][name] = "tiered_expr"
		}
		if _, ok := values["billing_setting.billing_expr"][name]; !ok {
			values["billing_setting.billing_expr"][name] = expression
		}
	}
	for key, entries := range values {
		encoded, err := common.Marshal(entries)
		if err != nil {
			return nil, err
		}
		result.Options[key] = string(encoded)
	}
	return result, nil
}

func ValidateModelPricing(name string, values PricingValues) error {
	variants := make(map[string]any)
	for key, expression := range billing_setting.GetPluginBillingExprCopy() {
		if plugin, model, ok := billing_setting.SplitPluginBillingExprKey(key); ok && model == name {
			variants[plugin] = expression
		}
	}
	previous := PricingValues{billing_setting.PluginBillingExprOption: variants}
	if expression, ok := billing_setting.GetBillingExpr(name); ok {
		previous["billing_setting.billing_expr"] = expression
	}
	return validateModelPricing(name, values, previous)
}

// Writes pass the locked database snapshot here, so allowing an unchanged stale
// override cannot bypass validation through an out-of-date process-local cache.
func validateModelPricing(name string, values, previous PricingValues) error {
	generation := jsplugin.DefaultRegistry.Generation()
	plugins := make(map[string]jsplugin.Meta)
	for _, plugin := range generation.Plugins() {
		plugins[plugin.Meta.Key] = plugin.Meta
	}
	return validateModelPricingWithDependencies(name, values, previous, modelPricingValidationDependencies{
		plugins:      plugins,
		resolveAlias: func(name string) (TaskAliasTarget, bool) { return ResolveTaskModelAlias(generation, name) },
	})
}

// Explicit dependencies let staged managed validation reuse the ordinary rules
// without observing a live registry, alias cache, DB or previous-value cache.
type modelPricingValidationDependencies struct {
	plugins      map[string]jsplugin.Meta
	resolveAlias func(string) (TaskAliasTarget, bool)
}

func validateModelPricingWithDependencies(name string, values, previous PricingValues, dependencies modelPricingValidationDependencies) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("model name is required")
	}
	previousVariants, _ := previous[billing_setting.PluginBillingExprOption].(map[string]any)
	variants := map[string]any{}
	if value, exists := values[billing_setting.PluginBillingExprOption]; exists {
		var ok bool
		variants, ok = value.(map[string]any)
		if !ok || variants == nil {
			return errors.New("plugin billing expressions must be a plugin-to-expression object")
		}
		for key, value := range variants {
			expression, ok := value.(string)
			if !ok || strings.TrimSpace(expression) == "" {
				return fmt.Errorf("model %s: plugin %s: billing expression is required", name, key)
			}
			plugin, exists := dependencies.plugins[key]
			if !exists || !slices.Contains(plugin.Models, name) {
				if previousVariants[key] == expression {
					continue
				}
				return fmt.Errorf("model %s: plugin %s does not declare this model", name, key)
			}
			schema, _ := plugin.UsageForModel(name)
			if err := billing_setting.SmokeTestTaskExpr(expression, schema); err != nil {
				return fmt.Errorf("model %s: plugin %s: %w", name, key, err)
			}
		}
	}
	for key, value := range values {
		if key == billing_setting.PluginBillingExprOption {
			continue
		}
		if !IsModelPricingOption(key) {
			return fmt.Errorf("unsupported pricing field: %s", key)
		}
		if key == "billing_setting.billing_mode" {
			if value != "ratio" && value != "tiered_expr" {
				return errors.New("invalid billing mode")
			}
			continue
		}
		if key == "billing_setting.billing_expr" {
			expression, ok := value.(string)
			if !ok || strings.TrimSpace(expression) == "" {
				return errors.New("billing expression is required")
			}
			// Even a model expression currently shadowed by every provider must
			// compile; only its schema-specific smoke tests can be skipped.
			if _, err := billingexpr.CompileFromCache(expression); err != nil {
				return fmt.Errorf("model %s: %w", name, err)
			}
			var err error
			var plugins []jsplugin.Meta
			for _, key := range slices.Sorted(maps.Keys(dependencies.plugins)) {
				plugin := dependencies.plugins[key]
				if slices.Contains(plugin.Models, name) {
					plugins = append(plugins, plugin)
				}
			}
			if len(plugins) > 0 {
				for _, plugin := range plugins {
					if _, overridden := variants[plugin.Key]; overridden {
						continue
					}
					schema, _ := plugin.UsageForModel(name)
					if err = billing_setting.SmokeTestTaskExpr(expression, schema); err != nil {
						return fmt.Errorf("model %s: plugin %s: %w", name, plugin.Key, err)
					}
				}
			} else if target, resolved := dependencies.resolveAlias(name); resolved {
				if plugin, ok := dependencies.plugins[target.PluginKey]; ok {
					schema, _ := plugin.UsageForModel(target.Declared)
					err = billing_setting.SmokeTestTaskExpr(expression, schema)
				} else {
					err = billing_setting.SmokeTestExpr(expression)
				}
			} else if previous[key] != expression || len(billingexpr.UsedUsageKeys(expression)) == 0 {
				err = billing_setting.SmokeTestExpr(expression)
			}
			// With no remaining plugin, an unchanged stored usage expression has
			// no schema to test. Preserve it so removing stale overrides or saving
			// other model prices does not become impossible.
			if err != nil {
				return fmt.Errorf("model %s: %w", name, err)
			}
			continue
		}
		number, ok := value.(float64)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 {
			return fmt.Errorf("%s must be a finite, non-negative number", key)
		}
	}
	if values["billing_setting.billing_mode"] == "tiered_expr" {
		if _, exists := values["billing_setting.billing_expr"]; !exists {
			if _, builtin := billing_setting.GetBuiltinBillingExpr(name); !builtin {
				return errors.New("billing expression is required")
			}
		}
	}
	return nil
}

func UpdateModelPricing(changes []ModelPricingChange) error {
	if len(changes) == 0 {
		return errors.New("select model pricing changes before saving")
	}
	seen := make(map[string]bool)
	for _, change := range changes {
		if ratio_setting.IsStarAIVideoPricingModel(change.ModelName) {
			return seedancePricingManagedError(change.ModelName)
		}
		if seen[change.ModelName] {
			return errors.New("duplicate model pricing change")
		}
		seen[change.ModelName] = true
		if change.ExpectedVersion == "" {
			return ErrModelPricingConflict
		}
	}
	return savePreparedModelPricing(nil, func(draft *modelPricingSaveDraft) error {
		defaults := defaultPricingMaps()
		for _, change := range changes {
			currentCurrency := draft.previous.currencies[change.ModelName]
			if !currentCurrency.HasMetadata {
				return fmt.Errorf("model %s exact metadata is required before saving pricing", change.ModelName)
			}
			targetCurrency := currentCurrency.BillingCurrency
			if strings.TrimSpace(change.BillingCurrency) != "" {
				var err error
				targetCurrency, err = normalizeModelBillingCurrency(change.BillingCurrency)
				if err != nil {
					return fmt.Errorf("model %s: %w", change.ModelName, err)
				}
			}
			previous := modelPricingValues(draft.values, change.ModelName)
			if modelPricingVersion(previous, string(currentCurrency.BillingCurrency)) != change.ExpectedVersion {
				return fmt.Errorf("%w: %s", ErrModelPricingConflict, change.ModelName)
			}
			pricing := change.Pricing
			if change.Reset {
				pricing = modelPricingValues(defaults, change.ModelName)
			}
			if err := validateModelPricingWithDependencies(change.ModelName, pricing, previous, draft.validation); err != nil {
				return err
			}
			replaceModelPricing(draft.values, change.ModelName, pricing)
			draft.currencies[change.ModelName] = string(targetCurrency)
		}
		return nil
	})
}

// UpdateModelPricingOptions keeps legacy single-option callers on the same
// locking, validation and transaction path as the model-level API.
func UpdateModelPricingOptions(updates map[string]string) error {
	return savePreparedModelPricing(nil, func(draft *modelPricingSaveDraft) error {
		values := draft.values
		previous := cloneModelPricingMaps(values)
		names := make(map[string]bool)
		for key, raw := range updates {
			if !IsModelPricingOption(key) {
				return fmt.Errorf("unsupported pricing field: %s", key)
			}
			draft.explicit[key] = true
			var entries map[string]any
			if err := common.UnmarshalJsonStr(raw, &entries); err != nil {
				return err
			}
			if entries == nil {
				return fmt.Errorf("%s must be a JSON object", key)
			}
			preserveStarAIVideoPricingForOption(key, values[key], entries)
			for _, entriesForKey := range []map[string]any{values[key], entries} {
				for name := range entriesForKey {
					if key == billing_setting.PluginBillingExprOption {
						_, model, ok := billing_setting.SplitPluginBillingExprKey(name)
						if !ok {
							return fmt.Errorf("invalid plugin billing expression key: %s", name)
						}
						name = model
					}
					names[name] = true
				}
			}
			values[key] = entries
		}
		for name := range names {
			if ratio_setting.IsStarAIVideoPricingModel(name) {
				continue
			}
			if err := validateModelPricingWithDependencies(name, modelPricingValues(values, name), modelPricingValues(previous, name), draft.validation); err != nil {
				return err
			}
		}
		return nil
	})
}

func preserveStarAIVideoPricingForOption(key string, previous, incoming map[string]any) {
	if key != billing_setting.PluginBillingExprOption {
		for _, name := range ratio_setting.StarAIVideoPricingModels() {
			if value, ok := previous[name]; ok {
				incoming[name] = value
			} else {
				delete(incoming, name)
			}
		}
		return
	}
	for variant := range incoming {
		if _, name, ok := billing_setting.SplitPluginBillingExprKey(variant); ok && ratio_setting.IsStarAIVideoPricingModel(name) {
			delete(incoming, variant)
		}
	}
	for variant, value := range previous {
		if _, name, ok := billing_setting.SplitPluginBillingExprKey(variant); ok && ratio_setting.IsStarAIVideoPricingModel(name) {
			incoming[variant] = value
		}
	}
}

func cloneModelPricingMaps(values map[string]map[string]any) map[string]map[string]any {
	cloned := make(map[string]map[string]any, len(values))
	for key, entries := range values {
		cloned[key] = maps.Clone(entries)
	}
	return cloned
}

func seedancePricingManagedError(name string) error {
	return fmt.Errorf("model %s pricing is managed in the Seedance 2.0 tab", name)
}

// UpdateStarAIVideoPricing atomically persists the editable Seedance matrix
// and replaces the four affected models with the expressions generated from
// that matrix. Provider overrides are removed; legacy ratios remain as an
// inactive fallback while the generated expression is published, avoiding a
// transient unpriced state for concurrent requests.
func UpdateStarAIVideoPricing(prices ratio_setting.StarAIVideoPriceSetting) error {
	expressions, err := ratio_setting.BuildStarAIVideoBillingExpressions(prices)
	if err != nil {
		return err
	}
	return savePreparedModelPricing(prices.OptionValues(), func(draft *modelPricingSaveDraft) error {
		values := draft.values
		modelNames := ratio_setting.StarAIVideoPricingModels()
		for _, name := range modelNames {
			if !draft.previous.currencies[name].HasMetadata {
				return fmt.Errorf("model %s exact metadata is required before publishing Seedance pricing", name)
			}
		}
		for name, expression := range expressions {
			previous := modelPricingValues(values, name)
			next := maps.Clone(previous)
			next["billing_setting.billing_mode"] = billing_setting.BillingModeTieredExpr
			next["billing_setting.billing_expr"] = expression
			next[billing_setting.PluginBillingExprOption] = map[string]any{}
			if err := validateModelPricingWithDependencies(name, next, previous, draft.validation); err != nil {
				return err
			}
			replaceModelPricing(values, name, next)
		}
		for _, name := range modelNames {
			draft.currencies[name] = "CNY"
		}
		return nil
	})
}

// Retained only for the statically referenced, unreachable removePricing branch
// in DeleteModelMetadata, which rejects pricing removal at entry. Never run an
// arbitrary SQL/validation callback through the old publication choreography.
func mutateModelPricingOptions(_ func(*gorm.DB, map[string]map[string]any) error) error {
	return errors.New("pricing removal is not supported for the Molii catalog; edit pricing separately")
}

type modelPricingSaveDraft struct {
	previous   catalogRuntimeInput
	values     map[string]map[string]any
	currencies map[string]string
	validation modelPricingValidationDependencies
	explicit   map[string]bool
}

// The three ordinary pricing savers provide pure transformations of detached
// actual previous values. Only preparation is repeatable; the final callback
// contains SQL alone and uses the authoritative whole-input recheck.
func savePreparedModelPricing(extraOptions map[string]string, prepare func(*modelPricingSaveDraft) error) error {
	var updates, currencies map[string]string
	return withPreparedOrdinaryCatalogMutation(func(ctx context.Context) (*catalogOrdinaryMutation, error) {
		previous, err := captureOrdinaryCatalogMutation(ctx, DB)
		if err != nil {
			return nil, err
		}
		dependencies, err := catalogRuntimeDependencies(previous)
		if err != nil {
			return nil, err
		}
		values := defaultPricingMaps()
		for _, key := range modelPricingOptionKeys {
			if raw, exists := previous.options[key]; exists {
				var entries map[string]any
				if err := common.UnmarshalJsonStr(raw, &entries); err != nil {
					return nil, err
				}
				if entries == nil {
					return nil, fmt.Errorf("%s must be a JSON object", key)
				}
				values[key] = entries
			}
		}
		initial := cloneModelPricingMaps(values)
		draft := modelPricingSaveDraft{previous: previous, values: values, currencies: make(map[string]string), explicit: make(map[string]bool),
			validation: modelPricingValidationDependencies{plugins: dependencies.Plugins, resolveAlias: func(name string) (TaskAliasTarget, bool) {
				target, exists := dependencies.Aliases[jsplugin.ASCIIFold(name)]
				return target, exists
			}}}
		if err := prepare(&draft); err != nil {
			return nil, err
		}
		updates = make(map[string]string, len(modelPricingOptionKeys)+len(extraOptions))
		for _, key := range modelPricingOptionKeys {
			if !draft.explicit[key] && reflect.DeepEqual(initial[key], values[key]) {
				// Preserve untouched raw rows and absence. Missing maps inherit
				// compiled defaults; explicitly supplied empty maps remain overrides.
				continue
			}
			encoded, err := common.Marshal(values[key])
			if err != nil {
				return nil, err
			}
			updates[key] = string(encoded)
		}
		maps.Copy(updates, extraOptions)
		currencies = draft.currencies
		return stageOrdinaryCatalogMutation(previous, updates)
	}, func(prepared *catalogOrdinaryMutation) error {
		return commitOrdinaryCatalogMutationGuarded(context.Background(), DB, prepared, func(tx *gorm.DB) error {
			for _, name := range slices.Sorted(maps.Keys(currencies)) {
				result := tx.Model(&Model{}).Where("model_name = ? AND name_rule = ?", name, NameRuleExact).Update("billing_currency", currencies[name])
				if result.Error != nil {
					return result.Error
				}
				// MySQL reports changed rows, so an unchanged currency may
				// legitimately return zero. Prove the exact postimage instead.
				var matches int64
				if err := tx.Model(&Model{}).Where("model_name = ? AND name_rule = ? AND billing_currency = ?", name, NameRuleExact, currencies[name]).Count(&matches).Error; err != nil {
					return err
				}
				if matches != 1 {
					return fmt.Errorf("model %s exact metadata is required before saving pricing", name)
				}
			}
			for _, key := range slices.Sorted(maps.Keys(updates)) {
				row := Option{Key: key, Value: updates[key]}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value"})}).Create(&row).Error; err != nil {
					return err
				}
			}
			return nil
		}, nil)
	})
}
