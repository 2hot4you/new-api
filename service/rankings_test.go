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

func TestApplyRankingGroupSuccessKeepsRankingsAvailableWhenMetricsFail(t *testing.T) {
	response := &RankingsResponse{
		Models: []RankedModel{{ModelName: "still-ranked"}},
	}

	err := applyRankingGroupSuccess(
		response,
		24,
		[]string{"vip"},
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
