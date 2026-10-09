package model

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CaptureCatalogTargetTx adds local opaque incarnation evidence. HMAC prevents
// IDs from being exposed or guessed from the wire; soft-delete/recreate obtains
// a different ID even when name, timestamps and mutable values are identical.
func CaptureCatalogTargetTx(ctx context.Context, tx *gorm.DB, targetID string, state *CatalogSyncState) (catalogmanifest.Snapshot, error) {
	return captureCatalogTargetTx(ctx, tx, targetID, state, false)
}

// Structural capture is for attestation rechecks, never standalone approval.
func captureCatalogTargetTx(ctx context.Context, tx *gorm.DB, targetID string, state *CatalogSyncState, structureOnly bool) (catalogmanifest.Snapshot, error) {
	snapshot, err := exportManagedCatalogTx(ctx, tx, targetID, structureOnly)
	if err != nil {
		return snapshot, err
	}
	if state == nil || len(state.IncarnationKey) != 64 {
		return catalogmanifest.Snapshot{}, fmt.Errorf("missing catalog incarnation key")
	}
	snapshot.ObjectVersions = map[string]string{}
	for _, kind := range []string{catalogmanifest.KindModel, catalogmanifest.KindVendor} {
		var rows []struct {
			ID   int
			Name string
		}
		query := tx.Model(&Vendor{}).Select("id", "name")
		if kind == catalogmanifest.KindModel {
			query = tx.Model(&Model{}).Select("id", "model_name AS name")
		}
		if err := query.Scan(&rows).Error; err != nil {
			return catalogmanifest.Snapshot{}, err
		}
		for _, row := range rows {
			id := catalogmanifest.EntryID(catalogmanifest.Entry{Kind: kind, Key: row.Name})
			mac := hmac.New(sha256.New, []byte(state.IncarnationKey))
			fmt.Fprintf(mac, "%s\x00%d", kind, row.ID)
			snapshot.ObjectVersions[id] = fmt.Sprintf("%x", mac.Sum(nil))
		}
	}
	snapshot.Digest, err = catalogmanifest.SnapshotDigest(snapshot)
	return snapshot, err
}

// A repair save must work even if existing metadata/prices cannot yet pass v2
// validation. Hash persisted fields, limiting option values to the whitelist.
func catalogPersistedDigest(tx *gorm.DB) (string, error) {
	var models []Model
	var vendors []Vendor
	var options []Option
	if err := tx.Order("id").Find(&models).Error; err != nil {
		return "", err
	}
	if err := tx.Order("id").Find(&vendors).Error; err != nil {
		return "", err
	}
	keys := make([]string, 0)
	for key := range catalogmanifest.PriceOptions() {
		keys = append(keys, key)
	}
	if err := tx.Where(map[string]any{"key": keys}).Order(clause.OrderByColumn{Column: clause.Column{Name: "key"}}).Find(&options).Error; err != nil {
		return "", err
	}
	for i := range options {
		if canonical, err := catalogmanifest.CanonicalJSON(options[i].Value); err == nil {
			options[i].Value = canonical
		}
	}
	encoded, err := common.Marshal([]any{models, vendors, options})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(encoded)), nil
}

// ExportManagedCatalogTx reads through the caller's stable transaction and
// catalog read/write barrier. It never reads process price caches.
func ExportManagedCatalogTx(ctx context.Context, tx *gorm.DB, sourceID string) (catalogmanifest.Snapshot, error) {
	return exportManagedCatalogTx(ctx, tx, sourceID, false)
}

func exportManagedCatalogTx(ctx context.Context, tx *gorm.DB, sourceID string, structureOnly bool) (catalogmanifest.Snapshot, error) {
	validatePrice := catalogmanifest.ValidatePriceValue
	validateSnapshot := catalogmanifest.ValidateSnapshot
	if structureOnly {
		validatePrice = catalogmanifest.ValidatePriceValueStructure
		validateSnapshot = catalogmanifest.ValidateSnapshotStructure
	}
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
	var vendors []Vendor
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
	var models []Model
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
		if record.NameRule == NameRuleExact {
			exactCurrencies[record.ModelName] = value.BillingCurrency
		}
		if err := appendManagedEntry(&snapshot, catalogmanifest.KindModel, record.ModelName, value); err != nil {
			return catalogmanifest.Snapshot{}, err
		}
	}

	whitelist := catalogmanifest.PriceOptions()
	// Enumerate names without reading credentials or other general option values.
	var optionNames []Option
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
	var options []Option
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
			if err := validatePrice(option.Key, raw); err != nil {
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
	if err := validateSnapshot(snapshot); err != nil {
		return catalogmanifest.Snapshot{}, err
	}
	return snapshot, nil
}

// Source and target use the same whitelist projection; local IDs never cross
// the wire, and audit timestamps retain source provenance.
func managedModelValue(record Model, vendorName string) catalogmanifest.ModelValue {
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
