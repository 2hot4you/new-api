package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPricingResponsePublishesCatalogBillingCurrency(t *testing.T) {
	tests := []struct {
		name         string
		modelName    string
		wantCurrency string
		wantVideo    bool
		wantGrok     bool
	}{
		{name: "MiniMax direct CNY expression", modelName: "minimax-m3", wantCurrency: "CNY"},
		{name: "Qwen Flash direct CNY expression", modelName: "qwen3.5-flash", wantCurrency: "CNY"},
		{name: "Qwen Plus direct CNY expression", modelName: "qwen3.5-plus", wantCurrency: "CNY"},
		{name: "StarAI direct CNY matrix", modelName: "doubao-seedance-2-0-260128", wantCurrency: "CNY", wantVideo: true},
		{name: "Grok direct CNY matrix", modelName: "grok-imagine-image", wantCurrency: "CNY", wantGrok: true},
		{name: "ordinary USD pricing", modelName: "catalog-usd-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetPricingEndpointTestTables(t)
			row := completePublishedLLM()
			row.ModelName = tt.modelName
			row.DisplayName = tt.modelName
			seedVendorPriceGroupAndEndpoint(t, &row)

			pricing := findPricingModel(GetPricing(), row.ModelName)
			require.NotNil(t, pricing)
			assert.Equal(t, tt.wantCurrency, pricing.BillingCurrency)
			assert.Equal(t, tt.wantVideo, pricing.VideoPricing != nil)
			assert.Equal(t, tt.wantGrok, pricing.MoliiGrokPricing != nil)

			response, err := json.Marshal(pricing)
			require.NoError(t, err)
			if tt.wantCurrency == "" {
				assert.NotContains(t, string(response), `"billing_currency"`)
			} else {
				assert.Contains(t, string(response), `"billing_currency":"CNY"`)
			}
		})
	}
}
