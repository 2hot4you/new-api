package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRankedModelsIncludesConfiguredModelIcon(t *testing.T) {
	rows := buildRankedModels(
		[]model.RankingQuotaTotal{{ModelName: "claude-sonnet-4-6", TotalTokens: 42}},
		42,
		nil,
		nil,
		map[string]rankingModelMeta{
			"claude-sonnet-4-6": {
				vendor:     "Anthropic",
				vendorIcon: "Anthropic.Color",
				modelIcon:  "Claude.Color",
			},
		},
		false,
	)

	require.Len(t, rows, 1)
	assert.Equal(t, "Claude.Color", rows[0].ModelIcon)
}

func TestBuildModelHistoryKeepsEveryModelSeparateWithConfiguredIcons(t *testing.T) {
	totals := []model.RankingQuotaTotal{
		{ModelName: "model-01", TotalTokens: 120},
		{ModelName: "model-02", TotalTokens: 110},
		{ModelName: "model-03", TotalTokens: 100},
		{ModelName: "model-04", TotalTokens: 90},
		{ModelName: "model-05", TotalTokens: 80},
		{ModelName: "model-06", TotalTokens: 70},
		{ModelName: "model-07", TotalTokens: 60},
		{ModelName: "model-08", TotalTokens: 50},
		{ModelName: "model-09", TotalTokens: 40},
		{ModelName: "model-10", TotalTokens: 30},
		{ModelName: "model-11", TotalTokens: 20},
		{ModelName: "model-12", TotalTokens: 10},
	}
	buckets := make([]model.RankingQuotaBucket, 0, len(totals))
	meta := make(map[string]rankingModelMeta, len(totals))
	for _, item := range totals {
		buckets = append(buckets, model.RankingQuotaBucket{
			ModelName: item.ModelName,
			Bucket:    1_700_000_000,
			Tokens:    item.TotalTokens,
		})
		meta[item.ModelName] = rankingModelMeta{
			vendor:    "Vendor",
			modelIcon: item.ModelName + ".Color",
		}
	}

	history := buildModelHistory(
		buckets,
		totals,
		meta,
		rankingPeriodConfig{labelLayout: "Jan 2"},
	)

	require.Len(t, history.Models, 12)
	assert.Equal(t, "model-01", history.Models[0].Name)
	assert.Equal(t, "model-01.Color", history.Models[0].ModelIcon)
	assert.Equal(t, "model-12", history.Models[11].Name)
	assert.Equal(t, "model-12.Color", history.Models[11].ModelIcon)
	assert.NotContains(t, history.Models, ModelHistoryModel{Name: rankingOthersLabel, Vendor: "Various", Total: 30})

	require.Len(t, history.Points, 12)
	pointNames := make([]string, 0, len(history.Points))
	for _, point := range history.Points {
		pointNames = append(pointNames, point.Model)
	}
	assert.Equal(t, []string{
		"model-01", "model-02", "model-03", "model-04", "model-05", "model-06",
		"model-07", "model-08", "model-09", "model-10", "model-11", "model-12",
	}, pointNames)
	assert.NotContains(t, pointNames, rankingOthersLabel)
}

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
