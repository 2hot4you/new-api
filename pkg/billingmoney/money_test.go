package billingmoney

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextNormalizeSourceCost(t *testing.T) {
	tests := []struct {
		name           string
		currency       string
		rate           float64
		sourceCost     float64
		wantCurrency   Currency
		wantSourceCost float64
		wantCostUSD    float64
	}{
		{name: "USD", currency: "USD", rate: 7, sourceCost: 5, wantCurrency: CurrencyUSD, wantSourceCost: 5, wantCostUSD: 5},
		{name: "CNY", currency: "CNY", rate: 7, sourceCost: 2, wantCurrency: CurrencyCNY, wantSourceCost: 2, wantCostUSD: 2.0 / 7.0},
		{name: "zero CNY cost", currency: "CNY", rate: 7, sourceCost: 0, wantCurrency: CurrencyCNY, wantSourceCost: 0, wantCostUSD: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, err := NewContext(test.currency, test.rate)
			require.NoError(t, err)
			assert.Equal(t, test.wantCurrency, ctx.SourceCurrency)
			assert.Equal(t, test.rate, ctx.CNYPerUSD)

			amounts, err := ctx.Normalize(test.sourceCost)
			require.NoError(t, err)
			assert.Equal(t, test.wantSourceCost, amounts.SourceCost)
			assert.InDelta(t, test.wantCostUSD, amounts.CostUSD, 1e-12)
		})
	}
}

func TestNewContextRejectsInvalidSourceCurrency(t *testing.T) {
	_, err := NewContext("EUR", 7)
	require.ErrorContains(t, err, "source currency")
}

func TestNewContextRejectsInvalidCNYRate(t *testing.T) {
	for _, rate := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		t.Run(rateName(rate), func(t *testing.T) {
			_, err := NewContext("CNY", rate)
			require.ErrorContains(t, err, "CNY per USD")
		})
	}
}

func TestContextNormalizeRejectsInvalidSourceCost(t *testing.T) {
	ctx, err := NewContext("USD", 7)
	require.NoError(t, err)

	for _, sourceCost := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		t.Run(rateName(sourceCost), func(t *testing.T) {
			_, err := ctx.Normalize(sourceCost)
			require.ErrorContains(t, err, "source cost")
		})
	}
}

func rateName(value float64) string {
	switch {
	case math.IsNaN(value):
		return "nan"
	case math.IsInf(value, 1):
		return "positive infinity"
	case math.IsInf(value, -1):
		return "negative infinity"
	default:
		return "finite"
	}
}
