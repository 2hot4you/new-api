package model

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This entry point never initializes a shared database. Each selected feature
// test owns a new PostgreSQL database; a missing fixture therefore fails closed.
func runCatalogPostgresTests(m *testing.M) int {
	if os.Getenv("CATALOG_SYNC_POSTGRES_ONLY") != "1" {
		fmt.Fprintln(os.Stderr, "CATALOG_SYNC_POSTGRES_ONLY must be exactly 1 when set")
		return 1
	}
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if catalogPostgresDSN(dsn) != nil {
		fmt.Fprintln(os.Stderr, "catalog PostgreSQL gate requires a loopback TEST_POSTGRES_DSN URL with sslmode=disable")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "catalog PostgreSQL preflight connection failed")
		return 1
	}
	err = conn.Ping(ctx)
	closeErr := conn.Close(ctx)
	if err != nil || closeErr != nil {
		fmt.Fprintln(os.Stderr, "catalog PostgreSQL preflight failed")
		return 1
	}
	DB, LOG_DB = nil, nil
	common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypePostgreSQL)
	common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, true
	initCol()
	return m.Run()
}

// Real LOGIN role, no superuser/role/database creation privileges. Provisioning
// uses the task administrator; the measured transaction uses the restricted DSN.
func TestCatalogPostgresHistoryLoadAndRollback(t *testing.T) {
	db := catalogFenceTestDB(t, "postgres")
	const rows = 10000
	for _, statement := range []string{
		`INSERT INTO tasks (task_id,status,properties,private_data) SELECT 'history-'||n,'SUCCESS','{"origin_model_name":"history-model"}','{}' FROM generate_series(1,10000) n`,
		`INSERT INTO midjourneys (mj_id,status) SELECT 'history-'||n,'SUCCESS' FROM generate_series(1,10000) n`,
		`INSERT INTO system_tasks (task_id,status) SELECT 'history-'||n,'succeeded' FROM generate_series(1,10000) n`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	role := fmt.Sprintf("catalog_load_%d", time.Now().UnixNano())
	require.NoError(t, db.Exec("CREATE ROLE "+role+" LOGIN PASSWORD 'catalog-load-test-only' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT").Error)
	t.Cleanup(func() {
		require.NoError(t, db.Exec("DROP OWNED BY "+role).Error)
		require.NoError(t, db.Exec("DROP ROLE "+role).Error)
	})
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA public TO "+role).Error)
	require.NoError(t, db.Exec("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO "+role).Error)
	require.NoError(t, db.Exec("GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO "+role).Error)
	var database string
	require.NoError(t, db.Raw("SELECT current_database()").Scan(&database).Error)
	u, err := url.Parse(os.Getenv("TEST_POSTGRES_DSN"))
	require.NoError(t, err)
	u.Path, u.User = "/"+database, url.UserPassword(role, "catalog-load-test-only")
	limited, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := limited.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	var privileged bool
	require.NoError(t, limited.Raw("SELECT rolsuper OR rolcreatedb OR rolcreaterole FROM pg_roles WHERE rolname = current_user").Scan(&privileged).Error)
	require.False(t, privileged)
	registry := jsplugin.NewRegistry()
	start := time.Now()
	err = WithCatalogWriteBarrier(func() error {
		return catalogReferenceTransaction(context.Background(), limited, registry, func(tx *gorm.DB, _ *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			_, err := captureCatalogReferencesTx(tx, pin)
			return err
		})
	})
	require.NoError(t, err)
	t.Logf("restricted PostgreSQL full reference scan: tasks=%d midjourneys=%d system_tasks=%d elapsed=%s", rows, rows, rows, time.Since(start))
	// Include nonterminal historical payload decoding as well as terminal rows.
	require.NoError(t, db.Exec("UPDATE tasks SET status = 'IN_PROGRESS' WHERE id % 2 = 0").Error)
	require.NoError(t, db.Exec("UPDATE midjourneys SET status = 'IN_PROGRESS' WHERE id % 2 = 0").Error)
	require.NoError(t, db.Exec("UPDATE system_tasks SET status = 'pending' WHERE id % 2 = 0").Error)
	start = time.Now()
	require.NoError(t, WithCatalogWriteBarrier(func() error {
		return catalogReferenceTransaction(context.Background(), limited, registry, func(tx *gorm.DB, _ *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			view, err := captureCatalogReferencesTx(tx, pin)
			if err == nil {
				assert.Len(t, view.Tasks, rows)
				assert.True(t, view.Scheduled)
			}
			return err
		})
	}))
	t.Logf("mixed history scan: total=%d active=%d elapsed=%s", rows*3, rows*3/2, time.Since(start))
	// Timeout after an actual write and full history scan must roll back the
	// complete root transaction and release SQL fences and the registry pin.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start = time.Now()
	entered := false
	err = WithCatalogWriteBarrier(func() error {
		return catalogReferenceTransaction(ctx, limited, registry, func(tx *gorm.DB, _ *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			if err := tx.Create(&Option{Key: "load-rollback", Value: "must-not-commit"}).Error; err != nil {
				return err
			}
			if _, err := captureCatalogReferencesTx(tx, pin); err != nil {
				return err
			}
			entered = true
			return tx.Exec("SELECT pg_sleep(5)").Error
		})
	})
	require.True(t, entered, "timeout must occur after the complete measured scan")
	require.Error(t, err)
	t.Logf("history scan + forced SQL timeout rolled back after %s: %v", time.Since(start), err)
	var count int64
	require.NoError(t, db.Model(&Option{}).Where("key = ?", "load-rollback").Count(&count).Error)
	assert.Zero(t, count)
	for _, table := range []any{&Task{}, &Midjourney{}, &SystemTask{}} {
		require.NoError(t, db.Model(table).Count(&count).Error)
		assert.EqualValues(t, rows, count)
	}
	require.NoError(t, limited.Create(&Option{Key: "after-load-timeout"}).Error)
	require.NoError(t, WithCatalogWriteBarrier(func() error {
		return catalogReferenceTransaction(context.Background(), limited, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
			return tx.Create(&Option{Key: "after-load-fence"}).Error
		})
	}))
	registry.SetEnabled(false)
	assertRuntimeLocksReleased(t)
}

func catalogPostgresDSN(dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.User == nil || u.User.Username() == "" || strings.Trim(u.Path, "/") == "" || u.Fragment != "" || u.Opaque != "" {
		return errors.New("invalid task PostgreSQL URL")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(q) != 1 || len(q["sslmode"]) != 1 || q.Get("sslmode") != "disable" {
		return errors.New("unsupported task PostgreSQL connection options")
	}
	return nil
}

func TestCatalogPostgresDSNGuard(t *testing.T) {
	for _, dsn := range []string{"", "host=127.0.0.1 dbname=postgres", "postgresql://u@localhost/db?sslmode=disable", "postgresql://u@example.com/db?sslmode=disable", "postgresql://u@127.0.0.1/?sslmode=disable", "postgresql://u@127.0.0.1/db?sslmode=disable&host=example.com", "postgresql://u@127.0.0.1/db?sslmode=disable&service=remote", "postgresql://u@127.0.0.1/db?sslmode=disable&port=5433", "postgresql://u@127.0.0.1/db?sslmode=disable&sslmode=require"} {
		require.Error(t, catalogPostgresDSN(dsn))
	}
	require.NoError(t, catalogPostgresDSN("postgresql://u:p@127.0.0.1:5432/task?sslmode=disable"))
	require.NoError(t, catalogPostgresDSN("postgresql://u:p@[::1]:5432/task?sslmode=disable"))
}

// These older feature fixtures predate the singleton/recovery contract. Keep
// the adaptation explicit and local: no users, sessions or ready markers are
// fabricated, and negative authorization tests retain their original actors.
func catalogPostgresLegacyPrerequisite(t *testing.T, db *gorm.DB, point string) {
	t.Helper()
	if os.Getenv("CATALOG_SYNC_POSTGRES_ONLY") != "1" {
		return
	}
	name := strings.Split(t.Name(), "/")[0]
	selected := false
	switch point {
	case "actor":
		selected = strings.HasPrefix(name, "TestCatalogSyncRestore") || strings.HasPrefix(name, "TestCatalogSyncBusiness") || name == "TestCatalogSyncHistoryPagination" || (strings.HasPrefix(name, "TestCatalogRuntime") && name != "TestCatalogRuntimeOrdinaryMissingPrerequisites")
	case "fence":
		selected = name == "TestCatalogSyncStore" || name == "TestCatalogSyncStoreBaselineIncarnation" || name == "TestCatalogSyncBusinessPreview"
	case "ordinary":
		selected = name == "TestCatalogSyncStoreOptionWritersAndNoop" || name == "TestCatalogSyncSourceConcurrentMutation"
	}
	if !selected {
		return
	}
	preserveOrdinaryCatalogRuntime(t)
	preserveCatalogCandidate(t)
	if point != "actor" {
		prior := jsplugin.DefaultRegistry
		jsplugin.DefaultRegistry = jsplugin.NewRegistry()
		t.Cleanup(func() { jsplugin.DefaultRegistry = prior })
	}
	if point == "ordinary" {
		require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}, &Channel{}, &Ability{}, &Task{}, &TaskPlugin{}, &Midjourney{}, &SystemTask{}))
		require.NoError(t, MigrateCatalogSync(db))
	}
	require.NoError(t, db.AutoMigrate(&SystemInstance{}))
	previous, master, started := common.GetNodeIdentity(), common.IsMasterNode, common.StartTime
	t.Cleanup(func() {
		common.NodeName, common.NodeNameSource, common.NodeNameManuallyConfigured = previous.Name, previous.Source, previous.ManuallyConfigured
		common.IsMasterNode, common.StartTime = master, started
	})
	common.NodeName, common.NodeNameSource, common.NodeNameManuallyConfigured = "catalog-test", common.NodeNameSourceManual, true
	common.IsMasterNode, common.StartTime = true, time.Now().Unix()-10
	t.Setenv("CATALOG_SYNC_SINGLE_INSTANCE", "true")
	catalogStartupHeartbeat(t)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
}

// Only these audited engine-neutral legacy feature cases may remap their
// historical SQLite fixture label. Engine-specific cases must never enter it.
func catalogPostgresFixtureEngine(t *testing.T, engine string) string {
	t.Helper()
	if os.Getenv("CATALOG_SYNC_POSTGRES_ONLY") != "1" {
		return engine
	}
	require.NoError(t, catalogPostgresDSN(os.Getenv("TEST_POSTGRES_DSN")))
	if engine == "postgres" {
		return engine
	}
	name := strings.Split(t.Name(), "/")[0]
	neutral := map[string]bool{
		"TestCatalogSyncValidationPreservation":              true,
		"TestCatalogSyncValidationRuntimeStates":             true,
		"TestCatalogSyncRefreshExcludesWriter":               true,
		"TestCatalogRuntimeOrdinaryPreparationBudget":        true,
		"TestCatalogRuntimeOrdinaryMissingPrerequisites":     true,
		"TestCatalogRuntimeOrdinaryForeignRoot":              true,
		"TestCatalogRuntimeResetAndHistoricalOrphan":         true,
		"TestCatalogRuntimeReadyRecoveryFailureBlocksWrites": true,
		"TestCatalogRuntimeCacheContention":                  true,
		"TestCatalogRuntimeQueryHelperSemantics":             true,
		"TestCatalogRuntimeRejectsInvalidCurrency":           true,
		"TestCatalogRuntimeCompatibilityAndCacheRemoval":     true,
		"TestCatalogRuntimeRecheckAfterRebuild":              true,
		"TestCatalogRuntimeRejectsCorruptOperation":          true,
		"TestCatalogRuntimeUnknownReadyState":                true,
	}
	require.True(t, engine == "sqlite" && neutral[name], "unexpected non-PostgreSQL fixture in catalog gate: %s", t.Name())
	t.Log("audited engine-neutral fixture: PostgreSQL opt-in")
	return "postgres"
}
