package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const groupMetadataOptionKey = "group_ratio_setting.group_metadata"

func TestUpdateOptionNormalizesAndPersistsGroupMetadata(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	originalMetadata := ratio_setting.GroupMetadata2JSONString()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.OptionMap = originalOptionMap
		require.NoError(t, ratio_setting.UpdateGroupMetadataByJSONString(originalMetadata))
	})

	value := `[{"name":"vip","icon":"DeepSeek.Color","recommendation":4}]`
	require.NoError(t, UpdateOption(groupMetadataOptionKey, value))
	assert.JSONEq(t, `[{"name":"vip","icon":"DeepSeek.Color"}]`, requireOptionValue(t, db, groupMetadataOptionKey))
	assert.JSONEq(t, `[{"name":"vip","icon":"DeepSeek.Color"}]`, common.OptionMap[groupMetadataOptionKey])
	assert.Equal(t, []ratio_setting.GroupMetadata{{Name: "vip", Icon: "DeepSeek.Color"}}, ratio_setting.GetGroupMetadataCopy())

	assert.Error(t, UpdateOption(groupMetadataOptionKey, `[{"name":"","icon":"OpenAI.Color"}]`))
	assert.JSONEq(t, `[{"name":"vip","icon":"DeepSeek.Color"}]`, requireOptionValue(t, db, groupMetadataOptionKey))
}

func TestLoadOptionsFromDatabaseNormalizesLegacyGroupMetadata(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	originalMetadata := ratio_setting.GroupMetadata2JSONString()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.OptionMap = originalOptionMap
		require.NoError(t, ratio_setting.UpdateGroupMetadataByJSONString(originalMetadata))
	})

	require.NoError(t, db.Create(&Option{
		Key:   groupMetadataOptionKey,
		Value: `[{"name":"vip","icon":"DeepSeek.Color","recommendation":4}]`,
	}).Error)

	loadOptionsFromDatabase()

	assert.JSONEq(t, `[{"name":"vip","icon":"DeepSeek.Color"}]`, requireOptionValue(t, db, groupMetadataOptionKey))
	assert.JSONEq(t, `[{"name":"vip","icon":"DeepSeek.Color"}]`, common.OptionMap[groupMetadataOptionKey])
	assert.Equal(t, []ratio_setting.GroupMetadata{{Name: "vip", Icon: "DeepSeek.Color"}}, ratio_setting.GetGroupMetadataCopy())
}
