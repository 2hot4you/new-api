package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRotateTokenKeyAllowsDefaultAndInvalidatesOldKey(t *testing.T) {
	db := setupDefaultTokenTestDB(t)
	token := &Token{UserId: 1, Key: "old-default-key", Name: "Default", IsDefault: true}
	require.NoError(t, db.Create(token).Error)

	rotated, err := RotateTokenKeyByID(token.Id, token.UserId)
	require.NoError(t, err)
	require.NotEqual(t, "old-default-key", rotated.Key)
	require.True(t, rotated.IsDefault)
	_, err = GetTokenByKey("old-default-key", true)
	require.Error(t, err)
	current, err := GetTokenByKey(rotated.Key, true)
	require.NoError(t, err)
	require.Equal(t, token.Id, current.Id)
}

func TestRotateTokenKeyRejectsOtherUsersAndLeavesKeyUnchangedOnDatabaseFailure(t *testing.T) {
	db := setupDefaultTokenTestDB(t)
	token := &Token{UserId: 1, Key: "old-owned-key", Name: "Owned"}
	require.NoError(t, db.Create(token).Error)
	_, err := RotateTokenKeyByID(token.Id, 2)
	require.Error(t, err)

	forcedErr := errors.New("forced rotate failure")
	callbackName := "test:fail_token_key_rotation"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "tokens" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callbackName) })
	_, err = RotateTokenKeyByID(token.Id, token.UserId)
	require.ErrorIs(t, err, forcedErr)

	var unchanged Token
	require.NoError(t, db.First(&unchanged, token.Id).Error)
	require.Equal(t, "old-owned-key", unchanged.Key)
}

func TestRotateTokenKeyLeavesOldKeyUnchangedWhenGenerationFails(t *testing.T) {
	db := setupDefaultTokenTestDB(t)
	token := &Token{UserId: 1, Key: "generation-failure-old-key", Name: "Owned"}
	require.NoError(t, db.Create(token).Error)
	previousGenerator := generateTokenKey
	generateTokenKey = func() (string, error) { return "", errors.New("forced key generation failure") }
	t.Cleanup(func() { generateTokenKey = previousGenerator })

	_, err := RotateTokenKeyByID(token.Id, token.UserId)
	require.EqualError(t, err, "forced key generation failure")
	var unchanged Token
	require.NoError(t, db.First(&unchanged, token.Id).Error)
	require.Equal(t, "generation-failure-old-key", unchanged.Key)
}
