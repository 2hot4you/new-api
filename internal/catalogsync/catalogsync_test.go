package catalogsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
