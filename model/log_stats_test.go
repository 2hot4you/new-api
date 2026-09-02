package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSumUsedQuotaPreservesQuotaWhenScanningRateStatistics(t *testing.T) {
	previousLogDB := LOG_DB
	previousLogDatabaseType := common.LogDatabaseType()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&Log{}))
	LOG_DB = database
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	initCol()
	t.Cleanup(func() {
		LOG_DB = previousLogDB
		common.SetLogDatabaseType(previousLogDatabaseType)
		initCol()
		sqlDB, sqlErr := database.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           1,
		Username:         "quota-stat-user",
		CreatedAt:        now,
		Type:             LogTypeConsume,
		Quota:            42,
		PromptTokens:     3,
		CompletionTokens: 4,
	}).Error)

	stat, err := SumUsedQuota(LogTypeConsume, now-1, now+1, "", "quota-stat-user", "", 0, "")
	require.NoError(t, err)
	assert.Equal(t, 42, stat.Quota)
	assert.Equal(t, 1, stat.Rpm)
	assert.Equal(t, 7, stat.Tpm)
}
