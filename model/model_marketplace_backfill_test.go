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
	"deepseek-v4-flash-202605",
	"deepseek-v4-pro-202606",
	"glm-5.2",
	"kimi-k3",
	"minimax-m3",
	"qwen3.5-flash",
	"qwen3.5-plus",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
	"grok-imagine-image",
	"grok-imagine-image-quality",
	"grok-imagine-video",
	"grok-imagine-video-1.5",
}

func newMarketplaceMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "marketplace-backfill.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Model{}))
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
		require.Equal(t, row.ModelName, row.DisplayName)
		require.Equal(t, "existing catalog description", row.Description)
		require.NotEmpty(t, row.MetadataSource)
		require.NotEmpty(t, row.MetadataVerifiedAt)
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

	image := loadMarketplaceRow(t, db, "grok-imagine-image-quality")
	require.Equal(t, []string{"1k", "2k"}, image.SupportedResolutions)
	require.Equal(t, 3, image.MaxInputImages)
	require.Equal(t, []string{"url"}, image.OutputFormats)

	legacyVideo := loadMarketplaceRow(t, db, "grok-imagine-video")
	require.Equal(t, []string{"480p", "720p"}, legacyVideo.SupportedResolutions)
	require.Equal(t, []string{"image", "video"}, legacyVideo.ReferenceModalities)

	video15 := loadMarketplaceRow(t, db, "grok-imagine-video-1.5")
	require.Equal(t, []string{"480p", "720p", "1080p"}, video15.SupportedResolutions)
	require.Equal(t, []string{"image"}, video15.ReferenceModalities)
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
