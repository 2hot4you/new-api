package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPricingResponsePublishesCatalogBillingCurrency(t *testing.T) {
	resetPricingEndpointTestTables(t)
	row := completePublishedLLM()
	row.ModelName = "qwen3.5-flash"
	row.DisplayName = "Qwen 3.5 Flash"
	seedVendorPriceGroupAndEndpoint(t, &row)

	pricing := findPricingModel(GetPricing(), row.ModelName)
	require.NotNil(t, pricing)
	assert.Equal(t, "CNY", pricing.BillingCurrency)

	response, err := json.Marshal(struct {
		Data []Pricing `json:"data"`
	}{Data: GetPricing()})
	require.NoError(t, err)
	assert.Contains(t, string(response), `"billing_currency":"CNY"`)
}
