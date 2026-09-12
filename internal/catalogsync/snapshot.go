package catalogsync

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const SnapshotSchemaVersion = 1

var modelPricingOptionKeys = []string{
	"AudioCompletionRatio",
	"AudioRatio",
	"CacheRatio",
	"CompletionRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"ModelPrice",
	"ModelRatio",
	"billing_setting.billing_expr",
	"billing_setting.billing_mode",
}

var wholePricingOptionKeys = []string{
	"tool_price_setting.prices",
}

var pricingOptionPrefixes = []string{
	"molii_grok_price.",
	"molii_grok_tool_price.",
	"starai_video_price.",
	"task_pricing_setting.",
}

type Snapshot struct {
	SchemaVersion int               `json:"schema_version"`
	ExportedAt    string            `json:"exported_at"`
	ContentDigest string            `json:"content_digest,omitempty"`
	Vendors       []VendorRecord    `json:"vendors"`
	Models        []ModelRecord     `json:"models"`
	Options       map[string]string `json:"options"`
}

type VendorRecord struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Icon         string `json:"icon,omitempty"`
	Status       int    `json:"status"`
	DisplayOrder int    `json:"display_order"`
}

type ModelRecord struct {
	ModelName             string   `json:"model_name"`
	DisplayName           string   `json:"display_name,omitempty"`
	Description           string   `json:"description,omitempty"`
	DescriptionEN         string   `json:"description_en,omitempty"`
	Icon                  string   `json:"icon,omitempty"`
	Tags                  string   `json:"tags,omitempty"`
	Vendor                string   `json:"vendor,omitempty"`
	BillingCurrency       string   `json:"billing_currency"`
	Endpoints             string   `json:"endpoints,omitempty"`
	Status                int      `json:"status"`
	SyncOfficial          int      `json:"sync_official"`
	NameRule              int      `json:"name_rule"`
	ContextLength         int      `json:"context_length,omitempty"`
	MaxOutputTokens       int      `json:"max_output_tokens,omitempty"`
	KnowledgeCutoff       string   `json:"knowledge_cutoff,omitempty"`
	ReleaseDate           string   `json:"release_date,omitempty"`
	InputModalities       []string `json:"input_modalities,omitempty"`
	OutputModalities      []string `json:"output_modalities,omitempty"`
	Capabilities          []string `json:"capabilities,omitempty"`
	MetadataSource        string   `json:"metadata_source,omitempty"`
	MetadataVerifiedAt    string   `json:"metadata_verified_at,omitempty"`
	MarketplaceEnabled    bool     `json:"marketplace_enabled"`
	DisplayOrder          int      `json:"display_order"`
	SupportedParameters   []string `json:"supported_parameters,omitempty"`
	SupportedResolutions  []string `json:"supported_resolutions,omitempty"`
	SupportedAspectRatios []string `json:"supported_aspect_ratios,omitempty"`
	MaxInputImages        int      `json:"max_input_images,omitempty"`
	OutputFormats         []string `json:"output_formats,omitempty"`
	MinDuration           int      `json:"min_duration,omitempty"`
	MaxDuration           int      `json:"max_duration,omitempty"`
	ReferenceModalities   []string `json:"reference_modalities,omitempty"`
}

func Export(ctx context.Context, db *gorm.DB) (Snapshot, error) {
	var vendors []model.Vendor
	if err := db.WithContext(ctx).Order("display_order ASC, id ASC").Find(&vendors).Error; err != nil {
		return Snapshot{}, fmt.Errorf("load vendors: %w", err)
	}
	var models []model.Model
	if err := db.WithContext(ctx).Order("display_order ASC, id ASC").Find(&models).Error; err != nil {
		return Snapshot{}, fmt.Errorf("load models: %w", err)
	}
	var options []model.Option
	exactOptionKeys := append(slices.Clone(modelPricingOptionKeys), wholePricingOptionKeys...)
	optionQuery := db.WithContext(ctx).Where("key IN ?", exactOptionKeys)
	for _, prefix := range pricingOptionPrefixes {
		optionQuery = optionQuery.Or("key LIKE ?", prefix+"%")
	}
	if err := optionQuery.Find(&options).Error; err != nil {
		return Snapshot{}, fmt.Errorf("load pricing options: %w", err)
	}

	vendorNames := make(map[int]string, len(vendors))
	vendorRecords := make([]VendorRecord, 0, len(vendors))
	for _, vendor := range vendors {
		vendorNames[vendor.Id] = vendor.Name
		vendorRecords = append(vendorRecords, VendorRecord{
			Name: vendor.Name, Description: vendor.Description, Icon: vendor.Icon,
			Status: vendor.Status, DisplayOrder: vendor.DisplayOrder,
		})
	}

	modelRecords := make([]ModelRecord, 0, len(models))
	for _, entry := range models {
		vendorName := ""
		if entry.VendorID != 0 {
			var exists bool
			vendorName, exists = vendorNames[entry.VendorID]
			if !exists {
				return Snapshot{}, fmt.Errorf("model %q references missing vendor id %d", entry.ModelName, entry.VendorID)
			}
		}
		modelRecords = append(modelRecords, modelRecordFromModel(entry, vendorName))
	}

	pricingOptions := make(map[string]string)
	for _, option := range options {
		if isPricingOption(option.Key) {
			pricingOptions[option.Key] = option.Value
		}
	}

	snapshot := Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Vendors:       vendorRecords,
		Models:        modelRecords,
		Options:       pricingOptions,
	}
	if err := snapshot.NormalizeAndValidate(); err != nil {
		return Snapshot{}, err
	}
	snapshot.ContentDigest, _ = snapshot.contentDigest()
	return snapshot, nil
}

func modelRecordFromModel(entry model.Model, vendorName string) ModelRecord {
	return ModelRecord{
		ModelName: entry.ModelName, DisplayName: entry.DisplayName, Description: entry.Description,
		DescriptionEN: entry.DescriptionEN, Icon: entry.Icon, Tags: entry.Tags, Vendor: vendorName,
		BillingCurrency: entry.BillingCurrency, Endpoints: entry.Endpoints, Status: entry.Status,
		SyncOfficial: entry.SyncOfficial, NameRule: entry.NameRule, ContextLength: entry.ContextLength,
		MaxOutputTokens: entry.MaxOutputTokens, KnowledgeCutoff: entry.KnowledgeCutoff, ReleaseDate: entry.ReleaseDate,
		InputModalities: slices.Clone(entry.InputModalities), OutputModalities: slices.Clone(entry.OutputModalities),
		Capabilities: slices.Clone(entry.Capabilities), MetadataSource: entry.MetadataSource,
		MetadataVerifiedAt: entry.MetadataVerifiedAt, MarketplaceEnabled: entry.MarketplaceEnabled,
		DisplayOrder: entry.DisplayOrder, SupportedParameters: slices.Clone(entry.SupportedParameters),
		SupportedResolutions:  slices.Clone(entry.SupportedResolutions),
		SupportedAspectRatios: slices.Clone(entry.SupportedAspectRatios), MaxInputImages: entry.MaxInputImages,
		OutputFormats: slices.Clone(entry.OutputFormats), MinDuration: entry.MinDuration, MaxDuration: entry.MaxDuration,
		ReferenceModalities: slices.Clone(entry.ReferenceModalities),
	}
}

func isPricingOption(key string) bool {
	if slices.Contains(modelPricingOptionKeys, key) || slices.Contains(wholePricingOptionKeys, key) {
		return true
	}
	for _, prefix := range pricingOptionPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func (snapshot *Snapshot) NormalizeAndValidate() error {
	if snapshot.SchemaVersion != SnapshotSchemaVersion {
		return fmt.Errorf("unsupported snapshot schema version %d", snapshot.SchemaVersion)
	}
	if snapshot.Options == nil {
		snapshot.Options = make(map[string]string)
	}
	for _, key := range modelPricingOptionKeys {
		if _, exists := snapshot.Options[key]; !exists {
			snapshot.Options[key] = "{}"
		}
	}
	for _, key := range wholePricingOptionKeys {
		if _, exists := snapshot.Options[key]; !exists {
			snapshot.Options[key] = "{}"
		}
	}
	sort.Slice(snapshot.Vendors, func(i, j int) bool { return snapshot.Vendors[i].Name < snapshot.Vendors[j].Name })
	sort.Slice(snapshot.Models, func(i, j int) bool { return snapshot.Models[i].ModelName < snapshot.Models[j].ModelName })

	vendors := make(map[string]struct{}, len(snapshot.Vendors))
	for _, vendor := range snapshot.Vendors {
		if strings.TrimSpace(vendor.Name) == "" {
			return fmt.Errorf("vendor name is required")
		}
		if vendor.Status != 0 && vendor.Status != 1 {
			return fmt.Errorf("vendor %q has invalid status %d", vendor.Name, vendor.Status)
		}
		if _, exists := vendors[vendor.Name]; exists {
			return fmt.Errorf("duplicate vendor %q", vendor.Name)
		}
		vendors[vendor.Name] = struct{}{}
	}
	models := make(map[string]struct{}, len(snapshot.Models))
	for index := range snapshot.Models {
		entry := &snapshot.Models[index]
		normalizeModelSlices(entry)
		if strings.TrimSpace(entry.ModelName) == "" {
			return fmt.Errorf("model name is required")
		}
		if _, exists := models[entry.ModelName]; exists {
			return fmt.Errorf("duplicate model %q", entry.ModelName)
		}
		if entry.Status != 0 && entry.Status != 1 {
			return fmt.Errorf("model %q has invalid status %d", entry.ModelName, entry.Status)
		}
		if entry.SyncOfficial != 0 && entry.SyncOfficial != 1 {
			return fmt.Errorf("model %q has invalid sync_official %d", entry.ModelName, entry.SyncOfficial)
		}
		if entry.NameRule < model.NameRuleExact || entry.NameRule > model.NameRuleSuffix {
			return fmt.Errorf("model %q has invalid name_rule %d", entry.ModelName, entry.NameRule)
		}
		models[entry.ModelName] = struct{}{}
		if entry.Vendor != "" {
			if _, exists := vendors[entry.Vendor]; !exists {
				return fmt.Errorf("model %q references missing vendor %q", entry.ModelName, entry.Vendor)
			}
		}
		entry.BillingCurrency = strings.ToUpper(strings.TrimSpace(entry.BillingCurrency))
		if entry.BillingCurrency != "USD" && entry.BillingCurrency != "CNY" {
			return fmt.Errorf("model %q has invalid billing_currency %q", entry.ModelName, entry.BillingCurrency)
		}
	}
	for key, value := range snapshot.Options {
		if !isPricingOption(key) {
			return fmt.Errorf("option %q is outside catalog pricing scope", key)
		}
		if isModelPricingOption(key) || slices.Contains(wholePricingOptionKeys, key) {
			var entries map[string]any
			if err := common.UnmarshalJsonStr(value, &entries); err != nil || entries == nil {
				return fmt.Errorf("option %q must be a JSON object", key)
			}
		}
	}
	return nil
}

func normalizeModelSlices(entry *ModelRecord) {
	if entry.InputModalities == nil {
		entry.InputModalities = []string{}
	}
	if entry.OutputModalities == nil {
		entry.OutputModalities = []string{}
	}
	if entry.Capabilities == nil {
		entry.Capabilities = []string{}
	}
	if entry.SupportedParameters == nil {
		entry.SupportedParameters = []string{}
	}
	if entry.SupportedResolutions == nil {
		entry.SupportedResolutions = []string{}
	}
	if entry.SupportedAspectRatios == nil {
		entry.SupportedAspectRatios = []string{}
	}
	if entry.OutputFormats == nil {
		entry.OutputFormats = []string{}
	}
	if entry.ReferenceModalities == nil {
		entry.ReferenceModalities = []string{}
	}
}

func ReadSnapshot(reader io.Reader) (Snapshot, error) {
	var snapshot Snapshot
	if err := common.DecodeJson(reader, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	providedDigest := snapshot.ContentDigest
	if err := snapshot.NormalizeAndValidate(); err != nil {
		return Snapshot{}, err
	}
	digest, err := snapshot.contentDigest()
	if err != nil {
		return Snapshot{}, err
	}
	if providedDigest != "" && providedDigest != digest {
		return Snapshot{}, fmt.Errorf("snapshot content digest mismatch")
	}
	snapshot.ContentDigest = digest
	return snapshot, nil
}

func WriteSnapshot(writer io.Writer, snapshot Snapshot) error {
	if err := snapshot.NormalizeAndValidate(); err != nil {
		return err
	}
	digest, err := snapshot.contentDigest()
	if err != nil {
		return err
	}
	snapshot.ContentDigest = digest
	encoded, err := common.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	indented, err := common.IndentJson(encoded)
	if err != nil {
		return fmt.Errorf("format snapshot: %w", err)
	}
	if _, err := writer.Write(append(indented, '\n')); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

func (snapshot Snapshot) contentDigest() (string, error) {
	snapshot.ExportedAt = ""
	snapshot.ContentDigest = ""
	encoded, err := common.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode snapshot digest: %w", err)
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(encoded)), nil
}
