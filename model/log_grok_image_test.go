package model

import (
	"fmt"
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
		{UserId: 7, CreatedAt: 30, Type: LogTypeConsume, ModelName: "grok-imagine-image-extra", RequestId: "near-match"},
		{UserId: 8, CreatedAt: 40, Type: LogTypeConsume, ModelName: "seedance-1.0-pro", RequestId: "other-fixed-price"},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)
}

func TestGetAllLogsGrokImageCategoryIsExactAndPaginated(t *testing.T) {
	useGrokImageLogTestDB(t)
	seedGrokImageLogs(t)

	firstPage, total, err := GetAllLogs(LogTypeUnknown, 0, 0, "", "", "", 0, 1, 0, "", "", "", "grok_image")
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, firstPage, 1)
	assert.Equal(t, "grok-imagine-image-quality", firstPage[0].ModelName)

	secondPage, total, err := GetAllLogs(LogTypeUnknown, 0, 0, "", "", "", 1, 1, 0, "", "", "", "grok_image")
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, secondPage, 1)
	assert.Equal(t, "grok-imagine-image", secondPage[0].ModelName)
}

func TestGetUserLogsGrokImageCategoryAndNoCategoryCompatibility(t *testing.T) {
	useGrokImageLogTestDB(t)
	seedGrokImageLogs(t)

	grokLogs, total, err := GetUserLogs(7, LogTypeUnknown, 0, 0, "", "", 0, 10, "", "", "", "grok_image")
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.ElementsMatch(t, []string{"grok-imagine-image", "grok-imagine-image-quality"}, []string{grokLogs[0].ModelName, grokLogs[1].ModelName})

	allLogs, total, err := GetUserLogs(7, LogTypeUnknown, 0, 0, "", "", 0, 10, "", "", "", "")
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	assert.Len(t, allLogs, 3)
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
