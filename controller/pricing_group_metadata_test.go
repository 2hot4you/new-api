package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
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

func TestFilterPublicPricingGroupsKeepsOnlyGroupsWithVisibleModels(t *testing.T) {
	groups := filterPublicPricingGroups(
		[]model.Pricing{
			{ModelName: "doubao-seedance-2-5", EnableGroup: []string{"ByteDance"}},
		},
		map[string]string{
			"ByteDance": "ByteDance routes",
			"empty":     "No published models",
		},
	)

	assert.Equal(t, map[string]string{"ByteDance": "ByteDance routes"}, groups)
}

func TestBuildPublicPricingGroupRatiosAppliesIdentityOverrideWithoutExposingIdentity(t *testing.T) {
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalSpecialRatios := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalSpecialRatios))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{
		"ByteDance": 1,
		"zhoujian": 1
	}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{
		"zhoujian": {"ByteDance": 0.77}
	}`))

	ratios := buildPublicPricingGroupRatios(
		"zhoujian",
		map[string]string{"ByteDance": "ByteDance routes"},
	)

	assert.Equal(t, map[string]float64{"ByteDance": 0.77}, ratios)
	assert.NotContains(t, ratios, "zhoujian")
}
