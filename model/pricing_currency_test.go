package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPricingResponsePublishesCatalogBillingCurrency(t *testing.T) {
	tests := []struct {
		name           string
		modelName      string
		wantCurrency   string
		storedCurrency string
		wantVideo      bool
		wantGrok       bool
	}{
		{name: "stored CNY expression", modelName: "minimax-m3", storedCurrency: "CNY", wantCurrency: "CNY"},
		{name: "stored USD expression", modelName: "qwen3.5-flash", storedCurrency: "USD", wantCurrency: "USD"},
		{name: "stored USD StarAI matrix", modelName: "doubao-seedance-2-0-260128", storedCurrency: "USD", wantCurrency: "USD", wantVideo: true},
		{name: "stored CNY Grok matrix", modelName: "grok-imagine-image", storedCurrency: "CNY", wantCurrency: "CNY", wantGrok: true},
		{name: "ordinary USD pricing", modelName: "catalog-usd-model", storedCurrency: "USD", wantCurrency: "USD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetPricingEndpointTestTables(t)
			row := completePublishedLLM()
			row.ModelName = tt.modelName
			row.DisplayName = tt.modelName
			row.BillingCurrency = tt.storedCurrency
			seedVendorPriceGroupAndEndpoint(t, &row)

			pricing := findPricingModel(GetPricing(), row.ModelName)
			require.NotNil(t, pricing)
			assert.Equal(t, tt.wantCurrency, pricing.BillingCurrency)
			assert.Equal(t, tt.wantVideo, pricing.VideoPricing != nil)
			assert.Equal(t, tt.wantGrok, pricing.MoliiGrokPricing != nil)

			response, err := json.Marshal(pricing)
			require.NoError(t, err)
			assert.Contains(t, string(response), `"billing_currency":"`+tt.wantCurrency+`"`)
		})
	}
}
