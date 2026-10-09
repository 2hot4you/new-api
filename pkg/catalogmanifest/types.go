// Package catalogmanifest defines the database-independent managed catalog wire contract.
package catalogmanifest

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/task_pricing_setting"
)

const SchemaVersion = 2

const (
	KindVendor       = "vendor"
	KindModel        = "model"
	KindModelPrice   = "model_price"
	KindPluginPrice  = "plugin_price"
	KindSpecialPrice = "special_price"
	KindToolPrice    = "tool_price"
)

type Actor struct {
	UserID    int    `json:"user_id"`
	SessionID string `json:"session_id"`
	TargetID  string `json:"target_id"`
	// Management callers must populate and revalidate both versions under the
	// apply transaction; a matching session ID alone is not authentication.
	AuthVersion    int `json:"auth_version"`
	SessionVersion int `json:"session_version"`
}

type Entry struct {
	Kind  string `json:"kind"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Snapshot struct {
	SchemaVersion int               `json:"schema_version"`
	SourceID      string            `json:"source_id"`
	ExportedAt    int64             `json:"exported_at"`
	Digest        string            `json:"digest"`
	Complete      bool              `json:"complete"`
	Capabilities  map[string]string `json:"capabilities"`
	Coverage      map[string]int    `json:"coverage"`
	Entries       []Entry           `json:"entries"`
	// ObjectVersions is optional local-only opaque target incarnation state.
	// It contains no raw database IDs and is not required on the source wire.
	// Nonempty tokens bind the target digest and are persisted in its baseline.
	ObjectVersions map[string]string `json:"object_versions,omitempty"`
}

type Baseline struct {
	SourceID       string            `json:"source_id"`
	Generation     int64             `json:"generation"`
	Entries        []Entry           `json:"entries"`
	ObjectVersions map[string]string `json:"object_versions"`
}

type Resolution struct {
	OverwriteKeys  []string `json:"overwrite_keys"`
	ConfirmDeletes bool     `json:"confirm_deletes"`
}

type Change struct {
	Kind   string `json:"kind"`
	Key    string `json:"key"`
	Action string `json:"action"`
	Reason string `json:"reason"`
	Before *Entry `json:"before"`
	Base   *Entry `json:"base"`
	After  *Entry `json:"after"`
}

type Plan struct {
	ID                 string     `json:"id"`
	Kind               string     `json:"kind"`
	RestoreOperationID string     `json:"restore_operation_id,omitempty"`
	ValidationDigest   string     `json:"validation_digest"`
	ReferenceDigest    string     `json:"reference_digest"`
	Snapshot           Snapshot   `json:"snapshot"`
	TargetDigest       string     `json:"target_digest"`
	BaselineGeneration int64      `json:"baseline_generation"`
	Actor              Actor      `json:"actor"`
	ExpiresAt          int64      `json:"expires_at"`
	Resolution         Resolution `json:"resolution"`
	Digest             string     `json:"digest"`
	Changes            []Change   `json:"changes"`
}

type Result struct {
	OperationID string `json:"operation_id"`
	State       string `json:"state"`
	Revision    int64  `json:"revision"`
}

// PriceKey encodes compound identities without delimiter collisions. Path is
// an RFC 6901 JSON Pointer within Option, never a dot-separated field name.
type PriceKey struct {
	Option string `json:"option"`
	Model  string `json:"model,omitempty"`
	Plugin string `json:"plugin,omitempty"`
	Path   string `json:"path"`
}

// PriceValue wraps an explicitly stored leaf; absence means no Entry exists.
// Unit describes the stored value, without converting coefficients or currency.
type PriceValue struct {
	Value           string `json:"value"`
	BillingCurrency string `json:"billing_currency"`
	Unit            string `json:"unit"`
}

type VendorValue struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Icon         string `json:"icon"`
	Status       int    `json:"status"`
	DisplayOrder int    `json:"display_order"`
	CreatedTime  int64  `json:"created_time"`
	UpdatedTime  int64  `json:"updated_time"`
}

// ModelValue contains persisted metadata, replacing the numeric vendor foreign
// key with its name. Audit times are retained for provenance; consumers must
// not treat source audit times as target object generations or mutable fields.
type ModelValue struct {
	ModelName             string   `json:"model_name"`
	DisplayName           string   `json:"display_name"`
	Description           string   `json:"description"`
	DescriptionEN         string   `json:"description_en"`
	Icon                  string   `json:"icon"`
	Tags                  string   `json:"tags"`
	Vendor                string   `json:"vendor"`
	BillingCurrency       string   `json:"billing_currency"`
	Endpoints             string   `json:"endpoints"`
	Status                int      `json:"status"`
	SyncOfficial          int      `json:"sync_official"`
	NameRule              int      `json:"name_rule"`
	ContextLength         int      `json:"context_length"`
	MaxOutputTokens       int      `json:"max_output_tokens"`
	KnowledgeCutoff       string   `json:"knowledge_cutoff"`
	ReleaseDate           string   `json:"release_date"`
	InputModalities       []string `json:"input_modalities"`
	OutputModalities      []string `json:"output_modalities"`
	Capabilities          []string `json:"capabilities"`
	MetadataSource        string   `json:"metadata_source"`
	MetadataVerifiedAt    string   `json:"metadata_verified_at"`
	MarketplaceEnabled    bool     `json:"marketplace_enabled"`
	DisplayOrder          int      `json:"display_order"`
	SupportedParameters   []string `json:"supported_parameters"`
	SupportedResolutions  []string `json:"supported_resolutions"`
	SupportedAspectRatios []string `json:"supported_aspect_ratios"`
	MaxInputImages        int      `json:"max_input_images"`
	OutputFormats         []string `json:"output_formats"`
	MinDuration           int      `json:"min_duration"`
	MaxDuration           int      `json:"max_duration"`
	ReferenceModalities   []string `json:"reference_modalities"`
	CreatedTime           int64    `json:"created_time"`
	UpdatedTime           int64    `json:"updated_time"`
}

// PriceOption describes only a supported persisted field, not its runtime default.
type PriceOption struct {
	Kind     string
	Map      bool
	Currency string
	Unit     string
}

// PriceOptions derives the exact special-price field whitelist from the
// setting definitions. No prefix grants permission to export a new field.
func PriceOptions() map[string]PriceOption {
	options := make(map[string]PriceOption)
	for _, name := range []string{"AudioCompletionRatio", "AudioRatio", "CacheRatio", "CompletionRatio", "CreateCacheRatio", "ImageRatio", "ModelPrice", "ModelRatio", "billing_setting.billing_expr", "billing_setting.billing_mode"} {
		unit := "legacy_ratio"
		switch name {
		case "ModelPrice":
			unit = "request"
		case "billing_setting.billing_expr":
			unit = "expression"
		case "billing_setting.billing_mode":
			unit = "mode"
		}
		options[name] = PriceOption{Kind: KindModelPrice, Map: true, Unit: unit}
	}
	options[billing_setting.PluginBillingExprOption] = PriceOption{Kind: KindPluginPrice, Map: true, Unit: "task_expression"}
	options[operation_setting.ToolPriceOptionKey] = PriceOption{Kind: KindToolPrice, Map: true, Currency: "USD", Unit: "thousand_calls"}
	for _, group := range []struct {
		name                 string
		definition           reflect.Type
		kind, currency, unit string
	}{
		{"molii_grok_price", reflect.TypeFor[ratio_setting.MoliiGrokPriceSetting](), KindSpecialPrice, "CNY", "image"},
		{"starai_video_price", reflect.TypeFor[ratio_setting.StarAIVideoPriceSetting](), KindSpecialPrice, "CNY", "million_tokens"},
		{"molii_grok_tool_price", reflect.TypeFor[operation_setting.MoliiGrokToolPriceSetting](), KindToolPrice, "CNY", "thousand_calls"},
		{"task_pricing_setting", reflect.TypeFor[task_pricing_setting.TaskPricingSetting](), KindSpecialPrice, "", "multiplier"},
	} {
		for i := range group.definition.NumField() {
			field := group.definition.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" {
				continue
			}
			unit := group.unit
			if group.name == "molii_grok_price" && strings.HasPrefix(name, "video_") && !strings.HasSuffix(name, "image_input") {
				unit = "second"
			}
			if group.name == "molii_grok_tool_price" && name == "image_generation" {
				unit = "image"
			}
			options[group.name+"."+name] = PriceOption{Kind: group.kind, Map: field.Type.Kind() == reflect.Map, Currency: group.currency, Unit: unit}
		}
	}
	return options
}

// IsPricingNamespace is a detection boundary only: unknown keys inside these
// namespaces must block a complete export, never acquire whitelist permission.
func IsPricingNamespace(key string) bool {
	for _, namespace := range []string{"billing_setting", "molii_grok_price", "molii_grok_tool_price", "starai_video_price", "task_pricing_setting", "tool_price_setting"} {
		if key == namespace || strings.HasPrefix(key, namespace+".") {
			return true
		}
	}
	return false
}

func Kinds() []string {
	return []string{KindVendor, KindModel, KindModelPrice, KindPluginPrice, KindSpecialPrice, KindToolPrice}
}

// RequiredCapabilities includes the embedded calendar's actual day data, not
// just its schema number, so equal schema versions cannot hide different fees.
func RequiredCapabilities() map[string]string {
	metadata, _ := billingexpr.CalendarMetadataForCountry("CN")
	days := make([]string, 0)
	for _, year := range metadata.SupportedYears {
		for day := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC); day.Year() == year; day = day.AddDate(0, 0, 1) {
			classification, available := billingexpr.ClassifyCalendarDay("CN", day)
			if available {
				days = append(days, day.Format("2006-01-02")+":"+string(classification))
			}
		}
	}
	encoded, _ := common.Marshal(struct {
		Metadata billingexpr.CalendarMetadata `json:"metadata"`
		Days     []string                     `json:"days"`
	}{metadata, days})
	return map[string]string{"catalog_scope": "managed-leaves.v1", "billing_engine": "billingexpr.v1", "plugin_contract": "task-api.v1.usage-profiles", "calendar_cn": fmt.Sprintf("sha256:%x", sha256.Sum256(encoded))}
}

// EntryID is the unambiguous identity used by resolution choices and baselines.
func EntryID(entry Entry) string {
	if entry.Kind != KindModel && entry.Kind != KindVendor {
		if key, err := DecodePriceKey(entry.Key); err == nil {
			if canonical, err := EncodePriceKey(key); err == nil {
				entry.Key = canonical
			}
		}
	}
	encoded, _ := common.Marshal([2]string{entry.Kind, entry.Key})
	return string(encoded)
}

func PointerSegment(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func EncodePriceKey(key PriceKey) (string, error) {
	encoded, err := common.Marshal(key)
	if err != nil {
		return "", err
	}
	return CanonicalJSON(string(encoded))
}

func DecodePriceKey(raw string) (PriceKey, error) {
	var key PriceKey
	if err := decodeFields(raw, &key, false); err != nil {
		return key, err
	}
	if key.Option == "" {
		return key, fmt.Errorf("price option is required")
	}
	return key, nil
}

// CanonicalJSON sorts object keys recursively and retains numeric precision.
// It does not reinterpret or rewrite JSON strings, including billing expressions.
func CanonicalJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	var value common.RawMessage
	if err := common.UnmarshalJsonStr(raw, &value); err != nil {
		return "", fmt.Errorf("invalid JSON")
	}
	switch common.GetJsonType(value) {
	case "object":
		var fields map[string]common.RawMessage
		if err := common.Unmarshal(value, &fields); err != nil {
			return "", err
		}
		names := slices.Sorted(maps.Keys(fields))
		parts := make([]string, 0, len(names))
		for _, name := range names {
			encoded, _ := common.Marshal(name)
			canonical, err := CanonicalJSON(string(fields[name]))
			if err != nil {
				return "", err
			}
			parts = append(parts, string(encoded)+":"+canonical)
		}
		return "{" + strings.Join(parts, ",") + "}", nil
	case "array":
		var values []common.RawMessage
		if err := common.Unmarshal(value, &values); err != nil {
			return "", err
		}
		parts := make([]string, len(values))
		for i := range values {
			canonical, err := CanonicalJSON(string(values[i]))
			if err != nil {
				return "", err
			}
			parts[i] = canonical
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	case "string":
		var text string
		if err := common.Unmarshal(value, &text); err != nil {
			return "", err
		}
		encoded, err := common.Marshal(text)
		return string(encoded), err
	case "number":
		// Normalize exact decimal notation without a float64 round trip or
		// expanding attacker-supplied exponents into enormous strings.
		number, negative := strings.CutPrefix(raw, "-")
		exponent := int64(0)
		if index := strings.IndexAny(number, "eE"); index >= 0 {
			parsed, err := strconv.ParseInt(number[index+1:], 10, 32)
			if err != nil {
				return "", fmt.Errorf("JSON number exponent is out of range")
			}
			exponent = parsed
			number = number[:index]
		}
		whole, fraction, _ := strings.Cut(number, ".")
		digits := strings.TrimLeft(whole+fraction, "0")
		if digits == "" {
			return "0", nil
		}
		exponent -= int64(len(fraction))
		trimmed := strings.TrimRight(digits, "0")
		exponent += int64(len(digits) - len(trimmed))
		digits = trimmed
		// JSON decoding into persisted Go integer fields requires plain integer
		// notation. Keep all int64/uint64-sized integers plain, with bounded work.
		if exponent > 0 && int64(len(digits))+exponent <= 20 {
			digits += strings.Repeat("0", int(exponent))
			exponent = 0
		}
		if negative {
			digits = "-" + digits
		}
		if exponent != 0 {
			digits += "e" + strconv.FormatInt(exponent, 10)
		}
		return digits, nil
	default:
		return raw, nil
	}
}

// decodeFields rejects fields which an older node cannot understand. All
// persisted metadata fields are present in v2, even when their value is zero.
func decodeFields(raw string, destination any, requireAll bool) error {
	var fields map[string]common.RawMessage
	if err := common.UnmarshalJsonStr(raw, &fields); err != nil || fields == nil {
		return fmt.Errorf("expected JSON object")
	}
	t := reflect.TypeOf(destination).Elem()
	for i := range t.NumField() {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		value, exists := fields[name]
		if requireAll && !exists {
			return fmt.Errorf("missing field %q", name)
		}
		if exists && string(value) == "null" && field.Type.Kind() != reflect.Slice {
			return fmt.Errorf("null field %q", name)
		}
		delete(fields, name)
	}
	if len(fields) > 0 {
		return fmt.Errorf("unknown JSON fields")
	}
	if err := common.UnmarshalJsonStr(raw, destination); err != nil {
		return fmt.Errorf("invalid field type")
	}
	return nil
}

func ValidateSnapshot(snapshot Snapshot) error {
	return validateSnapshot(snapshot, ValidatePriceValue)
}

// ValidateSnapshotStructure checks the wire shape, identities and digest only.
// It never compiles or evaluates expressions and cannot authorize application.
// Attested transactions must additionally match their staged full validation.
func ValidateSnapshotStructure(snapshot Snapshot) error {
	return validateSnapshot(snapshot, ValidatePriceValueStructure)
}

func validateSnapshot(snapshot Snapshot, validatePrice func(string, string) error) error {
	if snapshot.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported snapshot schema version %d", snapshot.SchemaVersion)
	}
	if strings.TrimSpace(snapshot.SourceID) == "" || !snapshot.Complete {
		return fmt.Errorf("complete snapshot and source identity are required")
	}
	if !maps.Equal(snapshot.Capabilities, RequiredCapabilities()) {
		return fmt.Errorf("incompatible catalog capabilities; upgrade source and target")
	}
	counts := make(map[string]int)
	for _, kind := range Kinds() {
		counts[kind] = 0
	}
	seen := make(map[string]bool)
	vendors := make(map[string]VendorValue)
	models := make(map[string]ModelValue)
	for _, entry := range snapshot.Entries {
		if _, known := counts[entry.Kind]; !known {
			return fmt.Errorf("unknown entry kind %q", entry.Kind)
		}
		if strings.TrimSpace(entry.Key) == "" {
			return fmt.Errorf("empty entry key")
		}
		id := EntryID(entry)
		if seen[id] {
			return fmt.Errorf("duplicate catalog identity")
		}
		seen[id] = true
		counts[entry.Kind]++
		switch entry.Kind {
		case KindVendor:
			var value VendorValue
			if err := decodeFields(entry.Value, &value, true); err != nil {
				return fmt.Errorf("vendor %q: %w", entry.Key, err)
			}
			if value.Name != entry.Key || (value.Status != 0 && value.Status != 1) {
				return fmt.Errorf("invalid vendor %q", entry.Key)
			}
			vendors[entry.Key] = value
		case KindModel:
			var value ModelValue
			if err := decodeFields(entry.Value, &value, true); err != nil {
				return fmt.Errorf("model %q: %w", entry.Key, err)
			}
			if value.ModelName != entry.Key || (value.BillingCurrency != "USD" && value.BillingCurrency != "CNY") || value.NameRule < 0 || value.NameRule > 3 || (value.Status != 0 && value.Status != 1) || (value.SyncOfficial != 0 && value.SyncOfficial != 1) {
				return fmt.Errorf("invalid model %q metadata or currency", entry.Key)
			}
			models[entry.Key] = value
		}
	}
	if !maps.Equal(counts, snapshot.Coverage) {
		return fmt.Errorf("incomplete catalog coverage counts")
	}
	for name, value := range models {
		if value.Vendor != "" {
			if _, exists := vendors[value.Vendor]; !exists {
				return fmt.Errorf("model %q references missing vendor", name)
			}
		}
	}
	options := PriceOptions()
	for _, entry := range snapshot.Entries {
		if entry.Kind == KindVendor || entry.Kind == KindModel {
			continue
		}
		key, err := DecodePriceKey(entry.Key)
		if err != nil {
			return err
		}
		spec, known := options[key.Option]
		if !known || spec.Kind != entry.Kind {
			return fmt.Errorf("unknown price field %q", key.Option)
		}
		var price PriceValue
		if err := decodeFields(entry.Value, &price, true); err != nil {
			return fmt.Errorf("price %q: %w", key.Option, err)
		}
		if price.Unit != spec.Unit {
			return fmt.Errorf("price %q has incompatible unit", key.Option)
		}
		currency := spec.Currency
		if entry.Kind == KindModelPrice || entry.Kind == KindPluginPrice {
			metadata, exists := models[key.Model]
			if !exists || metadata.NameRule != 0 {
				return fmt.Errorf("price %q model %q currency requires exact metadata", key.Option, key.Model)
			}
			currency = metadata.BillingCurrency
			leaf := key.Model
			if entry.Kind == KindPluginPrice {
				plugin, model, ok := billing_setting.SplitPluginBillingExprKey(key.Plugin + "::" + key.Model)
				if !ok || plugin != key.Plugin || model != key.Model {
					return fmt.Errorf("invalid plugin price identity")
				}
				leaf = key.Plugin + "::" + key.Model
			} else if key.Plugin != "" {
				return fmt.Errorf("model price cannot have plugin identity")
			}
			if key.Path != "/"+PointerSegment(leaf) {
				return fmt.Errorf("invalid model price path")
			}
		} else {
			if key.Model != "" || key.Plugin != "" {
				return fmt.Errorf("unexpected model or plugin price identity")
			}
			if spec.Map {
				segment, ok := strings.CutPrefix(key.Path, "/")
				if !ok || segment == "" || strings.Contains(segment, "/") {
					return fmt.Errorf("invalid leaf price path")
				}
				unescaped := strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
				if PointerSegment(unescaped) != segment {
					return fmt.Errorf("invalid JSON Pointer escaping")
				}
			} else if key.Path != "" {
				return fmt.Errorf("scalar price cannot have a leaf path")
			}
		}
		if price.BillingCurrency != currency {
			return fmt.Errorf("price %q currency mismatch", key.Option)
		}
		if err := validatePrice(key.Option, price.Value); err != nil {
			return err
		}
	}
	if snapshot.Digest == "" {
		return fmt.Errorf("snapshot digest is required")
	}
	digest, err := SnapshotDigest(snapshot)
	if err != nil {
		return err
	}
	if digest != snapshot.Digest {
		return fmt.Errorf("snapshot content digest mismatch")
	}
	return nil
}

// ValidatePriceValue reuses the billing compiler for expressions. Plugin
// binding/schema smoke tests still belong to the target's complete draft.
func ValidatePriceValue(option, raw string) error {
	if err := ValidatePriceValueStructure(option, raw); err != nil {
		return err
	}
	if option != "billing_setting.billing_expr" && option != billing_setting.PluginBillingExprOption {
		return nil
	}
	var expression string
	_ = common.UnmarshalJsonStr(raw, &expression)
	if _, err := billingexpr.CompileFromCache(expression); err != nil {
		return fmt.Errorf("price %q has invalid expression", option)
	}
	if option == billing_setting.PluginBillingExprOption && billingexpr.UsesFixedPricing(expression) {
		return fmt.Errorf("plugin price cannot use fixed pricing")
	}
	if len(billingexpr.UsedUsageKeys(expression)) == 0 {
		if err := billing_setting.SmokeTestExpr(expression); err != nil {
			return fmt.Errorf("price %q failed expression validation", option)
		}
	}
	return nil
}

// ValidatePriceValueStructure performs no compilation or evaluation. Semantic
// expression validation is mandatory outside the fenced transaction.
func ValidatePriceValueStructure(option, raw string) error {
	if option == "billing_setting.billing_mode" {
		var mode string
		if err := common.UnmarshalJsonStr(raw, &mode); err != nil || (mode != "ratio" && mode != "tiered_expr") {
			return fmt.Errorf("price %q has invalid billing mode", option)
		}
		return nil
	}
	if option == "billing_setting.billing_expr" || option == billing_setting.PluginBillingExprOption {
		var expression string
		if err := common.UnmarshalJsonStr(raw, &expression); err != nil || strings.TrimSpace(expression) == "" {
			return fmt.Errorf("price %q requires an expression", option)
		}
		if prefix, _, found := strings.Cut(strings.TrimSpace(expression), ":"); found && strings.HasPrefix(prefix, "v") && prefix != "v1" {
			if _, err := strconv.ParseUint(prefix[1:], 10, 64); err == nil {
				return fmt.Errorf("price %q has unsupported expression version", option)
			}
		}
		return nil
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || common.GetJsonType(common.RawMessage(trimmed)) != "number" {
		return fmt.Errorf("price %q must be a finite non-negative number", option)
	}
	number, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) {
		return fmt.Errorf("price %q must be a finite non-negative number", option)
	}
	return nil
}

// SnapshotDigest excludes display time and the supplied digest, and includes
// source, coverage, compatibility declarations and every explicit leaf.
func SnapshotDigest(snapshot Snapshot) (string, error) {
	canonical, err := CanonicalSnapshotContent(snapshot)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(canonical))), nil
}

// CanonicalSnapshotContent is the exact content hashed by SnapshotDigest. It
// neither compiles nor evaluates expressions. Attestations reuse this encoding
// so entry order, equivalent numeric JSON and display time are not new drafts.
func CanonicalSnapshotContent(snapshot Snapshot) (string, error) {
	snapshot.ExportedAt = 0
	snapshot.Digest = ""
	snapshot.Entries = slices.Clone(snapshot.Entries)
	for i := range snapshot.Entries {
		entry := &snapshot.Entries[i]
		var err error
		entry.Value, err = CanonicalJSON(entry.Value)
		if err != nil {
			return "", err
		}
		if entry.Kind != KindModel && entry.Kind != KindVendor {
			key, err := DecodePriceKey(entry.Key)
			if err != nil {
				return "", err
			}
			entry.Key, err = EncodePriceKey(key)
			if err != nil {
				return "", err
			}
		}
		// PriceValue.Value is itself JSON text and also needs canonicalization.
		if entry.Kind == KindModelPrice || entry.Kind == KindPluginPrice || entry.Kind == KindSpecialPrice || entry.Kind == KindToolPrice {
			var price PriceValue
			if err := common.UnmarshalJsonStr(entry.Value, &price); err != nil {
				return "", err
			}
			price.Value, err = CanonicalJSON(price.Value)
			if err != nil {
				return "", err
			}
			encoded, err := common.Marshal(price)
			if err != nil {
				return "", err
			}
			entry.Value, err = CanonicalJSON(string(encoded))
			if err != nil {
				return "", err
			}
		}
	}
	slices.SortFunc(snapshot.Entries, func(a, b Entry) int {
		if compared := strings.Compare(a.Kind, b.Kind); compared != 0 {
			return compared
		}
		return strings.Compare(a.Key, b.Key)
	})
	encoded, err := common.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	canonical, err := CanonicalJSON(string(encoded))
	if err != nil {
		return "", err
	}
	return canonical, nil
}
