package service

import (
	"errors"
	"testing"

	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRankingConfiguredGroupsPreservesMetadataOrder(t *testing.T) {
	groups := rankingConfiguredGroups(
		map[string]float64{
			"default": 1,
			"vip":     1,
			"alpha":   1,
			"zeta":    1,
		},
		[]ratio_setting.GroupMetadata{
			{Name: "vip"},
			{Name: "default"},
			{Name: "retired"},
		},
	)

	assert.Equal(t, []string{"vip", "default", "alpha", "zeta"}, groups)
	assert.NotContains(t, groups, "auto")
}

func TestRankingConfiguredGroupSuccessKeepsMetadataInConfiguredOrder(t *testing.T) {
	groups := rankingConfiguredGroupSuccess(
		map[string]float64{
			"default": 1,
			"vip":     1,
			"alpha":   1,
			"zeta":    1,
		},
		[]ratio_setting.GroupMetadata{
			{Name: "vip", Icon: "DeepSeek.Color"},
			{Name: "default", Icon: "OpenAI.Color"},
			{Name: "retired", Icon: "Claude.Color"},
		},
		map[string]string{
			"vip":     "Priority access",
			"default": "Standard access",
			"alpha":   "Alpha access",
		},
	)

	assert.Equal(t, []RankingGroupSuccess{
		{Group: "vip", Icon: "DeepSeek.Color", Description: "Priority access"},
		{Group: "default", Icon: "OpenAI.Color", Description: "Standard access"},
		{Group: "alpha", Description: "Alpha access"},
		{Group: "zeta"},
	}, groups)
}

func TestApplyRankingGroupSuccessMergesUnorderedMetricsIntoConfiguredMetadata(t *testing.T) {
	response := &RankingsResponse{}
	vipRate := 98.5
	defaultRate := 87.5
	configuredGroups := []RankingGroupSuccess{
		{Group: "vip", Icon: "DeepSeek.Color", Description: "Priority access"},
		{Group: "default", Icon: "OpenAI.Color", Description: "Standard access"},
		{Group: "alpha", Description: "Alpha access"},
	}

	err := applyRankingGroupSuccess(
		response,
		24,
		configuredGroups,
		func(int, []string) (perfmetrics.SummaryAllResult, error) {
			return perfmetrics.SummaryAllResult{Groups: []perfmetrics.GroupSummary{
				{Group: "default", RequestCount: 8, SuccessRate: &defaultRate},
				{Group: "vip", RequestCount: 12, SuccessRate: &vipRate},
			}}, nil
		},
	)

	require.NoError(t, err)
	assert.True(t, response.GroupSuccessAvailable)
	assert.Equal(t, []RankingGroupSuccess{
		{
			Group:        "vip",
			RequestCount: 12,
			SuccessRate:  &vipRate,
			Icon:         "DeepSeek.Color",
			Description:  "Priority access",
		},
		{
			Group:        "default",
			RequestCount: 8,
			SuccessRate:  &defaultRate,
			Icon:         "OpenAI.Color",
			Description:  "Standard access",
		},
		{Group: "alpha", Description: "Alpha access"},
	}, response.GroupSuccess)
}

func TestApplyRankingGroupSuccessKeepsRankingsAvailableWhenMetricsFail(t *testing.T) {
	response := &RankingsResponse{
		Models: []RankedModel{{ModelName: "still-ranked"}},
	}

	err := applyRankingGroupSuccess(
		response,
		24,
		[]RankingGroupSuccess{{Group: "vip"}},
		func(int, []string) (perfmetrics.SummaryAllResult, error) {
			return perfmetrics.SummaryAllResult{}, errors.New("metrics unavailable")
		},
	)

	require.Error(t, err)
	assert.False(t, response.GroupSuccessAvailable)
	assert.Empty(t, response.GroupSuccess)
	assert.NotNil(t, response.GroupSuccess)
	require.Len(t, response.Models, 1)
	assert.Equal(t, "still-ranked", response.Models[0].ModelName)
}
