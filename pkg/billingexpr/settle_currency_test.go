package billingexpr_test

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeTieredQuotaNormalizesExpressionSourceCurrency(t *testing.T) {
	tests := []struct {
		name            string
		expression      string
		params          billingexpr.TokenParams
		request         billingexpr.RequestInput
		taskUsage       bool
		sourceCurrency  string
		cnyPerUSD       float64
		wantSourceCost  float64
		wantCostUSD     float64
		wantQuota       int
		wantBillingUnit billingexpr.BillingUnit
		wantFixedPrice  *float64
	}{
		{
			name:       "CNY token expression",
			expression: `tier("base", p * 2)`, params: billingexpr.TokenParams{P: 1_000_000},
			sourceCurrency: "CNY", cnyPerUSD: 7, wantSourceCost: 2, wantCostUSD: 2.0 / 7.0,
			wantQuota: 142_857, wantBillingUnit: billingexpr.BillingUnitToken,
		},
		{
			name:       "CNY task usage expression",
			expression: `tier("base", u("tokens") * 46 / 1000000)`,
			request:    billingexpr.RequestInput{Usage: map[string]any{"tokens": float64(1_000_000)}},
			taskUsage:  true, sourceCurrency: "CNY", cnyPerUSD: 7,
			wantSourceCost: 46, wantCostUSD: 46.0 / 7.0, wantQuota: 3_285_714,
			wantBillingUnit: billingexpr.BillingUnitToken,
		},
		{
			name:       "CNY fixed request expression",
			expression: `tier("fixed", fixed(14))`, sourceCurrency: "CNY", cnyPerUSD: 7,
			wantSourceCost: 14, wantCostUSD: 2, wantQuota: 1_000_000,
			wantBillingUnit: billingexpr.BillingUnitRequest, wantFixedPrice: floatPointer(14),
		},
		{
			name:       "USD token expression preserves quota",
			expression: `tier("base", p * 5)`, params: billingexpr.TokenParams{P: 1_000_000},
			sourceCurrency: "USD", cnyPerUSD: 7, wantSourceCost: 5, wantCostUSD: 5,
			wantQuota: 2_500_000, wantBillingUnit: billingexpr.BillingUnitToken,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snap := &billingexpr.BillingSnapshot{
				BillingMode: "tiered_expr", ExprString: test.expression,
				ExprHash: billingexpr.ExprHashString(test.expression), GroupRatio: 1,
				QuotaPerUnit: 500_000, ExprVersion: 1, TaskUsageBilling: test.taskUsage,
				SourceCurrency: test.sourceCurrency, CNYPerUSD: test.cnyPerUSD,
			}

			result, err := billingexpr.ComputeTieredQuotaWithRequest(snap, test.params, test.request)
			require.NoError(t, err)
			assert.InDelta(t, test.wantSourceCost, result.ActualSourceCost, 1e-12)
			assert.InDelta(t, test.wantCostUSD, result.ActualCostUSD, 1e-12)
			assert.Equal(t, test.wantQuota, result.ActualQuotaAfterGroup)
			assert.Equal(t, test.wantBillingUnit, result.BillingUnit)
			assert.Equal(t, test.wantFixedPrice, result.FixedPrice)
		})
	}
}

func TestComputeTieredQuotaUsesFrozenCurrencyRate(t *testing.T) {
	expression := `tier("base", p * 7)`
	snap := &billingexpr.BillingSnapshot{
		BillingMode: "tiered_expr", ExprString: expression, ExprHash: billingexpr.ExprHashString(expression),
		GroupRatio: 1.2, QuotaPerUnit: 500_000, ExprVersion: 1,
		SourceCurrency: "CNY", CNYPerUSD: 7,
	}

	result, err := billingexpr.ComputeTieredQuota(snap, billingexpr.TokenParams{P: 1_000_000})
	require.NoError(t, err)
	assert.Equal(t, 600_000, result.ActualQuotaAfterGroup)
	assert.Equal(t, 7.0, result.ActualSourceCost)
	assert.Equal(t, 1.0, result.ActualCostUSD)
}

func floatPointer(value float64) *float64 {
	return &value
}
