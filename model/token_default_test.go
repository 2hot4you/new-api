package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDefaultTokenTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}))
	DB = db
	LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestCreateSelfRegisteredUserCreatesOneDefaultToken(t *testing.T) {
	db := setupDefaultTokenTestDB(t)
	user := &User{Username: "default-key-user", Password: "password1", DisplayName: "Default Key User", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}

	require.NoError(t, CreateSelfRegisteredUser(user, 0))

	var tokens []Token
	require.NoError(t, db.Where("user_id = ?", user.Id).Find(&tokens).Error)
	require.Len(t, tokens, 1)
	require.True(t, tokens[0].IsDefault)
	require.Equal(t, int64(-1), tokens[0].ExpiredTime)
	require.True(t, tokens[0].UnlimitedQuota)
}

func TestCreateDefaultTokenWithTxIsIdempotent(t *testing.T) {
	db := setupDefaultTokenTestDB(t)
	user := &User{Username: "one-default-user", Password: "password1", DisplayName: "One Default", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, user.InsertWithTx(tx, 0))
		require.NoError(t, CreateDefaultTokenWithTx(tx, user))
		return CreateDefaultTokenWithTx(tx, user)
	}))

	var count int64
	require.NoError(t, db.Model(&Token{}).Where("user_id = ? AND is_default = ?", user.Id, true).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestTokenAutoMigrateAddsDefaultFlag(t *testing.T) {
	db := setupDefaultTokenTestDB(t)
	require.True(t, db.Migrator().HasColumn(&Token{}, "is_default"))
	token := &Token{UserId: 1, Key: "default-flag-value", Name: "Default flag"}
	require.NoError(t, db.Create(token).Error)
	var fetched Token
	require.NoError(t, db.First(&fetched, token.Id).Error)
	require.False(t, fetched.IsDefault)
}

func TestCreateSelfRegisteredUserRollsBackWhenDefaultTokenCreationFails(t *testing.T) {
	db := setupDefaultTokenTestDB(t)
	forcedErr := errors.New("forced default token failure")
	callbackName := "test:fail_default_token_creation"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "tokens" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })

	user := &User{Username: "rollback-default-key", Password: "password1", DisplayName: "Rollback", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	err := CreateSelfRegisteredUser(user, 0)
	require.ErrorIs(t, err, forcedErr)

	var users, tokens int64
	require.NoError(t, db.Model(&User{}).Count(&users).Error)
	require.NoError(t, db.Model(&Token{}).Count(&tokens).Error)
	require.Zero(t, users)
	require.Zero(t, tokens)
}

func TestDefaultTokenCannotBeDeletedButCanBeEditedAndDisabled(t *testing.T) {
	db := setupDefaultTokenTestDB(t)
	token := &Token{UserId: 1, Key: "default-token-protected", Name: "Default", IsDefault: true, Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}
	require.NoError(t, db.Create(token).Error)

	require.ErrorIs(t, DeleteTokenById(token.Id, token.UserId), ErrDefaultTokenDeleteForbidden)
	require.NoError(t, db.First(&Token{}, token.Id).Error)

	token.Name = "Renamed default"
	token.Status = common.TokenStatusDisabled
	require.NoError(t, token.Update())
	var updated Token
	require.NoError(t, db.First(&updated, token.Id).Error)
	require.Equal(t, "Renamed default", updated.Name)
	require.Equal(t, common.TokenStatusDisabled, updated.Status)
}

func TestBatchDeleteDefaultTokenIsAtomicAndNormalTokensRemainDeletable(t *testing.T) {
	db := setupDefaultTokenTestDB(t)
	defaultToken := &Token{UserId: 1, Key: "batch-default-token", Name: "Default", IsDefault: true, Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}
	normalToken := &Token{UserId: 1, Key: "batch-normal-token", Name: "Normal", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}
	require.NoError(t, db.Create(defaultToken).Error)
	require.NoError(t, db.Create(normalToken).Error)

	_, err := BatchDeleteTokens([]int{defaultToken.Id, normalToken.Id}, 1)
	require.ErrorIs(t, err, ErrDefaultTokenDeleteForbidden)
	var preserved int64
	require.NoError(t, db.Model(&Token{}).Where("user_id = ?", 1).Count(&preserved).Error)
	require.Equal(t, int64(2), preserved)

	count, err := BatchDeleteTokens([]int{normalToken.Id}, 1)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.NoError(t, db.First(&Token{}, defaultToken.Id).Error)
	require.Error(t, db.First(&Token{}, normalToken.Id).Error)
}
