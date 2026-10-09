package catalogsync

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExportManaged reads through the caller's transaction and catalog write barrier.
func ExportManaged(ctx context.Context, tx *gorm.DB, sourceID string) (catalogmanifest.Snapshot, error) {
	if tx == nil || strings.TrimSpace(sourceID) == "" {
		return catalogmanifest.Snapshot{}, fmt.Errorf("catalog transaction and source identity are required")
	}
	snapshot := catalogmanifest.Snapshot{
		SchemaVersion: catalogmanifest.SchemaVersion, SourceID: sourceID,
		ExportedAt: time.Now().UTC().Unix(), Complete: true,
		Capabilities: catalogmanifest.RequiredCapabilities(), Coverage: make(map[string]int),
		Entries: make([]catalogmanifest.Entry, 0),
	}
	for _, kind := range catalogmanifest.Kinds() {
		snapshot.Coverage[kind] = 0
	}
	var vendors []model.Vendor
	if err := tx.WithContext(ctx).Order("name ASC").Find(&vendors).Error; err != nil {
		return catalogmanifest.Snapshot{}, fmt.Errorf("load managed vendors: %w", err)
	}
	vendorNames := make(map[int]string, len(vendors))
	for _, vendor := range vendors {
		vendorNames[vendor.Id] = vendor.Name
		value := catalogmanifest.VendorValue{
			Name: vendor.Name, Description: vendor.Description, Icon: vendor.Icon,
			Status: vendor.Status, DisplayOrder: vendor.DisplayOrder,
			CreatedTime: vendor.CreatedTime, UpdatedTime: vendor.UpdatedTime,
		}
		if err := appendManagedEntry(&snapshot, catalogmanifest.KindVendor, vendor.Name, value); err != nil {
			return catalogmanifest.Snapshot{}, err
		}
	}
	var models []model.Model
	if err := tx.WithContext(ctx).Order("model_name ASC").Find(&models).Error; err != nil {
		return catalogmanifest.Snapshot{}, fmt.Errorf("load managed models: %w", err)
	}
	exactCurrencies := make(map[string]string, len(models))
	for _, record := range models {
		vendorName := ""
		if record.VendorID != 0 {
			var exists bool
			vendorName, exists = vendorNames[record.VendorID]
			if !exists {
				return catalogmanifest.Snapshot{}, fmt.Errorf("model %q references missing vendor", record.ModelName)
			}
		}
		value := managedModelValue(record, vendorName)
		if value.BillingCurrency != "USD" && value.BillingCurrency != "CNY" {
			return catalogmanifest.Snapshot{}, fmt.Errorf("model %q has unresolved billing currency", record.ModelName)
		}
		if record.NameRule == model.NameRuleExact {
			exactCurrencies[record.ModelName] = value.BillingCurrency
		}
		if err := appendManagedEntry(&snapshot, catalogmanifest.KindModel, record.ModelName, value); err != nil {
			return catalogmanifest.Snapshot{}, err
		}
	}

	whitelist := catalogmanifest.PriceOptions()
	// Enumerate names without reading credentials or other general option values.
	var optionNames []model.Option
	keyOrder := clause.OrderByColumn{Column: clause.Column{Name: "key"}}
	if err := tx.WithContext(ctx).Select("key").Order(keyOrder).Find(&optionNames).Error; err != nil {
		return catalogmanifest.Snapshot{}, fmt.Errorf("enumerate price fields: %w", err)
	}
	keys := make([]string, 0, len(whitelist))
	for _, option := range optionNames {
		if _, known := whitelist[option.Key]; known {
			keys = append(keys, option.Key)
		} else if catalogmanifest.IsPricingNamespace(option.Key) {
			return catalogmanifest.Snapshot{}, fmt.Errorf("unknown price field %q", option.Key)
		}
	}
	var options []model.Option
	if len(keys) > 0 {
		if err := tx.WithContext(ctx).Where(map[string]any{"key": keys}).Order(keyOrder).Find(&options).Error; err != nil {
			return catalogmanifest.Snapshot{}, fmt.Errorf("load managed price fields: %w", err)
		}
	}
	for _, option := range options {
		spec := whitelist[option.Key]
		leaves := map[string]common.RawMessage{"": common.RawMessage(option.Value)}
		if spec.Map {
			leaves = nil
			if err := common.UnmarshalJsonStr(option.Value, &leaves); err != nil || leaves == nil {
				return catalogmanifest.Snapshot{}, fmt.Errorf("price field %q must be a JSON object", option.Key)
			}
		}
		leafNames := make([]string, 0, len(leaves))
		for name := range leaves {
			leafNames = append(leafNames, name)
		}
		slices.Sort(leafNames)
		for _, leafName := range leafNames {
			key := catalogmanifest.PriceKey{Option: option.Key}
			currency := spec.Currency
			if spec.Map {
				if strings.TrimSpace(leafName) == "" {
					return catalogmanifest.Snapshot{}, fmt.Errorf("price field %q has empty leaf key", option.Key)
				}
				key.Path = "/" + catalogmanifest.PointerSegment(leafName)
			}
			if spec.Kind == catalogmanifest.KindModelPrice || spec.Kind == catalogmanifest.KindPluginPrice {
				key.Model = leafName
				if spec.Kind == catalogmanifest.KindPluginPrice {
					var valid bool
					key.Plugin, key.Model, valid = billing_setting.SplitPluginBillingExprKey(leafName)
					if !valid {
						return catalogmanifest.Snapshot{}, fmt.Errorf("invalid plugin price key in %q", option.Key)
					}
				}
				var reliable bool
				currency, reliable = exactCurrencies[key.Model]
				if !reliable {
					return catalogmanifest.Snapshot{}, fmt.Errorf("price %q model %q currency requires exact metadata", option.Key, key.Model)
				}
			}
			raw := string(leaves[leafName])
			if err := catalogmanifest.ValidatePriceValue(option.Key, raw); err != nil {
				return catalogmanifest.Snapshot{}, err
			}
			canonical, err := catalogmanifest.CanonicalJSON(raw)
			if err != nil {
				return catalogmanifest.Snapshot{}, fmt.Errorf("price field %q has invalid JSON", option.Key)
			}
			encodedKey, err := catalogmanifest.EncodePriceKey(key)
			if err != nil {
				return catalogmanifest.Snapshot{}, err
			}
			value := catalogmanifest.PriceValue{Value: canonical, BillingCurrency: currency, Unit: spec.Unit}
			if err := appendManagedEntry(&snapshot, spec.Kind, encodedKey, value); err != nil {
				return catalogmanifest.Snapshot{}, err
			}
		}
	}
	slices.SortFunc(snapshot.Entries, func(a, b catalogmanifest.Entry) int {
		if compared := strings.Compare(a.Kind, b.Kind); compared != 0 {
			return compared
		}
		return strings.Compare(a.Key, b.Key)
	})
	digest, err := catalogmanifest.SnapshotDigest(snapshot)
	if err != nil {
		return catalogmanifest.Snapshot{}, err
	}
	snapshot.Digest = digest
	if err := catalogmanifest.ValidateSnapshot(snapshot); err != nil {
		return catalogmanifest.Snapshot{}, err
	}
	return snapshot, nil
}

// This projection is kept separate so model can own/reuse it when target
// snapshot capture is introduced, without importing this CLI-facing package.
func managedModelValue(record model.Model, vendorName string) catalogmanifest.ModelValue {
	return catalogmanifest.ModelValue{
		ModelName: record.ModelName, DisplayName: record.DisplayName, Description: record.Description,
		DescriptionEN: record.DescriptionEN, Icon: record.Icon, Tags: record.Tags, Vendor: vendorName,
		BillingCurrency: record.BillingCurrency, Endpoints: record.Endpoints, Status: record.Status,
		SyncOfficial: record.SyncOfficial, NameRule: record.NameRule, ContextLength: record.ContextLength,
		MaxOutputTokens: record.MaxOutputTokens, KnowledgeCutoff: record.KnowledgeCutoff, ReleaseDate: record.ReleaseDate,
		InputModalities: slices.Clone(record.InputModalities), OutputModalities: slices.Clone(record.OutputModalities),
		Capabilities: slices.Clone(record.Capabilities), MetadataSource: record.MetadataSource,
		MetadataVerifiedAt: record.MetadataVerifiedAt, MarketplaceEnabled: record.MarketplaceEnabled,
		DisplayOrder: record.DisplayOrder, SupportedParameters: slices.Clone(record.SupportedParameters),
		SupportedResolutions: slices.Clone(record.SupportedResolutions), SupportedAspectRatios: slices.Clone(record.SupportedAspectRatios),
		MaxInputImages: record.MaxInputImages, OutputFormats: slices.Clone(record.OutputFormats),
		MinDuration: record.MinDuration, MaxDuration: record.MaxDuration, ReferenceModalities: slices.Clone(record.ReferenceModalities),
		CreatedTime: record.CreatedTime, UpdatedTime: record.UpdatedTime,
	}
}

func appendManagedEntry(snapshot *catalogmanifest.Snapshot, kind, key string, value any) error {
	encoded, err := common.Marshal(value)
	if err != nil {
		return err
	}
	canonical, err := catalogmanifest.CanonicalJSON(string(encoded))
	if err != nil {
		return err
	}
	snapshot.Entries = append(snapshot.Entries, catalogmanifest.Entry{Kind: kind, Key: key, Value: canonical})
	snapshot.Coverage[kind]++
	return nil
}
