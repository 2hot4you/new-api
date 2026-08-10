package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitializeChannelCacheAtStartupWarnsWithMemoryCacheDisabled(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	model.DB = db
	for _, name := range []string{"first", "second"} {
		require.NoError(t, db.Create(&model.Channel{
			Type:   constant.ChannelTypeStarAI,
			Status: common.ChannelStatusEnabled,
			Name:   name,
			Models: "doubao-seedance-2-0-260128",
			Group:  "default",
		}).Error)
	}

	var output bytes.Buffer
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultWriter
	gin.DefaultWriter = &output
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		common.LogWriterMu.Unlock()
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	require.NotPanics(t, initializeChannelCacheAtStartup)
	assert.Contains(t, output.String(), "expected exactly one enabled channel")
	assert.Contains(t, output.String(), "found 2")
	assert.Contains(t, output.String(), "startup will continue")
	assert.NotContains(t, output.String(), "channels synced from database")
}
