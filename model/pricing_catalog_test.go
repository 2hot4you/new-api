package model

import "testing"

func TestPricingCatalogHasNoRuntimeMetadataOverlay(t *testing.T) {
	if _, ok := getMarketplaceCatalogMetadata("deepseek-v4-flash-202605"); ok {
		t.Fatal("legacy catalog metadata must not be available to public pricing at runtime")
	}
	if currency := getMarketplaceBillingCurrency("qwen3.5-flash"); currency != "" {
		t.Fatalf("legacy catalog billing currency = %q, want empty", currency)
	}
}
