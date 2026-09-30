package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
)

func TestInjectTieredBillingInfoIncludesFrozenCurrencyAudit(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	previousDisplay := setting.QuotaDisplayType
	setting.QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	t.Cleanup(func() { setting.QuotaDisplayType = previousDisplay })

	other := model.NewLogOther()
	info := &relaycommon.RelayInfo{TieredBillingSnapshot: &billingexpr.BillingSnapshot{
		ExprString:          `tier("base", p * 7)`,
		SourceCurrency:      "CNY",
		CNYPerUSD:           7,
		EstimatedSourceCost: 7,
		EstimatedCostUSD:    1,
	}}
	result := &billingexpr.TieredResult{ActualSourceCost: 14, ActualCostUSD: 2, MatchedTier: "base"}

	InjectTieredBillingInfo(other, info, result)
	snapshot := other.Snapshot()

	assert.Equal(t, "CNY", snapshot["source_currency"])
	assert.Equal(t, 14.0, snapshot["source_cost"])
	assert.Equal(t, 7.0, snapshot["cny_per_usd"])
	assert.Equal(t, 2.0, snapshot["cost_usd"])
	assert.Equal(t, "CNY", snapshot["display_currency"])
	assert.Equal(t, 14.0, snapshot["display_cost"])

	fallback := model.NewLogOther()
	InjectTieredBillingInfo(fallback, info, nil)
	fallbackSnapshot := fallback.Snapshot()
	assert.Equal(t, 7.0, fallbackSnapshot["source_cost"])
	assert.Equal(t, 1.0, fallbackSnapshot["cost_usd"])
	assert.Equal(t, 7.0, fallbackSnapshot["display_cost"])
}
