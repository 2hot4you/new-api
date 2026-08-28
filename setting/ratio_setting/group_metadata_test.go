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
		{name: "empty name", value: `[{"name":"","icon":"OpenAI.Color"}]`},
		{name: "duplicate name", value: `[{"name":"default","icon":"OpenAI.Color"},{"name":"default","icon":"DeepSeek.Color"}]`},
		{name: "icon longer than 128 characters", value: `[{"name":"default","icon":"` + strings.Repeat("a", 129) + `"}]`},
		{name: "non-positive vendor id", value: `[{"name":"default","icon":"OpenAI.Color","vendor_ids":[0]}]`},
		{name: "duplicate vendor id", value: `[{"name":"default","icon":"OpenAI.Color","vendor_ids":[2,2]}]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Error(t, UpdateGroupMetadataByJSONString(test.value))
		})
	}
}

func TestUpdateGroupMetadataPreservesVendorAssociations(t *testing.T) {
	original := GroupMetadata2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupMetadataByJSONString(original))
	})

	require.NoError(t, UpdateGroupMetadataByJSONString(`[
		{"name":"vip","icon":"Claude.Color","vendor_ids":[7,3]},
		{"name":"default","icon":"OpenAI.Color"}
	]`))

	metadata := GetGroupMetadataCopy()
	require.Len(t, metadata, 2)
	assert.Equal(t, []int{7, 3}, metadata[0].VendorIDs)
	assert.Empty(t, metadata[1].VendorIDs)

	metadata[0].VendorIDs[0] = 99
	assert.Equal(t, []int{7, 3}, GetGroupMetadataCopy()[0].VendorIDs)
	assert.JSONEq(t, `[
		{"name":"vip","icon":"Claude.Color","vendor_ids":[7,3]},
		{"name":"default","icon":"OpenAI.Color"}
	]`, GroupMetadata2JSONString())
}

func TestUpdateGroupMetadataDropsRetiredRecommendation(t *testing.T) {
	original := GroupMetadata2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupMetadataByJSONString(original))
	})

	require.NoError(t, UpdateGroupMetadataByJSONString(`[{"name":"vip","icon":"DeepSeek.Color","recommendation":99}]`))
	assert.Equal(t, []GroupMetadata{{Name: "vip", Icon: "DeepSeek.Color"}}, GetGroupMetadataCopy())
	assert.JSONEq(t, `[{"name":"vip","icon":"DeepSeek.Color"}]`, GroupMetadata2JSONString())
}

func TestUpdateGroupMetadataPreservesConfiguredOrder(t *testing.T) {
	original := GroupMetadata2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupMetadataByJSONString(original))
	})

	require.NoError(t, UpdateGroupMetadataByJSONString(`[
		{"name":"vip","icon":"DeepSeek.Color"},
		{"name":"default","icon":"OpenAI.Color"}
	]`))

	assert.Equal(t, []GroupMetadata{
		{Name: "vip", Icon: "DeepSeek.Color"},
		{Name: "default", Icon: "OpenAI.Color"},
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
	assert.Equal(t, []GroupMetadata{{Name: "vip", Icon: "DeepSeek.Color"}}, setting.GroupMetadata.copy())
	assert.JSONEq(t, `[{"name":"vip","icon":"DeepSeek.Color"}]`, setting.GroupMetadata.jsonString())
}
