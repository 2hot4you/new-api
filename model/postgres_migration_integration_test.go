package model

import (
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestPostgresFullSchemaMigrationIsIdempotent(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("FULL_MIGRATION_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("FULL_MIGRATION_POSTGRES_TEST_DSN is not configured")
	}

	scopedDB, databaseType, err := chooseDB("FULL_MIGRATION_POSTGRES_TEST_DSN", false)
	require.NoError(t, err)
	require.Equal(t, common.DatabaseTypePostgreSQL, databaseType)
	require.IsType(t, postgresMigrationDialector{}, scopedDB.Dialector)
	scopedSQL, err := scopedDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, scopedSQL.Close()) })

	previousDB, previousLogDB := DB, LOG_DB
	previousMaster := common.IsMasterNode
	common.IsMasterNode = true
	t.Setenv("LOG_SQL_DSN", "")
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	DB, LOG_DB = scopedDB, scopedDB
	common.SetMainDatabaseType(common.DatabaseTypePostgreSQL)
	common.SetLogDatabaseType(common.DatabaseTypePostgreSQL)
	initCol()
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.IsMasterNode = previousMaster
		common.SetMainDatabaseType(previousMainType)
		common.SetLogDatabaseType(previousLogType)
		initCol()
	})

	require.NoError(t, migrateDB())
	require.NoError(t, InitLogDB())
	require.True(t, LOG_DB.Migrator().HasTable(&AuditLog{}))
	require.True(t, DB.Migrator().HasTable(&TaskBillingJob{}))
	require.True(t, DB.Migrator().HasTable(&MoliiFile{}))
	require.NoError(t, ensureUserQuotaColumns(scopedDB, common.DatabaseTypePostgreSQL))

	var firstTableCount int64
	require.NoError(t, scopedDB.Raw(`
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = 'public'`).Scan(&firstTableCount).Error)
	require.Greater(t, firstTableCount, int64(20))

	seedToken := Token{UserId: 1, Key: "rc30-preserved-key", Name: "preserved token"}
	require.NoError(t, scopedDB.Create(&seedToken).Error)
	seedGroup := PrefillGroup{
		Name:        "rc30-preserved-group",
		Type:        "model",
		Items:       JSONValue(`["gpt-test"]`),
		Description: "preserved group",
	}
	require.NoError(t, scopedDB.Create(&seedGroup).Error)

	require.NoError(t, migrateDB())
	require.NoError(t, InitLogDB())
	require.True(t, LOG_DB.Migrator().HasTable(&AuditLog{}))
	require.True(t, DB.Migrator().HasTable(&TaskBillingJob{}))
	require.True(t, DB.Migrator().HasTable(&MoliiFile{}))
	require.NoError(t, ensureUserQuotaColumns(scopedDB, common.DatabaseTypePostgreSQL))

	var secondTableCount int64
	require.NoError(t, scopedDB.Raw(`
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = 'public'`).Scan(&secondTableCount).Error)
	require.Equal(t, firstTableCount, secondTableCount)

	var preservedToken Token
	require.NoError(t, scopedDB.First(&preservedToken, seedToken.Id).Error)
	require.Equal(t, seedToken.Key, preservedToken.Key)
	require.Equal(t, seedToken.Name, preservedToken.Name)
	var preservedGroup PrefillGroup
	require.NoError(t, scopedDB.First(&preservedGroup, seedGroup.Id).Error)
	require.Equal(t, seedGroup.Name, preservedGroup.Name)
	require.Equal(t, seedGroup.Description, preservedGroup.Description)
}
