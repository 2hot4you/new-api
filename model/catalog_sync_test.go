package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func catalogSyncTestSource(t *testing.T) catalogmanifest.Snapshot {
	t.Helper()
	value, err := common.Marshal(catalogmanifest.VendorValue{Name: "source-vendor", Description: "original", Status: 1, DisplayOrder: 1, CreatedTime: 13, UpdatedTime: 17})
	require.NoError(t, err)
	source := catalogmanifest.Snapshot{SchemaVersion: 2, SourceID: "dev", Complete: true, ExportedAt: 42, Capabilities: catalogmanifest.RequiredCapabilities(), Coverage: map[string]int{}, Entries: []catalogmanifest.Entry{{Kind: "vendor", Key: "source-vendor", Value: string(value)}}}
	for _, kind := range catalogmanifest.Kinds() {
		source.Coverage[kind] = 0
	}
	source.Coverage["vendor"] = 1
	source.Digest, err = catalogmanifest.SnapshotDigest(source)
	require.NoError(t, err)
	return source
}

func TestCatalogSyncRefreshExcludesWriter(t *testing.T) {
	db := catalogSyncTestDB(t, "sqlite")
	require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Channel{}, &Ability{}))
	var reads atomic.Int32
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("catalog-refresh-reader", func(tx *gorm.DB) {
		reads.Add(1)
		if catalogBarrier.TryLock() {
			catalogBarrier.Unlock()
			t.Error("catalog writer entered during pricing refresh DB reads")
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("catalog-refresh-reader"); InvalidatePricingCache() })
	RefreshPricing()
	require.Positive(t, reads.Load())
}

func TestCatalogSyncStore(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}, &marketplaceOrderLock{}))
			require.NoError(t, MigrateCatalogSync(db))
			require.NoError(t, db.Create(&Vendor{Name: "source-vendor", Description: "original", Status: 1, DisplayOrder: 1, CreatedTime: 99, UpdatedTime: 101}).Error)
			actor := catalogmanifest.Actor{UserID: 1, SessionID: "opaque-session", TargetID: "local", AuthVersion: 2, SessionVersion: 3}
			source := catalogSyncTestSource(t)
			now := time.Now().UTC()
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, now)
			require.NoError(t, err)
			assert.NotEmpty(t, plan.ID)
			assert.Equal(t, now.Add(10*time.Minute).Unix(), plan.ExpiresAt)
			require.Len(t, plan.Changes, 1)
			assert.Equal(t, "adopt", plan.Changes[0].Action)
			assert.Equal(t, source, plan.Snapshot)
			got, err := GetCatalogSyncPlan(context.Background(), plan.ID, actor)
			require.NoError(t, err)
			assert.Equal(t, plan, got)
			resolved, err := ResolveCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
			require.NoError(t, err)
			_, err = ResolveCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, actor, catalogmanifest.Resolution{})
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
			_, err = ResolveCatalogSyncPlan(context.Background(), plan.ID, resolved.Digest, actor, resolved.Resolution)
			require.NoError(t, err, "repeating the already-current resolution is idempotent")
			for _, field := range []string{"user", "session", "target", "auth_version", "session_version"} {
				wrong := actor
				switch field {
				case "user":
					wrong.UserID++
				case "session":
					wrong.SessionID += "wrong"
				case "target":
					wrong.TargetID += "wrong"
				case "auth_version":
					wrong.AuthVersion++
				case "session_version":
					wrong.SessionVersion++
				}
				_, err := GetCatalogSyncPlan(context.Background(), plan.ID, wrong)
				require.Error(t, err, field)
			}
			expired, err := CreateCatalogSyncPlan(context.Background(), source, actor, now.Add(-11*time.Minute))
			require.NoError(t, err)
			_, err = GetCatalogSyncPlan(context.Background(), expired.ID, actor)
			require.Error(t, err)
			actor.AuthVersion = 0
			_, err = CreateCatalogSyncPlan(context.Background(), source, actor, now)
			require.Error(t, err)
			// Snapshots and operation backups exceed MySQL's 64 KiB TEXT ceiling.
			actor.AuthVersion = 2
			large := catalogSyncTestSource(t)
			var value catalogmanifest.VendorValue
			require.NoError(t, common.UnmarshalJsonStr(large.Entries[0].Value, &value))
			value.Description = strings.Repeat("x", 80_000)
			encoded, err := common.Marshal(value)
			require.NoError(t, err)
			large.Entries[0].Value = string(encoded)
			large.Digest, err = catalogmanifest.SnapshotDigest(large)
			require.NoError(t, err)
			largePlan, err := CreateCatalogSyncPlan(context.Background(), large, actor, now)
			require.NoError(t, err)
			large.Entries[0].Value = "changed caller buffer"
			stored, err := GetCatalogSyncPlan(context.Background(), largePlan.ID, actor)
			require.NoError(t, err)
			assert.Equal(t, string(encoded), stored.Snapshot.Entries[0].Value)
			require.NoError(t, db.Create(&CatalogSyncOperation{ID: largePlan.ID, PlanID: largePlan.ID, State: "prepared", Backup: CatalogSyncText(encoded), History: CatalogSyncText(encoded), Result: "{}"}).Error)
		})
	}
}

func TestCatalogSyncStoreBaselineIncarnation(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}))
			require.NoError(t, MigrateCatalogSync(db))
			vendor := Vendor{Name: "managed-vendor", Status: 1, CreatedTime: 10}
			require.NoError(t, db.Create(&vendor).Error)
			model := Model{ModelName: "managed-model", VendorID: vendor.Id, BillingCurrency: "CNY", CreatedTime: 20}
			require.NoError(t, db.Create(&model).Error)
			source, err := ExportManagedCatalogTx(context.Background(), db, "dev")
			require.NoError(t, err)
			var original catalogmanifest.Snapshot
			require.NoError(t, WithCatalogWriteBarrier(func() error {
				return db.Transaction(func(tx *gorm.DB) error {
					state, err := LockCatalogMutationTx(tx)
					if err != nil {
						return err
					}
					original, err = CaptureCatalogTargetTx(context.Background(), tx, "target", state)
					if err != nil {
						return err
					}
					_, err = SaveCatalogSyncBaselineTx(tx, source, original.ObjectVersions, 0)
					return err
				})
			}))
			var state CatalogSyncState
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			baseline, err := LoadCatalogSyncBaselineTx(db, &state)
			require.NoError(t, err)
			assert.ElementsMatch(t, source.Entries, baseline.Entries, "baseline must retain full source audit metadata")
			assert.Equal(t, original.ObjectVersions, baseline.ObjectVersions)
			assert.Equal(t, int64(1), baseline.Generation)
			require.NoError(t, withMarketplaceOrderTransaction(db, func(tx *gorm.DB) error {
				if err := tx.Delete(&model).Error; err != nil {
					return err
				}
				model.Id, model.DeletedAt = 0, gorm.DeletedAt{}
				return tx.Create(&model).Error
			}))
			actor := catalogmanifest.Actor{UserID: 1, SessionID: "session", TargetID: "target", AuthVersion: 1, SessionVersion: 1}
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			for _, change := range plan.Changes {
				if change.Kind == "model" {
					assert.Equal(t, "conflict", change.Action)
				}
			}
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			recreated, err := CaptureCatalogTargetTx(context.Background(), db, "target", &state)
			require.NoError(t, err)
			id := catalogmanifest.EntryID(catalogmanifest.Entry{Kind: "model", Key: "managed-model"})
			assert.NotEqual(t, original.ObjectVersions[id], recreated.ObjectVersions[id])
			assert.Len(t, recreated.ObjectVersions[id], 64)
			assert.NotEqual(t, original.Digest, recreated.Digest)
			// Keep only the vendor so this complete source explicitly removes the model.
			source.Entries = nil
			for _, entry := range baseline.Entries {
				if entry.Kind == "vendor" {
					source.Entries = append(source.Entries, entry)
				}
			}
			source.Coverage["model"] = 0
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			removed, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			for _, change := range removed.Changes {
				if change.Kind == "model" {
					assert.Equal(t, "blocked", change.Action)
					assert.Equal(t, "recreated_object_not_managed", change.Reason)
				}
			}
			_, err = SaveCatalogSyncBaselineTx(db, source, original.ObjectVersions, 0)
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
		})
	}
}

func TestCatalogSyncStoreOptionWritersAndNoop(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}))
			require.NoError(t, MigrateCatalogSync(db))
			previousOptions, previousRate := maps.Clone(common.OptionMap), operation_setting.USDExchangeRate
			previousAddress := system_setting.ServerAddress
			settings := config.GlobalConfig.ExportAllConfigs()
			t.Cleanup(func() {
				common.OptionMap = previousOptions
				operation_setting.USDExchangeRate = previousRate
				system_setting.ServerAddress = previousAddress
				require.NoError(t, config.GlobalConfig.LoadFromDB(settings))
			})
			common.OptionMap = map[string]string{}
			system_setting.ServerAddress = "https://catalog.example.com"
			*system_setting.GetPasskeySettings() = system_setting.PasskeySettings{RPID: "catalog.example.com", Origins: "https://catalog.example.com"}
			// Observe lock coverage before any option read, including reloads.
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("catalog-test-option-read", func(tx *gorm.DB) {
				if tx.Statement.Table != "options" {
					return
				}
				if catalogBarrier.TryRLock() {
					catalogBarrier.RUnlock()
					t.Error("option writer/reload read without catalog barrier")
				}
			}))
			t.Cleanup(func() { _ = db.Callback().Query().Remove("catalog-test-option-read") })
			require.NoError(t, UpdateOption("molii_grok_price.image_standard_1k", "0.125"))
			var before CatalogSyncState
			require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
			assert.Equal(t, int64(1), before.Revision)
			require.NoError(t, UpdateOption("molii_grok_price.image_standard_1k", "0.125"))
			loadOptionsFromDatabase()
			require.NoError(t, UpdateOption("USDExchangeRate", "7.25"))
			var after CatalogSyncState
			require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
			assert.Equal(t, before.Revision, after.Revision)
			assert.Equal(t, before.CurrentDigest, after.CurrentDigest)
			assert.Equal(t, 7.25, operation_setting.USDExchangeRate)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"molii_grok_price.image_standard_1k": "0.25", "ServerAddress": "https://catalog.example.com"}))
			require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
			assert.Equal(t, before.Revision+1, after.Revision, "mixed passkey/catalog options keep the common transaction")
			// Existing anchor counter activity from a no-op reorder is not a revision.
			require.NoError(t, ReorderModels(nil))
			var reordered CatalogSyncState
			require.NoError(t, db.First(&reordered, CatalogSyncStateID).Error)
			assert.Equal(t, after.Revision, reordered.Revision)
		})
	}
}

// These are the persisted fields from released v1.0.0-rc.40 (model_meta.go and
// vendor_meta.go), not fields reflected out of the new Model under test.
type catalogReleasedModel struct {
	Id           int
	ModelName    string         `gorm:"size:128;not null;uniqueIndex:uk_model_name_delete_at,priority:1"`
	Description  string         `gorm:"type:text"`
	Icon         string         `gorm:"type:varchar(128)"`
	Tags         string         `gorm:"type:varchar(255)"`
	VendorID     int            `gorm:"index:idx_models_vendor_id"`
	Endpoints    string         `gorm:"type:text"`
	Status       int            `gorm:"default:1"`
	SyncOfficial int            `gorm:"default:1"`
	CreatedTime  int64          `gorm:"bigint"`
	UpdatedTime  int64          `gorm:"bigint"`
	DeletedAt    gorm.DeletedAt `gorm:"index:idx_models_deleted_at;uniqueIndex:uk_model_name_delete_at,priority:2"`
	NameRule     int            `gorm:"default:0"`
}

func (catalogReleasedModel) TableName() string { return "models" }

type catalogReleasedVendor struct {
	Id          int
	Name        string         `gorm:"size:128;not null;uniqueIndex:uk_vendor_name_delete_at,priority:1"`
	Description string         `gorm:"type:text"`
	Icon        string         `gorm:"type:varchar(128)"`
	Status      int            `gorm:"default:1"`
	CreatedTime int64          `gorm:"bigint"`
	UpdatedTime int64          `gorm:"bigint"`
	DeletedAt   gorm.DeletedAt `gorm:"index:idx_vendors_deleted_at;uniqueIndex:uk_vendor_name_delete_at,priority:2"`
}

func (catalogReleasedVendor) TableName() string { return "vendors" }

func TestCatalogSyncStoreReleasedUpgrade(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			require.NoError(t, db.AutoMigrate(&catalogReleasedModel{}, &catalogReleasedVendor{}, &Option{}))
			vendor := catalogReleasedVendor{Name: "released-vendor", Description: "preserved", Status: 1}
			require.NoError(t, db.Create(&vendor).Error)
			old := catalogReleasedModel{ModelName: "released-model", Description: "do not replace", VendorID: vendor.Id, CreatedTime: 123, UpdatedTime: 456}
			require.NoError(t, db.Create(&old).Error)
			for attempt := range 3 {
				recorder := &migrationSQLRecorder{}
				migrationDB := db.Session(&gorm.Session{Logger: recorder})
				require.NoError(t, migrationDB.AutoMigrate(&Model{}, &Vendor{}, &Option{}, &marketplaceOrderLock{}))
				require.NoError(t, MigrateCatalogSync(migrationDB))
				require.NoError(t, ensureModelMarketplaceMetadataSchema(migrationDB))
				if attempt > 0 {
					assert.Empty(t, recorder.schemaMutations(), "repeat migration must not mutate schema")
				}
				var model Model
				require.NoError(t, db.First(&model, old.Id).Error)
				assert.Equal(t, "do not replace", model.Description)
				assert.Equal(t, vendor.Id, model.VendorID)
				assert.Equal(t, int64(123), model.CreatedTime)
				assert.Empty(t, model.SupportedParameters)
				assert.True(t, db.Migrator().HasIndex(&Model{}, "uk_model_name_delete_at"))
				assert.True(t, db.Migrator().HasIndex(&Vendor{}, "uk_vendor_name_delete_at"))
			}
		})
	}
}

func TestCatalogSyncSourceConcurrentMutation(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			if engine == "sqlite" {
				require.NoError(t, db.Exec("PRAGMA journal_mode=WAL").Error)
			}
			require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}))
			require.NoError(t, MigrateCatalogSync(db))
			vendor := Vendor{Name: "concurrent", Description: "before", Status: 1}
			require.NoError(t, db.Create(&vendor).Error)
			require.NoError(t, db.Create(&Model{ModelName: "concurrent-model", VendorID: vendor.Id, BillingCurrency: "USD"}).Error)
			// Simulate an independent committed DB writer between source queries.
			// A process mutex alone cannot protect PostgreSQL READ COMMITTED.
			require.NoError(t, WithCatalogReadSnapshot(context.Background(), func(tx *gorm.DB) error {
				var first Vendor
				if err := tx.First(&first, vendor.Id).Error; err != nil {
					return err
				}
				committed := make(chan error, 1)
				go func() {
					committed <- db.Transaction(func(write *gorm.DB) error {
						if err := write.Model(&Vendor{}).Where("id = ?", vendor.Id).Update("description", "after").Error; err != nil {
							return err
						}
						return write.Model(&Model{}).Where("model_name = ?", "concurrent-model").Update("billing_currency", "CNY").Error
					})
				}()
				if err := <-committed; err != nil {
					return err
				}
				snapshot, err := ExportManagedCatalogTx(context.Background(), tx, "dev")
				if err != nil {
					return err
				}
				for _, entry := range snapshot.Entries {
					if entry.Kind == "vendor" {
						var value catalogmanifest.VendorValue
						require.NoError(t, common.UnmarshalJsonStr(entry.Value, &value))
						assert.Equal(t, "before", value.Description)
					}
					if entry.Kind == "model" {
						var value catalogmanifest.ModelValue
						require.NoError(t, common.UnmarshalJsonStr(entry.Value, &value))
						assert.Equal(t, "USD", value.BillingCurrency)
					}
				}
				return nil
			}))
			// Observe the production writer at its DB boundary: it must exclude
			// billing/snapshot readers before any metadata reads or writes begin.
			var observed atomic.Bool
			writerPaused, releaseWriter := make(chan struct{}), make(chan struct{})
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("catalog-test-writer", func(tx *gorm.DB) {
				if tx.Statement.Table != "vendors" {
					return
				}
				observed.Store(true)
				if catalogBarrier.TryRLock() {
					catalogBarrier.RUnlock()
					t.Error("ordinary vendor writer did not hold catalog barrier")
				}
				close(writerPaused)
				<-releaseWriter
			}))
			t.Cleanup(func() { _ = db.Callback().Create().Remove("catalog-test-writer") })
			written := make(chan error, 1)
			go func() { written <- (&Vendor{Name: "normal-writer", Status: 1}).Insert() }()
			<-writerPaused
			readerStarted, readDone := make(chan struct{}), make(chan error, 1)
			var concurrent catalogmanifest.Snapshot
			go func() {
				close(readerStarted)
				readDone <- WithCatalogReadSnapshot(context.Background(), func(tx *gorm.DB) error {
					var err error
					concurrent, err = ExportManagedCatalogTx(context.Background(), tx, "dev")
					return err
				})
			}()
			<-readerStarted
			close(releaseWriter)
			require.NoError(t, <-written)
			require.NoError(t, <-readDone)
			assert.Equal(t, 2, concurrent.Coverage["vendor"], "export waits for the real ordinary writer to finish")
			assert.True(t, observed.Load())
			var state CatalogSyncState
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			assert.Equal(t, int64(1), state.Revision)
		})
	}
}

// Every server fixture gets its own disposable database, and accepts only a
// loopback test DSN. Production dialectors are essential migration coverage.
func catalogSyncTestDB(t *testing.T, engine string) *gorm.DB {
	t.Helper()
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	var db *gorm.DB
	var err error
	name := fmt.Sprintf("catalog_sync_%d", time.Now().UnixNano())
	switch engine {
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "catalog.db")), cfg)
	case "postgres":
		dsn := os.Getenv("TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("TEST_POSTGRES_DSN is not set")
		}
		u, parseErr := url.Parse(dsn)
		require.NoError(t, parseErr)
		require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname())
		admin, openErr := gorm.Open(postgres.Open(dsn), cfg)
		require.NoError(t, openErr)
		require.NoError(t, admin.Exec(`CREATE DATABASE "`+name+`"`).Error)
		t.Cleanup(func() {
			require.NoError(t, admin.Exec(`DROP DATABASE "`+name+`"`).Error)
			sqlDB, _ := admin.DB()
			_ = sqlDB.Close()
		})
		u.Path = "/" + name
		db, err = gorm.Open(postgresMigrationDialector{postgres.Dialector{Config: &postgres.Config{DSN: u.String(), PreferSimpleProtocol: true}}}, cfg)
	case "mysql":
		dsn := os.Getenv("TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("TEST_MYSQL_DSN is not set")
		}
		parsed, parseErr := mysqldriver.ParseDSN(dsn)
		require.NoError(t, parseErr)
		require.True(t, strings.HasPrefix(parsed.Addr, "127.0.0.1:") || strings.HasPrefix(parsed.Addr, "localhost:"))
		admin, openErr := gorm.Open(mysql.Open(dsn), cfg)
		require.NoError(t, openErr)
		require.NoError(t, admin.Exec("CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4").Error)
		t.Cleanup(func() {
			require.NoError(t, admin.Exec("DROP DATABASE `"+name+"`").Error)
			sqlDB, _ := admin.DB()
			_ = sqlDB.Close()
		})
		parsed.DBName = name
		db, err = gorm.Open(mysqlMigrationDialector{mysql.Dialector{Config: &mysql.Config{DSN: parsed.FormatDSN()}}}, cfg)
	}
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	var version string
	query := "SELECT version()"
	if engine == "sqlite" {
		query = "SELECT sqlite_version()"
	}
	require.NoError(t, db.Raw(query).Scan(&version).Error)
	t.Logf("%s: %s", engine, version)
	previous, previousType := DB, common.MainDatabaseType()
	DB = db
	switch engine {
	case "sqlite":
		common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	case "mysql":
		common.SetMainDatabaseType(common.DatabaseTypeMySQL)
	case "postgres":
		common.SetMainDatabaseType(common.DatabaseTypePostgreSQL)
	}
	initCol()
	t.Cleanup(func() { DB = previous; common.SetMainDatabaseType(previousType); initCol() })
	return db
}

// Removing portable handling of TEXT defaults must break actual fresh MySQL
// migration; reflecting into an invented fixture model cannot satisfy this.
func TestCatalogSyncStoreMigrations(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}, &marketplaceOrderLock{}))
			require.NoError(t, db.Create(&Model{ModelName: "portable-model", DescriptionEN: "existing description", SupportedParameters: []string{"stream"}}).Error)
			for range 2 {
				require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}, &marketplaceOrderLock{}))
			}
			var row Model
			require.NoError(t, db.First(&row).Error)
			assert.Equal(t, "existing description", row.DescriptionEN)
			assert.Equal(t, []string{"stream"}, row.SupportedParameters)
			assert.True(t, db.Migrator().HasIndex(&Model{}, "uk_model_name_delete_at"))
		})
	}
}

// The released Channel declaration predates management credentials. Keep this
// fixture independent of Channel so a broken production TEXT default is visible.
type catalogReleasedChannel struct {
	Id                 int
	Type               int    `gorm:"default:0"`
	Key                string `gorm:"not null"`
	OpenAIOrganization *string
	TestModel          *string
	Status             int    `gorm:"default:1"`
	Name               string `gorm:"index"`
	Weight             *uint  `gorm:"default:0"`
	CreatedTime        int64  `gorm:"bigint"`
	TestTime           int64  `gorm:"bigint"`
	ResponseTime       int
	BaseURL            *string `gorm:"column:base_url;default:''"`
	Other              string
	Balance            float64
	BalanceUpdatedTime int64 `gorm:"bigint"`
	Models             string
	Group              string  `gorm:"type:varchar(64);default:'default'"`
	UsedQuota          int64   `gorm:"bigint;default:0"`
	ModelMapping       *string `gorm:"type:text"`
	StatusCodeMapping  *string `gorm:"type:varchar(1024);default:''"`
	Priority           *int64  `gorm:"bigint;default:0"`
	AutoBan            *int    `gorm:"default:1"`
	OtherInfo          string
	Tag                *string     `gorm:"index"`
	Setting            *string     `gorm:"type:text"`
	ParamOverride      *string     `gorm:"type:text"`
	HeaderOverride     *string     `gorm:"type:text"`
	Remark             *string     `gorm:"type:varchar(255)"`
	ChannelInfo        ChannelInfo `gorm:"type:json"`
	OtherSettings      string      `gorm:"column:settings"`
}

func (catalogReleasedChannel) TableName() string { return "channels" }

func TestCatalogSyncChannelMigrations(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		for _, upgrade := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/upgrade=%t", engine, upgrade), func(t *testing.T) {
				db := catalogSyncTestDB(t, engine)
				if upgrade {
					require.NoError(t, db.AutoMigrate(&catalogReleasedChannel{}))
					require.NoError(t, db.Create(&catalogReleasedChannel{Id: 7, Key: "fixture-only", Name: "retained", Models: "fixture-model", CreatedTime: 123}).Error)
				}
				credentialChannel := Channel{Key: "fixture-only", Name: "configured", MoliiGrokManagementAccessToken: "fixture-management-token"}
				for pass := range 3 {
					recorder := &migrationSQLRecorder{}
					require.NoError(t, db.Session(&gorm.Session{Logger: recorder}).AutoMigrate(&Channel{}))
					if pass > 0 {
						assert.Empty(t, recorder.schemaMutations())
					} else {
						require.NoError(t, db.Create(&credentialChannel).Error)
					}
				}
				channel := Channel{Key: "fixture-only", Name: "new"}
				require.NoError(t, db.Create(&channel).Error)
				var got Channel
				require.NoError(t, db.First(&got, channel.Id).Error)
				assert.Empty(t, got.MoliiGrokManagementAccessToken)
				assert.True(t, db.Migrator().HasIndex(&Channel{}, "idx_channels_name"))
				got = Channel{}
				require.NoError(t, db.First(&got, credentialChannel.Id).Error)
				assert.Equal(t, "fixture-management-token", got.MoliiGrokManagementAccessToken)
				if upgrade {
					got = Channel{}
					require.NoError(t, db.First(&got, 7).Error)
					assert.Equal(t, "retained", got.Name)
					assert.Equal(t, "fixture-model", got.Models)
					assert.Equal(t, int64(123), got.CreatedTime)
					assert.Empty(t, got.MoliiGrokManagementAccessToken)
				}
			})
		}
	}
}

func catalogFenceTestDB(t *testing.T, engine string) *gorm.DB {
	t.Helper()
	db := catalogSyncTestDB(t, engine)
	require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}, &Channel{}, &Ability{}, &Task{}, &TaskPlugin{}, &Midjourney{}, &SystemTask{}))
	require.NoError(t, MigrateCatalogSync(db))
	return db
}

// Removing any physical table fence permits a bypass writer to commit while
// the authoritative catalog transaction is still open, including empty tables.
func TestCatalogSyncReferenceFences(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			pool, err := db.DB()
			require.NoError(t, err)
			registry := jsplugin.NewRegistry()
			for _, fixture := range []struct {
				name   string
				insert string
				update string
				remove string
			}{
				{"channels", "INSERT INTO channels (id, " + commonKeyCol + ", status, molii_grok_management_access_token) VALUES (10, 'fixture', 2, '')", "UPDATE channels SET status = 1 WHERE id = 10", "DELETE FROM channels WHERE id = 10"},
				{"abilities", "INSERT INTO abilities (" + commonGroupCol + ", model, channel_id, enabled) VALUES ('g', 'm', 10, " + commonFalseVal + ")", "UPDATE abilities SET enabled = " + commonTrueVal + " WHERE channel_id = 10", "DELETE FROM abilities WHERE channel_id = 10"},
				{"tasks", "INSERT INTO tasks (id, status) VALUES (10, 'SUCCESS')", "UPDATE tasks SET status = 'IN_PROGRESS' WHERE id = 10", "DELETE FROM tasks WHERE id = 10"},
				{"task_plugins", "INSERT INTO task_plugins (id, " + commonKeyCol + ", version, source_hash, source, api_version, enabled, active, created_at) VALUES (10, 'fixture', '1', 'fixture', '', 1, " + commonFalseVal + ", " + commonFalseVal + ", 0)", "UPDATE task_plugins SET enabled = " + commonTrueVal + ", active = " + commonTrueVal + " WHERE id = 10", "DELETE FROM task_plugins WHERE id = 10"},
				{"options", "INSERT INTO options (" + commonKeyCol + ", value) VALUES ('TaskPluginEnabled', 'false')", "UPDATE options SET value = 'true' WHERE " + commonKeyCol + " = 'TaskPluginEnabled'", "DELETE FROM options WHERE " + commonKeyCol + " = 'TaskPluginEnabled'"},
				{"vendors", "INSERT INTO vendors (id, name, deleted_at) VALUES (10, 'fixture', '2020-01-01')", "UPDATE vendors SET deleted_at = NULL WHERE id = 10", "DELETE FROM vendors WHERE id = 10"},
				{"models", "INSERT INTO models (id, model_name, description_en, supported_parameters, supported_resolutions, supported_aspect_ratios, output_formats, reference_modalities, deleted_at) VALUES (10, 'fixture', '', '[]', '[]', '[]', '[]', '[]', '2020-01-01')", "UPDATE models SET deleted_at = NULL WHERE id = 10", "DELETE FROM models WHERE id = 10"},
				{"midjourneys", "INSERT INTO midjourneys (id, status) VALUES (10, 'SUCCESS')", "UPDATE midjourneys SET status = 'IN_PROGRESS' WHERE id = 10", "DELETE FROM midjourneys WHERE id = 10"},
				{"system_tasks", "INSERT INTO system_tasks (id, status) VALUES (10, 'succeeded')", "UPDATE system_tasks SET status = 'pending' WHERE id = 10", "DELETE FROM system_tasks WHERE id = 10"},
			} {
				t.Run(fixture.name, func(t *testing.T) {
					// ExecContext on an independently acquired connection is the actual
					// writer attempt; no goroutine-start signal or sleep stands in for it.
					second := strings.NewReplacer("(10,", "(20,", ", 10,", ", 20,", "'fixture'", "'second'", "'TaskPluginEnabled'", "'TaskPluginDisabledFactoryKeys'").Replace(fixture.insert)
					reverse := strings.NewReplacer("status = 1", "status = 2", "status = 'IN_PROGRESS'", "status = 'SUCCESS'", "enabled = "+commonTrueVal, "enabled = "+commonFalseVal, "active = "+commonTrueVal, "active = "+commonFalseVal, "value = 'true'", "value = 'false'", "deleted_at = NULL", "deleted_at = '2020-01-01'", "status = 'pending'", "status = 'succeeded'").Replace(fixture.update)
					for _, statement := range []string{fixture.insert, second, fixture.update, reverse, fixture.remove} {
						err := WithCatalogWriteBarrier(func() error {
							return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
								require.NotNil(t, state)
								require.NotNil(t, pin.Generation)
								catalogAssertWriterBlocked(t, pool, engine, statement)
								return nil
							})
						})
						require.NoError(t, err)
						// The same write is valid after the root COMMIT and all fences release.
						require.NoError(t, db.Exec(statement).Error)
					}
				})
			}
		})
	}
}

func catalogAssertWriterBlocked(t *testing.T, pool *sql.DB, engine, statement string) {
	t.Helper()
	conn, err := pool.Conn(context.Background())
	require.NoError(t, err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if engine == "sqlite" {
		_, err = conn.ExecContext(ctx, "PRAGMA busy_timeout = 0")
		require.NoError(t, err)
	}
	writer, err := conn.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer writer.Rollback()
	_, err = writer.ExecContext(ctx, statement)
	require.Error(t, err, "bypass writer completed before catalog commit: %s", statement)
}

func TestCatalogSyncReferenceRootRollback(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			stopped := errors.New("stop before commit")
			err := WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					require.NoError(t, tx.Create(&Option{Key: "rollback", Value: "uncommitted"}).Error)
					called := false
					err := catalogReferenceTransaction(context.Background(), tx, registry, func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error { called = true; return nil })
					require.Error(t, err, "nested/savepoint entry must be refused")
					require.False(t, called)
					return stopped
				})
			})
			require.ErrorIs(t, err, stopped)
			var count int64
			require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", "rollback").Count(&count).Error)
			assert.Zero(t, count)
			require.NoError(t, db.Create(&Option{Key: "after-rollback"}).Error)
			// Explicit cancellation rejects the root before any business callback.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err = catalogReferenceTransaction(ctx, db, registry, func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
				t.Error("canceled callback ran")
				return nil
			})
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

// The observer delegates every operation to the actual database transaction.
// It only pauses the real COMMIT boundary, where callback-only leases are wrong.
type catalogCommitObserver struct {
	gorm.ConnPool
	commit   func() error
	rollback func() error
}

func (o *catalogCommitObserver) Commit() error   { return o.commit() }
func (o *catalogCommitObserver) Rollback() error { return o.rollback() }

func TestCatalogSyncReferenceCommitLease(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			before := registry.Generation().Number
			publication := make(chan struct{})
			err := WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, pin *jsplugin.GenerationPin) error {
					require.Equal(t, before, pin.Generation.Number)
					require.NoError(t, tx.Create(&Option{Key: "lease-commit", Value: "durable"}).Error)
					real := tx.Statement.ConnPool.(*sql.Tx)
					tx.Statement.ConnPool = &catalogCommitObserver{ConnPool: real, rollback: real.Rollback, commit: func() error {
						started := make(chan struct{})
						go func() { close(started); registry.SetEnabled(false); close(publication) }()
						<-started
						require.Eventually(t, func() bool {
							probe, err := registry.TryPinGeneration()
							if probe != nil {
								probe.Release()
							}
							return errors.Is(err, jsplugin.ErrGenerationBusy)
						}, time.Second, time.Millisecond, "registry writer must actually be queued at COMMIT")
						assert.Equal(t, before, registry.Generation().Number, "lease released on callback return")
						select {
						case <-publication:
							t.Error("registry switched before COMMIT")
						default:
						}
						err := real.Commit()
						assert.Equal(t, before, registry.Generation().Number, "lease must span actual COMMIT return")
						return err
					}}
					return nil
				})
			})
			require.NoError(t, err)
			select {
			case <-publication:
			case <-time.After(3 * time.Second):
				t.Fatal("registry lease leaked after root return")
			}
			assert.Greater(t, registry.Generation().Number, before)
			var option Option
			require.NoError(t, db.First(&option, commonKeyCol+" = ?", "lease-commit").Error)
			assert.Equal(t, "durable", option.Value)
			// A lost COMMIT acknowledgement must remain distinguishable even when
			// the actual database committed. The helper must not replay the callback.
			calls := 0
			err = WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					calls++
					require.NoError(t, tx.Create(&Option{Key: "uncertain", Value: "committed"}).Error)
					real := tx.Statement.ConnPool.(*sql.Tx)
					tx.Statement.ConnPool = &catalogCommitObserver{ConnPool: real, rollback: real.Rollback, commit: func() error {
						if err := real.Commit(); err != nil {
							return err
						}
						return errors.New("test transport lost commit acknowledgement")
					}}
					return nil
				})
			})
			require.ErrorIs(t, err, ErrCatalogCommitUncertain)
			assert.Equal(t, 1, calls)
			option = Option{}
			require.NoError(t, db.First(&option, commonKeyCol+" = ?", "uncertain").Error)
			assert.Equal(t, "committed", option.Value)
		})
	}
}

func TestCatalogSyncReferenceSchemaAndFailure(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			callback := func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
				t.Error("unsafe schema reached callback")
				return nil
			}
			if engine == "mysql" {
				// The production legacy migration accepts this exact unique layout.
				require.NoError(t, db.Exec("ALTER TABLE options DROP PRIMARY KEY, ADD UNIQUE INDEX legacy_option_key (`key`)").Error)
				require.NoError(t, WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
						pool, err := db.DB()
						require.NoError(t, err)
						catalogAssertWriterBlocked(t, pool, engine, "INSERT INTO options (`key`, value) VALUES ('legacy-gap', '')")
						return tx.Create(&Option{Key: "legacy-index"}).Error
					})
				}))
				require.NoError(t, db.Exec("ALTER TABLE options DROP INDEX legacy_option_key").Error)
				require.Error(t, catalogReferenceTransaction(context.Background(), db, registry, callback))
				require.NoError(t, db.Exec("ALTER TABLE options ADD UNIQUE INDEX legacy_option_key (`key`)").Error)
				require.NoError(t, db.Exec("ALTER TABLE options ENGINE=MyISAM").Error)
				require.Error(t, catalogReferenceTransaction(context.Background(), db, registry, callback))
				require.NoError(t, db.Exec("ALTER TABLE options ENGINE=InnoDB").Error)
			}
			// A late missing relation cannot be treated as an empty reference set.
			require.NoError(t, db.Migrator().DropTable(&Task{}))
			require.Error(t, catalogReferenceTransaction(context.Background(), db, registry, callback))
			require.NoError(t, db.AutoMigrate(&Task{}))
			// Cancellation after a real scan obtained rows must release partial fences.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			seen := false
			if engine == "postgres" {
				require.NoError(t, db.Callback().Raw().After("gorm:raw").Register("cancel-fence", func(tx *gorm.DB) {
					if strings.HasPrefix(tx.Statement.SQL.String(), "LOCK TABLE") && strings.Contains(tx.Statement.SQL.String(), "channels") {
						seen = true
						cancel()
					}
				}))
			} else {
				require.NoError(t, db.Callback().Row().After("gorm:row").Register("cancel-fence", func(tx *gorm.DB) {
					if strings.HasPrefix(tx.Statement.SQL.String(), "SELECT `id` FROM `channels`") {
						seen = true
						cancel()
					}
				}))
			}
			err := WithCatalogWriteBarrier(func() error { return catalogReferenceTransaction(ctx, db, registry, callback) })
			require.Error(t, err)
			assert.True(t, seen, "cancellation must happen after actual partial fence acquisition")
			require.NoError(t, db.Callback().Raw().Remove("cancel-fence"))
			require.NoError(t, db.Callback().Row().Remove("cancel-fence"))
			require.NoError(t, db.Create(&Channel{Key: "after-cancel"}).Error)
			require.NoError(t, WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					return tx.Create(&Option{Key: "after-failure"}).Error
				})
			}))
		})
	}
}

// Poll actual server lock wait state; a goroutine start or elapsed sleep cannot
// prove the fence acquisition reached the database before the other commit.
func catalogWaitForDBLock(t *testing.T, db *gorm.DB, engine string, outcomes ...<-chan error) {
	t.Helper()
	query := "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'"
	interval := 10 * time.Millisecond
	if engine == "mysql" {
		var version string
		require.NoError(t, db.Raw("SELECT VERSION()").Scan(&version).Error)
		query = "SELECT count(*) FROM performance_schema.data_lock_waits"
		if strings.HasPrefix(version, "5.7.") {
			query = "SELECT count(*) FROM information_schema.innodb_lock_waits"
			// 5.7 caches InnoDB transaction metadata while queries arrive less
			// than 0.1s apart; faster polling can keep an empty snapshot forever.
			interval = 150 * time.Millisecond
		}
	}
	require.Eventually(t, func() bool {
		for _, outcome := range outcomes {
			select {
			case err := <-outcome:
				t.Fatalf("root returned before database lock wait: %v", err)
			default:
			}
		}
		var count int64
		err := db.Raw(query).Scan(&count).Error
		return err == nil && count > 0
	}, 5*time.Second, interval, "competing SQL must actually reach a lock wait")
}

func TestCatalogSyncReferenceWaitedSnapshot(t *testing.T) {
	for _, engine := range []string{"mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			require.NoError(t, db.Create(&Channel{Id: 10, Key: "fixture", Status: 2}).Error)
			// An opposite pool default catches accidental default-isolation use.
			pool, err := db.DB()
			require.NoError(t, err)
			conn, err := pool.Conn(context.Background())
			require.NoError(t, err)
			writer, err := conn.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			defer conn.Close()
			defer writer.Rollback()
			_, err = writer.Exec("UPDATE channels SET status = 1 WHERE id = 10")
			require.NoError(t, err)
			finished := make(chan error, 1)
			go func() {
				finished <- WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
						var status int
						if err := tx.Model(&Channel{}).Select("status").Where("id = ?", 10).Scan(&status).Error; err != nil {
							return err
						}
						if status != 1 {
							return fmt.Errorf("stale pre-wait reference snapshot: status=%d", status)
						}
						return nil
					})
				})
			}()
			catalogWaitForDBLock(t, db, engine, finished)
			require.NoError(t, writer.Commit())
			require.NoError(t, <-finished)
			// Now hold the same writer until the root deadline; all earlier fences
			// and the anchor must roll back without entering business code.
			writer, err = conn.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			_, err = writer.Exec("UPDATE channels SET status = 2 WHERE id = 10")
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
			defer cancel()
			go func() {
				finished <- WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(ctx, db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
						return errors.New("unexpected callback")
					})
				})
			}()
			catalogWaitForDBLock(t, db, engine)
			require.Error(t, <-finished)
			require.NoError(t, writer.Rollback())
			require.NoError(t, db.Create(&Ability{Group: "g", Model: "after-timeout", ChannelId: 10}).Error)
		})
	}
}

func TestCatalogSyncReferenceMySQLUnsafeGaps(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_UNSAFE_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_UNSAFE_DSN is not set")
	}
	t.Setenv("TEST_MYSQL_DSN", dsn)
	db := catalogFenceTestDB(t, "mysql")
	var unsafe int
	require.NoError(t, db.Raw("SELECT @@global.innodb_locks_unsafe_for_binlog").Scan(&unsafe).Error)
	require.Equal(t, 1, unsafe)
	err := catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
		t.Error("unsafe gap configuration admitted")
		return nil
	})
	require.ErrorContains(t, err, "gap locking")
}

func TestCatalogSyncReferenceMySQLTemporaryShadow(t *testing.T) {
	db := catalogFenceTestDB(t, "mysql")
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("CREATE TEMPORARY TABLE channels (id bigint PRIMARY KEY)").Error)
	err = catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
		t.Error("temporary shadow hid persistent references")
		return nil
	})
	require.Error(t, err)
	require.NoError(t, db.Exec("DROP TEMPORARY TABLE channels").Error)
}

func TestCatalogSyncReferenceMySQLAnchorEngineRace(t *testing.T) {
	db := catalogFenceTestDB(t, "mysql")
	var before marketplaceOrderLock
	require.NoError(t, db.First(&before).Error)
	raced := false
	swap := func(tx *gorm.DB) {
		query := tx.Statement.SQL.String()
		if !raced && (strings.HasPrefix(query, "UPDATE `marketplace_order_locks`") || strings.HasPrefix(query, "SELECT `name` FROM `marketplace_order_locks`")) {
			raced = true
			require.NoError(t, db.Exec("ALTER TABLE marketplace_order_locks ENGINE=MyISAM").Error)
		}
	}
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("swap-anchor-engine", swap))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("swap-anchor-engine", swap))
	err := catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
		t.Error("engine race reached callback")
		return nil
	})
	require.Error(t, err)
	require.NoError(t, db.Callback().Raw().Remove("swap-anchor-engine"))
	require.NoError(t, db.Callback().Row().Remove("swap-anchor-engine"))
	assert.True(t, raced)
	var after marketplaceOrderLock
	require.NoError(t, db.First(&after).Error)
	assert.Equal(t, before.Version, after.Version, "unsupported engine must not leave a nontransactional anchor write")
}

func TestCatalogSyncReferenceRangesAndSession(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			pool, err := db.DB()
			require.NoError(t, err)
			// Seed only one pooled connection, configure a contrary isolation or
			// long busy handler, then let root lease that exact connection first.
			pool.SetMaxIdleConns(1)
			switch engine {
			case "mysql":
				require.NoError(t, db.Exec("SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED").Error)
			case "postgres":
				require.NoError(t, db.Exec("SET SESSION CHARACTERISTICS AS TRANSACTION ISOLATION LEVEL REPEATABLE READ").Error)
			case "sqlite":
				require.NoError(t, db.Exec("PRAGMA busy_timeout = 60000").Error)
			}
			require.NoError(t, db.Create(&[]Channel{{Id: 10, Key: "fixture"}, {Id: 30, Key: "fixture"}}).Error)
			require.NoError(t, WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					if engine == "postgres" {
						var isolation string
						require.NoError(t, tx.Raw("SHOW transaction_isolation").Scan(&isolation).Error)
						assert.Equal(t, "read committed", isolation)
					}
					for _, id := range []int{1, 20, 40} {
						catalogAssertWriterBlocked(t, pool, engine, fmt.Sprintf("INSERT INTO channels (id, %s, molii_grok_management_access_token) VALUES (%d, 'fixture', '')", commonKeyCol, id))
					}
					catalogAssertWriterBlocked(t, pool, engine, "INSERT INTO channels ("+commonKeyCol+", molii_grok_management_access_token) VALUES ('auto', '')")
					// SQLite reset must be checked on this exact leased connection.
					if engine == "sqlite" {
						var busy int
						require.NoError(t, tx.Raw("PRAGMA busy_timeout").Scan(&busy).Error)
						assert.Zero(t, busy)
					}
					return nil
				})
			}))
			// Closing spare idle connections above lets the last-returning root
			// replace any writer connection; query exact remaining session value.
			if engine == "sqlite" {
				// Reader above used a real file-backed DEFERRED connection. Normal
				// post-commit writes still work after the temporary timeout override.
				require.NoError(t, db.Create(&Channel{Key: "after-ranges"}).Error)
			}
		})
	}
}

func TestCatalogSyncReferenceTrustedNaming(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			base := catalogFenceTestDB(t, engine)
			pool, err := base.DB()
			require.NoError(t, err)
			cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NamingStrategy: schema.NamingStrategy{TablePrefix: "fence space_"}}
			var dialector gorm.Dialector
			switch engine {
			case "sqlite":
				dialector = sqlite.Dialector{Conn: pool}
			case "mysql":
				dialector = mysqlMigrationDialector{mysql.Dialector{Config: &mysql.Config{Conn: pool}}}
			case "postgres":
				dialector = postgresMigrationDialector{postgres.Dialector{Config: &postgres.Config{Conn: pool, PreferSimpleProtocol: true}}}
			}
			db, err := gorm.Open(dialector, cfg)
			require.NoError(t, err)
			// Use actual production migrations, then rename those physical tables.
			// Custom-prefix migration support is separate from fence resolution.
			for _, model := range []any{&Model{}, &Vendor{}, &Option{}, &Channel{}, &Ability{}, &Task{}, &TaskPlugin{}, &Midjourney{}, &SystemTask{}, &CatalogSyncState{}, &CatalogSyncPlan{}, &CatalogSyncBaseline{}, &CatalogSyncOperation{}} {
				old, renamed := &gorm.Statement{DB: base}, &gorm.Statement{DB: db}
				require.NoError(t, old.Parse(model))
				require.NoError(t, renamed.Parse(model))
				require.NoError(t, base.Migrator().RenameTable(old.Schema.Table, renamed.Schema.Table))
			}
			require.NoError(t, WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					stmt := &gorm.Statement{DB: db}
					require.NoError(t, stmt.Parse(&Channel{}))
					query := "INSERT INTO " + stmt.Quote(stmt.Schema.Table) + " (" + commonKeyCol + ", molii_grok_management_access_token) VALUES ('naming', '')"
					catalogAssertWriterBlocked(t, pool, engine, query)
					return tx.Create(&Channel{Key: "naming"}).Error
				})
			}))
		})
	}
}

func TestCatalogSyncReferenceSessionReset(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			pool, err := db.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(1)
			pool.SetMaxIdleConns(1)
			query, expected := "PRAGMA busy_timeout", "60000"
			switch engine {
			case "sqlite":
				require.NoError(t, db.Exec("PRAGMA busy_timeout = 60000").Error)
			case "mysql":
				require.NoError(t, db.Exec("SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED").Error)
				var version string
				require.NoError(t, db.Raw("SELECT VERSION()").Scan(&version).Error)
				query, expected = "SELECT @@session.transaction_isolation", "READ-COMMITTED"
				if strings.HasPrefix(version, "5.7.") {
					query = "SELECT @@session.tx_isolation"
				}
			case "postgres":
				require.NoError(t, db.Exec("SET SESSION CHARACTERISTICS AS TRANSACTION ISOLATION LEVEL REPEATABLE READ").Error)
				query, expected = "SHOW transaction_isolation", "repeatable read"
			}
			for _, failure := range []bool{false, true} {
				err = WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
						if failure {
							return errors.New("rollback")
						}
						return nil
					})
				})
				if failure {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				var actual string
				require.NoError(t, db.Raw(query).Scan(&actual).Error)
				assert.Equal(t, expected, actual, "session configuration leaked across root transaction")
			}
		})
	}
}

func TestCatalogSyncReferencePostgresPrivileges(t *testing.T) {
	db := catalogFenceTestDB(t, "postgres")
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	role := fmt.Sprintf("catalog_fence_role_%d", time.Now().UnixNano())
	require.NoError(t, db.Exec("CREATE ROLE "+role+" NOLOGIN").Error)
	t.Cleanup(func() {
		require.NoError(t, db.Exec("RESET ROLE").Error)
		require.NoError(t, db.Exec("DROP OWNED BY "+role).Error)
		require.NoError(t, db.Exec("DROP ROLE "+role).Error)
	})
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA public TO "+role).Error)
	require.NoError(t, db.Exec("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO "+role).Error)
	require.NoError(t, db.Exec("SET ROLE "+role).Error)
	run := func() error {
		return WithCatalogWriteBarrier(func() error {
			return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
				return tx.Create(&Option{Key: "ordinary-role"}).Error
			})
		})
	}
	require.NoError(t, run(), "ordinary application table permissions must suffice")
	require.NoError(t, db.Exec("RESET ROLE").Error)
	require.NoError(t, db.Exec("REVOKE UPDATE, DELETE ON channels FROM "+role).Error)
	require.NoError(t, db.Exec("SET ROLE "+role).Error)
	err = run()
	require.ErrorContains(t, err, "permission denied", "SELECT-only channels cannot establish a DML fence")
}

func TestCatalogSyncReferenceDeadlockRollback(t *testing.T) {
	for _, engine := range []string{"mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			require.NoError(t, db.Create(&Channel{Id: 10, Key: "fixture", Status: 2}).Error)
			writer := db.Begin()
			require.NoError(t, writer.Error)
			defer writer.Rollback()
			require.NoError(t, writer.Exec("UPDATE channels SET status = 1 WHERE id = 10").Error)
			finished := make(chan error, 1)
			go func() {
				finished <- WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
						return tx.Create(&Option{Key: "deadlock-winner"}).Error
					})
				})
			}()
			catalogWaitForDBLock(t, db, engine, finished)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			writerErr := writer.WithContext(ctx).Exec("UPDATE marketplace_order_locks SET version = version + 1 WHERE name = ?", marketplaceOrderLockName).Error
			_ = writer.Rollback().Error
			rootErr := <-finished
			combined := errors.Join(writerErr, rootErr)
			require.Error(t, combined)
			require.Contains(t, strings.ToLower(combined.Error()), "deadlock", "exercise an actual cross-table deadlock, not only a timeout")
			var count int64
			require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", "deadlock-winner").Count(&count).Error)
			if rootErr != nil {
				assert.Zero(t, count, "deadlock victim must fully roll back")
			} else {
				assert.Equal(t, int64(1), count)
			}
			require.NoError(t, db.Create(&Channel{Key: "after-deadlock"}).Error)
		})
	}
}

func TestCatalogSyncReferenceRealCommitFailure(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			var reader *gorm.DB
			if engine == "sqlite" {
				// A real SHARED read lock prevents the RESERVED writer's COMMIT
				// upgrade in rollback-journal mode, even after the callback succeeds.
				reader = db.Begin()
				require.NoError(t, reader.Error)
				defer reader.Rollback()
				var count int64
				require.NoError(t, reader.Model(&Option{}).Count(&count).Error)
			}
			if engine == "postgres" {
				require.NoError(t, db.Exec("CREATE TABLE fence_parent (id bigint PRIMARY KEY)").Error)
				require.NoError(t, db.Exec("CREATE TABLE fence_child (id bigint PRIMARY KEY, parent_id bigint REFERENCES fence_parent(id) DEFERRABLE INITIALLY DEFERRED)").Error)
			}
			err := WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					if err := tx.Create(&Option{Key: "failed-commit"}).Error; err != nil {
						return err
					}
					if engine == "postgres" {
						return tx.Exec("INSERT INTO fence_child (id, parent_id) VALUES (1, 999)").Error
					}
					if engine == "mysql" {
						var id int64
						require.NoError(t, tx.Raw("SELECT CONNECTION_ID()").Scan(&id).Error)
						return db.Exec(fmt.Sprintf("KILL CONNECTION %d", id)).Error
					}
					return nil
				})
			})
			require.ErrorIs(t, err, ErrCatalogCommitUncertain)
			if reader != nil {
				require.NoError(t, reader.Rollback().Error)
			}
			var count int64
			require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", "failed-commit").Count(&count).Error)
			assert.Zero(t, count)
			registry.SetEnabled(false) // no registry lease may survive COMMIT error
			require.NoError(t, db.Create(&Option{Key: "after-commit-error"}).Error)
		})
	}
}

func TestCatalogSyncReferenceRegistryBusy(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			go func() {
				done <- registry.SetGenerationPreparer(func(candidate, current *jsplugin.RoutingGeneration) (jsplugin.PreparedRoutingGeneration, error) {
					close(entered)
					<-release
					return jsplugin.PreparedRoutingGeneration{}, errors.New("test publication stopped")
				})
			}()
			<-entered
			err := WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
					t.Error("busy registry admitted business callback")
					return nil
				})
			})
			close(release)
			require.Error(t, <-done)
			require.ErrorIs(t, err, jsplugin.ErrGenerationBusy)
			require.NoError(t, db.Create(&Option{Key: "after-busy"}).Error)
		})
	}
}

func TestCatalogSyncReferenceMySQLVersionGate(t *testing.T) {
	for _, version := range []string{"5.7.8", "5.7.43", "8.0.45", "8.4.10", "8.2.0", "9.0.0", "8.4.11-MariaDB", "8.4.bad"} {
		require.Error(t, catalogMySQLFenceVersion(version, "MySQL Community Server - GPL"), version)
	}
	for _, variant := range []string{"MariaDB Server", "TiDB", "Percona Server", "unknown"} {
		require.Error(t, catalogMySQLFenceVersion("8.4.11", variant), variant)
	}
}

func TestCatalogSyncReferencePostgresSearchPath(t *testing.T) {
	db := catalogFenceTestDB(t, "postgres")
	require.NoError(t, db.Exec(`CREATE SCHEMA "fence schema"`).Error)
	for _, model := range []any{&marketplaceOrderLock{}, &Model{}, &Vendor{}, &Option{}, &Channel{}, &Ability{}, &Task{}, &TaskPlugin{}, &Midjourney{}, &SystemTask{}, &CatalogSyncState{}, &CatalogSyncPlan{}, &CatalogSyncBaseline{}, &CatalogSyncOperation{}} {
		stmt := &gorm.Statement{DB: db}
		require.NoError(t, stmt.Parse(model))
		require.NoError(t, db.Exec("ALTER TABLE "+stmt.Quote(stmt.Schema.Table)+` SET SCHEMA "fence schema"`).Error)
	}
	require.NoError(t, db.Exec(`SET search_path TO "fence schema"`).Error)
	pool, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, WithCatalogWriteBarrier(func() error {
		return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
			catalogAssertWriterBlocked(t, pool, "postgres", `INSERT INTO "fence schema".channels (key, molii_grok_management_access_token) VALUES ('fixture', '')`)
			return tx.Create(&Option{Key: "schema-bound"}).Error
		})
	}))
	var count int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM "fence schema".options WHERE key = 'schema-bound'`).Scan(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCatalogSyncReferenceSQLiteBusyBound(t *testing.T) {
	db := catalogFenceTestDB(t, "sqlite")
	pool, err := db.DB()
	require.NoError(t, err)
	writer := db.Begin()
	require.NoError(t, writer.Error)
	defer writer.Rollback()
	require.NoError(t, writer.Exec("UPDATE marketplace_order_locks SET version = version + 1").Error)
	conn, err := pool.Conn(context.Background())
	require.NoError(t, err)
	_, err = conn.ExecContext(context.Background(), "PRAGMA busy_timeout = 60000")
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		finished <- catalogReferenceTransaction(ctx, db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
			return errors.New("unexpected callback")
		})
	}()
	select {
	case err := <-finished:
		require.ErrorContains(t, err, "locked")
	case <-time.After(2 * time.Second):
		t.Fatal("root inherited the unbounded busy wait")
	}
	require.NoError(t, writer.Rollback().Error)
	require.NoError(t, db.Create(&Option{Key: "after-sqlite-busy"}).Error)
}

func TestCatalogSyncReferencePostgresSearchPathShadow(t *testing.T) {
	db := catalogFenceTestDB(t, "postgres")
	require.NoError(t, db.Create(&Option{Key: "proof", Value: "fenced"}).Error)
	require.NoError(t, db.Exec("CREATE SCHEMA fence_shadow").Error)
	require.NoError(t, db.Exec("SET search_path TO fence_shadow, public").Error)
	created := false
	require.NoError(t, db.Callback().Raw().After("gorm:raw").Register("shadow-search-path", func(tx *gorm.DB) {
		query := tx.Statement.SQL.String()
		if !created && strings.HasPrefix(query, "LOCK TABLE") && strings.Contains(query, "options") {
			created = true
			require.NoError(t, db.Exec("CREATE TABLE fence_shadow.options (key text PRIMARY KEY, value text)").Error)
			require.NoError(t, db.Exec("INSERT INTO fence_shadow.options (key, value) VALUES ('proof', 'unfenced')").Error)
		}
	}))
	defer db.Callback().Raw().Remove("shadow-search-path")
	err := WithCatalogWriteBarrier(func() error {
		return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
			var option Option
			if err := tx.First(&option, "key = ?", "proof").Error; err != nil {
				return err
			}
			if option.Value != "fenced" {
				return errors.New("plain read followed an unfenced search_path shadow")
			}
			return nil
		})
	})
	require.NoError(t, err)
	assert.True(t, created)
}

func TestCatalogSyncReferencePostgresUnsupportedNamespaces(t *testing.T) {
	for _, layout := range []string{"temporary", "mixed"} {
		t.Run(layout, func(t *testing.T) {
			db := catalogFenceTestDB(t, "postgres")
			pool, err := db.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(1)
			if layout == "temporary" {
				require.NoError(t, db.Exec("CREATE TEMPORARY TABLE options (key text PRIMARY KEY, value text)").Error)
			} else {
				require.NoError(t, db.Exec("CREATE SCHEMA mixed_fence").Error)
				require.NoError(t, db.Exec("ALTER TABLE options SET SCHEMA mixed_fence").Error)
				require.NoError(t, db.Exec("SET search_path TO mixed_fence, public").Error)
			}
			err = catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
				t.Error("unsupported namespace admitted")
				return nil
			})
			require.Error(t, err)
			if layout == "mixed" {
				require.ErrorContains(t, err, "mixed unqualified")
			}
		})
	}
}

func TestCatalogSyncReferenceUnsupportedTableLayouts(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			switch engine {
			case "postgres":
				require.NoError(t, db.Exec("ALTER TABLE channels ENABLE ROW LEVEL SECURITY").Error)
				require.NoError(t, db.Exec("CREATE POLICY hidden_catalog_references ON channels USING (false)").Error)
			case "mysql":
				require.NoError(t, db.Exec("ALTER TABLE channels PARTITION BY HASH(id) PARTITIONS 2").Error)
			case "sqlite":
				require.NoError(t, db.Migrator().DropTable(&Channel{}))
				require.NoError(t, db.Exec("CREATE VIRTUAL TABLE channels USING fts5(id)").Error)
			}
			callback := func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
				t.Error("unsupported physical layout reached callback")
				return nil
			}
			require.Error(t, catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), callback))
			if engine == "postgres" {
				require.NoError(t, db.Exec("ALTER TABLE channels DISABLE ROW LEVEL SECURITY").Error)
				require.NoError(t, db.Exec("ALTER TABLE channels FORCE ROW LEVEL SECURITY").Error)
				require.Error(t, catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), callback))
				require.NoError(t, db.Migrator().DropTable(&Channel{}))
				require.NoError(t, db.Exec("CREATE TABLE channels (id bigint) PARTITION BY HASH(id)").Error)
				require.Error(t, catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), callback))
				require.NoError(t, db.Migrator().DropTable(&Channel{}))
				require.NoError(t, db.Exec("CREATE TABLE channel_family (id bigint) PARTITION BY HASH(id)").Error)
				require.NoError(t, db.Exec("CREATE TABLE channels PARTITION OF channel_family FOR VALUES WITH (MODULUS 1, REMAINDER 0)").Error)
				require.Error(t, catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), callback))
			}
		})
	}
}

func TestCatalogSyncReferenceSQLiteAttachedAndTemporary(t *testing.T) {
	for _, layout := range []string{"attached", "temporary"} {
		t.Run(layout, func(t *testing.T) {
			db := catalogFenceTestDB(t, "sqlite")
			pool, err := db.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(1)
			if layout == "attached" {
				require.NoError(t, db.Exec("ATTACH DATABASE ? AS extra", filepath.Join(t.TempDir(), "extra.db")).Error)
				require.NoError(t, db.Migrator().DropTable(&Channel{}))
				require.NoError(t, db.Exec("CREATE TABLE extra.channels (id integer PRIMARY KEY)").Error)
			} else {
				require.NoError(t, db.Exec("CREATE TEMPORARY TABLE channels (id integer PRIMARY KEY)").Error)
			}
			err = catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
				t.Error("references resolved outside the fenced main database")
				return nil
			})
			require.Error(t, err)
		})
	}
}
