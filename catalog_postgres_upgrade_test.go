package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Representative persisted catalog columns independently transcribed from
// v1.0.0-rc.40:model/{model_meta,vendor_meta}.go. This is not a full old release
// database or an execution of the released binary.
type catalogBootReleasedModel struct {
	Id           int
	ModelName    string         `gorm:"size:128;not null;uniqueIndex:uk_model_name_delete_at,priority:1"`
	Description  string         `gorm:"type:text"`
	Icon         string         `gorm:"type:varchar(128)"`
	Tags         string         `gorm:"type:varchar(255)"`
	VendorID     int            `gorm:"index"`
	Endpoints    string         `gorm:"type:text"`
	Status       int            `gorm:"default:1"`
	SyncOfficial int            `gorm:"default:1"`
	CreatedTime  int64          `gorm:"bigint"`
	UpdatedTime  int64          `gorm:"bigint"`
	DeletedAt    gorm.DeletedAt `gorm:"index;uniqueIndex:uk_model_name_delete_at,priority:2"`
	NameRule     int            `gorm:"default:0"`
}

func (catalogBootReleasedModel) TableName() string { return "models" }

type catalogBootReleasedVendor struct {
	Id          int
	Name        string         `gorm:"size:128;not null;uniqueIndex:uk_vendor_name_delete_at,priority:1"`
	Description string         `gorm:"type:text"`
	Icon        string         `gorm:"type:varchar(128)"`
	Status      int            `gorm:"default:1"`
	CreatedTime int64          `gorm:"bigint"`
	UpdatedTime int64          `gorm:"bigint"`
	DeletedAt   gorm.DeletedAt `gorm:"index;uniqueIndex:uk_vendor_name_delete_at,priority:2"`
}

func (catalogBootReleasedVendor) TableName() string { return "vendors" }

func catalogBootPostgresURL(t *testing.T, dsn string) *url.URL {
	t.Helper()
	u, err := url.Parse(dsn)
	require.True(t, err == nil, "task PostgreSQL URL required")
	require.Contains(t, []string{"postgres", "postgresql"}, u.Scheme)
	require.Contains(t, []string{"127.0.0.1", "::1"}, u.Hostname())
	require.NotNil(t, u.User)
	require.NotEmpty(t, u.User.Username())
	require.NotEmpty(t, strings.Trim(u.Path, "/"))
	require.Empty(t, u.Fragment)
	require.Empty(t, u.Opaque)
	query, err := url.ParseQuery(u.RawQuery)
	require.NoError(t, err)
	require.Equal(t, url.Values{"sslmode": {"disable"}}, query, "connection overrides are forbidden")
	return u
}

func TestCatalogPostgresReleasedBootTwice(t *testing.T) {
	if os.Getenv("CATALOG_RELEASED_BOOT_CHILD") == "1" {
		catalogBootPostgresURL(t, os.Getenv("SQL_DSN"))
		require.NoError(t, InitResources())
		defer model.CloseDB()
		require.NoError(t, model.WithCatalogPricingRead(context.Background(), "preserved-release-model", func() error { return nil }))
		var entry model.Model
		require.NoError(t, model.DB.First(&entry, 51).Error)
		assert.Equal(t, "preserved original description", entry.Description)
		assert.Equal(t, 41, entry.VendorID)
		assert.EqualValues(t, 123, entry.CreatedTime)
		var vendor model.Vendor
		require.NoError(t, model.DB.First(&vendor, 41).Error)
		assert.Equal(t, "preserved vendor", vendor.Description)
		assert.True(t, model.DB.Migrator().HasIndex(&model.Model{}, "uk_model_name_delete_at"))
		assert.True(t, model.DB.Migrator().HasIndex(&model.Vendor{}, "uk_vendor_name_delete_at"))
		var state model.CatalogSyncState
		require.NoError(t, model.DB.First(&state, model.CatalogSyncStateID).Error)
		assert.Empty(t, state.PendingOperationID)
		return
	}
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	u := catalogBootPostgresURL(t, dsn)
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := admin.DB()
	require.NoError(t, err)
	defer pool.Close()
	name := fmt.Sprintf("catalog_released_boot_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec(`CREATE DATABASE "`+name+`"`).Error)
	defer func() { require.NoError(t, admin.Exec(`DROP DATABASE "`+name+`"`).Error) }()
	u.Path = "/" + name
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&catalogBootReleasedModel{}, &catalogBootReleasedVendor{}))
	require.NoError(t, db.Create(&catalogBootReleasedVendor{Id: 41, Name: "preserved-release-vendor", Description: "preserved vendor"}).Error)
	require.NoError(t, db.Create(&catalogBootReleasedModel{Id: 51, ModelName: "preserved-release-model", Description: "preserved original description", VendorID: 41, CreatedTime: 123}).Error)
	fixturePool, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, fixturePool.Close())
	for stage := range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCatalogPostgresReleasedBootTwice$", "-test.v", "--log-dir="+t.TempDir())
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "CATALOG_RELEASED_BOOT_CHILD=1", "SQL_DSN="+u.String(), "LOG_SQL_DSN=", "REDIS_CONN_STRING=", "NODE_TYPE=master", "NODE_NAME=catalog-released-boot", "CATALOG_SYNC_ROLE=disabled", "CATALOG_SYNC_SINGLE_INSTANCE=", "CATALOG_SYNC_TOKEN=", "CATALOG_SYNC_READERS_JSON=", "MEMORY_CACHE_ENABLED=false")
		output, runErr := cmd.CombinedOutput()
		cancel()
		t.Logf("real boot %d output:\n%s", stage+1, output)
		require.NoError(t, runErr)
	}
}
