package billing_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/require"
)

func TestDefaultTieredModelExpressionsChargeEveryPublishedCoefficient(t *testing.T) {
	cases := []struct {
		model  string
		params billingexpr.TokenParams
		want   float64
		tier   string
	}{
		// p/c/cr are intentionally all non-zero. len selects the provider's
		// input-window tier even when most context was served from cache.
		{model: "minimax-m3", params: billingexpr.TokenParams{P: 100, C: 10, CR: 511_890, Len: 512_000}, want: 100*2.1 + 10*8.4 + 511_890*0.42, tier: "up_to_512k"},
		{model: "minimax-m3", params: billingexpr.TokenParams{P: 100, C: 10, CR: 511_891, Len: 512_001}, want: 100*4.2 + 10*16.8 + 511_891*0.84, tier: "over_512k"},
		{model: "qwen3.5-flash", params: billingexpr.TokenParams{P: 100, C: 10, CR: 127_890, Len: 128_000}, want: 100*0.2 + 10*2 + 127_890*0.02, tier: "up_to_128k"},
		{model: "qwen3.5-flash", params: billingexpr.TokenParams{P: 100, C: 10, CR: 255_890, Len: 256_000}, want: 100*0.8 + 10*8 + 255_890*0.08, tier: "128k_to_256k"},
		{model: "qwen3.5-flash", params: billingexpr.TokenParams{P: 100, C: 10, CR: 999_890, Len: 1_000_000}, want: 100*1.2 + 10*12 + 999_890*0.12, tier: "256k_to_1m"},
		{model: "qwen3.5-plus", params: billingexpr.TokenParams{P: 100, C: 10, CR: 127_890, Len: 128_000}, want: 100*0.8 + 10*4.8 + 127_890*0.08, tier: "up_to_128k"},
		{model: "qwen3.5-plus", params: billingexpr.TokenParams{P: 100, C: 10, CR: 255_890, Len: 256_000}, want: 100*2 + 10*12 + 255_890*0.2, tier: "128k_to_256k"},
		{model: "qwen3.5-plus", params: billingexpr.TokenParams{P: 100, C: 10, CR: 999_890, Len: 1_000_000}, want: 100*4 + 10*24 + 999_890*0.4, tier: "256k_to_1m"},
	}

	for _, tc := range cases {
		t.Run(tc.model+"_"+tc.tier, func(t *testing.T) {
			expr, ok := GetBillingExpr(tc.model)
			require.True(t, ok)
			cost, trace, err := billingexpr.RunExpr(expr, tc.params)
			require.NoError(t, err)
			require.Equal(t, tc.tier, trace.MatchedTier)
			require.InDelta(t, tc.want, cost, 1e-9)
		})
	}
}

func TestDefaultTieredModelExpressionsUseFullInputLengthBoundaries(t *testing.T) {
	cases := []struct {
		model string
		len   float64
		want  string
	}{
		{model: "minimax-m3", len: 512_000, want: "up_to_512k"},
		{model: "minimax-m3", len: 512_001, want: "over_512k"},
		{model: "qwen3.5-flash", len: 128_000, want: "up_to_128k"},
		{model: "qwen3.5-flash", len: 128_001, want: "128k_to_256k"},
		{model: "qwen3.5-flash", len: 256_000, want: "128k_to_256k"},
		{model: "qwen3.5-flash", len: 256_001, want: "256k_to_1m"},
		{model: "qwen3.5-flash", len: 1_000_000, want: "256k_to_1m"},
		{model: "qwen3.5-plus", len: 128_000, want: "up_to_128k"},
		{model: "qwen3.5-plus", len: 128_001, want: "128k_to_256k"},
		{model: "qwen3.5-plus", len: 256_000, want: "128k_to_256k"},
		{model: "qwen3.5-plus", len: 256_001, want: "256k_to_1m"},
		{model: "qwen3.5-plus", len: 1_000_000, want: "256k_to_1m"},
	}

	for _, tc := range cases {
		t.Run(tc.model+"_"+tc.want, func(t *testing.T) {
			expr, ok := GetBillingExpr(tc.model)
			require.True(t, ok)

			_, trace, err := billingexpr.RunExpr(expr, billingexpr.TokenParams{
				// p is deliberately below every pricing threshold: the tier must
				// follow the full request context in len, including cache hits.
				P:   1,
				C:   1,
				CR:  tc.len - 1,
				Len: tc.len,
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, trace.MatchedTier)
		})
	}
}
