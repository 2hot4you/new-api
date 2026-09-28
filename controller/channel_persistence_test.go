package controller

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPersistedChannelTypesSurviveDatabaseReload(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "persisted-channels.db")
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	var initialDB, reopenedDB *gorm.DB
	initialClosed := false
	t.Cleanup(func() {
		if reopenedDB != nil {
			if sqlDB, dbErr := reopenedDB.DB(); dbErr == nil {
				_ = sqlDB.Close()
			}
		}
		if initialDB != nil && !initialClosed {
			if sqlDB, dbErr := initialDB.DB(); dbErr == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedisEnabled
	})

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	initialDB, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = initialDB, initialDB
	require.NoError(t, initialDB.AutoMigrate(&model.Channel{}))

	customGrokURL := "https://override.invalid"
	pluginURL := "https://plugin.example.invalid"
	seedanceURL := "https://seedance.example.invalid"
	vllmURL := "https://vllm.example.invalid"
	sglangURL := "https://sglang.example.invalid"
	tests := []struct {
		persistedType        int
		name                 string
		baseURL              *string
		wantRegistryName     string
		wantEffectiveBaseURL string
	}{
		{
			persistedType:        61,
			name:                 "persisted-starai",
			wantRegistryName:     "Molii Volcengine Imagine API",
			wantEffectiveBaseURL: "https://openapi.starcube.art",
		},
		{
			persistedType:        62,
			name:                 "persisted-molii-grok",
			baseURL:              &customGrokURL,
			wantRegistryName:     "Molii Grok Imagine API",
			wantEffectiveBaseURL: "https://api.wxiai.com/xai",
		},
		{
			persistedType:        63,
			name:                 "persisted-task-plugin",
			baseURL:              &pluginURL,
			wantRegistryName:     "Task Plugin",
			wantEffectiveBaseURL: pluginURL,
		},
		{
			persistedType:        64,
			name:                 "persisted-bytedance-seedance",
			baseURL:              &seedanceURL,
			wantRegistryName:     "ByteDance Seedance",
			wantEffectiveBaseURL: seedanceURL,
		},
		{
			persistedType:        65,
			name:                 "persisted-vllm",
			baseURL:              &vllmURL,
			wantRegistryName:     "vLLM",
			wantEffectiveBaseURL: vllmURL,
		},
		{
			persistedType:        66,
			name:                 "persisted-sglang",
			baseURL:              &sglangURL,
			wantRegistryName:     "SGLang",
			wantEffectiveBaseURL: sglangURL,
		},
	}

	channelIDs := make(map[int]int, len(tests))
	for _, test := range tests {
		channel := &model.Channel{
			Type:    test.persistedType,
			Name:    test.name,
			Key:     "fixture-key",
			Group:   "default",
			BaseURL: test.baseURL,
		}
		require.NoError(t, initialDB.Create(channel).Error)
		channelIDs[test.persistedType] = channel.Id
	}

	initialSQLDB, err := initialDB.DB()
	require.NoError(t, err)
	require.NoError(t, initialSQLDB.Close())
	initialClosed = true

	reopenedDB, err = gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = reopenedDB, reopenedDB

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reloaded, err := model.GetChannelById(channelIDs[test.persistedType], true)
			require.NoError(t, err)
			assert.Equal(t, test.persistedType, reloaded.Type)
			assert.Equal(t, test.wantRegistryName, constant.GetChannelTypeName(reloaded.Type))
			assert.Equal(t, test.wantEffectiveBaseURL, reloaded.GetBaseURL())
		})
	}
}
