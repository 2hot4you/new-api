package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"gorm.io/gorm"
)

// Inputs and attestations are server-owned immutable canonical payloads. Neither
// is a request DTO; only their digest may be exposed in a sealed plan.
type catalogValidationInput struct{ canonical string }
type catalogValidationAttestation struct{ payload, digest string }

type catalogValidationPlugin struct {
	Key           string
	APIVersion    int
	Version       string
	SourceHash    string
	Models        []string
	UsageSchema   map[string]jsplugin.UsageFieldSchema
	UsageProfiles []jsplugin.UsageProfile
	Capabilities  []string
}

type catalogValidationData struct {
	Before          catalogmanifest.Snapshot
	Draft           catalogmanifest.Snapshot
	StrictPrices    []string
	Desired         []catalogDesiredPlugin
	Effective       map[string]catalogValidationPlugin
	Factory         map[string]catalogValidationPlugin
	Overrides       map[string]catalogValidationPlugin
	Aliases         map[string]TaskAliasTarget
	Switches        map[string]string
	Enabled         bool
	DisabledFactory []string
	Status          string
}

// Capture only reads, projects and canonicalizes. Call with the authoritative
// transaction and its existing pin; do not acquire another registry lock here.
// Before/Draft must be the complete snapshots from that transaction. Their
// exact canonical contents, including target currency, are committed below.
func captureCatalogValidationInputTx(tx *gorm.DB, pin *jsplugin.GenerationPin, before, draft catalogmanifest.Snapshot, strictPrices []string) (catalogValidationInput, error) {
	if tx == nil || pin == nil || pin.Generation == nil || pin.Status != "success" {
		return catalogValidationInput{}, errors.New("catalog effective plugins are not ready")
	}
	for _, snapshot := range []catalogmanifest.Snapshot{before, draft} {
		if err := catalogmanifest.ValidateSnapshotStructure(snapshot); err != nil {
			return catalogValidationInput{}, err
		}
	}
	desired, err := readCatalogDesiredPluginsTx(tx)
	if err != nil {
		return catalogValidationInput{}, err
	}
	var options []Option
	if err := tx.Select("key", "value").Where(map[string]any{"key": []string{"TaskPluginEnabled", setting.TaskPluginDisabledFactoryKeysKey}}).Find(&options).Error; err != nil {
		return catalogValidationInput{}, err
	}
	switches := make(map[string]string)
	enabled := true // production startup default; presence remains fingerprinted
	disabled := []string{}
	for _, option := range options {
		if _, duplicate := switches[option.Key]; duplicate {
			return catalogValidationInput{}, errors.New("catalog plugin switches are ambiguous")
		}
		switches[option.Key] = option.Value
		switch option.Key {
		case "TaskPluginEnabled":
			if option.Value != "true" && option.Value != "false" {
				return catalogValidationInput{}, errors.New("catalog plugin switch is invalid")
			}
			enabled = option.Value == "true"
		case setting.TaskPluginDisabledFactoryKeysKey:
			if strings.TrimSpace(option.Value) == "" {
				continue
			}
			if err := common.UnmarshalJsonStr(option.Value, &disabled); err != nil || disabled == nil {
				return catalogValidationInput{}, errors.New("catalog factory switch is invalid")
			}
			seen := make(map[string]bool)
			for _, key := range disabled {
				if key == "" || strings.TrimSpace(key) != key || seen[key] {
					return catalogValidationInput{}, errors.New("catalog factory switch is ambiguous")
				}
				seen[key] = true
			}
		}
	}
	slices.Sort(disabled)
	if enabled != pin.Enabled || !slices.Equal(disabled, pin.DisabledFactory) {
		return catalogValidationInput{}, errors.New("catalog desired and effective plugin switches differ")
	}
	expectedOverrides := make(map[string]jsplugin.PinnedPluginIdentity)
	for _, plugin := range desired {
		if !plugin.Enabled {
			continue
		}
		actual, ok := pin.Overrides[plugin.Key]
		if !ok || actual.SourceHash != plugin.SourceHash || actual.Meta.Key != plugin.Key || actual.Meta.APIVersion != plugin.APIVersion || actual.Meta.Version != plugin.Version {
			return catalogValidationInput{}, errors.New("catalog desired and effective plugin identities differ")
		}
		expectedOverrides[plugin.Key] = actual
	}
	if len(expectedOverrides) != len(pin.Overrides) {
		return catalogValidationInput{}, errors.New("catalog plugin override publication is pending")
	}
	expected := make(map[string]jsplugin.PinnedPluginIdentity)
	if enabled {
		for key, plugin := range pin.Factory {
			if !slices.Contains(disabled, key) {
				expected[key] = plugin
			}
		}
		maps.Copy(expected, expectedOverrides)
	}
	if len(expected) != len(pin.Effective) {
		return catalogValidationInput{}, errors.New("catalog effective plugin set differs")
	}
	projected := make([]map[string]catalogValidationPlugin, 0, 3)
	for _, layer := range []map[string]jsplugin.PinnedPluginIdentity{pin.Effective, pin.Factory, pin.Overrides} {
		facts := make(map[string]catalogValidationPlugin)
		for key, plugin := range layer {
			if len(plugin.SourceHash) != 64 || plugin.Meta.Key != key || plugin.Meta.APIVersion != 1 || plugin.Meta.Version == "" {
				return catalogValidationInput{}, errors.New("catalog compiled plugin identity is unprovable")
			}
			facts[key] = catalogValidationPlugin{key, plugin.Meta.APIVersion, plugin.Meta.Version, plugin.SourceHash, plugin.Meta.Models, plugin.Meta.UsageSchema, plugin.Meta.UsageProfiles, plugin.Meta.RequiredCapabilities}
		}
		projected = append(projected, facts)
	}
	for key, plugin := range expected {
		actual, ok := pin.Effective[key]
		left, err := common.Marshal(plugin)
		if err != nil {
			return catalogValidationInput{}, err
		}
		right, err := common.Marshal(actual)
		if err != nil {
			return catalogValidationInput{}, err
		}
		// RetainsIncumbent is publication intent, also set for healthy unchanged
		// overrides during factory-switch rebuilds. Exact identity/schema and
		// successful status prove agreement; stale retained programs differ here.
		if !ok || string(left) != string(right) {
			return catalogValidationInput{}, errors.New("catalog effective plugin was retained or changed")
		}
	}
	aliases, err := buildTaskAliasViewTx(tx, pin.Generation, true)
	if err != nil {
		return catalogValidationInput{}, err
	}
	// Reuse the snapshot digest's exact content representation, including
	// nested price JSON, rather than depending on a map-to-entry iteration order.
	for _, snapshot := range []*catalogmanifest.Snapshot{&before, &draft} {
		canonical, err := catalogmanifest.CanonicalSnapshotContent(*snapshot)
		if err != nil {
			return catalogValidationInput{}, err
		}
		if err := common.UnmarshalJsonStr(canonical, snapshot); err != nil {
			return catalogValidationInput{}, err
		}
	}
	strictPrices = slices.Clone(strictPrices)
	slices.Sort(strictPrices)
	data := catalogValidationData{Before: before, Draft: draft, StrictPrices: strictPrices, Desired: desired, Effective: projected[0], Factory: projected[1], Overrides: projected[2], Aliases: aliases.byFold, Switches: switches, Enabled: enabled, DisabledFactory: disabled, Status: pin.Status}
	encoded, err := common.Marshal(data)
	if err != nil {
		return catalogValidationInput{}, err
	}
	canonical, err := catalogmanifest.CanonicalJSON(string(encoded))
	return catalogValidationInput{canonical: canonical}, err
}

// Must run after releasing all SQL fences and registry pins. This is the ONLY
// expression compiler/smoke boundary in managed validation. Decoding an owned
// canonical payload prevents callers mutating maps after capture from changing
// what was validated. This function does not read DB/OptionMap/live registries.
func stageCatalogValidation(input catalogValidationInput) (catalogValidationAttestation, error) {
	var data catalogValidationData
	if input.canonical == "" {
		return catalogValidationAttestation{}, ErrCatalogSyncPlanStale
	}
	if err := common.UnmarshalJsonStr(input.canonical, &data); err != nil {
		return catalogValidationAttestation{}, err
	}
	previous := make(map[string]map[string]any)
	next := make(map[string]map[string]any)
	names := make(map[string]bool)
	strict := make(map[string]bool)
	for _, id := range data.StrictPrices {
		if strict[id] {
			return catalogValidationAttestation{}, errors.New("duplicate managed validation identity")
		}
		strict[id] = true
	}
	found := make(map[string]bool)
	for i, snapshot := range []catalogmanifest.Snapshot{data.Before, data.Draft} {
		values := previous
		if i == 1 {
			values = next
		}
		for _, entry := range snapshot.Entries {
			id := catalogmanifest.EntryID(entry)
			if i == 1 && entry.Kind != catalogmanifest.KindModel && entry.Kind != catalogmanifest.KindVendor {
				found[id] = true
			}
			if entry.Kind != catalogmanifest.KindModelPrice && entry.Kind != catalogmanifest.KindPluginPrice {
				continue
			}
			key, err := catalogmanifest.DecodePriceKey(entry.Key)
			if err != nil {
				return catalogValidationAttestation{}, err
			}
			if i == 0 && strict[id] {
				continue
			}
			var price catalogmanifest.PriceValue
			if err := common.UnmarshalJsonStr(entry.Value, &price); err != nil {
				return catalogValidationAttestation{}, err
			}
			var value any
			if err := common.UnmarshalJsonStr(price.Value, &value); err != nil {
				return catalogValidationAttestation{}, err
			}
			if values[key.Option] == nil {
				values[key.Option] = make(map[string]any)
			}
			leaf := key.Model
			if entry.Kind == catalogmanifest.KindPluginPrice {
				leaf = billing_setting.PluginBillingExprKey(key.Plugin, key.Model)
			}
			values[key.Option][leaf] = value
			names[key.Model] = true
		}
	}
	for id := range strict {
		if !found[id] {
			return catalogValidationAttestation{}, errors.New("managed validation identity is absent from draft")
		}
	}
	dependencies := modelPricingValidationDependencies{plugins: make(map[string]jsplugin.Meta), resolveAlias: func(name string) (TaskAliasTarget, bool) {
		target, ok := data.Aliases[jsplugin.ASCIIFold(name)]
		return target, ok
	}}
	for key, plugin := range data.Effective {
		dependencies.plugins[key] = jsplugin.Meta{Key: key, APIVersion: plugin.APIVersion, Version: plugin.Version, Models: plugin.Models, UsageSchema: plugin.UsageSchema, UsageProfiles: plugin.UsageProfiles, RequiredCapabilities: plugin.Capabilities}
	}
	for _, name := range slices.Sorted(maps.Keys(names)) {
		if err := validateModelPricingWithDependencies(name, modelPricingValues(next, name), modelPricingValues(previous, name), dependencies); err != nil {
			return catalogValidationAttestation{}, err
		}
	}
	return catalogValidationAttestation{payload: input.canonical, digest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(input.canonical)))}, nil
}

// No compilation, hooks, evaluation or global reads. 4c2 must invoke this with
// freshly captured authoritative inputs before any catalog/baseline writes.
// It is an additional prerequisite, never a replacement for auth, plan digest,
// expiry, reference checks, target incarnations or operation idempotency.
func verifyCatalogValidation(row CatalogSyncPlan, plan catalogmanifest.Plan, input catalogValidationInput) error {
	if input.canonical == "" || row.Validation == "" || plan.ValidationDigest == "" || plan.ReferenceDigest == "" || string(row.Validation) != input.canonical {
		return ErrCatalogSyncPlanStale
	}
	if plan.ValidationDigest != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(input.canonical))) {
		return ErrCatalogSyncPlanStale
	}
	return nil
}
