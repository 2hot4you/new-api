package model

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var currentMarketplaceCatalogModelNames = []string{
	"deepseek-flash",
	"deepseek-v4-flash-202605",
	"deepseek-v4-pro-202606",
	"gemini-3.8-flash",
	"claude-sonnet-5-5",
	"claude-opus-5-5",
	"claude-fable-5-1",
	"gpt-6-sol",
	"gpt-6-luna",
	"gpt-6-astra",
	"gpt-image-2.5-sunburst",
	"gpt-image-2.5-flare",
	"glm-5.2",
	"kimi-k3",
	"minimax-m3",
	"qwen3.5-flash",
	"qwen3.5-plus",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
	"doubao-seedance-2-0-mini-260615",
	"doubao-seedance-2-5-260628",
	"grok-imagine-image",
	"grok-imagine-image-quality",
	"grok-imagine-image-2.0",
	"grok-imagine-video",
	"grok-imagine-video-1.5",
}

var officialMarketplaceCatalogVendorNames = map[string]string{
	"deepseek-flash":         "DeepSeek",
	"gemini-3.8-flash":       "Google",
	"claude-sonnet-5-5":      "Anthropic",
	"claude-opus-5-5":        "Anthropic",
	"claude-fable-5-1":       "Anthropic",
	"gpt-6-sol":              "OpenAI",
	"gpt-6-luna":             "OpenAI",
	"gpt-6-astra":            "OpenAI",
	"gpt-image-2.5-sunburst": "OpenAI",
	"gpt-image-2.5-flare":    "OpenAI",
}

func newMarketplaceMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "marketplace-backfill.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Vendor{}, &Model{}))
	return db
}

func seedCurrentCatalogRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, modelName := range currentMarketplaceCatalogModelNames {
		require.NoError(t, db.Create(&Model{
			ModelName:   modelName,
			Description: "existing catalog description",
			VendorID:    1,
			Status:      1,
		}).Error)
	}
}

func loadMarketplaceRows(t *testing.T, db *gorm.DB) []Model {
	t.Helper()
	var rows []Model
	require.NoError(t, db.Order("model_name ASC").Find(&rows).Error)
	return rows
}

func loadMarketplaceRow(t *testing.T, db *gorm.DB, modelName string) Model {
	t.Helper()
	var row Model
	require.NoError(t, db.Where("model_name = ?", modelName).First(&row).Error)
	return row
}

func openMarketplacePostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MODEL_MARKETPLACE_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("MODEL_MARKETPLACE_POSTGRES_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return db
}

func TestMarketplacePostgresFreshBootstrapConverges(t *testing.T) {
	db := openMarketplacePostgresTestDB(t)
	require.False(t, db.Migrator().HasTable(&Model{}), "the explicit Compose migration must have run before models exists")

	require.NoError(t, db.AutoMigrate(&Model{}))
	require.NoError(t, ensureModelMarketplaceMetadataSchema(db))

	var requiredColumns int64
	require.NoError(t, db.Raw(`
		SELECT count(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'models'
		  AND column_name IN (
		    'display_name', 'description_en', 'marketplace_enabled',
		    'supported_parameters', 'supported_resolutions', 'supported_aspect_ratios',
		    'max_input_images', 'output_formats', 'min_duration', 'max_duration',
		    'reference_modalities'
		  )
		  AND is_nullable = 'NO'
	`).Scan(&requiredColumns).Error)
	require.Equal(t, int64(11), requiredColumns)

	var indexCount int64
	require.NoError(t, db.Raw(`
		SELECT count(*)
		FROM pg_indexes
		WHERE schemaname = 'public'
		  AND tablename = 'models'
		  AND indexname = 'idx_models_marketplace_enabled_status'
		  AND indexdef LIKE '%(marketplace_enabled, status)%'
		  AND indexdef LIKE '%WHERE (deleted_at IS NULL)%'
	`).Scan(&indexCount).Error)
	require.Equal(t, int64(1), indexCount)

	type defaultsRow struct {
		DisplayName           string
		DescriptionEN         string
		MarketplaceEnabled    bool
		SupportedParameters   string
		SupportedResolutions  string
		SupportedAspectRatios string
		MaxInputImages        int
		OutputFormats         string
		MinDuration           int
		MaxDuration           int
		ReferenceModalities   string
	}
	var defaults defaultsRow
	require.NoError(t, db.Raw(`
		INSERT INTO public.models (model_name)
		VALUES ('fresh-bootstrap-defaults')
		RETURNING display_name, description_en, marketplace_enabled,
		  supported_parameters, supported_resolutions, supported_aspect_ratios,
		  max_input_images, output_formats, min_duration, max_duration,
		  reference_modalities
	`).Scan(&defaults).Error)
	require.Equal(t, "", defaults.DisplayName)
	require.Equal(t, "", defaults.DescriptionEN)
	require.False(t, defaults.MarketplaceEnabled)
	require.Equal(t, "[]", defaults.SupportedParameters)
	require.Equal(t, "[]", defaults.SupportedResolutions)
	require.Equal(t, "[]", defaults.SupportedAspectRatios)
	require.Zero(t, defaults.MaxInputImages)
	require.Equal(t, "[]", defaults.OutputFormats)
	require.Zero(t, defaults.MinDuration)
	require.Zero(t, defaults.MaxDuration)
	require.Equal(t, "[]", defaults.ReferenceModalities)
}

func TestBackfillLocalMarketplaceMetadataPreservesConcurrentAdministratorUpdate(t *testing.T) {
	setupDB := openMarketplacePostgresTestDB(t)
	require.NoError(t, setupDB.AutoMigrate(&Model{}))
	require.NoError(t, ensureModelMarketplaceMetadataSchema(setupDB))
	require.NoError(t, setupDB.Exec("TRUNCATE TABLE public.models RESTART IDENTITY").Error)
	require.NoError(t, setupDB.Create(&Model{
		ModelName: "qwen3.5-flash",
		VendorID:  1,
		Status:    1,
	}).Error)

	dsn := strings.TrimSpace(os.Getenv("MODEL_MARKETPLACE_POSTGRES_TEST_DSN"))
	backfillDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	for _, connection := range []*gorm.DB{backfillDB, adminDB} {
		sqlDB, dbErr := connection.DB()
		require.NoError(t, dbErr)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	}

	readObserved := make(chan struct{})
	resumeBackfill := make(chan struct{})
	var blockOnce sync.Once
	require.NoError(t, backfillDB.Callback().Query().After("gorm:query").Register("test:block_after_marketplace_read", func(tx *gorm.DB) {
		if tx.Statement.Table != "models" {
			return
		}
		blockOnce.Do(func() {
			close(readObserved)
			<-resumeBackfill
		})
	}))

	backfillResult := make(chan error, 1)
	go func() { backfillResult <- BackfillLocalMarketplaceMetadata(backfillDB) }()

	select {
	case <-readObserved:
	case <-time.After(5 * time.Second):
		close(resumeBackfill)
		t.Fatal("backfill did not reach the post-read pause")
	}
	require.NoError(t, adminDB.Model(&Model{}).
		Where("model_name = ?", "qwen3.5-flash").
		UpdateColumns(map[string]interface{}{
			"description": "concurrent administrator description",
			"status":      0,
		}).Error)
	close(resumeBackfill)

	select {
	case err := <-backfillResult:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("backfill did not finish after resuming")
	}

	var stored Model
	require.NoError(t, adminDB.Where("model_name = ?", "qwen3.5-flash").First(&stored).Error)
	require.Equal(t, "concurrent administrator description", stored.Description)
	require.Zero(t, stored.Status)
	require.False(t, stored.MarketplaceEnabled)
}

func TestBackfillLocalMarketplaceMetadataCoversCurrentCatalog(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	seedCurrentCatalogRows(t, db)

	require.NoError(t, BackfillLocalMarketplaceMetadata(db))

	rows := loadMarketplaceRows(t, db)
	require.Len(t, rows, len(currentMarketplaceCatalogModelNames))
	for _, row := range rows {
		readiness := row.EvaluateMarketplaceReadiness()
		require.Truef(t, readiness.Complete, "%s missing metadata: %v", row.ModelName, readiness.Missing)
		require.Truef(t, row.MarketplaceEnabled, "%s was not published", row.ModelName)
		require.NotEmpty(t, row.DisplayName)
		require.Equal(t, "existing catalog description", row.Description)
		if officialMarketplaceCatalogVendorNames[row.ModelName] != "" {
			require.NotEmpty(t, row.MetadataSource)
			require.Equal(t, "2026-09-29", row.MetadataVerifiedAt)
		} else {
			require.Empty(t, row.MetadataSource)
			require.Empty(t, row.MetadataVerifiedAt)
		}
	}
}

func TestBackfillLocalMarketplaceMetadataUsesValidatedLocalCapabilities(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	seedCurrentCatalogRows(t, db)
	require.NoError(t, BackfillLocalMarketplaceMetadata(db))

	flash := loadMarketplaceRow(t, db, "qwen3.5-flash")
	require.Equal(t, []string{"stream", "tools", "tool_choice", "reasoning_effort", "response_format"}, flash.SupportedParameters)

	seedance := loadMarketplaceRow(t, db, "doubao-seedance-2-0-260128")
	require.Equal(t, []string{"480p", "720p", "1080p", "4k"}, seedance.SupportedResolutions)
	require.Equal(t, []string{"16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"}, seedance.SupportedAspectRatios)
	require.Equal(t, 4, seedance.MinDuration)
	require.Equal(t, 15, seedance.MaxDuration)
	require.Equal(t, 9, seedance.MaxInputImages)
	require.Equal(t, []string{"image", "video", "audio"}, seedance.ReferenceModalities)

	fast := loadMarketplaceRow(t, db, "doubao-seedance-2-0-fast-260128")
	require.Equal(t, []string{"480p", "720p"}, fast.SupportedResolutions)

	mini := loadMarketplaceRow(t, db, "doubao-seedance-2-0-mini-260615")
	require.Equal(t, []string{"480p", "720p"}, mini.SupportedResolutions)
	require.Equal(t, 15, mini.MaxDuration)

	seedance25 := loadMarketplaceRow(t, db, "doubao-seedance-2-5-260628")
	require.Equal(t, []string{"480p", "720p", "1080p"}, seedance25.SupportedResolutions)
	require.Equal(t, 30, seedance25.MaxInputImages)
	require.Equal(t, 30, seedance25.MaxDuration)

	image := loadMarketplaceRow(t, db, "grok-imagine-image-quality")
	require.Equal(t, []string{"1k", "2k"}, image.SupportedResolutions)
	require.Equal(t, 3, image.MaxInputImages)
	require.Equal(t, []string{"url"}, image.OutputFormats)

	image20 := loadMarketplaceRow(t, db, "grok-imagine-image-2.0")
	require.Contains(t, image20.SupportedParameters, "quality")
	require.Equal(t, []string{"1k", "2k"}, image20.SupportedResolutions)

	legacyVideo := loadMarketplaceRow(t, db, "grok-imagine-video")
	require.Equal(t, []string{"480p", "720p"}, legacyVideo.SupportedResolutions)
	require.Equal(t, []string{"image", "video"}, legacyVideo.ReferenceModalities)

	video15 := loadMarketplaceRow(t, db, "grok-imagine-video-1.5")
	require.Equal(t, []string{"480p", "720p", "1080p"}, video15.SupportedResolutions)
	require.Equal(t, []string{"image"}, video15.ReferenceModalities)
}

func TestBackfillLocalMarketplaceMetadataAddsCurrentOfficialModels(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	vendorIDs := make(map[string]int)
	for _, vendorName := range []string{"Anthropic", "DeepSeek", "Google", "OpenAI"} {
		vendor := Vendor{Name: vendorName, Status: 1}
		require.NoError(t, db.Create(&vendor).Error)
		vendorIDs[vendorName] = vendor.Id
	}
	for modelName := range officialMarketplaceCatalogVendorNames {
		require.NoError(t, db.Create(&Model{ModelName: modelName, Status: 1}).Error)
	}

	require.NoError(t, BackfillLocalMarketplaceMetadata(db))

	for modelName, vendorName := range officialMarketplaceCatalogVendorNames {
		entry := loadMarketplaceRow(t, db, modelName)
		require.Equalf(t, vendorIDs[vendorName], entry.VendorID, "%s vendor", modelName)
		require.Truef(t, entry.EvaluateMarketplaceReadiness().Complete, "%s missing metadata: %v", modelName, entry.EvaluateMarketplaceReadiness().Missing)
		require.Truef(t, entry.MarketplaceEnabled, "%s was not published", modelName)
		require.NotEmptyf(t, entry.DescriptionEN, "%s English description", modelName)
		require.NotEmptyf(t, entry.Icon, "%s icon", modelName)
		require.NotEmptyf(t, entry.MetadataSource, "%s metadata source", modelName)
		require.Equalf(t, "2026-09-29", entry.MetadataVerifiedAt, "%s verification date", modelName)
	}

	deepseek := loadMarketplaceRow(t, db, "deepseek-flash")
	require.Equal(t, "DeepSeek V4.1 Flash", deepseek.DisplayName)
	require.Equal(t, 1_000_000, deepseek.ContextLength)
	require.Equal(t, 384_000, deepseek.MaxOutputTokens)
	require.Equal(t, "2026-09-10", deepseek.ReleaseDate)
	require.Equal(t, []string{"text", "image"}, deepseek.InputModalities)

	gemini := loadMarketplaceRow(t, db, "gemini-3.8-flash")
	require.Equal(t, 1_048_576, gemini.ContextLength)
	require.Equal(t, 65_536, gemini.MaxOutputTokens)
	require.Equal(t, "2026-09-02", gemini.ReleaseDate)
	require.Equal(t, []string{"text", "image", "video", "audio", "file"}, gemini.InputModalities)

	sonnet := loadMarketplaceRow(t, db, "claude-sonnet-5-5")
	require.Equal(t, 1_000_000, sonnet.ContextLength)
	require.Equal(t, 128_000, sonnet.MaxOutputTokens)
	require.Equal(t, "2026-06", sonnet.KnowledgeCutoff)
	require.Equal(t, "2026-09-28", sonnet.ReleaseDate)

	sol := loadMarketplaceRow(t, db, "gpt-6-sol")
	require.Equal(t, 1_050_000, sol.ContextLength)
	require.Equal(t, 128_000, sol.MaxOutputTokens)
	require.Equal(t, "2026-04-20", sol.KnowledgeCutoff)
	require.Equal(t, "2026-09-22", sol.ReleaseDate)

	sunburst := loadMarketplaceRow(t, db, "gpt-image-2.5-sunburst")
	require.Equal(t, []string{"auto", "1024x1024", "1536x1024", "1024x1536", "2048x2048", "2048x1152", "3840x2160", "2160x3840", "custom"}, sunburst.SupportedResolutions)
	require.Equal(t, []string{"auto", "1:1", "3:2", "2:3", "16:9", "9:16", "custom ≤3:1"}, sunburst.SupportedAspectRatios)
	require.Equal(t, 16, sunburst.MaxInputImages)
	require.Equal(t, []string{"b64_json"}, sunburst.OutputFormats)
}

func TestBackfillLocalMarketplaceMetadataPreservesOfficialAdministratorValues(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	customVendor := Vendor{Name: "Administrator Vendor", Status: 1}
	require.NoError(t, db.Create(&customVendor).Error)
	admin := Model{
		ModelName:          "gpt-6-sol",
		DisplayName:        "gpt-6-sol",
		Description:        "administrator description",
		DescriptionEN:      "administrator English description",
		VendorID:           customVendor.Id,
		MetadataSource:     "administrator",
		MetadataVerifiedAt: "2026-09-01",
		Status:             1,
	}
	require.NoError(t, db.Create(&admin).Error)

	require.NoError(t, BackfillLocalMarketplaceMetadata(db))

	stored := loadMarketplaceRow(t, db, admin.ModelName)
	require.Equal(t, admin.DisplayName, stored.DisplayName)
	require.Equal(t, admin.Description, stored.Description)
	require.Equal(t, admin.DescriptionEN, stored.DescriptionEN)
	require.Equal(t, admin.VendorID, stored.VendorID)
	require.Equal(t, admin.MetadataSource, stored.MetadataSource)
	require.Equal(t, admin.MetadataVerifiedAt, stored.MetadataVerifiedAt)
}

func TestBackfillLocalMarketplaceMetadataSkipsOfficialSyncDisabledModel(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	openAI := Vendor{Name: "OpenAI", Status: 1}
	require.NoError(t, db.Create(&openAI).Error)
	entry := Model{
		ModelName:    "gpt-6-luna",
		DisplayName:  "administrator draft",
		Description:  "administrator description",
		VendorID:     openAI.Id,
		Status:       1,
		SyncOfficial: 0,
	}
	require.NoError(t, db.Create(&entry).Error)
	require.NoError(t, db.Model(&Model{}).Where("id = ?", entry.Id).UpdateColumn("sync_official", 0).Error)

	require.NoError(t, BackfillLocalMarketplaceMetadata(db))

	stored := loadMarketplaceRow(t, db, entry.ModelName)
	require.Zero(t, stored.SyncOfficial)
	require.Zero(t, stored.ContextLength)
	require.Empty(t, stored.DescriptionEN)
	require.Empty(t, stored.Icon)
	require.False(t, stored.MarketplaceEnabled)
}

func TestBackfillLocalMarketplaceMetadataStopsWhenOfficialSyncIsDisabledDuringBackfill(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	openAI := Vendor{Name: "OpenAI", Status: 1}
	require.NoError(t, db.Create(&openAI).Error)
	entry := Model{ModelName: "gpt-6-astra", Status: 1, SyncOfficial: 1}
	require.NoError(t, db.Create(&entry).Error)

	initialReadDone := false
	var disableOnce sync.Once
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:observe_official_backfill_read", func(tx *gorm.DB) {
		if tx.Statement.Table == "models" {
			if _, ok := tx.Statement.Dest.(*[]Model); ok {
				initialReadDone = true
			}
		}
	}))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:disable_official_sync_during_backfill", func(tx *gorm.DB) {
		if !initialReadDone || tx.Statement.Table != "models" {
			return
		}
		disableOnce.Do(func() {
			require.NoError(t, tx.Exec("UPDATE models SET sync_official = 0 WHERE id = ?", entry.Id).Error)
		})
	}))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Query().Remove("test:observe_official_backfill_read"))
		require.NoError(t, db.Callback().Update().Remove("test:disable_official_sync_during_backfill"))
	})

	require.NoError(t, BackfillLocalMarketplaceMetadata(db))

	stored := loadMarketplaceRow(t, db, entry.ModelName)
	require.Zero(t, stored.SyncOfficial)
	require.Empty(t, stored.DisplayName)
	require.Zero(t, stored.ContextLength)
	require.Empty(t, stored.Capabilities)
	require.False(t, stored.MarketplaceEnabled)
}

func TestBackfillLocalMarketplaceMetadataCorrectsOnlyLegacySeedance25ImageLimit(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	require.NoError(t, db.Create(&Model{
		ModelName:      "doubao-seedance-2-5-260628",
		Description:    "existing catalog description",
		VendorID:       1,
		Status:         1,
		MaxInputImages: 9,
	}).Error)
	require.NoError(t, db.Create(&Model{
		ModelName:      "doubao-seedance-2-0-260128",
		Description:    "existing catalog description",
		VendorID:       1,
		Status:         1,
		MaxInputImages: 12,
	}).Error)

	require.NoError(t, BackfillLocalMarketplaceMetadata(db))

	seedance25 := loadMarketplaceRow(t, db, "doubao-seedance-2-5-260628")
	require.Equal(t, 30, seedance25.MaxInputImages)
	customSeedance20 := loadMarketplaceRow(t, db, "doubao-seedance-2-0-260128")
	require.Equal(t, 12, customSeedance20.MaxInputImages)
}

func TestBackfillLocalMarketplaceMetadataPreservesAdministratorValues(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	admin := Model{
		ModelName:             "qwen3.5-plus",
		DisplayName:           "Administrator title",
		Description:           "Administrator description",
		DescriptionEN:         "Administrator English description",
		Icon:                  "AdministratorIcon",
		Tags:                  "administrator,tags",
		VendorID:              99,
		Status:                0,
		ContextLength:         123,
		MaxOutputTokens:       45,
		KnowledgeCutoff:       "administrator cutoff",
		ReleaseDate:           "2025-01-01",
		InputModalities:       []string{"text"},
		OutputModalities:      []string{"text"},
		Capabilities:          []string{"streaming"},
		MetadataSource:        "administrator",
		MetadataVerifiedAt:    "2025-01-02",
		SupportedParameters:   []string{"temperature"},
		SupportedResolutions:  []string{"administrator-resolution"},
		SupportedAspectRatios: []string{"administrator-ratio"},
		MaxInputImages:        8,
		OutputFormats:         []string{"url"},
		MinDuration:           2,
		MaxDuration:           3,
		ReferenceModalities:   []string{"image"},
	}
	require.NoError(t, db.Create(&admin).Error)
	require.NoError(t, db.Model(&Model{}).Where("id = ?", admin.Id).Update("status", 0).Error)
	admin.Status = 0

	require.NoError(t, BackfillLocalMarketplaceMetadata(db))

	stored := loadMarketplaceRow(t, db, admin.ModelName)
	require.Equal(t, admin.DisplayName, stored.DisplayName)
	require.Equal(t, admin.Description, stored.Description)
	require.Equal(t, admin.DescriptionEN, stored.DescriptionEN)
	require.Equal(t, admin.Icon, stored.Icon)
	require.Equal(t, admin.Tags, stored.Tags)
	require.Equal(t, admin.VendorID, stored.VendorID)
	require.Equal(t, admin.Status, stored.Status)
	require.Equal(t, admin.ContextLength, stored.ContextLength)
	require.Equal(t, admin.MaxOutputTokens, stored.MaxOutputTokens)
	require.Equal(t, admin.KnowledgeCutoff, stored.KnowledgeCutoff)
	require.Equal(t, admin.ReleaseDate, stored.ReleaseDate)
	require.Equal(t, admin.InputModalities, stored.InputModalities)
	require.Equal(t, admin.OutputModalities, stored.OutputModalities)
	require.Equal(t, admin.Capabilities, stored.Capabilities)
	require.Equal(t, admin.MetadataSource, stored.MetadataSource)
	require.Equal(t, admin.MetadataVerifiedAt, stored.MetadataVerifiedAt)
	require.Equal(t, admin.SupportedParameters, stored.SupportedParameters)
	require.Equal(t, admin.SupportedResolutions, stored.SupportedResolutions)
	require.Equal(t, admin.SupportedAspectRatios, stored.SupportedAspectRatios)
	require.Equal(t, admin.MaxInputImages, stored.MaxInputImages)
	require.Equal(t, admin.OutputFormats, stored.OutputFormats)
	require.Equal(t, admin.MinDuration, stored.MinDuration)
	require.Equal(t, admin.MaxDuration, stored.MaxDuration)
	require.Equal(t, admin.ReferenceModalities, stored.ReferenceModalities)
	require.False(t, stored.MarketplaceEnabled)
}

func TestBackfillLocalMarketplaceMetadataDoesNotRestoreClearedOptionalProvenance(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	require.NoError(t, db.Create(&Model{
		ModelName:           "qwen3.5-plus",
		DisplayName:         "Qwen3.5 Plus",
		Description:         "curated description",
		DescriptionEN:       "curated English description",
		VendorID:            1,
		Status:              1,
		ContextLength:       1_000_000,
		MaxOutputTokens:     64_000,
		ReleaseDate:         "2026-02-16",
		InputModalities:     []string{"text", "image", "file", "video"},
		OutputModalities:    []string{"text"},
		Capabilities:        []string{"streaming", "vision", "tools"},
		SupportedParameters: []string{"stream", "tools", "tool_choice"},
	}).Error)

	require.NoError(t, BackfillLocalMarketplaceMetadata(db))

	stored := loadMarketplaceRow(t, db, "qwen3.5-plus")
	require.Empty(t, stored.KnowledgeCutoff)
	require.Empty(t, stored.MetadataSource)
	require.Empty(t, stored.MetadataVerifiedAt)
}

func TestBackfillLocalMarketplaceMetadataPublishesOnlyCompleteEnabledRows(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	require.NoError(t, db.Create(&Model{
		ModelName: "glm-5.2",
		Status:    1,
	}).Error)
	disabledSeed := Model{
		ModelName:   "kimi-k3",
		Description: "disabled model",
		VendorID:    1,
		Status:      0,
	}
	require.NoError(t, db.Create(&disabledSeed).Error)
	require.NoError(t, db.Model(&Model{}).Where("id = ?", disabledSeed.Id).Update("status", 0).Error)

	require.NoError(t, BackfillLocalMarketplaceMetadata(db))

	incomplete := loadMarketplaceRow(t, db, "glm-5.2")
	require.False(t, incomplete.EvaluateMarketplaceReadiness().Complete)
	require.False(t, incomplete.MarketplaceEnabled)
	disabled := loadMarketplaceRow(t, db, "kimi-k3")
	require.True(t, disabled.EvaluateMarketplaceReadiness().Complete)
	require.False(t, disabled.MarketplaceEnabled)
}

func TestBackfillLocalMarketplaceMetadataIsIdempotent(t *testing.T) {
	db := newMarketplaceMigrationTestDB(t)
	seedCurrentCatalogRows(t, db)
	require.NoError(t, BackfillLocalMarketplaceMetadata(db))
	first := loadMarketplaceRows(t, db)
	require.NoError(t, BackfillLocalMarketplaceMetadata(db))
	second := loadMarketplaceRows(t, db)
	require.Equal(t, first, second)
}
