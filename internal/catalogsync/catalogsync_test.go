package catalogsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var catalogSyncTestDatabaseID atomic.Uint64

func openCatalogSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_CATALOG_SYNC_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_CATALOG_SYNC_POSTGRES_DSN is not configured")
	}
	prefix := fmt.Sprintf("catalog_sync_%d_%d_", os.Getpid(), catalogSyncTestDatabaseID.Add(1))
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix}})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.Vendor{}, &model.Model{}))
	t.Cleanup(func() {
		require.NoError(t, db.Migrator().DropTable(&model.Model{}, &model.Vendor{}, &model.Option{}))
	})
	return db
}

func TestExportIncludesCompleteCatalogAndOnlyPricingOptions(t *testing.T) {
	db := openCatalogSyncTestDB(t)
	vendor := model.Vendor{
		Name: "ByteDance", Description: "Volcengine models", Icon: "BytedanceColor", Status: 1, DisplayOrder: 3,
	}
	require.NoError(t, db.Create(&vendor).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "doubao-seedance-2-5-260628", DisplayName: "Seedance 2.5", Description: "视频生成",
		DescriptionEN: "Video generation", Icon: "BytedanceColor", Tags: "video,seedance", VendorID: vendor.Id,
		BillingCurrency: "CNY", Endpoints: `["video"]`, Status: 1, SyncOfficial: 0, NameRule: model.NameRuleExact,
		ContextLength: 128000, MaxOutputTokens: 4096, KnowledgeCutoff: "2026-06", ReleaseDate: "2026-06-28",
		InputModalities: []string{"text", "image", "video", "audio"}, OutputModalities: []string{"video"},
		Capabilities: []string{"video_generation"}, MetadataSource: "manual", MetadataVerifiedAt: "2026-09-12",
		MarketplaceEnabled: true, DisplayOrder: 4,
		SupportedParameters: []string{"prompt", "content", "duration"}, SupportedResolutions: []string{"480p", "720p", "1080p"},
		SupportedAspectRatios: []string{"16:9", "9:16"}, MaxInputImages: 9, OutputFormats: []string{"mp4"},
		MinDuration: 4, MaxDuration: 15, ReferenceModalities: []string{"image", "video", "audio"},
	}).Error)

	options := []model.Option{
		{Key: "ModelRatio", Value: `{"doubao-seedance-2-5-260628":70}`},
		{Key: "billing_setting.billing_expr", Value: `{"doubao-seedance-2-5-260628":"u(\"total_tokens\") * 70"}`},
		{Key: "starai_video_price.seedance25_720p", Value: "70"},
		{Key: "molii_grok_price.video_720p", Value: "0.07"},
		{Key: "molii_grok_tool_price.web_search", Value: "5"},
		{Key: "tool_price_setting.prices", Value: `{"custom_search":3}`},
		{Key: "task_pricing_setting.sora_size_ratio", Value: `{"1792x1024":1.666667}`},
		{Key: "GroupRatio", Value: `{"ByteDance":1}`},
		{Key: "group_ratio_setting.group_metadata", Value: `{"ByteDance":{"icon":"BytedanceColor"}}`},
		{Key: "SystemName", Value: "development-only"},
	}
	require.NoError(t, db.Create(&options).Error)

	snapshot, err := Export(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, SnapshotSchemaVersion, snapshot.SchemaVersion)
	require.Len(t, snapshot.Vendors, 1)
	assert.Equal(t, "ByteDance", snapshot.Vendors[0].Name)
	assert.Equal(t, "BytedanceColor", snapshot.Vendors[0].Icon)
	require.Len(t, snapshot.Models, 1)
	assert.Equal(t, "ByteDance", snapshot.Models[0].Vendor)
	assert.Equal(t, "CNY", snapshot.Models[0].BillingCurrency)
	assert.Equal(t, []string{"prompt", "content", "duration"}, snapshot.Models[0].SupportedParameters)
	assert.Equal(t, 15, snapshot.Models[0].MaxDuration)
	assert.Equal(t, []string{"image", "video", "audio"}, snapshot.Models[0].ReferenceModalities)

	assert.Equal(t, `{"doubao-seedance-2-5-260628":70}`, snapshot.Options["ModelRatio"])
	assert.Contains(t, snapshot.Options, "billing_setting.billing_expr")
	assert.Contains(t, snapshot.Options, "starai_video_price.seedance25_720p")
	assert.Contains(t, snapshot.Options, "molii_grok_price.video_720p")
	assert.Contains(t, snapshot.Options, "molii_grok_tool_price.web_search")
	assert.JSONEq(t, `{"custom_search":3}`, snapshot.Options["tool_price_setting.prices"])
	assert.Contains(t, snapshot.Options, "task_pricing_setting.sora_size_ratio")
	assert.NotContains(t, snapshot.Options, "GroupRatio")
	assert.NotContains(t, snapshot.Options, "group_ratio_setting.group_metadata")
	assert.NotContains(t, snapshot.Options, "SystemName")
	require.NotEmpty(t, snapshot.ContentDigest)
}

func TestExportRejectsModelWithMissingVendor(t *testing.T) {
	db := openCatalogSyncTestDB(t)
	require.NoError(t, db.Create(&model.Model{ModelName: "orphan", VendorID: 999, BillingCurrency: "USD"}).Error)

	_, err := Export(context.Background(), db)
	require.ErrorContains(t, err, "missing vendor")
}

func TestBuildPlanPreservesTargetOnlyCatalogAndReconcilesManagedPricing(t *testing.T) {
	db := openCatalogSyncTestDB(t)
	targetVendor := model.Vendor{Name: "ByteDance", Description: "old", Icon: "old", Status: 1, DisplayOrder: 8}
	require.NoError(t, db.Create(&targetVendor).Error)
	targetOnlyVendor := model.Vendor{Name: "TargetOnly", Status: 1, DisplayOrder: 9}
	require.NoError(t, db.Create(&targetOnlyVendor).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "shared-model", DisplayName: "Old", VendorID: targetVendor.Id, BillingCurrency: "USD", Status: 1,
	}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "target-only-model", DisplayName: "Keep", VendorID: targetOnlyVendor.Id, BillingCurrency: "CNY", Status: 1,
	}).Error)
	require.NoError(t, db.Create(&[]model.Option{
		{Key: "ModelRatio", Value: `{"shared-model":1,"target-only-model":3}`},
		{Key: "ModelPrice", Value: `{"shared-model":9,"target-only-model":5}`},
		{Key: "starai_video_price.stale", Value: "999"},
	}).Error)

	snapshot := Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		Vendors:       []VendorRecord{{Name: "ByteDance", Description: "current", Icon: "BytedanceColor", Status: 1, DisplayOrder: 1}},
		Models: []ModelRecord{{
			ModelName: "shared-model", DisplayName: "Current", Vendor: "ByteDance", BillingCurrency: "CNY",
			Status: 1, MarketplaceEnabled: true, SupportedParameters: []string{"prompt"},
		}},
		Options: map[string]string{
			"ModelRatio":                           `{"shared-model":2}`,
			"ModelPrice":                           `{}`,
			"starai_video_price.standard_720p":     "46",
			"task_pricing_setting.sora_size_ratio": `{"1792x1024":1.666667}`,
		},
	}
	require.NoError(t, snapshot.NormalizeAndValidate())

	plan, err := BuildPlan(context.Background(), db, snapshot)
	require.NoError(t, err)
	require.NotEmpty(t, plan.ConfirmationDigest)
	require.Equal(t, 1, plan.Summary.VendorsUpdated)
	require.Equal(t, 1, plan.Summary.ModelsUpdated)
	assert.Zero(t, plan.Summary.VendorsDeleted)
	assert.Zero(t, plan.Summary.ModelsDeleted)

	vendorChange := plan.Vendors[0]
	assert.Equal(t, ChangeUpdate, vendorChange.Action)
	assert.Equal(t, "current", vendorChange.After.Description)
	modelChange := plan.Models[0]
	assert.Equal(t, ChangeUpdate, modelChange.Action)
	assert.Equal(t, "ByteDance", modelChange.After.Vendor)
	assert.Equal(t, "CNY", modelChange.After.BillingCurrency)

	changes := make(map[string]OptionChange, len(plan.Options))
	for _, change := range plan.Options {
		changes[change.Key] = change
	}
	assert.JSONEq(t, `{"shared-model":2,"target-only-model":3}`, *changes["ModelRatio"].After)
	assert.JSONEq(t, `{"target-only-model":5}`, *changes["ModelPrice"].After)
	assert.Equal(t, ChangeDelete, changes["starai_video_price.stale"].Action)
	assert.Nil(t, changes["starai_video_price.stale"].After)
	assert.Equal(t, "46", *changes["starai_video_price.standard_720p"].After)

	var vendors []model.Vendor
	require.NoError(t, db.Order("name").Find(&vendors).Error)
	require.Len(t, vendors, 2)
	assert.True(t, slices.ContainsFunc(vendors, func(v model.Vendor) bool { return v.Name == "TargetOnly" }))
	var untouched model.Model
	require.NoError(t, db.Where("model_name = ?", "target-only-model").First(&untouched).Error)
	assert.Equal(t, "Keep", untouched.DisplayName)
}

func TestBuildPlanDigestIsDeterministic(t *testing.T) {
	db := openCatalogSyncTestDB(t)
	snapshot := Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		Vendors:       []VendorRecord{{Name: "OpenAI", Status: 1}},
		Models:        []ModelRecord{{ModelName: "gpt-test", Vendor: "OpenAI", BillingCurrency: "USD", Status: 1}},
	}
	require.NoError(t, snapshot.NormalizeAndValidate())

	first, err := BuildPlan(context.Background(), db, snapshot)
	require.NoError(t, err)
	second, err := BuildPlan(context.Background(), db, snapshot)
	require.NoError(t, err)
	assert.Equal(t, first.ConfirmationDigest, second.ConfirmationDigest)
}

func TestApplyRequiresCurrentConfirmationAndCreatesBackupBeforeMutation(t *testing.T) {
	db := openCatalogSyncTestDB(t)
	vendor := model.Vendor{Name: "OpenAI", Description: "old", Status: 1}
	require.NoError(t, db.Create(&vendor).Error)
	targetOnlyVendor := model.Vendor{Name: "TargetOnly", Status: 1}
	require.NoError(t, db.Create(&targetOnlyVendor).Error)
	entry := model.Model{ModelName: "gpt-test", DisplayName: "Old", VendorID: vendor.Id, BillingCurrency: "USD", Status: 1}
	require.NoError(t, db.Create(&entry).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "target-only", DisplayName: "Keep", VendorID: targetOnlyVendor.Id, BillingCurrency: "CNY", Status: 1,
	}).Error)
	require.NoError(t, db.Create(&model.Option{Key: "ModelRatio", Value: `{"gpt-test":1,"target-only":7}`}).Error)

	snapshot := Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		Vendors:       []VendorRecord{{Name: "OpenAI", Description: "new", Icon: "OpenAI", Status: 1}},
		Models: []ModelRecord{
			{ModelName: "gpt-test", DisplayName: "Updated", Vendor: "OpenAI", BillingCurrency: "USD", Status: 1},
			{ModelName: "gpt-new", DisplayName: "New", Vendor: "OpenAI", BillingCurrency: "USD", Status: 1},
		},
		Options: map[string]string{"ModelRatio": `{"gpt-test":2,"gpt-new":3}`},
	}
	require.NoError(t, snapshot.NormalizeAndValidate())
	plan, err := BuildPlan(context.Background(), db, snapshot)
	require.NoError(t, err)

	var rejectedBackup bytes.Buffer
	_, err = Apply(context.Background(), db, snapshot, "sha256:wrong", &rejectedBackup)
	require.ErrorContains(t, err, "confirmation digest")
	assert.Zero(t, rejectedBackup.Len())
	var unchanged model.Model
	require.NoError(t, db.Where("model_name = ?", "gpt-test").First(&unchanged).Error)
	assert.Equal(t, "Old", unchanged.DisplayName)

	var backup bytes.Buffer
	result, err := Apply(context.Background(), db, snapshot, plan.ConfirmationDigest, &backup)
	require.NoError(t, err)
	assert.Equal(t, plan.ConfirmationDigest, result.ConfirmationDigest)
	assert.NotEmpty(t, result.BackupDigest)
	require.NotZero(t, backup.Len())

	backupSnapshot, err := ReadSnapshot(bytes.NewReader(backup.Bytes()))
	require.NoError(t, err)
	require.Len(t, backupSnapshot.Models, 2)
	assert.True(t, slices.ContainsFunc(backupSnapshot.Models, func(entry ModelRecord) bool {
		return entry.ModelName == "gpt-test" && entry.DisplayName == "Old"
	}))

	var updated model.Model
	require.NoError(t, db.Where("model_name = ?", "gpt-test").First(&updated).Error)
	assert.Equal(t, entry.Id, updated.Id)
	assert.Equal(t, vendor.Id, updated.VendorID)
	assert.Equal(t, "Updated", updated.DisplayName)
	var created model.Model
	require.NoError(t, db.Where("model_name = ?", "gpt-new").First(&created).Error)
	assert.Equal(t, vendor.Id, created.VendorID)
	var ratio model.Option
	require.NoError(t, db.Where("key = ?", "ModelRatio").First(&ratio).Error)
	assert.JSONEq(t, `{"gpt-test":2,"gpt-new":3,"target-only":7}`, ratio.Value)
	var targetOnly model.Model
	require.NoError(t, db.Where("model_name = ?", "target-only").First(&targetOnly).Error)
	assert.Equal(t, "Keep", targetOnly.DisplayName)

	secondPlan, err := BuildPlan(context.Background(), db, snapshot)
	require.NoError(t, err)
	assert.Equal(t, PlanSummary{}, secondPlan.Summary)
	var secondBackup bytes.Buffer
	_, err = Apply(context.Background(), db, snapshot, secondPlan.ConfirmationDigest, &secondBackup)
	require.NoError(t, err)
}

type alwaysFailWriter struct{}

func (alwaysFailWriter) Write([]byte) (int, error) {
	return 0, errors.New("backup unavailable")
}

type syncFailWriter struct {
	bytes.Buffer
}

func (syncFailWriter) Sync() error {
	return errors.New("backup fsync unavailable")
}

func TestApplyRollsBackWhenBackupCannotBeWritten(t *testing.T) {
	db := openCatalogSyncTestDB(t)
	snapshot := Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		Vendors:       []VendorRecord{{Name: "NewVendor", Status: 1}},
		Models:        []ModelRecord{{ModelName: "new-model", Vendor: "NewVendor", BillingCurrency: "USD", Status: 1}},
	}
	require.NoError(t, snapshot.NormalizeAndValidate())
	plan, err := BuildPlan(context.Background(), db, snapshot)
	require.NoError(t, err)

	_, err = Apply(context.Background(), db, snapshot, plan.ConfirmationDigest, alwaysFailWriter{})
	require.ErrorContains(t, err, "write backup")
	var vendorCount int64
	require.NoError(t, db.Model(&model.Vendor{}).Count(&vendorCount).Error)
	assert.Zero(t, vendorCount)
	var modelCount int64
	require.NoError(t, db.Model(&model.Model{}).Count(&modelCount).Error)
	assert.Zero(t, modelCount)
}

func TestApplyRollsBackWhenBackupCannotBeSynced(t *testing.T) {
	db := openCatalogSyncTestDB(t)
	snapshot := Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		Vendors:       []VendorRecord{{Name: "NewVendor", Status: 1}},
		Models:        []ModelRecord{{ModelName: "new-model", Vendor: "NewVendor", BillingCurrency: "USD", Status: 1}},
	}
	require.NoError(t, snapshot.NormalizeAndValidate())
	plan, err := BuildPlan(context.Background(), db, snapshot)
	require.NoError(t, err)

	_, err = Apply(context.Background(), db, snapshot, plan.ConfirmationDigest, &syncFailWriter{})
	require.ErrorContains(t, err, "sync backup")
	var vendorCount int64
	require.NoError(t, db.Model(&model.Vendor{}).Count(&vendorCount).Error)
	assert.Zero(t, vendorCount)
	var modelCount int64
	require.NoError(t, db.Model(&model.Model{}).Count(&modelCount).Error)
	assert.Zero(t, modelCount)
}

// This catches exports that only traverse metadata, merge plugin identities
// into model names, invent defaults, or leak unrelated options.
func TestManagedExportCompletePricingContract(t *testing.T) {
	db := openCatalogSyncTestDB(t)
	vendor := model.Vendor{Name: "Vendor./~", Status: 1, DisplayOrder: 3}
	require.NoError(t, db.Create(&vendor).Error)
	metadata := model.Model{
		ModelName: "model.a/~::b", VendorID: vendor.Id, BillingCurrency: "CNY", Status: 1,
		DisplayName: "Display", Description: "说明", DescriptionEN: "Description", Icon: "icon", Tags: "video",
		Endpoints: `["video"]`, SyncOfficial: 1, NameRule: model.NameRuleExact,
		ContextLength: 123, MaxOutputTokens: 12, KnowledgeCutoff: "2026-01", ReleaseDate: "2026-01-01",
		InputModalities: []string{"text", "image"}, OutputModalities: []string{"video"}, Capabilities: []string{"video_generation"},
		MetadataSource: "manual", MetadataVerifiedAt: "2026-10-08", MarketplaceEnabled: true, DisplayOrder: 4,
		SupportedParameters: []string{"prompt"}, SupportedResolutions: []string{"720p"}, SupportedAspectRatios: []string{"16:9"},
		MaxInputImages: 2, OutputFormats: []string{"mp4"}, MinDuration: 1, MaxDuration: 15, ReferenceModalities: []string{"image"},
		CreatedTime: 1234, UpdatedTime: 5678,
	}
	require.NoError(t, db.Create(&metadata).Error)
	options := []model.Option{
		{Key: "ModelPrice", Value: `{"model.a/~::b":0}`},
		{Key: "billing_setting.billing_mode", Value: `{"model.a/~::b":"ratio"}`},
		{Key: "billing_setting.plugin_billing_expr", Value: `{"provider-a::model.a/~::b":"tier(\"base\", u(\"seconds\") * 0.5)"}`},
		{Key: "starai_video_price.seedance_25_720p", Value: `0`},
		{Key: "molii_grok_price.video_720p", Value: `0.07`},
		{Key: "molii_grok_tool_price.web_search", Value: `5`},
		{Key: "tool_price_setting.prices", Value: `{"search.a/~:model.b":0,"search":{"model.a/~":3}}`},
		{Key: "task_pricing_setting.sora_size_ratio", Value: `{"1792x1024":1.666667}`},
		{Key: "GroupRatio", Value: `{"vip":0.5}`},
		{Key: "USDExchangeRate", Value: `7`},
		{Key: "SystemName", Value: "private-brand"},
		{Key: "catalog_sync.token", Value: "do-not-export-secret"},
	}
	// The current tool setting is a flat number map; reject nested invalid
	// payloads before exporting, then exercise leaf identity with valid prices.
	require.NoError(t, db.Create(&options).Error)
	_, err := ExportManaged(context.Background(), db, "dev")
	require.ErrorContains(t, err, "tool_price_setting.prices")
	require.NoError(t, db.Model(&model.Option{}).Where("key = ?", "tool_price_setting.prices").Update("value", `{"search.a/~:model.b":0,"search:model.a/~":3}`).Error)
	snapshot, err := ExportManaged(context.Background(), db, "dev")
	require.NoError(t, err)
	require.NoError(t, catalogmanifest.ValidateSnapshot(snapshot))
	assert.Equal(t, 2, snapshot.SchemaVersion)
	assert.True(t, snapshot.Complete)
	assert.Equal(t, "dev", snapshot.SourceID)
	assert.Equal(t, map[string]int{"vendor": 1, "model": 1, "model_price": 2, "plugin_price": 1, "special_price": 3, "tool_price": 3}, snapshot.Coverage)
	entries := make(map[string]catalogmanifest.Entry)
	for _, entry := range snapshot.Entries {
		entries[entry.Kind+":"+entry.Key] = entry
	}
	assert.Contains(t, entries, "model:model.a/~::b")
	assert.NotContains(t, entries, "model:provider-a::model.a/~::b")
	var gotMetadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(entries["model:model.a/~::b"].Value, &gotMetadata))
	assert.Equal(t, "Vendor./~", gotMetadata["vendor"])
	assert.Equal(t, "CNY", gotMetadata["billing_currency"])
	assert.EqualValues(t, 1234, gotMetadata["created_time"])
	assert.EqualValues(t, 5678, gotMetadata["updated_time"])
	assert.Equal(t, []any{"720p"}, gotMetadata["supported_resolutions"])
	assert.NotContains(t, gotMetadata, "id")
	assert.NotContains(t, gotMetadata, "vendor_id")
	assert.JSONEq(t, `{
		"model_name":"model.a/~::b","display_name":"Display","description":"说明","description_en":"Description",
		"icon":"icon","tags":"video","vendor":"Vendor./~","billing_currency":"CNY","endpoints":"[\"video\"]",
		"status":1,"sync_official":1,"name_rule":0,"context_length":123,"max_output_tokens":12,
		"knowledge_cutoff":"2026-01","release_date":"2026-01-01","input_modalities":["text","image"],
		"output_modalities":["video"],"capabilities":["video_generation"],"metadata_source":"manual",
		"metadata_verified_at":"2026-10-08","marketplace_enabled":true,"display_order":4,
		"supported_parameters":["prompt"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],
		"max_input_images":2,"output_formats":["mp4"],"min_duration":1,"max_duration":15,
		"reference_modalities":["image"],"created_time":1234,"updated_time":5678
	}`, entries["model:model.a/~::b"].Value)
	for _, entry := range snapshot.Entries {
		if entry.Kind == "vendor" || entry.Kind == "model" {
			continue
		}
		var key catalogmanifest.PriceKey
		var value catalogmanifest.PriceValue
		require.NoError(t, common.UnmarshalJsonStr(entry.Key, &key))
		require.NoError(t, common.UnmarshalJsonStr(entry.Value, &value))
		if key.Option == "ModelPrice" {
			assert.Equal(t, "model.a/~::b", key.Model)
			assert.Equal(t, "/model.a~1~0::b", key.Path)
			assert.JSONEq(t, `0`, value.Value)
			assert.Equal(t, "CNY", value.BillingCurrency)
		}
		if entry.Kind == "plugin_price" {
			assert.Equal(t, "provider-a", key.Plugin)
			assert.Equal(t, "model.a/~::b", key.Model)
			assert.JSONEq(t, `"tier(\"base\", u(\"seconds\") * 0.5)"`, value.Value)
		}
		if key.Option == "molii_grok_tool_price.web_search" {
			assert.Equal(t, "CNY", value.BillingCurrency, "the Grok settings store direct CNY tool prices")
			assert.Equal(t, "thousand_calls", value.Unit)
		}
		assert.NotEqual(t, "CacheRatio", key.Option, "absent defaults must remain absent")
	}
	encoded, err := common.Marshal(snapshot)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "do-not-export-secret")
	assert.NotContains(t, string(encoded), "private-brand")
	assert.NotContains(t, string(encoded), "USDExchangeRate")
	assert.NotContains(t, string(encoded), "GroupRatio")

	// A price with no metadata must still be discovered, and must block rather
	// than silently vanish or acquire a guessed USD currency.
	require.NoError(t, db.Model(&model.Option{}).Where("key = ?", "ModelPrice").Update("value", `{"model.a/~::b":0,"standalone":1}`).Error)
	_, err = ExportManaged(context.Background(), db, "dev")
	require.ErrorContains(t, err, "standalone")
	require.ErrorContains(t, err, "ModelPrice")
	require.ErrorContains(t, err, "currency")
	require.NoError(t, db.Create(&model.Model{ModelName: "standalone", BillingCurrency: "USD"}).Error)
	snapshot, err = ExportManaged(context.Background(), db, "dev")
	require.NoError(t, err)
	assert.Equal(t, 3, snapshot.Coverage["model_price"])

	// Unknown fields in a pricing namespace must fail closed, not leak through
	// a wildcard whitelist or disappear from a purportedly complete export.
	require.NoError(t, db.Create(&model.Option{Key: "molii_grok_price.unknown_secret", Value: "redacted"}).Error)
	_, err = ExportManaged(context.Background(), db, "dev")
	require.ErrorContains(t, err, "unknown price field")
	assert.NotContains(t, err.Error(), "redacted")
}

func TestManagedExportSnapshotIntegrity(t *testing.T) {
	db := openCatalogSyncTestDB(t)
	require.NoError(t, db.Create(&model.Model{ModelName: "m", BillingCurrency: "USD"}).Error)
	require.NoError(t, db.Create(&[]model.Option{
		{Key: "ModelPrice", Value: `{"m":0}`},
		{Key: "billing_setting.billing_expr", Value: `{"m":"tier(\"request\", fixed(0))"}`},
	}).Error)
	snapshot, err := ExportManaged(context.Background(), db, "dev")
	require.NoError(t, err)
	require.NoError(t, catalogmanifest.ValidateSnapshot(snapshot))
	digest, err := catalogmanifest.SnapshotDigest(snapshot)
	require.NoError(t, err)
	assert.Equal(t, snapshot.Digest, digest)
	reordered := snapshot
	reordered.Entries = slices.Clone(snapshot.Entries)
	slices.Reverse(reordered.Entries)
	reordered.ExportedAt++
	for i := range reordered.Entries {
		var value any
		require.NoError(t, common.UnmarshalJsonStr(reordered.Entries[i].Value, &value))
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		reordered.Entries[i].Value = " \n" + string(encoded) + "\t "
	}
	gotDigest, err := catalogmanifest.SnapshotDigest(reordered)
	require.NoError(t, err)
	assert.Equal(t, snapshot.Digest, gotDigest, "entry order, JSON whitespace, and display time must not affect digest")
	require.NoError(t, catalogmanifest.ValidateSnapshot(reordered))

	for _, tc := range []struct {
		name   string
		mutate func(*catalogmanifest.Snapshot)
	}{
		{"partial", func(s *catalogmanifest.Snapshot) { s.Complete = false }},
		{"wrong schema", func(s *catalogmanifest.Snapshot) { s.SchemaVersion = 1 }},
		{"wrong digest", func(s *catalogmanifest.Snapshot) { s.Digest = "sha256:wrong" }},
		{"missing coverage", func(s *catalogmanifest.Snapshot) { delete(s.Coverage, "tool_price") }},
		{"partial entries", func(s *catalogmanifest.Snapshot) { s.Entries = s.Entries[1:] }},
		{"duplicate identity", func(s *catalogmanifest.Snapshot) { s.Entries = append(s.Entries, s.Entries[0]) }},
		{"unknown capability", func(s *catalogmanifest.Snapshot) { s.Capabilities["billing_engine"] = "unsupported" }},
		{"missing source", func(s *catalogmanifest.Snapshot) { s.SourceID = "" }},
		{"unknown entry", func(s *catalogmanifest.Snapshot) {
			s.Entries = append(s.Entries, catalogmanifest.Entry{Kind: "secret", Key: "x", Value: `"private"`})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := common.Marshal(snapshot)
			require.NoError(t, err)
			var modified catalogmanifest.Snapshot
			require.NoError(t, common.Unmarshal(encoded, &modified))
			tc.mutate(&modified)
			if tc.name != "wrong digest" {
				if digest, err := catalogmanifest.SnapshotDigest(modified); err == nil {
					modified.Digest = digest
				}
			}
			require.Error(t, catalogmanifest.ValidateSnapshot(modified))
		})
	}

	// Null, negative and unknown options are invalid pricing, not explicit zero.
	for _, raw := range []string{`{"m":null}`, `{"m":-1}`, `{"m":"0"}`} {
		require.NoError(t, db.Model(&model.Option{}).Where("key = ?", "ModelPrice").Update("value", raw).Error)
		_, err := ExportManaged(context.Background(), db, "dev")
		require.ErrorContains(t, err, "ModelPrice")
	}
}

func TestManagedExportCanonicalIdentityAndMetadata(t *testing.T) {
	db := openCatalogSyncTestDB(t)
	require.NoError(t, db.Create(&model.Model{ModelName: "m", BillingCurrency: "USD", InputModalities: []string{}, ContextLength: 128000, UpdatedTime: 1700000000}).Error)
	require.NoError(t, db.Create(&model.Option{Key: "ModelPrice", Value: `{"m":0}`}).Error)
	snapshot, err := ExportManaged(context.Background(), db, "dev")
	require.NoError(t, err)
	for _, entry := range snapshot.Entries {
		if entry.Kind != "model" {
			continue
		}
		var metadata map[string]any
		require.NoError(t, common.UnmarshalJsonStr(entry.Value, &metadata))
		assert.Equal(t, []any{}, metadata["input_modalities"], "stored empty lists must not become null")
	}
	var priceEntry catalogmanifest.Entry
	for _, entry := range snapshot.Entries {
		if entry.Kind == "model_price" {
			priceEntry = entry
		}
	}
	priceEntry.Key = `{"option":"ModelPrice","path":"/m","plugin":"","model":"m"}`
	snapshot.Entries = append(snapshot.Entries, priceEntry)
	snapshot.Coverage["model_price"]++
	snapshot.Digest, err = catalogmanifest.SnapshotDigest(snapshot)
	require.NoError(t, err)
	require.ErrorContains(t, catalogmanifest.ValidateSnapshot(snapshot), "duplicate")

	require.NoError(t, db.Create(&model.Option{Key: "billing_setting.billing_expr", Value: `{"m":"v2:tier(\"base\", p * 2)"}`}).Error)
	_, err = ExportManaged(context.Background(), db, "dev")
	require.ErrorContains(t, err, "version")
}

func TestManagedExportDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var dialector gorm.Dialector
			switch dialect {
			case "sqlite":
				dialector = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				dialector = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				dialector = postgres.Open(dsn)
			}
			prefix := fmt.Sprintf("managed_export_%d_%d_", os.Getpid(), catalogSyncTestDatabaseID.Add(1))
			db, err := gorm.Open(dialector, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&model.Model{}, &model.Vendor{}, &model.Option{}))
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&model.Option{}, &model.Vendor{}))
			if dialect == "mysql" {
				// This is an export READ fixture, not migration verification.
				// Existing Model TEXT defaults fail MySQL creation (Error 1101).
				// Retain every persisted column/type, removing only those defaults.
				definition := reflect.TypeFor[model.Model]()
				fields := make([]reflect.StructField, definition.NumField())
				for i := range definition.NumField() {
					field := definition.Field(i)
					gormTag := field.Tag.Get("gorm")
					if strings.Contains(gormTag, "type:text") {
						clauses := strings.Split(gormTag, ";")
						clauses = slices.DeleteFunc(clauses, func(clause string) bool { return strings.HasPrefix(clause, "default:") })
						field.Tag = reflect.StructTag(fmt.Sprintf("json:%q gorm:%q", field.Tag.Get("json"), strings.Join(clauses, ";")))
					}
					fields[i] = field
				}
				require.NoError(t, db.Table(prefix+"models").AutoMigrate(reflect.New(reflect.StructOf(fields)).Interface()))
				for i := range definition.NumField() {
					field := definition.Field(i)
					if field.Tag.Get("gorm") == "-" {
						continue
					}
					require.True(t, db.Migrator().HasColumn(&model.Model{}, field.Name), "missing persisted fixture column %s", field.Name)
				}
			} else {
				require.NoError(t, db.AutoMigrate(&model.Model{}))
			}
			require.NoError(t, db.Create(&model.Vendor{Name: "Provider", Status: 1}).Error)
			require.NoError(t, db.Create(&model.Model{
				ModelName: "m/~", BillingCurrency: "CNY", Status: 1, InputModalities: []string{},
				SupportedParameters: []string{}, SupportedResolutions: []string{}, SupportedAspectRatios: []string{},
				OutputFormats: []string{}, ReferenceModalities: []string{},
			}).Error)
			require.NoError(t, db.Create(&[]model.Option{
				{Key: "ModelPrice", Value: `{"m/~":0}`},
				{Key: "billing_setting.plugin_billing_expr", Value: `{"provider::m/~":"tier(\"base\", u(\"count\") * 0.5)"}`},
				{Key: "tool_price_setting.prices", Value: `{"search:m/~":0}`},
				{Key: "SystemName", Value: "private"},
			}).Error)
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				snapshot, err := ExportManaged(context.Background(), tx, "dev")
				require.NoError(t, err)
				require.NoError(t, catalogmanifest.ValidateSnapshot(snapshot))
				assert.Equal(t, map[string]int{"vendor": 1, "model": 1, "model_price": 1, "plugin_price": 1, "special_price": 0, "tool_price": 1}, snapshot.Coverage)
				return nil
			}))
		})
	}
}

func TestManagedExportCanonicalJSON(t *testing.T) {
	canonical, err := catalogmanifest.CanonicalJSON(` { "large":9007199254740993,"b":1.2300,"a":1.0 } `)
	require.NoError(t, err)
	assert.Equal(t, `{"a":1,"b":123e-2,"large":9007199254740993}`, canonical)
	for _, raw := range []string{"0", "-0.0", "0e20"} {
		got, err := catalogmanifest.CanonicalJSON(raw)
		require.NoError(t, err)
		assert.Equal(t, "0", got)
	}
	key, err := catalogmanifest.EncodePriceKey(catalogmanifest.PriceKey{Option: "ModelPrice", Model: "a.b/~", Path: "/a.b~1~0"})
	require.NoError(t, err)
	assert.Equal(t, `{"model":"a.b/~","option":"ModelPrice","path":"/a.b~1~0"}`, key)
	assert.Equal(t, `["model_price","{\"model\":\"a.b/~\",\"option\":\"ModelPrice\",\"path\":\"/a.b~1~0\"}"]`, catalogmanifest.EntryID(catalogmanifest.Entry{Kind: "model_price", Key: key}))
	snapshot := catalogmanifest.Snapshot{SchemaVersion: 2, SourceID: "target", Complete: true, Capabilities: catalogmanifest.RequiredCapabilities(), Coverage: map[string]int{}}
	before, err := catalogmanifest.SnapshotDigest(snapshot)
	require.NoError(t, err)
	snapshot.ObjectVersions = map[string]string{"model:m": "opaque-incarnation"}
	after, err := catalogmanifest.SnapshotDigest(snapshot)
	require.NoError(t, err)
	assert.NotEqual(t, before, after, "target digest must bind incarnation tokens")
}
