package model

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/task_pricing_setting"
	"github.com/tidwall/gjson"
)

// Supplied from an authoritative, coherent capture by the lifecycle caller.
// No registry, DB, alias cache or runtime setting is read while staging.
// Inputs must not be concurrently mutated; no reference escapes staging.
type catalogPricingDependencies struct {
	Plugins map[string]jsplugin.Meta
	Aliases map[string]TaskAliasTarget
}

// A leaf identifies a persisted map key (including plugin::model), or the empty
// name for a scalar. Adopted/replaced identities must not use stale retention.
type catalogPricingLeaf struct{ Option, Name string }

// Only successful staging constructs a candidate. Keep fields private and do
// not modify it afterwards; publish may be retried without changing its input.
type catalogPricingCandidate struct {
	options   map[string]string
	ratios    map[string]map[string]float64
	billing   billing_setting.BillingSetting
	grok      ratio_setting.MoliiGrokPriceSetting
	video     ratio_setting.StarAIVideoPriceSetting
	grokTools operation_setting.MoliiGrokToolPriceSetting
	factors   task_pricing_setting.TaskPricingSetting
	tools     *operation_setting.PreparedToolPrices
}

// stageCommittedCatalogPricing reconstructs already committed rows. Its caller
// must prove that provenance; never use this API to authorize prospective edits.
// Historical orphans and unchanged stale usage overrides follow ordinary save
// policy. This does NOT satisfy managed export's metadata/currency completeness.
// Run all staging outside SQL fences, registry pins and catalog selection locks.
func stageCommittedCatalogPricing(options map[string]string, dependencies catalogPricingDependencies) (*catalogPricingCandidate, error) {
	return stageProspectiveCatalogPricing(options, options, dependencies, nil)
}

// stageProspectiveCatalogPricing requires the actual prior persisted options,
// not the proposed values. strict identifies source-created/changed/adopted
// leaves, even same-value adoption. Managed attestation, dependency equality,
// currency completeness and transaction binding remain separate prerequisites.
func stageProspectiveCatalogPricing(options, previous map[string]string, dependencies catalogPricingDependencies, strict []catalogPricingLeaf) (*catalogPricingCandidate, error) {
	defaults, err := catalogPricingDefaults()
	if err != nil {
		return nil, err
	}
	whitelist := catalogmanifest.PriceOptions()
	values := maps.Clone(defaults)
	for key, value := range options {
		if _, known := whitelist[key]; known {
			values[key] = value
		} else if catalogmanifest.IsPricingNamespace(key) {
			return nil, fmt.Errorf("unsupported catalog pricing field %q", key)
		}
	}
	decoded := make(map[string]map[string]common.RawMessage, len(values))
	for _, key := range slices.Sorted(maps.Keys(values)) {
		if _, known := whitelist[key]; !known {
			return nil, fmt.Errorf("unsupported compiled catalog pricing field %q", key)
		}
		raw := values[key]
		leaves := map[string]common.RawMessage{"": common.RawMessage(raw)}
		if whitelist[key].Map {
			leaves, err = decodeCatalogPricingMap(key, raw)
			if err != nil {
				return nil, err
			}
		}
		for _, leaf := range slices.Sorted(maps.Keys(leaves)) {
			if key == billing_setting.PluginBillingExprOption {
				if _, _, valid := billing_setting.SplitPluginBillingExprKey(leaf); !valid {
					return nil, fmt.Errorf("invalid plugin price identity %q", leaf)
				}
			}
			if err := catalogmanifest.ValidatePriceValue(key, string(leaves[leaf])); err != nil {
				return nil, err
			}
		}
		decoded[key] = leaves
	}
	if len(values) != len(whitelist) {
		return nil, fmt.Errorf("compiled catalog pricing defaults are incomplete")
	}
	next, err := catalogCandidateModelMaps(values)
	if err != nil {
		return nil, err
	}
	priorOptions := maps.Clone(defaults)
	for key, value := range previous {
		if IsModelPricingOption(key) {
			priorOptions[key] = value
		}
	}
	prior, err := catalogCandidateModelMaps(priorOptions)
	if err != nil {
		return nil, err
	}
	seen := make(map[catalogPricingLeaf]bool)
	for _, leaf := range strict {
		if seen[leaf] {
			return nil, fmt.Errorf("duplicate strict catalog price identity")
		}
		seen[leaf] = true
		if _, explicit := options[leaf.Option]; !explicit {
			return nil, fmt.Errorf("strict catalog price identity is not explicitly configured")
		}
		if _, exists := decoded[leaf.Option][leaf.Name]; !exists {
			return nil, fmt.Errorf("strict catalog price identity is absent from candidate")
		}
		delete(prior[leaf.Option], leaf.Name)
	}
	validation := modelPricingValidationDependencies{plugins: dependencies.Plugins, resolveAlias: func(name string) (TaskAliasTarget, bool) {
		target, ok := dependencies.Aliases[jsplugin.ASCIIFold(name)]
		return target, ok
	}}
	names := make(map[string]bool)
	for key, entries := range next {
		for name := range entries {
			if key == billing_setting.PluginBillingExprOption {
				_, name, _ = billing_setting.SplitPluginBillingExprKey(name)
			}
			names[name] = true
		}
	}
	for _, name := range slices.Sorted(maps.Keys(names)) {
		if err := validateModelPricingWithDependencies(name, modelPricingValues(next, name), modelPricingValues(prior, name), validation); err != nil {
			return nil, err
		}
	}
	candidate := &catalogPricingCandidate{options: values, ratios: make(map[string]map[string]float64)}
	for key := range ratio_setting.GetDefaultPricingMaps() {
		var ratios map[string]float64
		if err := common.UnmarshalJsonStr(values[key], &ratios); err != nil {
			return nil, err
		}
		candidate.ratios[key] = ratios
	}
	var tools operation_setting.ToolPriceSetting
	for namespace, destination := range map[string]any{
		"billing_setting": &candidate.billing, "molii_grok_price": &candidate.grok,
		"starai_video_price": &candidate.video, "molii_grok_tool_price": &candidate.grokTools,
		"task_pricing_setting": &candidate.factors, "tool_price_setting": &tools,
	} {
		fields := make(map[string]common.RawMessage)
		for key, raw := range values {
			if field, matches := strings.CutPrefix(key, namespace+"."); matches {
				fields[field] = common.RawMessage(raw)
			}
		}
		encoded, err := common.Marshal(fields)
		if err != nil {
			return nil, err
		}
		if err := common.Unmarshal(encoded, destination); err != nil {
			return nil, err
		}
	}
	candidate.tools, err = operation_setting.PrepareToolPrices(tools.Prices)
	if err != nil {
		return nil, err
	}
	return candidate, nil
}

func catalogPricingDefaults() (map[string]string, error) {
	options := make(map[string]string)
	for key, values := range ratio_setting.GetDefaultPricingMaps() {
		encoded, err := common.Marshal(values)
		if err != nil {
			return nil, err
		}
		options[key] = string(encoded)
	}
	for namespace, value := range map[string]any{
		"billing_setting":       billing_setting.DefaultBillingSetting(),
		"molii_grok_price":      ratio_setting.DefaultMoliiGrokPriceSetting(),
		"starai_video_price":    ratio_setting.DefaultStarAIVideoPriceSetting(),
		"molii_grok_tool_price": operation_setting.DefaultMoliiGrokToolPriceSetting(),
		"task_pricing_setting":  task_pricing_setting.DefaultTaskPricingSetting(),
		"tool_price_setting":    operation_setting.DefaultToolPriceSetting(),
	} {
		encoded, err := common.Marshal(value)
		if err != nil {
			return nil, err
		}
		var fields map[string]common.RawMessage
		if err := common.Unmarshal(encoded, &fields); err != nil {
			return nil, err
		}
		for field, raw := range fields {
			options[namespace+"."+field] = string(raw)
		}
	}
	return options, nil
}

func decodeCatalogPricingMap(key, raw string) (map[string]common.RawMessage, error) {
	var entries map[string]common.RawMessage
	if err := common.UnmarshalJsonStr(raw, &entries); err != nil || entries == nil {
		return nil, fmt.Errorf("catalog pricing field %q must be a JSON object", key)
	}
	seen := make(map[string]bool)
	ambiguous := false
	gjson.Parse(raw).ForEach(func(name, _ gjson.Result) bool {
		leaf := name.String()
		if strings.TrimSpace(leaf) == "" || seen[leaf] {
			ambiguous = true
		}
		seen[leaf] = true
		return true
	})
	if ambiguous {
		return nil, fmt.Errorf("catalog pricing field %q has ambiguous leaf identities", key)
	}
	return entries, nil
}

func catalogCandidateModelMaps(options map[string]string) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any)
	for _, key := range modelPricingOptionKeys {
		// Prior values confer the ordinary stale-retention exception. Ambiguous
		// JSON must not silently turn last-key-wins decoding into provenance.
		if _, err := decodeCatalogPricingMap(key, options[key]); err != nil {
			return nil, err
		}
		var entries map[string]any
		if err := common.UnmarshalJsonStr(options[key], &entries); err != nil || entries == nil {
			return nil, fmt.Errorf("catalog pricing field %q must be a JSON object", key)
		}
		result[key] = entries
	}
	return result, nil
}

// publishGuarded requires the caller's catalog write barrier across commit,
// this batch, derived-cache invalidation and durable revision confirmation.
// It does no decoding, compilation, evaluation, DB work or callbacks. It does
// not attest lifecycle success. Direct config reflection needs the same outer
// barrier; package getters are individually synchronized against typed setters.
func (candidate *catalogPricingCandidate) publishGuarded() {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	ratio_setting.PublishPricingMaps(candidate.ratios)
	billing_setting.PublishBillingSetting(candidate.billing)
	ratio_setting.PublishMoliiGrokPriceSetting(candidate.grok)
	ratio_setting.PublishStarAIVideoPriceSetting(candidate.video)
	operation_setting.PublishMoliiGrokToolPriceSetting(candidate.grokTools)
	task_pricing_setting.PublishTaskPricingSetting(candidate.factors)
	operation_setting.PublishToolPrices(candidate.tools)
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	for key, value := range candidate.options {
		common.OptionMap[key] = value
	}
}
