package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestBuildPublicPricingGroupMetadataOnlyExposesUsableConfiguredGroups(t *testing.T) {
	original := ratio_setting.GroupMetadata2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupMetadataByJSONString(original))
	})
	require.NoError(t, ratio_setting.UpdateGroupMetadataByJSONString(`[
		{"name":"anthropic","icon":"Anthropic.Color","vendor_ids":[3]},
		{"name":"private","icon":"OpenAI.Color","vendor_ids":[4]}
	]`))

	metadata := buildPublicPricingGroupMetadata(map[string]string{
		"anthropic": "Anthropic routes",
		"default":   "Default routes",
	})

	require.Equal(t, "Anthropic.Color", metadata["anthropic"].Icon)
	require.Equal(t, "Anthropic routes", metadata["anthropic"].Description)
	require.NotContains(t, metadata, "private")
	require.Equal(t, "Default routes", metadata["default"].Description)
}
