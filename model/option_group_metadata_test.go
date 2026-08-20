package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const groupMetadataOptionKey = "group_ratio_setting.group_metadata"

func TestUpdateOptionValidatesAndPersistsGroupMetadata(t *testing.T) {
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
	assert.Equal(t, value, requireOptionValue(t, db, groupMetadataOptionKey))
	assert.Equal(t, []ratio_setting.GroupMetadata{{Name: "vip", Icon: "DeepSeek.Color", Recommendation: 4}}, ratio_setting.GetGroupMetadataCopy())

	assert.Error(t, UpdateOption(groupMetadataOptionKey, `[{"name":"","icon":"OpenAI.Color","recommendation":5}]`))
	assert.Equal(t, value, requireOptionValue(t, db, groupMetadataOptionKey))
}
