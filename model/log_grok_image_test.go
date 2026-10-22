package model

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useGrokImageLogTestDB(t *testing.T) {
	t.Helper()
	if mode := os.Getenv("CATALOG_SYNC_POSTGRES_ONLY"); mode != "" {
		require.Equal(t, "1", mode, "invalid PostgreSQL-only test mode")
		db := catalogSyncTestDB(t, "postgres")
		previousLogDB, previousLogType := LOG_DB, common.LogDatabaseType()
		LOG_DB = db
		common.SetLogDatabaseType(common.DatabaseTypePostgreSQL)
		t.Cleanup(func() {
			LOG_DB = previousLogDB
			common.SetLogDatabaseType(previousLogType)
		})
		require.NoError(t, db.AutoMigrate(&Log{}))
		return
	}
	previousDB, previousLogDB := DB, LOG_DB
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	DB, LOG_DB = db, db
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func seedGrokImageLogs(t *testing.T) {
	t.Helper()
	logs := []Log{
		{UserId: 7, CreatedAt: 10, Type: LogTypeConsume, ModelName: "grok-imagine-image", RequestId: "grok-basic"},
		{UserId: 7, CreatedAt: 20, Type: LogTypeConsume, ModelName: "grok-imagine-image-quality", RequestId: "grok-quality"},
		{UserId: 7, CreatedAt: 30, Type: LogTypeConsume, ModelName: "grok-imagine-image-2.0", RequestId: "grok-2"},
		{UserId: 7, CreatedAt: 40, Type: LogTypeConsume, ModelName: "grok-imagine-image-extra", RequestId: "near-match"},
		{UserId: 8, CreatedAt: 50, Type: LogTypeConsume, ModelName: "seedance-1.0-pro", RequestId: "other-fixed-price"},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)
}

func TestGetAllLogsGrokImageCategoryIsExactAndPaginated(t *testing.T) {
	useGrokImageLogTestDB(t)
	seedGrokImageLogs(t)

	firstPage, total, err := GetAllLogs(LogFilter{Type: LogTypeUnknown, LogCategory: "grok_image"}, 0, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, firstPage, 1)
	assert.Equal(t, "grok-imagine-image-2.0", firstPage[0].ModelName)

	secondPage, total, err := GetAllLogs(LogFilter{Type: LogTypeUnknown, LogCategory: "grok_image"}, 1, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, secondPage, 1)
	assert.Equal(t, "grok-imagine-image-quality", secondPage[0].ModelName)
}

func TestGetUserLogsGrokImageCategoryAndNoCategoryCompatibility(t *testing.T) {
	useGrokImageLogTestDB(t)
	seedGrokImageLogs(t)

	grokLogs, total, err := GetUserLogs(7, LogFilter{Type: LogTypeUnknown, LogCategory: "grok_image"}, 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	assert.ElementsMatch(t, []string{"grok-imagine-image", "grok-imagine-image-quality", "grok-imagine-image-2.0"}, []string{grokLogs[0].ModelName, grokLogs[1].ModelName, grokLogs[2].ModelName})

	allLogs, total, err := GetUserLogs(7, LogFilter{Type: LogTypeUnknown}, 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 4, total)
	assert.Len(t, allLogs, 4)
}

func TestFormatUserLogsPreservesGrokImageBilling(t *testing.T) {
	logs := []*Log{{
		Other: common.MapToJsonStr(map[string]interface{}{
			"admin_info": map[string]interface{}{"use_channel": []int{1}},
			"grok_image_billing": map[string]interface{}{
				"version":    1,
				"final_cost": 0,
			},
		}),
	}}

	formatUserLogs(logs, 0)

	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, other, "admin_info")
	billing, ok := other["grok_image_billing"].(map[string]interface{})
	require.True(t, ok)
	assert.EqualValues(t, 1, billing["version"])
	assert.EqualValues(t, 0, billing["final_cost"])
}

func TestGetUserLogsGPTImage2CategoryIsExact(t *testing.T) {
	useGrokImageLogTestDB(t)
	logs := []Log{
		{UserId: 7, CreatedAt: 10, Type: LogTypeConsume, ModelName: "gpt-image-2", RequestId: "gpt-image-2"},
		{UserId: 7, CreatedAt: 20, Type: LogTypeConsume, ModelName: "gpt-image-2-preview", RequestId: "near-match"},
		{UserId: 7, CreatedAt: 30, Type: LogTypeConsume, ModelName: "gpt-image-1", RequestId: "other-image"},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	imageLogs, total, err := GetUserLogs(7, LogFilter{Type: LogTypeUnknown, LogCategory: "gpt_image_2"}, 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, imageLogs, 1)
	assert.Equal(t, "gpt-image-2", imageLogs[0].ModelName)
}

func TestStreamImageLogsPreservesCategoryAndUserIsolation(t *testing.T) {
	useGrokImageLogTestDB(t)
	seedGrokImageLogs(t)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 8, CreatedAt: 60, Type: LogTypeConsume, ModelName: "grok-imagine-image", RequestId: "other-user"}).Error)
	filter := LogFilter{Type: LogTypeConsume, LogCategory: "grok_image"}
	var userRequests, adminRequests []string
	require.NoError(t, StreamUserLogs(context.Background(), 7, filter, func(log *Log) error {
		assert.Equal(t, 7, log.UserId)
		userRequests = append(userRequests, log.RequestId)
		return nil
	}))
	assert.ElementsMatch(t, []string{"grok-basic", "grok-quality", "grok-2"}, userRequests)
	require.NoError(t, StreamAllLogs(context.Background(), filter, common.RoleRootUser, func(log *Log) error {
		adminRequests = append(adminRequests, log.RequestId)
		return nil
	}))
	assert.Equal(t, []string{"other-user", "grok-2", "grok-quality", "grok-basic"}, adminRequests)
}
