package model

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestPostgresFullSchemaMigrationIsIdempotent(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("FULL_MIGRATION_POSTGRES_TEST_DSN"))
	if os.Getenv("CATALOG_SYNC_POSTGRES_ONLY") == "1" {
		dsn = os.Getenv("TEST_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("FULL_MIGRATION_POSTGRES_TEST_DSN is not configured")
	}
	require.NoError(t, catalogPostgresDSN(dsn), "startup fixtures require a disposable numeric loopback PostgreSQL URL")
	t.Setenv("FULL_MIGRATION_POSTGRES_TEST_DSN", dsn)
	admin, _, err := chooseDB("FULL_MIGRATION_POSTGRES_TEST_DSN", false)
	require.NoError(t, err)
	adminPool, err := admin.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, adminPool.Close()) })
	name := fmt.Sprintf("rc46_startup_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE DATABASE "+name).Error)
	t.Cleanup(func() { require.NoError(t, admin.Exec("DROP DATABASE "+name).Error) })
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	parsed.Path = "/" + name
	t.Setenv("FULL_MIGRATION_POSTGRES_TEST_DSN", parsed.String())

	scopedDB, databaseType, err := chooseDB("FULL_MIGRATION_POSTGRES_TEST_DSN", false)
	require.NoError(t, err)
	require.Equal(t, common.DatabaseTypePostgreSQL, databaseType)
	require.IsType(t, postgresMigrationDialector{}, scopedDB.Dialector)
	scopedSQL, err := scopedDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, scopedSQL.Close()) })

	previousDB, previousLogDB := DB, LOG_DB
	previousRetireAt := legacyAccessTokenRetireAt.Load()
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
		legacyAccessTokenRetireAt.Store(previousRetireAt)
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
	require.True(t, DB.Migrator().HasTable(&UserAccessToken{}))
	require.True(t, DB.Migrator().HasColumn(&QuotaData{}, "cache_read_tokens"))
	require.True(t, DB.Migrator().HasColumn(&QuotaData{}, "cache_creation_tokens"))
	require.True(t, LOG_DB.Migrator().HasIndex(&AuditLog{}, "idx_audit_time_event"))
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
	legacyToken := "0123456789abcdef0123456789abcdef"
	seedUser := User{Username: "rc46-preserved-user", Password: "preserved-hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AuthVersion: 1, Quota: 1234567, AccessToken: &legacyToken, AffCode: "rc46-preserved-aff"}
	require.NoError(t, scopedDB.Create(&seedUser).Error)
	seedUsage := QuotaData{UserID: seedUser.Id, ModelName: "preserved-model", Quota: 9876, Count: 3}
	require.NoError(t, scopedDB.Create(&seedUsage).Error)
	// Recreate the rc.40 absence of the upstream rc.46 schema additions while
	// retaining populated custom tables and the legacy dashboard credential.
	require.NoError(t, scopedDB.Migrator().DropTable(&UserAccessToken{}))
	require.NoError(t, scopedDB.Migrator().DropColumn(&QuotaData{}, "cache_read_tokens"))
	require.NoError(t, scopedDB.Migrator().DropColumn(&QuotaData{}, "cache_creation_tokens"))
	require.NoError(t, scopedDB.Where("key = ?", legacyAccessTokenRetireAtKey).Delete(&Option{}).Error)
	require.NoError(t, LOG_DB.Migrator().DropIndex(&AuditLog{}, "idx_audit_time_event"))

	require.NoError(t, migrateDB())
	require.NoError(t, InitLogDB())
	retireAt := LegacyAccessTokenRetireAt()
	require.Greater(t, retireAt, time.Now().Unix()+29*24*60*60)
	pat := UserAccessToken{UserId: seedUser.Id, Name: "retained scoped token", TokenHash: AccessTokenFingerprint("nap_migration-only-plaintext"), ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, pat.SetScopes([]string{"profile:read"}))
	require.NoError(t, scopedDB.Create(&pat).Error)
	// A second upgrade startup must preserve the deadline and the scoped token.
	require.NoError(t, migrateDB())
	require.NoError(t, InitLogDB())
	require.Equal(t, retireAt, LegacyAccessTokenRetireAt())
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
	var preservedUser User
	require.NoError(t, scopedDB.First(&preservedUser, seedUser.Id).Error)
	require.Equal(t, seedUser.Quota, preservedUser.Quota)
	require.Equal(t, legacyToken, *preservedUser.AccessToken)
	var preservedUsage QuotaData
	require.NoError(t, scopedDB.First(&preservedUsage, seedUsage.Id).Error)
	require.Equal(t, seedUsage.Quota, preservedUsage.Quota)
	require.Zero(t, preservedUsage.CacheReadTokens)
	require.Zero(t, preservedUsage.CacheCreationTokens)
	var preservedPAT UserAccessToken
	require.NoError(t, scopedDB.First(&preservedPAT, pat.Id).Error)
	require.Equal(t, pat.TokenHash, preservedPAT.TokenHash)
	require.Equal(t, []string{"profile:read"}, preservedPAT.GetScopes())
	require.True(t, LOG_DB.Migrator().HasIndex(&AuditLog{}, "idx_audit_time_event"))

	// Independently configured logs must migrate on every startup as well.
	logName := name + "_log"
	require.NoError(t, admin.Exec("CREATE DATABASE "+logName).Error)
	t.Cleanup(func() { require.NoError(t, admin.Exec("DROP DATABASE "+logName).Error) })
	parsed.Path = "/" + logName
	t.Setenv("LOG_SQL_DSN", parsed.String())
	for startup := range 3 {
		require.NoError(t, InitLogDB())
		require.NotSame(t, DB, LOG_DB)
		require.True(t, LOG_DB.Migrator().HasIndex(&AuditLog{}, "idx_audit_time_event"))
		if startup == 0 {
			require.NoError(t, LOG_DB.Create(&AuditLog{EventId: "preserved-independent-audit", UserId: seedUser.Id, ActorRole: common.RoleCommonUser, CreatedAt: 100}).Error)
			require.NoError(t, LOG_DB.Migrator().DropIndex(&AuditLog{}, "idx_audit_time_event"))
		} else {
			var preservedAudit AuditLog
			require.NoError(t, LOG_DB.Where("event_id = ?", "preserved-independent-audit").First(&preservedAudit).Error)
			require.Equal(t, seedUser.Id, preservedAudit.UserId)
		}
		logPool, err := LOG_DB.DB()
		require.NoError(t, err)
		require.NoError(t, logPool.Close())
	}
}
