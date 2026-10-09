package model

import (
	"context"
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
