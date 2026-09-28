package model

import (
	"context"
	"database/sql/driver"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	gosqlite "github.com/glebarez/go-sqlite"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type mysqlNamedSQLiteDialector struct{ gorm.Dialector }

func (mysqlNamedSQLiteDialector) Name() string { return "mysql" }

var (
	registerOptionLockFunctionsOnce sync.Once
	registerOptionLockFunctionsErr  error
)

func TestOptionPrimaryKeyMigrationSQLiteRepairsLegacyTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "options.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	require.NoError(t, db.Exec(`CREATE TABLE options (key text, value text)`).Error)
	legacyRows := []Option{
		{Key: "alpha", Value: "one"},
		{Key: "duplicate", Value: "beta"},
		{Key: "duplicate", Value: "beta"},
		{Key: "", Value: "cannot become a primary key"},
	}
	for _, row := range legacyRows {
		require.NoError(t, db.Exec(
			`INSERT INTO options (key, value) VALUES (?, ?)`, row.Key, row.Value,
		).Error)
	}

	unique, err := optionsKeyIsUnique(db)
	require.NoError(t, err)
	require.False(t, unique, "the fixture must represent a legacy table without key uniqueness")

	require.NoError(t, migrateOptionPrimaryKey(db))

	unique, err = optionsKeyIsUnique(db)
	require.NoError(t, err)
	require.True(t, unique)

	var repaired []Option
	require.NoError(t, db.Order("key").Find(&repaired).Error)
	require.Equal(t, []Option{
		{Key: "alpha", Value: "one"},
		{Key: "duplicate", Value: "beta"},
	}, repaired)

	legacyTables := optionPrimaryKeyLegacyTables(t, db)
	require.Len(t, legacyTables, 1)
	var backedUp []Option
	require.NoError(t, db.Table(legacyTables[0]).Find(&backedUp).Error)
	require.ElementsMatch(t, legacyRows, backedUp, "the backup must retain every original row")

	err = db.Exec(`INSERT INTO options (key, value) VALUES (?, ?)`, "alpha", "two").Error
	require.Error(t, err, "the repaired table must reject duplicate keys")

	require.NoError(t, migrateOptionPrimaryKey(db))
	require.Equal(t, legacyTables, optionPrimaryKeyLegacyTables(t, db), "an idempotent run must not create another backup")

	var afterSecondRun []Option
	require.NoError(t, db.Order("key").Find(&afterSecondRun).Error)
	require.Equal(t, repaired, afterSecondRun, "an idempotent run must not lose or change data")
}

func TestOptionPrimaryKeyDedupeIsIndependentOfDatabaseRowOrder(t *testing.T) {
	forward := []Option{
		{Key: "duplicate", Value: "same"},
		{Key: "other", Value: "preserved"},
		{Key: "duplicate", Value: "same"},
		{Key: "", Value: "skipped"},
	}
	reverse := []Option{forward[3], forward[2], forward[1], forward[0]}

	deduped, skippedEmpty, err := dedupeOptionRows(forward)
	require.NoError(t, err)
	reversed, reversedSkippedEmpty, err := dedupeOptionRows(reverse)
	require.NoError(t, err)

	require.Equal(t, 1, skippedEmpty)
	require.Equal(t, skippedEmpty, reversedSkippedEmpty)
	require.Equal(t, []Option{
		{Key: "duplicate", Value: "same"},
		{Key: "other", Value: "preserved"},
	}, deduped)
	require.Equal(t, deduped, reversed)
}

func TestOptionPrimaryKeyMigrationRejectsConflictingDuplicateValues(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "conflicting-options.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	require.NoError(t, db.Exec(`CREATE TABLE options (key text, value text)`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO options (key, value) VALUES (?, ?), (?, ?), (?, ?)`,
		"conflict", "alpha", "conflict", "omega", "safe", "value",
	).Error)

	err = migrateOptionPrimaryKey(db)
	require.ErrorContains(t, err, `options key "conflict" has conflicting values`)

	unique, inspectErr := optionsKeyIsUnique(db)
	require.NoError(t, inspectErr)
	require.False(t, unique, "a failed migration must leave the original table in place")
	var rows []Option
	require.NoError(t, db.Order("key, value").Find(&rows).Error)
	require.Equal(t, []Option{
		{Key: "conflict", Value: "alpha"},
		{Key: "conflict", Value: "omega"},
		{Key: "safe", Value: "value"},
	}, rows)
	require.Empty(t, optionPrimaryKeyLegacyTables(t, db), "failure before swap must not create a backup table")
}

func TestWithOptionPrimaryKeyLockPinsMySQLSessionToOneConnection(t *testing.T) {
	registerOptionLockFunctionsOnce.Do(func() {
		lockFunction := func(_ *gosqlite.FunctionContext, _ []driver.Value) (driver.Value, error) {
			return int64(1), nil
		}
		registerOptionLockFunctionsErr = gosqlite.RegisterScalarFunction("get_lock", 2, lockFunction)
		if registerOptionLockFunctionsErr == nil {
			registerOptionLockFunctionsErr = gosqlite.RegisterScalarFunction("release_lock", 1, lockFunction)
		}
	})
	require.NoError(t, registerOptionLockFunctionsErr)

	dialector := mysqlNamedSQLiteDialector{Dialector: sqlite.Open(filepath.Join(t.TempDir(), "mysql-lock.db"))}
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = withOptionPrimaryKeyLock(db.WithContext(ctx), func(locked *gorm.DB) error {
		var one int
		return locked.Raw("SELECT 1").Scan(&one).Error
	})
	require.NoError(t, err)
}

func TestOptionPrimaryKeyMigrationMySQLWithOneConnection(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	if db.Migrator().HasTable(&Option{}) {
		t.Skip("TEST_MYSQL_DSN already contains options; refusing to replace it")
	}
	legacyBefore := make(map[string]struct{})
	for _, table := range optionPrimaryKeyMySQLLegacyTables(t, db) {
		legacyBefore[table] = struct{}{}
	}
	require.NoError(t, db.Exec("CREATE TABLE options (`key` varchar(191), `value` text)").Error)
	t.Cleanup(func() {
		sqlDB.SetMaxOpenConns(2)
		for _, table := range optionPrimaryKeyMySQLLegacyTables(t, db) {
			if _, existed := legacyBefore[table]; !existed {
				require.NoError(t, db.Migrator().DropTable(table))
			}
		}
		require.NoError(t, db.Migrator().DropTable("options", optionPrimaryKeyTmpTable))
	})
	require.NoError(t, db.Exec("INSERT INTO options (`key`, `value`) VALUES (?, ?), (?, ?)",
		"alpha", "one", "alpha", "one").Error)

	sqlDB.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, migrateOptionPrimaryKey(db.WithContext(ctx)))

	unique, err := optionsKeyIsUnique(db)
	require.NoError(t, err)
	require.True(t, unique)
}

func TestInitDBMasterReturnsOptionPrimaryKeyMigrationFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "master-invalid-options.db")
	fixture, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, fixture.Exec("CREATE TABLE options (`key` text, `value` text)").Error)
	require.NoError(t, fixture.Exec("CREATE VIEW options_pk_tmp AS SELECT `key`, `value` FROM options").Error)
	fixtureSQL, err := fixture.DB()
	require.NoError(t, err)
	require.NoError(t, fixtureSQL.Close())

	restoreInitDBTestGlobals(t, path, true)
	err = InitDB()
	require.ErrorContains(t, err, "create options_pk_tmp")
}

func TestInitDBNonMasterRequiresOptionKeyUniqueness(t *testing.T) {
	tests := []struct {
		name          string
		createOptions string
		wantError     string
	}{
		{name: "missing_table", wantError: "options table is missing"},
		{name: "legacy_table", createOptions: "CREATE TABLE options (`key` text, `value` text)", wantError: "options.key is not unique"},
		{name: "current_table", createOptions: "CREATE TABLE options (`key` text PRIMARY KEY, `value` text)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "non-master-options.db")
			if test.createOptions != "" {
				fixture, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
				require.NoError(t, err)
				require.NoError(t, fixture.Exec(test.createOptions).Error)
				fixtureSQL, err := fixture.DB()
				require.NoError(t, err)
				require.NoError(t, fixtureSQL.Close())
			}

			restoreInitDBTestGlobals(t, path, false)
			err := InitDB()
			if test.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantError)
		})
	}
}

func optionPrimaryKeyLegacyTables(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name GLOB ? ORDER BY name`,
		optionLegacyTablePrefix+"*",
	).Scan(&tables).Error)
	return tables
}

func optionPrimaryKeyMySQLLegacyTables(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw(
		"SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name LIKE ? ORDER BY table_name",
		optionLegacyTablePrefix+"%",
	).Scan(&tables).Error)
	return tables
}

func restoreInitDBTestGlobals(t *testing.T, sqlitePath string, master bool) {
	t.Helper()
	previousDB := DB
	previousSQLitePath := common.SQLitePath
	previousMaster := common.IsMasterNode
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	t.Setenv("SQL_DSN", "local")
	t.Setenv("LOG_SQL_DSN", "")
	common.SQLitePath = sqlitePath
	common.IsMasterNode = master
	t.Cleanup(func() {
		if DB != nil && DB != previousDB {
			if sqlDB, err := DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		DB = previousDB
		common.SQLitePath = previousSQLitePath
		common.IsMasterNode = previousMaster
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
}
