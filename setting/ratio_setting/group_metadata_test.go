package ratio_setting

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupMetadataDefaultsToAnEmptyArray(t *testing.T) {
	original := GroupMetadata2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupMetadataByJSONString(original))
	})

	require.NoError(t, UpdateGroupMetadataByJSONString(`[]`))
	assert.Empty(t, GetGroupMetadataCopy())
	assert.JSONEq(t, `[]`, GroupMetadata2JSONString())
}

func TestUpdateGroupMetadataRejectsInvalidEntries(t *testing.T) {
	original := GroupMetadata2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupMetadataByJSONString(original))
	})

	tests := []struct {
		name  string
		value string
	}{
		{name: "empty name", value: `[{"name":"","icon":"OpenAI.Color","recommendation":5}]`},
		{name: "duplicate name", value: `[{"name":"default","icon":"OpenAI.Color","recommendation":5},{"name":"default","icon":"DeepSeek.Color","recommendation":4}]`},
		{name: "icon longer than 128 characters", value: `[{"name":"default","icon":"` + strings.Repeat("a", 129) + `","recommendation":5}]`},
		{name: "negative recommendation", value: `[{"name":"default","icon":"OpenAI.Color","recommendation":-1}]`},
		{name: "recommendation above five", value: `[{"name":"default","icon":"OpenAI.Color","recommendation":6}]`},
		{name: "non integer recommendation", value: `[{"name":"default","icon":"OpenAI.Color","recommendation":1.5}]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Error(t, UpdateGroupMetadataByJSONString(test.value))
		})
	}
}

func TestUpdateGroupMetadataPreservesConfiguredOrder(t *testing.T) {
	original := GroupMetadata2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupMetadataByJSONString(original))
	})

	require.NoError(t, UpdateGroupMetadataByJSONString(`[
		{"name":"vip","icon":"DeepSeek.Color","recommendation":4},
		{"name":"default","icon":"OpenAI.Color","recommendation":5}
	]`))

	assert.Equal(t, []GroupMetadata{
		{Name: "vip", Icon: "DeepSeek.Color", Recommendation: 4},
		{Name: "default", Icon: "OpenAI.Color", Recommendation: 5},
	}, GetGroupMetadataCopy())
}

func TestGroupMetadataStartupReloadHandlesMissingAndConfiguredOption(t *testing.T) {
	setting := GroupRatioSetting{GroupMetadata: newGroupMetadataStore()}
	manager := config.NewConfigManager()
	manager.Register("group_ratio_setting", &setting)

	require.NoError(t, manager.LoadFromDB(map[string]string{}))
	assert.JSONEq(t, `[]`, setting.GroupMetadata.jsonString())

	require.NoError(t, manager.LoadFromDB(map[string]string{
		"group_ratio_setting.group_metadata": `[{"name":"vip","icon":"DeepSeek.Color","recommendation":4}]`,
	}))
	assert.Equal(t, []GroupMetadata{{Name: "vip", Icon: "DeepSeek.Color", Recommendation: 4}}, setting.GroupMetadata.copy())
}
