package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
)

func TestApplyEstimatedVideoQuotaUsesEstimatedTokens(t *testing.T) {
	info := &common.RelayInfo{
		EstimatedVideoTokens: 40595,
		PriceData: types.PriceData{
			ModelRatio: 18.5,
			Quota:      4_625_000,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
	}
	info.PriceData.AddOtherRatio("seedance-480p-no-video-input", 1)

	applyEstimatedVideoQuota(info)
	quotaWithRatios := info.PriceData.ApplyOtherRatiosToFloat(float64(info.PriceData.Quota))

	assert.InDelta(t, 40595*18.5, quotaWithRatios, 1)
	assert.Less(t, quotaWithRatios, 4_625_000.0)
}

func TestApplyEstimatedVideoQuotaIncludesConfiguredTierRatio(t *testing.T) {
	info := &common.RelayInfo{
		EstimatedVideoTokens: 40595,
		PriceData: types.PriceData{
			ModelRatio: 23,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
	}
	info.PriceData.AddOtherRatio("seedance-480p-video-input", 28.0/46.0)

	applyEstimatedVideoQuota(info)
	quotaWithRatios := info.PriceData.ApplyOtherRatiosToFloat(float64(info.PriceData.Quota))

	assert.InDelta(t, 40595*14, quotaWithRatios, 1)
}
